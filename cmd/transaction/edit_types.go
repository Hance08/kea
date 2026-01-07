package transaction

import (
	"errors"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)

type EditView interface {
	ShowError(msg string, err error)
	ShowWarning(msg string)
	ShowSuccess(msg string)
	ShowInfo(msg string)

	RenderDetail(detail *service.TransactionDetail) error
	RenderSplitsPreview(splits []service.SplitDetail)

	AskSelection(label string, options []string) (string, error)
	AskInput(label, defaultVal string) (string, error)
	AskConfirm(label string) bool

	AskDescription(current string) (string, error)
	AskDate(currentTimestamp int64) (int64, error)
	AskStatus(current model.TransactionStatus) (model.TransactionStatus, error)
	AskAccountFromList(accounts []*model.Account, defaultName string) (string, error)
	AskAmount(label string, currentCents int64, allowEmpty bool) (int64, error)
	AskSplitSelection(splits []service.SplitDetail) (int, error)
}

type EditProvider interface {
	GetTransactionByID(txID int64) (*service.TransactionDetail, error)
	IsEditable(detail *service.TransactionDetail) bool
	DetermineType(splits []service.SplitDetail) (service.TransactionType, error)
	GetAllowedAccounts(txType service.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account
	ValidateTransactionEdit(splits []service.SplitDetail) error
	UpdateTransactionComplete(txID int64, description string, timestamp int64, status model.TransactionStatus, splits []service.SplitDetail) error
}

type AccountProvider interface {
	GetAccountByName(name string) (*model.Account, error)
	GetAllAccounts() ([]*model.Account, error)
}

const (
	OptBasicInfo    = "Basic Info (description, date, status)"
	OptQuickAccount = "Change Account (quick edit)"
	OptQuickAmount  = "Change Amount (both sides)"
	OptEditSplits   = "Edit Splits (Advanced)"
	OptSave         = "Save & Exit"
	OptCancel       = "Cancel (discard changes)"
)

var errExitLoop = errors.New("exit loop")

type menuItem struct {
	Label     string
	Condition func(d *service.TransactionDetail) bool
	Action    func(d *service.TransactionDetail) error
}
