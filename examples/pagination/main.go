// Pagination shows all three ways to walk a result set: the lazy
// iterator, Collect, and a manual offset loop.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/pagination
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Lazy iterator: fetches pages only as you consume them.
	fmt.Println("== Iter ==")
	count := 0
	for article, err := range client.News.Iter(ctx, &mediastack.NewsParams{Limit: 25}) {
		if err != nil {
			return err
		}
		if count == 0 {
			fmt.Println("first:", article.Title)
		}
		count++
		if count >= 60 {
			break // stop early; the iterator cleans up
		}
	}
	fmt.Println("iterated:", count)

	// 2. Collect: a bounded number of items in one call.
	fmt.Println("== Collect ==")
	articles, err := client.News.Collect(ctx, &mediastack.NewsParams{
		Categories: mediastack.Filter[mediastack.Category]{
			Include: []mediastack.Category{mediastack.CategoryTechnology},
		},
	}, 30)
	if err != nil {
		return err
	}
	fmt.Println("collected:", len(articles))

	// 3. Manual loop: full control over offsets.
	fmt.Println("== Manual ==")
	params := &mediastack.NewsParams{Limit: 25}
	for {
		resp, err := client.News.List(ctx, params)
		if err != nil {
			return err
		}
		for _, a := range resp.Data {
			_ = a // process articles
		}
		fmt.Printf("page %d: %d items, %d remaining\n",
			resp.Pagination.Page(), resp.Pagination.Count, resp.Pagination.Remaining())
		if !resp.Pagination.HasMore() {
			break
		}
		params.Offset = resp.Pagination.NextOffset()
	}
	return nil
}
