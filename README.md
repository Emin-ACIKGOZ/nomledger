# NomLedger

**NomLedger** is a storage-agnostic, double-entry **validation core** for Go. It
checks in-memory financial data against explicit accounting invariants before
the data is persisted anywhere.

## Overview

NomLedger validates transactions and returns structurally immutable,
currency-aware values ready for persistence. It guarantees non-empty entries,
exact zero-sum balancing, period closure rules, functional-currency consistency,
and as-of exchange-rate integrity.

It does **not** store, aggregate, report, or source financial data. Its job is
to reject invalid financial state before a caller writes it elsewhere.

## Installation

```bash
go get github.com/Emin-ACIKGOZ/nomledger
```

## Quick start

This is a complete, compilable program using the v1 API and an injected clock
for deterministic `PostedAt` values.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"
)

func main() {
	config := core.LedgerConfig{
		FunctionalCurrency: "USD",
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		RateTolerance:      decimal.RequireFromString("0.00001"),
	}

	provider := interfaces.RateProviderFunc(func(ctx context.Context, req interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.NewFromInt(1), nil
	})
	clock := core.ClockFunc(func() time.Time {
		return time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	ledger, err := core.NewLedger(config, provider, core.WithClock(clock))
	if err != nil {
		panic(err)
	}

	cash, err := core.NewEntry(
		"acc-cash",
		decimal.RequireFromString("100.00"),
		"USD",
		decimal.NewFromInt(1),
		"USD",
	)
	if err != nil {
		panic(err)
	}
	revenue, err := core.NewEntry(
		"acc-revenue",
		decimal.RequireFromString("-100.00"),
		"USD",
		decimal.NewFromInt(1),
		"USD",
	)
	if err != nil {
		panic(err)
	}

	input := core.TransactionInput{
		ID:            "tx-101",
		EffectiveDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Entries:       []core.Entry{cash, revenue},
		Metadata:      map[string]any{"source": "readme"},
	}

	tx, err := ledger.ValidateTransaction(context.Background(), input)
	if err != nil {
		panic(err)
	}

	fmt.Printf("validated %s with %d entries at %s\n",
		tx.ID(), len(tx.Entries()), tx.PostedAt().Format(time.RFC3339))
}
```

## Error handling

Every validation failure matches a sentinel with `errors.Is` and exposes
structured context through `errors.As`.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"
)

func main() {
	config := core.LedgerConfig{
		FunctionalCurrency: "USD",
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		RateTolerance:      decimal.Zero,
	}

	provider := interfaces.RateProviderFunc(func(ctx context.Context, req interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.NewFromInt(1), nil
	})

	ledger, err := core.NewLedger(config, provider)
	if err != nil {
		panic(err)
	}

	debit, err := core.NewEntry(
		"acc-cash",
		decimal.RequireFromString("100.00"),
		"USD",
		decimal.NewFromInt(1),
		"USD",
	)
	if err != nil {
		panic(err)
	}
	credit, err := core.NewEntry(
		"acc-revenue",
		decimal.RequireFromString("-99.00"),
		"USD",
		decimal.NewFromInt(1),
		"USD",
	)
	if err != nil {
		panic(err)
	}

	input := core.TransactionInput{
		ID:            "tx-unbalanced",
		EffectiveDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Entries:       []core.Entry{debit, credit},
	}

	_, err = ledger.ValidateTransaction(context.Background(), input)
	if err == nil {
		fmt.Println("unexpected success")
		return
	}

	var validationErr *pkgerr.ValidationError
	if errors.As(err, &validationErr) {
		fmt.Printf("operation=%s field=%s entry=%d\n",
			validationErr.Operation, validationErr.Field, validationErr.EntryIndex)
	}
	if errors.Is(err, pkgerr.ErrZeroSumViolation) {
		fmt.Println("zero-sum validation failed as expected")
	}
}
```

### Error taxonomy

Retryable means the caller can retry without changing the input, for example
after a transient provider failure or with a fresh context. Non-retryable errors
require changing the input, configuration, or caller usage.

| Sentinel | Message | Returned when | Retryable |
|---|---|---|---|
| `ErrZeroSumViolation` | `transaction does not sum to zero` | `Build` sees functional amounts whose exact sum is non-zero. | No |
| `ErrPeriodClosed` | `effective date falls in closed period` | A non-adjusting transaction has an effective date at or before `ClosedDate`. | No |
| `ErrRateDeviation` | `exchange rate exceeds tolerance` | Provider and entry rates differ by more than `RateTolerance`. | No |
| `ErrRateProvider` | `rate provider error` | Context cancellation or deadline expiry before any validation/provider work (including same-currency transactions), cancellation during a provider call, or a provider call returns an error. | Yes |
| `ErrInvalidPrecision` | `amount exceeds currency precision` | An `Entry` or `Money` amount has more decimal places than the currency's minor units. | No |
| `ErrCurrencyMismatch` | `entry functional currency does not match ledger` | An entry's functional currency differs from the ledger's, or `Money` arithmetic mixes currencies. | No |
| `ErrEmptyTransaction` | `transaction must contain at least one entry` | `Build` receives zero entries. | No |
| `ErrZeroAmount` | `transaction amount must be non-zero` | `NewEntry` receives a zero transaction amount. | No |
| `ErrFunctionalAmountZero` | `functional amount rounds to zero` | A non-zero transaction amount rounds to zero in the functional currency. | No |
| `ErrInvalidEntry` | `invalid entry` | `Entry.Validate` sees a zero-value or internally inconsistent entry, or `Build`'s entry check fails. | No |
| `ErrNilRateProvider` | `rate provider must not be nil` | `NewLedger` receives a nil provider, or a rate lookup is attempted without one. | No |
| `ErrInvalidTransactionID` | `invalid transaction ID` | `Build` receives an empty, over-255-byte, or control-character transaction ID. | No |
| `ErrInvalidEffectiveDate` | `invalid effective date` | `Build` receives the zero time as the effective date. | No |
| `ErrInvalidMetadata` | `invalid metadata` | `Build` receives a metadata key that is empty, over 64 bytes, or contains a control character. | No |
| `ErrInvalidAccountID` | `invalid account ID` | `NewAccount`, `NewEntry`, or their `Validate` methods receive an invalid account ID. | No |
| `ErrInvalidAccountType` | `invalid account type` | `NewAccount` or `Account.Validate` receives an unsupported account type. | No |
| `ErrInvalidMeasurementCategory` | `invalid measurement category` | `NewAccount` or `Account.Validate` receives an unsupported measurement category. | No |
| `ErrInvalidExchangeRate` | `exchange rate must be strictly positive` | An entry or provider rate is zero or negative, or a same-currency entry rate is not exactly one. | No |
| `ErrInvalidConfiguration` | `invalid ledger configuration` | `LedgerConfig` has an invalid functional currency or a negative tolerance. | No |
| `ErrUnknownCurrency` | `unknown or unsupported currency code` | A well-formed code is absent from the active ISO 4217 registry. | No |
| `ErrInvalidCurrencyCode` | `currency code must be exactly three ASCII letters` | A currency code is not exactly three ASCII letters. | No |
| `ErrCurrencyNotRatable` | `currency has no defined minor units` | A currency with N.A. minor units is used where ratability is required. | No |
| `ErrInvalidRoundingMode` | `invalid rounding mode` | `Round` receives an unknown `RoundingMode`. | No |
| `ErrInexactRounding` | `rounding would change the value` | `RoundUnnecessary` would change the numeric amount. | No |
| `ErrBuilderFinalized` | `builder is already finalized` | `Build` is called again after a successful build finalized the builder. | No |

## Guarantees

Every successfully validated transaction satisfies:

- **Non-empty and valid entries**: at least one entry; each entry has a bounded,
  control-character-free account ID, valid and ratable currencies, a non-zero
  amount at or below the currency's minor units, and a positive exchange rate.
- **Exact zero-sum**: the decimal sum of all functional amounts is exactly zero.
  Ties are not resolved with a tolerance.
- **Period closure**: non-adjusting transactions at or before `ClosedDate` are
  rejected. Adjusting events bypass only the period-closure rule.
- **Currency consistency**: every entry's functional currency equals the
  ledger's configured functional currency.
- **As-of rate integrity**: foreign entries carry an effective date in each rate
  request. Provider rates are positive and must match the entry rate within the
  inclusive `RateTolerance`. The provider is called at most once per unique
  `(base, quote, as-of)` request per validation.
- **Immutable results**: `Transaction.Entries()` and `Transaction.Metadata()`
  return defensive copies; the transaction exposes no exported mutable fields.
- **Typed failures**: every failure matches a sentinel with `errors.Is` and
  carries a `*ValidationError` with operation, entry index, field, and optional
  cause.
- **Determinism**: the same configuration, input, provider responses, and clock
  produce the same transaction, including a UTC `PostedAt`.
- **Metadata copying**: the top-level metadata map is copied, but metadata values
  that are reference types remain caller-owned and shared.

Validation order is fixed: configuration, transaction ID, effective date,
non-empty entries, metadata keys, entry validity, currency consistency,
zero-sum, period closure, and rate integrity.

## Conformance profile

NomLedger targets the **NomLedger Core Conformance Profile v1**. It is informed
by IAS 21 (foreign currency), IAS 10 (adjusting events), and IFRS 9
classification tags. It claims only that explicit validation profile. It makes
**no blanket IFRS compliance claim** and does not perform measurement,
classification tests, or financial reporting.

## API summary

### `pkg/core`

- `LedgerConfig` with `FunctionalCurrency`, `ClosedDate`, and `RateTolerance`;
  `Validate()` checks the configuration.
- `NewLedger(config, provider, opts...)`, `WithClock`, `Config()`, and
  `UpdateConfig(config)`; `NewLedger` and `UpdateConfig` return errors.
  `NewTxBuilder` returns a builder and defers option errors to `Build`.
- `Ledger.ValidateTransaction(ctx, TransactionInput)` is the canonical path.
- `ValidateTransactionLegacy` preserves the pre-1.0 variadic call shape and is
  deprecated.
- `TransactionInput` carries ID, effective date, adjusting flag, entries, and
  metadata.
- `TxBuilder`/`NewTxBuilder`/`WithBuilderClock` provide an alternate construction
  path that runs the same pipeline. A successful `Build` finalizes the builder;
  later mutations are no-ops and a rebuild returns `ErrBuilderFinalized`.
- `Entry`/`NewEntry`, `Account`/`NewAccount`, `Clock`/`ClockFunc`,
  `AccountType`, and `MeasurementCategory` (including `FVOCI` and
  `NotApplicable`).
- `Transaction` accessors: `ID`, `EffectiveDate`, `IsAdjustingEvent`,
  `PostedAt`, `Entries`, and `Metadata`.

### `pkg/currency`

- `Code` and `Currency` with case-insensitive `Lookup`, `All`, `MinorUnits`,
  `Numeric`, `Name`, and `IsFund`.
- The registry contains 178 active codes from SIX ISO 4217 List One published
  2026-09-17. Unknown codes are rejected; there is no default precision.
- `RoundingMode` and `Round` for HalfEven, HalfUp, HalfDown, Floor, Ceiling, and
  Unnecessary rounding.
- `Money`/`NewMoney` with currency-safe `Add`, `Sub`, `Neg`, `IsZero`, and
  `String`.
- `GetPrecision` and `RoundToPrecision` remain as deprecated compatibility
  wrappers.

### `pkg/errors`

- Typed sentinels matched with `errors.Is`, including `ErrZeroSumViolation`,
  `ErrPeriodClosed`, `ErrCurrencyMismatch`, `ErrRateProvider`,
  `ErrInvalidPrecision`, `ErrUnknownCurrency`, and `ErrInvalidConfiguration`.
- `ValidationError` plus `NewValidationError` and `NewEntryValidationError` for
  structured context extracted with `errors.As`.

### `pkg/interfaces`

- `RateRequest` carries `Base` (transaction currency), `Quote` (functional
  currency), and the UTC `AsOf` effective date.
- `RateProvider.GetRate(ctx, req)` is context- and date-aware.
- `RateProviderFunc` adapts an ordinary function to `RateProvider`.

## Migration from pre-1.0

- The module path is `github.com/Emin-ACIKGOZ/nomledger`.
- `NewLedger` and `UpdateConfig` now return `(*Ledger, error)`. `NewTxBuilder`
  still returns only `*TxBuilder`; builder option errors surface from `Build`.
- `RateProvider` now uses `GetRate(ctx context.Context, req RateRequest)`; the
  old `GetRate(base, quote string)` method is gone.
- Use `ValidateTransaction(ctx, TransactionInput)`; the legacy variadic method
  is deprecated.
- Entry construction is strict: amounts beyond the transaction currency's minor
  units, zero amounts, non-positive rates, and non-one same-currency rates are
  rejected. Non-zero amounts that round to zero in the functional currency are
  also rejected.
- `MeasurementCategory` adds `NotApplicable` and `FVOCI`.
- The currency registry contains 178 active codes. Unknown currencies are no
  longer given a default precision.
- Provider errors are wrapped with `ErrRateProvider` and preserve their cause;
  they are no longer returned verbatim.
- A builder is final after one successful `Build`.

## Concurrency

- `Ledger` is safe for concurrent use.
- `TxBuilder` is not safe for concurrent use and should live within one
  execution scope.
- A shared `RateProvider` and `Clock` must be safe for concurrent use. The
  default `time.Now` clock is safe.

## Metadata copying

Metadata is non-normative. The top-level map is shallowly copied on input and
output, so callers cannot add, remove, or replace keys in a transaction through
the original map or through `Metadata()`. Values that are reference types (for
example pointers, maps, or slices) remain caller-owned and may still be mutated
by their owner. This shallow-copy behavior is intentional.

## Non-goals

NomLedger intentionally does not provide:

- Persistence or database integration
- Reporting, balances, or aggregation logic
- Natural-balance enforcement by account type
- FX sourcing or rate-provider implementations
- Tax calculations or full IFRS measurement
- Historical currencies
- An account registry or account storage
- Duplicate detection
- Serialization codecs
- Multi-tenancy
- Full IFRS compliance

## Documentation

API documentation is available on demand through `go doc`:

```bash
go doc github.com/Emin-ACIKGOZ/nomledger/pkg/core
go doc github.com/Emin-ACIKGOZ/nomledger/pkg/currency
go doc github.com/Emin-ACIKGOZ/nomledger/pkg/errors
go doc github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces
```

The stale generated `documentation.txt` snapshot has been removed.

## License

MIT
