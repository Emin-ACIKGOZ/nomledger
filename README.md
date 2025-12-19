
# NomLedger

**NomLedger** is a storage-agnostic financial **validation engine** for Go that enforces strict accounting invariants on in-memory data before persistence.

## Overview

NomLedger ensures that every transaction satisfies explicit accounting rules, including zero-sum balancing, period closure enforcement, currency consistency, and exchange rate integrity. It validates financial state deterministically before data is written to any database, message log, or external system.

The library is **not** an ORM, ledger database, or reporting engine. Its sole responsibility is enforcing correctness invariants.

## Installation

```bash
go get github.com/Emin-ACIKGOZ/nomledger
```

## Usage

NomLedger validates raw financial inputs and returns immutable, validated transactions ready for persistence.

### 1. Define a Rate Provider

Implement `interfaces.RateProvider` to supply exchange rates from your chosen source.

```go
import (
	"github.com/shopspring/decimal"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"
)

type FixedRateProvider struct{}

func (f FixedRateProvider) GetRate(base, quote string) (decimal.Decimal, error) {
	return decimal.NewFromFloat(1.0), nil
}
```

### 2. Validate a Transaction

Initialize the engine with an explicit configuration and validate atomically.

```go
package main

import (
	"log"
	"time"

	"github.com/shopspring/decimal"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
)

func main() {
	config := core.LedgerConfig{
		FunctionalCurrency: "USD",
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		RateTolerance:      decimal.NewFromFloat(0.00001),
	}

	ledger := core.NewLedger(config, FixedRateProvider{})

	tx, err := ledger.ValidateTransaction(
		"tx-101",
		time.Now(),
		false,

		core.NewEntry(
			"acc-marketing",
			decimal.NewFromInt(100),
			"USD",
			decimal.NewFromInt(1),
			"USD",
		),

		core.NewEntry(
			"acc-cash",
			decimal.NewFromInt(-100),
			"USD",
			decimal.NewFromInt(1),
			"USD",
		),
	)

	if err != nil {
		log.Fatalf("validation failed: %v", err)
	}

	// tx is immutable and safe to persist or share across goroutines
	_ = tx
}
```

## Core Guarantees

All validated transactions enforce the following invariants:

* **Non-Empty Transactions**: A transaction must contain at least one entry, and each entry must have a non-zero functional amount. Zero-entry and zero-valued entries are invalid.
* **Zero-Sum Accounting**: The sum of all functional amounts must equal zero
  *(convention: positive = debit, negative = credit; zero-valued entries are invalid)*.
* **Period Closure**: Transactions with an effective date less than or equal to the configured closed date are rejected unless explicitly marked as adjusting events.
* **Currency Consistency**: All entries must resolve into the single functional currency defined by the ledger configuration.
* **Rate Integrity**:

  * Entry-provided exchange rates must be strictly positive.
  * Provider-returned exchange rates must be strictly positive.
  * The absolute difference between the entry rate and provider rate must not exceed the explicitly configured tolerance.

Validated transactions are immutable and safe for concurrent use.

## Architecture

* **`core`**: Validation engine, `Ledger` facade, and `TxBuilder`.
* **`currency`**: ISO 4217 precision tables and banker's rounding utilities.
* **`errors`**: Typed sentinel errors for deterministic failure handling.
* **`interfaces`**: Dependency injection contracts (e.g., `RateProvider`).

The engine is storage-agnostic and framework-independent.

## Concurrency Model

* **`Ledger`**: Safe for concurrent use. Instantiate one per configuration (for example, per entity or tenant).
* **`TxBuilder`**: Not thread-safe. Intended for constructing a single transaction within a single execution scope.
  `Ledger.ValidateTransaction` manages this lifecycle automatically.

## Non-Goals

NomLedger intentionally does not provide:

* Persistence or database integration
* Reporting, balances, or aggregation logic
* Natural balance enforcement by account type
* Currency discovery or FX sourcing mechanisms

## Use Cases

* Financial backends requiring deterministic validation before persistence
* Accounting systems enforcing IFRS-aligned invariants
* Event-sourced architectures with strict command validation
* High-integrity transaction processing pipelines

## License

MIT
