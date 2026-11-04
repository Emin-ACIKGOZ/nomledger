// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"
	"github.com/Emin-ACIKGOZ/nomledger/pkg/interfaces"

	"github.com/shopspring/decimal"
)

type rateKey struct {
	base  currency.Code
	quote currency.Code
	asOf  time.Time
}

// validateRates checks foreign-entry rates against the provider.
//
// Each unique (base, quote, as-of) request is resolved at most once per
// validation. Same-currency entries are skipped entirely. Provider errors are
// wrapped with ErrRateProvider while preserving their cause, and context
// cancellation aborts before any further provider call.
func validateRates(
	ctx context.Context,
	entries []Entry,
	tolerance decimal.Decimal,
	provider interfaces.RateProvider,
	asOf time.Time,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	asOf = asOf.UTC()
	cache := make(map[rateKey]decimal.Decimal)

	for i, entry := range entries {
		if entry.TxCurrency() == entry.FunctionalCurrency() {
			continue
		}
		if err := validateEntryRate(ctx, entry, i, tolerance, provider, asOf, cache); err != nil {
			return err
		}
	}

	return nil
}

func validateEntryRate(
	ctx context.Context,
	entry Entry,
	entryIndex int,
	tolerance decimal.Decimal,
	provider interfaces.RateProvider,
	asOf time.Time,
	cache map[rateKey]decimal.Decimal,
) error {
	if entry.ExchangeRate().LessThanOrEqual(decimal.Zero) {
		return rateError(pkgerr.ErrInvalidExchangeRate, entryIndex, "exchangeRate", nil)
	}
	if err := ctx.Err(); err != nil {
		return rateError(pkgerr.ErrRateProvider, entryIndex, "context", err)
	}
	if provider == nil {
		return rateError(pkgerr.ErrNilRateProvider, entryIndex, "rate", nil)
	}

	request := interfaces.RateRequest{
		Base:  currency.Code(entry.TxCurrency()),
		Quote: currency.Code(entry.FunctionalCurrency()),
		AsOf:  asOf,
	}
	providerRate, err := resolveRate(ctx, request, entryIndex, provider, cache)
	if err != nil {
		return err
	}

	if providerRate.LessThanOrEqual(decimal.Zero) {
		return rateError(pkgerr.ErrInvalidExchangeRate, entryIndex, "rate", nil)
	}

	deviation := providerRate.Sub(entry.ExchangeRate()).Abs()
	if deviation.GreaterThan(tolerance) {
		return rateError(pkgerr.ErrRateDeviation, entryIndex, "rate", nil)
	}
	return nil
}

// resolveRate returns a cached or freshly fetched provider rate. If the
// provider fails while the context is also cancelled, both the provider error
// and the context error are preserved in the returned ValidationError's cause.
func resolveRate(
	ctx context.Context,
	request interfaces.RateRequest,
	entryIndex int,
	provider interfaces.RateProvider,
	cache map[rateKey]decimal.Decimal,
) (decimal.Decimal, error) {
	key := rateKey{base: request.Base, quote: request.Quote, asOf: request.AsOf}
	if providerRate, cached := cache[key]; cached {
		return providerRate, nil
	}

	providerRate, providerErr := provider.GetRate(ctx, request)
	ctxErr := ctx.Err()
	if providerErr != nil {
		cause := fmt.Errorf(
			"rate %s/%s as of %s: %w",
			request.Base,
			request.Quote,
			request.AsOf.Format(time.RFC3339Nano),
			providerErr,
		)
		if ctxErr != nil {
			return decimal.Zero, rateError(
				pkgerr.ErrRateProvider,
				entryIndex,
				"context",
				errors.Join(cause, ctxErr),
			)
		}
		return decimal.Zero, rateError(pkgerr.ErrRateProvider, entryIndex, "rate", cause)
	}
	if ctxErr != nil {
		return decimal.Zero, rateError(pkgerr.ErrRateProvider, entryIndex, "context", ctxErr)
	}

	cache[key] = providerRate
	return providerRate, nil
}

func rateError(code error, entryIndex int, field string, cause error) *pkgerr.ValidationError {
	return pkgerr.NewEntryValidationError(code, buildOperation, entryIndex, field, cause)
}
