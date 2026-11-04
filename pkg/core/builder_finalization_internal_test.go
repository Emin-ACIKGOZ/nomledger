// SPDX-License-Identifier: MIT

package core

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

const (
	finalizedNoteKey  = "note"
	finalizedBefore   = "before"
	finalizedAfter    = "after"
	finalizedAddedKey = "added"
)

type finalizedBuilderSnapshot struct {
	entries     []Entry
	metadataLen int
	adjusting   bool
}

// TestFinalizedBuilderMutatorsInternal proves the finalized guards are real:
// the mutators must leave unexported builder state untouched after Build.
func TestFinalizedBuilderMutatorsInternal(t *testing.T) {
	builder, snapshot := buildFinalizedBuilderInternal(t)

	extra, err := NewEntry("finalized-extra", decimal.NewFromInt(1), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry extra: %v", err)
	}
	builder.AddEntry(extra)
	builder.WithMetadata(map[string]any{
		finalizedNoteKey:  finalizedAfter,
		finalizedAddedKey: true,
	})
	builder.SetAdjustingEvent(true)

	assertFinalizedBuilderSnapshot(t, builder, snapshot)
}

func buildFinalizedBuilderInternal(t *testing.T) (*TxBuilder, finalizedBuilderSnapshot) {
	t.Helper()
	config := LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
		FunctionalCurrency: testCurrencyUSD,
	}
	builder := NewTxBuilder("internal-finalized", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), config, nil)

	debit, err := NewEntry("finalized-debit", decimal.NewFromInt(100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry debit: %v", err)
	}
	credit, err := NewEntry("finalized-credit", decimal.NewFromInt(-100), testCurrencyUSD, decimal.NewFromInt(1), testCurrencyUSD)
	if err != nil {
		t.Fatalf("NewEntry credit: %v", err)
	}
	builder.AddEntry(debit).AddEntry(credit)
	builder.WithMetadata(map[string]any{finalizedNoteKey: finalizedBefore})
	builder.SetAdjustingEvent(false)

	if _, err := builder.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}

	return builder, finalizedBuilderSnapshot{
		entries:     append([]Entry(nil), builder.entries...),
		metadataLen: len(builder.metadata),
		adjusting:   builder.isAdjustingEvent,
	}
}

func assertFinalizedBuilderSnapshot(t *testing.T, builder *TxBuilder, snapshot finalizedBuilderSnapshot) {
	t.Helper()
	if len(builder.entries) != len(snapshot.entries) {
		t.Fatalf("builder entries length = %d; want %d", len(builder.entries), len(snapshot.entries))
	}
	for i := range snapshot.entries {
		if builder.entries[i].AccountID() != snapshot.entries[i].AccountID() {
			t.Errorf("builder entries[%d].AccountID() = %q; want %q",
				i, builder.entries[i].AccountID(), snapshot.entries[i].AccountID())
		}
		if !builder.entries[i].TxAmount().Equal(snapshot.entries[i].TxAmount()) {
			t.Errorf("builder entries[%d].TxAmount() = %s; want %s",
				i, builder.entries[i].TxAmount(), snapshot.entries[i].TxAmount())
		}
	}

	if len(builder.metadata) != snapshot.metadataLen {
		t.Errorf("builder metadata length = %d; want %d", len(builder.metadata), snapshot.metadataLen)
	}
	if builder.metadata[finalizedNoteKey] != finalizedBefore {
		t.Errorf("builder metadata[%q] = %v; want %q",
			finalizedNoteKey, builder.metadata[finalizedNoteKey], finalizedBefore)
	}
	if _, ok := builder.metadata[finalizedAddedKey]; ok {
		t.Errorf("builder metadata gained key %q after finalization", finalizedAddedKey)
	}
	if builder.isAdjustingEvent != snapshot.adjusting {
		t.Errorf("builder isAdjustingEvent = %t; want %t", builder.isAdjustingEvent, snapshot.adjusting)
	}
}
