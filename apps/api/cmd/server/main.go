package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/Abhi1264/auctioneer/apps/api/internal/config"
	eng "github.com/Abhi1264/auctioneer/apps/api/internal/engine"
	"github.com/Abhi1264/auctioneer/apps/api/internal/obs"
	grpcapi "github.com/Abhi1264/auctioneer/apps/api/internal/transport/grpc"
	wsapi "github.com/Abhi1264/auctioneer/apps/api/internal/transport/ws"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	debug.SetGCPercent(cfg.GOGC)
	if cfg.GoMemLimitBytes > 0 {
		debug.SetMemoryLimit(cfg.GoMemLimitBytes)
	}

	reg := prometheus.NewRegistry()
	metrics := obs.NewMetrics(reg)

	router, err := eng.NewRedisRouter(cfg.RedisAddresses, cfg.RedisPassword, cfg.RedisDB, cfg.RedisPoolSize)
	if err != nil {
		logger.Error("failed to initialize redis router", "err", err)
		os.Exit(1)
	}
	defer router.Close()

	store := eng.NewRedisStore(router, cfg)
	service := eng.NewService(store, cfg.DefaultAuctionDuration)
	hub := wsapi.NewHub(cfg.MaxWSQueueDepth, metrics.WsDroppedClients)
	go hub.Run()

	dispatcher := eng.NewEventDispatcher(store, hub, logger, metrics, cfg.StreamReadCount)
	go dispatcher.Run(context.Background())

	grpcSrv, lis, err := grpcapi.NewServer(cfg.GRPCAddress, service, cfg.MaxInFlightBids, metrics, store.BreakerOpen)
	if err != nil {
		logger.Error("failed to initialize gRPC server", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", wsapi.NewHandler(hub, logger))
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if err := router.ForAuction("health").Ping(ctx).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"redis_unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	errCh := make(chan error, 2)

	go func() {
		logger.Info("starting gRPC server", "addr", cfg.GRPCAddress)
		errCh <- grpcSrv.Serve(lis)
	}()
	go func() {
		logger.Info("starting HTTP server", "addr", cfg.HTTPAddress)
		errCh <- httpSrv.ListenAndServe()
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	case serveErr := <-errCh:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("server terminated", "err", serveErr)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer cancel()
	grpcSrv.GracefulStop()
	_ = httpSrv.Shutdown(shutdownCtx)
	logger.Info("server shutdown complete")
}
