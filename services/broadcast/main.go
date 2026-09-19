package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fastgame/pkg/async"
	applog "fastgame/pkg/log"
	"fastgame/services/broadcast/internal/hub"
	"fastgame/services/broadcast/internal/worker"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/broadcast.yaml", "the config file")

type Config struct {
	Log   logx.LogConf
	Host  string
	Port  int
	Kafka struct {
		Brokers []string
		GroupID string
	}
	CORS struct {
		AllowedOrigins []string
	}
}

func makeCheckOrigin(allowed []string) func(*http.Request) bool {
	if len(allowed) == 0 {
		return func(r *http.Request) bool { return true }
	}
	set := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		set[origin] = struct{}{}
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		_, ok := set[origin]
		return ok
	}
}

func main() {
	flag.Parse()

	var c Config
	conf.MustLoad(*configFile, &c)
	applog.MustSetup("broadcast", c.Log)

	h := hub.NewHub()
	w := worker.NewWorker(c.Kafka.Brokers, c.Kafka.GroupID, h)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 后台任务统一交给 async.Runner：panic 会被 recover 并打日志（裸 goroutine
	// 里的 panic 会直接终止进程），进程退出前可以 Drain 等在途任务结束。
	tasks := async.New("broadcast")
	tasks.Run(ctx, func(runCtx context.Context) {
		if err := w.Run(runCtx); err != nil && err != context.Canceled {
			// 原来是 logx.Must(err)：消费循环一报错就 panic 打挂整个 websocket 服务
			logx.Errorf("kafka consumer stopped: %v", err)
		}
	})

	upgrader := websocket.Upgrader{CheckOrigin: makeCheckOrigin(c.CORS.AllowedOrigins)}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/bigwin", func(rw http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(rw, r, nil)
		if err != nil {
			return
		}
		h.Register(conn)
		defer h.Unregister(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	logx.Infof("broadcast websocket started at http://%s/ws/bigwin", addr)
	server := &http.Server{Addr: addr, Handler: mux}
	tasks.Run(ctx, func(context.Context) {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = w.Close()
	})
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logx.Must(err)
	}

	// 退出前等在途任务收尾（超时会打日志而不是静默丢弃）
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer drainCancel()
	if err := tasks.Shutdown(drainCtx); err != nil {
		logx.Errorf("broadcast shutdown: %v", err)
	}
}
