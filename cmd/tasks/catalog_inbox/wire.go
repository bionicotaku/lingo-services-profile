//go:build wireinject
// +build wireinject

// Package main 为 catalog inbox 任务提供 Wire 依赖注入定义。
package main

import (
	"context"
	"fmt"

	configloader "github.com/bionicotaku/lingo-services-profile/internal/infrastructure/configloader"
	"github.com/bionicotaku/lingo-services-profile/internal/repositories"
	cataloginbox "github.com/bionicotaku/lingo-services-profile/internal/tasks/catalog_inbox"

	"github.com/bionicotaku/lingo-utils/gclog"
	"github.com/bionicotaku/lingo-utils/gcpubsub"
	obswire "github.com/bionicotaku/lingo-utils/observability"
	"github.com/bionicotaku/lingo-utils/pgxpoolx"
	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

//go:generate go run github.com/google/wire/cmd/wire

var catalogInboxRepoSet = wire.NewSet(
	repositories.NewInboxRepository,
	repositories.NewProfileVideoProjectionRepository,
)

func wireCatalogInboxTask(context.Context, configloader.Params) (*catalogInboxApp, func(), error) {
	panic(wire.Build(
		configloader.ProviderSet,
		gclog.ProviderSet,
		obswire.ProviderSet,
		pgxpoolx.ProviderSet,
		txmanager.ProviderSet,
		gcpubsub.ProviderSet,
		catalogInboxRepoSet,
		cataloginbox.ProvideTask,
		newCatalogInboxApp,
	))
}

func newCatalogInboxApp(_ *obswire.Component, logger log.Logger, task *cataloginbox.Task) (*catalogInboxApp, error) {
	if task == nil {
		return &catalogInboxApp{Logger: logger}, nil
	}
	if logger == nil {
		return nil, fmt.Errorf("logger not initialized")
	}
	return &catalogInboxApp{
		Task:   task,
		Logger: logger,
	}, nil
}
