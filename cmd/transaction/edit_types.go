package transaction

import (
	"errors"

	"github.com/hance08/kea/internal/model"
)

type EditView interface {
	ShowError(msg string, err error)
	ShowWarning(msg string)
	ShowSuccess(msg string)
	ShowInfo(msg string)

	RenderDetail(detail *model.TransactionDetail) error
	RenderSplitsPreview(splits []model.SplitDetail)

	AskSelection(label string, options []string) (string, error)
	AskInput(label, defaultVal string) (string, error)
	AskConfirm(label string) bool
	AskDescription(current string) (string, error)
	AskDate(currentTimestamp int64) (int64, error)
	AskStatus(current model.TransactionStatus) (model.TransactionStatus, error)
	AskAccountFromList(accounts []*model.Account, defaultName string) (string, error)
	AskAmount(label string, currentCents int64, allowEmpty bool) (int64, error)
	AskSplitSelection(splits []model.SplitDetail) (int, error)
}

type EditProvider interface {
	GetTransactionByID(txID int64) (*model.TransactionDetail, error)
	IsEditable(detail *model.TransactionDetail) bool
	DetermineType(splits []model.SplitDetail) (model.TransactionType, error)
	GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account
	ValidateTransactionEdit(splits []model.SplitDetail) error
	UpdateTransactionComplete(txID int64, description string, timestamp int64, status model.TransactionStatus, splits []model.SplitDetail) error
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
	Condition func(d *model.TransactionDetail) bool
	Action    func(d *model.TransactionDetail) error
}
