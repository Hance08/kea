// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package service

import (
	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/repository"
)

type Service struct {
	account     *AccountService
	transaction *TransactionService
	config      *config.Config
}

func NewService(accRepo repository.AccountRepository, txRepo repository.TransactionRepository, tm repository.TransactionManager, cfg *config.Config) *Service {
	svc := &Service{
		account:     NewAccountService(accRepo, cfg, tm),
		transaction: NewTransactionService(txRepo, accRepo, tm, cfg),
		config:      cfg,
	}

	return svc
}

func (s *Service) Account() *AccountService     { return s.account }
func (s *Service) Transaction() *TransactionService { return s.transaction }
func (s *Service) Config() *config.Config        { return s.config }
