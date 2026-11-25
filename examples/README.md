# Examples

Every example is a self-contained `package main` that reads the access key
from `MEDIASTACK_ACCESS_KEY` and compiles with `go build ./examples/...`.

| Example | Shows |
| --- | --- |
| `basic` | Minimal client setup and a first `/v1/news` call |
| `filters` | Include/exclude filters, keywords, `only_with_image`, sorting |
| `pagination` | `Iter`, `Collect` and a manual offset loop |
| `historical` | Date ranges, named presets, paid-plan errors |
| `sources` | Required `search` term, filters, source iteration |
| `errors` | `errors.Is`/`errors.As`, `APIError` fields, validation errors |
| `ratelimit` | Client-side token bucket, quota headers, shared limiter |

Run any of them with:

```bash
export MEDIASTACK_ACCESS_KEY=your_key
go run ./examples/basic
```
