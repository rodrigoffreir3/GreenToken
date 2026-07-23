// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tokens

import "testing"

func TestCountTokens(t *testing.T) {
	tests := []struct {
		line          string
		expectedCount int64
		expectedMatch bool
	}{
		{
			line:          "INFO: Processed a request, generated 45 tokens successfully.",
			expectedCount: 45,
			expectedMatch: true,
		},
		{
			line:          "llama_print_timings: eval count = 120, time = 2400 ms",
			expectedCount: 120,
			expectedMatch: true,
		},
		{
			line:          "eval tokens = 80, speed = 32.5 tokens/sec",
			expectedCount: 80,
			expectedMatch: true,
		},
		{
			line:          "summary -> tokens: 256",
			expectedCount: 256,
			expectedMatch: true,
		},
		{
			line:          "some random logging line that doesn't mention tokens",
			expectedCount: 0,
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		count, matched := CountTokens(tt.line)
		if matched != tt.expectedMatch {
			t.Errorf("Para a linha %q, matched esperado: %v, obtido: %v", tt.line, tt.expectedMatch, matched)
		}
		if count != tt.expectedCount {
			t.Errorf("Para a linha %q, count esperado: %d, obtido: %d", tt.line, tt.expectedCount, count)
		}
	}
}
