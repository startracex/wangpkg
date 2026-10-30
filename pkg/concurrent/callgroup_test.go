package concurrent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCallGroupRecursiveCall(t *testing.T) {
	group := NewCallGroup[int]()
	ctx := WithCallGroupToken(context.Background())

	value, err := group.DoContext(ctx, "key", func() (int, error) {
		_, err := group.DoContext(ctx, "key", func() (int, error) {
			return 99, nil
		})
		if !errors.Is(err, ErrRecursiveCallGroupCall) {
			return 0, err
		}
		return 42, nil
	})
	if err != nil || value != 42 {
		t.Fatalf("DoWithContext() = (%d, %v), want (42, nil)", value, err)
	}
}

func TestCallGroupContextCallback(t *testing.T) {
	group := NewCallGroup[int]()
	ctx := WithCallGroupToken(context.Background())
	value, err := group.DoContextFunc(ctx, "key", func(got context.Context) (int, error) {
		if got != ctx {
			t.Fatal("callback received a different context")
		}
		return 7, nil
	})
	if err != nil || value != 7 {
		t.Fatalf("DoWithContextFn() = (%d, %v), want (7, nil)", value, err)
	}
}

func TestCallGroupRequiresToken(t *testing.T) {
	group := NewCallGroup[int]()
	_, err := group.DoContext(context.Background(), "key", func() (int, error) {
		return 1, nil
	})
	if !errors.Is(err, ErrRecursiveCallGroupCall) {
		t.Fatalf("error = %v, want ErrRecursiveCall", err)
	}
}

func TestCallGroupConcurrentWaiters(t *testing.T) {
	group := NewCallGroup[int]()
	var calls atomic.Int32
	start := make(chan struct{})
	var wg sync.WaitGroup

	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := WithCallGroupToken(context.Background())
			value, err := group.DoContext(ctx, "key", func() (int, error) {
				calls.Add(1)
				<-start
				return 7, nil
			})
			if err != nil || value != 7 {
				t.Errorf("DoWithContext() = (%d, %v), want (7, nil)", value, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("callback calls = %d, want 1", calls.Load())
	}
}

func TestCallGroupCachesResultAndError(t *testing.T) {
	group := NewCallGroup[int]()
	ctx := WithCallGroupToken(context.Background())
	var calls atomic.Int32
	wantErr := errors.New("failure")

	for range 2 {
		value, err := group.DoContext(ctx, "error", func() (int, error) {
			calls.Add(1)
			return 0, wantErr
		})
		if value != 0 || !errors.Is(err, wantErr) {
			t.Fatalf("result = (%d, %v), want (0, failure)", value, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("callback calls = %d, want 1", calls.Load())
	}
}

func TestCallGroupPanicReleasesWaiters(t *testing.T) {
	group := NewCallGroup[int]()
	started := make(chan struct{})
	release := make(chan struct{})
	ownerDone := make(chan struct{})
	waiterDone := make(chan error, 1)

	go func() {
		defer close(ownerDone)
		defer func() { _ = recover() }()
		ctx := WithCallGroupToken(context.Background())
		_, _ = group.DoContext(ctx, "panic", func() (int, error) {
			close(started)
			<-release
			panic("boom")
		})
	}()
	<-started
	go func() {
		ctx := WithCallGroupToken(context.Background())
		_, err := group.DoContext(ctx, "panic", func() (int, error) {
			return 1, nil
		})
		waiterDone <- err
	}()
	close(release)
	<-ownerDone

	if err := <-waiterDone; err == nil || err.Error() != "concurrent: CallGroup callback panic: boom" {
		t.Fatalf("waiter panic result error = %v", err)
	}
}
