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
		r.Method(http.MethodGet, "/accounts/by-name", apiHandler(s.handleAccountByName))
		r.Method(http.MethodGet, "/accounts/{id}",         apiHandler(s.handleAccountByID))
		r.Method(http.MethodGet, "/accounts/{id}/balance", apiHandler(s.handleAccountBalance))
	})

	return r
}
