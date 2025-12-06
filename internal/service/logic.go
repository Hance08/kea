package service

import (
	"github.com/hance08/kea/internal/store"
)

type Config struct {
	DefaultCurrency string
}

type AccountingService struct {
	store  store.Repository
	config Config
}

func NewLogic(s store.Repository, cfg Config) *AccountingService {
	return &AccountingService{
		store:  s,
		config: cfg,
	}
}
