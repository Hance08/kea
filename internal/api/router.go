// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Recoverer)
	r.Use(requestIDMiddleware)
	r.Use(loggerMiddleware(s.logger))
	r.Use(accessLogMiddleware)
	r.Use(corsMiddleware(s.cfg.Server.CORSOrigins))

	r.Route("/api", func(r chi.Router) {
		r.Method(http.MethodGet, "/health", apiHandler(s.handleHealth))
		r.Method(http.MethodGet, "/version", apiHandler(s.handleVersion))
		r.Method(http.MethodGet, "/accounts", apiHandler(s.handleListAccounts))
		r.Method(http.MethodPost, "/accounts", apiHandler(s.handleCreateAccount))
		r.Method(http.MethodGet, "/accounts/tree", apiHandler(s.handleAccountTree))
		r.Method(http.MethodGet, "/accounts/by-name", apiHandler(s.handleAccountByName))
		r.Method(http.MethodGet, "/accounts/{id}", apiHandler(s.handleAccountByID))
		r.Method(http.MethodPatch, "/accounts/{id}", apiHandler(s.handleUpdateAccount))
		r.Method(http.MethodDelete, "/accounts/{id}", apiHandler(s.handleDeleteAccount))
		r.Method(http.MethodGet, "/accounts/{id}/balance", apiHandler(s.handleAccountBalance))
		r.Method(http.MethodGet, "/accounts/{id}/unreconciled", apiHandler(s.handleListUnreconciled))
		r.Method(http.MethodPost, "/accounts/{id}/reconcile/preview", apiHandler(s.handleReconcilePreview))
		r.Method(http.MethodPost, "/accounts/{id}/reconcile", apiHandler(s.handleReconcileCommit))
		r.Method(http.MethodGet, "/transactions", apiHandler(s.handleListTransactions))
		r.Method(http.MethodPost, "/transactions", apiHandler(s.handleCreateTransaction))
		r.Method(http.MethodGet, "/transactions/{id}", apiHandler(s.handleTransactionByID))
		r.Method(http.MethodDelete, "/transactions/{id}", apiHandler(s.handleDeleteTransaction))
		r.Method(http.MethodPatch, "/transactions/{id}", apiHandler(s.handleUpdateTransaction))
		r.Method(http.MethodPatch, "/transactions/{id}/status", apiHandler(s.handleUpdateTransactionStatus))
		r.Method(http.MethodGet, "/reports/income-statement", apiHandler(s.handleIncomeStatement))
		r.Method(http.MethodGet, "/reports/income-breakdown", apiHandler(s.handleIncomeBreakdown))
		r.Method(http.MethodGet, "/reports/expense-breakdown", apiHandler(s.handleExpenseBreakdown))
		r.Method(http.MethodGet, "/reports/balance-sheet", apiHandler(s.handleBalanceSheet))
		r.Method(http.MethodGet, "/reports/net-worth", apiHandler(s.handleNetWorth))
		r.Method(http.MethodGet, "/ledgers/active", apiHandler(s.handleActiveLedger))
		r.Method(http.MethodGet, "/ledgers", apiHandler(s.handleListLedgers))
		r.Method(http.MethodPost, "/ledgers", apiHandler(s.handleCreateLedger))
		r.Method(http.MethodPost, "/ledgers/switch", apiHandler(s.handleSwitchLedger))
		r.Method(http.MethodDelete, "/ledgers/{name}", apiHandler(s.handleDeleteLedger))
	})

	return r
}
