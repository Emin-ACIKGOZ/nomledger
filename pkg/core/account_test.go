// SPDX-License-Identifier: MIT

package core_test

import (
	"nomledger/pkg/core"
	"testing"
)

func TestAccountTypes(t *testing.T) {
	// Verify strict distinctness of Account Types to ensure reporting buckets do not overlap.
	types := []core.AccountType{
		core.Asset,
		core.Liability,
		core.Equity,
		core.Income,
		core.Expense,
	}

	seen := make(map[core.AccountType]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("Duplicate AccountType definition found: %v", typ)
		}
		seen[typ] = true
	}
}

func TestMeasurementCategories(t *testing.T) {
	// Verify strict distinctness of IFRS 9 categories.
	cats := []core.MeasurementCategory{
		core.AmortizedCost,
		core.FVTPL,
	}

	seen := make(map[core.MeasurementCategory]bool)
	for _, cat := range cats {
		if seen[cat] {
			t.Errorf("Duplicate MeasurementCategory definition found: %v", cat)
		}
		seen[cat] = true
	}
}
