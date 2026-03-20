package engine

import (
	"errors"
	"sync"
	"time"
)

var ErrRedisBreakerOpen = errors.New("redis breaker is open")

type circuitBreaker struct {
	mu sync.Mutex

	failureThreshold int
	openFor          time.Duration
	consecutiveFails int
	openUntil        time.Time
}

func newCircuitBreaker(failureThreshold int, openFor time.Duration) *circuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if openFor <= 0 {
		openFor = 500 * time.Millisecond
	}
	return &circuitBreaker{
		failureThreshold: failureThreshold,
		openFor:          openFor,
	}
}

func (b *circuitBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() {
		return true
	}
	if time.Now().After(b.openUntil) {
		b.openUntil = time.Time{}
		b.consecutiveFails = 0
		return true
	}
	return false
}

func (b *circuitBreaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails = 0
	b.openUntil = time.Time{}
}

func (b *circuitBreaker) fail() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails++
	if b.consecutiveFails >= b.failureThreshold {
		b.openUntil = time.Now().Add(b.openFor)
	}
}

func (b *circuitBreaker) isOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.openUntil.IsZero() && time.Now().Before(b.openUntil)
}
