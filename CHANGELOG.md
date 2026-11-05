# Changelog

All notable changes to NomLedger are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2026-11-06

This is the first tagged release; changes below are relative to the pre-release
prototype.

### Added

- ISO 4217 currency registry generated from SIX List One (published 2026-09-17),
  covering 178 active alphabetic codes with numeric codes, minor units, names,
  and List Two fund flags.
- `currency.Money` value type and six rounding modes: HalfEven, HalfUp,
  HalfDown, Floor, Ceiling, and Unnecessary.
- Account validation through `NewAccount` and `Account.Validate`, including ID
  bounds, account types, measurement categories, and ratable currency checks.
- `NotApplicable` and `FVOCI` measurement categories.
- Strict entry construction through `NewEntry`, with currency validation,
  precision enforcement, zero-amount rejection, positive-rate checks,
  same-currency identity checks, HalfEven conversion, and functional-zero
  rejection.
- Structurally immutable `Transaction` values with defensive copies of entries
  and metadata maps, plus `IsAdjustingEvent`, `PostedAt`, and `Metadata`
  accessors.
- Validated `LedgerConfig`, `NewLedger` and `UpdateConfig` error returns, config
  normalization, and `Config()` copies.
- `TransactionInput` and `Ledger.ValidateTransaction(ctx, TransactionInput)` as
  the canonical validation API.
- Context- and date-aware `RateProvider`, `RateRequest`, and `RateProviderFunc`,
  with per-validation request caching and wrapped provider errors.
- Injectable `Clock`/`ClockFunc` support for both ledgers and builders, with UTC
  `PostedAt` values.
- Structured `ValidationError` values with operation, entry index, field, code,
  and optional cause, plus a complete sentinel taxonomy.
- CI quality gates for formatting, vetting, linting, race tests, coverage,
  fuzzing, mutation tests, SPDX headers, documentation checks, and
  vulnerability scanning.
- README quick-start and error-handling examples, changelog, and release notes.

### Changed

- **Breaking from the prototype:** the module path is now
  `github.com/Emin-ACIKGOZ/nomledger`.
- **Breaking from the prototype:** `NewLedger` and `UpdateConfig` now return `(*Ledger, error)`.
- **Breaking from the prototype:** `RateProvider` now uses
  `GetRate(ctx context.Context, req RateRequest)`; the old
  `GetRate(base, quote string)` method is gone.
- **Breaking from the prototype:** `Ledger.ValidateTransaction(ctx, TransactionInput)` is the
  canonical validation path. `ValidateTransactionLegacy` preserves the earlier
  variadic call shape and is deprecated.
- **Breaking from the prototype:** entry construction now rejects amounts beyond the transaction
  currency's minor units, zero amounts, non-positive rates, non-one
  same-currency rates, and functional amounts that round to zero.
- **Breaking from the prototype:** unknown currencies are rejected instead of receiving a default
  precision.
- **Breaking from the prototype:** provider errors are wrapped with `ErrRateProvider` and preserve
  their cause; they are no longer returned verbatim.
- A successfully built `TxBuilder` is final; later mutations are no-ops and a
  rebuild returns `ErrBuilderFinalized`.
- Requires Go 1.22.2 or newer and uses `github.com/shopspring/decimal` v1.6.0.

### Deprecated

- `Ledger.ValidateTransactionLegacy` is deprecated in favor of
  `ValidateTransaction(ctx, TransactionInput)`. It remains available for at
  least one minor release, with the earliest removal in v1.0.0 (a future major
  version).
- `currency.GetPrecision` and `currency.RoundToPrecision` remain deprecated
  compatibility wrappers for the registry and rounding APIs.

### Removed

- `documentation.txt`. API documentation is available through `go doc`.

### Fixed

- Builder entries and metadata no longer alias the returned transaction.
- Returned transaction entries and metadata maps are defensive copies.
- Zero transaction amounts and invalid exchange rates are rejected at
  construction.
- Currency precision is validated instead of silently rounding transaction
  amounts.
- Metadata maps are copied on input and output, preventing top-level map
  aliasing.
