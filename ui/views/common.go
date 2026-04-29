// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import "github.com/pterm/pterm"

type CommonView struct {
}

func NewCommonView() *CommonView {
	return &CommonView{}
}

func (v *CommonView) ShowError(msg string, err error) {
	if err != nil {
		pterm.Error.Printf("%s: %v\n", msg, err)
	} else {
		pterm.Error.Println(msg)
	}
}

func (v *CommonView) ShowWarning(msg string) {
	pterm.Warning.Println(msg)
}

func (v *CommonView) ShowSuccess(msg string) {
	pterm.Success.Println(msg)
}

func (v *CommonView) ShowInfo(msg string) {
	pterm.Info.Println(msg)
}
