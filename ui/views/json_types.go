// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"strconv"
	"time"

	"github.com/hance08/kea/internal/model"
)

// ── JSON DTOs ─────────────────────────────────────────────────────────────────
// Amounts are float64 currency units (not cents). Dates are YYYY-MM-DD strings.
// TransactionStatus is serialized via .String(). AccountType as single-letter code.

type JSONAccount struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	ParentID    *int64  `json:"parent_id"`
	Currency    string  `json:"currency"`
	Description string  `json:"description"`
	IsHidden    bool    `json:"is_hidden"`
	Balance     float64 `json:"balance"`
}

type JSONSplitDetail struct {
	ID          int64   `json:"id"`
	AccountID   int64   `json:"account_id"`
	AccountName string  `json:"account_name"`
	AccountType string  `json:"account_type"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Memo        string  `json:"memo"`
}

type JSONTxDetail struct {
	ID          int64             `json:"id"`
	Date        string            `json:"date"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Splits      []JSONSplitDetail `json:"splits"`
}

type JSONTxListItem struct {
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	Type        string  `json:"type"`
	Account     string  `json:"account"`
	Offset      string  `json:"offset"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
}

type JSONSystemInfo struct {
	ConfigPath      string `json:"config_path"`
	ActiveLedger    string `json:"active_ledger"`
	DBPath          string `json:"db_path"`
	// false means DB does not yet exist; it will be created on first use.
	DBExists        bool   `json:"db_exists"`
	DefaultCurrency string `json:"default_currency"`
	AppDataDir      string `json:"app_data_dir"`
}

// ── Converters ────────────────────────────────────────────────────────────────

func ToJSONAccount(acc *model.Account, balanceCents int64) JSONAccount {
	return JSONAccount{
		ID:          acc.ID,
		Name:        acc.Name,
		Type:        string(acc.Type),
		ParentID:    acc.ParentID,
		Currency:    acc.Currency,
		Description: acc.Description,
		IsHidden:    acc.IsHidden,
		Balance:     CentsToUnit(balanceCents),
	}
}

func toJSONSplit(s model.SplitDetail) JSONSplitDetail {
	return JSONSplitDetail{
		ID:          s.ID,
		AccountID:   s.AccountID,
		AccountName: s.AccountName,
		AccountType: string(s.AccountType),
		Amount:      CentsToUnit(s.Amount),
		Currency:    s.Currency,
		Memo:        s.Memo,
	}
}

func ToJSONTxDetail(d *model.TransactionDetail) JSONTxDetail {
	splits := make([]JSONSplitDetail, len(d.Splits))
	for i, s := range d.Splits {
		splits[i] = toJSONSplit(s)
	}
	return JSONTxDetail{
		ID:          d.ID,
		Date:        time.Unix(d.Timestamp, 0).UTC().Format("2006-01-02"),
		Description: d.Description,
		Status:      d.Status.String(),
		Splits:      splits,
	}
}

func ToJSONTxListItem(item TransactionListItem) JSONTxListItem {
	amount, _ := strconv.ParseFloat(item.Amount, 64)
	return JSONTxListItem{
		ID:          item.ID,
		Date:        item.Date,
		Type:        item.Type,
		Account:     item.Account,
		Offset:      item.Offset,
		Description: item.Description,
		Amount:      amount,
		Currency:    item.Currency,
		Status:      item.Status,
	}
}

func ToJSONSystemInfo(info SystemInfo) JSONSystemInfo {
	return JSONSystemInfo{
		ConfigPath:      info.ConfigPath,
		ActiveLedger:    info.ActiveLedger,
		DBPath:          info.DBPath,
		DBExists:        info.DBExists,
		DefaultCurrency: info.DefaultCurrency,
		AppDataDir:      info.AppDataDir,
	}
}
