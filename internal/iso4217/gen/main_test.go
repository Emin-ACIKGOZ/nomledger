// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	realListOne = "../list-one.xml"
	realListTwo = "../list-two.txt"
	realTable   = "../table_gen.go"

	expectedPublished        = "2026-09-17"
	expectedListOneSHA       = "33139b438657d1cee116ba737807ea71d19d6de4b90f799a09c56f0cc6a1b0ff"
	expectedListTwoURL       = "https://www.six-group.com/dam/download/financial-information/data-center/iso-currrency/lists/list-two.doc"
	expectedListTwoSHA       = "2c59a66ec930fd72332b7aebb6f5d940205497ca4ab39c6d34a076af2d70468c"
	expectedRetrieved        = "2026-11-04"
	expectedFundLine         = "XAD BOV CLF COU MXV CHW CHE USN UYI UYW"
	expectedCodeCount        = 178
	expectedRowCount         = 280
	expectedFundCount        = 10
	testFileMode             = os.FileMode(0o600)
	wantMinorZero      int32 = 0
	wantMinorTwo       int32 = 2

	testCodeUSD    = "USD"
	testNumericUSD = "840"
	testNameUSD    = "US Dollar"
	testMinorTwo   = "2"
	testFundXAD    = "XAD"
	errInvalidCode = "invalid alphabetic code"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), testFileMode); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func xmlDocument(published string, rows ...string) string {
	header := `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	if published != "" {
		header += `<ISO_4217 Pblshd="` + published + `">` + "\n"
	} else {
		header += `<ISO_4217>` + "\n"
	}
	return header + "<CcyTbl>\n" + strings.Join(rows, "") + "</CcyTbl>\n</ISO_4217>\n"
}

func xmlRow(code, numeric, name, minor string) string {
	return "<CcyNtry>" +
		"<CtryNm>TESTLAND</CtryNm>" +
		"<CcyNm>" + name + "</CcyNm>" +
		"<Ccy>" + code + "</Ccy>" +
		"<CcyNbr>" + numeric + "</CcyNbr>" +
		"<CcyMnrUnts>" + minor + "</CcyMnrUnts>" +
		"</CcyNtry>\n"
}

// TestRunRealSources exercises the whole generator path against the checked-in
// sources and asserts it reproduces the committed table byte-for-byte.
func TestRunRealSources(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "table_gen.go")
	if err := run(realListOne, realListTwo, outPath); err != nil {
		t.Fatalf("run: %v", err)
	}

	generated, err := os.ReadFile(outPath) //nolint:gosec // test-local temp file
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	checkedIn, err := os.ReadFile(realTable)
	if err != nil {
		t.Fatalf("read checked-in table: %v", err)
	}
	if !bytes.Equal(generated, checkedIn) {
		t.Error("regenerated table differs from checked-in table_gen.go")
	}
}

func TestParseListOneRealSource(t *testing.T) {
	doc, checksum, err := parseListOne(realListOne)
	if err != nil {
		t.Fatalf("parseListOne: %v", err)
	}
	if doc.Published != expectedPublished {
		t.Errorf("Published = %q; want %q", doc.Published, expectedPublished)
	}
	if len(doc.Entries) != expectedRowCount {
		t.Errorf("rows = %d; want %d", len(doc.Entries), expectedRowCount)
	}
	if checksum != expectedListOneSHA {
		t.Errorf("checksum = %q; want %q", checksum, expectedListOneSHA)
	}
}

func TestParseListOneErrors(t *testing.T) {
	tests := []struct {
		name    string
		xml     string
		wantErr string
	}{
		{
			name:    "malformed XML",
			xml:     `<ISO_4217 Pblshd="2026-09-17"><CcyTbl>`,
			wantErr: "parse List One XML",
		},
		{
			name:    "missing publication date",
			xml:     xmlDocument("", xmlRow(testCodeUSD, testNumericUSD, testNameUSD, testMinorTwo)),
			wantErr: "missing ISO_4217 Pblshd attribute",
		},
		{
			name:    "wrong root element",
			xml:     `<WrongRoot Pblshd="2026-09-17"></WrongRoot>`,
			wantErr: "expected element type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, "list-one.xml", tc.xml)
			_, _, err := parseListOne(path)
			assertErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("missing file", func(t *testing.T) {
		_, _, err := parseListOne(filepath.Join(t.TempDir(), "absent.xml"))
		assertErrorContains(t, err, "read List One XML")
	})
}

func TestReadFundsRealSource(t *testing.T) {
	funds, err := readFunds(realListTwo)
	if err != nil {
		t.Fatalf("readFunds: %v", err)
	}
	if got := strings.Join(funds, " "); got != expectedFundLine {
		t.Errorf("funds = %q; want %q", got, expectedFundLine)
	}
	if len(funds) != expectedFundCount {
		t.Errorf("fund count = %d; want %d", len(funds), expectedFundCount)
	}
}

func TestReadFundsErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "empty file", content: "", wantErr: "no fund codes found"},
		{name: "comments only", content: "# no codes here\n\n", wantErr: "no fund codes found"},
		{name: "multiple code lines", content: "XAD BOV\nCLF COU\n", wantErr: "multiple fund-code lines"},
		{name: "invalid code", content: "usd\n", wantErr: errInvalidCode},
		{name: "short code", content: "US\n", wantErr: errInvalidCode},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, "list-two.txt", tc.content)
			_, err := readFunds(path)
			assertErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("missing file", func(t *testing.T) {
		_, err := readFunds(filepath.Join(t.TempDir(), "absent.txt"))
		assertErrorContains(t, err, "read List Two fund codes")
	})
}

func TestFundSetValidation(t *testing.T) {
	t.Run("accepts valid codes", func(t *testing.T) {
		set, err := fundSet([]string{testFundXAD, "BOV"})
		if err != nil {
			t.Fatalf("fundSet: %v", err)
		}
		if len(set) != 2 {
			t.Errorf("set size = %d; want 2", len(set))
		}
	})

	tests := []struct {
		name    string
		funds   []string
		wantErr string
	}{
		{name: "duplicate", funds: []string{testFundXAD, testFundXAD}, wantErr: "duplicate fund code XAD"},
		{name: "invalid code", funds: []string{"usd"}, wantErr: errInvalidCode},
		{name: "empty code list is not rejected here", funds: nil, wantErr: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fundSet(tc.funds)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("fundSet: unexpected error %v", err)
				}
				return
			}
			assertErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestBuildTableRealSource(t *testing.T) {
	doc, _, err := parseListOne(realListOne)
	if err != nil {
		t.Fatalf("parseListOne: %v", err)
	}
	funds, err := readFunds(realListTwo)
	if err != nil {
		t.Fatalf("readFunds: %v", err)
	}

	entries, err := buildTable(doc.Entries, funds)
	if err != nil {
		t.Fatalf("buildTable: %v", err)
	}
	if len(entries) != expectedCodeCount {
		t.Fatalf("entries = %d; want %d", len(entries), expectedCodeCount)
	}

	fundCount := 0
	for i, e := range entries {
		if i > 0 && entries[i-1].Code >= e.Code {
			t.Errorf("entries not strictly sorted: %s before %s", entries[i-1].Code, e.Code)
		}
		if e.Fund {
			fundCount++
		}
	}
	if fundCount != expectedFundCount {
		t.Errorf("fund entries = %d; want %d", fundCount, expectedFundCount)
	}
}

func TestBuildTableValidation(t *testing.T) {
	validRow := xmlEntry{Code: testCodeUSD, Numeric: testNumericUSD, Name: testNameUSD, MinorUnits: testMinorTwo}

	tests := []struct {
		name    string
		rows    []xmlEntry
		funds   []string
		wantErr string
	}{
		{
			name:    "empty table",
			rows:    nil,
			wantErr: "no currency entries found",
		},
		{
			name:    "only no-currency rows",
			rows:    []xmlEntry{{Name: noCurrencyName}},
			wantErr: "no currency entries found",
		},
		{
			name: "inconsistent duplicate metadata",
			rows: []xmlEntry{
				validRow,
				{Code: testCodeUSD, Numeric: "841", Name: testNameUSD, MinorUnits: testMinorTwo},
			},
			wantErr: "inconsistent duplicate metadata",
		},
		{
			name:    "fund absent from List One",
			rows:    []xmlEntry{validRow},
			funds:   []string{testFundXAD},
			wantErr: "list-two fund code XAD is absent from List One",
		},
		{
			name:    "duplicate fund code",
			rows:    []xmlEntry{validRow},
			funds:   []string{testFundXAD, testFundXAD},
			wantErr: "duplicate fund code XAD",
		},
		{
			name:    "unknown malformed fund code",
			rows:    []xmlEntry{validRow},
			funds:   []string{"ZZ"},
			wantErr: errInvalidCode,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTable(tc.rows, tc.funds)
			assertErrorContains(t, err, tc.wantErr)
		})
	}

	t.Run("consistent duplicate metadata is deduplicated", func(t *testing.T) {
		entries, err := buildTable([]xmlEntry{validRow, validRow}, nil)
		if err != nil {
			t.Fatalf("buildTable: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("entries = %d; want 1", len(entries))
		}
	})
}

func TestMakeEntryValidation(t *testing.T) {
	tests := []struct {
		name     string
		row      xmlEntry
		funds    map[string]struct{}
		wantKeep bool
		want     entry
		wantErr  string
	}{
		{
			name:     "valid non-fund entry",
			row:      xmlEntry{Code: testCodeUSD, Numeric: testNumericUSD, Name: testNameUSD, MinorUnits: testMinorTwo},
			wantKeep: true,
			want: entry{
				Code: testCodeUSD, Numeric: testNumericUSD, Name: testNameUSD,
				MinorUnits: wantMinorTwo, Ratable: true, Fund: false,
			},
		},
		{
			name:     "valid fund entry",
			row:      xmlEntry{Code: testFundXAD, Numeric: "396", Name: "Arab Accounting Dinar", MinorUnits: testMinorTwo},
			funds:    map[string]struct{}{testFundXAD: {}},
			wantKeep: true,
			want: entry{
				Code: testFundXAD, Numeric: "396", Name: "Arab Accounting Dinar",
				MinorUnits: wantMinorTwo, Ratable: true, Fund: true,
			},
		},
		{
			name:     "N.A. minor units are non-ratable",
			row:      xmlEntry{Code: "XAU", Numeric: "959", Name: "Gold", MinorUnits: "N.A."},
			wantKeep: true,
			want: entry{
				Code: "XAU", Numeric: "959", Name: "Gold",
				MinorUnits: wantMinorZero, Ratable: false, Fund: false,
			},
		},
		{
			name:     "no-universal-currency row is skipped",
			row:      xmlEntry{Name: noCurrencyName},
			wantKeep: false,
		},
		{
			name:    "empty code with other name",
			row:     xmlEntry{Name: "Euro", Numeric: "", MinorUnits: ""},
			wantErr: "empty Ccy is only allowed for no-universal-currency rows",
		},
		{
			name:    "no-universal-currency row with numeric data",
			row:     xmlEntry{Name: noCurrencyName, Numeric: testNumericUSD},
			wantErr: "no-universal-currency row carries currency data",
		},
		{
			name:    "no-universal-currency row with minor units",
			row:     xmlEntry{Name: noCurrencyName, MinorUnits: testMinorTwo},
			wantErr: "no-universal-currency row carries currency data",
		},
		{
			name:    "lowercase code",
			row:     xmlEntry{Code: "usd", Numeric: testNumericUSD, Name: testNameUSD, MinorUnits: testMinorTwo},
			wantErr: errInvalidCode,
		},
		{
			name:    "short code",
			row:     xmlEntry{Code: "US", Numeric: testNumericUSD, Name: testNameUSD, MinorUnits: testMinorTwo},
			wantErr: errInvalidCode,
		},
		{
			name:    "missing name",
			row:     xmlEntry{Code: testCodeUSD, Numeric: testNumericUSD, MinorUnits: testMinorTwo},
			wantErr: "missing CcyNm",
		},
		{
			name:    "short numeric",
			row:     xmlEntry{Code: testCodeUSD, Numeric: "84", Name: testNameUSD, MinorUnits: testMinorTwo},
			wantErr: "invalid numeric code",
		},
		{
			name:    "non-digit numeric",
			row:     xmlEntry{Code: testCodeUSD, Numeric: "84A", Name: testNameUSD, MinorUnits: testMinorTwo},
			wantErr: "invalid numeric code",
		},
		{
			name:    "unsupported minor units",
			row:     xmlEntry{Code: testCodeUSD, Numeric: testNumericUSD, Name: testNameUSD, MinorUnits: "1"},
			wantErr: "invalid CcyMnrUnts",
		},
		{
			name:    "empty minor units",
			row:     xmlEntry{Code: testCodeUSD, Numeric: testNumericUSD, Name: testNameUSD, MinorUnits: ""},
			wantErr: "invalid CcyMnrUnts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, keep, err := makeEntry(tc.row, tc.funds)
			if tc.wantErr != "" {
				assertErrorContains(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("makeEntry: unexpected error %v", err)
			}
			if keep != tc.wantKeep {
				t.Fatalf("keep = %t; want %t", keep, tc.wantKeep)
			}
			if keep && got != tc.want {
				t.Errorf("entry = %+v; want %+v", got, tc.want)
			}
		})
	}
}

func TestRenderDeterministic(t *testing.T) {
	doc, checksum, err := parseListOne(realListOne)
	if err != nil {
		t.Fatalf("parseListOne: %v", err)
	}
	funds, err := readFunds(realListTwo)
	if err != nil {
		t.Fatalf("readFunds: %v", err)
	}
	entries, err := buildTable(doc.Entries, funds)
	if err != nil {
		t.Fatalf("buildTable: %v", err)
	}

	first, err := render(doc.Published, checksum, funds, entries)
	if err != nil {
		t.Fatalf("render first: %v", err)
	}
	second, err := render(doc.Published, checksum, funds, entries)
	if err != nil {
		t.Fatalf("render second: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("render is not deterministic for identical inputs")
	}

	assertValidGo(t, first)
	assertRenderedHeader(t, string(first), doc.Published, checksum, funds)
}

func assertValidGo(t *testing.T, src []byte) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), "table_gen.go", src, parser.AllErrors); err != nil {
		t.Errorf("rendered output is not valid Go: %v", err)
	}
}

func assertRenderedHeader(t *testing.T, text, published, checksum string, funds []string) {
	t.Helper()
	if !strings.HasPrefix(text, "// SPDX-License-Identifier: MIT\n// Code generated by internal/iso4217/gen; DO NOT EDIT.\n") {
		t.Error("rendered header is missing SPDX or generated marker")
	}
	for _, want := range []string{
		sourceURL,
		published,
		checksum,
		expectedListTwoURL,
		expectedListTwoSHA,
		expectedRetrieved,
		"Fund codes: " + strings.Join(funds, " "),
		"go generate ./...",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered header is missing %q", want)
		}
	}
	if strings.Contains(text, "lists/list-two.xml") {
		t.Error("rendered header still references the stale List Two XML URL")
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.go")
	want := []byte("package example\n")
	if err := writeFile(path, want); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // test-local temp file
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("written bytes = %q; want %q", got, want)
	}

	t.Run("unwritable directory", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "absent", "out.go")
		assertErrorContains(t, writeFile(missing, want), "write")
	})
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q; got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q; want it to contain %q", err, want)
	}
}
