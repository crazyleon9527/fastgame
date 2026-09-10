package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	applog "fastgame/pkg/log"
	"fastgame/services/rollback/internal/config"
	"fastgame/services/rollback/internal/svc"
	"fastgame/services/rollback/internal/worker"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/rollback.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	applog.MustSetup("rollback", c.Log)

	svcCtx, err := svc.NewServiceContext(c)
	if err != nil {
		logx.Must(err)
	}
	defer svcCtx.Writer.Close()

	w := worker.NewWorker(svcCtx)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logx.Infof("rollback consumer started: topic=%s group=%s", c.Kafka.Topic, c.Kafka.GroupID)

	if err := w.Run(ctx); err != nil && err != context.Canceled {
		logx.Must(err)
	}
	_ = w.Close()
}
