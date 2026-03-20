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
	cfg := Config{
		GRPCAddress:             getEnv("AUCTION_GRPC_ADDR", ":8081"),
		HTTPAddress:             getEnv("AUCTION_HTTP_ADDR", ":8080"),
		RedisAddresses:          strings.Split(getEnv("REDIS_ADDRS", "127.0.0.1:6379"), ","),
		RedisPassword:           getEnv("REDIS_PASSWORD", ""),
		RedisDB:                 getEnvInt("REDIS_DB", 0),
		RedisPoolSize:           getEnvInt("REDIS_POOL_SIZE", 256),
		BidIdempotencyTTL:       getEnvDuration("BID_IDEMPOTENCY_TTL", 10*time.Minute),
		DefaultAuctionDuration:  getEnvDuration("DEFAULT_AUCTION_DURATION", 10*time.Minute),
		StreamReadCount:         int64(getEnvInt("STREAM_READ_COUNT", 256)),
		MaxInFlightBids:         getEnvInt("MAX_INFLIGHT_BIDS", 100000),
		MaxWSQueueDepth:         getEnvInt("MAX_WS_QUEUE_DEPTH", 2048),
		BreakerFailureThreshold: getEnvInt("BREAKER_FAILURE_THRESHOLD", 20),
		BreakerOpenFor:          getEnvDuration("BREAKER_OPEN_FOR", 500*time.Millisecond),
		LogLevel:                parseLevel(getEnv("LOG_LEVEL", "info")),
		GracefulShutdownTimeout: getEnvDuration("GRACEFUL_SHUTDOWN_TIMEOUT", 10*time.Second),
		GOGC:                    getEnvInt("GOGC", 100),
		GoMemLimitBytes:         getEnvInt64("GOMEMLIMIT_BYTES", 0),
	}

	if len(cfg.RedisAddresses) == 0 || strings.TrimSpace(cfg.RedisAddresses[0]) == "" {
		return Config{}, fmt.Errorf("redis address is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getEnvInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
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
