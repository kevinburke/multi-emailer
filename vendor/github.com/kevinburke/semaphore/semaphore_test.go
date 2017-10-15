package semaphore

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLen(t *testing.T) {
	s := New(10)
	defer s.Drain()
	if l := s.Len(); l != 10 {
		t.Errorf("wrong Len: %d", l)
	}
}

func TestAcquireContextBusy(t *testing.T) {
	s := New(1)
	defer s.Drain()
	s.Acquire()
	// ugh this is a mess haha
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		ok := s.AcquireContext(ctx)
		if ok {
			t.Errorf("should not have acquired a resource, but did")
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestAcquireContextNotBusy(t *testing.T) {
	s := New(2)
	defer s.Drain()
	s.Acquire()
	// ugh this is a mess haha
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		ok := s.AcquireContext(ctx)
		if !ok {
			t.Errorf("should have acquired a resource, but did not")
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestAcquireContextBecomesAvailableBeforeTimeout(t *testing.T) {
	t.Skip("this test is racy, ugh")
	s := New(1)
	defer s.Drain()
	s.Acquire()
	go func() {
		time.Sleep(5 * time.Millisecond)
		s.Release()
	}()
	ok := s.AcquireContext(context.Background())
	if !ok {
		t.Errorf("should have acquired a resource, but did not")
	}
}

func SimultaneousAcquire(t *testing.T) {
	s := New(2)
	defer s.Drain()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			s.Acquire()
			time.Sleep(50 * time.Nanosecond)
			s.Release()
			wg.Done()
		}(i)
	}
	if avail := s.Available(); avail != 2 {
		t.Errorf("expected 2 available, got %d", avail)
	}
	wg.Wait()
}
