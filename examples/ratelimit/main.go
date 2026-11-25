// Ratelimit shows the optional client-side limiter, the APILayer quota
// headers and how to throttle a fleet of workers.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/ratelimit
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	mediastack "github.com/tigusigalpa/mediastack-go"
)

func main() {
	// 2 requests per second with a burst of 2, and an observer that
	// reports quota headers whenever the API sends them.
	client, err := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey),
		mediastack.WithRateLimit(2, 2),
		mediastack.WithRateLimitObserver(func(rl mediastack.RateLimit) {
			fmt.Printf("quota: %d/%d left today, %d/%d this month\n",
				rl.RemainingDay, rl.LimitDay, rl.RemainingMonth, rl.LimitMonth)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Six concurrent workers are paced by the shared limiter.
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			_, err := client.News.List(ctx, &mediastack.NewsParams{Limit: 1})
			if err != nil {
				fmt.Printf("worker %d: %v\n", worker, err)
				return
			}
			fmt.Printf("worker %d done at %s\n", worker, time.Now().Format("15:04:05.000"))
		}(i)
	}
	wg.Wait()

	// The client also exposes the latest quota snapshot directly.
	if rl := client.RateLimit(); rl.Present {
		fmt.Printf("latest snapshot: %d remaining this month\n", rl.RemainingMonth)
	}
}
