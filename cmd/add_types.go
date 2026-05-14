// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"context"

	"github.com/hance08/kea/internal/model"
)

type AddView interface {
	Render(input *model.TransactionDetail, isCreate bool) error
}

type AddProvider interface {
	GetAllAccounts(ctx context.Context) ([]*model.Account, error)
	GetAccountBalanceFormatted(ctx context.Context, accountID int64) (string, error)
	GetAccountByName(ctx context.Context, name string) (*model.Account, error)
	ValidateSelectableAccount(ctx context.Context, name string, allowedTypes []string) error
}

type TransactionProvider interface {
	GetTransactionRule(mode model.TransactionType) (model.TransactionRule, error)
	CreateSimpleTransaction(ctx context.Context, input model.CreateSimpleTransactionInput) (model.TransactionDetail, error)
	CreateTransactionFromSplits(ctx context.Context, input model.CreateTransactionFromSplitsInput) (model.TransactionDetail, error)
	ParseTransactionDate(dateStr string) (int64, error)
}

type addFlags struct {
	Description string
	Amount      string
	From        string
	To          string
	Status      string
	Timestamp   string
	Type        string
	JSON        bool
	Splits      []string
}

type addTransactionInput struct {
	FromAccountID string
	ToAccountID   string
	AmountCents   int64
	Description   string
	Timestamp     int64
	Status        model.TransactionStatus
	Type          model.TransactionType
	Splits        []model.SplitDetail
}
