package account

import (
	"context"
	"errors"
	"fmt"

	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type createRunner struct {
	defaultCurrency string
	accSvc          CreateProvider
	view            CreateView
}

func NewCreateCmd(svc *service.Service) *cobra.Command {
	flags := &createFlags{}

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c"},
		Short:   "Create a new account.",
		Long: `In the beginning of using this tool, you need to create new accounts.
You must create type A (Asset), L(Liabilities), E(Expenses), R(Revenue)
four basic accounts, e.g. create an Asset account called Bank.

Advanced users can also create Equity (C) accounts.

Example:
  kea account create --type A --name Bank --balance 100000

  kea account create --parent Assets:Bank --name Bank1 --balance 100000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &createRunner{
				defaultCurrency: svc.Config().Defaults.Currency,
				accSvc:          svc.Account(),
				view:            views.NewAccountCreateView(),
			}

			return runner.Run(cmd.Context(), flags, cmd)
		},
	}
	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Account name")
	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Account type: A, L, R, E, C (no need when creating subaccount)")
	cmd.Flags().StringVarP(&flags.Parent, "parent", "p", "", "Parent account full name")
	cmd.Flags().StringVarP(&flags.BalanceStr, "balance", "b", "0", "Initial balance")
	cmd.Flags().StringVar(&flags.Currency, "currency", "", "Currency code")
	cmd.Flags().StringVarP(&flags.Description, "desc", "d", "", "Account description")
	cmd.Flags().BoolVarP(&flags.JSON, "json", "j", false, "output created account as JSON")

	return cmd
}

func (r *createRunner) Run(ctx context.Context, flags *createFlags, cmd *cobra.Command) error {
	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("parent") || cmd.Flags().Changed("balance") ||
		cmd.Flags().Changed("currency") || cmd.Flags().Changed("desc")

	if flags.JSON && !hasFlags {
		return fmt.Errorf("--json requires flags: --name and one of --type or --parent")
	}

	var input createInput
	var err error

	if hasFlags {
		input, err = r.runFromFlags(ctx, flags)
	} else {
		input, err = r.runInteractive(ctx)
	}
	if err != nil {
		if errors.Is(err, service.ErrAlreadyExists) {
			if flags.JSON {
				return err
			}
			pterm.Error.Println("Account already exists")
			return nil
		}
		return err
	}

	newAccount, err := r.createAccount(ctx, input)
	if err != nil {
		return err
	}

	if flags.JSON {
		bal, err := r.accSvc.GetAccountBalance(ctx, newAccount.ID)
		if err != nil {
			return err
		}
		return views.WriteJSON(views.ToJSONAccount(newAccount, bal))
	}
	r.view.ShowSuccess(fmt.Sprintf("Account created successfully (ID: %d)", newAccount.ID))
	return nil
}

func (r *createRunner) runFromFlags(ctx context.Context, flags *createFlags) (createInput, error) {
	if flags.Parent == "" && flags.Type == "" {
		return createInput{}, fmt.Errorf("must enter at least one of --type or --parent flag")
	}
	if flags.Parent != "" && flags.Type != "" {
		return createInput{}, fmt.Errorf("--type and --parent flags cannot be used at the same time")
	}

	if err := r.accSvc.ValidateAccountName(flags.Name); err != nil {
		return createInput{}, fmt.Errorf("invalid account name: %w", err)
	}

	var input createInput
	input.description = flags.Description

	if flags.Parent != "" {
		if err := r.buildFromParentName(ctx, flags.Parent, flags.Currency, &input); err != nil {
			return createInput{}, err
		}
	} else {
		if err := r.buildFromTypeFlag(flags.Type, flags.Currency, &input); err != nil {
			return createInput{}, err
		}
	}

	// input.fullName is now the prefix (parentAccount.Name or rootName)
	// FormatAccountName combines prefix + flags.Name to get the full path
	input.fullName = r.accSvc.FormatAccountName(input.fullName, flags.Name)
	if err := r.accSvc.ValidateFullAccountName(input.fullName); err != nil {
		return createInput{}, fmt.Errorf("validate account name: %w", err)
	}

	balanceCents, err := utils.ParseAmount(flags.BalanceStr)
	if err != nil {
		return createInput{}, fmt.Errorf("invalid balance format '%s': please enter a number (e.g. 100 or 100.50)", flags.BalanceStr)
	}
	input.balanceCents = balanceCents

	return input, nil
}

func (r *createRunner) runInteractive(ctx context.Context) (createInput, error) {
	var input createInput

	isSubAccount, err := prompts.PromptIsSubAccount()
	if err != nil {
		return createInput{}, err
	}

	if isSubAccount {
		parentAccount, err := r.promptParent(ctx)
		if err != nil {
			return createInput{}, err
		}
		nameInput, err := r.promptName(ctx, parentAccount.Name)
		if err != nil {
			return createInput{}, err
		}
		r.applyParentSettings(parentAccount, parentAccount.Currency, &input)
		input.fullName = r.accSvc.FormatAccountName(parentAccount.Name, nameInput)
	} else {
		accType, err := r.promptType()
		if err != nil {
			return createInput{}, err
		}
		rootName, err := r.accSvc.GetRootNameByType(accType)
		if err != nil {
			return createInput{}, err
		}
		nameInput, err := r.promptName(ctx, rootName)
		if err != nil {
			return createInput{}, err
		}
		if err := r.applyTypeSettings(accType, "", &input); err != nil {
			return createInput{}, err
		}
		input.fullName = r.accSvc.FormatAccountName(rootName, nameInput)
	}

	currency, err := r.promptCurrency(input)
	if err != nil {
		return createInput{}, err
	}
	input.currency = currency

	if input.accountType == "A" || input.accountType == "L" {
		balanceCents, err := r.promptBalance()
		if err != nil {
			return createInput{}, err
		}
		input.balanceCents = balanceCents
	}

	desc, err := r.promptDescription()
	if err != nil {
		return createInput{}, err
	}
	input.description = desc

	if err := r.view.RenderSummary(views.AccountSummaryItem{
		FullName:    input.fullName,
		Type:        input.accountType,
		Currency:    input.currency,
		Balance:     input.balanceCents,
		Description: input.description,
	}); err != nil {
		return createInput{}, err
	}

	if err := r.confirm(); err != nil {
		return createInput{}, err
	}

	return input, nil
}
