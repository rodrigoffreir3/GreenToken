// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aggregator

import (
	"os"
	"testing"
)

func TestGetKWhPrice(t *testing.T) {
	// Limpar env se existir
	orig := os.Getenv("GT_KWH_PRICE")
	defer os.Setenv("GT_KWH_PRICE", orig)

	os.Unsetenv("GT_KWH_PRICE")
	if price := GetKWhPrice(); price != DefaultKWhPrice {
		t.Errorf("Esperava preço default %f, obteve %f", DefaultKWhPrice, price)
	}

	os.Setenv("GT_KWH_PRICE", "1.25")
	if price := GetKWhPrice(); price != 1.25 {
		t.Errorf("Esperava preço customizado 1.25, obteve %f", price)
	}

	os.Setenv("GT_KWH_PRICE", "invalido")
	if price := GetKWhPrice(); price != DefaultKWhPrice {
		t.Errorf("Esperava fallback para default ao passar valor inválido, obteve %f", price)
	}
}

func TestCalculateJoules(t *testing.T) {
	tests := []struct {
		watts   float64
		seconds float64
		want    float64
	}{
		{100, 2, 200},
		{-5, 10, 0},
		{50, -2, 0},
		{0, 5, 0},
	}

	for _, tt := range tests {
		got := CalculateJoules(tt.watts, tt.seconds)
		if got != tt.want {
			t.Errorf("CalculateJoules(%f, %f) = %f; want %f", tt.watts, tt.seconds, got, tt.want)
		}
	}
}

func TestCalculateCost(t *testing.T) {
	// 3.600.000 Joules com preço de 1.0 = 1.0 de custo
	got := CalculateCost(3600000.0, 1.0)
	if got != 1.0 {
		t.Errorf("CalculateCost(3600000, 1.0) = %f; want 1.0", got)
	}

	// Tratar negativos
	if got := CalculateCost(-100, 1.0); got != 0 {
		t.Errorf("CalculateCost(-100, 1.0) = %f; want 0", got)
	}
}

func TestCalculateCostPerToken(t *testing.T) {
	// Teste com entrada do spec: 100W total, 2s, 500 tokens, preço kWh 0.80
	joules := CalculateJoules(100, 2) // 200 J
	cost := CalculateCost(joules, 0.80) // (200 / 3600000) * 0.80 = 4.4444e-5
	costPerToken := CalculateCostPerToken(cost, 500)

	expected := (200.0 / 3600000.0) * 0.80 / 500.0
	if costPerToken != expected {
		t.Errorf("costPerToken = %f; want %f", costPerToken, expected)
	}

	// Divisão por zero tokens
	if got := CalculateCostPerToken(cost, 0); got != 0 {
		t.Errorf("CalculateCostPerToken(cost, 0) = %f; want 0", got)
	}
}
