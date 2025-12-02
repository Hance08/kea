package validation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/store"
)

// AccountStore defines the interface for account storage operations
// This prevents circular dependency with logic package
type AccountStore interface {
	CheckAccountExists(name string) (bool, error)
	GetAccountByName(name string) (*store.Account, error)
}

// AccountValidator handles account validation logic
type AccountValidator struct {
	store AccountStore
}

// NewAccountValidator creates a new account validator
func NewAccountValidator(store AccountStore) *AccountValidator {
	return &AccountValidator{store: store}
}

// ValidateAccountName validates a basic account name (without checking existence)
func (v *AccountValidator) ValidateAccountName(val any) error {
	name, ok := val.(string)
	if !ok {
		return fmt.Errorf("account name must be a string")
	}

	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("account name can't be empty")
	}

	if strings.Contains(name, ":") {
		return fmt.Errorf("account name cannot contain ':' character")
	}

	reservedNames := []string{"Assets", "Liabilities", "Equity", "Revenue", "Expenses"}
	for _, reserved := range reservedNames {
		if strings.EqualFold(name, reserved) {
			return fmt.Errorf("'%s' is a reserved root account name", name)
		}
	}

	if len(name) > constants.MaxNameLen {
		return fmt.Errorf("account name too long (max %d characters)", constants.MaxNameLen)
	}
	return nil
}

// ValidateAccountNameWithPrefix returns a validator that checks both name format and existence
func (v *AccountValidator) ValidateAccountNameWithPrefix(prefix string) func(any) error {
	return func(val any) error {
		partialName := val.(string)

		if err := v.ValidateAccountName(partialName); err != nil {
			return err
		}

		fullName := prefix + ":" + partialName
		exists, err := v.store.CheckAccountExists(fullName)
		if err != nil {
			return fmt.Errorf("failed to check account: %w", err)
		}
		if exists {
			return fmt.Errorf("account '%s' already exists", fullName)
		}

		return nil
	}
}

// ValidateFullAccountName validates a full account name and checks if it exists
func (v *AccountValidator) ValidateFullAccountName(fullName string) error {
	if len(fullName) > constants.MaxNameLen {
		return fmt.Errorf("account name too long (max %d characters)", constants.MaxNameLen)
	}

	exists, err := v.store.CheckAccountExists(fullName)
	if err != nil {
		return fmt.Errorf("failed to check account existence: %w", err)
	}
	if exists {
		return fmt.Errorf("account '%s' already exists", fullName)
	}

	return nil
}

// ValidateParentAccount validates and retrieves a parent account
func (v *AccountValidator) ValidateParentAccount(name string) (*store.Account, error) {
	if name == "" {
		return nil, fmt.Errorf("parent account name can't be empty")
	}

	parentAccount, err := v.store.GetAccountByName(name)
	if err != nil {
		return nil, fmt.Errorf("parent account not found: %w", err)
	}

	return parentAccount, nil
}

// ValidateCurrency validates a currency code format
// Accepts both string and any (for survey compatibility)
func ValidateCurrency(val any) error {
	var currency string
	
	// Handle both string and any types
	switch v := val.(type) {
	case string:
		currency = v
	default:
		return fmt.Errorf("currency code must be a string")
	}
	
	currency = strings.TrimSpace(strings.ToUpper(currency))
	
	if currency == "" {
		return nil // Empty is allowed (will use default)
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

// ValidateInitialBalance validates initial balance input
func ValidateInitialBalance(val any) error {
	input, ok := val.(string)
	if !ok {
		return fmt.Errorf("balance must be a string")
	}
	
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

	if balanceFloat > 9223372036854775 {
		return fmt.Errorf("balance amount too large")
	}

	return nil
}
