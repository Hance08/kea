// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewStore(dbPath, migrations.FS)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// boolPtr is a shared test helper for constructing *bool literals, used when
// setting Transaction.Regular on Income/Expense fixtures (required by the
// "regular" column's CHECK constraint).
func boolPtr(b bool) *bool { return &b }

func accountNames(accounts []*model.Account) []string {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = a.Name
	}
	return names
}

// TestRenameAccount_LIKEWildcardsInName verifies that accounts whose names share
// a prefix but are not true descendants are not incorrectly renamed.
// Assets:AXB and Assets:AXB:Sub must be unaffected when Assets:A_B is renamed.
func TestRenameAccount_LIKEWildcardsInName(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for _, name := range []string{"Assets:A_B", "Assets:A_B:Sub", "Assets:AXB", "Assets:AXB:Sub"} {
		_, err := s.CreateAccount(ctx, name, model.AccountTypeAsset, "USD", "", nil)
		require.NoError(t, err, "creating account %q", name)
	}

	err := s.RenameAccount(ctx, "Assets:A_B", "Assets:C")
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:C", "renamed account should exist")
	assert.Contains(t, names, "Assets:C:Sub", "child of renamed account should be renamed")
	assert.Contains(t, names, "Assets:AXB", "sibling with similar prefix should be unchanged")
	assert.Contains(t, names, "Assets:AXB:Sub", "child of sibling with similar prefix should be unchanged")
	assert.NotContains(t, names, "Assets:A_B", "old account name should not exist")
	assert.NotContains(t, names, "Assets:A_B:Sub", "old child name should not exist")
}

// TestRenameAccount_DeepNesting verifies that a rename cascades correctly through
// multiple levels of nesting.
func TestRenameAccount_DeepNesting(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for _, name := range []string{"Assets:Bank", "Assets:Bank:Checking", "Assets:Bank:Checking:Joint"} {
		_, err := s.CreateAccount(ctx, name, model.AccountTypeAsset, "USD", "", nil)
		require.NoError(t, err, "creating account %q", name)
	}

	err := s.RenameAccount(ctx, "Assets:Bank", "Assets:CU")
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:CU", "renamed root account should exist")
	assert.Contains(t, names, "Assets:CU:Checking", "first-level descendant should be renamed")
	assert.Contains(t, names, "Assets:CU:Checking:Joint", "second-level descendant should be renamed")
	assert.NotContains(t, names, "Assets:Bank", "old root account name should not exist")
	assert.NotContains(t, names, "Assets:Bank:Checking", "old first-level descendant should not exist")
	assert.NotContains(t, names, "Assets:Bank:Checking:Joint", "old second-level descendant should not exist")
}

// TestRenameAccount_SiblingUnaffected verifies that accounts whose name starts
// with the old name but lack the ":" separator are not touched by the cascade.
func TestRenameAccount_SiblingUnaffected(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	for _, name := range []string{"Assets:Bank", "Assets:Bank:Checking", "Assets:Bankrupt"} {
		_, err := s.CreateAccount(ctx, name, model.AccountTypeAsset, "USD", "", nil)
		require.NoError(t, err, "creating account %q", name)
	}

	err := s.RenameAccount(ctx, "Assets:Bank", "Assets:CreditUnion")
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:CreditUnion", "renamed account should exist")
	assert.Contains(t, names, "Assets:CreditUnion:Checking", "child of renamed account should be renamed")
	assert.Contains(t, names, "Assets:Bankrupt", "account with name starting with old prefix should be unchanged")
	assert.NotContains(t, names, "Assets:Bank", "old account name should not exist")
	assert.NotContains(t, names, "Assets:Bank:Checking", "old child name should not exist")
}

func TestRenameAccount_InsideExecTx(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Assets:Bank:Checking", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	err = s.ExecTx(ctx, func(repo repository.Repository) error {
		return repo.RenameAccount(ctx, "Assets:Bank", "Assets:CU")
	})
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	names := accountNames(accounts)

	assert.Contains(t, names, "Assets:CU")
	assert.Contains(t, names, "Assets:CU:Checking")
	assert.NotContains(t, names, "Assets:Bank")
	assert.NotContains(t, names, "Assets:Bank:Checking")
}

func TestCreateAccount_And_GetByName(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "checking", nil)
	require.NoError(t, err)
	assert.Positive(t, id)

	acc, err := s.GetAccountByName(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.Equal(t, id, acc.ID)
	assert.Equal(t, "Assets:Bank", acc.Name)
	assert.Equal(t, model.AccountTypeAsset, acc.Type)
	assert.Equal(t, "USD", acc.Currency)
	assert.Equal(t, "checking", acc.Description)
	assert.Nil(t, acc.ParentID)
	assert.False(t, acc.IsHidden)
}

func TestCreateAccount_DuplicateNameReturnsError(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrAlreadyExists)
}

func TestGetAccountByID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "groceries", nil)
	require.NoError(t, err)

	acc, err := s.GetAccountByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Expenses:Food", acc.Name)
	assert.Equal(t, model.AccountTypeExpense, acc.Type)
}

func TestGetAccountByID_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.GetAccountByID(ctx, 99999)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestGetAccountByName_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.GetAccountByName(ctx, "NoSuchAccount")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestAccountExists(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	exists, err = s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestGetAllAccounts(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	accounts, err := s.GetAllAccounts(ctx)
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
	assert.Equal(t, "Assets:Bank", accounts[0].Name)
	assert.Equal(t, "Expenses:Food", accounts[1].Name)
}

func TestGetAccountsByType(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Assets:Cash", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	_, err = s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	assets, err := s.GetAccountsByType(ctx, model.AccountTypeAsset)
	require.NoError(t, err)
	assert.Len(t, assets, 2)

	expenses, err := s.GetAccountsByType(ctx, model.AccountTypeExpense)
	require.NoError(t, err)
	assert.Len(t, expenses, 1)
}

func TestCreateAccount_WithParentID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	parentID, err := s.CreateAccount(ctx, "Assets", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	childID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", &parentID)
	require.NoError(t, err)

	child, err := s.GetAccountByID(ctx, childID)
	require.NoError(t, err)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parentID, *child.ParentID)
}

func TestHasChildAccounts(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	parentID, err := s.CreateAccount(ctx, "Assets", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	has, err := s.HasChildAccounts(ctx, parentID)
	require.NoError(t, err)
	assert.False(t, has)

	_, err = s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", &parentID)
	require.NoError(t, err)

	has, err = s.HasChildAccounts(ctx, parentID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestDeleteAccount(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	err = s.DeleteAccount(ctx, id)
	require.NoError(t, err)

	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteAccount_BlockedByTransactions(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "tx", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	// FK RESTRICT should prevent deletion while splits reference this account
	err = s.DeleteAccount(ctx, assetID)
	require.Error(t, err)

	// Account should still exist
	exists, err := s.AccountExists(ctx, "Assets:Bank")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestDeleteAccount_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.DeleteAccount(ctx, 99999)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestUpdateAccountMetadata(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	id, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)

	err = s.UpdateAccountMetadata(ctx, id, "updated desc", true)
	require.NoError(t, err)

	acc, err := s.GetAccountByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "updated desc", acc.Description)
	assert.True(t, acc.IsHidden)
}

func TestUpdateAccountMetadata_NotFound(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	err := s.UpdateAccountMetadata(ctx, 99999, "desc", false)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestAccountHasTransactions(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	has, err := s.AccountHasTransactions(ctx, assetID)
	require.NoError(t, err)
	assert.False(t, has)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "test", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	has, err = s.AccountHasTransactions(ctx, assetID)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestGetAccountBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	bal, err := s.GetAccountBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "groceries", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -2000, Currency: "USD"},
		{AccountID: expenseID, Amount: 2000, Currency: "USD"},
	})
	require.NoError(t, err)

	bal, err = s.GetAccountBalance(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, int64(-2000), bal)

	bal, err = s.GetAccountBalance(ctx, expenseID)
	require.NoError(t, err)
	assert.Equal(t, int64(2000), bal)
}

func TestCreateAccount_RejectsInvalidAccountType(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.CreateAccount(ctx, "Assets:Test", model.AccountType("X"), "USD", "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrInvalidAccountType)
}

func TestGetAllAccountBalances(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	assetID, err := s.CreateAccount(ctx, "Assets:Bank", model.AccountTypeAsset, "USD", "", nil)
	require.NoError(t, err)
	expenseID, err := s.CreateAccount(ctx, "Expenses:Food", model.AccountTypeExpense, "USD", "", nil)
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 1000, Description: "early", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -500, Currency: "USD"},
		{AccountID: expenseID, Amount: 500, Currency: "USD"},
	})
	require.NoError(t, err)

	_, err = s.CreateTransactionWithSplits(ctx, model.Transaction{
		Timestamp: 2000, Description: "later", Status: model.StatusPending, Type: model.TxTypeExpense,
		Regular: boolPtr(true),
	}, []model.Split{
		{AccountID: assetID, Amount: -300, Currency: "USD"},
		{AccountID: expenseID, Amount: 300, Currency: "USD"},
	})
	require.NoError(t, err)

	balances, err := s.GetAllAccountBalances(ctx, 1500)
	require.NoError(t, err)
	assert.Equal(t, int64(-500), balances[assetID])
	assert.Equal(t, int64(500), balances[expenseID])

	balances, err = s.GetAllAccountBalances(ctx, 2500)
	require.NoError(t, err)
	assert.Equal(t, int64(-800), balances[assetID])
	assert.Equal(t, int64(800), balances[expenseID])
}

func TestAccountTypeCheckConstraint_RejectsInvalidTypeAtDBLevel(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO accounts (name, type, currency, description) VALUES (?, ?, ?, ?)`,
		"Test:Invalid", "X", "USD", "")
	require.Error(t, err, "DB should reject invalid account type")
}
