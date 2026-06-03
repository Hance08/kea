// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

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
	return writeJSON(w, http.StatusOK, balanceResponse{
		AccountID: id,
		Amount:    amount,
		Currency:  acc.Currency,
	})
}

// applyHiddenFilter removes hidden accounts from the result and updates
// TotalCount to reflect what was returned. Used after SearchAccounts, which
// does not itself filter hidden.
func applyHiddenFilter(res *model.ListResult[*model.Account], includeHidden bool) *model.ListResult[*model.Account] {
	if includeHidden {
		return res
	}
	out := make([]*model.Account, 0, len(res.Items))
	for _, a := range res.Items {
		if !a.IsHidden {
			out = append(out, a)
		}
	}
	res.Items = out
	res.TotalCount = len(out)
	return res
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
		res, err := s.svc.Account().SearchAccounts(ctx, filter, opts)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, applyHiddenFilter(res, includeHidden))
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
