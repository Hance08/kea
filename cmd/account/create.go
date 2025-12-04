/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
// TODO: add back to previous step command
package account

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/logic/accounting"
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/ui"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/validation"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Command-line flags
var (
	accName     string
	accType     string
	accParent   string
	accBalance  int
	accCurrency string
	accDesc     string
)

// AccountCreator manages the state and logic for creating an account
type AccountCreator struct {
	name        string
	fullName    string
	parentID    *int64
	accountType string
	currency    string
	balance     int64
	description string

	// Dependencies (injected)
	logic     *accounting.AccountingLogic
	validator *validation.AccountValidator
}

// NewAccountCreator creates a new AccountCreator instance with injected dependencies
func NewAccountCreator(l *accounting.AccountingLogic, v *validation.AccountValidator) *AccountCreator {
	return &AccountCreator{
		logic:     l,
		validator: v,
	}
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account.",
	Long: `In the beginning of using this tool, you need to create new accounts.
You must create type A (Asset), L(Liabilities), E(Expenses), R(Revenue)
four basic accounts, e.g. create an Asset account called Bank.

Advanced users can also create Equity (C) accounts.

Example: kea account create -t A -n Bank -b 100000`,
	SilenceUsage: true,
	RunE:         runAccountCreate,
}

func init() {
	createCmd.Flags().StringVarP(&accName, "name", "n", "", "Account name (without parent prefix)")
	createCmd.Flags().StringVarP(&accType, "type", "t", "", "Account type: A (Assets), L (Liabilities), R (Revenue), E (Expenses), C (Equity)")
	createCmd.Flags().StringVarP(&accParent, "parent", "p", "", "Parent account full name (mutually exclusive with --type)")
	createCmd.Flags().IntVarP(&accBalance, "balance", "b", 0, "Initial balance (integer, for Assets and Liabilities only)")
	createCmd.Flags().StringVar(&accCurrency, "currency", "", "Currency code (defaults to parent's currency or config default)")
	createCmd.Flags().StringVarP(&accDesc, "description", "d", "", "Account description (optional)")
}

func runAccountCreate(cmd *cobra.Command, args []string) error {
	// Inject dependencies from package-level variables
	creator := NewAccountCreator(logic, validator)

	hasFlags := cmd.Flags().Changed("name") ||
		cmd.Flags().Changed("type") ||
		cmd.Flags().Changed("parent")

	if hasFlags {
		return creator.FlagsMode(cmd)
	}

	return creator.InteractiveMode()
}

// FlagsMode builds an account from command-line flags
func (ac *AccountCreator) FlagsMode(cmd *cobra.Command) error {
	// Validate flag combinations
	if accParent == "" && accType == "" {
		return fmt.Errorf("must enter at least one of --type or --parent flag")
	}
	if accParent != "" && accType != "" {
		return fmt.Errorf("--type and --parent flags cannot be used at the same time")
	}

	// Validate account name (before combining with parent/root)
	if err := ac.validator.ValidateAccountName(accName); err != nil {
		return fmt.Errorf("invalid account name: %w", err)
	}

	ac.name = accName
	ac.description = accDesc

	// Build account based on parent or type
	if accParent != "" {
		if err := ac.buildFromParent(accParent, accCurrency); err != nil {
			return err
		}
	} else {
		if err := ac.buildFromType(accType, accCurrency); err != nil {
			return err
		}
	}

	// Validate final name using validation package
	if err := ac.validator.ValidateFullAccountName(ac.fullName); err != nil {
		return fmt.Errorf("validate account name: %w", err)
	}

	// Handle balance
	if accBalance != 0 {
		if accBalance < 0 {
			return fmt.Errorf("initial balance can't be negative")
		}
		balanceFloat := float64(accBalance)
		ac.balance = int64(math.Round(balanceFloat * constants.CentsPerUnit))
	}

	// Save account
	newAccount, err := ac.Save()
	if err != nil {
		return err
	}

	ac.displaySummary()
	displaySuccessInformation(newAccount.ID, ac.fullName)
	return nil
}

// InteractiveMode builds an account through interactive prompts
func (ac *AccountCreator) InteractiveMode() error {
	// Step 1: Check if is subaccount
	isSubAccount, err := prompts.PromptIsSubAccount()
	if err != nil {
		return err
	}

	if isSubAccount {
		// Step 2: Select parent account
		parentAccount, err := runSelectParentStep(ac)
		if err != nil {
			return err
		}

		// Step 3: Enter account name
		nameInput, err := runNameStep(ac, parentAccount.Name)
		if err != nil {
			return err
		}

		ac.setName(nameInput)

		if err := ac.buildFromParent(parentAccount.Name, parentAccount.Currency); err != nil {
			return err
		}

	} else {
		// Step 2: Select account type
		accType, err := runSelectTypeStep()
		if err != nil {
			return err
		}

		rootName, err := ac.logic.GetRootNameByType(accType)
		if err != nil {
			return err
		}

		// Step 3: Enter account name
		nameInput, err := runNameStep(ac, rootName)
		if err != nil {
			return err
		}

		ac.setName(nameInput)

		if err := ac.buildFromType(accType, ""); err != nil {
			return err
		}
	}

	// Step 4: Currency setting
	currency, err := runCurrencyStep(ac)
	if err != nil {
		return err
	}
	ac.setCurrency(currency)

	// Step 5: Initial balance setting
	if ac.accountType == "A" || ac.accountType == "L" {
		balance, err := runBalanceStep()
		if err != nil {
			return err // 同樣建議回傳 err
		}
		ac.setBalance(balance)
	}

	// Step 6: Description setting
	desc, err := runDescStep()
	if err != nil {
		return err
	}

	ac.setDescription(desc)
	ac.displaySummary()

	// Confirm proceed with creation
	if err := confirmProceed(); err != nil {
		return err
	}

	// Save account
	newAccount, err := ac.Save()
	if err != nil {
		return err
	}

	displaySuccessInformation(newAccount.ID, ac.fullName)
	return nil
}

// buildFromParent sets up account details based on parent account
func (ac *AccountCreator) buildFromParent(parentName, currency string) error {
	parentAccount, err := ac.logic.GetAccountByName(parentName)
	if err != nil {
		return err
	}

	ac.fullName = parentName + ":" + ac.name
	ac.accountType = parentAccount.Type
	ac.parentID = &parentAccount.ID

	if currency != "" {
		ac.currency = currency
	} else {
		ac.currency = parentAccount.Currency
	}

	return nil
}

// buildFromType sets up account details based on account type
func (ac *AccountCreator) buildFromType(accType, currency string) error {
	rootName, err := ac.logic.GetRootNameByType(accType)
	if err != nil {
		return fmt.Errorf("get root name: %w", err)
	}

	ac.fullName = rootName + ":" + ac.name
	ac.accountType = accType

	if currency != "" {
		if err := validator.ValidateCurrency(currency); err != nil {
			return err
		}
		ac.currency = strings.ToUpper(strings.TrimSpace(currency))
	} else {
		ac.currency = viper.GetString("defaults.currency")
	}

	return nil
}

func (ac *AccountCreator) setName(name string) {
	ac.name = name
}

func (ac *AccountCreator) setCurrency(currency string) {
	ac.currency = currency
}

func (ac *AccountCreator) setBalance(balance int64) {
	ac.balance = balance
}

func (ac *AccountCreator) setDescription(desc string) {
	ac.description = desc
}

func (ac *AccountCreator) displaySummary() {
	ui.Separator()

	balanceStr := fmt.Sprintf("%.2f", float64(ac.balance)/100)

	descStr := ac.description
	if descStr == "" {
		descStr = "None"
	}

	tableData := pterm.TableData{
		{pterm.Blue("Full Name"), ac.fullName},
		{pterm.Blue("Type"), ac.accountType},
		{pterm.Blue("Currency"), ac.currency},
		{pterm.Blue("Balance"), balanceStr},
		{pterm.Blue("Description"), descStr},
	}

	pterm.DefaultTable.WithData(tableData).Render()
}

// Save persists the account to the database
func (ac *AccountCreator) Save() (*store.Account, error) {
	newAccount, err := ac.logic.CreateAccount(ac.fullName, ac.accountType, ac.currency, ac.description, ac.parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	if ac.balance != 0 {
		err = ac.logic.SetBalance(newAccount, ac.balance)
		if err != nil {
			return nil, fmt.Errorf("failed to set balance: %w", err)
		}
	}

	return newAccount, nil
}

// Helper functions for interactive mode
func runSelectParentStep(ac *AccountCreator) (*store.Account, error) {
	allAccounts, err := ac.logic.GetAllAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	selectedName, selectedAccount, err := prompts.PromptParentAccount(allAccounts)
	if err != nil {
		return nil, err
	}

	parentAccount, err := ac.logic.GetAccountByName(selectedName)
	if err != nil {
		return nil, err
	}

	if selectedAccount != nil && selectedAccount.Name == parentAccount.Name {
		parentAccount = selectedAccount
	}

	return parentAccount, nil
}

func runNameStep(ac *AccountCreator, prefix string) (string, error) {
	return prompts.PromptAccountName(ac.validator.ValidateAccountNameWithPrefix(prefix))
}

func runSelectTypeStep() (string, error) {
	return prompts.PromptAccountType()
}

func runCurrencyStep(ac *AccountCreator) (string, error) {
	defaultCurrency := ac.currency

	if defaultCurrency == "" {
		//TODO: Validate the string in the config file
		defaultCurrency = viper.GetString("defaults.currency")
	}

	isInherited := ac.parentID != nil

	return prompts.PromptCurrency(defaultCurrency, isInherited, validator.ValidateCurrency)

}

func runBalanceStep() (int64, error) {
	balanceInput, err := prompts.PromptInitialBalance(validator.ValidateInitialBalance)
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

func displaySuccessInformation(newAccountID int64, finalName string) {
	ui.Separator()
	tableData := pterm.TableData{
		{pterm.Blue("Account ID"), fmt.Sprintf("%d", newAccountID)},
		{pterm.Blue("Full Name"), finalName},
	}
	pterm.DefaultTable.WithData(tableData).Render()
	pterm.Success.Print("Account created successfully!")
}
