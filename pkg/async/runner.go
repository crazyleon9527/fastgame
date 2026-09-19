// Package async 提供受管的后台任务执行器，用来取代裸 `go func()`。
//
// 为什么要它：裸 go func 有四个问题，且都只在出故障时才暴露。
//
//  1. panic 会打挂整个进程。Go 里 goroutine 的 panic 不会被调用方 recover，
//     主 goroutine 也不在调用栈上，进程直接退出——一个旁路埋点的空指针
//     就能让资金服务下线。Runner 里每个任务都 recover 并记录堆栈。
//  2. 没有生命周期。进程收到 SIGTERM 时，在途的 goroutine 会被直接抛弃，
//     任务处于"做了一半"的状态，没人知道。Runner 跟踪 WaitGroup，
//     支持带超时的 Drain。
//  3. 没有并发上限。突发流量下每秒几千个 goroutine 会让内存和下游连接池
//     一起炸。Runner 可以配并发闸门，过载时明确拒绝而不是无限制堆积。
//  4. 丢链路。裸 goroutine 里通常用 context.Background()，日志里就没有
//     trace_id 了。Runner 从父 context 继承 trace_id。
package async

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"fastgame/pkg/traceid"

	"github.com/zeromicro/go-zero/core/logx"
)

// DefaultDrainTimeout 关闭时等待在途任务的默认上限。
const DefaultDrainTimeout = 5 * time.Second

// Stats 是 Runner 的运行时计数，用于监控与排障。
type Stats struct {
	Name      string
	Started   int64 // 已启动的任务数
	Running   int64 // 当前在途任务数
	Panicked  int64 // 被 recover 的 panic 数
	Rejected  int64 // 因并发闸门满而被拒绝的任务数
	Dropped   int64 // 关闭后被拒绝的任务数
	Completed int64 // 正常结束的任务数
}

// Runner 是受管的后台任务执行器。
type Runner struct {
	name         string
	sem          chan struct{} // 并发闸门；nil 表示不限制
	drainTimeout time.Duration

	wg      sync.WaitGroup
	closed  atomic.Bool
	started atomic.Int64
	running atomic.Int64
	panics  atomic.Int64
	rejects atomic.Int64
	drops   atomic.Int64
	done    atomic.Int64
}

// Option 配置 Runner。
type Option func(*Runner)

// WithMaxConcurrency 限制同时在途的任务数；<=0 表示不限制。
// 超过上限时 Go 返回 false（拒绝），GoWait 阻塞等待空位。
func WithMaxConcurrency(n int) Option {
	return func(r *Runner) {
		if n > 0 {
			r.sem = make(chan struct{}, n)
		}
	}
}

// WithDrainTimeout 设置 Shutdown 等待在途任务的上限。
func WithDrainTimeout(d time.Duration) Option {
	return func(r *Runner) {
		if d > 0 {
			r.drainTimeout = d
		}
	}
}

// New 构造一个 Runner。name 会出现在所有日志里，便于定位是哪个后台任务出的问题。
func New(name string, opts ...Option) *Runner {
	r := &Runner{name: name, drainTimeout: DefaultDrainTimeout}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Go 异步执行 task，立即返回是否成功入队。
//
// 返回 false 的两种情况：Runner 已关闭，或并发闸门已满（过载）。
// 调用方应当根据返回值决定降级策略（丢弃 / 同步执行 / 打点告警），
// 而不是假设它一定会跑——这正是裸 go func 做不到的。
func (r *Runner) Go(parent context.Context, task func(ctx context.Context)) bool {
	if r == nil || task == nil {
		return false
	}
	if r.closed.Load() {
		r.drops.Add(1)
		return false
	}
	if r.sem != nil {
		select {
		case r.sem <- struct{}{}:
		default:
			r.rejects.Add(1)
			return false
		}
	}
	r.launch(parent, task)
	return true
}

// GoWait 与 Go 类似，但在并发闸门满时阻塞等待空位（尊重 ctx 取消）。
// 用于"可以等一下但不能丢"的任务。
func (r *Runner) GoWait(ctx context.Context, parent context.Context, task func(ctx context.Context)) bool {
	if r == nil || task == nil {
		return false
	}
	if r.closed.Load() {
		r.drops.Add(1)
		return false
	}
	if r.sem != nil {
		select {
		case r.sem <- struct{}{}:
		case <-ctx.Done():
			r.rejects.Add(1)
			return false
		}
	}
	r.launch(parent, task)
	return true
}

func (r *Runner) launch(parent context.Context, task func(ctx context.Context)) {
	r.wg.Add(1)
	r.started.Add(1)
	r.running.Add(1)

	go func() {
		defer r.wg.Done()
		defer func() {
			r.running.Add(-1)
			r.done.Add(1)
			if r.sem != nil {
				<-r.sem
			}
		}()
		r.exec(parent, task)
	}()
}

// exec 统一做 panic 兜底与链路继承。
//
// trace_id 从 parent 复制到任务 context：裸 goroutine 常见写法是
// context.Background()，链路就断在这里。
func (r *Runner) exec(parent context.Context, task func(ctx context.Context)) {
	defer func() {
		if rec := recover(); rec != nil {
			r.panics.Add(1)
			// 用 Background + trace_id 记录，保证 panic 一定留痕
			ctx := context.Background()
			if parent != nil {
				ctx = traceid.WithID(ctx, traceid.ID(parent))
			}
			logx.WithContext(ctx).Errorw("async_task_panic",
				logx.Field("runner", r.name),
				logx.Field("panic", fmt.Sprint(rec)),
				logx.Field("stack", string(debug.Stack())),
			)
		}
	}()

	ctx, cancel := Detach(parent, 0)
	defer cancel()
	task(ctx)
}

// Detach 返回一个只保留父 context 取值（trace_id 等）但不受其取消影响的
// context，并按需叠加 timeout；调用方应 defer cancel 以释放计时器。
//
// 为什么必须 Detach：调用方的请求 context 在响应返回后就会被 cancel，
// 而"请求返回后仍要跑完"的后台任务如果直接用父 ctx，会在写入前被取消。
func Detach(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if parent != nil {
		ctx = context.WithoutCancel(parent)
		if traceID := traceid.ID(parent); traceID != "" {
			ctx = traceid.WithID(ctx, traceID)
		}
	}
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

// Run 跑一个长驻循环（例如 outbox dispatcher），并把它纳入生命周期管理。
// loop 收到 ctx.Done() 后应当立刻返回。
func (r *Runner) Run(ctx context.Context, loop func(ctx context.Context)) {
	if r == nil || loop == nil {
		return
	}
	r.wg.Add(1)
	r.started.Add(1)
	r.running.Add(1)
	go func() {
		defer r.wg.Done()
		defer func() {
			r.running.Add(-1)
			r.done.Add(1)
		}()
		r.exec(ctx, loop)
	}()
}

// Running 返回当前在途任务数。
func (r *Runner) Running() int64 {
	if r == nil {
		return 0
	}
	return r.running.Load()
}

// Stats 返回计数快照。
func (r *Runner) Stats() Stats {
	if r == nil {
		return Stats{}
	}
	return Stats{
		Name:      r.name,
		Started:   r.started.Load(),
		Running:   r.running.Load(),
		Panicked:  r.panics.Load(),
		Rejected:  r.rejects.Load(),
		Dropped:   r.drops.Load(),
		Completed: r.done.Load(),
	}
}

// Shutdown 标记关闭并等待在途任务结束。
//
// 超时不会杀掉 goroutine（Go 里做不到），但会返回错误让调用方知道
// "还有任务没跑完就退出了"——日志比静默丢弃有用得多。
func (r *Runner) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.closed.Store(true)

	timeout := r.drainTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("async: runner %s drain aborted: %w (running=%d)", r.name, ctx.Err(), r.Running())
	case <-timer.C:
		return fmt.Errorf("async: runner %s drain timed out after %s, still running=%d", r.name, timeout, r.Running())
	}
}
