// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: "name", Message: "name is required"}
	assert.Equal(t, "name is required", ve.Error())
}

func TestValidationError_ErrorsAs(t *testing.T) {
	ve := &ValidationError{Field: "amount", Message: "amount must be positive"}
	wrapped := fmt.Errorf("split #1: %w", ve)

	var target *ValidationError
	assert.True(t, errors.As(wrapped, &target))
	assert.Equal(t, "amount", target.Field)
	assert.Equal(t, "amount must be positive", target.Message)
}

func TestValidationError_Unwrap(t *testing.T) {
	inner := errors.New("parse error")
	ve := &ValidationError{Field: "date", Message: "invalid date", Err: inner}

	assert.True(t, errors.Is(ve, inner))
}

func TestValidationError_NilUnwrap(t *testing.T) {
	ve := &ValidationError{Field: "name", Message: "name is required"}
	assert.Nil(t, ve.Unwrap())
}

func TestValidationErrorf(t *testing.T) {
	err := validationErrorf("splits", "must have at least %d splits (got %d)", 2, 1)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "splits", ve.Field)
	assert.Equal(t, "must have at least 2 splits (got 1)", ve.Message)
}

func TestValidationErrorf_EmptyField(t *testing.T) {
	err := validationErrorf("", "source and destination cannot be the same")

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "", ve.Field)
}

func TestValidationWrap(t *testing.T) {
	inner := &ValidationError{Field: "", Message: "can't be empty"}
	err := validationWrap("name", "invalid account name", inner)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "name", ve.Field)
	assert.Contains(t, ve.Message, "invalid account name")
	assert.Contains(t, ve.Message, "can't be empty")
	assert.True(t, errors.Is(err, inner))
}

func TestValidationError_NotMatchSentinels(t *testing.T) {
	ve := &ValidationError{Field: "name", Message: "bad name"}
	assert.False(t, errors.Is(ve, ErrNotFound))
	assert.False(t, errors.Is(ve, ErrNotEditable))
}

func TestValidateAccountName_ReturnsValidationError(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name  string
		input string
		field string
	}{
		{"empty name", "", "name"},
		{"has colon", "foo:bar", "name"},
		{"too long", string(make([]byte, 256)), "name"},
		{"leading space", " foo", "name"},
		{"reserved name", "assets", "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateAccountName(tt.input)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
			assert.Equal(t, tt.field, ve.Field)
		})
	}
}

func TestValidateCurrency_ReturnsValidationError(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name  string
		input string
		field string
	}{
		{"too short", "US", "currency"},
		{"has digits", "U2D", "currency"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateCurrency(tt.input)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
			assert.Equal(t, tt.field, ve.Field)
		})
	}
}

func TestValidateFullAccountName_ReturnsValidationError(t *testing.T) {
	svc := newTestAccountService(newMockAccountRepo(), newMockTransactionRepo())

	tests := []struct {
		name  string
		input string
		field string
	}{
		{"empty", "", "name"},
		{"bad root", "Foo:Bar", "name"},
		{"empty segment", "Assets::Checking", "name"},
		{"reserved in segment", "Assets:Equity", "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateFullAccountName(tt.input)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
			assert.Equal(t, tt.field, ve.Field)
		})
	}
}

func TestCreateAccount_ValidationErrors(t *testing.T) {
	accRepo := newMockAccountRepo()
	svc := newTestAccountService(accRepo, newMockTransactionRepo())

	tests := []struct {
		name    string
		accName string
		accType model.AccountType
		field   string
	}{
		{"invalid name", "", model.AccountTypeAsset, "name"},
		{"invalid type", "Assets:Cash", model.AccountType("X"), "type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateAccount(context.Background(), tt.accName, tt.accType, "USD", "", nil)
			assert.Error(t, err)

			var ve *ValidationError
			assert.True(t, errors.As(err, &ve), "expected ValidationError for %s, got: %T: %v", tt.name, err, err)
		})
	}
}

func TestDeleteAccount_HasChildren_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.childMap[1] = true

	svc := newTestAccountService(accRepo, newMockTransactionRepo())
	err := svc.DeleteAccountByName(context.Background(), "Assets:Bank")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestDeleteAccount_HasTransactions_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset})
	accRepo.txExistsMap[1] = true

	svc := newTestAccountService(accRepo, newMockTransactionRepo())
	err := svc.DeleteAccountByName(context.Background(), "Assets:Bank")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestRenameAccount_DuplicateName_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Old", Type: model.AccountTypeAsset})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Assets:Existing", Type: model.AccountTypeAsset})

	svc := newTestAccountService(accRepo, newMockTransactionRepo())
	err := svc.RenameAccount(context.Background(), "Assets:Old", "Existing")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestCreateAccountWithBalance_NonAL_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	svc := newTestAccountService(accRepo, newMockTransactionRepo())

	_, err := svc.CreateAccountWithBalance(context.Background(), "Revenue:Sales", model.AccountTypeRevenue, "USD", "", nil, 1000)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
}

func TestCreateTransaction_TooFewSplits_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	input := model.TransactionDetail{
		Type:   model.TxTypeExpense,
		Splits: []model.SplitDetail{{AccountName: "A", Amount: 100}},
	}
	_, err := svc.CreateTransaction(context.Background(), input)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "splits", ve.Field)
}

func TestCreateTransaction_EmptyType_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	input := model.TransactionDetail{
		Splits: []model.SplitDetail{
			{AccountName: "A", Amount: 100},
			{AccountName: "B", Amount: -100},
		},
	}
	_, err := svc.CreateTransaction(context.Background(), input)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "type", ve.Field)
}

func TestCreateSimpleTransaction_SameAccount_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.CreateSimpleTransaction(context.Background(), "Assets:Cash", "Assets:Cash", 100, "test", 0, 0, model.TxTypeTransfer)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestCreateSimpleTransaction_NegativeAmount_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.CreateSimpleTransaction(context.Background(), "A", "B", -5, "test", 0, 0, model.TxTypeTransfer)
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve))
	assert.Equal(t, "amount", ve.Field)
}

func TestUpdateTransactionStatus_InvalidStatus_ReturnsValidationError(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	err := svc.UpdateTransactionStatus(context.Background(), 5, model.TransactionStatus(99))
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "status", ve.Field)
}

func TestParseTransactionDate_InvalidFormat_ReturnsValidationError(t *testing.T) {
	txRepo := newMockTransactionRepo()
	accRepo := newMockAccountRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.ParseTransactionDate("not-a-date")
	assert.Error(t, err)

	var ve *ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T: %v", err, err)
	assert.Equal(t, "date", ve.Field)
}
