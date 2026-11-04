// SPDX-License-Identifier: MIT

package interfaces

import (
	"context"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	"github.com/shopspring/decimal"
)

// RateRequest identifies a rate lookup.
//
// Base is the transaction currency, Quote is the functional currency, and the
// returned rate is quote units per one base unit. AsOf is the transaction's
// effective date in UTC.
type RateRequest struct {
	Base  currency.Code
	Quote currency.Code
	AsOf  time.Time
}

// RateProvider abstracts the source of foreign exchange rates.
//
// Implementations MUST be deterministic for the same request during one
// validation and MUST be thread-safe when the ledger is shared concurrently.
type RateProvider interface {
	// GetRate returns the exchange rate for a request, or an error if the rate
	// cannot be determined. Implementations should honour ctx cancellation and
	// deadlines.
	GetRate(ctx context.Context, req RateRequest) (decimal.Decimal, error)
}

// RateProviderFunc adapts a function to the RateProvider interface.
type RateProviderFunc func(ctx context.Context, req RateRequest) (decimal.Decimal, error)

// GetRate calls f(ctx, req).
func (f RateProviderFunc) GetRate(ctx context.Context, req RateRequest) (decimal.Decimal, error) {
	return f(ctx, req)
}
