// SPDX-License-Identifier: MIT

// Package iso4217 holds the generated ISO 4217 currency table and its source data.
//
// The table is generated from the SIX List One XML checked in as list-one.xml.
// Fund flags are derived from the SIX List Two fund-code register checked in as
// list-two.txt, because List One does not carry a fund flag.
package iso4217

//go:generate go run ./gen -xml list-one.xml -out table_gen.go

// Entry describes one active ISO 4217 currency.
type Entry struct {
	Code       string
	Numeric    string
	Name       string
	MinorUnits int32
	Ratable    bool
	Fund       bool
}

// Entries returns a defensive copy of the generated currency table.
func Entries() []Entry {
	return append([]Entry(nil), table...)
}
