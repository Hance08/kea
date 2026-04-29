// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package prompts

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

// ValidateInitialBalance validates initial balance string input from CLI prompt
func ValidateInitialBalance(input string) error {
	input = strings.TrimSpace(input)
	if input == "" || input == "0" {
		return nil
	}

	balanceFloat, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return fmt.Errorf("invalid number format")
	}
	if balanceFloat < 0 {
		return fmt.Errorf("initial balance can't be negative")
	}
	if balanceFloat > model.MaxSafeBalanceFloat {
		return fmt.Errorf("balance amount too large")
	}
	return nil
}

func ValidateAmountFormat(allowEmpty bool) func(string) error {
	return func(input string) error {
		if strings.TrimSpace(input) == "" {
			if allowEmpty {
				return nil
			}
			return fmt.Errorf("amount is required")
		}
		_, err := utils.ParseAmount(input)
		if err != nil {
			return fmt.Errorf("%v", err)
		}
		return nil
	}

}

func ValidateDateFormat(allowEmpty bool) func(string) error {
	return func(input string) error {
		if strings.TrimSpace(input) == "" {
			if allowEmpty {
				return nil
			}
			return fmt.Errorf("date is required")
		}

		_, err := time.Parse("2006-01-02", input)
		if err != nil {
			return fmt.Errorf("invalid format (YYYY-MM-DD)")
		}
		return nil
	}
}
