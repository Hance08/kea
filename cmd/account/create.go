package account

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/internal/validation"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// Command-line flags
type createFlags struct {
	Name        string
	Type        string
	Parent      string
	BalanceStr  string
	Currency    string
	Description string
}

// createRunner manages the state and svc for creating an account
type createRunner struct {
	name        string
	fullName    string
	parentID    *int64
	accountType string
	currency    string
	balance     int64
	description string

	svc       *service.Service
	validator *validation.AccountValidator
}

func NewCreateCmd(svc *service.Service) *cobra.Command {
	flags := &createFlags{}
	validator := validation.NewAccountValidator()

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c"},
		Short:   "Create a new account.",
		Long: `In the beginning of using this tool, you need to create new accounts.
You must create type A (Asset), L(Liabilities), E(Expenses), R(Revenue)
four basic accounts, e.g. create an Asset account called Bank.

Advanced users can also create Equity (C) accounts.

Example: kea account create -t A -n Bank -b 100000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &createRunner{
				svc:       svc,
				validator: validator,
			}

			return runner.Run(flags, cmd)
		},
	}
	cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "Account name")
	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Account type: A, L, R, E, C")
	cmd.Flags().StringVarP(&flags.Parent, "parent", "p", "", "Parent account full name")
	cmd.Flags().StringVarP(&flags.BalanceStr, "balance", "b", "0", "Initial balance")
	cmd.Flags().StringVar(&flags.Currency, "currency", "", "Currency code")
	cmd.Flags().StringVarP(&flags.Description, "description", "d", "", "Account description")

	return cmd
}

func (r *createRunner) Run(flags *createFlags, cmd *cobra.Command) error {
	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("parent")

	if hasFlags {
		err := r.runFromFlags(flags)
		if err != nil {
			if errors.Is(err, store.ErrAccountExists) {
				pterm.Error.Println("Account already exists")
			} else {
				return err
			}
		}
		return nil
	}

	err := r.runInteractive()
	if err != nil {
		return err
	}
	return nil
}

func (r *createRunner) runFromFlags(flags *createFlags) error {
	// Validate flag combinations
	if flags.Parent == "" && flags.Type == "" {
		return fmt.Errorf("must enter at least one of --type or --parent flag")
	}
	if flags.Parent != "" && flags.Type != "" {
		return fmt.Errorf("--type and --parent flags cannot be used at the same time")
	}

	// Validate account name (before combining with parent/root)
	if err := r.validator.ValidateAccountName(flags.Name); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	r.name = flags.Name
	r.description = flags.Description

	// Build account based on parent or type
	if flags.Parent != "" {
		if err := r.buildFromParentName(flags.Parent, flags.Currency); err != nil {
			return err
		}
	} else {
		if err := r.buildFromTypeFlag(flags.Type, flags.Currency); err != nil {
			return err
		}
	}

	// Validate final name using validation package
	if err := r.validator.ValidateFullAccountName(r.fullName); err != nil {
		return fmt.Errorf("validate account name: %w", err)
	}

	// Handle balance
	balance, err := utils.ParseToCents(flags.BalanceStr)
	if err != nil {
		return fmt.Errorf("invalid balance format '%s': please enter a number (e.g. 100 or 100.50)", flags.BalanceStr)
	}

	r.balance = balance

	// createAccount account
	newAccount, err := r.createAccount()
	if err != nil {
		return err
	}

	if err := views.RenderAccountSuccess(newAccount.ID, r.fullName); err != nil {
		return err
	}
	return nil
}

func (r *createRunner) runInteractive() error {
	// Step 1: Check if is subaccount
	isSubAccount, err := prompts.PromptIsSubAccount()
	if err != nil {
		return err
	}

	if isSubAccount {
		// Step 2: Select parent account
		parentAccount, err := r.promptParent()
		if err != nil {
			return err
		}

		// Step 3: Enter account name
		nameInput, err := r.promptName(parentAccount.Name)
		if err != nil {
			return err
		}

		r.name = nameInput

		r.applyParentSettings(parentAccount, parentAccount.Currency)

	} else {
		// Step 2: Select account type
		accType, err := r.promptType()
		if err != nil {
			return err
		}

		rootName, err := r.svc.Account.GetRootNameByType(accType)
		if err != nil {
			return err
		}

		// Step 3: Enter account name
		nameInput, err := r.promptName(rootName)
		if err != nil {
			return err
		}

		r.name = nameInput

		if err := r.applyTypeSettings(rootName, accType, ""); err != nil {
			return err
		}
	}

	// Step 4: Currency setting
	currency, err := r.promptCurrency()
	if err != nil {
		return err
	}
	r.currency = currency

	// Step 5: Initial balance setting
	if r.accountType == "A" || r.accountType == "L" {
		balance, err := r.promptBalance()
		if err != nil {
			return err
		}
		r.balance = balance
	}

	// Step 6: Description setting
	desc, err := r.promptDescription()
	if err != nil {
		return err
	}

	r.description = desc
	if err := views.RenderAccountSummary(views.AccountSummaryItem{
		FullName:    r.fullName,
		Type:        r.accountType,
		Currency:    r.currency,
		Balance:     r.balance,
		Description: r.description}); err != nil {
		return err
	}

	// Confirm proceed with creation
	if err := r.confirm(); err != nil {
		return err
	}

	// createAccount account
	newAccount, err := r.createAccount()
	if err != nil {
		return err
	}

	if err := views.RenderAccountSuccess(newAccount.ID, r.fullName); err != nil {
		return err
	}
	return nil
}

func (r *createRunner) createAccount() (*model.Account, error) {

	return r.svc.CreateAccountWithBalance(
		r.fullName,
		r.accountType,
		r.currency,
		r.description,
		r.parentID,
		r.balance,
	)

}

func (r *createRunner) applyTypeSettings(rootName, accType, currencyOverride string) error {
	r.fullName = r.svc.Account.FormatAccountName(rootName, r.name)
	r.accountType = accType

	if currencyOverride != "" {
		if err := r.validator.ValidateCurrency(currencyOverride); err != nil {
			return err
		}
		r.currency = strings.ToUpper(strings.TrimSpace(currencyOverride))
	} else {
		r.currency = r.svc.Config.Defaults.Currency
	}
	return nil
}

func (r *createRunner) applyParentSettings(parent *model.Account, currencyOverride string) {
	r.fullName = r.svc.Account.FormatAccountName(parent.Name, r.name)
	r.accountType = parent.Type
	r.parentID = &parent.ID

	if currencyOverride != "" {
		r.currency = currencyOverride
	} else {
		r.currency = parent.Currency
	}
}

func (r *createRunner) buildFromParentName(parentName, currency string) error {
	parentAccount, err := r.svc.Account.GetAccountByName(parentName)
	if err != nil {
		return err
	}

	r.applyParentSettings(parentAccount, currency)
	return nil
}

func (r *createRunner) buildFromTypeFlag(accType, currency string) error {
	rootName, err := r.svc.Account.GetRootNameByType(accType)
	if err != nil {
		return fmt.Errorf("get root name: %w", err)
	}

	return r.applyTypeSettings(rootName, accType, currency)
}

func (r *createRunner) promptType() (string, error) {
	return prompts.PromptAccountType()
}

func (r *createRunner) promptParent() (*model.Account, error) {
	allAccounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	_, selectedAccount, err := prompts.PromptParentAccount(allAccounts)
	if err != nil {
		return nil, err
	}

	if selectedAccount == nil {
		return nil, fmt.Errorf("no account selected")
	}

	return selectedAccount, nil
}

func (r *createRunner) promptName(prefix string) (string, error) {
	surveyValidator := func(inputStr string) error {
		if err := r.validator.ValidateAccountName(inputStr); err != nil {
			return err
		}

		fullName := r.svc.Account.FormatAccountName(prefix, inputStr)

		exists, err := r.svc.Account.CheckAccountExists(fullName)
		if err != nil {
			return fmt.Errorf("failed to validate: %w", err)
		}
		if exists {
			return fmt.Errorf("account '%s' already exists", fullName)
		}
		return nil
	}
	return prompts.PromptAccountName(surveyValidator)
}

func (r *createRunner) promptCurrency() (string, error) {
	defaultCurrency := r.currency

	if defaultCurrency == "" {
		//TODO: Validate the string in the config file
		defaultCurrency = r.svc.Config.Defaults.Currency
	}

	isInherited := r.parentID != nil

	return prompts.PromptCurrency(defaultCurrency, isInherited, r.validator.ValidateCurrency)

}

func (r *createRunner) promptBalance() (int64, error) {
	balanceInput, err := prompts.PromptInitialBalance(r.validator.ValidateInitialBalance)
	if err != nil {
		return 0, err
	}

	return utils.ParseToCents(balanceInput)
}

func (r *createRunner) promptDescription() (string, error) {
	return prompts.PromptDescription("Description (optional):", false)
}

func (r *createRunner) confirm() error {
	confirm, err := prompts.PromptConfirm("Proceed with account creation?", true)
	if err != nil {
		return err
	}

	if !confirm {
		return fmt.Errorf("account creation cancelled")
	}

	return nil
}
