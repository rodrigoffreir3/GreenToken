// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aggregator

import (
	"os"
	"strconv"
)

// DefaultKWhPrice é a tarifa média estimada do kWh no Brasil (R$ 0.80) caso não configurada.
const DefaultKWhPrice = 0.80

// GetKWhPrice retorna o preço do kWh configurado na variável de ambiente GT_KWH_PRICE
// ou o valor padrão.
func GetKWhPrice() float64 {
	priceStr := os.Getenv("GT_KWH_PRICE")
	if priceStr == "" {
		return DefaultKWhPrice
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return DefaultKWhPrice
	}
	return price
}

// CalculateJoules calcula a energia consumida em Joules (Watts * segundos)
func CalculateJoules(watts float64, seconds float64) float64 {
	if watts < 0 || seconds < 0 {
		return 0
	}
	return watts * seconds
}

// CalculateCost calcula o custo financeiro a partir dos Joules consumidos e da tarifa de kWh.
// 1 kWh = 3.600.000 Joules
func CalculateCost(joules float64, pricePerKWh float64) float64 {
	if joules <= 0 || pricePerKWh <= 0 {
		return 0
	}
	kWh := joules / 3600000.0
	return kWh * pricePerKWh
}

// CalculateCostPerToken calcula o custo por token. Retorna 0 se os tokens forem zero.
func CalculateCostPerToken(cost float64, tokens int64) float64 {
	if tokens <= 0 || cost <= 0 {
		return 0
	}
	return cost / float64(tokens)
}
