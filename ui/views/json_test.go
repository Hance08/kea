// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCentsToUnit(t *testing.T) {
	assert.Equal(t, 1.0, CentsToUnit(100))
	assert.Equal(t, 1.5, CentsToUnit(150))
	assert.Equal(t, 0.0, CentsToUnit(0))
	assert.Equal(t, -1.5, CentsToUnit(-150))
}

func TestToJSONAccount(t *testing.T) {
	parentID := int64(1)
	acc := &model.Account{
		ID:          2,
		Name:        "Assets:Bank",
		Type:        model.AccountTypeAsset,
		ParentID:    &parentID,
		Currency:    "TWD",
		Description: "Main bank",
		IsHidden:    false,
	}
	got := ToJSONAccount(acc, 15000)
	assert.Equal(t, int64(2), got.ID)
	assert.Equal(t, "Assets:Bank", got.Name)
	assert.Equal(t, "A", got.Type)
	assert.Equal(t, &parentID, got.ParentID)
	assert.Equal(t, "TWD", got.Currency)
	assert.Equal(t, "Main bank", got.Description)
	assert.False(t, got.IsHidden)
	assert.Equal(t, 150.0, got.Balance)
}

func TestToJSONTxDetail(t *testing.T) {
	detail := &model.TransactionDetail{
		ID:          42,
		Timestamp:   1711238400, // 2024-03-24
		Description: "Buy coffee",
		Status:      model.StatusCleared,
		Type:        model.TxTypeExpense,
		Splits: []model.SplitDetail{
			{ID: 1, AccountID: 10, AccountName: "Assets:Cash", AccountType: model.AccountTypeAsset, Amount: -500, Currency: "TWD", Memo: ""},
			{ID: 2, AccountID: 20, AccountName: "Expenses:Food", AccountType: model.AccountTypeExpense, Amount: 500, Currency: "TWD", Memo: "lunch"},
		},
	}
	got := ToJSONTxDetail(detail)
	assert.Equal(t, int64(42), got.ID)
	assert.Equal(t, "Cleared", got.Status)
	assert.Equal(t, "Expense", got.Type)
	assert.Len(t, got.Splits, 2)
	assert.Equal(t, -5.0, got.Splits[0].Amount)
	assert.Equal(t, 5.0, got.Splits[1].Amount)
	assert.Equal(t, "lunch", got.Splits[1].Memo)
}

func TestToJSONTxDetail_LocalDate(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	origLocal := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = origLocal })

	// 2026-04-01 00:00:00 UTC+8 = 2026-03-31T16:00:00Z
	ts := time.Date(2026, time.April, 1, 0, 0, 0, 0, loc).Unix()
	detail := &model.TransactionDetail{
		ID:        1,
		Timestamp: ts,
		Status:    model.StatusCleared,
	}
	got := ToJSONTxDetail(detail)
	assert.Equal(t, "2026-04-01", got.Date)
}

func TestToJSONTxListItem(t *testing.T) {
	item := TransactionListItem{
		ID: 7, Date: "2024-03-24", Type: "Expense",
		Account: "Assets:Cash", Offset: "Expenses:Food",
		Description: "lunch", Amount: "5.50", Currency: "TWD", Status: "Cleared",
	}
	got := ToJSONTxListItem(item)
	assert.Equal(t, int64(7), got.ID)
	assert.Equal(t, 5.5, got.Amount)
	assert.Equal(t, "TWD", got.Currency)
	assert.Equal(t, "Cleared", got.Status)
}

func TestToJSONTxListItem_commaAmount(t *testing.T) {
	item := TransactionListItem{
		ID: 10, Date: "2024-03-24", Type: "Income",
		Account: "Assets:Bank", Offset: "Revenue:Salary",
		Description: "salary", Amount: "1,234.56", Currency: "TWD", Status: "Cleared",
	}
	got := ToJSONTxListItem(item)
	assert.Equal(t, 1234.56, got.Amount)
}

func TestWriteJSON_validOutput(t *testing.T) {
	// Redirect stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := WriteJSON(map[string]string{"key": "value"})
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var result map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "value", result["key"])
}
