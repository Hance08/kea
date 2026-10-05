// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

// Command kea is the entry point of the kea CLI, TUI, and HTTP server. See
// docs/architecture.md.
package main

import (
	"github.com/hance08/kea/cmd"
	"github.com/hance08/kea/migrations"
)

func main() {
	cmd.Execute(migrations.FS)
}
