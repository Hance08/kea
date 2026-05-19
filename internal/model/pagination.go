// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package model

// ListOptions controls pagination for list queries.
// IncludeCount lets callers skip the COUNT query when they don't need the total.
// Default limit is 20 when Limit is zero or negative.
type ListOptions struct {
	Limit        int  `json:"limit"`
	Offset       int  `json:"offset"`
	IncludeCount bool `json:"include_count,omitempty"`
}

// ListResult holds a paginated slice of items alongside pagination metadata.
type ListResult[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"total_count"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
}
