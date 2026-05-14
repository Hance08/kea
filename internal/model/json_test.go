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
