package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/constants"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/hance08/kea/internal/utils"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type addFlags struct {
	Desc      string
	Amount    string
	From      string
	To        string
	Status    string
	Timestamp string
}

type addRunner struct {
	svc   *service.Service
	flags *addFlags
	cmd   *cobra.Command
}

func NewAddCmd(svc *service.Service) *cobra.Command {
	flags := &addFlags{}

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new transaction",
		Long: `Add a new transaction to your accounting system.

	This command allows you to record financial transactions using double-entry bookkeeping.
	You can use flags for quick entry or interactive mode for guided input.

	Examples:
	# Interactive mode (recommended for beginners)
	kea add

	# Quick mode with flags
	kea add --desc "Buy Coffee" --amount 150 --from "Assets:Cash" --to "Expenses:Food:Coffee"
	
	# With pending status (default is cleared)
	kea add --desc "Pending cost" --amount 500 --from "Assets:Bank" --to "Expenses:Shopping" --status pending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &addRunner{
				svc:   svc,
				flags: flags,
				cmd:   cmd,
			}
			return runner.Run()
		},
	}
	cmd.Flags().StringVarP(&flags.Desc, "desc", "d", "", "Transaction description")
	cmd.Flags().StringVarP(&flags.Amount, "amount", "a", "", "Transaction amount (e.g., 150 or 150.50)")
	cmd.Flags().StringVarP(&flags.From, "from", "f", "", "Source account (where money comes from)")
	cmd.Flags().StringVarP(&flags.To, "to", "t", "", "Destination account (where money goes to)")
	cmd.Flags().StringVarP(&flags.Status, "status", "s", "cleared", "Transaction status: pending or cleared")
	cmd.Flags().StringVar(&flags.Timestamp, "date", "", "Transaction date (YYYY-MM-DD), default is today")

	return cmd
}

func (r *addRunner) Run() error {
	var input service.TransactionDetail
	var err error

	// Check if using flag mode or interactive mode
	hasFlags := r.cmd.Flags().Changed("desc") || r.cmd.Flags().Changed("amount") ||
		r.cmd.Flags().Changed("from") || r.cmd.Flags().Changed("to")

	if hasFlags {
		// Flag mode: validate all required flags
		input, err = r.flagsMode()
	} else {
		// Interactive mode
		input, err = r.interactiveMode()
	}
	if err != nil {
		return err
	}

	// Display transaction summary
	if err := views.RenderTransactionDetail(&input, true); err != nil {
		return err
	}

	return nil
}

func (r *addRunner) flagsMode() (service.TransactionDetail, error) {

	// Flag mode: validate all required flags
	if r.flags.Amount == "" || r.flags.From == "" || r.flags.To == "" {
		return service.TransactionDetail{}, fmt.Errorf("when using flags, --amount, --from, and --to are all required")
	}

	if r.flags.Desc == "" {
		r.flags.Desc = "-"
	}

	// Parse amount
	amountCents, err := utils.ParseToCents(r.flags.Amount)
	if err != nil {
		return service.TransactionDetail{}, fmt.Errorf("invalid amount: %w", err)
	}

	// Parse status
	status := 1 // Default: cleared
	if strings.ToLower(r.flags.Status) == "pending" {
		status = 0
	}

	// Parse timestamp
	var timestamp int64
	if r.flags.Timestamp != "" {
		t, err := time.Parse("2006-01-02", r.flags.Timestamp)
		if err != nil {
			return service.TransactionDetail{}, fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
		}
		timestamp = t.Unix()
	} else {
		timestamp = time.Now().Unix()
	}

	input, err := r.svc.Transaction.CreateSimpleTransaction(
		r.flags.From,
		r.flags.To,
		amountCents,
		r.flags.Desc,
		timestamp,
		status,
	)

	if err != nil {
		return service.TransactionDetail{}, err
	}

	return input, nil
}

func (r *addRunner) interactiveMode() (service.TransactionDetail, error) {
	// Get all accounts
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return service.TransactionDetail{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Step 1: Select transaction type
	rawType, err := prompts.PromptTransactionType()
	if err != nil {
		return service.TransactionDetail{}, err
	}

	mode := r.determineMode(rawType)

	// Step 2: Get description (optional)
	description, err := prompts.PromptDescription("Transaction description (optional):", false)
	if err != nil {
		return service.TransactionDetail{}, err
	}
	if description == "" {
		description = "-"
	}

	// Step 3: Get amount
	amountStr, err := prompts.PromptAmount(
		"Amount:",
		"Enter the amount, no need currency symbol(e.g. 150 or 150.50)",
		prompts.ValidateAmountFormat(false),
	)
	if err != nil {
		return service.TransactionDetail{}, err
	}

	amountCents, _ := utils.ParseToCents(amountStr)

	uiConfigs := map[string]struct{ Src, Dst string }{
		constants.ModeExpense:  {"Payment Source:", "Expense Type:"},
		constants.ModeIncome:   {"Revenue Type:", "Deposit To:"},
		constants.ModeTransfer: {"From Account:", "To Account:"},
	}

	// Step 4 & 5: Select accounts based on mode
	rule, err := r.svc.Transaction.GetTransactionRule(mode)
	if err != nil {
		return service.TransactionDetail{}, err
	}

	uiConf, ok := uiConfigs[mode]
	if !ok {
		return service.TransactionDetail{}, fmt.Errorf("UI config missing for mode: %s", mode)
	}

	fromAccount, err := r.selectAccount(accounts, rule.SourceTypes, uiConf.Src, true)
	if err != nil {
		return service.TransactionDetail{}, err
	}

	toAccount, err := r.selectAccount(accounts, rule.DestTypes, uiConf.Src, mode != "expense")
	if err != nil {
		return service.TransactionDetail{}, err
	}

	// Step 6: Transaction status
	statusStr, err := prompts.PromptTransactionStatus("Cleared")
	if err != nil {
		return service.TransactionDetail{}, err
	}

	status := 1
	if statusStr == "Pending" {
		status = 0
	}

	// Step 7: Transaction date
	dateStr, err := prompts.PromptTransactionDate()
	if err != nil {
		return service.TransactionDetail{}, err
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return service.TransactionDetail{}, fmt.Errorf("invalid date format: %w", err)
	}
	timestamp := t.Unix()

	input, err := r.svc.Transaction.CreateSimpleTransaction(
		fromAccount,
		toAccount,
		amountCents,
		description,
		timestamp,
		status,
	)

	if err != nil {
		return service.TransactionDetail{}, err
	}

	return input, nil
}

// r.selectAccount filters accounts by type and displays them with optional balance
func (r *addRunner) selectAccount(accounts []*model.Account, allowedTypes []string, message string, showBalance bool) (string, error) {
	var balanceGetter func(int64) (string, error)
	if showBalance {
		balanceGetter = r.svc.Account.GetAccountBalanceFormatted
	}

	return prompts.PromptAccountSelection(accounts, allowedTypes, message, showBalance, balanceGetter)
}

func (r *addRunner) determineMode(rawInput string) string {
	if strings.Contains(rawInput, "Expense") {
		return constants.ModeExpense
	}
	if strings.Contains(rawInput, "Income") {
		return constants.ModeIncome
	}
	return constants.ModeTransfer
}
