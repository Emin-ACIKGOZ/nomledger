// SPDX-License-Identifier: MIT

package core_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

func exampleClock() core.Clock {
	return core.ClockFunc(func() time.Time {
		return time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	})
}

func ExampleLedger_ValidateTransaction() {
	config := core.LedgerConfig{
		FunctionalCurrency: codeUSD,
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	provider := interfaces.RateProviderFunc(func(context.Context, interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.NewFromInt(1), nil
	})
	ledger, err := core.NewLedger(config, provider, core.WithClock(exampleClock()))
	if err != nil {
		panic(err)
	}

	debit, err := core.NewEntry("acc-cash", decimal.NewFromInt(100), codeUSD, decimal.NewFromInt(1), codeUSD)
	if err != nil {
		panic(err)
	}
	credit, err := core.NewEntry("acc-revenue", decimal.NewFromInt(-100), codeUSD, decimal.NewFromInt(1), codeUSD)
	if err != nil {
		panic(err)
	}
	transaction, err := ledger.ValidateTransaction(context.Background(), core.TransactionInput{
		ID:            "tx-example",
		EffectiveDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Entries:       []core.Entry{debit, credit},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("id=%s entries=%d posted=%s\n",
		transaction.ID(), len(transaction.Entries()), transaction.PostedAt().Format(time.RFC3339))
	// Output: id=tx-example entries=2 posted=2024-06-01T12:00:00Z
}

func ExampleLedger_ValidateTransaction_foreign() {
	config := core.LedgerConfig{
		FunctionalCurrency: codeUSD,
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		RateTolerance:      decimal.RequireFromString("0.01"),
	}
	provider := interfaces.RateProviderFunc(func(context.Context, interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.RequireFromString("1.1"), nil
	})
	ledger, err := core.NewLedger(config, provider, core.WithClock(exampleClock()))
	if err != nil {
		panic(err)
	}

	debit, err := core.NewEntry(
		"acc-eur",
		decimal.RequireFromString("100.00"),
		"EUR",
		decimal.RequireFromString("1.1"),
		codeUSD,
	)
	if err != nil {
		panic(err)
	}
	credit, err := core.NewEntry(
		"acc-usd",
		decimal.RequireFromString("-110.00"),
		codeUSD,
		decimal.NewFromInt(1),
		codeUSD,
	)
	if err != nil {
		panic(err)
	}
	transaction, err := ledger.ValidateTransaction(context.Background(), core.TransactionInput{
		ID:            "tx-foreign",
		EffectiveDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Entries:       []core.Entry{debit, credit},
	})
	if err != nil {
		panic(err)
	}

	entry := transaction.Entries()[0]
	fmt.Printf("%s %s\n", entry.FunctionalAmount().StringFixed(2), entry.FunctionalCurrency())
	// Output: 110.00 USD
}

func Example_validationError() {
	config := core.LedgerConfig{
		FunctionalCurrency: codeUSD,
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	provider := interfaces.RateProviderFunc(func(context.Context, interfaces.RateRequest) (decimal.Decimal, error) {
		return decimal.NewFromInt(1), nil
	})
	ledger, err := core.NewLedger(config, provider, core.WithClock(exampleClock()))
	if err != nil {
		panic(err)
	}

	debit, err := core.NewEntry("acc-cash", decimal.NewFromInt(100), codeUSD, decimal.NewFromInt(1), codeUSD)
	if err != nil {
		panic(err)
	}
	credit, err := core.NewEntry("acc-revenue", decimal.NewFromInt(-99), codeUSD, decimal.NewFromInt(1), codeUSD)
	if err != nil {
		panic(err)
	}
	_, err = ledger.ValidateTransaction(context.Background(), core.TransactionInput{
		ID:            "tx-unbalanced",
		EffectiveDate: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Entries:       []core.Entry{debit, credit},
	})

	var validationErr *pkgerr.ValidationError
	structured := errors.As(err, &validationErr)
	zeroSum := errors.Is(err, pkgerr.ErrZeroSumViolation)
	operation := ""
	entryIndex := 0
	if structured {
		operation = validationErr.Operation
		entryIndex = validationErr.EntryIndex
	}
	fmt.Printf("zeroSum=%t structured=%t operation=%s entry=%d\n",
		zeroSum, structured, operation, entryIndex)
	// Output: zeroSum=true structured=true operation=ValidateTransaction entry=-1
}
