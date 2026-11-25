// Historical fetches articles from a past date range. Historical data
// requires a paid plan; on the Free plan the API answers with a plan
// restriction error.
//
// Run it with:
//
//	MEDIASTACK_ACCESS_KEY=your_key go run ./examples/historical
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

	from := time.Now().AddDate(0, 0, -30)
	to := time.Now().AddDate(0, 0, -7)
	resp, err := client.News.Historical(ctx, from, to, &mediastack.NewsParams{
		Limit: 5,
	})
	if errors.Is(err, mediastack.ErrAccessRestricted) {
		fmt.Println("historical data requires a paid plan")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("%d total articles between %s and %s\n",
		resp.Pagination.Total, from.Format("2006-01-02"), to.Format("2006-01-02"))
	for _, a := range resp.Data {
		fmt.Printf("- %s (%s)\n", a.Title, a.PublishedAt.Format("2006-01-02"))
	}

	// Named presets work too: last week, last month, year to date.
	preset, err := client.News.List(ctx, &mediastack.NewsParams{
		Date:  mediastack.Preset(mediastack.DateLast7Days),
		Limit: 5,
	})
	if err != nil {
		return err
	}
	fmt.Println("last 7 days:", preset.Pagination.Total, "articles")
	return nil
}
