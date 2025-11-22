/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// accountCmd represents the account command
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "It can create, edit, delete account and show the list of all accounts.",
	Long: `It can create, edit, delete account and show the list of all accounts.`,
}

func init() {
	rootCmd.AddCommand(accountCmd)
}
