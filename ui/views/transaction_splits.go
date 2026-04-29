// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package views

import (
	"fmt"
	"io"
	"os"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pterm/pterm"
)

type TransactionSplitsView struct {
	out io.Writer
}

func NewTransactionSplitsView() *TransactionSplitsView {
	return &TransactionSplitsView{
		out: os.Stdout,
	}
}

func (v *TransactionSplitsView) Render(splits []model.SplitDetail) {
	table := tablewriter.NewWriter(v.out)

	table.SetColumnAlignment([]int{
		tablewriter.ALIGN_LEFT,
		tablewriter.ALIGN_RIGHT,
		tablewriter.ALIGN_LEFT,
	})
	table.SetHeader([]string{})
	table.SetBorder(false)
	table.SetColumnSeparator("|")
	table.SetAutoWrapText(false)

	for _, split := range splits {
		formattedAmt := utils.FormatAmount(split.Amount)
		if len(formattedAmt) > 0 && formattedAmt[0] == '-' {
			formattedAmt = formattedAmt[1:]
		}

		var displayAccount string
		var displayAmount string

		if split.Amount < 0 {
			displayAccount = fmt.Sprintf("    %s", split.AccountName)
			displayAmount = formattedAmt
		} else {
			displayAccount = split.AccountName
			displayAmount = formattedAmt
		}

		table.Append([]string{
			displayAccount,
			displayAmount,
		})
	}

	table.Render()

	if v.out == os.Stdout {
		pterm.Println()
	} else {
		_, _ = fmt.Fprintln(v.out)
	}
}

func (v *TransactionSplitsView) SetOutput(w io.Writer) {
	v.out = w
}
