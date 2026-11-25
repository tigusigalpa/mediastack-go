// Filters demonstrates include/exclude filters, keyword search, sorting
// and the only_with_image flag.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/filters
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

	withImage := true
	resp, err := client.News.List(ctx, &mediastack.NewsParams{
		// business and technology news, but nothing sports-related
		Categories: mediastack.Filter[mediastack.Category]{
			Include: []mediastack.Category{mediastack.CategoryBusiness, mediastack.CategoryTechnology},
			Exclude: []mediastack.Category{mediastack.CategorySports},
		},
		// US and UK sources, excluding German ones
		Countries: mediastack.Filter[mediastack.Country]{
			Include: []mediastack.Country{mediastack.CountryUS, mediastack.CountryGB},
			Exclude: []mediastack.Country{mediastack.CountryDE},
		},
		// must mention "markets", must not mention "crypto"
		Keywords: mediastack.Filter[string]{
			Include: []string{"markets"},
			Exclude: []string{"crypto"},
		},
		OnlyWithImage: &withImage,
		Sort:          mediastack.SortPopularity,
		Limit:         5,
	})
	if err != nil {
		return err
	}
	for _, a := range resp.Data {
		fmt.Printf("- %s\n  image: %s\n", a, a.Image)
	}
	return nil
}
