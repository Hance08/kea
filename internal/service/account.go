package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/store"
)

type AccountService struct {
	repo   store.Repository
	config Config
}

func NewAccountService(repo store.Repository, cfg Config) *AccountService {
	return &AccountService{repo: repo, config: cfg}
}

func (as *AccountService) CreateAccount(name, accType, currency, description string, parentID *int64) (*store.Account, error) {
	newID, err := as.repo.CreateAccount(name, accType, currency, description, parentID)
	if err != nil {
		return nil, err
	}

	return &store.Account{
		ID:          newID,
		Name:        name,
		Type:        accType,
		Currency:    currency,
		Description: description,
		ParentID:    parentID,
		IsHidden:    false,
	}, nil
}

func (as *AccountService) GetAllAccounts() ([]*store.Account, error) {
	return as.repo.GetAllAccounts()
}

func (as *AccountService) GetAccountByName(name string) (*store.Account, error) {
	return as.repo.GetAccountByName(name)
}

func (as *AccountService) CheckAccountExists(name string) (bool, error) {
	return as.repo.AccountExists(name)
}

func (as *AccountService) GetAccountsByType(accType string) ([]*store.Account, error) {
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

func (as *AccountService) SetBalance(account *store.Account, amountInCents int64) error {
	currency := as.config.DefaultCurrency

	if amountInCents == 0 {
		return nil
	}

	openingBalanceAccount, err := as.repo.GetAccountByName("Equity:OpeningBalances")
	if err != nil {
		return fmt.Errorf("error : can not find 'Equity:OpeningBalances' account, failed to set initial balance")
	}

	var balanceAmount int64
	var equityAmount int64

	switch account.Type {
	case "A":
		balanceAmount = amountInCents
		equityAmount = -amountInCents
	case "L":
		balanceAmount = -amountInCents
		equityAmount = amountInCents
	default:
		return fmt.Errorf("only Assets(A) and Liabilities(L) account can set balance")
	}

	tx := store.Transaction{
		Timestamp:   time.Now().Unix(),
		Description: "Opening Balance",
		Status:      1,
	}

	splits := []store.Split{
		{
			AccountID: account.ID,
			Amount:    balanceAmount,
			Currency:  currency,
			Memo:      "Opening Balance",
		},
		{
			AccountID: openingBalanceAccount.ID,
			Amount:    equityAmount,
			Currency:  currency,
			Memo:      "Opening Balance",
		},
	}

	_, err = as.repo.CreateTransactionWithSplits(tx, splits)
	return err
}
