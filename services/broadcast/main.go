package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func main() {
	flag.Parse()

	var c Config
	conf.MustLoad(*configFile, &c)
	applog.MustSetup("broadcast", c.Log)

	h := hub.NewHub()
	w := worker.NewWorker(c.Kafka.Brokers, c.Kafka.GroupID, h)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			logx.Must(err)
		}
	}()

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
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
		_ = w.Close()
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logx.Must(err)
	}
}
