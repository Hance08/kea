package service

import (
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/repository"
)

type Service struct {
	Account     *AccountService
	Transaction *TransactionService
	Config      *config.Config
}

func NewService(accRepo repository.AccountRepository, txRepo repository.TransactionRepository, tm repository.TransactionManager, cfg *config.Config) *Service {
	return &Service{
		Account:     NewAccountService(accRepo, cfg),
		Transaction: NewTransactionService(txRepo, accRepo, tm, cfg),
		Config:      cfg,
	}
}
