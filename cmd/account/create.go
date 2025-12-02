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
	"github.com/hance08/kea/internal/store"
	"github.com/hance08/kea/internal/ui"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/validation"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	RunE:         runAccountCreate,
}

func runAccountCreate(cmd *cobra.Command, args []string) error {
	var finalName, finalType, finalCurrency string
	var parentID *int64
	var amountInCents int64 = 0

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
		if err := validator.ValidateAccountName(accName); err != nil {
			return err
		}

		if accParent != "" {
			parentAccount, err := logic.GetAccountByName(accParent)
			if err != nil {
				return err
			}

			finalName = accParent + ":" + accName
			finalType = parentAccount.Type
			if accCurrency != "" {
				finalCurrency = accCurrency
			} else {
				finalCurrency = parentAccount.Currency
			}
			parentID = &parentAccount.ID

		} else {
			rootName, err := logic.GetRootNameByType(accType)
			if err != nil {
				return err
			}

			finalName = rootName + ":" + accName
			finalType = accType

			if accCurrency != "" {
				if err := validation.ValidateCurrency(accCurrency); err != nil {
					return err
				}

				finalCurrency = strings.ToUpper(strings.TrimSpace(accCurrency))
			} else {
				finalCurrency = viper.GetString("defaults.currency")
			}
		}

		// Validate final name (check existence and length, but allow colons)
		if len(finalName) > constants.MaxNameLen {
			return fmt.Errorf("account name too long (max 100 characters)")
		}

		exists, err := logic.CheckAccountExists(finalName)
		if err != nil {
			return fmt.Errorf("failed to check account existence: %w", err)
		}
		if exists {
			return fmt.Errorf("account '%s' already exists", finalName)
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
			amountInCents = int64(math.Round(balanceFloat * constants.CentsPerUnit))

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
	// step 1: check if is subaccount
	isSubAccount, err := prompts.PromptIsSubAccount()
	if err != nil {
		return err
	}

	if isSubAccount {
		// step 2a: select parent account
		parentAccount, err := selectParentAccount()
		if err != nil {
			return err
		}
		accParent = parentAccount.Name

		// step 3: enter account name
		if err := enterAccountName(accParent); err != nil {
			return err
		}

		finalName = accParent + ":" + accName
		finalType = parentAccount.Type
		finalCurrency = parentAccount.Currency
		parentID = &parentAccount.ID

	} else {
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
		if err := enterAccountName(rootName); err != nil {
			return err
		}

		finalName = rootName + ":" + accName
		finalType = accType
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
}

func init() {
	createCmd.Flags().StringVarP(&accName, "name", "n", "", "Account Name")
	createCmd.Flags().StringVarP(&accParent, "parent", "p", "", "Parent FULL NAME)")
	createCmd.Flags().StringVarP(&accType, "type", "t", "", "Account Type (A,L,R,E) (Only use with top level accounts)")
	createCmd.Flags().IntVarP(&accBalance, "balance", "b", 0, "Setting Balance (Integer)")
	createCmd.Flags().StringVar(&accCurrency, "currency", "", "Currency Code (If not specified, it will use parent's currency or default from config)")
	createCmd.Flags().StringVarP(&accDesc, "description", "d", "", "Account description")
}

func selectType() (string, error) {
	return prompts.PromptAccountType()
}

func selectParentAccount() (*store.Account, error) {
	allAccounts, err := logic.GetAllAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve accounts: %w", err)
	}

	_, _, err = prompts.PromptParentAccount(allAccounts)
	if err != nil {
		return nil, err
	}

	// Validate parent account
	parentAccount, err := validator.ValidateParentAccount(accParent)
	if err != nil {
		return nil, err
	}
	return parentAccount, nil
}

func enterAccountName(prefix string) error {
	name, err := prompts.PromptAccountName(validator.ValidateAccountNameWithPrefix(prefix))
	if err != nil {
		return err
	}
	accName = name
	return nil
}

func selectCurrency(defaultCurrency string, isInherited bool) (string, error) {
	return prompts.PromptCurrency(defaultCurrency, isInherited, validation.ValidateCurrency)
}

func setInitialBalance() (int64, error) {
	balanceInput, err := prompts.PromptInitialBalance(validation.ValidateInitialBalance)
	if err != nil {
		return 0, err
	}

	balanceInput = strings.TrimSpace(balanceInput)
	if balanceInput == "" || balanceInput == "0" {
		return 0, nil
	}

	balanceFloat, err := strconv.ParseFloat(balanceInput, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid input of Initial Balance")
	}

	amountInCents := int64(math.Round(balanceFloat * constants.CentsPerUnit))
	return amountInCents, nil
}

func setDescription() error {
	desc, err := prompts.PromptDescription("Description (optional):", false)
	if err != nil {
		return err
	}
	accDesc = desc
	return nil
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

func displayAccountSummary(finalName, finalType, finalCurrency string, amountInCents int64, description string) {
	ui.Separator()

	balanceStr := fmt.Sprintf("%.2f", float64(amountInCents)/100)

	descStr := description
	if descStr == "" {
		descStr = "None"
	}

	tableData := pterm.TableData{
		{pterm.Blue("Full Name"), finalName},
		{pterm.Blue("Type"), finalType},
		{pterm.Blue("Currency"), finalCurrency},
		{pterm.Blue("Balance"), balanceStr},
		{pterm.Blue("Description"), descStr},
	}

	pterm.DefaultTable.WithData(tableData).Render()
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
