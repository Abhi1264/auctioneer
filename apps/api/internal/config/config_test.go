package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsMalformedEnv(t *testing.T) {
	t.Setenv("REDIS_POOL_SIZE", "not-a-number")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "REDIS_POOL_SIZE") {
		t.Fatalf("got %v", err)
	}
}

func TestRedisAddrFromEnv(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_ADDRS", "  10.0.0.1:6379, 10.0.0.2:6379")
	if got := RedisAddrFromEnv(); got != "10.0.0.1:6379" {
		t.Fatalf("got %q", got)
	}

	t.Setenv("REDIS_ADDR", "127.0.0.1:6380")
	if got := RedisAddrFromEnv(); got != "127.0.0.1:6380" {
		t.Fatalf("REDIS_ADDR should win, got %q", got)
	}
}
