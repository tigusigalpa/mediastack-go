// Basic fetches the first three live news articles and prints them.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/basic
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	mediastack "github.com/tigusigalpa/mediastack-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	client, err := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Limit 3: on the Free plan every request counts against a 100/month
	// quota, so keep the page small while experimenting.
	resp, err := client.News.List(ctx, &mediastack.NewsParams{Limit: 3})
	if err != nil {
		return err
	}

	fmt.Printf("page %d of %d (total: %d)\n\n",
		resp.Pagination.Page(), resp.Pagination.TotalPages(), resp.Pagination.Total)
	for _, a := range resp.Data {
		fmt.Printf("- %s\n  %s\n", a, a.URL)
	}
	return nil
}
