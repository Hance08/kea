package account

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hance08/kea/internal/constants"
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

// CreateCommandRunner manages the state and svc for creating an account
type CreateCommandRunner struct {
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
		Use:   "create",
		Short: "Create a new account.",
		Long: `In the beginning of using this tool, you need to create new accounts.
You must create type A (Asset), L(Liabilities), E(Expenses), R(Revenue)
four basic accounts, e.g. create an Asset account called Bank.

Advanced users can also create Equity (C) accounts.

Example: kea account create -t A -n Bank -b 100000`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &CreateCommandRunner{
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

func (r *CreateCommandRunner) Run(flags *createFlags, cmd *cobra.Command) error {
	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("parent")

	if hasFlags {
		err := r.flagsMode(flags)
		if err != nil {
			if errors.Is(err, store.ErrAccountExists) {
				pterm.Error.Println("Account already exists")
			} else {
				return err
			}
		}
		return nil
	}

	err := r.interactiveMode()
	if err != nil {
		return err
	}
	return nil
}

// FlagsMode builds an account from command-line flags
func (r *CreateCommandRunner) flagsMode(flags *createFlags) error {
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
		if err := r.buildFromParent(flags.Parent, flags.Currency); err != nil {
			return err
		}
	} else {
		if err := r.buildFromType(flags.Type, flags.Currency); err != nil {
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

	// Save account
	newAccount, err := r.Save()
	if err != nil {
		return err
	}

	if err := views.RenderAccountSummary(views.AccountSummaryItem{
		FullName:    r.fullName,
		Type:        r.accountType,
		Currency:    r.currency,
		Balance:     r.balance,
		Description: r.description}); err != nil {
		return err
	}

	if err := views.RenderAccountSuccess(newAccount.ID, r.fullName); err != nil {
		return err
	}
	return nil
}

// InteractiveMode builds an account through interactive prompts
func (r *CreateCommandRunner) interactiveMode() error {
	// Step 1: Check if is subaccount
	isSubAccount, err := prompts.PromptIsSubAccount()
	if err != nil {
		return err
	}

	if isSubAccount {
		// Step 2: Select parent account
		parentAccount, err := r.runSelectParentStep()
		if err != nil {
			return err
		}

		// Step 3: Enter account name
		nameInput, err := r.runNameStep(parentAccount.Name)
		if err != nil {
			return err
		}

		r.setName(nameInput)

		if err := r.buildFromParent(parentAccount.Name, parentAccount.Currency); err != nil {
			return err
		}

	} else {
		// Step 2: Select account type
		accType, err := runSelectTypeStep()
		if err != nil {
			return err
		}

		rootName, err := r.svc.Account.GetRootNameByType(accType)
		if err != nil {
			return err
		}

		// Step 3: Enter account name
		nameInput, err := r.runNameStep(rootName)
		if err != nil {
			return err
		}

		r.setName(nameInput)

		if err := r.buildFromType(accType, ""); err != nil {
			return err
		}
	}

	// Step 4: Currency setting
	currency, err := r.runCurrencyStep()
	if err != nil {
		return err
	}
	r.setCurrency(currency)

	// Step 5: Initial balance setting
	if r.accountType == "A" || r.accountType == "L" {
		balance, err := r.runBalanceStep()
		if err != nil {
			return err
		}
		r.CreateOpeningBalance(balance)
	}

	// Step 6: Description setting
	desc, err := runDescStep()
	if err != nil {
		return err
	}

	r.setDescription(desc)
	views.RenderAccountSummary(views.AccountSummaryItem{
		FullName:    r.fullName,
		Type:        r.accountType,
		Currency:    r.currency,
		Balance:     r.balance,
		Description: r.description})

	// Confirm proceed with creation
	if err := confirmProceed(); err != nil {
		return err
	}

	// Save account
	newAccount, err := r.Save()
	if err != nil {
		return err
	}

	views.RenderAccountSuccess(newAccount.ID, r.fullName)
	return nil
}

// buildFromParent sets up account details based on parent account
func (r *CreateCommandRunner) buildFromParent(parentName, currency string) error {
	parentAccount, err := r.svc.Account.GetAccountByName(parentName)
	if err != nil {
		return err
	}

	r.fullName = parentName + ":" + r.name
	r.accountType = parentAccount.Type
	r.parentID = &parentAccount.ID

	if currency != "" {
		r.currency = currency
	} else {
		r.currency = parentAccount.Currency
	}

	return nil
}

// buildFromType sets up account details based on account type
func (r *CreateCommandRunner) buildFromType(accType, currency string) error {
	rootName, err := r.svc.Account.GetRootNameByType(accType)
	if err != nil {
		return fmt.Errorf("get root name: %w", err)
	}

	r.fullName = rootName + ":" + r.name
	r.accountType = accType

	if currency != "" {
		if err := r.validator.ValidateCurrency(currency); err != nil {
			return err
		}
		r.currency = strings.ToUpper(strings.TrimSpace(currency))
	} else {
		r.currency = r.svc.Config.Defaults.Currency
	}

	return nil
}

func (r *CreateCommandRunner) setName(name string) {
	r.name = name
}

func (r *CreateCommandRunner) setCurrency(currency string) {
	r.currency = currency
}

func (r *CreateCommandRunner) CreateOpeningBalance(balance int64) {
	r.balance = balance
}

func (r *CreateCommandRunner) setDescription(desc string) {
	r.description = desc
}

// Save persists the account to the database
func (r *CreateCommandRunner) Save() (*store.Account, error) {
	newAccount, err := r.svc.Account.CreateAccount(r.fullName, r.accountType, r.currency, r.description, r.parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	if r.balance != 0 {
		err = r.svc.Transaction.CreateOpeningBalance(newAccount, r.balance)
		if err != nil {
			return nil, fmt.Errorf("failed to set balance: %w", err)
		}
	}

	return newAccount, nil
}

func (r *CreateCommandRunner) runBalanceStep() (int64, error) {
	balanceInput, err := prompts.PromptInitialBalance(r.validator.ValidateInitialBalance)
	if err != nil {
		return 0, err
	}

	balanceInput = strings.TrimSpace(balanceInput)
	if balanceInput == "" || balanceInput == "0" {
		return 0, nil
	}

	balanceFloat, err := strconv.ParseFloat(balanceInput, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid balance input: must be a number")
	}

	return int64(math.Round(balanceFloat * constants.CentsPerUnit)), nil

}

// Helper functions for interactive mode
func (r *CreateCommandRunner) runSelectParentStep() (*store.Account, error) {
	allAccounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	selectedName, selectedAccount, err := prompts.PromptParentAccount(allAccounts)
	if err != nil {
		return nil, err
	}

	parentAccount, err := r.svc.Account.GetAccountByName(selectedName)
	if err != nil {
		return nil, err
	}

	if selectedAccount != nil && selectedAccount.Name == parentAccount.Name {
		parentAccount = selectedAccount
	}

	return parentAccount, nil
}

func (r *CreateCommandRunner) runNameStep(prefix string) (string, error) {
	surveyValidator := func(val interface{}) error {
		inputStr, ok := val.(string)
		if !ok {
			return fmt.Errorf("invalid type")
		}

		if err := r.validator.ValidateAccountName(inputStr); err != nil {
			return err
		}

		fullName := prefix + ":" + inputStr

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

func (r *CreateCommandRunner) runCurrencyStep() (string, error) {
	defaultCurrency := r.currency

	if defaultCurrency == "" {
		//TODO: Validate the string in the config file
		defaultCurrency = r.svc.Config.Defaults.Currency
	}

	isInherited := r.parentID != nil

	return prompts.PromptCurrency(defaultCurrency, isInherited, r.validator.ValidateCurrency)

}

func runSelectTypeStep() (string, error) {
	return prompts.PromptAccountType()
}

func runDescStep() (string, error) {
	return prompts.PromptDescription("Description (optional):", false)
}

func confirmProceed() error {
	confirm, err := prompts.PromptConfirm("Proceed with account creation?", true)
	if err != nil {
		return err
	}

	if !confirm {
		return fmt.Errorf("account creation cancelled")
	}

	return nil
}
