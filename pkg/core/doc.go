// SPDX-License-Identifier: MIT

// Package core provides the normative validation engine for NomLedger.
//
// It acts as a storage-agnostic validation engine that enforces strict financial
// invariants on in-memory data structures. Any Transaction produced by the
// builders or facades is structurally immutable and satisfies these guarantees:
//
//  1. Valid configuration: the ledger currency is active and ratable, and the
//     rate tolerance is non-negative.
//  2. Valid transaction fields: the ID and metadata keys are bounded and free of
//     control characters, and the effective date is stored in UTC.
//  3. Non-empty entries: a transaction contains at least one valid entry.
//  4. Currency consistency: every entry uses the ledger's functional currency.
//  5. Zero-sum: the exact sum of all functional amounts is zero.
//  6. Period closure: no non-adjusting transaction falls in a closed period.
//  7. Rate integrity: foreign-entry rates match the provider within the
//     configured tolerance.
//
// Validation follows a fixed order: configuration, transaction ID, effective
// date, non-empty entries, metadata keys, entry validity, currency consistency,
// zero-sum, period closure, and rate integrity. It fails fast, returns the zero
// Transaction on every error, and performs no later provider calls after an
// earlier failure.
//
// Failures are *ValidationError values wrapping typed sentinels from the
// pkg/errors package, so callers can use errors.Is and errors.As. Provider
// errors are wrapped with ErrRateProvider while preserving their original cause.
//
// The primary entry point is the Ledger struct, which is safe for concurrent
// use. Lower-level primitives like TxBuilder are provided for granular control
// but require manual lifecycle management. Metadata values are shallowly copied
// and remain caller-owned.
package core
