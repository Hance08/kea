package service

import (
	"github.com/hance08/kea/internal/store"
)

type AccountingService struct {
	store store.Repository
}

func NewLogic(s store.Repository) *AccountingService {
	return &AccountingService{store: s}
}
