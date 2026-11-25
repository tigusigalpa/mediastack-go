// Errors demonstrates the typed error model: errors.Is for sentinels,
// errors.As for the structured *APIError, and how the two documented
// error formats normalize to the same shape.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/errors
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mediastack "github.com/tigusigalpa/mediastack-go"
)

func main() {
	client, err := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Deliberately invalid: sort outside the documented set is caught
	// client-side before a request is sent.
	_, err = client.News.List(ctx, &mediastack.NewsParams{Sort: "newest"})
	if errors.Is(err, mediastack.ErrInvalidParams) {
		fmt.Println("client-side validation:", err)
	}

	// An invalid date preset reaches the API and comes back as a 422.
	_, err = client.News.List(ctx, &mediastack.NewsParams{
		Extra: map[string][]string{"date": {"bogus"}},
	})
	var apiErr *mediastack.APIError
	switch {
	case errors.As(err, &apiErr):
		fmt.Printf("api error: status=%d code=%q\n  message: %s\n",
			apiErr.HTTPStatus, apiErr.Code, apiErr.Message)
		if vc := apiErr.ValidationContext(); vc != "" {
			fmt.Println("  fields:", vc)
		}
	case errors.Is(err, mediastack.ErrRateLimitReached):
		fmt.Println("rate limited — slow down")
	case errors.Is(err, mediastack.ErrUsageLimitReached):
		fmt.Println("monthly quota exhausted — upgrade or wait")
	default:
		fmt.Println("unexpected:", err)
	}

	// A deliberately bad key shows the authentication path.
	bad, _ := mediastack.NewClient("invalid-key")
	_, err = bad.News.List(ctx, &mediastack.NewsParams{Limit: 1})
	fmt.Println("bad key:", err)
}
