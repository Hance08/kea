// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
	"time"

	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleIncomeStatement(w http.ResponseWriter, r *http.Request) error {
	params := parseDateRangeParams(r)
	result, err := s.svc.Transaction().GenerateFullIncomeStatement(r.Context(), params)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleIncomeBreakdown(w http.ResponseWriter, r *http.Request) error {
	params := parseDateRangeParams(r)
	result, err := s.svc.Transaction().GenerateFullIncomeBreakdown(r.Context(), params)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleExpenseBreakdown(w http.ResponseWriter, r *http.Request) error {
	params := parseDateRangeParams(r)
	result, err := s.svc.Transaction().GenerateFullExpenseBreakdown(r.Context(), params)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBalanceSheet(w http.ResponseWriter, r *http.Request) error {
	asOfPtr, err := parseInt64Query(r, "as_of")
	if err != nil {
		return err
	}
	asOf := time.Now().Unix()
	if asOfPtr != nil {
		asOf = *asOfPtr
	}
	result, err := s.svc.Transaction().GenerateBalanceSheet(r.Context(), asOf)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, result)
}

type netWorthResponse struct {
	At       int64            `json:"at"`
	NetWorth map[string]int64 `json:"net_worth"`
}

func (s *Server) handleNetWorth(w http.ResponseWriter, r *http.Request) error {
	atPtr, err := parseInt64Query(r, "at")
	if err != nil {
		return err
	}
	at := time.Now().Unix()
	if atPtr != nil {
		at = *atPtr
	}
	nw, err := s.svc.Transaction().GetNetWorthAt(r.Context(), at)
	if err != nil {
		return err
	}
	if nw == nil {
		nw = map[string]int64{}
	}
	return writeJSON(w, http.StatusOK, netWorthResponse{At: at, NetWorth: nw})
}

type balanceHistoryResponse struct {
	Items []model.AccountMonthlySeries `json:"items"`
}

func (s *Server) handleBalanceHistory(w http.ResponseWriter, r *http.Request) error {
	items, err := s.svc.Transaction().GetMonthlyBalanceHistory(r.Context())
	if err != nil {
		return err
	}
	if items == nil {
		items = []model.AccountMonthlySeries{}
	}
	return writeJSON(w, http.StatusOK, balanceHistoryResponse{Items: items})
}
