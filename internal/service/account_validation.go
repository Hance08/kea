// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"strings"

	"github.com/hance08/kea/internal/model"
)

// ValidateAccountName validates a basic account name segment (without path prefix)
func (as *AccountService) ValidateAccountName(name string) error {
	if name != strings.TrimSpace(name) {
		return validationErrorf("name", "account name cannot start or end with spaces")
	}
	name = strings.TrimSpace(name)

	if name == "" {
		return validationErrorf("name", "account name can't be empty")
	}
	if strings.Contains(name, ":") {
		return validationErrorf("name", "account name cannot contain ':' character")
	}
	if model.ReservedNames[strings.ToLower(name)] {
		return validationErrorf("name", "'%s' is a reserved root account name", name)
	}
	if len(name) > model.AccountNameMaxLength {
		return validationErrorf("name", "account name too long (max %d characters)", model.AccountNameMaxLength)
	}
	return nil
}

// ValidateFullAccountName validates the full hierarchical account name (e.g. "Assets:Bank:Checking")
func (as *AccountService) ValidateFullAccountName(fullName string) error {
	if fullName != strings.TrimSpace(fullName) {
		return validationErrorf("name", "account name cannot start or end with spaces")
	}
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return validationErrorf("name", "account name can't be empty")
	}
	if len(fullName) > model.AccountNameMaxLength {
		return validationErrorf("name", "account name too long (max %d characters)", model.AccountNameMaxLength)
	}

	parts := strings.Split(fullName, ":")
	if len(parts) == 0 {
		return validationErrorf("name", "invalid account name")
	}

	root := strings.ToLower(strings.TrimSpace(parts[0]))
	if !model.ReservedNames[root] {
		return validationErrorf("name", "account root must be one of: Assets, Liabilities, Equity, Revenue, Expenses")
	}

	for i, part := range parts {
		if part != strings.TrimSpace(part) {
			return validationErrorf("name", "account segment at level %d cannot start or end with spaces", i+1)
		}
		segment := strings.TrimSpace(part)
		if segment == "" {
			return validationErrorf("name", "account name has empty segment at level %d", i+1)
		}
		if strings.Contains(segment, ":") {
			return validationErrorf("name", "account segment '%s' cannot contain ':'", segment)
		}
		if len(segment) > model.AccountNameMaxLength {
			return validationErrorf("name", "account segment too long (max %d characters)", model.AccountNameMaxLength)
		}
		if i > 0 && model.ReservedNames[strings.ToLower(segment)] {
			return validationErrorf("name", "account segment '%s' cannot use reserved root account name", segment)
		}
	}

	return nil
}

// ValidateCurrency validates a 3-letter ISO currency code
func (as *AccountService) ValidateCurrency(currency string) error {
	currency = strings.TrimSpace(strings.ToUpper(currency))

	if currency == "" {
		return nil // empty is allowed, will use default
	}
	if len(currency) != 3 {
		return validationErrorf("currency", "currency code must be 3 characters (e.g. USD)")
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			return validationErrorf("currency", "currency code must contain only letters")
		}
	}
	return nil
}
