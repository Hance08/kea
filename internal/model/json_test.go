// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model_test

import (
	"encoding/json"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccount_JSONKeys(t *testing.T) {
	parentID := int64(5)
	acc := model.Account{
		ID:          1,
		Name:        "Assets:Bank",
		Type:        model.AccountTypeAsset,
		ParentID:    &parentID,
		Currency:    "USD",
		Description: "Main bank",
		IsHidden:    false,
	}

	data, err := json.Marshal(acc)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "name")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "parent_id")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "is_hidden")

	assert.NotContains(t, m, "ID")
	assert.NotContains(t, m, "ParentID")
	assert.NotContains(t, m, "IsHidden")
}

func TestAccount_JSON_OmitsNullParentID(t *testing.T) {
	acc := model.Account{
		ID:       1,
		Name:     "Assets:Cash",
		Type:     model.AccountTypeAsset,
		Currency: "USD",
	}

	data, err := json.Marshal(acc)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, exists := m["parent_id"]
	assert.False(t, exists, "parent_id should be omitted when nil")
}

func TestTransaction_JSONKeys(t *testing.T) {
	extID := "ext-123"
	tx := model.Transaction{
		ID:          10,
		Timestamp:   1700000000,
		Description: "Groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		ExternalID:  &extID,
	}

	data, err := json.Marshal(tx)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "external_id")

	assert.NotContains(t, m, "ID")
	assert.NotContains(t, m, "ExternalID")
}

func TestTransaction_JSON_OmitsNullExternalID(t *testing.T) {
	tx := model.Transaction{
		ID:          10,
		Timestamp:   1700000000,
		Description: "Groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
	}

	data, err := json.Marshal(tx)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	_, exists := m["external_id"]
	assert.False(t, exists, "external_id should be omitted when nil")
}

func TestTransactionDetail_JSONKeys(t *testing.T) {
	td := model.TransactionDetail{
		ID:          10,
		Timestamp:   1700000000,
		Description: "Groceries",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		Splits: []model.SplitDetail{
			{ID: 1, AccountID: 2, AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 1000, Currency: "USD"},
		},
	}

	data, err := json.Marshal(td)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "timestamp")
	assert.Contains(t, m, "description")
	assert.Contains(t, m, "status")
	assert.Contains(t, m, "type")
	assert.Contains(t, m, "splits")

	assert.NotContains(t, m, "Splits")
}

func TestSplit_JSONKeys(t *testing.T) {
	s := model.Split{
		ID:            1,
		TransactionID: 10,
		AccountID:     2,
		Amount:        1000,
		Currency:      "USD",
		Memo:          "test",
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "transaction_id")
	assert.Contains(t, m, "account_id")
	assert.Contains(t, m, "amount")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "memo")

	assert.NotContains(t, m, "TransactionID")
	assert.NotContains(t, m, "AccountID")
}

func TestSplitDetail_JSONKeys(t *testing.T) {
	sd := model.SplitDetail{
		ID:          1,
		AccountID:   2,
		AccountName: "Assets:Bank",
		AccountType: model.AccountTypeAsset,
		Amount:      1000,
		Currency:    "USD",
		Memo:        "test",
	}

	data, err := json.Marshal(sd)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Contains(t, m, "id")
	assert.Contains(t, m, "account_id")
	assert.Contains(t, m, "account_name")
	assert.Contains(t, m, "account_type")
	assert.Contains(t, m, "amount")
	assert.Contains(t, m, "currency")
	assert.Contains(t, m, "memo")

	assert.NotContains(t, m, "AccountID")
	assert.NotContains(t, m, "AccountName")
	assert.NotContains(t, m, "AccountType")
}
