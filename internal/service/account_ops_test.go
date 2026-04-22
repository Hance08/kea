package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/hance08/kea/internal/model"
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

		acc, err := svc.CreateAccount("Assets:Bank", model.AccountTypeAsset, "USD", "My bank", nil)
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
		_, err := svc.CreateAccount("Money:Bank", model.AccountTypeAsset, "USD", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid account name")
	})

	t.Run("invalid currency code rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateAccount("Assets:Bank", model.AccountTypeAsset, "US", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid currency")
	})

	t.Run("invalid account type rejected", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateAccount("Assets:Bank", model.AccountType("X"), "USD", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid account type")
	})

	t.Run("repo failure returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.createErr = errors.New("db error")
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccount("Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
		require.Error(t, err)
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

		acc, err := svc.CreateAccountWithBalance("Assets:Bank", model.AccountTypeAsset, "USD", "", nil, 5000)
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

		acc, err := svc.CreateAccountWithBalance("Assets:Loan", model.AccountTypeAsset, "USD", "", nil, -3000)
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

		acc, err := svc.CreateAccountWithBalance("Liabilities:CreditCard", model.AccountTypeLiability, "USD", "", nil, 2000)
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

		_, err := svc.CreateAccountWithBalance("Assets:Bank", model.AccountTypeAsset, "USD", "", nil, 0)
		require.NoError(t, err)
		assert.Empty(t, txRepo.transactions, "no transaction should be created for zero balance")
	})

	t.Run("expense account type not supported for opening balance", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, txRepo)

		_, err := svc.CreateAccountWithBalance("Expenses:Food", model.AccountTypeExpense, "USD", "", nil, 500)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "opening balance")
	})

	t.Run("missing Equity:OpeningBalances_USD account is auto-created", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		// intentionally omit the opening balance account
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		_, err := svc.CreateAccountWithBalance("Assets:Bank", model.AccountTypeAsset, "USD", "", nil, 1000)
		require.NoError(t, err)

		equityName := model.OpeningBalancesAccountName("USD")
		equityAcc, err := accRepo.GetAccountByName(equityName)
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeEquity, equityAcc.Type)
		assert.Equal(t, "USD", equityAcc.Currency)
	})
}

func TestCreateAccountWithBalance_CurrencyRouting(t *testing.T) {
	t.Run("account currency TWD uses Equity:OpeningBalances_TWD", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		addOpeningBalancesForCurrency(accRepo, "TWD")
		svc := newTestAccountService(accRepo, txRepo)

		acc, err := svc.CreateAccountWithBalance("Assets:TWDBank", model.AccountTypeAsset, "TWD", "", nil, 30000)
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

		_, err := svc.CreateAccountWithBalance("Assets:Bank", model.AccountTypeAsset, "", "", nil, 5000)
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

		_, err := svc.CreateAccountWithBalance("Assets:TWDBank", model.AccountTypeAsset, "TWD", "", nil, 30000)
		require.NoError(t, err)

		// equity account should now exist
		equityName := model.OpeningBalancesAccountName("TWD")
		equityAcc, err := accRepo.GetAccountByName(equityName)
		require.NoError(t, err)
		assert.Equal(t, model.AccountTypeEquity, equityAcc.Type)
		assert.Equal(t, "TWD", equityAcc.Currency)
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

		err := svc.DeleteAccountByName("Assets:OldAccount")
		require.NoError(t, err)
		_, err = accRepo.GetAccountByName("Assets:OldAccount")
		assert.Error(t, err, "account should not exist after deletion")
	})

	t.Run("system account Equity:OpeningBalances_USD rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName(model.OpeningBalancesAccountName("USD"))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
	})

	t.Run("any Equity:OpeningBalances_* account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 10, Name: "Equity:OpeningBalances_TWD", Type: model.AccountTypeEquity})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName("Equity:OpeningBalances_TWD")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
	})

	t.Run("account with child accounts rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank"})
		accRepo.childMap[5] = true
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName("Assets:Bank")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "child")
	})

	t.Run("account with transactions rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 5, Name: "Assets:Bank"})
		accRepo.txExistsMap[5] = true
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.DeleteAccountByName("Assets:Bank")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transactions")
	})

	t.Run("non-existent account returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.DeleteAccountByName("Assets:NonExistent")
		require.Error(t, err)
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
		got, err := svc.GetAccountBalance(1)
		require.NoError(t, err)
		assert.Equal(t, int64(5000), got)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo.getBalanceErr[99] = errors.New("not found")
		_, err := svc.GetAccountBalance(99)
		assert.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// RenameAccount
// ──────────────────────────────────────────────

func TestRenameAccount(t *testing.T) {
	t.Run("leaf account renamed with correct full name constructed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "Savings")
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, "Assets:Bank", accRepo.renameCalls[0].old)
		assert.Equal(t, "Assets:Savings", accRepo.renameCalls[0].new)
	})

	t.Run("repo called with old full name and new full name (not bare segment)", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank:Checking", "Current")
		require.NoError(t, err)

		require.Len(t, accRepo.renameCalls, 1)
		assert.Equal(t, "Assets:Bank:Checking", accRepo.renameCalls[0].old)
		assert.Equal(t, "Assets:Bank:Current", accRepo.renameCalls[0].new)
	})

	t.Run("segment containing colon rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "Bad:Name")
		require.Error(t, err)
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("empty segment rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "")
		require.Error(t, err)
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("new full name already exists is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Savings", Type: model.AccountTypeAsset})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount("Assets:Bank", "Savings")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("system account is rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		addOpeningBalanceAccount(accRepo)
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		err := svc.RenameAccount(model.OpeningBalancesAccountName("USD"), "Other")
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable))
		assert.Empty(t, accRepo.renameCalls)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.RenameAccount("Assets:Ghost", "NewName")
		require.Error(t, err)
	})
}
