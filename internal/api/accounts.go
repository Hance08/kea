// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)

// wrapAsListResult wraps a flat slice into a ListResult with limit=0/offset=0
// and total_count=len. Used by /api/accounts when the request has no filter or
// pagination intent so the response shape is consistent.
func wrapAsListResult(accounts []*model.Account) *model.ListResult[*model.Account] {
	return &model.ListResult[*model.Account]{
		Items:      accounts,
		TotalCount: len(accounts),
		Limit:      0,
		Offset:     0,
	}
}

func (s *Server) handleAccountByName(w http.ResponseWriter, r *http.Request) error {
	name := r.URL.Query().Get("name")
	if name == "" {
		return &service.ValidationError{Field: "name", Message: "name query parameter is required"}
	}
	acc, err := s.svc.Account().GetAccountByName(r.Context(), name)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, acc)
}

func (s *Server) handleAccountByID(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.svc.Account().GetAccountByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, acc)
}

type balanceResponse struct {
	AccountID int64  `json:"account_id"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

func (s *Server) handleAccountBalance(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	acc, err := s.svc.Account().GetAccountByID(r.Context(), id)
	if err != nil {
		return err
	}
	amount, err := s.svc.Account().GetAccountBalance(r.Context(), id)
	if err != nil {
		return err
	}
	currency := acc.Currency
	if currency == "" {
		currency = s.svc.Config().Defaults.Currency
	}
	return writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: id,
		Amount:    amount,
		Currency:  currency,
	})
}

// endOfTodayUTC returns 23:59:59 UTC on the same calendar day as now.
// The web form encodes a user-picked date as noon UTC of that day, so a
// transaction "dated today" lands ahead of the wall-clock instant for users
// east of UTC. Defaulting the balance cutoff to end-of-day prevents same-day
// transactions from being silently excluded.
func endOfTodayUTC(now time.Time) int64 {
	u := now.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 23, 59, 59, 0, time.UTC).Unix()
}

func (s *Server) handleListBalances(w http.ResponseWriter, r *http.Request) error {
	asOfPtr, err := parseInt64Query(r, "as_of")
	if err != nil {
		return err
	}
	asOf := endOfTodayUTC(time.Now())
	if asOfPtr != nil {
		asOf = *asOfPtr
	}
	includeHidden, err := parseBoolQuery(r, "include_hidden")
	if err != nil {
		return err
	}
	rows, err := s.svc.Account().GetAccountBalancesBulk(r.Context(), asOf, includeHidden)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, &model.ListResult[model.AccountBalance]{
		Items:      rows,
		TotalCount: len(rows),
		Limit:      0,
		Offset:     0,
	})
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	opts, err := parseListOptions(r)
	if err != nil {
		return err
	}
	filter, err := parseAccountFilter(r)
	if err != nil {
		return err
	}
	includeHidden, err := parseBoolQuery(r, "include_hidden")
	if err != nil {
		return err
	}

	// Route to SearchAccounts when the request has any filter or pagination intent.
	// Otherwise use ListAccounts, which already filters hidden internally.
	hasSearchIntent := filter.Query != nil || filter.Currency != nil ||
		opts.Limit > 0 || opts.Offset > 0 || opts.IncludeCount

	if hasSearchIntent {
		filter.IncludeHidden = includeHidden
		res, err := s.svc.Account().SearchAccounts(ctx, filter, opts)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, res)
	}

	accounts, err := s.svc.Account().ListAccounts(ctx, service.ListAccountsOptions{
		Type:       filter.Type,
		ShowHidden: includeHidden,
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, wrapAsListResult(accounts))
}

func (s *Server) handleAccountTree(w http.ResponseWriter, r *http.Request) error {
	includeHidden, err := parseBoolQuery(r, "include_hidden")
	if err != nil {
		return err
	}
	roots, err := s.svc.Account().GetAccountTree(r.Context(), service.AccountTreeOptions{
		ShowHidden: includeHidden,
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, roots)
}
