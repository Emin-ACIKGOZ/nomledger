// SPDX-License-Identifier: MIT

package currency_test

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
	pkgerr "github.com/Emin-ACIKGOZ/nomledger/pkg/errors"

	"github.com/shopspring/decimal"
)

const (
	listOnePath = "../../internal/iso4217/list-one.xml"
	listTwoPath = "../../internal/iso4217/list-two.txt"

	expectedPublished  = "2026-09-17"
	expectedCodeCount  = 178
	expectedRowCount   = 280
	expectedEmptyRows  = 3
	expectedNonRatable = 13
	expectedFundCount  = 10

	codeLength    = 3
	numericLength = 3
	minorZero     = int32(0)
	minorTwo      = int32(2)

	codeUSD          = "USD"
	codeUSDMixedCase = "UsD"
	codeJPY          = "JPY"
	codeBHD          = "BHD"
	codeCLF          = "CLF"
	codeXAU          = "XAU"
	codeXDR          = "XDR"
	codeBGN          = "BGN"
	codeUnknown      = "ABC"
)

var expectedMinorCounts = map[int32]int{0: 17, 2: 139, 3: 7, 4: 2}

type sourceXML struct {
	XMLName   xml.Name    `xml:"ISO_4217"`
	Published string      `xml:"Pblshd,attr"`
	Rows      []sourceRow `xml:"CcyTbl>CcyNtry"`
}

type sourceRow struct {
	Country    string `xml:"CtryNm"`
	Name       string `xml:"CcyNm"`
	Code       string `xml:"Ccy"`
	Numeric    string `xml:"CcyNbr"`
	MinorUnits string `xml:"CcyMnrUnts"`
}

type sourceEntry struct {
	code       string
	numeric    string
	name       string
	minorUnits int32
	ratable    bool
}

func readSourceXML(t *testing.T) sourceXML {
	t.Helper()
	data, err := os.ReadFile(listOnePath)
	if err != nil {
		t.Fatalf("read %s: %v", listOnePath, err)
	}
	var doc sourceXML
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", listOnePath, err)
	}
	return doc
}

// expectedEntries derives the expected registry from the checked-in SIX List
// One XML without using internal/iso4217 as the oracle.
func expectedEntries(t *testing.T) map[string]sourceEntry {
	t.Helper()
	doc := readSourceXML(t)
	if doc.Published != expectedPublished {
		t.Errorf("XML Pblshd = %q; want %q", doc.Published, expectedPublished)
	}

	entries := make(map[string]sourceEntry, expectedCodeCount)
	emptyRows := 0
	for i, row := range doc.Rows {
		if row.Code == "" {
			emptyRows++
			continue
		}
		want, err := sourceEntryFromRow(row)
		if err != nil {
			t.Fatalf("row %d (%s, %s): %v", i, row.Country, row.Code, err)
		}
		if prev, ok := entries[row.Code]; ok {
			if prev != want {
				t.Fatalf("inconsistent duplicate XML rows for %s: %+v != %+v", row.Code, prev, want)
			}
			continue
		}
		entries[row.Code] = want
	}

	assertSourceCounts(t, doc.Rows, emptyRows, entries)
	return entries
}

func sourceEntryFromRow(row sourceRow) (sourceEntry, error) {
	minorUnits, ratable, err := parseSourceMinorUnits(row.MinorUnits)
	if err != nil {
		return sourceEntry{}, err
	}
	return sourceEntry{
		code:       row.Code,
		numeric:    row.Numeric,
		name:       row.Name,
		minorUnits: minorUnits,
		ratable:    ratable,
	}, nil
}

func parseSourceMinorUnits(raw string) (int32, bool, error) {
	if raw == "N.A." {
		return minorZero, false, nil
	}
	switch raw {
	case "0":
		return minorZero, true, nil
	case "2":
		return minorTwo, true, nil
	case "3":
		return 3, true, nil
	case "4":
		return 4, true, nil
	default:
		return 0, false, fmt.Errorf("unexpected CcyMnrUnts value %q", raw)
	}
}

func assertSourceCounts(t *testing.T, rows []sourceRow, emptyRows int, entries map[string]sourceEntry) {
	t.Helper()
	if len(rows) != expectedRowCount {
		t.Errorf("XML rows = %d; want %d", len(rows), expectedRowCount)
	}
	if emptyRows != expectedEmptyRows {
		t.Errorf("XML rows with empty Ccy = %d; want %d", emptyRows, expectedEmptyRows)
	}
	if len(entries) != expectedCodeCount {
		t.Fatalf("XML unique codes = %d; want %d", len(entries), expectedCodeCount)
	}

	minorCounts := map[int32]int{}
	nonRatable := 0
	for _, entry := range entries {
		if entry.ratable {
			minorCounts[entry.minorUnits]++
		} else {
			nonRatable++
		}
	}
	assertMinorCounts(t, minorCounts, nonRatable)
}

func assertMinorCounts(t *testing.T, minorCounts map[int32]int, nonRatable int) {
	t.Helper()
	for precision, want := range expectedMinorCounts {
		if got := minorCounts[precision]; got != want {
			t.Errorf("ratable codes with %d minor units = %d; want %d", precision, got, want)
		}
	}
	if len(minorCounts) != len(expectedMinorCounts) {
		t.Errorf("ratable minor-unit values = %v; want exactly %v", minorCounts, expectedMinorCounts)
	}
	if nonRatable != expectedNonRatable {
		t.Errorf("non-ratable codes = %d; want %d", nonRatable, expectedNonRatable)
	}
}

// parseFunds independently parses the SIX List Two fund codes from list-two.txt.
func parseFunds(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(listTwoPath)
	if err != nil {
		t.Fatalf("read %s: %v", listTwoPath, err)
	}

	funds := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, code := range strings.Fields(line) {
			funds[code] = true
		}
	}
	if len(funds) != expectedFundCount {
		t.Fatalf("List Two fund codes = %d; want %d", len(funds), expectedFundCount)
	}
	return funds
}

// TestAllMatchesXML: exact set equality between All() and the XML.
func TestAllMatchesXML(t *testing.T) {
	wantByCode := expectedEntries(t)

	all := currency.All()
	if len(all) != expectedCodeCount {
		t.Fatalf("All() returned %d codes; want %d", len(all), expectedCodeCount)
	}

	seen := make(map[string]bool, len(all))
	for _, c := range all {
		code := string(c.Code())
		if seen[code] {
			t.Errorf("All() contains duplicate code %s", code)
		}
		seen[code] = true
		if _, ok := wantByCode[code]; !ok {
			t.Errorf("All() contains %s which is absent from the XML", code)
		}
	}
	for code := range wantByCode {
		if !seen[code] {
			t.Errorf("All() is missing XML code %s", code)
		}
	}
}

// TestCurrencyFieldsMatchXML: every field is compared to the source.
func TestCurrencyFieldsMatchXML(t *testing.T) {
	wantByCode := expectedEntries(t)
	funds := parseFunds(t)

	all := currency.All()
	for _, c := range all {
		code := string(c.Code())
		t.Run(code, func(t *testing.T) {
			want, ok := wantByCode[code]
			if !ok {
				t.Fatalf("%s is absent from the XML", code)
			}
			assertCurrencyFields(t, c, want, funds[code])
		})
	}

	assertRegistryCounts(t, all)
}

func assertCurrencyFields(t *testing.T, got currency.Currency, want sourceEntry, wantFund bool) {
	t.Helper()
	if string(got.Code()) != want.code {
		t.Errorf("Code() = %q; want %q", got.Code(), want.code)
	}
	if got.Numeric() != want.numeric {
		t.Errorf("Numeric() = %q; want %q", got.Numeric(), want.numeric)
	}
	if got.Name() != want.name {
		t.Errorf("Name() = %q; want %q", got.Name(), want.name)
	}
	minorUnits, ratable := got.MinorUnits()
	if minorUnits != want.minorUnits || ratable != want.ratable {
		t.Errorf("MinorUnits() = (%d, %t); want (%d, %t)",
			minorUnits, ratable, want.minorUnits, want.ratable)
	}
	if got.IsFund() != wantFund {
		t.Errorf("IsFund() = %t; want %t (List Two)", got.IsFund(), wantFund)
	}
}

func assertRegistryCounts(t *testing.T, all []currency.Currency) {
	t.Helper()
	minorCounts := map[int32]int{}
	nonRatable := 0
	for _, c := range all {
		minorUnits, ratable := c.MinorUnits()
		if ratable {
			minorCounts[minorUnits]++
		} else {
			nonRatable++
		}
	}
	assertMinorCounts(t, minorCounts, nonRatable)
}

// TestLookupNormalization: case folding plus strict malformed input.
func TestLookupNormalization(t *testing.T) {
	t.Run("normalizes case", func(t *testing.T) {
		for _, input := range []string{codeUSDLower, codeUSDMixedCase, codeUSD} {
			c, err := currency.Lookup(input)
			if err != nil {
				t.Fatalf("Lookup(%q): %v", input, err)
			}
			if c.Code() != codeUSD {
				t.Errorf("Lookup(%q).Code() = %q; want %q", input, c.Code(), codeUSD)
			}
		}
	})

	t.Run("rejects malformed", func(t *testing.T) {
		for _, input := range []string{" USD", "US", "БГЛ", "US1", ""} {
			assertLookupError(t, input, pkgerr.ErrInvalidCurrencyCode)
		}
	})

	t.Run("rejects unknown well-formed code", func(t *testing.T) {
		assertLookupError(t, codeUnknown, pkgerr.ErrUnknownCurrency)
	})
}

func assertLookupError(t *testing.T, input string, want error) {
	t.Helper()
	_, err := currency.Lookup(input)
	if !errors.Is(err, want) {
		t.Errorf("Lookup(%q) error = %v; want %v", input, err, want)
	}
}

// TestNotRatable: N.A. currencies are discoverable but not usable for
// monetary amounts.
func TestNotRatable(t *testing.T) {
	xau, err := currency.Lookup(codeXAU)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", codeXAU, err)
	}
	assertNotRatable(t, xau, codeXAU)
	if xau.IsFund() {
		t.Errorf("%s IsFund() = true; want false", codeXAU)
	}

	usd, err := currency.Lookup(codeUSD)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", codeUSD, err)
	}
	minorUnits, ratable := usd.MinorUnits()
	if minorUnits != minorTwo || !ratable {
		t.Errorf("%s MinorUnits() = (%d, %t); want (%d, true)", codeUSD, minorUnits, ratable, minorTwo)
	}

	assertAllNotRatableRejected(t)
}

func assertNotRatable(t *testing.T, c currency.Currency, code string) {
	t.Helper()
	minorUnits, ratable := c.MinorUnits()
	if minorUnits != minorZero || ratable {
		t.Errorf("%s MinorUnits() = (%d, %t); want (%d, false)", code, minorUnits, ratable, minorZero)
	}
	if _, err := currency.RoundToPrecision(decimal.NewFromInt(1), code); !errors.Is(err, pkgerr.ErrCurrencyNotRatable) {
		t.Errorf("RoundToPrecision(1, %s) error = %v; want %v", code, err, pkgerr.ErrCurrencyNotRatable)
	}
}

func assertAllNotRatableRejected(t *testing.T) {
	t.Helper()
	for code, want := range expectedEntries(t) {
		if want.ratable {
			continue
		}
		t.Run(code, func(t *testing.T) {
			c, err := currency.Lookup(code)
			if err != nil {
				t.Fatalf("Lookup(%s): %v", code, err)
			}
			assertNotRatable(t, c, code)
		})
	}
}

// TestAllReturnsCopy: All() must expose no mutable registry state.
func TestAllReturnsCopy(t *testing.T) {
	first := currency.All()
	if len(first) != expectedCodeCount {
		t.Fatalf("All() returned %d codes; want %d", len(first), expectedCodeCount)
	}
	snapshot := append([]currency.Currency(nil), first...)

	first[0] = currency.Currency{}
	extended := append(currency.All(), first[0])
	if len(extended) != expectedCodeCount+1 {
		t.Errorf("appended slice length = %d; want %d", len(extended), expectedCodeCount+1)
	}

	second := currency.All()
	if !reflect.DeepEqual(second, snapshot) {
		t.Error("mutating the slice returned by All() changed a subsequent All()")
	}
}

// TestCanonicalCodes: every stored code is canonical and numeric codes
// preserve their published zero padding.
func TestCanonicalCodes(t *testing.T) {
	for _, c := range currency.All() {
		code := string(c.Code())
		if !isThreeASCIILetters(code) {
			t.Errorf("All() code %q is not exactly three ASCII letters", code)
		}
		if code != strings.ToUpper(code) {
			t.Errorf("All() code %q is not uppercase", code)
		}
		if !isThreeDigits(c.Numeric()) {
			t.Errorf("%s Numeric() = %q; want three digits", code, c.Numeric())
		}
	}

	zeroPadded := map[string]string{
		"ALL":   "008",
		"AMD":   "051",
		"AUD":   "036",
		"BBD":   "052",
		codeBHD: "048",
		"BDT":   "050",
	}
	for code, wantNumeric := range zeroPadded {
		c, err := currency.Lookup(code)
		if err != nil {
			t.Fatalf("Lookup(%s): %v", code, err)
		}
		if c.Numeric() != wantNumeric {
			t.Errorf("%s Numeric() = %q; want %q", code, c.Numeric(), wantNumeric)
		}
	}
}

func isThreeASCIILetters(code string) bool {
	if len(code) != codeLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		b := code[i]
		if (b < 'A' || b > 'Z') && (b < 'a' || b > 'z') {
			return false
		}
	}
	return true
}

func isThreeDigits(numeric string) bool {
	if len(numeric) != numericLength {
		return false
	}
	for i := 0; i < len(numeric); i++ {
		if numeric[i] < '0' || numeric[i] > '9' {
			return false
		}
	}
	return true
}

// TestLookupDoesNotEnforceRatability verifies that discovery and ratability are
// separate concerns, and that a historical code is unknown in the active list.
func TestLookupDoesNotEnforceRatability(t *testing.T) {
	for _, code := range []string{codeXAU, codeXDR, "XTS", "XXX"} {
		c, err := currency.Lookup(code)
		if err != nil {
			t.Errorf("Lookup(%s) = error %v; want success", code, err)
			continue
		}
		if _, ratable := c.MinorUnits(); ratable {
			t.Errorf("%s MinorUnits() ratable = true; want false", code)
		}
	}

	if _, err := currency.Lookup(codeBGN); !errors.Is(err, pkgerr.ErrUnknownCurrency) {
		t.Errorf("Lookup(%s) error = %v; want %v", codeBGN, err, pkgerr.ErrUnknownCurrency)
	}
}

func TestGetPrecision(t *testing.T) {
	cases := []struct {
		code     string
		expected int32
		found    bool
	}{
		{codeUSD, minorTwo, true},
		{codeUSDMixedCase, minorTwo, true},
		{codeJPY, 0, true},
		{"US", 0, false},
		{codeUnknown, 0, false},
		{codeXAU, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			got, ok := currency.GetPrecision(tc.code)
			if ok != tc.found || (ok && got != tc.expected) {
				t.Errorf("GetPrecision(%s) = %d, %v; want %d, %v", tc.code, got, ok, tc.expected, tc.found)
			}
		})
	}
}

func TestRoundToPrecision_ErrorTaxonomy(t *testing.T) {
	cases := []struct {
		name string
		code string
		want error
	}{
		{"malformed code", "UNKNOWN", pkgerr.ErrInvalidCurrencyCode},
		{"unknown well-formed code", codeUnknown, pkgerr.ErrUnknownCurrency},
		{"non-ratable code", codeXAU, pkgerr.ErrCurrencyNotRatable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := currency.RoundToPrecision(decimal.NewFromInt(1), tc.code)
			if !errors.Is(err, tc.want) {
				t.Errorf("RoundToPrecision(1, %s) error = %v; want %v", tc.code, err, tc.want)
			}
		})
	}
}

func TestRoundToPrecision(t *testing.T) {
	cases := []struct {
		name     string
		amount   string
		currency string
		expected string
	}{
		// Standard Rounding (USD - 2 decimals)
		{"USD Normal Down", "10.123", codeUSD, "10.12"},
		{"USD Normal Up", "10.126", codeUSD, "10.13"},

		// Banker's Rounding (Half to Even)
		{"USD Half Down to Even", "10.125", codeUSD, "10.12"},
		{"USD Half Up to Even", "10.135", codeUSD, "10.14"},

		// Zero Precision (JPY)
		{"JPY Normal", "100.1", codeJPY, "100"},
		{"JPY Half Down", "100.5", codeJPY, "100"},
		{"JPY Half Up", "101.5", codeJPY, "102"},

		// High Precision (BHD - 3 decimals)
		{"BHD Precision", "10.1234", codeBHD, "10.123"},
		{"BHD Rounding", "10.1235", codeBHD, "10.124"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input, err := decimal.NewFromString(tc.amount)
			if err != nil {
				t.Fatalf("Invalid input amount in test case: %s", tc.amount)
			}
			want, err := decimal.NewFromString(tc.expected)
			if err != nil {
				t.Fatalf("Invalid expected amount in test case: %s", tc.expected)
			}

			got, err := currency.RoundToPrecision(input, tc.currency)
			if err != nil {
				t.Errorf("RoundToPrecision(%s, %s) returned unexpected error: %v", tc.amount, tc.currency, err)
				return
			}

			if !got.Equal(want) {
				t.Errorf("RoundToPrecision(%s, %s) = %s; want %s",
					tc.amount, tc.currency, got.String(), want.String())
			}
		})
	}
}
