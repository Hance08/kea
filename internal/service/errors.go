// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"errors"
	"fmt"
)

var (
	ErrReconciled     = errors.New("transaction has been reconciled")
	ErrNotEditable    = errors.New("operation denied on protected record")
	ErrNotFound       = errors.New("record not found")
	ErrAlreadyExists  = errors.New("record already exists")
	ErrCircularParent = errors.New("circular parent reference detected")
)

// ValidationError represents a user-input validation failure.
type ValidationError struct {
	Field   string // which field failed (empty for cross-field validations)
	Message string
	Err     error // optional wrapped error
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Unwrap() error { return e.Err }

func validationErrorf(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func validationWrap(field, prefix string, err error) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: prefix + ": " + err.Error(),
		Err:     err,
	}
}
