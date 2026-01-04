package service

import (
	"github.com/hance08/kea/internal/config"
)

type Service struct {
	Account     *AccountService
	Transaction *TransactionService
	Config      *config.Config
}

func NewService(accRepo AccountRepository, txRepo TransactionRepository, tm TransactionManager, cfg *config.Config) *Service {
	return &Service{
		Account:     NewAccountService(accRepo, cfg),
		Transaction: NewTransactionService(txRepo, accRepo, tm, cfg),
		Config:      cfg,
	}
}
