// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

// Package migrations embeds kea's SQL schema migrations for golang-migrate. See
// docs/recipes/add-migration.md.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
