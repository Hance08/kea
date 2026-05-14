// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"errors"
	"fmt"
	"testing"

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
