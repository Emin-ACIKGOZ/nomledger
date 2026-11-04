// SPDX-License-Identifier: MIT

package core

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// TestTransactionBuilderNoAliasInternal mutates builder state directly to prove
// Build copied the accumulated entries and metadata.
func TestTransactionBuilderNoAliasInternal(t *testing.T) {
	config := LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: testCurrencyUSD,
	}
	builder := NewTxBuilder("tx-internal", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), config, nil)

	debit, err := NewEntry("tx-internal-debit", decimal.NewFromInt(100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry debit: %v", err)
	}
	credit, err := NewEntry("tx-internal-credit", decimal.NewFromInt(-100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry credit: %v", err)
	}
	builder.AddEntry(debit).AddEntry(credit)
	builder.WithMetadata(map[string]any{"note": "before"})

	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	before := transaction.Entries()

	builder.entries[0] = Entry{}
	builder.entries = append(builder.entries, Entry{})
	builder.metadata["note"] = "after"
	builder.metadata["added"] = true

	after := transaction.Entries()
	if len(after) != len(before) {
		t.Fatalf("Entries() length = %d; want %d", len(after), len(before))
	}
	for i := range before {
		if after[i].AccountID() != before[i].AccountID() {
			t.Errorf("entries[%d].AccountID() = %q; want %q", i, after[i].AccountID(), before[i].AccountID())
		}
		if !after[i].TxAmount().Equal(before[i].TxAmount()) {
			t.Errorf("entries[%d].TxAmount() = %s; want %s", i, after[i].TxAmount(), before[i].TxAmount())
		}
	}

	if transaction.Metadata()["note"] != "before" {
		t.Errorf("Metadata()[note] = %v; want before", transaction.Metadata()["note"])
	}
	if _, ok := transaction.Metadata()["added"]; ok {
		t.Error("direct builder metadata mutation leaked into the transaction")
	}
}
