// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import "errors"

var (
	ErrAccountExists       = errors.New("account already exists")
	ErrRecordNotFound      = errors.New("record not found")
	ErrConstraintViolation = errors.New("database constraint violation")
)
