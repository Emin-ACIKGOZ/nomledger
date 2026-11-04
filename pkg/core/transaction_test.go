// SPDX-License-Identifier: MIT

package core_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/core"

	"github.com/shopspring/decimal"
)

const (
	txID            = "tx-wu009"
	txDebitAccount  = "acc-debit"
	txCreditAccount = "acc-credit"
	txExtraAccount  = "acc-extra"

	txAmountHundred          = "100.00"
	txAmountNegHundred       = "-100.00"
	txAmountOne              = "1.00"
	txAmountNegOne           = "-1.00"
	txAmountOverPrecision    = "1.001"
	txAmountNegOverPrecision = "-1.001"
	codeCLF                  = "CLF"
	txRateOne                = "1"

	txMetaNote    = "note"
	txMetaCount   = "n"
	txMetaNested  = "nested"
	txMetaItems   = "items"
	txMetaInner   = "key"
	txMetaBuilder = "builder"
	txMetaValue   = "value"
	txMetaChanged = "changed"
)

func mustTxDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q): %v", value, err)
	}
	return d
}

func mustTxEntry(t *testing.T, accountID, amount string) core.Entry {
	t.Helper()
	entry, err := core.NewEntry(
		accountID,
		mustTxDecimal(t, amount),
		codeUSD,
		mustTxDecimal(t, txRateOne),
		codeUSD,
	)
	if err != nil {
		t.Fatalf("NewEntry(%s, %s): %v", accountID, amount, err)
	}
	return entry
}

func txTestConfig() core.LedgerConfig {
	return core.LedgerConfig{
		ClosedDate:         time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		FunctionalCurrency: codeUSD,
	}
}

func newValidTransaction(t *testing.T, id string, metadata map[string]any) (core.Transaction, *core.TxBuilder) {
	t.Helper()
	builder := core.NewTxBuilder(
		id,
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		txTestConfig(),
		mockRateProvider{},
	)
	builder.AddEntry(mustTxEntry(t, txDebitAccount, txAmountHundred))
	builder.AddEntry(mustTxEntry(t, txCreditAccount, txAmountNegHundred))
	if metadata != nil {
		builder.WithMetadata(metadata)
	}

	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return transaction, builder
}

func assertEntriesEqual(t *testing.T, got, want []core.Entry) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("entries length = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].AccountID() != want[i].AccountID() {
			t.Errorf("entries[%d].AccountID() = %q; want %q", i, got[i].AccountID(), want[i].AccountID())
		}
		if !got[i].TxAmount().Equal(want[i].TxAmount()) {
			t.Errorf("entries[%d].TxAmount() = %s; want %s", i, got[i].TxAmount(), want[i].TxAmount())
		}
		if got[i].TxCurrency() != want[i].TxCurrency() {
			t.Errorf("entries[%d].TxCurrency() = %q; want %q", i, got[i].TxCurrency(), want[i].TxCurrency())
		}
		if !got[i].FunctionalAmount().Equal(want[i].FunctionalAmount()) {
			t.Errorf("entries[%d].FunctionalAmount() = %s; want %s",
				i, got[i].FunctionalAmount(), want[i].FunctionalAmount())
		}
	}
}

// TestTransactionEntriesIndependent.
func TestTransactionEntriesIndependent(t *testing.T) {
	transaction, _ := newValidTransaction(t, txID, nil)
	before := append([]core.Entry(nil), transaction.Entries()...)

	returned := transaction.Entries()
	returned[0] = core.Entry{}
	extra := mustTxEntry(t, txExtraAccount, txAmountOne)
	returned = append(returned, extra)
	if len(returned) != len(before)+1 {
		t.Errorf("appended slice length = %d; want %d", len(returned), len(before)+1)
	}

	afterMutation := transaction.Entries()
	assertEntriesEqual(t, afterMutation, before)
}

// TestTransactionMetadataIndependent.
func TestTransactionMetadataIndependent(t *testing.T) {
	input := map[string]any{
		txMetaNote:  txMetaValue,
		txMetaCount: 1,
	}
	builder := core.NewTxBuilder(txID, ledgerEffectiveDate(), txTestConfig(), mockRateProvider{})
	for _, entry := range transactionFieldEntries(t) {
		builder.AddEntry(entry)
	}
	builder.WithMetadata(input)

	input[txMetaNote] = "mutated input"
	input["later"] = true

	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if transaction.Metadata()[txMetaNote] != txMetaValue {
		t.Errorf("Metadata()[note] = %v; want %v", transaction.Metadata()[txMetaNote], txMetaValue)
	}
	if _, ok := transaction.Metadata()["later"]; ok {
		t.Error("caller input mutation leaked into the builder")
	}

	assertReturnedMetadataIndependent(t, transaction)

	input[txMetaNote] = "mutated after build"
	builder.WithMetadata(map[string]any{txMetaBuilder: txMetaChanged})
	if _, ok := transaction.Metadata()[txMetaBuilder]; ok {
		t.Error("WithMetadata after Build leaked into the transaction")
	}
	if transaction.Metadata()[txMetaNote] != txMetaValue {
		t.Errorf("transaction Metadata()[note] = %v; want %v", transaction.Metadata()[txMetaNote], txMetaValue)
	}
}

func assertReturnedMetadataIndependent(t *testing.T, transaction core.Transaction) {
	t.Helper()
	returned := transaction.Metadata()
	returned[txMetaNote] = txMetaChanged
	delete(returned, txMetaCount)
	returned["added"] = true

	again := transaction.Metadata()
	if again[txMetaNote] != txMetaValue {
		t.Errorf("Metadata()[note] = %v; want %v", again[txMetaNote], txMetaValue)
	}
	if _, ok := again[txMetaCount]; !ok {
		t.Errorf("Metadata() lost key %q after returned-map mutation", txMetaCount)
	}
	if _, ok := again["added"]; ok {
		t.Error("Metadata() gained key \"added\" after returned-map mutation")
	}
}

// TestTransactionAccessors.
func TestTransactionAccessors(t *testing.T) {
	effective := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	config := txTestConfig()
	builder := core.NewTxBuilder(txID, effective, config, mockRateProvider{})
	builder.SetAdjustingEvent(true)
	builder.WithMetadata(map[string]any{txMetaNote: txMetaValue})
	builder.AddEntry(mustTxEntry(t, txDebitAccount, txAmountHundred))
	builder.AddEntry(mustTxEntry(t, txCreditAccount, txAmountNegHundred))

	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	assertTransactionScalars(t, transaction, effective)
	assertTransactionCollections(t, transaction)
	assertTransactionUnexportedFields(t)
	assertTransactionCopyStable(t, transaction)
}

func assertTransactionScalars(t *testing.T, transaction core.Transaction, effective time.Time) {
	t.Helper()
	if transaction.ID() != txID {
		t.Errorf("ID() = %q; want %q", transaction.ID(), txID)
	}
	if !transaction.EffectiveDate().Equal(effective) {
		t.Errorf("EffectiveDate() = %s; want %s", transaction.EffectiveDate(), effective)
	}
	if !transaction.IsAdjustingEvent() {
		t.Error("IsAdjustingEvent() = false; want true")
	}
	if transaction.PostedAt().IsZero() {
		t.Error("PostedAt() = zero; want non-zero UTC instant")
	}
	if transaction.PostedAt().Location() != time.UTC {
		t.Errorf("PostedAt().Location() = %v; want UTC", transaction.PostedAt().Location())
	}
}

func assertTransactionCollections(t *testing.T, transaction core.Transaction) {
	t.Helper()
	entries := transaction.Entries()
	if len(entries) != 2 {
		t.Fatalf("Entries() length = %d; want 2", len(entries))
	}
	if entries[0].AccountID() != txDebitAccount || entries[1].AccountID() != txCreditAccount {
		t.Errorf("entry accounts = %q/%q; want %q/%q",
			entries[0].AccountID(), entries[1].AccountID(), txDebitAccount, txCreditAccount)
	}
	if transaction.Metadata()[txMetaNote] != txMetaValue {
		t.Errorf("Metadata()[note] = %v; want %v", transaction.Metadata()[txMetaNote], txMetaValue)
	}
}

func assertTransactionUnexportedFields(t *testing.T) {
	t.Helper()
	typ := reflect.TypeOf(core.Transaction{})
	if typ.Kind() != reflect.Struct {
		t.Fatalf("Transaction kind = %v; want struct", typ.Kind())
	}
	if typ.NumField() == 0 {
		t.Fatal("Transaction has no fields")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			t.Errorf("Transaction field %s is exported; want unexported", field.Name)
		}
	}
}

func assertTransactionCopyStable(t *testing.T, transaction core.Transaction) {
	t.Helper()
	copied := transaction
	if copied.ID() != transaction.ID() {
		t.Errorf("copied ID() = %q; want %q", copied.ID(), transaction.ID())
	}
	if !copied.EffectiveDate().Equal(transaction.EffectiveDate()) {
		t.Errorf("copied EffectiveDate() = %s; want %s", copied.EffectiveDate(), transaction.EffectiveDate())
	}
	if copied.IsAdjustingEvent() != transaction.IsAdjustingEvent() {
		t.Errorf("copied IsAdjustingEvent() = %t; want %t",
			copied.IsAdjustingEvent(), transaction.IsAdjustingEvent())
	}
	if !copied.PostedAt().Equal(transaction.PostedAt()) {
		t.Errorf("copied PostedAt() = %s; want %s", copied.PostedAt(), transaction.PostedAt())
	}
	assertEntriesEqual(t, copied.Entries(), transaction.Entries())
	if copied.Metadata()[txMetaNote] != transaction.Metadata()[txMetaNote] {
		t.Errorf("copied Metadata()[note] = %v; want %v",
			copied.Metadata()[txMetaNote], transaction.Metadata()[txMetaNote])
	}
}

// TestTransactionMetadataNil verifies the nil-to-empty metadata contract.
func TestTransactionMetadataNil(t *testing.T) {
	transaction, _ := newValidTransaction(t, txID, nil)

	metadata := transaction.Metadata()
	if metadata == nil {
		t.Fatal("Metadata() = nil; want non-nil empty map")
	}
	if len(metadata) != 0 {
		t.Errorf("Metadata() length = %d; want 0", len(metadata))
	}

	metadata["added"] = true
	again := transaction.Metadata()
	if len(again) != 0 {
		t.Errorf("Metadata() length after mutation = %d; want 0", len(again))
	}
}

// TestTransactionMetadataShallowLimit documents the shallow-copy behavior for nested values.
func TestTransactionMetadataShallowLimit(t *testing.T) {
	nested := map[string]any{txMetaInner: "original"}
	items := []string{"first", "second"}
	input := map[string]any{
		txMetaNested: nested,
		txMetaItems:  items,
	}
	transaction, _ := newValidTransaction(t, txID, input)

	// Top-level map is copied.
	input["added"] = true
	if _, ok := transaction.Metadata()["added"]; ok {
		t.Error("top-level input mutation leaked into transaction metadata")
	}
	returned := transaction.Metadata()
	returned["added2"] = true
	if _, ok := transaction.Metadata()["added2"]; ok {
		t.Error("top-level returned-map mutation leaked into stored metadata")
	}

	// Nested values remain shared by design.
	nested[txMetaInner] = "mutated"
	nestedAfter := transaction.Metadata()[txMetaNested].(map[string]any)
	if nestedAfter[txMetaInner] != "mutated" {
		t.Errorf("nested map value = %v; want shared mutation visible", nestedAfter[txMetaInner])
	}

	items[0] = txMetaChanged
	itemsAfter := transaction.Metadata()[txMetaItems].([]string)
	if itemsAfter[0] != txMetaChanged {
		t.Errorf("nested slice value = %q; want shared mutation visible", itemsAfter[0])
	}

	nestedAfter[txMetaInner] = "via-metadata"
	if nested[txMetaInner] != "via-metadata" {
		t.Errorf("original nested map = %v; want mutation through Metadata() visible", nested[txMetaInner])
	}
}

// TestBuilderMetadataInputCopy verifies WithMetadata copies the caller's map at
// call time.
func TestBuilderMetadataInputCopy(t *testing.T) {
	input := map[string]any{txMetaNote: "before"}
	builder := core.NewTxBuilder(
		txID,
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		txTestConfig(),
		mockRateProvider{},
	)
	builder.WithMetadata(input)
	builder.AddEntry(mustTxEntry(t, txDebitAccount, txAmountHundred))
	builder.AddEntry(mustTxEntry(t, txCreditAccount, txAmountNegHundred))

	input[txMetaNote] = "after"
	input["later"] = true

	transaction, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if transaction.Metadata()[txMetaNote] != "before" {
		t.Errorf("Metadata()[note] = %v; want before", transaction.Metadata()[txMetaNote])
	}
	if _, ok := transaction.Metadata()["later"]; ok {
		t.Error("post-WithMetadata input mutation leaked into the transaction")
	}
}

// TestTransactionEntriesOrder verifies that entry order is preserved through
// Build and Entries().
func TestTransactionEntriesOrder(t *testing.T) {
	transaction, _ := newValidTransaction(t, txID, nil)
	entries := transaction.Entries()
	if len(entries) != 2 {
		t.Fatalf("Entries() length = %d; want 2", len(entries))
	}
	if entries[0].AccountID() != txDebitAccount {
		t.Errorf("entries[0].AccountID() = %q; want %q", entries[0].AccountID(), txDebitAccount)
	}
	if entries[1].AccountID() != txCreditAccount {
		t.Errorf("entries[1].AccountID() = %q; want %q", entries[1].AccountID(), txCreditAccount)
	}
}
