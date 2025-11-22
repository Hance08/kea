/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
// TODO: upgrade enter parent account using experience
// TODO: add back to previous step command
package cmd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hance08/kea/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	CentsPerUnit = 100
)

var (
	accName     string
	accParent   string
	accType     string
	accBalance  int
	accDesc     string
	accCurrency string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account.",
	Long: `In the beginning of using this tool, you need to create new accounts.
You must create type A (Asset), L(Liabilities), E(Expenses), R(Revenue)
four basic accounts, e.g. create an Asset account called Bank.

Advanced users can also create Equity (C) accounts.

Example: kea account create -t A -n Bank -b 100000`,
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		var finalName, finalType, finalCurrency string
		var parentID *int64
		var amountInCents int64 = 0

		scanner := bufio.NewScanner(os.Stdin)

		hasFlags := cmd.Flags().Changed("name") ||
			cmd.Flags().Changed("type") ||
			cmd.Flags().Changed("parent")

		// Flag mode
		if hasFlags {
			if accParent == "" && accType == "" {
				return fmt.Errorf("must enter at least one of --type or --parent flag")
			}
			if accParent != "" && accType != "" {
				return fmt.Errorf("--type and --parent flags can not use as the sametime")
			}

			// Validate account name first (before combining with parent/root)
			if err := validateAccountName(accName); err != nil {
				return err
			}

			if accParent != "" {
				parentAccount, err := logic.GetAccountByName(accParent)
				if err != nil {
					return err
				}

				finalName = accParent + ":" + accName
				finalType = parentAccount.Type
				finalCurrency = parentAccount.Currency
				parentID = &parentAccount.ID

			} else {
				rootName, err := logic.GetRootNameByType(accType)
				if err != nil {
					return err
				}

				finalName = rootName + ":" + accName
				finalType = accType

				if accCurrency != "" {
					if err := checkCurrency(); err != nil {
						return err
					}

					finalCurrency = accCurrency
				} else {
					finalCurrency = viper.GetString("defaults.currency")
				}
			}

			if err := validateAccountName(finalName); err != nil {
				return err
			}

			newAccount, err := logic.CreateAccount(finalName, finalType, finalCurrency, accDesc, parentID)
			if err != nil {
				return err
			}

			if accBalance != 0 {
				if accBalance < 0 {
					return fmt.Errorf("initial balance can't be negative")
				}
				balanceFloat := float64(accBalance)
				amountInCents = int64(math.Round(balanceFloat * CentsPerUnit))

				err = logic.SetBalance(newAccount, amountInCents)
				if err != nil {
					return err
				}
			}

			displayAccountSummary(finalName, finalType, finalCurrency, amountInCents, accDesc)
			displaySuccessInformation(newAccount.ID, finalName)
			return nil
		}

		// Interaaction mode
		fmt.Println("\nCreating a new account")
		fmt.Println("----------------------------------------")

		// step 1: check if is subaccount
		fmt.Print("Is this a subaccount? (y/n): ")
		scanner.Scan()
		isSubAccount := strings.ToLower(strings.TrimSpace(scanner.Text()))

		switch isSubAccount {
		case "y", "yes":
			// step 2a: select parent account
			parentAccount, err := selectParentAccount()
			if err != nil {
				return err
			}
			accParent = parentAccount.Name

			// step 3: enter account name
			if enterAccountName(accParent); err != nil {
				return err
			}

			finalName = accParent + ":" + accName
			finalType = parentAccount.Type
			finalCurrency = parentAccount.Currency
			parentID = &parentAccount.ID

		case "n", "no":
			// step 2b: select account type
			accType, err := selectType()
			if err != nil {
				return err
			}

			rootName, err := logic.GetRootNameByType(accType)
			if err != nil {
				return err
			}

			// step 3: enter account name
			if enterAccountName(rootName); err != nil {
				return err
			}

			finalName = rootName + ":" + accName
			finalType = accType
		default:
			return fmt.Errorf("please enter y(yes) or n(no)")
		}

		// step 4: currency setting
		if finalCurrency == "" {
			defaultCurrency := viper.GetString("defaults.currency")
			selectedCurrency, err := selectCurrency(defaultCurrency, false)
			if err != nil {
				return err
			}

			finalCurrency = selectedCurrency
		} else {
			selectedCurrency, err := selectCurrency(finalCurrency, true)
			if err != nil {
				return err
			}
			finalCurrency = selectedCurrency

		}

		// step 5: initial balance setting
		if finalType == "A" || finalType == "L" {
			balance, err := setInitialBalance()
			if err != nil {
				return err
			}
			amountInCents = balance
		}

		// step 6: description setting
		if err := setDescription(); err != nil {
			return err
		}

		// print put full informtaion
		fmt.Println("Confirm the following details:")
		displayAccountSummary(finalName, finalType, finalCurrency, amountInCents, accDesc)

		// confirm proceed the creation
		if err := confirmProceed(); err != nil {
			return err
		}

		// create account
		newAccount, err := createAccount(finalName, finalType, finalCurrency, accDesc, amountInCents, parentID)
		if err != nil {
			return err
		}
		displaySuccessInformation(newAccount.ID, finalName)

		return nil
	},
}

func init() {
	accountCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&accName, "name", "n", "", "Account Name")
	createCmd.Flags().StringVarP(&accParent, "parent", "p", "", "Parent FULL NAME)")
	createCmd.Flags().StringVarP(&accType, "type", "t", "", "Account Type (A,L,R,E) (Only use with top level accounts)")
	createCmd.Flags().IntVarP(&accBalance, "balance", "b", 0, "Setting Balance (Integer)")
	createCmd.Flags().StringVar(&accCurrency, "currency", "", "Currency Code (If not specified, it will use parent's currency or default from config)")
	createCmd.Flags().StringVarP(&accDesc, "description", "d", "", "Account description")
}

func selectType() (string, error) {
	var selected string
	promptSelect := &survey.Select{
		Message: "Account Types:",
		Options: []string{
			"A - Assets",
			"L - Liabilities",
			"R - Revenue",
			"E - Expenses",
			"C - Equity (Advanced)",
		},
		Default: "A - Assets",
	}

	err := survey.AskOne(promptSelect, &selected, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}
	selectedType := strings.Split(selected, " ")[0]

	return selectedType, nil
}

func selectParentAccount() (*store.Account, error) {
	allAccounts, err := logic.GetAllAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	var accountNames []string
	for _, acc := range allAccounts {
		accountNames = append(accountNames, acc.Name)
	}

	prompt := &survey.Input{
		Message: "Parent account FULL NAME:",
		Suggest: func(toComplete string) []string {
			var filtered []string
			for _, name := range accountNames {
				if strings.Contains(strings.ToLower(name), strings.ToLower(toComplete)) {
					filtered = append(filtered, name)
				}
			}
			return filtered
		},
	}

	err = survey.AskOne(prompt, &accParent, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))
	if err != nil {
		return nil, fmt.Errorf("input cancelled: %w", err)
	}

	// check parent account
	parentAccount, err := validateParentAccount(accParent)
	if err != nil {
		return nil, err
	}
	return parentAccount, nil
}

func enterAccountName(prefix string) error {
	promptName := &survey.Input{
		Message: "Account Name:",
	}
	err := survey.AskOne(promptName, &accName, survey.WithValidator(validateAccountNameWithPrefix(prefix)), survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))
	if err != nil {
		return fmt.Errorf("input cancelled: %w", err)
	}

	return nil
}

func selectCurrency(defaultCurrency string, isInherited bool) (string, error) {
	commonCurrencies := []string{
		"USD - US Dollar",
		"EUR - Euro",
		"GBP - British Pound",
		"JPY - Japanese Yen",
		"CNY - Chinese Yuan",
		"TWD - Taiwan Dollar",
		"HKD - Hong Kong Dollar",
		"SGD - Singapore Dollar",
		"Other (Custom)",
	}

	// Find default option in the list
	var defaultOption string
	for _, curr := range commonCurrencies {
		if strings.HasPrefix(curr, defaultCurrency) {
			defaultOption = curr
			break
		}
	}
	// If not found in common currencies, use the first one
	if defaultOption == "" {
		defaultOption = commonCurrencies[0]
	}

	var message string
	if isInherited {
		message = fmt.Sprintf("Currency (inherited: %s):", defaultCurrency)
	} else {
		message = fmt.Sprintf("Currency (default: %s):", defaultCurrency)
	}

	var selected string
	promptSelect := &survey.Select{
		Message: message,
		Options: commonCurrencies,
		Default: defaultOption,
	}

	err := survey.AskOne(promptSelect, &selected, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))
	if err != nil {
		return "", fmt.Errorf("input cancelled: %w", err)
	}

	// If "Other (Custom)" is selected, ask for custom input
	if selected == "Other (Custom)" {
		var customCurrency string
		promptInput := &survey.Input{
			Message: "Enter currency code:",
		}
		err := survey.AskOne(promptInput, &customCurrency, survey.WithValidator(func(val interface{}) error {
			curr := strings.TrimSpace(strings.ToUpper(val.(string)))
			if curr == "" {
				return fmt.Errorf("currency code can't be empty")
			}
			if len(curr) != 3 {
				return fmt.Errorf("currency code must be 3 characters (e.g. USD)")
			}
			for _, c := range curr {
				if c < 'A' || c > 'Z' {
					return fmt.Errorf("currency code must contain only letters")
				}
			}
			return nil
		}), survey.WithIcons(func(icons *survey.IconSet) {
			icons.Question.Text = "-"
		}))
		if err != nil {
			return "", fmt.Errorf("input cancelled: %w", err)
		}
		return strings.ToUpper(strings.TrimSpace(customCurrency)), nil
	}

	// Extract currency code from selection (first 3 characters)
	currencyCode := strings.Split(selected, " ")[0]
	return currencyCode, nil
}

func setInitialBalance() (int64, error) {
	var balanceInput string
	promptBalance := &survey.Input{
		Message: "Initial Balance (press Enter for 0):",
		Default: "0",
	}

	err := survey.AskOne(promptBalance, &balanceInput, survey.WithValidator(func(val any) error {
		input := strings.TrimSpace(val.(string))
		if input == "" || input == "0" {
			return nil
		}

		balanceFloat, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return fmt.Errorf("invalid number format")
		}

		if balanceFloat < 0 {
			return fmt.Errorf("initial balance can't be negative")
		}

		if balanceFloat > 9223372036854775 {
			return fmt.Errorf("balance amount too large")
		}

		return nil
	}), survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))

	if err != nil {
		return 0, fmt.Errorf("input cancelled: %w", err)
	}

	balanceInput = strings.TrimSpace(balanceInput)
	if balanceInput == "" || balanceInput == "0" {
		return 0, nil
	}

	balanceFloat, err := strconv.ParseFloat(balanceInput, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid input of Initial Balance")
	}

	amountInCents := int64(math.Round(balanceFloat * CentsPerUnit))
	return amountInCents, nil
}

func setDescription() error {
	promptDesc := &survey.Input{
		Message: "Description (optional):",
	}
	err := survey.AskOne(promptDesc, &accDesc, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))
	if err != nil {
		return fmt.Errorf("input cancelled: %w", err)
	}
	return nil
}

func confirmProceed() error {
	var confirm bool
	promptConfirm := &survey.Confirm{
		Message: "Proceed with account creation?",
		Default: true,
	}

	err := survey.AskOne(promptConfirm, &confirm, survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question.Text = "-"
	}))
	if err != nil {
		return fmt.Errorf("input cancelled: %w", err)
	}

	if !confirm {
		return fmt.Errorf("account creation cancelled")
	}

	return nil
}

func createAccount(name, _type, currency, desc string, amountInCents int64, parentID *int64) (*store.Account, error) {
	newAccount, err := logic.CreateAccount(name, _type, currency, desc, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	if amountInCents != 0 {
		err = logic.SetBalance(newAccount, amountInCents)
		if err != nil {
			return nil, fmt.Errorf("failed to set balance: %w", err)
		}
	}
	return newAccount, nil
}

func validateAccountName(val any) error {
	name, ok := val.(string)
	if !ok {
		return fmt.Errorf("account name must be a string")
	}

	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("account name can't be empty")
	}

	isExisted, err := logic.CheckAccountExists(name)
	if err != nil {
		return fmt.Errorf("failed to check account existence: %w", err)
	}
	if isExisted {
		return fmt.Errorf("account name %s is already existeddddd", name)
	}

	if strings.Contains(name, ":") {
		return fmt.Errorf("account name cannot contain ':' character")
	}

	reservedNames := []string{"Assets", "Liabilities", "Equity", "Revenue", "Expenses"}
	for _, reserved := range reservedNames {
		if strings.EqualFold(name, reserved) {
			return fmt.Errorf("'%s' is a reserved root account name", name)
		}
	}

	if len(name) > 100 {
		return fmt.Errorf("account name too long (max 100 characters)")
	}
	return nil
}

func validateAccountNameWithPrefix(prefix string) func(any) error {
	return func(val any) error {
		partialName := val.(string)

		if err := validateAccountName(partialName); err != nil {
			return err
		}

		fullName := prefix + ":" + partialName
		exists, err := logic.CheckAccountExists(fullName)
		if err != nil {
			return fmt.Errorf("failed to check account: %w", err)
		}
		if exists {
			return fmt.Errorf("account '%s' already exists", fullName)
		}

		return nil
	}
}

func validateParentAccount(name string) (*store.Account, error) {
	if name == "" {
		return nil, fmt.Errorf("parent account name can't be empty")
	}

	parentAccount, err := logic.GetAccountByName(name)
	if err != nil {
		return nil, fmt.Errorf("parent account not found: %w", err)
	}

	return parentAccount, nil
}

func checkCurrency() error {
	if accCurrency != "" {
		if len(accCurrency) != 3 {
			return fmt.Errorf("currency code must be 3 characters (e.g. USD)")
		}

		for _, c := range accCurrency {
			if c < 'A' || c > 'Z' {
				return fmt.Errorf("currency code must contain only letters")
			}
		}
	}
	return nil
}

func displayAccountSummary(finalName, finalType, finalCurrency string, amountInCents int64, description string) {
	fmt.Println("----------------------------------------")
	fmt.Printf("  Full Name   : %s\n", finalName)
	fmt.Printf("  Type        : %s\n", finalType)
	fmt.Printf("  Currency    : %s\n", finalCurrency)
	if amountInCents != 0 {
		fmt.Printf("  Balance     : %.2f\n", float64(amountInCents)/100)
	}
	if description != "" {
		fmt.Printf("  Description : %s\n", description)
	}
	fmt.Println("----------------------------------------")
}

func displaySuccessInformation(newAccountID int64, finalName string) {
	fmt.Println("----------------------------------------")
	fmt.Println("✓ Account created successfully!")
	fmt.Printf("  Account ID  : %d\n", newAccountID)
	fmt.Printf("  Full Name   : %s\n", finalName)
	fmt.Println("\nNext steps:")
	fmt.Println("  • View all accounts: kea account list")
	fmt.Println("  • Add a transaction: kea add")
	fmt.Println("----------------------------------------")
}
