package service

import (
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/config"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/repository"
)

type TransactionCreator interface {
	CreateOpeningBalance(account *model.Account, amountCents int64) error
}

type AccountService struct {
	repo   repository.AccountRepository
	config *config.Config
	txSvc  TransactionCreator
}

func NewAccountService(repo repository.AccountRepository, cfg *config.Config) *AccountService {
	return &AccountService{
		repo:   repo,
		config: cfg,
	}
}

func (as *AccountService) SetTransactionService(txSvc TransactionCreator) {
	as.txSvc = txSvc
}

func (as *AccountService) GetAllAccounts() ([]*model.Account, error) {
	return as.repo.GetAllAccounts()
}

func (as *AccountService) GetAccountByName(name string) (*model.Account, error) {
	return as.repo.GetAccountByName(name)
}

func (as *AccountService) GetAccountsByType(accType model.AccountType) ([]*model.Account, error) {
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

func (as *AccountService) ValidateSelectableAccount(name string, allowedTypes []string) error {
	acc, err := as.repo.GetAccountByName(name)
	if err != nil {
		return err
	}

	if len(allowedTypes) > 0 {
		allowed := false
		for _, t := range allowedTypes {
			if string(acc.Type) == t {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("account %q has type %q, not allowed for this transaction type (allowed: %v)", name, acc.Type, allowedTypes)
		}
	}

	if acc.IsHidden {
		return fmt.Errorf("account %q is hidden", name)
	}

	hasChildren, err := as.repo.HasChildAccounts(acc.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return fmt.Errorf("account %q is a parent account; select a leaf account instead", name)
	}

	return nil
}
