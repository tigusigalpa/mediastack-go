# Contributing

Thanks for your interest in improving `mediastack-go`. This file describes the workflow that keeps the SDK consistent and releasable.

## Prerequisites

- Go 1.23 or newer (`go version`)
- `golangci-lint` v2 (`golangci-lint --version`)

## Setup and checks

```bash
git clone https://github.com/tigusigalpa/mediastack-go
cd mediastack-go
go build ./...
make check    # gofmt + go vet + build + race tests + golangci-lint
```

`make check` must pass before a PR is submitted. Coverage is expected to stay at or above 90% (`make cover`).

## Ground rules

- **No external dependencies.** The module intentionally depends only on the standard library; PRs adding a `require` block will not be merged without an exceptionally good reason. CI enforces this via `go list -m all`.
- **One package.** Everything lives in package `mediastack` at the repository root. No `internal/`, `pkg/`, `cmd/` or `models/` splits.
- **Test first.** For a bug: write a failing test that reproduces it, then fix. For a feature: tests land in the same PR as the implementation. Unit tests must not touch the network — use `httptest`. Real-API tests belong in `integration_test.go` behind the `integration` build tag.
- **godoc everywhere.** Every exported symbol needs a comment starting with its name. The `revive` linter enforces this.
- **Never log or print the access key.** If you touch transport, error or logging code, extend the redaction tests (`redact_test.go`), don't weaken them.
- **Don't invent API surface.** Only endpoints and parameters documented in `api/openapi.json` or on https://docs.apilayer.com/mediastack exist. When documentation conflicts, prefer the behavior verified against the live API and record the decision in README's "Documentation Discrepancies" section.

## Adding coverage for a changed API

If Mediastack adds a parameter or endpoint:

1. Update `api/openapi.json` from the published-definition URL (see README → Endpoint Documentation).
2. Add the field to `NewsParams`/`SourceParams` (or a new type), wire it in `encode()`, and document the wire format.
3. Add a fixture under `testdata/` matching real API output, plus a table test.
4. Update the parameter tables in README's Endpoint Documentation section.

## Commits and PRs

- Commit style: [Conventional Commits](https://www.conventionalcommits.org) — `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `ci:`.
- Keep commits atomic; describe *why*, not just *what*.
- Fill in the PR template checklist, including the `CHANGELOG.md` entry under `[Unreleased]`.

## Releasing (maintainers)

1. Bump `Version` in `version.go` and move `[Unreleased]` entries in `CHANGELOG.md` under a new `[x.y.z] - YYYY-MM-DD` heading.
2. `git tag -a vx.y.z -m "mediastack-go vx.y.z" && git push origin main --tags`.
3. The `release.yml` workflow runs tests and creates the GitHub release; pkg.go.dev indexes the new tag automatically.
