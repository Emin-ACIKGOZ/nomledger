// SPDX-License-Identifier: MIT

// Package currency provides the ISO 4217 currency registry, code validation,
// precision arithmetic, and rounding rules.
//
// The registry is compiled in from SIX List One (published 2026-09-17) through
// the generated internal/iso4217 table. Lookup accepts three ASCII letters in
// any case and returns the canonical uppercase currency. Currencies whose minor
// units are published as N.A. remain discoverable but are not ratable; monetary
// operations reject them with ErrCurrencyNotRatable.
//
// GetPrecision and RoundToPrecision are deprecated compatibility wrappers around
// the registry.
package currency
