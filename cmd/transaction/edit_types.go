// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package transaction

import (
	"context"
	"errors"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
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
	GetTransactionByID(ctx context.Context, txID int64) (*model.TransactionDetail, error)
	IsEditable(detail *model.TransactionDetail) (bool, service.NotEditableReason)
	GetAllowedAccounts(txType model.TransactionType, currentAccountType model.AccountType, allAccounts []*model.Account) []*model.Account
	ValidateTransactionEdit(ctx context.Context, splits []model.SplitDetail) error
	ValidateSplitsMatchType(ctx context.Context, txType model.TransactionType, splits []model.SplitDetail) error
	UpdateTransactionComplete(ctx context.Context, txID int64, description string, timestamp int64, status model.TransactionStatus, txType model.TransactionType, splits []model.SplitDetail) error
}

type AccountProvider interface {
	GetAccountByName(ctx context.Context, name string) (*model.Account, error)
	GetAllAccounts(ctx context.Context) ([]*model.Account, error)
}

const (
	OptBasicInfo    = "Basic Info (description, date, status)"
	OptChangeType   = "Change Type"
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
