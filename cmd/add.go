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

type AddCommandRunner struct {
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

		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &AddCommandRunner{
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

func (r *AddCommandRunner) Run() error {
	var input service.TransactionInput
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

	// Create transaction
	txID, err := r.svc.Transaction.CreateTransaction(input)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Success message
	pterm.Success.Printf("Transaction created successfully! (ID: %d)\n", txID)

	// Display transaction summary
	if err := views.RenderTransactionSummary(input); err != nil {
		return err
	}

	return nil
}

// TODO: 拆分邏輯，因為未來會需要更多的add方法(目前 新增消費、新增收入、新增轉帳)
func (r *AddCommandRunner) flagsMode() (service.TransactionInput, error) {
	var input service.TransactionInput

	// Flag mode: validate all required flags
	if r.flags.Amount == "" || r.flags.From == "" || r.flags.To == "" {
		return input, fmt.Errorf("when using flags, --amount, --from, and --to are all required")
	}

	if r.flags.Desc == "" {
		r.flags.Desc = "-"
	}

	// Parse amount
	amountCents, err := utils.ParseToCents(r.flags.Amount)
	if err != nil {
		return input, fmt.Errorf("invalid amount: %w", err)
	}

	// Validate accounts exist
	if exists, err := r.svc.Account.CheckAccountExists(r.flags.From); err != nil || !exists {
		return input, fmt.Errorf("source account '%s' does not exist", r.flags.From)
	}
	if exists, err := r.svc.Account.CheckAccountExists(r.flags.To); err != nil || !exists {
		return input, fmt.Errorf("destination account '%s' does not exist", r.flags.To)
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
			return input, fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
		}
		timestamp = t.Unix()
	} else {
		timestamp = time.Now().Unix()
	}

	// Build transaction input
	input = service.TransactionInput{
		Timestamp:   timestamp,
		Description: r.flags.Desc,
		Status:      status,
		Splits: []service.TransactionSplitInput{
			{
				AccountName: r.flags.To,
				Amount:      amountCents,
				Memo:        "",
			},
			{
				AccountName: r.flags.From,
				Amount:      -amountCents,
				Memo:        "",
			},
		},
	}

	return input, nil
}

func (r *AddCommandRunner) interactiveMode() (service.TransactionInput, error) {
	var input service.TransactionInput

	// Get all accounts
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return input, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Step 1: Select transaction type
	transactionType, err := prompts.PromptTransactionType()
	if err != nil {
		return input, err
	}

	// Determine transaction mode
	var mode string
	if strings.Contains(transactionType, "Expense") {
		mode = "expense"
	} else if strings.Contains(transactionType, "Income") {
		mode = "income"
	} else {
		mode = "transfer"
	}

	// Step 2: Get description (optional)
	description, err := prompts.PromptDescription("Transaction description (optional):", false)
	if err != nil {
		return input, err
	}
	if description == "" {
		description = "-"
	}

	// Step 3: Get amount
	amountStr, err := prompts.PromptAmount(
		"Amount:",
		"Enter the amount, no need currency symbol(e.g. 150 or 150.50)",
		nil, // No custom validator, will validate after
	)
	if err != nil {
		return input, err
	}
	if amountStr == "" {
		return input, fmt.Errorf("amount is required")
	}

	amountCents, err := utils.ParseToCents(amountStr)
	if err != nil {
		return input, fmt.Errorf("invalid amount format: %w", err)
	}

	// Step 4 & 5: Select accounts based on mode
	var fromAccount, toAccount string

	switch mode {
	case "expense":
		// Record Expense: From Assets/Liabilities, To Expenses
		fromAccount, err = r.selectAccount(accounts, []string{"A", "L"}, "Payment Source:", true)
		if err != nil {
			return input, err
		}

		toAccount, err = r.selectAccount(accounts, []string{"E"}, "Expense Type:", false)
		if err != nil {
			return input, err
		}

	case "income":
		// Record Income: From Revenue, To Assets
		fromAccount, err = r.selectAccount(accounts, []string{"R"}, "Revenue Type:", false)
		if err != nil {
			return input, err
		}

		toAccount, err = r.selectAccount(accounts, []string{"A"}, "Deposit To:", true)
		if err != nil {
			return input, err
		}

	case "transfer":
		// Transfer: From Assets/Liabilities, To Assets/Liabilities
		fromAccount, err = r.selectAccount(accounts, []string{"A", "L"}, "From Account:", true)
		if err != nil {
			return input, err
		}

		toAccount, err = r.selectAccount(accounts, []string{"A", "L"}, "To Account:", true)
		if err != nil {
			return input, err
		}

		// Validate: cannot transfer to the same account
		if fromAccount == toAccount {
			return input, fmt.Errorf("cannot transfer to the same account")
		}
	}

	// Step 6: Transaction status
	statusStr, err := prompts.PromptTransactionStatus("Cleared")
	if err != nil {
		return input, err
	}

	status := 1
	if statusStr == "Pending" {
		status = 0
	}

	// Step 7: Transaction date
	dateStr, err := prompts.PromptTransactionDate()
	if err != nil {
		return input, err
	}

	timestamp, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return input, fmt.Errorf("invalid date format: %w", err)
	}

	// Build transaction input
	input = service.TransactionInput{
		Timestamp:   timestamp.Unix(),
		Description: description,
		Status:      status,
		Splits: []service.TransactionSplitInput{
			{
				AccountName: toAccount,
				Amount:      amountCents,
				Memo:        "",
			},
			{
				AccountName: fromAccount,
				Amount:      -amountCents,
				Memo:        "",
			},
		},
	}

	return input, nil
}

// r.selectAccount filters accounts by type and displays them with optional balance
func (r *AddCommandRunner) selectAccount(accounts []*store.Account, allowedTypes []string, message string, showBalance bool) (string, error) {
	var balanceGetter func(int64) (string, error)
	if showBalance {
		balanceGetter = r.svc.Account.GetAccountBalanceFormatted
	}

	return prompts.PromptAccountSelection(accounts, allowedTypes, message, showBalance, balanceGetter)
}
