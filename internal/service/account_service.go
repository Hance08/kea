package service

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/store"
)

type AccountService struct {
	repo   store.AccountRepository
	config *config.Config
}

func NewAccountService(repo store.AccountRepository, cfg *config.Config) *AccountService {
	return &AccountService{repo: repo, config: cfg}
}

func (as *AccountService) GetAllAccounts() ([]*model.Account, error) {
	return as.repo.GetAllAccounts()
}

func (as *AccountService) GetAccountByName(name string) (*model.Account, error) {
	return as.repo.GetAccountByName(name)
}

func (as *AccountService) GetAccountsByType(accType string) ([]*model.Account, error) {
	return as.repo.GetAccountsByType(accType)
}

func (as *AccountService) GetAccountBalanceFormatted(accountID int64) (string, error) {
	balance, err := as.repo.GetAccountBalance(accountID)
	if err != nil {
		return "", err
	}

	balanceFloat := float64(balance) / 100
	return fmt.Sprintf("%.2f", balanceFloat), nil
}

func (as *AccountService) GetRootNameByType(accType string) (string, error) {
	switch strings.ToUpper(accType) {
	case "A":
		return "Assets", nil
	case "L":
		return "Liabilities", nil
	case "E":
		return "Expenses", nil
	case "R":
		return "Revenue", nil
	case "C":
		return "Equity", nil
	default:
		return "", fmt.Errorf("invalid account type '%s' (must be A, L, C, R, E)", accType)
	}
}

func (as *AccountService) CheckAccountExists(name string) (bool, error) {
	return as.repo.AccountExists(name)
}
