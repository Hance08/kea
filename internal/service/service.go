package service

import (
	"github.com/hance08/kea/internal/config"
)

type Service struct {
	Account     *AccountService
	Transaction *TransactionService
	Config      *config.Config
}

func NewService(repo Repository, cfg *config.Config) *Service {
	return &Service{
		Account:     NewAccountService(repo, cfg),
		Transaction: NewTransactionService(repo, cfg),
		Config:      cfg,
	}
}
