package errhandler

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pterm/pterm"
)

func HandleError(err error) {
	if errors.Is(err, terminal.InterruptErr) || strings.Contains(err.Error(), "interrupt") {
		pterm.Warning.Println("Operation Cancelled")
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}
