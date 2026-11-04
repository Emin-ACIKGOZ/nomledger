// SPDX-License-Identifier: MIT

// Package main implements the ISO 4217 table generator.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"sort"
	"strings"
)

const (
	sourceURL        = "https://www.six-group.com/dam/download/financial-information/data-center/iso-currrency/lists/list-one.xml"
	listTwoURL       = "https://www.six-group.com/dam/download/financial-information/data-center/iso-currrency/lists/list-two.doc"
	listTwoSHA256    = "2c59a66ec930fd72332b7aebb6f5d940205497ca4ab39c6d34a076af2d70468c"
	listTwoRetrieved = "2026-11-04"

	isoCodeLength     = 3
	numericCodeLength = 3

	minorUnitsZero  = 0
	minorUnitsTwo   = 2
	minorUnitsThree = 3
	minorUnitsFour  = 4

	noCurrencyName    = "No universal currency"
	generatedFileMode = os.FileMode(0o644)
)

type iso4217 struct {
	XMLName   xml.Name   `xml:"ISO_4217"`
	Published string     `xml:"Pblshd,attr"`
	Entries   []xmlEntry `xml:"CcyTbl>CcyNtry"`
}

type xmlEntry struct {
	Country    string `xml:"CtryNm"`
	Name       string `xml:"CcyNm"`
	Code       string `xml:"Ccy"`
	Numeric    string `xml:"CcyNbr"`
	MinorUnits string `xml:"CcyMnrUnts"`
}

type entry struct {
	Code       string
	Numeric    string
	Name       string
	MinorUnits int32
	Ratable    bool
	Fund       bool
}

func main() {
	xmlPath := flag.String("xml", "list-one.xml", "path to SIX List One XML")
	fundsPath := flag.String("funds", "list-two.txt", "path to derived List Two fund codes")
	outPath := flag.String("out", "table_gen.go", "path to generated Go file")
	flag.Parse()

	if err := run(*xmlPath, *fundsPath, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(xmlPath, fundsPath, outPath string) error {
	doc, checksum, err := parseListOne(xmlPath)
	if err != nil {
		return err
	}

	funds, err := readFunds(fundsPath)
	if err != nil {
		return err
	}

	entries, err := buildTable(doc.Entries, funds)
	if err != nil {
		return err
	}

	src, err := render(doc.Published, checksum, funds, entries)
	if err != nil {
		return err
	}

	return writeFile(outPath, src)
}

// parseListOne reads and parses the SIX List One XML, returning its contents
// and the SHA-256 of the exact bytes read.
func parseListOne(path string) (iso4217, string, error) {
	//nolint:gosec // path is the generator's own local source file
	data, err := os.ReadFile(path)
	if err != nil {
		return iso4217{}, "", fmt.Errorf("read List One XML: %w", err)
	}

	var doc iso4217
	if err := xml.Unmarshal(data, &doc); err != nil {
		return iso4217{}, "", fmt.Errorf("parse List One XML: %w", err)
	}
	if doc.Published == "" {
		return iso4217{}, "", errors.New("missing ISO_4217 Pblshd attribute")
	}

	sum := sha256.Sum256(data)
	return doc, hex.EncodeToString(sum[:]), nil
}

// readFunds reads the derived List Two fund-code file. Comment lines begin
// with '#' and the remaining line contains the space-separated codes.
func readFunds(path string) ([]string, error) {
	//nolint:gosec // path is the generator's own local source file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read List Two fund codes: %w", err)
	}

	var funds []string
	foundData := false
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if foundData {
			return nil, fmt.Errorf("line %d: multiple fund-code lines", i+1)
		}
		foundData = true

		for _, code := range strings.Fields(line) {
			if err := validateCode(code); err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			funds = append(funds, code)
		}
	}

	if len(funds) == 0 {
		return nil, errors.New("no fund codes found")
	}
	return funds, nil
}

// buildTable deduplicates List One rows and attaches fund flags from List Two.
func buildTable(rows []xmlEntry, funds []string) ([]entry, error) {
	set, err := fundSet(funds)
	if err != nil {
		return nil, err
	}

	seen, err := collectEntries(rows, set)
	if err != nil {
		return nil, err
	}
	if len(seen) == 0 {
		return nil, errors.New("no currency entries found")
	}
	if err := checkFundsPresent(funds, seen); err != nil {
		return nil, err
	}

	return sortedEntries(seen), nil
}

func fundSet(funds []string) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(funds))
	for _, code := range funds {
		if err := validateCode(code); err != nil {
			return nil, fmt.Errorf("fund code: %w", err)
		}
		if _, exists := set[code]; exists {
			return nil, fmt.Errorf("duplicate fund code %s", code)
		}
		set[code] = struct{}{}
	}
	return set, nil
}

func collectEntries(rows []xmlEntry, funds map[string]struct{}) (map[string]entry, error) {
	seen := make(map[string]entry, len(rows))
	for i, row := range rows {
		got, keep, err := makeEntry(row, funds)
		if err != nil {
			return nil, fmt.Errorf("row %d (%s): %w", i, row.Country, err)
		}
		if !keep {
			continue
		}

		prev, exists := seen[got.Code]
		if exists && prev != got {
			return nil, fmt.Errorf("inconsistent duplicate metadata for %s: %+v != %+v", got.Code, prev, got)
		}
		if !exists {
			seen[got.Code] = got
		}
	}
	return seen, nil
}

func makeEntry(row xmlEntry, funds map[string]struct{}) (entry, bool, error) {
	if row.Code == "" {
		if row.Name != noCurrencyName {
			return entry{}, false, errors.New("empty Ccy is only allowed for no-universal-currency rows")
		}
		if row.Numeric != "" || row.MinorUnits != "" {
			return entry{}, false, errors.New("no-universal-currency row carries currency data")
		}
		return entry{}, false, nil
	}

	if err := validateCode(row.Code); err != nil {
		return entry{}, false, err
	}
	if row.Name == "" {
		return entry{}, false, errors.New("missing CcyNm")
	}
	if err := validateNumeric(row.Numeric); err != nil {
		return entry{}, false, err
	}
	minor, ratable, err := parseMinorUnits(row.MinorUnits)
	if err != nil {
		return entry{}, false, err
	}

	_, fund := funds[row.Code]
	return entry{
		Code:       row.Code,
		Numeric:    row.Numeric,
		Name:       row.Name,
		MinorUnits: minor,
		Ratable:    ratable,
		Fund:       fund,
	}, true, nil
}

func checkFundsPresent(funds []string, entries map[string]entry) error {
	for _, code := range funds {
		if _, exists := entries[code]; !exists {
			return fmt.Errorf("list-two fund code %s is absent from List One", code)
		}
	}
	return nil
}

func sortedEntries(seen map[string]entry) []entry {
	entries := make([]entry, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Code < entries[j].Code
	})
	return entries
}

func validateCode(code string) error {
	if len(code) != isoCodeLength {
		return fmt.Errorf("invalid alphabetic code %q: want %d uppercase ASCII letters", code, isoCodeLength)
	}
	for i := 0; i < len(code); i++ {
		if code[i] < 'A' || code[i] > 'Z' {
			return fmt.Errorf("invalid alphabetic code %q: want uppercase ASCII letters", code)
		}
	}
	return nil
}

func validateNumeric(numeric string) error {
	if len(numeric) != numericCodeLength {
		return fmt.Errorf("invalid numeric code %q: want %d digits", numeric, numericCodeLength)
	}
	for i := 0; i < len(numeric); i++ {
		if numeric[i] < '0' || numeric[i] > '9' {
			return fmt.Errorf("invalid numeric code %q: want %d digits", numeric, numericCodeLength)
		}
	}
	return nil
}

func parseMinorUnits(raw string) (int32, bool, error) {
	switch raw {
	case "N.A.":
		return minorUnitsZero, false, nil
	case "0":
		return minorUnitsZero, true, nil
	case "2":
		return minorUnitsTwo, true, nil
	case "3":
		return minorUnitsThree, true, nil
	case "4":
		return minorUnitsFour, true, nil
	default:
		return minorUnitsZero, false, fmt.Errorf("invalid CcyMnrUnts %q: want N.A., 0, 2, 3, or 4", raw)
	}
}

func render(published, checksum string, funds []string, entries []entry) ([]byte, error) {
	lines := []string{
		"// SPDX-License-Identifier: MIT",
		"// Code generated by internal/iso4217/gen; DO NOT EDIT.",
		"",
		"// Source: SIX ISO 4217 List One: Current Currency & Funds",
		"// URL: " + sourceURL,
		"// Published: " + published,
		"// SHA-256: " + checksum,
		"//",
		"// Fund flags are derived from SIX ISO 4217 List Two (funds codes registered",
		"// with the Maintenance Agency), not from List One.",
		"// List Two URL: " + listTwoURL,
		"// List Two SHA-256: " + listTwoSHA256,
		"// List Two retrieved: " + listTwoRetrieved,
		"// Fund codes: " + strings.Join(funds, " "),
		"//",
		"// Regenerate with: go generate ./...",
		"",
		"package iso4217",
		"",
		"var table = []Entry{",
	}
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf(
			"\t{Code: %q, Numeric: %q, Name: %q, MinorUnits: %d, Ratable: %t, Fund: %t},",
			e.Code, e.Numeric, e.Name, e.MinorUnits, e.Ratable, e.Fund,
		))
	}
	lines = append(lines, "}", "")

	src, err := format.Source([]byte(strings.Join(lines, "\n")))
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return src, nil
}

func writeFile(path string, data []byte) error {
	// The generated source is committed and must be repository-readable.
	if err := os.WriteFile(path, data, generatedFileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
