// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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
