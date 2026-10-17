package concurrent

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Weighted interface {
	Acquire(ctx context.Context, n int64) error
	Release(n int64)
}

type ErrGroup struct {
	Context  context.Context
	Weighted Weighted
	Pending  []func() error
}

func NewErrGroup() *ErrGroup {
	return &ErrGroup{}
}

func (e *ErrGroup) Add(f ...func() error) {
	e.Pending = append(e.Pending, f...)
}

func (e *ErrGroup) Len() int {
	return len(e.Pending)
}

func (e *ErrGroup) Wait() error {
	pending := e.Pending
	e.Pending = pending[:0]
	ctx := e.Context
	if ctx == nil {
		ctx = context.Background()
	}

	switch len(pending) {
	case 0:
		return nil
	case 1:
		task := pending[0]
		if task == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return e.run(ctx, task)
	default:
		eg, egCtx := errgroup.WithContext(ctx)
		for _, task := range pending {
			if task == nil {
				continue
			}
			eg.Go(func() error {
				return e.run(egCtx, task)
			})
		}
		return eg.Wait()
	}
}

func (e *ErrGroup) run(ctx context.Context, f func() error) error {
	if e.Weighted != nil {
		if err := e.Weighted.Acquire(ctx, 1); err != nil {
			return err
		}
		defer func() {
			e.Weighted.Release(1)
		}()
	}
	return f()
}
