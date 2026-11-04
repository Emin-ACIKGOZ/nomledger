// SPDX-License-Identifier: MIT

// Package interfaces defines the contracts required for external dependency
// injection.
//
// RateProvider is context- and date-aware: implementations receive a
// RateRequest containing the transaction currency, functional currency, and
// effective date. They MUST be deterministic for the same request during one
// validation and thread-safe when a ledger is shared across goroutines.
package interfaces
