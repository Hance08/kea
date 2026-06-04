// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

type unreconciledResponse struct {
	Entries               []*model.ReconcileEntry `json:"entries"`
	LastReconciledBalance int64                   `json:"last_reconciled_balance"`
}

func (s *Server) handleListUnreconciled(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	if _, err := s.svc.Account().GetAccountByID(ctx, id); err != nil {
		return err
	}
	entries, lastBalance, err := s.svc.Transaction().GetUnreconciledByAccount(ctx, id)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []*model.ReconcileEntry{}
	}
	return writeJSON(w, http.StatusOK, unreconciledResponse{
		Entries:               entries,
		LastReconciledBalance: lastBalance,
	})
}

type reconcilePreviewRequest struct {
	StatementBalance int64   `json:"statement_balance"`
	TransactionIDs   []int64 `json:"transaction_ids"`
}

type reconcilePreviewResponse struct {
	Difference int64 `json:"difference"`
}

func (s *Server) handleReconcilePreview(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	var req reconcilePreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	diff, err := s.svc.Transaction().PreviewReconcile(r.Context(), id, req.StatementBalance, req.TransactionIDs)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, reconcilePreviewResponse{Difference: diff})
}

type reconcileCommitRequest struct {
	StatementBalance int64   `json:"statement_balance"`
	TransactionIDs   []int64 `json:"transaction_ids"`
	AllowMismatch    bool    `json:"allow_mismatch"`
}

type reconcileCommitResponse struct {
	ReconciledCount       int   `json:"reconciled_count"`
	Difference            int64 `json:"difference"`
	LastReconciledBalance int64 `json:"last_reconciled_balance"`
}

func (s *Server) handleReconcileCommit(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	var req reconcileCommitRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()

	if !req.AllowMismatch {
		diff, err := s.svc.Transaction().PreviewReconcile(ctx, id, req.StatementBalance, req.TransactionIDs)
		if err != nil {
			return err
		}
		if diff != 0 {
			return &balanceMismatchError{Difference: diff}
		}
	}

	diff, err := s.svc.Transaction().ReconcileTransactions(ctx, id, req.StatementBalance, req.TransactionIDs)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, reconcileCommitResponse{
		ReconciledCount:       len(req.TransactionIDs),
		Difference:            diff,
		LastReconciledBalance: req.StatementBalance - diff,
	})
}
