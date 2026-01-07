package utils

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hance08/kea/internal/constant"
)

func FormatAmount(cents int64) string {
	amountVal := float64(cents) / float64(constant.CentsPerUnit)
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
		_, err := fmt.Sscanf(parts[0], "%d", &dollars)
		if err != nil {
			return 0, fmt.Errorf("invalid amount: %s", amountStr)
		}
	}

	// Parse cents part if exists
	if len(parts) == 2 {
		centStr := parts[1]
		// Pad or truncate to 2 digits
		if len(centStr) == 1 {
			centStr += "0" // "150.5" -> "50"
		} else if len(centStr) > 2 {
			centStr = centStr[:2] // Truncate extra digits
		}

		_, err := fmt.Sscanf(centStr, "%d", &cents)
		if err != nil {
			return 0, fmt.Errorf("invalid cents: %s", amountStr)
		}
	}

	total := dollars*int64(constant.CentsPerUnit) + cents

	if isNegative {
		total = -total
	}

	return total, nil
}
