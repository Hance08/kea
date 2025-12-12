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

func (al *AccountService) CreateAccount(name, accType, currency, description string, parentID *int64) (*store.Account, error) {
	newID, err := al.repo.CreateAccount(name, accType, currency, description, parentID)
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

func (al *AccountService) GetAllAccounts() ([]*store.Account, error) {
	return al.repo.GetAllAccounts()
}

func (al *AccountService) GetAccountByName(name string) (*store.Account, error) {
	return al.repo.GetAccountByName(name)
}

func (al *AccountService) CheckAccountExists(name string) (bool, error) {
	return al.repo.AccountExists(name)
}

func (al *AccountService) GetAccountsByType(accType string) ([]*store.Account, error) {
	return al.repo.GetAccountsByType(accType)
}

func (al *AccountService) GetAccountBalanceFormatted(accountID int64) (string, error) {
	balance, err := al.repo.GetAccountBalance(accountID)
	if err != nil {
		return "", err
	}

	balanceFloat := float64(balance) / 100
	return fmt.Sprintf("%.2f", balanceFloat), nil
}

func (al *AccountService) GetRootNameByType(accType string) (string, error) {
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

func (al *AccountService) SetBalance(account *store.Account, amountInCents int64) error {
	currency := al.config.DefaultCurrency

	if amountInCents == 0 {
		return nil
	}

	openingBalanceAccount, err := al.repo.GetAccountByName("Equity:OpeningBalances")
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

	_, err = al.repo.CreateTransactionWithSplits(tx, splits)
	return err
}
