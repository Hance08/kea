package service

import "errors"

var (
	// ErrReconciled is returned when an operation is attempted on a reconciled
	// transaction, which is considered immutable.
	ErrReconciled = errors.New("transaction has been reconciled")

	// ErrNotEditable is returned when an operation targets a protected record
	// (e.g. the initial opening balance transaction or a system account).
	ErrNotEditable = errors.New("operation denied on protected record")
)
