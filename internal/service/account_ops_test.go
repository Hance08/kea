// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// Test helper: inject system account
// ──────────────────────────────────────────────

func addOpeningBalanceAccount(accRepo *mockAccountRepo) {
	accRepo.addAccount(&model.Account{
		ID:       99,
		Name:     model.OpeningBalancesAccountName("USD"),
		Type:     model.AccountTypeEquity,
		Currency: "USD",
	})
}

func addOpeningBalancesForCurrency(accRepo *mockAccountRepo, currency string) {
	id := accRepo.nextID
	accRepo.nextID++
	accRepo.addAccount(&model.Account{
		ID:       id,
		Name:     model.OpeningBalancesAccountName(currency),
		Type:     model.AccountTypeEquity,
		Currency: currency,
	})
}

// ──────────────────────────────────────────────
// ValidateAccountName
// ──────────────────────────────────────────────

func TestValidateAccountName(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	valid := []struct {
		name  string
		input string
	}{
		{"plain name", "Checking"},
		{"with digits", "Bank2024"},
		{"with underscore", "My_Account"},
		{"exactly 100 chars", strings.Repeat("a", 100)},
	}
	for _, tt := range valid {
		t.Run("valid_"+tt.name, func(t *testing.T) {
			assert.NoError(t, svc.ValidateAccountName(tt.input))
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"leading space", " Bank"},
		{"trailing space", "Bank "},
		{"contains colon", "My:Account"},
		{"empty string", ""},
		{"reserved word assets", "assets"},
		{"reserved word Assets", "Assets"},
		{"reserved word liabilities", "liabilities"},
		{"reserved word equity", "Equity"},
		{"reserved word revenue", "Revenue"},
		{"reserved word expenses", "Expenses"},
		{"exceeds 100 chars", strings.Repeat("a", 101)},
	}
	for _, tt := range invalid {
		t.Run("invalid_"+tt.name, func(t *testing.T) {
			assert.Error(t, svc.ValidateAccountName(tt.input))
		})
	}
}

// ──────────────────────────────────────────────
// ValidateFullAccountName
// ──────────────────────────────────────────────

func TestValidateFullAccountName(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	valid := []struct {
		name  string
		input string
	}{
		{"two-level path", "Assets:Bank"},
		{"three-level path", "Assets:Bank:Checking"},
		{"lowercase root", "assets:Bank"},
		{"Liabilities root", "Liabilities:CreditCard"},
		{"Equity root", "Equity:Retained"},
		{"Revenue root", "Revenue:Salary"},
		{"Expenses root", "Expenses:Food"},
		{"root only", "Assets"},
	}
	for _, tt := range valid {
		t.Run("valid_"+tt.name, func(t *testing.T) {
			assert.NoError(t, svc.ValidateFullAccountName(tt.input))
		})
	}

	invalid := []struct {
		name  string
		input string
	}{
		{"non-reserved root", "Money:Bank"},
		{"empty string", ""},
		{"leading space", " Assets:Bank"},
		{"trailing space", "Assets:Bank "},
		{"segment with leading space", "Assets: Bank"},
		{"empty segment (double colon)", "Assets::Bank"},
		{"sub-segment is reserved word", "Assets:Liabilities"},
		{"sub-segment is reserved word lowercase", "Assets:assets"},
		{"exceeds 100 chars total", strings.Repeat("a", 101)},
	}
	for _, tt := range invalid {
		t.Run("invalid_"+tt.name, func(t *testing.T) {
			assert.Error(t, svc.ValidateFullAccountName(tt.input))
		})
	}
}

// ──────────────────────────────────────────────
// ValidateCurrency
// ──────────────────────────────────────────────

func TestValidateCurrency(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	valid := []struct{ name, input string }{
		{"USD uppercase", "USD"},
		{"twd lowercase auto-uppercased", "twd"},
		{"empty string allowed (uses system default)", ""},
		{"whitespace trimmed", " USD "},
	}
	for _, tt := range valid {
		t.Run("valid_"+tt.name, func(t *testing.T) {
			assert.NoError(t, svc.ValidateCurrency(tt.input))
		})
	}

	invalid := []struct{ name, input string }{
		{"two chars", "US"},
		{"four chars", "USDD"},
		{"contains digit", "US1"},
		{"contains symbol", "U$D"},
	}
	for _, tt := range invalid {
		t.Run("invalid_"+tt.name, func(t *testing.T) {
			assert.Error(t, svc.ValidateCurrency(tt.input))
		})
	}
}

// ──────────────────────────────────────────────
// CreateAccount
// ──────────────────────────────────────────────

func TestCreateAccount(t *testing.T) {
	t.Run("account created with correct fields", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Description: "My bank"})
		require.NoError(t, err)
		require.NotNil(t, acc)
		assert.Greater(t, acc.ID, int64(0))
		assert.Equal(t, "Assets:Bank", acc.Name)
		assert.Equal(t, model.AccountTypeAsset, acc.Type)
		assert.Equal(t, "USD", acc.Currency)
		assert.Equal(t, "My bank", acc.Description)
		assert.False(t, acc.IsHidden)
	})

	t.Run("invalid account name (non-reserved root) rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Money:Bank", Type: model.AccountTypeAsset, Currency: "USD"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid account name")
	})

	t.Run("invalid currency code rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "US"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid currency")
	})

	t.Run("invalid account type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountType("X"), Currency: "USD"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid account type")
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.createErr = errors.New("db error")
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})
		require.Error(t, err)
	})

	t.Run("duplicate account name returns ErrAlreadyExists", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrAlreadyExists), "expected ErrAlreadyExists, got: %v", err)
	})

	t.Run("lowercase currency is normalized to uppercase", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "usd", Description: "My bank"})
		require.NoError(t, err)
		assert.Equal(t, "USD", acc.Currency)
	})

	t.Run("currency with surrounding whitespace is trimmed and uppercased", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: " eur ", Description: "My bank"})
		require.NoError(t, err)
		assert.Equal(t, "EUR", acc.Currency)
	})
}

// ──────────────────────────────────────────────
// Account type vs root-segment validation
// ──────────────────────────────────────────────

func TestCreateAccount_RootTypeMismatch(t *testing.T) {
	t.Run("Expenses name with Asset type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Expenses:Food",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
		assert.Contains(t, err.Error(), "conflicts")
	})

	t.Run("Assets name with Expense type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Cash",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("Liabilities name with Revenue type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Liabilities:Loan",
			Type:     model.AccountTypeRevenue,
			Currency: "USD",
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("matching root and type accepted", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Expenses:Food",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, "Expenses:Food", acc.Name)
		assert.Equal(t, model.AccountTypeExpense, acc.Type)
	})

	t.Run("case-insensitive root matching accepted", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "assets:Cash",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeAsset, acc.Type)
	})
}

// ──────────────────────────────────────────────
// createOpeningBalance (tested via CreateAccountWithBalance)
// Verifies split direction — the most critical financial correctness test.
// ──────────────────────────────────────────────

func TestCreateAccountWithBalance_SplitDirection(t *testing.T) {
	t.Run("asset account positive balance: asset split positive, equity split negative", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Balance: 5000})
		require.NoError(t, err)

		splits := txRepo.splits[1] // first transaction ID=1
		require.Len(t, splits, 2)

		var assetSplit, equitySplit *model.Split
		for _, s := range splits {
			switch s.AccountID {
			case acc.ID:
				assetSplit = s
			case 99:
				equitySplit = s
			}
		}
		require.NotNil(t, assetSplit, "asset split missing")
		require.NotNil(t, equitySplit, "equity split missing")
		assert.Equal(t, int64(5000), assetSplit.Amount, "asset split should be +5000")
		assert.Equal(t, int64(-5000), equitySplit.Amount, "equity split should be -5000")
		// verify balance
		assert.Equal(t, int64(0), assetSplit.Amount+equitySplit.Amount)
	})

	t.Run("asset account negative balance: asset split negative, equity split positive", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:Loan", Type: model.AccountTypeAsset, Currency: "USD", Balance: -3000})
		require.NoError(t, err)

		splits := txRepo.splits[1]
		require.Len(t, splits, 2)

		var assetSplit, equitySplit *model.Split
		for _, s := range splits {
			if s.AccountID == acc.ID {
				assetSplit = s
			} else {
				equitySplit = s
			}
		}
		assert.Equal(t, int64(-3000), assetSplit.Amount)
		assert.Equal(t, int64(3000), equitySplit.Amount)
	})

	t.Run("liability account positive balance: liability split negative, equity split positive (reversed)", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Liabilities:CreditCard", Type: model.AccountTypeLiability, Currency: "USD", Balance: 2000})
		require.NoError(t, err)

		splits := txRepo.splits[1]
		require.Len(t, splits, 2)

		var liabSplit, equitySplit *model.Split
		for _, s := range splits {
			if s.AccountID == acc.ID {
				liabSplit = s
			} else {
				equitySplit = s
			}
		}
		assert.Equal(t, int64(-2000), liabSplit.Amount, "liability split should be -2000")
		assert.Equal(t, int64(2000), equitySplit.Amount, "equity split should be +2000")
	})

	t.Run("zero balance does not create opening transaction", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, txRepo)

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Balance: 0})
		require.NoError(t, err)
		assert.Empty(t, txRepo.transactions, "no transaction should be created for zero balance")
	})

	t.Run("expense account type not supported for opening balance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD", Balance: 500})
		require.Error(t, err)
		assert.Nil(t, acc, "account should be nil on unsupported type")
		assert.Contains(t, err.Error(), "opening balance")

		_, lookupErr := accRepo.GetAccountByName(context.Background(), "Expenses:Food")
		assert.ErrorIs(t, lookupErr, repository.ErrNotFound, "account should not exist after unsupported-type rollback")
	})

	t.Run("missing Equity:OpeningBalances_USD account is auto-created", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		// intentionally omit the opening balance account
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Balance: 1000})
		require.NoError(t, err)

		equityName := model.OpeningBalancesAccountName("USD")
		equityAcc, err := accRepo.GetAccountByName(context.Background(), equityName)
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeEquity, equityAcc.Type)
		assert.Equal(t, "USD", equityAcc.Currency)
	})

	t.Run("opening balance failure rolls back account creation", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		txRepo.createErr = errors.New("forced DB error")
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", Balance: 5000})
		require.Error(t, err)
		assert.Nil(t, acc, "account should be nil on rollback")

		_, lookupErr := accRepo.GetAccountByName(context.Background(), "Assets:Bank")
		assert.ErrorIs(t, lookupErr, repository.ErrNotFound, "account should not exist after rollback")
	})
}

func TestCreateAccountWithBalance_CurrencyRouting(t *testing.T) {
	t.Run("account currency TWD uses Equity:OpeningBalances_TWD", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalancesForCurrency(accRepo, "TWD")
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:TWDBank", Type: model.AccountTypeAsset, Currency: "TWD", Balance: 30000})
		require.NoError(t, err)

		splits := txRepo.splits[1]
		require.Len(t, splits, 2)

		for _, s := range splits {
			assert.Equal(t, "TWD", s.Currency, "all splits must use TWD")
		}

		var assetSplit *model.Split
		for _, s := range splits {
			if s.AccountID == acc.ID {
				assetSplit = s
			}
		}
		require.NotNil(t, assetSplit)
		assert.Equal(t, int64(30000), assetSplit.Amount)
	})

	t.Run("account with empty currency falls back to system default (USD)", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalancesForCurrency(accRepo, "USD")
		svc := newTestAccountService(accRepo, txRepo)

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "", Balance: 5000})
		require.NoError(t, err)

		splits := txRepo.splits[1]
		require.Len(t, splits, 2)
		for _, s := range splits {
			assert.Equal(t, "USD", s.Currency)
		}
	})

	t.Run("auto-creates Equity:OpeningBalances_TWD when missing", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		// no TWD equity account pre-seeded
		svc := newTestAccountService(accRepo, txRepo)

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{Name: "Assets:TWDBank", Type: model.AccountTypeAsset, Currency: "TWD", Balance: 30000})
		require.NoError(t, err)

		// equity account should now exist
		equityName := model.OpeningBalancesAccountName("TWD")
		equityAcc, err := accRepo.GetAccountByName(context.Background(), equityName)
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeEquity, equityAcc.Type)
		assert.Equal(t, "TWD", equityAcc.Currency)
	})
}

// ──────────────────────────────────────────────
// GetAccountByName
// ──────────────────────────────────────────────

func TestGetAccountByName(t *testing.T) {
	t.Run("unknown account returns ErrNotFound", func(t *testing.T) {
		repo := newMockAccountRepo()
		svc := newTestAccountService(repo, newMockTransactionRepo())

		_, err := svc.GetAccountByName(context.Background(), "Assets:Nonexistent")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
	})
}

// ──────────────────────────────────────────────
// DeleteAccountByName
// ──────────────────────────────────────────────

func TestDeleteAccountByName(t *testing.T) {
	t.Run("account with no children and no transactions deleted successfully", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:OldAccount"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(context.Background(), "Assets:OldAccount")
		require.NoError(t, err)
		_, err = accRepo.GetAccountByName(context.Background(), "Assets:OldAccount")
		assert.Error(t, err, "account should not exist after deletion")
	})

	t.Run("system account Equity:OpeningBalances_USD rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(context.Background(), model.OpeningBalancesAccountName("USD"))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
	})

	t.Run("any Equity:OpeningBalances_* account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 10, Name: "Equity:OpeningBalances_TWD", Type: model.AccountTypeEquity})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(context.Background(), "Equity:OpeningBalances_TWD")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
	})

	t.Run("account with child accounts rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank"})
		accRepo.childMap[5] = true
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(context.Background(), "Assets:Bank")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child")
	})

	t.Run("account with transactions rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank"})
		accRepo.txExistsMap[5] = true
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(context.Background(), "Assets:Bank")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transactions")
	})

	t.Run("non-existent account returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.DeleteAccountByName(context.Background(), "Assets:NonExistent")
		require.Error(t, err)
	})

	t.Run("non-existent account returns ErrNotFound", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(context.Background(), "Assets:DoesNotExist")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got: %v", err)
	})
}

// ──────────────────────────────────────────────
// FormatAccountName
// ──────────────────────────────────────────────

func TestFormatAccountName(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		prefix, name, want string
	}{
		{"Assets", "Bank", "Assets:Bank"},
		{"", "Assets", "Assets"},
		{"Assets:Bank", "Checking", "Assets:Bank:Checking"},
	}
	for _, tt := range tests {
		t.Run(tt.prefix+"_"+tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, svc.FormatAccountName(tt.prefix, tt.name))
		})
	}
}

// ──────────────────────────────────────────────
// GetAccountBalance
// ──────────────────────────────────────────────

func TestGetAccountBalance(t *testing.T) {
	repo := newMockAccountRepo()
	svc := newTestAccountService(repo, newMockTransactionRepo())

	repo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset})
	repo.balances[1] = 5000 // 50.00 in cents

	t.Run("returns raw cents", func(t *testing.T) {
		got, err := svc.GetAccountBalance(context.Background(), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), got)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo.addAccount(&model.Account{ID: 99, Name: "Assets:Test", Type: model.AccountTypeAsset})
		repo.getBalanceErr[99] = errors.New("balance fetch failed")
		_, err := svc.GetAccountBalance(context.Background(), 99)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "balance fetch failed")
	})

	t.Run("returns ErrNotFound for nonexistent account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalance(context.Background(), 999)

		assert.Equal(t, int64(0), bal)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors from GetAccountByID", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalance(context.Background(), 42)

		assert.Equal(t, int64(0), bal)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}

// ──────────────────────────────────────────────
// RenameAccount
// ──────────────────────────────────────────────

func TestRenameAccount(t *testing.T) {
	t.Run("leaf account renamed to new full name", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		got, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:Savings")
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, "Assets:Bank", accRepo.renameCalls[0].old)
		assert.Equal(t, "Assets:Savings", accRepo.renameCalls[0].new)

		assert.Equal(t, "Assets:Savings", got.Name)
		assert.Equal(t, int64(1), got.ID)
	})

	t.Run("repo called with old full name and new full name", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		got, err := svc.RenameAccount(context.Background(), "Assets:Bank:Checking", "Assets:Bank:Current")
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, "Assets:Bank:Checking", accRepo.renameCalls[0].old)
		assert.Equal(t, "Assets:Bank:Current", accRepo.renameCalls[0].new)

		assert.Equal(t, "Assets:Bank:Current", got.Name)
	})

	t.Run("empty trailing segment rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:")
		require.Error(t, err)
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("empty name rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.RenameAccount(context.Background(), "Assets:Bank", "")
		require.Error(t, err)
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("rename changing parent path is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Liabilities:Bank")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rename cannot change parent path")
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("new full name already exists is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Savings", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:Savings")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("system account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.RenameAccount(context.Background(), model.OpeningBalancesAccountName("USD"), "Equity:Other")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.RenameAccount(context.Background(), "Assets:Ghost", "Assets:NewName")
		require.Error(t, err)
	})

	t.Run("legacy opening-balances account renamed to currency-suffixed full path", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID:   1,
			Name: model.LegacyOpeningBalancesName,
			Type: model.AccountTypeEquity,
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		got, err := svc.RenameAccount(context.Background(), model.LegacyOpeningBalancesName, model.OpeningBalancesAccountName("USD"))
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, model.LegacyOpeningBalancesName, accRepo.renameCalls[0].old)
		assert.Equal(t, model.OpeningBalancesAccountName("USD"), accRepo.renameCalls[0].new)

		assert.Equal(t, model.OpeningBalancesAccountName("USD"), got.Name)
	})

	t.Run("ExecTx failure propagates error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		tm := &mockTransactionManager{accRepo: accRepo, txRepo: newMockTransactionRepo(), failTx: true}
		svc := NewAccountService(accRepo, defaultConfig(), tm)

		_, err := svc.RenameAccount(context.Background(), "Assets:Bank", "Assets:Savings")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "forced failure")
	})
}

// ──────────────────────────────────────────────
// UpdateAccountMetadata
// ──────────────────────────────────────────────

func TestUpdateAccountMetadata(t *testing.T) {
	t.Run("description and hidden updated correctly", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Description: "old desc", IsHidden: false})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		got, err := svc.UpdateAccountMetadata(context.Background(), 1, "new desc", true)
		require.NoError(t, err)

		require.Len(t, accRepo.updateMetadataCalls, 1)
		call := accRepo.updateMetadataCalls[0]
		assert.Equal(t, int64(1), call.id)
		assert.Equal(t, "new desc", call.description)
		assert.True(t, call.isHidden)

		assert.Equal(t, "new desc", got.Description)
		assert.True(t, got.IsHidden)
		assert.Equal(t, int64(1), got.ID)
	})

	t.Run("system account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 99, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.UpdateAccountMetadata(context.Background(), 99, "desc", false)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
		assert.Empty(t, accRepo.updateMetadataCalls)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.UpdateAccountMetadata(context.Background(), 999, "desc", false)
		require.Error(t, err)
	})

	t.Run("repo error propagated", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		accRepo.updateMetadataErr = errors.New("db error")
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.UpdateAccountMetadata(context.Background(), 1, "desc", false)
		require.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// Parent chain validation
// ──────────────────────────────────────────────

func int64Ptr(v int64) *int64 {
	return &v
}

func TestCreateAccount_ParentValidation(t *testing.T) {
	t.Run("valid parent accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		parent := &model.Account{ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"}
		accRepo.addAccount(parent)

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(10)})
		require.NoError(t, err)
		require.NotNil(t, acc.ParentID)
		assert.Equal(t, int64(10), *acc.ParentID)
	})

	t.Run("non-existent parent rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(999)})
		require.Error(t, err)
	})

	t.Run("deep parent chain accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Assets:Bank:Savings", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(2)})

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:Bank:Savings:Sub", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(3)})
		require.NoError(t, err)
		require.NotNil(t, acc.ParentID)
		assert.Equal(t, int64(3), *acc.ParentID)
	})

	t.Run("circular parent chain rejected via CreateAccount", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		// Circular: A(1)->C(3), B(2)->A(1), C(3)->B(2)
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:A", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(3)})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:B", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Assets:C", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(2)})

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{Name: "Assets:C:Child", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(3)})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrCircularParent), "expected ErrCircularParent, got: %v", err)
	})
}

func TestValidateParentChain(t *testing.T) {
	t.Run("detects cycle", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		// Circular: A(1)->C(3), B(2)->A(1), C(3)->B(2)
		accRepo.addAccount(&model.Account{ID: 1, Name: "A", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(3)})
		accRepo.addAccount(&model.Account{ID: 2, Name: "B", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "C", Type: model.AccountTypeAsset, Currency: "USD", ParentID: int64Ptr(2)})

		err := svc.validateParentChain(context.Background(), 99, int64Ptr(3))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrCircularParent), "expected ErrCircularParent, got: %v", err)
	})

	t.Run("nil parent is valid", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		err := svc.validateParentChain(context.Background(), 1, nil)
		require.NoError(t, err)
	})

	t.Run("repo error propagated", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		accRepo.getByIDErr[50] = repository.ErrNotFound

		err := svc.validateParentChain(context.Background(), 1, int64Ptr(50))
		require.Error(t, err)
	})

	t.Run("detects self-parent on reparent", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD"})

		err := svc.validateParentChain(context.Background(), 5, int64Ptr(5))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrCircularParent))
	})
}

// ──────────────────────────────────────────────
// Account type vs parent type validation
// ──────────────────────────────────────────────

func TestCreateAccount_ParentTypeMismatch(t *testing.T) {
	t.Run("child type differs from parent type rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Dining",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("child type matches parent type accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Savings",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeAsset, acc.Type)
	})

	t.Run("parent type mismatch rejected via CreateAccountWithBalance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Dining",
			Type:     model.AccountTypeExpense,
			Currency: "USD",
			ParentID: int64Ptr(10),
			Balance:  1000,
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "type", ve.Field)
	})

	t.Run("nil parent skips parent type check", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Revenue:Sales",
			Type:     model.AccountTypeRevenue,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeRevenue, acc.Type)
	})
}

func TestCreateAccount_ParentNameConsistency(t *testing.T) {
	t.Run("name under different branch than parent rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Expenses:Food",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
		assert.Contains(t, err.Error(), "Assets:Bank")
	})

	t.Run("name shares prefix but adds extra nesting rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Sub:Deep",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
	})

	t.Run("correct child name accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank:Checking", acc.Name)
	})

	t.Run("mismatch rejected via CreateAccountWithBalance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Cash",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
			Balance:  1000,
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
	})

	t.Run("nil parent skips name consistency check", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank", acc.Name)
	})
}

// ──────────────────────────────────────────────
// Parent with transactions guard (#150)
// ──────────────────────────────────────────────

// ──────────────────────────────────────────────
// Description length validation
// ──────────────────────────────────────────────

func TestCreateAccount_DescriptionValidation(t *testing.T) {
	t.Run("empty description is allowed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Assets:Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: "",
		}
		acc, err := svc.CreateAccount(context.Background(), input)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})

	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Assets:Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: strings.Repeat("d", model.DescriptionMaxLength+1),
		}
		acc, err := svc.CreateAccount(context.Background(), input)
		assert.Nil(t, acc)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("exactly max-length description accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Assets:Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: strings.Repeat("d", model.DescriptionMaxLength),
		}
		acc, err := svc.CreateAccount(context.Background(), input)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})
}

func TestCreateAccountWithBalance_DescriptionValidation(t *testing.T) {
	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalancesForCurrency(accRepo, "USD")
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		input := model.CreateAccountInput{
			Name:        "Assets:Checking",
			Type:        model.AccountTypeAsset,
			Currency:    "USD",
			Description: strings.Repeat("d", model.DescriptionMaxLength+1),
			Balance:     10000,
		}
		acc, err := svc.CreateAccountWithBalance(context.Background(), input)
		assert.Nil(t, acc)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
	})
}

// ──────────────────────────────────────────────
// Parent with transactions guard (#150)
// ──────────────────────────────────────────────

func TestCreateAccount_ParentHasTransactions(t *testing.T) {
	t.Run("parent with transactions rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		accRepo.txExistsMap[10] = true
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
		assert.Contains(t, err.Error(), "transactions")
	})

	t.Run("parent without transactions accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		// txExistsMap[10] defaults to false
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank:Checking", acc.Name)
	})

	t.Run("nil parent skips transaction check", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

		acc, err := svc.CreateAccount(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank", acc.Name)
	})

	t.Run("rejected via CreateAccountWithBalance too", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{
			ID: 10, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD",
		})
		accRepo.txExistsMap[10] = true
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance(context.Background(), model.CreateAccountInput{
			Name:     "Assets:Bank:Checking",
			Type:     model.AccountTypeAsset,
			Currency: "USD",
			ParentID: int64Ptr(10),
			Balance:  1000,
		})
		require.Error(t, err)

		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "parent", ve.Field)
		assert.Contains(t, err.Error(), "transactions")
	})
}
