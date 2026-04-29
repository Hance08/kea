// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package ui

import "github.com/pterm/pterm"

// Separator prints a green separator line to the console.
func Separator() {
	pterm.Println(pterm.Green("---------------------------------------------------------"))
}
