// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// Test helpers: build SplitDetail with AccountType
// ──────────────────────────────────────────────

func split(accName string, accType model.AccountType, amount int64) model.SplitDetail {
	return model.SplitDetail{
		AccountName: accName,
		AccountType: accType,
		Amount:      amount,
		Currency:    "USD",
	}
}

func splitWithMemo(accName string, accType model.AccountType, amount int64, memo string) model.SplitDetail {
	s := split(accName, accType, amount)
	s.Memo = memo
	return s
}

// classifierAccRepo returns a mock account repository pre-populated with all
// accounts referenced by classifier tests, allowing resolveAccountType to
// look up types by name without hitting a real database.
func classifierAccRepo() *mockAccountRepo {
	repo := newMockAccountRepo()
	repo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 3, Name: "Revenue:Salary", Type: model.AccountTypeRevenue})
	repo.addAccount(&model.Account{ID: 4, Name: "Liabilities:Card", Type: model.AccountTypeLiability})
	repo.addAccount(&model.Account{ID: 5, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
	repo.addAccount(&model.Account{ID: 6, Name: "Assets:Cash", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 7, Name: "Assets:Savings", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 8, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 9, Name: "Equity:Retained", Type: model.AccountTypeEquity})
	repo.addAccount(&model.Account{ID: 10, Name: "Expenses:Drink", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 11, Name: "Revenue:Bonus", Type: model.AccountTypeRevenue})
	repo.addAccount(&model.Account{ID: 12, Name: "Expenses:Tax", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 13, Name: "Expenses:A", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 14, Name: "Expenses:B", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 15, Name: "Assets:Investments:00878", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 16, Name: "Expenses:Fees:Stocks", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 17, Name: "Assets:Bank:DAWHO", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 18, Name: "Expenses:Food:Drink", Type: model.AccountTypeExpense})
	repo.addAccount(&model.Account{ID: 19, Name: "Assets:Receivable:Friends", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 20, Name: "Assets:Ewallet:LinePayMoney", Type: model.AccountTypeAsset})
	repo.addAccount(&model.Account{ID: 21, Name: model.OpeningBalancesAccountName("TWD"), Type: model.AccountTypeEquity})
	repo.addAccount(&model.Account{ID: 22, Name: "Assets:Card", Type: model.AccountTypeAsset})
	return repo
}

// ──────────────────────────────────────────────
// DetermineType
// ──────────────────────────────────────────────

func TestDetermineType(t *testing.T) {
	svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())

	tests := []struct {
		name   string
		splits []model.SplitDetail
		want   model.TransactionType
	}{
		{
			name: "expense: E + A",
			splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 500),
				split("Assets:Bank", model.AccountTypeAsset, -500),
			},
			want: model.TxTypeExpense,
		},
		{
			name: "income: R + A",
			splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, -3000),
				split("Assets:Bank", model.AccountTypeAsset, 3000),
			},
			want: model.TxTypeIncome,
		},
		{
			name: "transfer: A + A",
			splits: []model.SplitDetail{
				split("Assets:Savings", model.AccountTypeAsset, 1000),
				split("Assets:Checking", model.AccountTypeAsset, -1000),
			},
			want: model.TxTypeTransfer,
		},
		{
			name: "transfer: A + L",
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 1000),
				split("Liabilities:Card", model.AccountTypeLiability, -1000),
			},
			want: model.TxTypeTransfer,
		},
		{
			name: "opening: memo = Opening Balance",
			splits: []model.SplitDetail{
				splitWithMemo("Assets:Bank", model.AccountTypeAsset, 5000, model.OpeningAccountMemo),
				splitWithMemo(model.OpeningBalancesAccountName("USD"), model.AccountTypeEquity, -5000, model.OpeningAccountMemo),
			},
			want: model.TxTypeOpening,
		},
		{
			name: "deposit: equity decreases, asset increases",
			splits: []model.SplitDetail{
				split("Equity:Retained", model.AccountTypeEquity, -1000),
				split("Assets:Bank", model.AccountTypeAsset, 1000),
			},
			want: model.TxTypeDeposit,
		},
		{
			name: "withdrawal: equity increases, asset decreases",
			splits: []model.SplitDetail{
				split("Equity:Retained", model.AccountTypeEquity, 500),
				split("Assets:Bank", model.AccountTypeAsset, -500),
			},
			want: model.TxTypeWithdrawal,
		},
		{
			name: "mixed revenue and expense (income >= expense) → Income",
			splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, -1000), // AbsInt64 = 1000
				split("Expenses:Tax", model.AccountTypeExpense, 300),
				split("Assets:Bank", model.AccountTypeAsset, 700),
			},
			want: model.TxTypeIncome,
		},
		{
			name: "mixed revenue and expense (expense > income) → Expense",
			splits: []model.SplitDetail{
				split("Revenue:Bonus", model.AccountTypeRevenue, -200), // AbsInt64 = 200
				split("Expenses:Food", model.AccountTypeExpense, 500),
				split("Assets:Bank", model.AccountTypeAsset, -300),
			},
			want: model.TxTypeExpense,
		},
		{
			name: "mixed revenue and expense (income == expense) → Income (>= condition)",
			splits: []model.SplitDetail{
				split("Revenue:Bonus", model.AccountTypeRevenue, -400), // AbsInt64 = 400
				split("Expenses:Food", model.AccountTypeExpense, 400),
			},
			want: model.TxTypeIncome,
		},
		{
			name:   "empty splits → Other",
			splits: []model.SplitDetail{},
			want:   model.TxTypeOther,
		},
		{
			name: "unclassifiable → Other",
			splits: []model.SplitDetail{
				split("Expenses:A", model.AccountTypeExpense, 100),
				split("Expenses:B", model.AccountTypeExpense, -100),
			},
			want: model.TxTypeOther,
		},
		{
			name: "transfer with fee: A/L dominates E → Transfer",
			splits: []model.SplitDetail{
				split("Assets:Investments:00878", model.AccountTypeAsset, 5160),
				split("Expenses:Fees:Stocks", model.AccountTypeExpense, 7),
				split("Assets:Bank:DAWHO", model.AccountTypeAsset, -5167),
			},
			want: model.TxTypeTransfer,
		},
		{
			name: "expense split bill: E ties A/L → Expense",
			splits: []model.SplitDetail{
				split("Expenses:Food:Drink", model.AccountTypeExpense, 60),
				split("Assets:Receivable:Friends", model.AccountTypeAsset, 60),
				split("Assets:Ewallet:LinePayMoney", model.AccountTypeAsset, -120),
			},
			want: model.TxTypeExpense,
		},
		{
			name: "expense dominant: E > A/L → Expense",
			splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 100),
				split("Assets:Receivable:Friends", model.AccountTypeAsset, 40),
				split("Assets:Bank", model.AccountTypeAsset, -140),
			},
			want: model.TxTypeExpense,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.DetermineType(context.Background(), tt.splits)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetermineType_FallbackToRepo(t *testing.T) {
	t.Run("empty AccountType triggers repo lookup", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		// intentionally omit AccountType to force classifier to query repo
		splits := []model.SplitDetail{
			{AccountID: 1, AccountName: "Expenses:Food", Amount: 500},
			{AccountID: 2, AccountName: "Assets:Bank", Amount: -500},
		}
		got, err := svc.DetermineType(context.Background(), splits)
		require.NoError(t, err)
		assert.Equal(t, model.TxTypeExpense, got)
	})

	t.Run("repo lookup failure returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		// account ID 99 does not exist
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		splits := []model.SplitDetail{
			{AccountID: 99, AccountName: "Unknown", Amount: 100},
		}
		_, err := svc.DetermineType(context.Background(), splits)
		assert.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// GetDisplayAmount
// ──────────────────────────────────────────────

func TestGetDisplayAmount(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("empty splits returns 0 and empty currency", func(t *testing.T) {
		amount, currency := svc.GetDisplayAmount(nil)
		assert.Equal(t, int64(0), amount)
		assert.Equal(t, "", currency)
	})

	t.Run("selects the largest positive value", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: 500, Currency: "USD"},
			{Amount: 1000, Currency: "TWD"},
		}
		amount, currency := svc.GetDisplayAmount(splits)
		assert.Equal(t, int64(1000), amount)
		assert.Equal(t, "TWD", currency)
	})

	t.Run("single split", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: 300, Currency: "USD"},
		}
		amount, currency := svc.GetDisplayAmount(splits)
		assert.Equal(t, int64(300), amount)
		assert.Equal(t, "USD", currency)
	})

	t.Run("all negative splits: maxAmount is 0, currency taken from first split", func(t *testing.T) {
		splits := []model.SplitDetail{
			{Amount: -500, Currency: "USD"},
			{Amount: -200, Currency: "TWD"},
		}
		amount, currency := svc.GetDisplayAmount(splits)
		assert.Equal(t, int64(0), amount)
		assert.Equal(t, "USD", currency) // initialized from the first split
	})
}

// ──────────────────────────────────────────────
// GetDisplayAccount
// ──────────────────────────────────────────────

func TestGetDisplayAccount(t *testing.T) {
	t.Run("Expense → returns the expense account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Cash", Type: model.AccountTypeAsset})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		splits := []model.SplitDetail{
			{AccountName: "Expenses:Food", Amount: 500},
			{AccountName: "Assets:Cash", Amount: -500},
		}
		got, err := svc.GetDisplayAccount(context.Background(), splits, "Expense")
		require.NoError(t, err)
		assert.Equal(t, "Expenses:Food", got)
	})

	t.Run("Income → returns the revenue account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Revenue:Salary", Type: model.AccountTypeRevenue})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		splits := []model.SplitDetail{
			{AccountName: "Revenue:Salary", Amount: -3000},
			{AccountName: "Assets:Bank", Amount: 3000},
		}
		got, err := svc.GetDisplayAccount(context.Background(), splits, "Income")
		require.NoError(t, err)
		assert.Equal(t, "Revenue:Salary", got)
	})

	t.Run("Transfer → returns the asset/liability account with a positive amount", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Savings", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Checking", Type: model.AccountTypeAsset})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		splits := []model.SplitDetail{
			{AccountName: "Assets:Savings", Amount: 1000},
			{AccountName: "Assets:Checking", Amount: -1000},
		}
		got, err := svc.GetDisplayAccount(context.Background(), splits, "Transfer")
		require.NoError(t, err)
		assert.Equal(t, "Assets:Savings", got)
	})

	t.Run("Opening → returns the non-equity account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset})
		accRepo.addAccount(&model.Account{ID: 2, Name: model.OpeningBalancesAccountName("USD"), Type: model.AccountTypeEquity})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		splits := []model.SplitDetail{
			{AccountName: "Assets:Cash", Amount: 5000},
			{AccountName: model.OpeningBalancesAccountName("USD"), Amount: -5000},
		}
		got, err := svc.GetDisplayAccount(context.Background(), splits, "Opening")
		require.NoError(t, err)
		assert.Equal(t, "Assets:Cash", got)
	})

	t.Run("empty splits returns -", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		got, err := svc.GetDisplayAccount(context.Background(), nil, "Expense")
		require.NoError(t, err)
		assert.Equal(t, "-", got)
	})
}

// ──────────────────────────────────────────────
// GetDisplayOffsetAccount
// ──────────────────────────────────────────────

func TestGetDisplayOffsetAccount(t *testing.T) {
	t.Run("Expense: single offset (asset account)", func(t *testing.T) {
		svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
			{AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		got, err := svc.GetDisplayOffsetAccount(context.Background(), splits, "Expense", "Expenses:Food")
		require.NoError(t, err)
		assert.Equal(t, "Assets:Cash", got)
	})

	t.Run("Expense: multiple offsets returns (multiple)", func(t *testing.T) {
		svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
			{AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -300},
			{AccountName: "Assets:Card", AccountType: model.AccountTypeAsset, Amount: -200},
		}
		got, err := svc.GetDisplayOffsetAccount(context.Background(), splits, "Expense", "Expenses:Food")
		require.NoError(t, err)
		assert.Equal(t, "(multiple)", got)
	})

	t.Run("Transfer: excludes primary account by name", func(t *testing.T) {
		svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountName: "Assets:Savings", AccountType: model.AccountTypeAsset, Amount: 1000},
			{AccountName: "Assets:Checking", AccountType: model.AccountTypeAsset, Amount: -1000},
		}
		got, err := svc.GetDisplayOffsetAccount(context.Background(), splits, "Transfer", "Assets:Savings")
		require.NoError(t, err)
		assert.Equal(t, "Assets:Checking", got)
	})

	t.Run("no offset accounts returns -", func(t *testing.T) {
		svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
		splits := []model.SplitDetail{
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
		}
		got, err := svc.GetDisplayOffsetAccount(context.Background(), splits, "Expense", "Expenses:Food")
		require.NoError(t, err)
		assert.Equal(t, "-", got)
	})

	t.Run("empty splits returns -", func(t *testing.T) {
		svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())
		got, err := svc.GetDisplayOffsetAccount(context.Background(), nil, "Expense", "")
		require.NoError(t, err)
		assert.Equal(t, "-", got)
	})
}

// ──────────────────────────────────────────────
// GetAllowedAccounts
// ──────────────────────────────────────────────

func TestGetAllowedAccounts(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	allAccounts := []*model.Account{
		{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset},
		{ID: 2, Name: "Liabilities:Card", Type: model.AccountTypeLiability},
		{ID: 3, Name: "Expenses:Food", Type: model.AccountTypeExpense},
		{ID: 4, Name: "Revenue:Salary", Type: model.AccountTypeRevenue},
		{ID: 5, Name: "Equity:Retained", Type: model.AccountTypeEquity},
	}

	t.Run("Expense + currentType=Expense → only expense accounts", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeExpense, model.AccountTypeExpense, allAccounts)
		require.Len(t, result, 1)
		assert.Equal(t, model.AccountTypeExpense, result[0].Type)
	})

	t.Run("Expense + currentType=Asset → only asset and liability accounts", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeExpense, model.AccountTypeAsset, allAccounts)
		require.Len(t, result, 2)
		for _, acc := range result {
			assert.Contains(t, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability}, acc.Type)
		}
	})

	t.Run("Income + currentType=Revenue → only revenue accounts", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeIncome, model.AccountTypeRevenue, allAccounts)
		require.Len(t, result, 1)
		assert.Equal(t, model.AccountTypeRevenue, result[0].Type)
	})

	t.Run("Income + currentType=Asset → only asset and liability accounts", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeIncome, model.AccountTypeAsset, allAccounts)
		require.Len(t, result, 2)
	})

	t.Run("Transfer → only asset and liability accounts", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeTransfer, model.AccountTypeAsset, allAccounts)
		require.Len(t, result, 2)
		for _, acc := range result {
			assert.Contains(t, []model.AccountType{model.AccountTypeAsset, model.AccountTypeLiability}, acc.Type)
		}
	})

	t.Run("Other → returns all accounts", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeOther, model.AccountTypeExpense, allAccounts)
		assert.Len(t, result, len(allAccounts))
	})

	t.Run("empty account list returns empty slice", func(t *testing.T) {
		result := svc.GetAllowedAccounts(model.TxTypeExpense, model.AccountTypeExpense, nil)
		assert.Empty(t, result)
	})
}

// ──────────────────────────────────────────────
// ValidateSplitsMatchType
// ──────────────────────────────────────────────

func TestValidateSplitsMatchType(t *testing.T) {
	svc := newTestTransactionService(classifierAccRepo(), newMockTransactionRepo())

	tests := []struct {
		name    string
		txType  model.TransactionType
		splits  []model.SplitDetail
		wantErr bool
	}{
		{
			name:   "expense: E + A is valid",
			txType: model.TxTypeExpense,
			splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 500),
				split("Assets:Bank", model.AccountTypeAsset, -500),
			},
			wantErr: false,
		},
		{
			name:   "expense: missing E account",
			txType: model.TxTypeExpense,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Assets:Cash", model.AccountTypeAsset, -500),
			},
			wantErr: true,
		},
		{
			name:   "expense: missing A/L account",
			txType: model.TxTypeExpense,
			splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 500),
				split("Expenses:Drink", model.AccountTypeExpense, -500),
			},
			wantErr: true,
		},
		{
			name:   "income: R + A is valid",
			txType: model.TxTypeIncome,
			splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, -1000),
				split("Assets:Bank", model.AccountTypeAsset, 1000),
			},
			wantErr: false,
		},
		{
			name:   "income: missing R account",
			txType: model.TxTypeIncome,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Assets:Cash", model.AccountTypeAsset, -500),
			},
			wantErr: true,
		},
		{
			name:   "income: missing A/L account",
			txType: model.TxTypeIncome,
			splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, 1000),
				split("Revenue:Bonus", model.AccountTypeRevenue, -1000),
			},
			wantErr: true,
		},
		{
			name:   "transfer: two A accounts is valid",
			txType: model.TxTypeTransfer,
			splits: []model.SplitDetail{
				split("Assets:Checking", model.AccountTypeAsset, 500),
				split("Assets:Savings", model.AccountTypeAsset, -500),
			},
			wantErr: false,
		},
		{
			name:   "transfer: A + L is valid",
			txType: model.TxTypeTransfer,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Liabilities:Card", model.AccountTypeLiability, -500),
			},
			wantErr: false,
		},
		{
			name:   "transfer: contains E account is invalid",
			txType: model.TxTypeTransfer,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Expenses:Food", model.AccountTypeExpense, -500),
			},
			wantErr: true,
		},
		{
			name:   "opening: always valid",
			txType: model.TxTypeOpening,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
				split("Equity:OpeningBalances_TWD", model.AccountTypeEquity, -500),
			},
			wantErr: false,
		},
		{
			name:   "other: always valid",
			txType: model.TxTypeOther,
			splits: []model.SplitDetail{
				split("Assets:Bank", model.AccountTypeAsset, 500),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateSplitsMatchType(context.Background(), tt.txType, tt.splits)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSplitsMatchType_IgnoresCallerAccountType(t *testing.T) {
	accRepo := classifierAccRepo()
	svc := newTestTransactionService(accRepo, newMockTransactionRepo())

	t.Run("lying AccountType on expense tx is rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -1000},
			{AccountName: "Revenue:Salary", AccountType: model.AccountTypeExpense, Amount: 1000},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeExpense, splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Expense account")
	})

	t.Run("lying AccountType on income tx is rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 1000},
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeRevenue, Amount: -1000},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeIncome, splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Revenue account")
	})

	t.Run("lying AccountType on transfer tx is rejected", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 500},
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeTransfer, splits)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Asset and Liability")
	})

	t.Run("correct AccountType still passes (repo is truth)", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500},
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		err := svc.ValidateSplitsMatchType(context.Background(), model.TxTypeExpense, splits)
		require.NoError(t, err)
	})
}

func TestDetermineType_IgnoresCallerAccountType(t *testing.T) {
	accRepo := classifierAccRepo()
	svc := newTestTransactionService(accRepo, newMockTransactionRepo())

	t.Run("Revenue account claimed as Expense resolves correctly", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountName: "Revenue:Salary", AccountType: model.AccountTypeExpense, Amount: -1000},
			{AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: 1000},
		}
		got, err := svc.DetermineType(context.Background(), splits)
		require.NoError(t, err)
		assert.Equal(t, model.TxTypeIncome, got)
	})

	t.Run("resolves by AccountID when both ID and name are set", func(t *testing.T) {
		splits := []model.SplitDetail{
			{AccountID: 1, AccountName: "Expenses:Food", AccountType: model.AccountTypeRevenue, Amount: 500},
			{AccountID: 2, AccountName: "Assets:Bank", AccountType: model.AccountTypeAsset, Amount: -500},
		}
		got, err := svc.DetermineType(context.Background(), splits)
		require.NoError(t, err)
		assert.Equal(t, model.TxTypeExpense, got)
	})
}

func TestBuildTransactionListItems(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})

	txs := []*model.Transaction{
		{ID: 10, Timestamp: 1700000000, Description: "Lunch", Status: model.StatusCleared, Type: model.TxTypeExpense},
	}
	details := map[int64]*model.TransactionDetail{
		10: {
			ID: 10, Timestamp: 1700000000, Description: "Lunch",
			Status: model.StatusCleared, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 1500),
				split("Assets:Bank", model.AccountTypeAsset, -1500),
			},
		},
	}

	items := svc.BuildTransactionListItems(context.Background(), txs, details)

	require.Len(t, items, 1)
	item := items[0]
	assert.Equal(t, int64(10), item.ID)
	assert.Equal(t, time.Unix(1700000000, 0).Format(model.DateFormat), item.Date)
	assert.Equal(t, "Expense", item.Type)
	assert.Equal(t, "Expenses:Food", item.Account)
	assert.Equal(t, "Assets:Bank", item.OffsetAccount)
	assert.Equal(t, "Lunch", item.Description)
	assert.Equal(t, int64(1500), item.Amount)
	assert.Equal(t, "USD", item.Currency)
	assert.Equal(t, "Cleared", item.Status)
}

func TestBuildTransactionListItems_SkipsMissingDetail(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	txs := []*model.Transaction{
		{ID: 10, Timestamp: 1700000000, Description: "Lunch", Status: model.StatusCleared, Type: model.TxTypeExpense},
	}
	details := map[int64]*model.TransactionDetail{}

	items := svc.BuildTransactionListItems(context.Background(), txs, details)
	assert.Empty(t, items)
}

func TestBuildTransactionListItems_MultipleTransactions(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Expenses:Food", Type: model.AccountTypeExpense})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.addAccount(&model.Account{ID: 3, Name: "Revenue:Salary", Type: model.AccountTypeRevenue})

	txs := []*model.Transaction{
		{ID: 10, Timestamp: 1700000000, Description: "Lunch", Status: model.StatusCleared, Type: model.TxTypeExpense},
		{ID: 11, Timestamp: 1700086400, Description: "Paycheck", Status: model.StatusPending, Type: model.TxTypeIncome},
	}
	details := map[int64]*model.TransactionDetail{
		10: {
			ID: 10, Timestamp: 1700000000, Description: "Lunch",
			Status: model.StatusCleared, Type: model.TxTypeExpense,
			Splits: []model.SplitDetail{
				split("Expenses:Food", model.AccountTypeExpense, 1500),
				split("Assets:Bank", model.AccountTypeAsset, -1500),
			},
		},
		11: {
			ID: 11, Timestamp: 1700086400, Description: "Paycheck",
			Status: model.StatusPending, Type: model.TxTypeIncome,
			Splits: []model.SplitDetail{
				split("Revenue:Salary", model.AccountTypeRevenue, -500000),
				split("Assets:Bank", model.AccountTypeAsset, 500000),
			},
		},
	}

	items := svc.BuildTransactionListItems(context.Background(), txs, details)

	require.Len(t, items, 2)
	assert.Equal(t, "Expense", items[0].Type)
	assert.Equal(t, "Income", items[1].Type)
	assert.Equal(t, "Revenue:Salary", items[1].Account)
	assert.Equal(t, "Assets:Bank", items[1].OffsetAccount)
}
