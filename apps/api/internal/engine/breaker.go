package engine

import (
	"errors"
	"sync/atomic"
	"time"
)

var ErrRedisBreakerOpen = errors.New("redis breaker is open")

type circuitBreaker struct {
	failureThreshold int32
	openForNs        int64
	consecutiveFails atomic.Int32
	openUntilNs      atomic.Int64
}

func newCircuitBreaker(failureThreshold int, openFor time.Duration) *circuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if openFor <= 0 {
		openFor = 500 * time.Millisecond
	}
	return &circuitBreaker{
		failureThreshold: int32(failureThreshold),
		openForNs:        openFor.Nanoseconds(),
	}
}

func (b *circuitBreaker) allow() bool {
	for {
		until := b.openUntilNs.Load()
		if until == 0 {
			return true
		}
		if time.Now().UnixNano() < until {
			return false
		}
		if b.openUntilNs.CompareAndSwap(until, 0) {
			b.consecutiveFails.Store(0)
			return true
		}
	}
}

func (b *circuitBreaker) success() {
	b.consecutiveFails.Store(0)
	b.openUntilNs.Store(0)
}

func (b *circuitBreaker) fail() {
	if b.consecutiveFails.Add(1) < b.failureThreshold {
		return
	}
	b.openUntilNs.Store(time.Now().UnixNano() + b.openForNs)
}

func (b *circuitBreaker) isOpen() bool {
	until := b.openUntilNs.Load()
	return until != 0 && time.Now().UnixNano() < until
}
