package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	applog "fastgame/pkg/log"
	"fastgame/services/consumer/internal/config"
	"fastgame/services/consumer/internal/svc"
	"fastgame/services/consumer/internal/worker"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/consumer.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	applog.MustSetup("consumer", c.Log)

	svcCtx, err := svc.NewServiceContext(c)
	if err != nil {
		logx.Must(err)
	}
	defer svcCtx.Writer.Close()

	interval, err := time.ParseDuration(c.Batch.Interval)
	if err != nil {
		logx.Must(err)
	}

	w := worker.NewWorker(svcCtx, c.Batch.MaxSize, interval)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logx.Infof("consumer started: topic=%s group=%s batch=%d/%s",
		c.Kafka.Topic, c.Kafka.GroupID, c.Batch.MaxSize, c.Batch.Interval)

	if err := w.Run(ctx); err != nil && err != context.Canceled {
		logx.Must(err)
	}
	_ = w.Close()
}
