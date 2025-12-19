// SPDX-License-Identifier: MIT

// Package core provides the normative validation engine for NomLedger.
//
// It acts as a storage-agnostic validation engine that enforces strict financial
// invariants on in-memory data structures. The package guarantees that any
// Transaction created via its builders or facades satisfies the following rules:
//
//  1. Non-Empty: A transaction must contain at least one entry.
//  2. Zero-Sum: The sum of all entries in a transaction must equal zero.
//  3. Period Closure: Transactions cannot be booked into closed periods unless
//     marked as adjusting events.
//  4. Currency Consistency: All entries must conform to the ledger's functional currency.
//  5. Rate Integrity: Exchange rates must be within a tolerance explicitly
//     configured in LedgerConfig.
//
// Error Handling:
// Failures in normative checks return typed sentinel errors from the pkg/errors package
// (e.g., ErrZeroSumViolation).
//
// Failures in external dependencies (RateProvider) are returned verbatim to the
// caller. These errors are opaque to the engine and are not wrapped.
//
// The primary entry point is the Ledger struct, which is safe for concurrent use.
// Lower-level primitives like TxBuilder are provided for granular control but
// require manual lifecycle management.
package core
