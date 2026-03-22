package utils_test

import (
	"testing"

	"github.com/hance08/kea/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// FormatAmount
// ──────────────────────────────────────────────

// Note: humanize.CommafWithDigits trims trailing zeros.
// e.g. 100 cents (1.00) → "1", 150 cents (1.50) → "1.5"
func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  string
	}{
		{"zero", 0, "0"},
		{"whole number", 100, "1"},
		{"single decimal place", 150, "1.5"},
		{"two significant decimals", 151, "1.51"},
		{"one cent", 1, "0.01"},
		{"thousands separator", 1234567, "12,345.67"},
		{"one million whole", 100000000, "1,000,000"},
		{"negative whole", -500, "-5"},
		{"negative with decimal", -150, "-1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.FormatAmount(tt.cents)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ──────────────────────────────────────────────
// ParseAmount — valid inputs
// ──────────────────────────────────────────────

func TestParseAmount_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"integer", "100", 10000},
		{"two decimal places", "1.50", 150},
		{"one decimal place padded", "1.5", 150},
		{"zero integer no decimal", "0", 0},
		{"zero with decimals", "0.00", 0},
		{"decimal only", ".50", 50},
		{"decimal only single digit", ".5", 50},
		{"large number", "999999.99", 99999999},
		{"negative integer", "-100", -10000},
		{"negative with decimal", "-1.50", -150},
		{"negative zero", "-0.00", 0},

		// Three-decimal rounding boundary (critical cases)
		{"3 decimals no round-up", "1.004", 100},
		{"3 decimals exact round-up", "1.005", 101},
		{"3 decimals near boundary", "1.994", 199},
		{"3 decimals round-up carries to dollar", "0.995", 100}, // 99 cents + round up → 100 cents = 1 dollar carry
		{"3 decimals no carry", "0.994", 99},
		// "1.0099": dollars=1, centStr="0099", firstTwo=00, third='9'>=5 → cents=1, total=1*100+1=101
		{"4 decimals truncated with round-up", "1.0099", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ParseAmount(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ──────────────────────────────────────────────
// ParseAmount — invalid inputs
// ──────────────────────────────────────────────

// Confirmed edge-case behavior (production code, not test bugs):
// - ParseAmount("") → (0, nil): empty string treated as 0, potential bug
// - ParseAmount("-") → (0, nil): bare minus treated as -0 = 0, potential bug
func TestParseAmount_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"two decimal points", "1.2.3"},
		{"letter in integer part", "1a.00"},
		{"letter in decimal part", "1.0a"},
		{"letters only", "abc"},
		{"minus sign with letters", "-abc"},
		{"whitespace", " "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := utils.ParseAmount(tt.input)
			assert.Error(t, err, "expected %q to return an error", tt.input)
		})
	}
}

// ──────────────────────────────────────────────
// ParseAmount ↔ FormatAmount round-trip
// ──────────────────────────────────────────────

// FormatAmount trims trailing zeros, so "1.50" round-trips to "1.5".
func TestParseFormatRoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantFormatd string
	}{
		{"two significant decimals", "1.51", "1.51"},
		{"integer", "100", "100"},
		{"negative with decimal", "-1.51", "-1.51"},
		{"thousands", "1234.56", "1,234.56"},
		{"one cent", "0.01", "0.01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cents, err := utils.ParseAmount(tt.input)
			require.NoError(t, err)
			formatted := utils.FormatAmount(cents)
			assert.Equal(t, tt.wantFormatd, formatted)
		})
	}
}
