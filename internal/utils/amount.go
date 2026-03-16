package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hance08/kea/internal/model"
)

func FormatAmount(cents int64) string {
	amountVal := float64(cents) / float64(model.CentsPerUnit)
	return humanize.CommafWithDigits(amountVal, 2)
}

func ParseAmount(amountStr string) (int64, error) {
	isNegative := false
	if strings.HasPrefix(amountStr, "-") {
		isNegative = true
		amountStr = strings.TrimPrefix(amountStr, "-")
	}

	var dollars, cents int64
	parts := strings.Split(amountStr, ".")

	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount format: %s", amountStr)
	}

	// Parse dollar part
	if parts[0] != "" {
		parsed, err := parseDigitsStrict(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid amount: %s", amountStr)
		}
		dollars = parsed
	}

	// Parse cents part if exists
	var dollarCarry int64
	if len(parts) == 2 {
		centStr := parts[1]
		// Pad or round to 2 digits
		if len(centStr) == 1 {
			centStr += "0" // "150.5" -> "50"
		}

		if len(centStr) > 2 {
			for i := 0; i < len(centStr); i++ {
				if centStr[i] < '0' || centStr[i] > '9' {
					return 0, fmt.Errorf("invalid cents: %s", amountStr)
				}
			}

			firstTwo, err := parseDigitsStrict(centStr[:2])
			if err != nil {
				return 0, fmt.Errorf("invalid cents: %s", amountStr)
			}

			cents = firstTwo
			if centStr[2] >= '5' {
				cents++
				if cents == int64(model.CentsPerUnit) {
					dollarCarry = 1
					cents = 0
				}
			}
		} else {
			parsed, err := parseDigitsStrict(centStr)
			if err != nil {
				return 0, fmt.Errorf("invalid cents: %s", amountStr)
			}
			cents = parsed
		}
	}

	total := (dollars+dollarCarry)*int64(model.CentsPerUnit) + cents

	if isNegative {
		total = -total
	}

	return total, nil
}

func parseDigitsStrict(value string) (int64, error) {
	if value == "" {
		return 0, fmt.Errorf("empty numeric value")
	}

	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, fmt.Errorf("non-digit character found")
		}
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}

	return parsed, nil
}
