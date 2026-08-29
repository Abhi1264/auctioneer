package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GRPCAddress             string
	HTTPAddress             string
	RedisAddresses          []string
	RedisPassword           string
	RedisDB                 int
	RedisPoolSize           int
	BidIdempotencyTTL       time.Duration
	DefaultAuctionDuration  time.Duration
	StreamReadCount         int64
	MaxInFlightBids         int
	MaxWSQueueDepth         int
	BreakerFailureThreshold int
	BreakerOpenFor          time.Duration
	LogLevel                slog.Level
	GracefulShutdownTimeout time.Duration
	GOGC                    int
	GoMemLimitBytes         int64
}

func Load() (Config, error) {
	var err error
	intVal := func(key string, fallback int) int {
		v, e := getEnvInt(key, fallback)
		if err == nil {
			err = e
		}
		return v
	}
	durVal := func(key string, fallback time.Duration) time.Duration {
		v, e := getEnvDuration(key, fallback)
		if err == nil {
			err = e
		}
		return v
	}

	int64Val := func(key string, fallback int64) int64 {
		v, e := getEnvInt64(key, fallback)
		if err == nil {
			err = e
		}
		return v
	}

	cfg := Config{
		GRPCAddress:             getEnv("AUCTION_GRPC_ADDR", ":8081"),
		HTTPAddress:             getEnv("AUCTION_HTTP_ADDR", ":8080"),
		RedisAddresses:          splitAddrs(getEnv("REDIS_ADDRS", "127.0.0.1:6379")),
		RedisPassword:           getEnv("REDIS_PASSWORD", ""),
		RedisDB:                 intVal("REDIS_DB", 0),
		RedisPoolSize:           intVal("REDIS_POOL_SIZE", 256),
		BidIdempotencyTTL:       durVal("BID_IDEMPOTENCY_TTL", 10*time.Minute),
		DefaultAuctionDuration:  durVal("DEFAULT_AUCTION_DURATION", 10*time.Minute),
		StreamReadCount:         int64(intVal("STREAM_READ_COUNT", 256)),
		MaxInFlightBids:         intVal("MAX_INFLIGHT_BIDS", 100000),
		MaxWSQueueDepth:         intVal("MAX_WS_QUEUE_DEPTH", 2048),
		BreakerFailureThreshold: intVal("BREAKER_FAILURE_THRESHOLD", 20),
		BreakerOpenFor:          durVal("BREAKER_OPEN_FOR", 500*time.Millisecond),
		LogLevel:                parseLevel(getEnv("LOG_LEVEL", "info")),
		GracefulShutdownTimeout: durVal("GRACEFUL_SHUTDOWN_TIMEOUT", 10*time.Second),
		GOGC:                    intVal("GOGC", 100),
		GoMemLimitBytes:         int64Val("GOMEMLIMIT_BYTES", 0),
	}
	if err != nil {
		return Config{}, err
	}
	if len(cfg.RedisAddresses) == 0 {
		return Config{}, fmt.Errorf("redis address is required")
	}
	return cfg, nil
}

func RedisAddrFromEnv() string {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		return v
	}
	addrs := splitAddrs(os.Getenv("REDIS_ADDRS"))
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}

func splitAddrs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
