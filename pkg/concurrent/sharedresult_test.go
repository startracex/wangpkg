package concurrent

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSharedResult_ZeroValue(t *testing.T) {
	var sr SharedResult[int]

	v, err := sr.Do("key1", func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42 {
		t.Fatalf("expected 42, got %v", v)
	}
}

func TestSharedResult_SingleExecution(t *testing.T) {
	var sr SharedResult[int]
	var callCount int32
	var wg sync.WaitGroup

	for range 10 {
		wg.Go(func() {
			sr.Do("concurrent", func() (int, error) {
				atomic.AddInt32(&callCount, 1)
				return 0, nil
			})
		})
	}

	wg.Wait()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("fn should be called once, got %d", callCount)
	}
}

func TestSharedResult_ErrorPropagation(t *testing.T) {
	var sr SharedResult[int]

	fn := func() (int, error) {
		return 0, errors.New("fail")
	}

	v, err := sr.Do("errkey", fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if v != 0 {
		t.Fatalf("expected zero value, got %v", v)
	}

	called := false
	v, err = sr.Do("errkey", func() (int, error) {
		called = true
		return 99, nil
	})
	if called {
		t.Fatal("fn should not be called again after previous failure")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if v != 0 {
		t.Fatalf("expected zero value, got %v", v)
	}
}

func TestSharedResult_MultipleKeys(t *testing.T) {
	var sr SharedResult[string]

	var wg sync.WaitGroup
	keys := []string{"k1", "k2", "k3"}
	values := []string{"v1", "v2", "v3"}

	for i, key := range keys {
		wg.Add(1)
		go func(key, value string) {
			defer wg.Done()
			v, err := sr.Do(key, func() (string, error) {
				return value, nil
			})
			if err != nil {
				t.Errorf("unexpected error for key %s: %v", key, err)
			}
			if v != value {
				t.Errorf("unexpected value for key %s: got %v", key, v)
			}
		}(key, values[i])
	}

	wg.Wait()
}

func TestSharedResult_SequentialCalls(t *testing.T) {
	var sr SharedResult[int]

	count := 0
	fn := func() (int, error) {
		count++
		return 123, nil
	}

	v1, err := sr.Do("seq", fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v1 != 123 {
		t.Fatalf("expected 123, got %v", v1)
	}

	v2, err := sr.Do("seq", func() (int, error) {
		count++
		return 999, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v2 != 123 {
		t.Fatalf("expected 123 (cached), got %v", v2)
	}

	if count != 1 {
		t.Fatalf("expected fn to be called once (cached), got %d", count)
	}
}

func TestSharedResult_RecursiveCalls(t *testing.T) {
	var sr SharedResult[int]
	var callCount atomic.Int32

	var recursiveFn func() (int, error)
	recursiveFn = func() (int, error) {
		count := callCount.Add(1)

		if count == 1 {
			v, err := sr.Do("recursive", recursiveFn)
			if err != nil {
				return 0, err
			}
			return v + 1, nil
		}

		return int(count), nil
	}

	v, err := sr.Do("recursive", recursiveFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v != 3 {
		t.Fatalf("expected 3, got %v", v)
	}
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 calls (first + one direct recursive execution), got %d", callCount.Load())
	}
}
