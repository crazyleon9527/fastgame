package async

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"fastgame/pkg/traceid"
)

// TestRunnerRecoversPanic —— 这是 Runner 存在的首要理由：
// 裸 goroutine 里的 panic 会直接终止进程，Runner 必须把它兜住并计数。
func TestRunnerRecoversPanic(t *testing.T) {
	r := New("test-panic")
	done := make(chan struct{})
	r.Go(context.Background(), func(context.Context) {
		defer close(done)
		panic("boom: 模拟埋点空指针")
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("任务未执行")
	}

	// panic 后 Shutdown 必须能正常返回（说明 goroutine 已退出而不是卡住）
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 失败: %v", err)
	}

	st := r.Stats()
	if st.Panicked != 1 {
		t.Errorf("Panicked = %d，期望 1", st.Panicked)
	}
	if st.Completed != 1 {
		t.Errorf("Completed = %d，期望 1（panic 的任务也要算作结束）", st.Completed)
	}
	if st.Started != 1 {
		t.Errorf("Started = %d，期望 1", st.Started)
	}
}

// TestRunnerInheritsTraceID —— 裸 goroutine 常用 context.Background()，
// 日志里就丢了 trace_id；Runner 必须把父 context 的 trace_id 带进来。
func TestRunnerInheritsTraceID(t *testing.T) {
	r := New("test-trace")
	parent := traceid.WithID(context.Background(), "trace-abc-123")

	got := make(chan string, 1)
	r.Go(parent, func(ctx context.Context) {
		got <- traceid.ID(ctx)
	})

	select {
	case id := <-got:
		if id != "trace-abc-123" {
			t.Errorf("任务 context 里的 trace_id = %q，期望 %q", id, "trace-abc-123")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("任务未执行")
	}
}

// TestRunnerDetachesFromParentCancel —— 请求结束时父 ctx 会被 cancel，
// 后台任务不能因此被一起取消（否则"请求返回后仍要写"的数据就丢了）。
func TestRunnerDetachesFromParentCancel(t *testing.T) {
	r := New("test-detach")
	parent, cancelParent := context.WithCancel(context.Background())

	release := make(chan struct{})
	sawCancel := make(chan bool, 1)
	r.Go(parent, func(ctx context.Context) {
		cancelParent()
		for i := 0; i < 100; i++ {
			if ctx.Err() != nil {
				sawCancel <- true
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		_ = release
		sawCancel <- false
	})

	select {
	case cancelled := <-sawCancel:
		if cancelled {
			t.Error("父 context 被取消后任务 context 也取消了 —— 后台任务会被请求结束连坐")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("任务未在预期时间内结束")
	}
}

// TestRunnerRejectsWhenGateFull —— 过载时必须明确拒绝，而不是无限堆积 goroutine。
func TestRunnerRejectsWhenGateFull(t *testing.T) {
	r := New("test-gate", WithMaxConcurrency(1))

	block := make(chan struct{})
	if !r.Go(context.Background(), func(context.Context) { <-block }) {
		t.Fatal("第一个任务应当被接受")
	}
	// 等第一个任务真正占住闸门
	deadline := time.Now().Add(time.Second)
	for r.Running() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if r.Go(context.Background(), func(context.Context) {}) {
		t.Error("闸门已满时 Go 应返回 false")
	}
	if st := r.Stats(); st.Rejected != 1 {
		t.Errorf("Rejected = %d，期望 1", st.Rejected)
	}

	close(block)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 失败: %v", err)
	}
	// 闸门释放后又能接任务——但 Runner 已关闭，所以是 Dropped 而不是 Rejected
	if r.Go(context.Background(), func(context.Context) {}) {
		t.Error("Shutdown 之后 Go 应返回 false")
	}
	if st := r.Stats(); st.Dropped != 1 {
		t.Errorf("Dropped = %d，期望 1", st.Dropped)
	}
}

// TestRunnerGoWaitBlocksUntilSlot —— GoWait 用于"可以等但不能丢"的任务。
func TestRunnerGoWaitBlocksUntilSlot(t *testing.T) {
	r := New("test-gowait", WithMaxConcurrency(1))

	release := make(chan struct{})
	r.Go(context.Background(), func(context.Context) { <-release })

	ran := make(chan struct{})
	go func() {
		r.GoWait(context.Background(), context.Background(), func(context.Context) { close(ran) })
	}()

	select {
	case <-ran:
		t.Fatal("闸门未释放时 GoWait 不应立即执行任务")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("闸门释放后 GoWait 的任务仍未执行")
	}
}

// TestRunnerGoWaitHonoursContext —— 等不到槽位时要能被打断。
func TestRunnerGoWaitHonoursContext(t *testing.T) {
	r := New("test-gowait-ctx", WithMaxConcurrency(1))

	release := make(chan struct{})
	defer close(release)
	r.Go(context.Background(), func(context.Context) { <-release })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if r.GoWait(ctx, context.Background(), func(context.Context) {}) {
		t.Error("ctx 超时后 GoWait 应返回 false")
	}
	if st := r.Stats(); st.Rejected != 1 {
		t.Errorf("Rejected = %d，期望 1", st.Rejected)
	}
}

// TestRunnerShutdownWaitsForInflight —— 优雅关闭必须等在途任务结束。
func TestRunnerShutdownWaitsForInflight(t *testing.T) {
	r := New("test-drain")

	var mu sync.Mutex
	finished := 0
	for i := 0; i < 5; i++ {
		r.Go(context.Background(), func(context.Context) {
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			finished++
			mu.Unlock()
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 失败: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if finished != 5 {
		t.Errorf("Shutdown 返回时只有 %d/5 个任务结束 —— 没有真正 Drain", finished)
	}
	if st := r.Stats(); st.Running != 0 {
		t.Errorf("Running = %d，期望 0", st.Running)
	}
}

// TestRunnerShutdownTimesOutLoudly —— 超时不能静默：必须返回错误说明还有任务没跑完。
func TestRunnerShutdownTimesOutLoudly(t *testing.T) {
	r := New("test-drain-timeout", WithDrainTimeout(50*time.Millisecond))

	release := make(chan struct{})
	defer close(release)
	r.Go(context.Background(), func(context.Context) { <-release })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := r.Shutdown(ctx)
	if err == nil {
		t.Fatal("在途任务未结束时 Shutdown 应返回错误而不是静默通过")
	}
	if !strings.Contains(err.Error(), "drain timed out") {
		t.Errorf("错误信息应说明是 drain 超时，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "running=1") {
		t.Errorf("错误信息应带上未完成任务数，实际: %v", err)
	}
}

// TestDetachKeepsTraceAndIgnoresCancel 覆盖 Detach 的两个语义。
func TestDetachKeepsTraceAndIgnoresCancel(t *testing.T) {
	parent, cancel := context.WithCancel(traceid.WithID(context.Background(), "tid-1"))
	cancel()

	ctx, cancelDetached := Detach(parent, time.Second)
	defer cancelDetached()

	if ctx.Err() != nil {
		t.Errorf("Detach 不应继承父 ctx 的取消状态: %v", ctx.Err())
	}
	if got := traceid.ID(ctx); got != "tid-1" {
		t.Errorf("Detach 丢失了 trace_id: %q", got)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Error("带 timeout 的 Detach 应当有 deadline")
	}

	// nil 父 ctx 也要安全
	ctx2, cancel2 := Detach(nil, 0)
	defer cancel2()
	if ctx2 == nil {
		t.Error("Detach(nil) 不应返回 nil")
	}
}

// TestRunnerRunLongLivedLoop —— Run 用于长驻循环，也要有 panic 兜底与生命周期。
func TestRunnerRunLongLivedLoop(t *testing.T) {
	r := New("test-loop")
	started := make(chan struct{})
	r.Run(context.Background(), func(ctx context.Context) {
		close(started)
		panic("loop panic")
	})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("循环未启动")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 失败: %v", err)
	}
	if st := r.Stats(); st.Panicked != 1 {
		t.Errorf("长驻循环的 panic 未被计入: %+v", st)
	}
}
