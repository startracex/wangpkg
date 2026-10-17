package concurrent

import (
	"sync"

	"github.com/petermattis/goid"
)

type sharedEntry[T any] struct {
	doneSignal chan struct{}
	err        error
	mu         sync.Mutex
	result     T
	ownerGID   int64
	depth      int
	done       bool
}

type SharedResult[T any] struct {
	Map[string, *sharedEntry[T]]
}

func (sr *SharedResult[T]) Do(key string, fn func() (T, error)) (T, error) {
	e := sr.Take(key, func(oldValue *sharedEntry[T], hasOldValue bool) (newValue *sharedEntry[T], shouldDelete bool) {
		if hasOldValue {
			return oldValue, false
		}
		return &sharedEntry[T]{doneSignal: make(chan struct{})}, false
	})

	e.mu.Lock()
	if e.done {
		res, err := e.result, e.err
		e.mu.Unlock()
		return res, err
	}

	gid := goid.Get()
	if e.ownerGID == gid {
		e.depth++
		e.mu.Unlock()
		return fn()
	}

	if e.ownerGID != 0 {
		doneSignal := e.doneSignal
		e.mu.Unlock()
		<-doneSignal
		e.mu.Lock()
		res, err := e.result, e.err
		e.mu.Unlock()
		return res, err
	}

	e.ownerGID = gid
	e.depth = 1
	e.mu.Unlock()

	res, err := fn()
	e.mu.Lock()
	e.result = res
	e.err = err
	e.depth--
	if e.depth == 0 {
		e.done = true
		e.ownerGID = 0
		close(e.doneSignal)
	}
	e.mu.Unlock()

	return res, err
}
