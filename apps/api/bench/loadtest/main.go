package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	auctionv1 "github.com/Abhi1264/auctioneer/apps/api/auction/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	var (
		grpcAddr     = flag.String("grpc_addr", "127.0.0.1:8081", "gRPC server address")
		auctionID    = flag.String("auction_id", "load-auction", "auction ID")
		requests     = flag.Int("requests", 1_000_000, "number of total bid requests")
		concurrency  = flag.Int("concurrency", 512, "number of worker goroutines")
		rpcTimeoutMs = flag.Int("rpc_timeout_ms", 500, "per-request gRPC timeout in milliseconds")
	)
	flag.Parse()

	conn, err := grpc.Dial(
		*grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := auctionv1.NewAuctionServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := client.CreateAuction(ctx, &auctionv1.CreateAuctionRequest{
		AuctionId:         *auctionID,
		OpeningPriceCents: 100,
		DurationSec:       3600,
	}); err != nil {
		log.Printf("create auction warning: %v", err)
	}

	start := time.Now()
	var okCount int64
	var errCount int64
	errorByCode := map[string]int64{}
	errorByMessage := map[string]int64{}
	var errorMu sync.Mutex
	jobs := make(chan int, *concurrency*2)
	var wg sync.WaitGroup

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for n := range jobs {
				req := &auctionv1.PlaceBidRequest{
					AuctionId:   *auctionID,
					BidId:       fmt.Sprintf("load-%d-%d", workerID, n),
					UserId:      fmt.Sprintf("u-%d", workerID),
					AmountCents: int64(100 + n),
					ClientTsMs:  time.Now().UnixMilli(),
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*rpcTimeoutMs)*time.Millisecond)
				_, err := client.PlaceBid(ctx, req)
				cancel()
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					errorCode := status.Code(err).String()
					errorMessage := status.Convert(err).Message()
					errorMu.Lock()
					errorByCode[errorCode]++
					errorByMessage[errorMessage]++
					errorMu.Unlock()
					continue
				}
				atomic.AddInt64(&okCount, 1)
			}
		}(i)
	}

	for i := 0; i < *requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	elapsed := time.Since(start)
	throughput := float64(okCount) / elapsed.Seconds()
	log.Printf("load test finished: ok=%d err=%d elapsed=%s throughput=%.2f req/s", okCount, errCount, elapsed, throughput)
	if errCount > 0 {
		type kv struct {
			code  string
			count int64
		}
		codes := make([]kv, 0, len(errorByCode))
		for code, count := range errorByCode {
			codes = append(codes, kv{code: code, count: count})
		}
		sort.Slice(codes, func(i, j int) bool { return codes[i].count > codes[j].count })
		for _, item := range codes {
			log.Printf("error breakdown: code=%s count=%d", item.code, item.count)
		}

		messages := make([]kv, 0, len(errorByMessage))
		for message, count := range errorByMessage {
			messages = append(messages, kv{code: message, count: count})
		}
		sort.Slice(messages, func(i, j int) bool { return messages[i].count > messages[j].count })
		limit := 5
		if len(messages) < limit {
			limit = len(messages)
		}
		for i := 0; i < limit; i++ {
			log.Printf("error message: message=%q count=%d", messages[i].code, messages[i].count)
		}
	}
}
