// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
)

func parseInt64Query(r *http.Request, key string) (*int64, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, &service.ValidationError{Field: key, Message: key + " must be an integer"}
	}
	return &n, nil
}

func parseIntQuery(r *http.Request, key string) (*int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil, &service.ValidationError{Field: key, Message: key + " must be an integer"}
	}
	return &n, nil
}

func parseBoolQuery(r *http.Request, key string) (bool, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return false, nil
	}
	switch strings.ToLower(raw) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, &service.ValidationError{Field: key, Message: key + " must be true/false/1/0"}
	}
}

func parseStringQuery(r *http.Request, key string) *string {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	return &raw
}

func parseInt64Path(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, &service.ValidationError{Field: key, Message: key + " must be an integer"}
	}
	return n, nil
}

// parseListOptions reads ?limit=&offset=&include_count=.
// limit and offset must be >= 0 if present.
func parseListOptions(r *http.Request) (model.ListOptions, error) {
	var opts model.ListOptions
	limit, err := parseIntQuery(r, "limit")
	if err != nil {
		return opts, err
	}
	if limit != nil {
		if *limit < 0 {
			return opts, &service.ValidationError{Field: "limit", Message: "limit must be >= 0"}
		}
		opts.Limit = *limit
	}
	offset, err := parseIntQuery(r, "offset")
	if err != nil {
		return opts, err
	}
	if offset != nil {
		if *offset < 0 {
			return opts, &service.ValidationError{Field: "offset", Message: "offset must be >= 0"}
		}
		opts.Offset = *offset
	}
	inc, err := parseBoolQuery(r, "include_count")
	if err != nil {
		return opts, err
	}
	opts.IncludeCount = inc
	return opts, nil
}

func parseAccountFilter(r *http.Request) (model.AccountFilter, error) {
	var f model.AccountFilter
	f.Query = parseStringQuery(r, "q")
	f.Currency = parseStringQuery(r, "currency")
	if rawType := r.URL.Query().Get("type"); rawType != "" {
		at := model.AccountType(rawType)
		if !at.IsValid() {
			return f, &service.ValidationError{
				Field:   "type",
				Message: "type must be one of A, L, C, R, E",
			}
		}
		f.Type = &at
	}
	return f, nil
}

func parseTransactionFilter(r *http.Request) (model.TransactionFilter, error) {
	var f model.TransactionFilter

	accID, err := parseInt64Query(r, "account_id")
	if err != nil {
		return f, err
	}
	f.AccountID = accID

	if rawType := r.URL.Query().Get("type"); rawType != "" {
		tt := model.TransactionType(rawType)
		if !tt.IsValid() {
			return f, &service.ValidationError{
				Field:   "type",
				Message: "type must be one of Expense, Income, Transfer, Opening, Deposit, Withdrawal, Other",
			}
		}
		f.Type = &tt
	}

	if rawStatus := r.URL.Query().Get("status"); rawStatus != "" {
		st, perr := model.ParseTransactionStatus(rawStatus)
		if perr != nil {
			return f, &service.ValidationError{
				Field:   "status",
				Message: "status must be one of Pending, Cleared, Reconciled",
			}
		}
		f.Status = &st
	}

	start, err := parseInt64Query(r, "start_time")
	if err != nil {
		return f, err
	}
	f.StartTime = start

	end, err := parseInt64Query(r, "end_time")
	if err != nil {
		return f, err
	}
	f.EndTime = end

	if f.StartTime != nil && f.EndTime != nil && *f.StartTime > *f.EndTime {
		return f, &service.ValidationError{
			Field:   "end_time",
			Message: "end_time must be greater than or equal to start_time",
		}
	}

	f.Description = parseStringQuery(r, "description")
	return f, nil
}

func parseDateRangeParams(r *http.Request) service.DateRangeParams {
	q := r.URL.Query()
	return service.DateRangeParams{
		Month: q.Get("month"),
		From:  q.Get("from"),
		To:    q.Get("to"),
	}
}
