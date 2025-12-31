package prompts

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/utils"
)

func ValidateAmountFormat(allowEmpty bool) func(string) error {
	return func(input string) error {
		if strings.TrimSpace(input) == "" {
			if allowEmpty {
				return nil
			}
			return fmt.Errorf("amount is required")
		}
		_, err := utils.ParseToCents(input)
		if err != nil {
			return fmt.Errorf("%v", err)
		}
		return nil
	}

}

func ValidateDateFormat(allowEmpty bool) func(string) error {
	return func(input string) error {
		if strings.TrimSpace(input) == "" {
			if allowEmpty {
				return nil
			}
			return fmt.Errorf("date is required")
		}

		_, err := time.Parse("2006-01-02", input)
		if err != nil {
			return fmt.Errorf("invalid format (YYYY-MM-DD)")
		}
		return nil
	}
}
