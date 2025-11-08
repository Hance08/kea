/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	accName    string
	accParent  string
	accType    string
	accBalance int
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new account",
	Long: `Create a new account`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if accParent == "" && accType == "" {
			return fmt.Errorf("error : must enter at least one of --type or --parent flag")
		}
		if accParent != "" && accType != "" {
			return fmt.Errorf("error : --type and --parent flags can not use as the sametime")
		}

		var finalName string
		var finalType string
		// var parentID *int64

		if accParent != "" {
			fmt.Printf("verifying the parent account '%s'...\n", accParent)
			// TODO: call accounting.GetAccount(db, accParent) to get the parentAccount.ID and parentAccount.Type
			finalName = accParent + ":" + accName
			// finalType = parentAccount.Type
			// parentID = &parentAccount.ID
		} else {
			fmt.Printf("converting the type '%s'...\n", accType)
			// TODO: call accounting.GetRootNameByType(accType) e.g., "A" -> "Assets")
			rootName := strings.ToUpper(accType)
			finalName = rootName + ":" + accName
			finalType = accType
		}

		fmt.Printf("prepare for creating account in database :\n")
		fmt.Printf("  Name : %s\n", finalName)
		fmt.Printf("  Type : %s\n", finalType)
		// if parentID != nil {
		// 	 fmt.Printf("  parentID : %d\n", *parentID)
		// }

		// TODO: call accounting.CreateAccount(db, finalName, finalType, parentID, ...))

		if accBalance != 0 {
			if finalType != "A" && finalType != "L" && !(accParent != "") {
				// TODO: check the parent type 
				// return fmt.Errorf("error : only type A or type L account can set balance")
			}
			fmt.Printf("setting balance : %d...\n", accBalance)
			// TODO: call accounting.SetBalance(db, finalName, accBalance))
		}

		fmt.Println("--------------------")
		fmt.Println("Account is created successfully !")
		return nil
	},
}

func init() {
	accountCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&accName, "name", "n", "", "Account Name (e.g. 'Wallet' or 'Bank') (Required)")
	createCmd.Flags().StringVarP(&accParent, "parent", "p", "", "Parent Full Name (e.g. 'Assets:Bank')")
	createCmd.Flags().StringVarP(&accType, "type", "t", "", "Account Type (A,L,C,R,E) (Only use with top level accounts)")
	createCmd.Flags().IntVarP(&accBalance, "balance", "b", 0, "Setting Balance (e.g. 500000 represent 5000.00)")

	createCmd.MarkFlagRequired("name")
}
