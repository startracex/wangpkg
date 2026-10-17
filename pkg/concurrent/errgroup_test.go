package concurrent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingWeighted struct {
	mu       sync.Mutex
	failWith error
	acquired int64
	released int64
}

func (c *countingWeighted) Acquire(context.Context, int64) error {
	if c.failWith != nil {
		return c.failWith
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acquired++
	return nil
}

func (c *countingWeighted) Release(int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.released++
}

func (c *countingWeighted) counts() (int64, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.acquired, c.released
}

func TestErrGroupEmpty(t *testing.T) {
	e := NewErrGroup()
	if e.Len() != 0 {
		t.Errorf("Len() = %d, want 0", e.Len())
	}
	if err := e.Wait(); err != nil {
		t.Errorf("Wait() on an empty group = %v, want nil", err)
	}
	if e.Len() != 0 {
		t.Errorf("Len() after Wait = %d, want 0", e.Len())
	}
}

func TestErrGroupAddAndLen(t *testing.T) {
	e := NewErrGroup()
	e.Add(func() error { return nil })
	e.Add(func() error { return nil }, func() error { return nil })

	if e.Len() != 3 {
		t.Errorf("Len() = %d, want 3", e.Len())
	}
}

func TestErrGroupRunsSingleTask(t *testing.T) {
	e := NewErrGroup()
	called := false
	e.Add(func() error {
		called = true
		return nil
	})
	if err := e.Wait(); err != nil {
		t.Fatalf("Wait() = %v", err)
	}
	if !called {
		t.Error("the task was not executed")
	}
}

func TestErrGroupRunsMultipleTasks(t *testing.T) {
	e := NewErrGroup()
	var counter atomic.Int64
	const tasks = 32
	for range tasks {
		e.Add(func() error {
			counter.Add(1)
			return nil
		})
	}
	if err := e.Wait(); err != nil {
		t.Fatalf("Wait() = %v", err)
	}
	if got := counter.Load(); got != tasks {
		t.Errorf("executed %d tasks, want %d", got, tasks)
	}
}

func TestErrGroupWaitClearsPending(t *testing.T) {
	e := NewErrGroup()
	var counter atomic.Int64
	e.Add(func() error { counter.Add(1); return nil })

	if err := e.Wait(); err != nil {
		t.Fatalf("first Wait() = %v", err)
	}
	if err := e.Wait(); err != nil {
		t.Fatalf("second Wait() = %v", err)
	}
	if got := counter.Load(); got != 1 {
		t.Errorf("task executed %d times, want 1", got)
	}
}

func TestErrGroupSingleTaskError(t *testing.T) {
	want := errors.New("boom")
	e := NewErrGroup()
	e.Add(func() error { return want })

	if err := e.Wait(); !errors.Is(err, want) {
		t.Errorf("Wait() = %v, want %v", err, want)
	}
}

func TestErrGroupMultipleTasksErrorPropagation(t *testing.T) {
	want := errors.New("boom")
	e := NewErrGroup()
	e.Add(func() error { return nil })
	e.Add(func() error { return want })

	if err := e.Wait(); !errors.Is(err, want) {
		t.Errorf("Wait() = %v, want %v", err, want)
	}
}

func TestErrGroupNilTasksAreSkipped(t *testing.T) {
	e := NewErrGroup()
	e.Add(nil)
	if err := e.Wait(); err != nil {
		t.Errorf("Wait() with a single nil task = %v, want nil", err)
	}

	e = NewErrGroup()
	e.Add(nil, func() error { return errors.New("not nil") }, nil)
	if err := e.Wait(); err == nil {
		t.Error("Wait() should still surface the error from the non-nil task")
	}
}

func TestErrGroupRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e := NewErrGroup()
	e.Context = ctx
	called := false
	e.Add(func() error {
		called = true
		return nil
	})

	if err := e.Wait(); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait() = %v, want %v", err, context.Canceled)
	}
	if called {
		t.Error("the task must not run when the context is already cancelled")
	}
}

func TestErrGroupWeightedIsBalanced(t *testing.T) {
	w := &countingWeighted{}
	e := NewErrGroup()
	e.Weighted = w
	for range 5 {
		e.Add(func() error { return nil })
	}
	if err := e.Wait(); err != nil {
		t.Fatalf("Wait() = %v", err)
	}

	acquired, released := w.counts()
	if acquired != 5 || released != 5 {
		t.Errorf("Acquire/Release = %d/%d, want 5/5", acquired, released)
	}
}

func TestErrGroupWeightedAcquireError(t *testing.T) {
	want := errors.New("no capacity")
	e := NewErrGroup()
	e.Weighted = &countingWeighted{failWith: want}
	called := false
	e.Add(func() error {
		called = true
		return nil
	})

	if err := e.Wait(); !errors.Is(err, want) {
		t.Errorf("Wait() = %v, want %v", err, want)
	}
	if called {
		t.Error("the task must not run when Acquire fails")
	}
}

func TestErrGroupConcurrentExecution(t *testing.T) {
	const tasks = 8

	e := NewErrGroup()
	var counter atomic.Int64
	started := make(chan struct{}, tasks)
	release := make(chan struct{})

	for range tasks {
		e.Add(func() error {
			started <- struct{}{}
			<-release
			counter.Add(1)
			return nil
		})
	}

	done := make(chan error, 1)
	go func() { done <- e.Wait() }()

	for i := range tasks {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of %d tasks started: they are not running concurrently", i, tasks)
		}
	}
	close(release)

	if err := <-done; err != nil {
		t.Fatalf("Wait() = %v", err)
	}
	if got := counter.Load(); got != tasks {
		t.Errorf("counter = %d, want %d", got, tasks)
	}
}
