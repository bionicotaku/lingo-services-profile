package services_test

import (
	"context"
	"time"

	"github.com/bionicotaku/lingo-utils/txmanager"
	"github.com/jackc/pgx/v5"
)

type fakeTxManager struct{}

type fakeSession struct{ ctx context.Context }

func (fakeTxManager) WithinTx(ctx context.Context, _ txmanager.TxOptions, fn func(context.Context, txmanager.Session) error) error {
	return fn(ctx, fakeSession{ctx: ctx})
}

func (fakeTxManager) WithinReadOnlyTx(ctx context.Context, _ txmanager.TxOptions, fn func(context.Context, txmanager.Session) error) error {
	return fn(ctx, fakeSession{ctx: ctx})
}

func (fakeSession) Tx() pgx.Tx { return nil }

func (s fakeSession) Context() context.Context { return s.ctx }

func ptrString(v string) *string { return &v }

func ptrInt64(v int64) *int64 { return &v }

func ptrTime(t time.Time) *time.Time { return &t }
