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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAccountByID(t *testing.T) {
	t.Run("returns account when found", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, int64(1), acc.ID)
		assert.Equal(t, "Assets:Cash", acc.Name)
	})

	t.Run("wraps ErrNotFound from repo", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 999)

		assert.Nil(t, acc)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.GetAccountByID(context.Background(), 42)

		assert.Nil(t, acc)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}

func TestListAccounts(t *testing.T) {
	mkAccRepo := func() *mockAccountRepo {
		r := newMockAccountRepo()
		r.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		r.addAccount(&model.Account{ID: 2, Name: "Assets:Hidden", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		r.addAccount(&model.Account{ID: 3, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: "USD"})
		return r
	}

	t.Run("hides hidden accounts by default", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{})

		require.NoError(t, err)
		assert.Len(t, accounts, 2)
		for _, acc := range accounts {
			assert.False(t, acc.IsHidden)
		}
	})

	t.Run("shows hidden accounts when ShowHidden is true", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{ShowHidden: true})

		require.NoError(t, err)
		assert.Len(t, accounts, 3)
	})

	t.Run("filters by type", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())
		at := model.AccountTypeAsset

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{Type: &at})

		require.NoError(t, err)
		assert.Len(t, accounts, 1)
		assert.Equal(t, "Assets:Cash", accounts[0].Name)
	})

	t.Run("filters by type with ShowHidden", func(t *testing.T) {
		svc := newTestAccountService(mkAccRepo(), newMockTransactionRepo())
		at := model.AccountTypeAsset

		accounts, err := svc.ListAccounts(context.Background(), ListAccountsOptions{
			Type:       &at,
			ShowHidden: true,
		})

		require.NoError(t, err)
		assert.Len(t, accounts, 2)
	})
}

func TestGetAccountBalanceFormatted(t *testing.T) {
	t.Run("returns formatted balance for existing account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.balances[1] = 5000
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalanceFormatted(context.Background(), 1)

		require.NoError(t, err)
		assert.Equal(t, "50", bal)
	})

	t.Run("returns ErrNotFound for nonexistent account", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalanceFormatted(context.Background(), 999)

		assert.Empty(t, bal)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "999")
	})

	t.Run("passes through other errors from GetAccountByID", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		dbErr := fmt.Errorf("connection refused")
		accRepo.getByIDErr[42] = dbErr
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		bal, err := svc.GetAccountBalanceFormatted(context.Background(), 42)

		assert.Empty(t, bal)
		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrNotFound))
		assert.Equal(t, dbErr, err)
	})
}

func TestGetAccountTree(t *testing.T) {
	ptrID := func(id int64) *int64 { return &id }

	t.Run("empty repo returns non-nil empty slice", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.NoError(t, err)
		require.NotNil(t, tree)
		assert.Len(t, tree, 0)
	})

	t.Run("flat list with no parents becomes sorted roots", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Charlie", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Alpha", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Bravo", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.NoError(t, err)
		require.Len(t, tree, 3)
		assert.Equal(t, "Alpha", tree[0].Account.Name)
		assert.Equal(t, "Bravo", tree[1].Account.Name)
		assert.Equal(t, "Charlie", tree[2].Account.Name)
		for _, n := range tree {
			assert.Empty(t, n.Children)
		}
	})

	t.Run("two-level nesting groups children under parent and sorts them", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.NoError(t, err)
		require.Len(t, tree, 1)
		root := tree[0]
		assert.Equal(t, "Assets", root.Account.Name)
		require.Len(t, root.Children, 2)
		assert.Equal(t, "Assets:Bank", root.Children[0].Account.Name)
		assert.Equal(t, "Assets:Cash", root.Children[1].Account.Name)
		for _, c := range root.Children {
			assert.Empty(t, c.Children)
		}
	})

	t.Run("three-level nesting builds correctly at each level", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Assets:Bank:Checking", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(2)})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.NoError(t, err)
		require.Len(t, tree, 1)
		root := tree[0]
		assert.Equal(t, "Assets", root.Account.Name)
		require.Len(t, root.Children, 1)
		mid := root.Children[0]
		assert.Equal(t, "Assets:Bank", mid.Account.Name)
		require.Len(t, mid.Children, 1)
		leaf := mid.Children[0]
		assert.Equal(t, "Assets:Bank:Checking", leaf.Account.Name)
		assert.Empty(t, leaf.Children)
	})

	t.Run("ShowHidden false drops hidden accounts and promotes orphaned visible children to roots", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Expenses", Type: model.AccountTypeExpense, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 4, Name: "Expenses:Hidden", Type: model.AccountTypeExpense, Currency: "USD", ParentID: ptrID(3), IsHidden: true})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.NoError(t, err)
		require.Len(t, tree, 2)
		assert.Equal(t, "Assets:Cash", tree[0].Account.Name)
		assert.Empty(t, tree[0].Children)
		assert.Equal(t, "Expenses", tree[1].Account.Name)
		assert.Empty(t, tree[1].Children)
	})

	t.Run("ShowHidden true includes hidden accounts in the tree", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD", IsHidden: true})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Expenses", Type: model.AccountTypeExpense, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 4, Name: "Expenses:Hidden", Type: model.AccountTypeExpense, Currency: "USD", ParentID: ptrID(3), IsHidden: true})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{ShowHidden: true})

		require.NoError(t, err)
		require.Len(t, tree, 2)
		assert.Equal(t, "Assets", tree[0].Account.Name)
		require.Len(t, tree[0].Children, 1)
		assert.Equal(t, "Assets:Cash", tree[0].Children[0].Account.Name)
		assert.Equal(t, "Expenses", tree[1].Account.Name)
		require.Len(t, tree[1].Children, 1)
		assert.Equal(t, "Expenses:Hidden", tree[1].Children[0].Account.Name)
	})

	t.Run("propagates repo error from GetAllAccounts", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		boom := errors.New("boom")
		accRepo.getAllAccountsErr = boom
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.Error(t, err)
		assert.Same(t, boom, err)
		assert.Nil(t, tree)
	})

	t.Run("sorts siblings alphabetically regardless of insertion order", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		// Insert siblings in reverse-alphabetical order so the only thing that
		// can produce alphabetical output is sortNodes inside GetAccountTree.
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets", Type: model.AccountTypeAsset, Currency: "USD"})
		accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Zucchini", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		accRepo.addAccount(&model.Account{ID: 3, Name: "Assets:Mango", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		accRepo.addAccount(&model.Account{ID: 4, Name: "Assets:Apple", Type: model.AccountTypeAsset, Currency: "USD", ParentID: ptrID(1)})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		tree, err := svc.GetAccountTree(context.Background(), AccountTreeOptions{})

		require.NoError(t, err)
		require.Len(t, tree, 1)
		children := tree[0].Children
		require.Len(t, children, 3)
		assert.Equal(t, "Assets:Apple", children[0].Account.Name)
		assert.Equal(t, "Assets:Mango", children[1].Account.Name)
		assert.Equal(t, "Assets:Zucchini", children[2].Account.Name)
	})
}

func TestUpdateAccountMetadata_DescriptionValidation(t *testing.T) {
	t.Run("empty description is allowed", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		acc, err := svc.UpdateAccountMetadata(context.Background(), 1, "", false)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})

	t.Run("over-length description rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		longDesc := strings.Repeat("d", model.DescriptionMaxLength+1)
		acc, err := svc.UpdateAccountMetadata(context.Background(), 1, longDesc, false)
		assert.Nil(t, acc)
		require.Error(t, err)
		var ve *ValidationError
		require.True(t, errors.As(err, &ve))
		assert.Equal(t, "description", ve.Field)
		assert.Contains(t, ve.Message, "too long")
	})

	t.Run("exactly max-length description accepted", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "USD"})
		svc := newTestAccountService(accRepo, newMockTransactionRepo())

		exactDesc := strings.Repeat("d", model.DescriptionMaxLength)
		acc, err := svc.UpdateAccountMetadata(context.Background(), 1, exactDesc, false)
		require.NoError(t, err)
		assert.NotNil(t, acc)
	})
}
