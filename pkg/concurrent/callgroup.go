package concurrent

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrRecursiveCallGroupCall     = errors.New("concurrent: recursive CallGroup call")
	ErrContextHasNoCallGroupToken = errors.New("concurrent: context has no CallGroup token")
	ErrCallGroupCallbackPanic     = errors.New("concurrent: CallGroup callback panic")
)

// CallToken identifies one logical CallGroup call chain.
type CallToken struct {
	marker uint8
}

// NewCallToken creates a token for one logical CallGroup call chain.
func NewCallToken() *CallToken {
	return &CallToken{}
}

// WithCallGroupToken starts a logical CallGroup call chain in ctx.
func WithCallGroupToken(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, callTokenKey{}, NewCallToken())
}

type identityEntry[T any] struct {
	done  chan struct{}
	mu    sync.Mutex
	owner *CallToken
	value T
	err   error
	ready bool
}

type callTokenKey struct{}

func callGroupTokenFromContext(ctx context.Context) (*CallToken, bool) {
	if ctx == nil {
		return nil, false
	}
	token, ok := ctx.Value(callTokenKey{}).(*CallToken)
	return token, ok && token != nil
}

// CallGroup shares completed and in-flight results by key.
// Zero value is ready to use.
type CallGroup[T any] struct {
	mu      sync.RWMutex
	entries map[string]*identityEntry[T]
}

// NewCallGroup creates a new CallGroup.
func NewCallGroup[T any]() *CallGroup[T] {
	return &CallGroup[T]{
		entries: make(map[string]*identityEntry[T]),
	}
}

// Do shares the result for key using a background context.
func (s *CallGroup[T]) Do(key string, fn func() (T, error)) (T, error) {
	return s.DoContextFunc(context.Background(), key, func(context.Context) (T, error) {
		return fn()
	})
}

// DoContext shares the result for key and uses the context only for call-chain identity.
func (s *CallGroup[T]) DoContext(ctx context.Context, key string, fn func() (T, error)) (T, error) {
	return s.DoContextFunc(ctx, key, func(context.Context) (T, error) {
		return fn()
	})
}

// DoContextFunc passes the context to callbacks that need cancellation or nested context propagation.
func (s *CallGroup[T]) DoContextFunc(ctx context.Context, key string, fn func(context.Context) (T, error)) (T, error) {
	token, ok := callGroupTokenFromContext(ctx)
	if !ok {
		var zero T
		return zero, ErrContextHasNoCallGroupToken
	}

	return s.doIdentityResult(token, key, func() (T, error) {
		return fn(ctx)
	})
}

func (s *CallGroup[T]) entry(key string) *identityEntry[T] {
	s.mu.RLock()
	entry := s.entries[key]
	s.mu.RUnlock()
	if entry != nil {
		return entry
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]*identityEntry[T])
	}
	if entry = s.entries[key]; entry == nil {
		entry = &identityEntry[T]{done: make(chan struct{})}
		s.entries[key] = entry
	}
	return entry
}

func (s *CallGroup[T]) doIdentityResult(token *CallToken, key string, fn func() (T, error)) (T, error) {
	entry := s.entry(key)

	entry.mu.Lock()
	if entry.ready {
		value, err := entry.value, entry.err
		entry.mu.Unlock()
		return value, err
	}
	if entry.owner == token {
		var zero T
		entry.mu.Unlock()
		return zero, ErrRecursiveCallGroupCall
	}
	if entry.owner != nil {
		done := entry.done
		entry.mu.Unlock()
		<-done
		entry.mu.Lock()
		value, err := entry.value, entry.err
		entry.mu.Unlock()
		return value, err
	}

	entry.owner = token
	entry.mu.Unlock()
	return entry.completeIdentityEntry(fn)
}

func (entry *identityEntry[T]) completeIdentityEntry(fn func() (T, error)) (value T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = ErrCallGroupCallbackPanic
			entry.mu.Lock()
			entry.value, entry.err, entry.ready = value, err, true
			close(entry.done)
			entry.mu.Unlock()
			panic(recovered)
		}

		entry.mu.Lock()
		entry.value, entry.err, entry.ready = value, err, true
		close(entry.done)
		entry.mu.Unlock()
	}()

	return fn()
}
