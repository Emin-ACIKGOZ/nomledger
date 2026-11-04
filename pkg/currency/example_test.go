// SPDX-License-Identifier: MIT

package currency_test

import (
	"fmt"

	"github.com/Emin-ACIKGOZ/nomledger/pkg/currency"
)

func ExampleLookup() {
	currencyInfo, err := currency.Lookup("usd")
	if err != nil {
		panic(err)
	}
	fmt.Println(currencyInfo.Code(), currencyInfo.Numeric())
	// Output: USD 840
}
