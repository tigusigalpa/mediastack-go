// Sources demonstrates source discovery: search terms, country and
// language filters, and iterating a multi-page result set. The source
// IDs printed here are the values to feed into NewsParams.Sources.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/sources
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

	// The API requires a search term; the SDK fails fast without one.
	resp, err := client.Sources.Search(ctx, "news", &mediastack.SourceParams{
		Countries: mediastack.Filter[mediastack.Country]{
			Include: []mediastack.Country{mediastack.CountryUS},
		},
		Limit: 10,
	})
	if err != nil {
		return err
	}
	fmt.Printf("%d matching sources\n\n", resp.Pagination.Total)
	for _, s := range resp.Data {
		fmt.Printf("%-20s %-15s %-5s %s\n", s.ID, s.Category, s.Country, s.Name)
	}

	// Iterate every matching source across pages.
	fmt.Println("\n== iter ==")
	n := 0
	for src, err := range client.Sources.Iter(ctx, &mediastack.SourceParams{Search: "bbc", Limit: 10}) {
		if err != nil {
			return err
		}
		fmt.Println(src.ID, "-", src.Name)
		n++
	}
	fmt.Println("iterated:", n)
	return nil
}
