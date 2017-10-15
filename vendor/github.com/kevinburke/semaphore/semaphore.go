package semaphore

import (
	"context"
	"sync"
)

// Semaphore allows you to control the number of in-flight requests of a given
// service.
type Semaphore struct {
	n       int
	avail   int
	channel chan struct{}
	mu      sync.RWMutex
}

// Len returns the total number of workers in this semaphore.
func (s *Semaphore) Len() int {
	s.mu.RLock()
	n := s.n
	s.mu.RUnlock()
	return n
}

// New creates a new Semaphore with specified number of concurrent workers.
func New(n int) *Semaphore {
	if n < 1 {
		panic("Invalid number of permits. Less than 1")
	}

	// fill channel buffer
	channel := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		channel <- struct{}{}
	}

	return &Semaphore{
		n:       n,
		avail:   n,
		channel: channel,
	}
}

// Acquire blocks until a worker becomes available.
func (s *Semaphore) Acquire() {
	s.AcquireContext(context.Background())
}

// AcquireContext attempts to acquire a resource. AcquireContext returns false
// if we were unable to acquire a resource.
func (s *Semaphore) AcquireContext(ctx context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-s.channel:
		s.avail--
		return true
	}
}

// Release releases one worker.
func (s *Semaphore) Release() {
	if s.avail == s.n {
		panic("No workers available to release")
	}
	s.channel <- struct{}{}
	s.avail++
}

func (s *Semaphore) Drain() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.avail < s.n {
		s.Release()
	}
}

// Available gives number of unacquired resources.
func (s *Semaphore) Available() int {
	s.mu.RLock()
	avail := s.avail
	s.mu.RUnlock()
	return avail
}
