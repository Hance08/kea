// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import "errors"

var (
	ErrReconciled     = errors.New("transaction has been reconciled")
	ErrNotEditable    = errors.New("operation denied on protected record")
	ErrNotFound       = errors.New("record not found")
	ErrAlreadyExists  = errors.New("record already exists")
	ErrCircularParent = errors.New("circular parent reference detected")
)
