// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
	"time"
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
