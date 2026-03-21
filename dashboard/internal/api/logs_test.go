package api

import (
	"testing"
)

func TestParseStatusRange(t *testing.T) {
	tests := []struct {
		input   string
		wantMin int
		wantMax int
	}{
		{"", 0, 0},
		{"2xx", 200, 299},
		{"2XX", 200, 299},
		{"4xx", 400, 499},
		{"5xx", 500, 599},
		{"200", 200, 200},
		{"404", 404, 404},
		{"500", 500, 500},
		{"  2xx  ", 200, 299},
		{"abc", 0, 0},
		{"xx", 0, 0},
		{"1xx", 100, 199},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			gotMin, gotMax := parseStatusRange(tc.input)
			if gotMin != tc.wantMin || gotMax != tc.wantMax {
				t.Errorf("parseStatusRange(%q) = (%d, %d), want (%d, %d)",
					tc.input, gotMin, gotMax, tc.wantMin, tc.wantMax)
			}
		})
	}
}
