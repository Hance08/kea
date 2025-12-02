package ui

import "github.com/pterm/pterm"

// Separator prints a green separator line to the console.
func Separator() {
	pterm.Println(pterm.Green("---------------------------------------------------------"))
}
