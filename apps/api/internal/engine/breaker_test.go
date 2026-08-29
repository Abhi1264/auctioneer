package engine

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	b := newCircuitBreaker(3, time.Second)
	for i := 0; i < 2; i++ {
		b.fail()
		if !b.allow() {
			t.Fatalf("breaker opened after %d failures", i+1)
		}
	}
	b.fail()
	if b.allow() {
		t.Fatal("expected breaker open after threshold")
	}
	if !b.isOpen() {
		t.Fatal("expected isOpen")
	}
}

func TestCircuitBreakerSuccessResets(t *testing.T) {
	b := newCircuitBreaker(2, time.Second)
	b.fail()
	b.success()
	b.fail()
	if !b.allow() {
		t.Fatal("success should reset failure count")
	}
}

func TestCircuitBreakerRecoversAfterOpenFor(t *testing.T) {
	b := newCircuitBreaker(1, 15*time.Millisecond)
	b.fail()
	if b.allow() {
		t.Fatal("expected open")
	}
	time.Sleep(25 * time.Millisecond)
	if !b.allow() {
		t.Fatal("expected recovery after open interval")
	}
}

func TestCircuitBreakerConcurrentAllowDuringOpen(t *testing.T) {
	b := newCircuitBreaker(1, time.Second)
	b.fail()
	const workers = 64
	var allowed int32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if b.allow() {
				atomic.AddInt32(&allowed, 1)
			}
		}()
	}
	wg.Wait()
	if allowed != 0 {
		t.Fatalf("open breaker admitted %d requests", allowed)
	}
}
