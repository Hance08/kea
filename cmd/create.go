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
			if err := validateAccountName(accName); err != nil {
				return err
			}

			if accParent == "" && accType == "" {
				return fmt.Errorf("must enter at least one of --type or --parent flag")
			}
			if accParent != "" && accType != "" {
				return fmt.Errorf("--type and --parent flags can not use as the sametime")
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
			// step 2a: enter parent account name
			allAccounts, err := logic.ListAllAccounts()
			if err != nil {
				return fmt.Errorf("failed to retrieve accounts: %w", err)
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
				return fmt.Errorf("input cancelled: %w", err)
			}

			if accParent == "" {
				return fmt.Errorf("parent account name can't be empty")
			}
			// check parent account
			parentAccount, err := logic.GetAccountByName(accParent)
			if err != nil {
				return fmt.Errorf("parent account not found: %w", err)
			}
			// step 3: enter account name
			promptName := &survey.Input{
				Message: "Account Name:",
			}
			err = survey.AskOne(promptName, &accName, survey.WithValidator(validateAccountName))
			if err != nil {
				return fmt.Errorf("input cancelled: %w", err)
			}

			finalName = accParent + ":" + accName
			finalType = parentAccount.Type
			finalCurrency = parentAccount.Currency
			parentID = &parentAccount.ID

		case "n", "no":
			// step 2b: enter account type
			fmt.Print("\nAccount type\n")
			fmt.Println("----------------------------------------")
			fmt.Println("  A = Assets       L = Liabilities")
			fmt.Println("  R = Revenue      E = Expenses")
			fmt.Println("  (Advanced: C = Equity)")
			fmt.Println("----------------------------------------")
			fmt.Print("Choice: ")
			scanner.Scan()
			accType = strings.ToUpper(strings.TrimSpace(scanner.Text()))

			rootName, err := logic.GetRootNameByType(accType)
			if err != nil {
				return err
			}

			// step 3: enter account name
			promptName := &survey.Input{
				Message: "Account Name:",
			}
			err = survey.AskOne(promptName, &accName, survey.WithValidator(validateAccountName))
			if err != nil {
				return fmt.Errorf("input cancelled: %w", err)
			}

			finalName = rootName + ":" + accName
			finalType = accType
		default:
			return fmt.Errorf("please enter y(yes) or n(no)")
		}

		// step 4: currency setting
		if finalCurrency == "" {
			defaultCurrency := viper.GetString("defaults.currency")
			fmt.Printf("Currency (press Enter for default: %s): ", defaultCurrency)
			scanner.Scan()
			accCurrency = strings.ToUpper(strings.TrimSpace(scanner.Text()))
			if accCurrency != "" {
				if err := checkCurrency(); err != nil {
					return err
				}
				finalCurrency = accCurrency
			} else {
				finalCurrency = defaultCurrency
			}
		} else {
			fmt.Printf("Currency (inherited from parent: %s, press Enter to keep or type to override): ", finalCurrency)
			scanner.Scan()
			accCurrency = strings.ToUpper(strings.TrimSpace(scanner.Text()))
			if accCurrency != "" {
				if err := checkCurrency(); err != nil {
					return err
				}
				finalCurrency = accCurrency
			}
		}

		// step 5: initial balance setting
		if finalType == "A" || finalType == "L" {
			fmt.Print("Initial Balance (press Enter for 0): ")
			scanner.Scan()
			balanceInput := strings.TrimSpace(scanner.Text())

			if balanceInput != "" {
				balanceFloat, err := processBalance(balanceInput)
				if err != nil {
					return err
				}
				amountInCents = int64(balanceFloat * CentsPerUnit)
			}
		}

		fmt.Print("Description (press enter to skip): ")
		scanner.Scan()
		accDesc = strings.TrimSpace(scanner.Text())

		// print put full informtaion
		fmt.Println("Confirm the following details:")
		displayAccountSummary(finalName, finalType, finalCurrency, amountInCents, accDesc)
		fmt.Print("Proceed? (y/n): ")
		scanner.Scan()
		confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))

		if confirm != "y" && confirm != "yes" {
			fmt.Println("Account creation cancelled.")
			return nil
		}

		newAccount, err := logic.CreateAccount(finalName, finalType, finalCurrency, accDesc, parentID)
		if err != nil {
			return fmt.Errorf("failed to create account: %w", err)
		}

		if amountInCents != 0 {
			err = logic.SetBalance(newAccount, amountInCents)
			if err != nil {
				return fmt.Errorf("failed to set balance: %w", err)
			}
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

func validateAccountName(val interface{}) error {
	name, ok := val.(string)
	if !ok {
		return fmt.Errorf("account name must be a string")
	}

	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("account name can't be empty")
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

func processBalance(balanceInput string) (float64, error) {
	balanceFloat, err := strconv.ParseFloat(balanceInput, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid input of Initial Balance")
	}

	if balanceFloat < 0 {
		return 0, fmt.Errorf("initial balance can't be negative")
	}

	if balanceFloat > 9223372036854775 {
		return 0, fmt.Errorf("balance amount too large")
	}
	return balanceFloat, nil
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
