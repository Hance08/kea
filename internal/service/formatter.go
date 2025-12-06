package service

import (
	"fmt"
	"strings"
)

// FormatAmountFromCents converts cents to currency string
func (al *AccountingService) FormatAmountFromCents(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

// ParseAmountToCents converts currency string to cents
// e.g., "150.50" -> 15050, "150" -> 15000
func (al *AccountingService) ParseAmountToCents(amountStr string) (int64, error) {
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

	total := dollars*100 + cents
	return total, nil
}
