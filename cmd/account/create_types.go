package account

import (
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/ui/views"
)

type CreateView interface {
	RenderSummary(data views.AccountSummaryItem) error
	ShowSuccess(msg string)
}

type CreateProvider interface {
	GetAllAccounts() ([]*model.Account, error)
	GetAccountByName(name string) (*model.Account, error)
	GetRootNameByType(accType string) (string, error)
	CheckAccountExists(name string) (bool, error)
	FormatAccountName(prefix string, name string) string
	CreateAccountWithBalance(name string, accType model.AccountType, currency, description string, parentID *int64, balance int64) (*model.Account, error)
}

type createFlags struct {
	Name        string
	Type        string
	Parent      string
	BalanceStr  string
	Currency    string
	Description string
}
