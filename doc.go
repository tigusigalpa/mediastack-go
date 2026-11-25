// Package mediastack provides a production-ready Go client for the
// Mediastack news API, a product of APILayer. Mediastack aggregates live
// and historical news articles and blog posts from 7,500+ sources in 50+
// countries and 13 languages and returns them as flat JSON.
//
// The SDK covers the complete documented API surface: GET /v1/news and
// GET /v1/sources. It offers typed request parameters, include/exclude
// filters, lazy pagination iterators, exponential-backoff retries, an
// optional client-side rate limiter and typed errors that understand all
// error formats the API emits.
//
// Import path:
//
//	import "github.com/tigusigalpa/mediastack-go"
//
// Minimal example:
//
//	client, err := mediastack.NewClient(os.Getenv(mediastack.EnvAccessKey))
//	if err != nil {
//		log.Fatal(err)
//	}
//	resp, err := client.News.List(ctx, &mediastack.NewsParams{Limit: 3})
//	if err != nil {
//		log.Fatal(err)
//	}
//	for _, a := range resp.Data {
//		fmt.Println(a.Title)
//	}
//
// API documentation: https://docs.apilayer.com/mediastack/docs/api-documentation
// Endpoint reference: https://docs.apilayer.com/mediastack/docs/mediastack-api-v-1-0-0
//
// The public identity (namespace) of this SDK is Tigusigalpa/Mediastack,
// exposed as the Namespace constant. The Go package name is mediastack,
// following Go naming conventions.
//
// This SDK is unofficial and is maintained independently of APILayer.
package mediastack
