package account

import "github.com/hance08/kea/internal/model"

// EditProvider is the service interface required by the edit command.
type EditProvider interface {
	GetAccountByName(name string) (*model.Account, error)
	GetAccountBalance(id int64) (int64, error)
	RenameAccount(oldName, newSegment string) error
	UpdateAccountMetadata(id int64, description string, isHidden bool) error
	ValidateAccountName(name string) error
	CheckAccountExists(name string) (bool, error)
}

// EditView is the display interface required by the edit command.
type EditView interface {
	ShowSuccess(msg string)
}

type editFlags struct {
	Name     string
	Desc     string
	Hidden   bool
	NoHidden bool
	JSON     bool
}

// editInput carries only the fields that should change. nil pointer = no change.
type editInput struct {
	newName     *string
	description *string
	isHidden    *bool
}

type editRunner struct {
	svc  EditProvider
	view EditView
}
