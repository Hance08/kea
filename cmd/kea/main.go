// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package main

import (
	"github.com/hance08/kea/cmd"
	"github.com/hance08/kea/migrations"
)

func main() {
	cmd.Execute(migrations.FS)
}
