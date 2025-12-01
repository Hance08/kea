package cmd

import "github.com/pterm/pterm"

// printSeparator prints a green separator line to the console.
// It ensures consistency in visual separation across the application.
func printSeparator() {
	pterm.Println(pterm.Green("----------------------------------------"))
}
