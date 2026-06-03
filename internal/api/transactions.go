// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/hance08/kea/internal/model"
)

func (s *Server) handleTransactionByID(w http.ResponseWriter, r *http.Request) error {
	id, err := parseInt64Path(r, "id")
	if err != nil {
		return err
	}
	d, err := s.svc.Transaction().GetTransactionByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	opts, err := parseListOptions(r)
	if err != nil {
		return err
	}
	filter, err := parseTransactionFilter(r)
	if err != nil {
		return err
	}

	res, err := s.svc.Transaction().FilterTransactions(ctx, filter, opts)
	if err != nil {
		return err
	}

	details, err := s.svc.Transaction().GetTransactionDetailsByIDs(ctx, res.Items)
	if err != nil {
		return err
	}

	out := make([]*model.TransactionDetail, 0, len(res.Items))
	for _, tx := range res.Items {
		if d, ok := details[tx.ID]; ok {
			out = append(out, d)
		}
	}

	return writeJSON(w, http.StatusOK, &model.ListResult[*model.TransactionDetail]{
		Items:      out,
		TotalCount: res.TotalCount,
		Limit:      res.Limit,
		Offset:     res.Offset,
	})
}
