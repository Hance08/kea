// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package store

import (
	"fmt"

	"github.com/hance08/kea/internal/repository"
)

var (
	ErrAccountExists       = fmt.Errorf("account already exists: %w", repository.ErrAlreadyExists)
	ErrRecordNotFound      = fmt.Errorf("record not found: %w", repository.ErrNotFound)
	ErrConstraintViolation = fmt.Errorf("database constraint violation")
)
