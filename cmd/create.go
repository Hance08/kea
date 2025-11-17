/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	CentsPerUnit = 100
)

var (
	accName      string
	accParent    string
	accType      string
	accBalance   int
	accDesc      string
	accCurrency  string
	balanceInput string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account.",
	Long: `In the beginning of using this tool, you need to create new accounts.
You must create type A (Asset), L(Liabilities), E(Expenses), R(Revenue)
four basic accounts, e.g. create an Asset account called Bank,
command : kea account create -t A -n Bank -b 100000`,
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
				fmt.Printf("converting the type '%s'...\n", accType)
				rootName, err := logic.GetRootNameByType(accType)
				if err != nil {
					return err
				}

				finalName = rootName + ":" + accName
				finalType = accType

				if accCurrency != "" {
					finalCurrency = accCurrency
				} else {
					finalCurrency = viper.GetString("defaults.currency")
				}
			}

			fmt.Printf("prepare for creating account in database :\n")
			fmt.Printf("  Name     : %s\n", finalName)
			fmt.Printf("  Type     : %s\n", finalType)
			fmt.Printf("  Currency : %s\n", finalCurrency)

			newAccount, err := logic.CreateAccount(finalName, finalType, finalCurrency, accDesc, parentID)
			if err != nil {
				return err
			}

			if accBalance != 0 {
				balanceFloat := float64(accBalance)
				amountInCents := int64(math.Round(balanceFloat * 100))
				fmt.Printf("setting balance : %s (%d cents)...\n", finalName, amountInCents)

				err = logic.SetBalance(newAccount, amountInCents)
				if err != nil {
					return err
				}
			}

			fmt.Println("----------------------------------------")
			fmt.Println("Account is created successfully !")
			return nil
		}

		// Interaaction mode

		fmt.Println("\n Creating a new account")
		fmt.Println("----------------------------------------")

		// step 1: check if is subaccount
		fmt.Print("\n[1/5] Is this a subaccount? (y/n): ")
		scanner.Scan()
		isSubAccount := strings.ToLower(strings.TrimSpace(scanner.Text()))

		var finalName, finalType, finalCurrency string
		var parentID *int64

		switch isSubAccount {
		case "y", "yes":
			// step 2a: enter parent account name
			fmt.Print("[2/5] Parent account name (e.g. 'Assets:Bank'): ")
			scanner.Scan()
			accParent = strings.TrimSpace(scanner.Text())

			if accParent == "" {
				return fmt.Errorf("parent account name can't be empty")
			}
			// check parent account
			parentAccount, err := logic.GetAccountByName(accParent)
			if err != nil {
				return fmt.Errorf("parent account not found: %w", err)
			}

			// step 3: enter account name
			fmt.Print("[3/5] Account Name (e.g. 'Savings'): ")
			scanner.Scan()
			accName = strings.TrimSpace(scanner.Text())

			if accName == "" {
				return fmt.Errorf("account name can't be empty")
			}

			finalName = accParent + ":" + accName
			finalType = parentAccount.Type
			finalCurrency = parentAccount.Currency
			parentID = &parentAccount.ID
		case "n", "no":
			// step 2b: enter account type
			fmt.Print("[2/5] Account type\n")
			fmt.Print("      A = Assets    L = Liabilities    C = Equity\n")
			fmt.Print("      R = Revenue   E = Expenses\n")
			fmt.Print("      Choice: ")
			scanner.Scan()
			accType = strings.ToUpper(strings.TrimSpace(scanner.Text()))

			rootName, err := logic.GetRootNameByType(accType)
			if err != nil {
				return err
			}

			// step 3: enter account name
			fmt.Print("[3/5] Account Name (e.g. 'Bank'): ")
			scanner.Scan()
			accName = strings.TrimSpace(scanner.Text())

			if accName == "" {
				return fmt.Errorf("account name can't be empty")
			}

			finalName = rootName + ":" + accName
			finalType = accType
			finalCurrency = defaultCurrency
		default:
			return fmt.Errorf("please enter y(yes) or n(no)")
		}

		// step 4: currency setting
		fmt.Printf("[4/5] Currency (press Enter for default: %s): ", finalCurrency)
		scanner.Scan()
		accCurrency = strings.ToUpper(strings.TrimSpace(scanner.Text()))
		if accCurrency != "" {
			finalCurrency = accCurrency
		}

		// step 5: initial balance setting
		fmt.Print("[5/5] Initial Balance (press Enter for 0): ")
		scanner.Scan()
		balanceInput := strings.TrimSpace(scanner.Text())

		var amountInCents int64 = 0
		if balanceInput != "" {
			balanceFloat, err := strconv.ParseFloat(balanceInput, 64)
			if err != nil {
				return fmt.Errorf("invalid balance amount: %w", err)
			}
			amountInCents = int64(balanceFloat * 100)
		}

		// print put full informtaion
		fmt.Println("\n----------------------------------------")
		fmt.Println("Confirm the following details:")
		fmt.Printf("  Full Name : %s\n", finalName)
		fmt.Printf("  Type      : %s\n", finalType)
		fmt.Printf("  Currency  : %s\n", finalCurrency)
		if amountInCents != 0 {
			fmt.Printf("  Balance   : %.2f\n", float64(amountInCents)/100)
		} else {
			fmt.Printf("  Balance   : 0.00\n")
		}
		fmt.Println("----------------------------------------")
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

		fmt.Println("----------------------------------------")
		fmt.Println("Account is created successfully !")
		return nil
	},
}

func init() {
	accountCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&accName, "name", "n", "", "Account Name (e.g. 'Wallet' or 'Bank')")
	createCmd.Flags().StringVarP(&accParent, "parent", "p", "", "Parent Full Name (e.g. 'Assets:Bank')")
	createCmd.Flags().StringVarP(&accType, "type", "t", "", "Account Type (A,L,C,R,E) (Only use with top level accounts)")
	createCmd.Flags().IntVarP(&accBalance, "balance", "b", 0, "Setting Balance (e.g. 5000 or 5000.00)")
	createCmd.Flags().StringVar(&accCurrency, "currency", "", "Currency Code (e.g. TWD, USD, EUR). If not specified, use parent's currency or default from config")
	createCmd.Flags().StringVarP(&accDesc, "description", "d", "", "Account description")
}

func checkAccountName() error {
	if accName == "" {
		return fmt.Errorf("account name can't be empty")
	}

	if strings.Contains(accName, ":") {
		return fmt.Errorf("account name cannot contain ':' character")
	}

	reservedNames := []string{"Assets", "Liabilities", "Equity", "Revenue", "Expenses"}
	for _, reserved := range reservedNames {
		if accName == reserved {
			return fmt.Errorf("'%s' is a reserved root account name", accName)
		}
	}

	if len(accName) > 100 {
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
