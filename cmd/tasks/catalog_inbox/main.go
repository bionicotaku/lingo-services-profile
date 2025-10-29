// Package main 提供 Catalog Inbox Runner 的独立入口，负责消费 catalog.video.* 事件
// 并维护 profile.videos_projection 投影表。
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	configloader "github.com/bionicotaku/lingo-services-profile/internal/infrastructure/configloader"
	"github.com/go-kratos/kratos/v2/log"
)

type catalogInboxApp struct {
	Task   runner
	Logger log.Logger
}

type runner interface {
	Run(ctx context.Context) error
}

func main() {
	ctx := context.Background()

	confFlag := flag.String("conf", "", "config path or directory, eg: -conf configs/config.yaml")
	flag.Parse()

	params := configloader.Params{ConfPath: *confFlag}
	app, cleanup, err := wireCatalogInboxTask(ctx, params)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	logger := app.Logger
	if logger == nil {
		logger = log.NewStdLogger(os.Stdout)
	}
	helper := log.NewHelper(logger)

	if app.Task == nil {
		helper.Warn("catalog inbox runner disabled (missing messaging.pubsub configuration)")
		return
	}

	helper.Info("starting catalog inbox task")

	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Task.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		helper.Errorf("catalog inbox runner stopped unexpectedly: %v", err)
		os.Exit(1)
	}

	helper.Info("catalog inbox task stopped")
}
