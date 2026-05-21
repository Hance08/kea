// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hance08/kea/internal/model"
)

var ErrAmountOverflow = fmt.Errorf("amount too large: exceeds int64 range")

const maxDollars = math.MaxInt64 / int64(model.CentsPerUnit)

func FormatAmount(cents int64) string {
	if cents == math.MinInt64 {
		panic("utils.FormatAmount: undefined for math.MinInt64 (overflow)")
	}
	negative := cents < 0
	if negative {
		cents = -cents
	}

	whole := cents / int64(model.CentsPerUnit)
	frac := cents % int64(model.CentsPerUnit)

	wholeStr := insertCommas(strconv.FormatInt(whole, 10))

	var result string
	if frac == 0 {
		result = wholeStr
	} else if frac%10 == 0 {
		result = fmt.Sprintf("%s.%d", wholeStr, frac/10)
	} else {
		result = fmt.Sprintf("%s.%02d", wholeStr, frac)
	}

	if negative {
		return "-" + result
	}
	return result
}

func insertCommas(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}

	var buf strings.Builder
	firstGroup := n % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	buf.WriteString(s[:firstGroup])
	for i := firstGroup; i < n; i += 3 {
		buf.WriteByte(',')
		buf.WriteString(s[i : i+3])
	}
	return buf.String()
}

func ParseAmount(amountStr string) (int64, error) {
	isNegative := false
	if amountStr == "" || amountStr == "-" {
		return 0, fmt.Errorf("%q", amountStr)
	}
	if strings.HasPrefix(amountStr, "-") {
		isNegative = true
		amountStr = strings.TrimPrefix(amountStr, "-")
	}

	var dollars, cents int64
	parts := strings.Split(amountStr, ".")

	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid format: %s", amountStr)
	}

	// Parse dollar part
	if parts[0] != "" {
		parsed, err := parseDigitsStrict(parts[0])
		if err != nil {
			return 0, fmt.Errorf("%w: %s", err, amountStr)
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

	dollars += dollarCarry
	if dollars > maxDollars {
		return 0, ErrAmountOverflow
	}

	total := dollars*int64(model.CentsPerUnit) + cents
	if total < 0 {
		return 0, ErrAmountOverflow
	}

	if isNegative {
		total = -total
	}

	return total, nil
}

func parseDigitsStrict(value string) (int64, error) {
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
