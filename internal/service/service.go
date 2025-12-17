package service

import (
	"github.com/hance08/kea/internal/store"
)

type Config struct {
	DefaultCurrency string
}

type Service struct {
	Account     *AccountService
	Transaction *TransactionService
}

func NewService(repo store.Repository, cfg Config) *Service {
	return &Service{
		Account:     NewAccountService(repo, cfg),
		Transaction: NewTransactionService(repo, cfg),
	}
}
