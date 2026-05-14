// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package account

import (
	"context"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/ui/views"
)

type CreateView interface {
	RenderSummary(data views.AccountSummaryItem) error
	ShowSuccess(msg string)
}

type CreateProvider interface {
	GetAllAccounts(ctx context.Context) ([]*model.Account, error)
	GetAccountByName(ctx context.Context, name string) (*model.Account, error)
	GetRootNameByType(accType string) (string, error)
	CheckAccountExists(ctx context.Context, name string) (bool, error)
	FormatAccountName(prefix string, name string) string
	CreateAccountWithBalance(ctx context.Context, input model.CreateAccountInput) (*model.Account, error)
	ValidateAccountName(name string) error
	ValidateFullAccountName(name string) error
	ValidateCurrency(currency string) error
	GetAccountBalance(ctx context.Context, id int64) (int64, error)
}

type createInput struct {
	fullName     string
	accountType  model.AccountType
	currency     string
	description  string
	parentID     *int64
	balanceCents int64
}

type createFlags struct {
	Name        string
	Type        string
	Parent      string
	BalanceStr  string
	Currency    string
	Description string
	JSON        bool
}
