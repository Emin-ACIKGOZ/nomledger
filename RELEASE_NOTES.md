# NomLedger v0.1.1 Release Notes

| Field | Value |
|---|---|
| Version | v0.1.1 |
| Release date | 2026-11-05 |
| Status | Initial public release (pre-1.0) |
| Go requirement | Go 1.22.2 or newer |

## Highlights

NomLedger v0.1.1 is a storage-agnostic, double-entry validation core for Go. It
validates in-memory financial transactions against explicit accounting
invariants and returns structurally immutable results ready for persistence.

Highlights include:

- 178 active ISO 4217 codes generated from SIX List One (2026-09-17)
- Currency-aware `Money` and six rounding modes
- Strict account, entry, transaction, and ledger configuration validation
- Exact zero-sum validation, period closure, currency consistency, and as-of
  rate integrity
- Canonical `TransactionInput` validation API
- Context- and date-aware rate providers with per-validation caching
- `NotApplicable` and `FVOCI` measurement categories
- Injectable clocks for deterministic `PostedAt` values
- Structured validation errors with `errors.Is` and `errors.As` support
- CI gates for linting, race tests, coverage, fuzzing, mutation tests, SPDX,
  documentation, and vulnerability scanning

## Installation

```bash
go get github.com/Emin-ACIKGOZ/nomledger@v0.1.1
```

## Quick migration

| Old | v0.1.1 |
|---|---|
| `module nomledger` | `github.com/Emin-ACIKGOZ/nomledger` |
| `ledger := NewLedger(config, provider)` | `ledger, err := NewLedger(config, provider, opts...)` |
| `ledger.UpdateConfig(config)` | `updated, err := ledger.UpdateConfig(config)` |
| `GetRate(base, quote string)` | `GetRate(ctx, RateRequest{Base, Quote, AsOf})` |
| `ledger.ValidateTransaction(id, date, adjusting, entries...)` | `ledger.ValidateTransaction(ctx, TransactionInput{...})` |
| Legacy variadic call shape | `ValidateTransactionLegacy(...)`, deprecated |
| Unknown codes default to precision 2 | Unknown codes return `ErrUnknownCurrency` |
| Provider errors returned verbatim | Provider errors wrapped with `ErrRateProvider` |
| Reusable builder after success | Second `Build` returns `ErrBuilderFinalized` |

## Conformance profile

NomLedger targets the **NomLedger Core Conformance Profile v1**. It is informed
by IAS 21, IAS 10, and IFRS 9 classification tags, but claims only that explicit
validation profile. It makes no blanket IFRS compliance claim and does not
perform measurement, classification tests, or financial reporting.

## Changes from the prototype

- The canonical module path is `github.com/Emin-ACIKGOZ/nomledger`.
- `NewLedger` and `UpdateConfig` return errors.
- `RateProvider` is context- and date-aware; the old non-context signature is
  removed.
- `ValidateTransaction(ctx, TransactionInput)` is canonical; the legacy variadic
  method is deprecated.
- Entry construction rejects invalid precision, zero amounts, non-positive
  rates, non-one same-currency rates, and functional amounts that round to zero.
- Unknown currencies are rejected; there is no default precision.
- Provider errors are wrapped with `ErrRateProvider`.
- A successful build finalizes the builder.

## Deprecation

`Ledger.ValidateTransactionLegacy` is deprecated in favor of
`ValidateTransaction(ctx, TransactionInput)`. It remains available for at least
one minor release, with the earliest removal in v1.0.0 (a future major version).

`currency.GetPrecision` and `currency.RoundToPrecision` remain deprecated
compatibility wrappers.

## Removed

`documentation.txt` has been removed. API documentation is available through
`go doc`.

## Known limitations

- Metadata values are shallow-copied. Reference values such as pointers, maps,
  and slices remain caller-owned and may still be mutated by their owner.
- No full IFRS assurance is claimed. The library validates a named conformance
  profile and does not provide accounting measurement or reporting.

## Checks run

| Check | Result |
|---|---|
| Unit/integration/race tests | Pass on Go 1.27.3 and Go 1.22.11 |
| Coverage | 94.9% (threshold 90%) |
| Builder mutation tests | 10/10 caught, 0 invalid |
| Fuzz targets (`FuzzLookup`, `FuzzRound`, `FuzzNewEntry`, `FuzzValidateTransaction`) | 30 seconds each, no panics |
| Lint (`golangci-lint` v2.14.0) | 0 issues |
| API golden / no-float checks | 103 entries; pass |
| `govulncheck` | No vulnerabilities found |
| SPDX and documentation checks | Pass |

## Non-goals

NomLedger does not provide persistence, reporting or aggregation, natural-balance
enforcement, FX sourcing, tax calculations or full IFRS measurement, historical
currencies, an account registry, duplicate detection, serialization codecs,
multi-tenancy, or full IFRS compliance.
