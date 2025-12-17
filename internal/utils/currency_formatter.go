package utils

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/constants"
)

func FormatFromCents(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/float64(constants.CentsPerUnit))
}

func ParseToCents(amountStr string) (int64, error) {
	var dollars, cents int64

	// Handle formats: "150", "150.5", "150.50"
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

	total := dollars*int64(constants.CentsPerUnit) + cents
	return total, nil
}
