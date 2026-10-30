package concurrent

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

type Weighted interface {
	Acquire(ctx context.Context, n int64) error
	Release(n int64)
}

type ErrGroup struct {
	Context  context.Context
	Weighted Weighted

	mu        sync.Mutex
	pending   []func() error
	goGroup   *errgroup.Group
	goContext context.Context
}

func NewErrGroup() *ErrGroup {
	return &ErrGroup{}
}

// Add queues functions for execution when Wait is called.
func (e *ErrGroup) Add(f ...func() error) {
	e.mu.Lock()
	e.pending = append(e.pending, f...)
	e.mu.Unlock()
}

// Go starts functions immediately and waits for them in Wait.
func (e *ErrGroup) Go(f ...func() error) {
	e.mu.Lock()
	e.goInit()

	for _, task := range f {
		if task == nil {
			continue
		}

		e.goGroup.Go(func() error {
			return e.run(e.goContext, task)
		})
	}

	e.mu.Unlock()
}

// Len returns the number of pending functions.
func (e *ErrGroup) Len() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.pending)
}

// Wait for the function to finish executing called Add and Go.
func (e *ErrGroup) Wait() error {
	e.mu.Lock()
	e.goInit()

	pending := e.pending
	e.pending = nil

	for _, task := range pending {
		if task == nil {
			continue
		}

		e.goGroup.Go(func() error {
			return e.run(e.goContext, task)
		})
	}

	eg := e.goGroup
	e.mu.Unlock()

	err := eg.Wait()

	e.mu.Lock()
	e.goGroup = nil
	e.goContext = nil
	e.mu.Unlock()

	return err
}

func (e *ErrGroup) goInit() {
	if e.goGroup != nil {
		return
	}

	ctx := e.Context
	if ctx == nil {
		ctx = context.Background()
	}

	e.goGroup, e.goContext = errgroup.WithContext(ctx)
}

func (e *ErrGroup) run(ctx context.Context, f func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if e.Weighted != nil {
		if err := e.Weighted.Acquire(ctx, 1); err != nil {
			return err
		}
		defer e.Weighted.Release(1)
	}

	return f()
}
