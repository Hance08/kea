package store

import "errors"

var (
	ErrAccountExists       = errors.New("account already exists")
	ErrRecordNotFound      = errors.New("record not found")
	ErrConstraintViolation = errors.New("database constraint violation")
)
