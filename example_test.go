package mediastack_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	mediastack "github.com/tigusigalpa/mediastack-go"
)

// Creating a client from the access key in the environment.
func ExampleNewClient() {
	client, err := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	if err != nil {
		// handle error
		_ = err
		return
	}
	defer client.Close()
	_ = client
}

// Fetching a single page of technology news.
func ExampleNewsService_List() {
	client, _ := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	ctx := context.Background()

	resp, err := client.News.List(ctx, &mediastack.NewsParams{
		Categories: mediastack.Filter[mediastack.Category]{
			Include: []mediastack.Category{mediastack.CategoryTechnology},
		},
		Countries: mediastack.Filter[mediastack.Country]{
			Include: []mediastack.Country{mediastack.CountryUS},
		},
		Limit: 10,
	})
	if err != nil {
		// handle error
		return
	}
	for _, a := range resp.Data {
		fmt.Println(a.Title, "—", a.Source)
	}
}

// Iterating every matching article without manual offset bookkeeping.
func ExampleNewsService_Iter() {
	client, _ := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	ctx := context.Background()

	for article, err := range client.News.Iter(ctx, &mediastack.NewsParams{
		Keywords: mediastack.Filter[string]{Include: []string{"space"}},
		Limit:    100,
	}) {
		if err != nil {
			// handle error (iteration stops after the first one)
			break
		}
		fmt.Println(article.Title)
	}
}

// Collecting a bounded number of articles in one call.
func ExampleNewsService_Collect() {
	client, _ := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	articles, err := client.News.Collect(context.Background(), &mediastack.NewsParams{
		Sort: mediastack.SortPopularity,
	}, 250)
	if err != nil {
		return
	}
	fmt.Println("collected", len(articles))
}

// Fetching historical articles for a date range.
func ExampleNewsService_Historical() {
	client, _ := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	resp, err := client.News.Historical(context.Background(), from, to,
		&mediastack.NewsParams{Limit: 50})
	if err != nil {
		return
	}
	fmt.Println("total:", resp.Pagination.Total)
}

// Finding news sources by name.
func ExampleSourcesService_Search() {
	client, _ := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	resp, err := client.Sources.Search(context.Background(), "bbc", &mediastack.SourceParams{
		Countries: mediastack.Filter[mediastack.Country]{
			Include: []mediastack.Country{mediastack.CountryGB},
		},
	})
	if err != nil {
		return
	}
	for _, s := range resp.Data {
		fmt.Println(s.ID, s.Name)
	}
}

// Building include/exclude filters.
func ExampleFilter() {
	params := &mediastack.NewsParams{
		Categories: mediastack.Filter[mediastack.Category]{
			Include: []mediastack.Category{mediastack.CategoryBusiness},
			Exclude: []mediastack.Category{mediastack.CategorySports},
		},
	}
	fmt.Println(params.Categories.Include[0], params.Categories.Exclude[0])
	// Output:
	// business sports
}

// Distinguishing error kinds with errors.Is / errors.As.
func ExampleAPIError() {
	client, _ := mediastack.NewClient("invalid-key")
	_, err := client.News.List(context.Background(), nil)

	var apiErr *mediastack.APIError
	switch {
	case errors.As(err, &apiErr):
		fmt.Println(apiErr.HTTPStatus, apiErr.Code)
	case errors.Is(err, mediastack.ErrRateLimitReached):
		fmt.Println("slow down")
	}
}

// Manual pagination without the iterator.
func ExamplePagination_HasMore() {
	client, _ := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	ctx := context.Background()
	params := &mediastack.NewsParams{Limit: 100}

	for {
		resp, err := client.News.List(ctx, params)
		if err != nil {
			break
		}
		for _, a := range resp.Data {
			fmt.Println(a.Title)
		}
		if !resp.Pagination.HasMore() {
			break
		}
		params.Offset = resp.Pagination.NextOffset()
	}
}
