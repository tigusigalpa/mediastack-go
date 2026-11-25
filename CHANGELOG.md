# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-25

### Added
- Initial release.
- `GET /v1/news` via `NewsService.List`, `Historical`, `Iter`, `Collect`.
- `GET /v1/sources` via `SourcesService.List`, `Search`, `Iter`.
- Typed filters with include/exclude semantics (`Filter[T]`).
- Date filters: single day, inclusive range, named presets (`DateFilter`, `ParseDateFilter`).
- Typed errors with `errors.Is`/`errors.As` support covering all three API error formats.
- Retries with exponential backoff, jitter and `Retry-After` (seconds and HTTP-date).
- Optional client-side token-bucket rate limiter and APILayer quota header parsing (`RateLimit`, `WithRateLimitObserver`).
- Access-key redaction across errors, logs and `%v` output; response body size cap.
- `iter.Seq2` lazy pagination bounded by the 10,000-record API window (`ErrMaxOffset`).
- Zero external dependencies.

[Unreleased]: https://github.com/tigusigalpa/mediastack-go/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/tigusigalpa/mediastack-go/releases/tag/v1.0.0
