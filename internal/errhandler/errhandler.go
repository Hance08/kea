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
	keyInterrupt(err)
	invalidCommand(err)
	invalidFlag(err)

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

func keyInterrupt(err error) {
	if errors.Is(err, terminal.InterruptErr) || strings.Contains(err.Error(), "interrupt") {
		pterm.Warning.Println("Operation Cancelled")
		os.Exit(0)
	}
}

func invalidCommand(err error) {
	if strings.Contains(err.Error(), "unknown command") {
		cleanMsg := strings.TrimPrefix(err.Error(), "unknown command ")

		var invalidCommand string
		_, scanErr := fmt.Sscanf(cleanMsg, "%q", &invalidCommand)
		if scanErr == nil {
			pterm.Error.Printf("Unknown command: %s | Run 'kea --help' for more information\n", invalidCommand)
		}

		os.Exit(1)
	}
}

func invalidFlag(err error) {
	if strings.Contains(err.Error(), "unknown") && strings.Contains(err.Error(), "flag") {
		cleanMsg := strings.TrimPrefix(err.Error(), "unknown flag: ")
		cleanMsg = strings.TrimPrefix(cleanMsg, "unknown shorthand flag: ")

		if cleanMsg != "" {
			pterm.Error.Printf("Unknown flag: %s\n", cleanMsg)
		} else {
			pterm.Error.Println(err.Error())
		}

		os.Exit(1)
	}
}
