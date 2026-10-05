// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package budget

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type BudgetSetProvider interface {
	SetBudget(ctx context.Context, input model.SetBudgetInput) (*model.Budget, error)
}

type ExpenseAccountLister interface {
	GetAccountsByType(ctx context.Context, accType model.AccountType) ([]*model.Account, error)
}

type setFlags struct {
	Month string
	JSON  bool
}

type setRunner struct {
	svc      BudgetSetProvider
	accounts ExpenseAccountLister
	out      io.Writer
}

func NewSetCmd(svc *service.Service) *cobra.Command {
	flags := &setFlags{}
	cmd := &cobra.Command{
		Use:   "set [<account> <amount>]",
		Short: "Set a monthly budget on an Expense account",
		Long: `Set the monthly budget of an Expense account from --month (default: current month) on.
Setting the same account and month again replaces that version. Run without arguments for an interactive prompt.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 && len(args) != 2 {
				return fmt.Errorf("expected <account> <amount>, or no arguments for interactive mode")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &setRunner{svc: svc.Budget(), accounts: svc.Account(), out: os.Stdout}
			return r.Run(cmd.Context(), args, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.Month, "month", "m", "", "first month the budget applies to (YYYY-MM, default: current month)")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output result as JSON")
	return cmd
}

func (r *setRunner) Run(ctx context.Context, args []string, flags *setFlags) error {
	var input model.SetBudgetInput
	var err error
	if len(args) == 0 {
		input, err = r.promptInput(ctx, flags)
	} else {
		input, err = r.inputFromArgs(args, flags)
	}
	if err != nil {
		return err
	}

	b, err := r.svc.SetBudget(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to set budget: %w", err)
	}
	if flags.JSON {
		return writeJSON(r.out, views.ToJSONBudget(*b))
	}
	pterm.Success.Printf("Budget for %s set to %s from %s\n", b.AccountName, utils.FormatAmount(b.Amount), b.EffectiveMonth)
	return nil
}

func (r *setRunner) inputFromArgs(args []string, flags *setFlags) (model.SetBudgetInput, error) {
	amount, err := utils.ParseAmount(args[1])
	if err != nil {
		return model.SetBudgetInput{}, fmt.Errorf("invalid amount %q: %w", args[1], err)
	}
	month := monthOrCurrent(flags.Month)
	if err := service.ValidateBudgetMonth(month); err != nil {
		return model.SetBudgetInput{}, err
	}
	return model.SetBudgetInput{AccountName: args[0], EffectiveMonth: month, Amount: amount}, nil
}

func (r *setRunner) promptInput(ctx context.Context, flags *setFlags) (model.SetBudgetInput, error) {
	accs, err := r.accounts.GetAccountsByType(ctx, model.AccountTypeExpense)
	if err != nil {
		return model.SetBudgetInput{}, fmt.Errorf("failed to load Expense accounts: %w", err)
	}
	names := make([]string, 0, len(accs))
	for _, a := range accs {
		if !a.IsHidden {
			names = append(names, a.Name)
		}
	}
	if len(names) == 0 {
		return model.SetBudgetInput{}, fmt.Errorf("no Expense accounts; create one with `kea account create`")
	}
	sort.Strings(names)

	account, err := prompts.PromptSelect("Expense account", names, names[0])
	if err != nil {
		return model.SetBudgetInput{}, err
	}
	amountStr, err := prompts.PromptAmount("Monthly budget", "e.g. 8000 or 8000.50", func(s string) error {
		_, err := utils.ParseAmount(s)
		return err
	})
	if err != nil {
		return model.SetBudgetInput{}, err
	}
	month, err := prompts.PromptInput("Effective from (YYYY-MM)", monthOrCurrent(flags.Month), service.ValidateBudgetMonth)
	if err != nil {
		return model.SetBudgetInput{}, err
	}
	amount, err := utils.ParseAmount(amountStr)
	if err != nil {
		return model.SetBudgetInput{}, fmt.Errorf("invalid amount %q: %w", amountStr, err)
	}
	return model.SetBudgetInput{AccountName: account, EffectiveMonth: month, Amount: amount}, nil
}
