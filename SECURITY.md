# Security Policy

## Supported versions

| Version | Supported |
| --- | --- |
| 1.x (latest) | yes |
| < 1.0 | no |

Only the most recent release receives security fixes.

## What counts as a vulnerability

In scope for this SDK:

- Leakage of the API access key through errors, logs, `fmt` output, headers or serialized state.
- Input that causes credential exfiltration, request smuggling or unintended requests.
- Panics, data races or unbounded resource use reachable through normal SDK usage.
- Dependency or supply-chain issues affecting this module.

**Out of scope** — report these to APILayer support (support@apilayer.com) instead:

- Vulnerabilities of the Mediastack API service itself (authentication, quotas, data).
- Issues requiring a stolen/valid API key already in the reporter's possession.
- Social engineering, physical access, or bugs in your own application code.

## Reporting

Email **sovletig@gmail.com** with:

- a description and impact assessment,
- a minimal reproduction (code, not screenshots),
- the SDK version and Go version.

Please do **not** open a public issue for undisclosed vulnerabilities.

**Never include a real API access key in a report** — the SDK redacts keys in output, but paste nothing you wouldn't publish.

## Response

- Acknowledgement within 72 hours.
- Assessment and, when confirmed, a fix plan within 14 days.
- Credit in the release notes if you want it.

## Disclosure

Fixes are released, then the issue is disclosed publicly no earlier than 30 days after the fix release, coordinated with the reporter.
