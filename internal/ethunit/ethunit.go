// Package ethunit converts between wei and ETH. It is a leaf package with no
// internal imports, so both the data and analysis layers can share one
// implementation without creating an import cycle.
package ethunit

import "math/big"

// ToETH converts a wei amount (base-10 string) to ETH as a float64.
// Invalid input yields 0.
func ToETH(wei string) float64 {
	n, ok := new(big.Int).SetString(wei, 10)
	if !ok {
		return 0
	}
	eth, _ := new(big.Float).Quo(new(big.Float).SetInt(n), big.NewFloat(1e18)).Float64()
	return eth
}
