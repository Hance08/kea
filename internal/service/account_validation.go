package service

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
)

// ValidateAccountName validates a basic account name segment (without path prefix)
func (as *AccountService) ValidateAccountName(name string) error {
	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("account name can't be empty")
	}
	if strings.Contains(name, ":") {
		return fmt.Errorf("account name cannot contain ':' character")
	}
	if model.ReservedNames[strings.ToLower(name)] {
		return fmt.Errorf("'%s' is a reserved root account name", name)
	}
	if len(name) > model.AccountNameMaxLength {
		return fmt.Errorf("account name too long (max %d characters)", model.AccountNameMaxLength)
	}
	return nil
}

// ValidateFullAccountName validates the full hierarchical account name (e.g. "Assets:Bank:Checking")
func (as *AccountService) ValidateFullAccountName(fullName string) error {
	if len(fullName) > model.AccountNameMaxLength {
		return fmt.Errorf("account name too long (max %d characters)", model.AccountNameMaxLength)
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
		return fmt.Errorf("currency code must be 3 characters (e.g. USD)")
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			return fmt.Errorf("currency code must contain only letters")
		}
	}
	return nil
}
