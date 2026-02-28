package ui

import (
	"fmt"

	"github.com/pterm/pterm"
)

func PrintL1Title(format string, a ...interface{}) {
	style := pterm.NewStyle(pterm.BgCyan, pterm.FgBlack, pterm.Bold)

	text := fmt.Sprintf(format, a...)

	paddedText := fmt.Sprintf(" %s   ", text)

	style.Println(paddedText)
}

func PrintL2Title(format string, a ...interface{}) {
	style := pterm.NewStyle(pterm.FgCyan, pterm.Bold)

	text := fmt.Sprintf(format, a...)

	paddedText := fmt.Sprintf("# %s   ", text)

	style.Println(paddedText)
}
