// SPDX-License-Identifier: MIT

package core

import "time"

// Clock supplies the current time for transaction construction.
//
// The default clock is time.Now. Implementations must be safe for concurrent
// use when the ledger is shared; time.Now is. Values read from a clock are
// normalized to UTC before they are stored in Transaction.PostedAt.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to the Clock interface.
type ClockFunc func() time.Time

// Now calls f(). A nil ClockFunc falls back to time.Now.
func (f ClockFunc) Now() time.Time {
	if f == nil {
		return time.Now()
	}
	return f()
}

type ledgerOptions struct {
	clock Clock
}

// LedgerOption configures a Ledger during construction.
type LedgerOption func(*ledgerOptions) error

// WithClock selects the ledger clock.
//
// A nil clock means the default (time.Now). The option itself never returns an
// error.
func WithClock(c Clock) LedgerOption {
	return func(options *ledgerOptions) error {
		options.clock = c
		return nil
	}
}

type builderOptions struct {
	clock Clock
}

// BuilderOption configures a TxBuilder during construction.
type BuilderOption func(*builderOptions) error

// WithBuilderClock selects the clock used by a TxBuilder.
//
// A nil clock means the default (time.Now). The option itself never returns an
// error.
func WithBuilderClock(c Clock) BuilderOption {
	return func(options *builderOptions) error {
		options.clock = c
		return nil
	}
}
