package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/constant"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/service"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/ui/views"
	"github.com/hance08/kea/internal/utils"
	"github.com/spf13/cobra"
)

var modeUIConfigs = map[string]struct{ Src, Dst string }{
	constant.ModeExpense:  {"Payment Source:", "Expense Type:"},
	constant.ModeIncome:   {"Revenue Type:", "Deposit To:"},
	constant.ModeTransfer: {"From Account:", "To Account:"},
}

type addFlags struct {
	Description string
	Amount      string
	From        string
	To          string
	Status      string
	Timestamp   string
}

type addRunner struct {
	svc   *service.Service
	flags *addFlags
	cmd   *cobra.Command
}

type addTransactionInput struct {
	FromAccountID string
	ToAccountID   string
	AmountCents   int64
	Description   string
	Timestamp     int64
	Status        model.TransactionStatus
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
	kea add --description "Buy Coffee" --amount 150 --from "Assets:Cash" --to "Expenses:Food:Coffee"
	
	# With pending status (default is cleared)
	kea add --description "Pending cost" --amount 500 --from "Assets:Bank" --to "Expenses:Shopping" --status pending`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := &addRunner{
				svc:   svc,
				flags: flags,
				cmd:   cmd,
			}
			return runner.Run()
		},
	}
	cmd.Flags().StringVarP(&flags.Description, "desc", "d", "", "Transaction description")
	cmd.Flags().StringVarP(&flags.Amount, "amount", "a", "", "Transaction amount (e.g., 150 or 150.50)")
	cmd.Flags().StringVarP(&flags.From, "from", "f", "", "Source account (where money comes from)")
	cmd.Flags().StringVarP(&flags.To, "to", "t", "", "Destination account (where money goes to)")
	cmd.Flags().StringVarP(&flags.Status, "status", "s", "cleared", "Transaction status: pending or cleared")
	cmd.Flags().StringVar(&flags.Timestamp, "date", "", "Transaction date (YYYY-MM-DD), default is today")

	return cmd
}

func (r *addRunner) Run() error {
	var input addTransactionInput
	var err error

	// Check if using flag mode or interactive mode
	hasFlags := r.cmd.Flags().Changed("desc") || r.cmd.Flags().Changed("amount") ||
		r.cmd.Flags().Changed("from") || r.cmd.Flags().Changed("to")

	if hasFlags {
		// Flag mode: validate all required flags
		input, err = r.runFromFlags()
	} else {
		// Interactive mode
		input, err = r.runInteractive()
	}
	if err != nil {
		return err
	}

	result, err := r.svc.Transaction.CreateSimpleTransaction(
		input.FromAccountID,
		input.ToAccountID,
		input.AmountCents,
		input.Description,
		input.Timestamp,
		input.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	// Display transaction summary
	if err := views.RenderTransactionDetail(&result, true); err != nil {
		return err
	}

	return nil
}

func (r *addRunner) runFromFlags() (addTransactionInput, error) {

	// Flag mode: validate all required flags
	if r.flags.Amount == "" || r.flags.From == "" || r.flags.To == "" {
		return addTransactionInput{}, fmt.Errorf("when using flags, --amount, --from, and --to are all required")
	}

	description := r.flags.Description
	if description == "" {
		description = "-"
	}

	// Parse amount
	amountCents, err := utils.ParseToCents(r.flags.Amount)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("invalid amount: %w", err)
	}

	// Parse status
	status := model.StatusCleared
	if strings.ToLower(r.flags.Status) == "pending" {
		status = model.StatusPending
	}

	// Parse timestamp
	timestamp, err := r.parseDate(r.flags.Timestamp)
	if err != nil {
		return addTransactionInput{}, err
	}

	return addTransactionInput{
		FromAccountID: r.flags.From,
		ToAccountID:   r.flags.To,
		AmountCents:   amountCents,
		Description:   description,
		Timestamp:     timestamp,
		Status:        status,
	}, nil
}

func (r *addRunner) runInteractive() (addTransactionInput, error) {
	// Get all accounts
	accounts, err := r.svc.Account.GetAllAccounts()
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Step 1: Select transaction type
	rawType, err := prompts.PromptTransactionType()
	if err != nil {
		return addTransactionInput{}, err
	}

	mode := r.determineMode(rawType)

	// Step 2: Get description (optional)
	description, err := prompts.PromptDescription("Transaction description (optional):", false)
	if err != nil {
		return addTransactionInput{}, err
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
		return addTransactionInput{}, err
	}

	amountCents, err := utils.ParseToCents(amountStr)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("invalid amount: %w", err)
	}

	// Step 4 & 5: Select accounts based on mode
	rule, err := r.svc.Transaction.GetTransactionRule(mode)
	if err != nil {
		return addTransactionInput{}, err
	}

	uiConf, ok := modeUIConfigs[mode]
	if !ok {
		return addTransactionInput{}, fmt.Errorf("UI config missing for mode: %s", mode)
	}

	fromAccount, err := r.selectAccount(accounts, rule.SourceTypes, uiConf.Src, true)
	if err != nil {
		return addTransactionInput{}, err
	}

	toAccount, err := r.selectAccount(accounts, rule.DestTypes, uiConf.Dst, mode != constant.ModeExpense)
	if err != nil {
		return addTransactionInput{}, err
	}

	// Step 6: Transaction status
	statusStr, err := prompts.PromptTransactionStatus("Cleared")
	if err != nil {
		return addTransactionInput{}, err
	}

	status := model.StatusCleared
	if statusStr == "Pending" {
		status = model.StatusPending
	}

	// Step 7: Transaction date
	dateStr, err := prompts.PromptTransactionDate()
	if err != nil {
		return addTransactionInput{}, err
	}

	timestamp, err := r.parseDate(dateStr)
	if err != nil {
		return addTransactionInput{}, err
	}

	return addTransactionInput{
		FromAccountID: fromAccount,
		ToAccountID:   toAccount,
		AmountCents:   amountCents,
		Description:   description,
		Timestamp:     timestamp,
		Status:        status,
	}, nil
}

func (r *addRunner) selectAccount(accounts []*model.Account, allowedTypes []string, message string, showBalance bool) (string, error) {
	var balanceGetter func(int64) (string, error)
	if showBalance {
		balanceGetter = r.svc.Account.GetAccountBalanceFormatted
	}

	return prompts.PromptAccountSelection(accounts, allowedTypes, message, showBalance, balanceGetter)
}

func (r *addRunner) determineMode(rawInput string) string {
	if strings.Contains(rawInput, "Expense") {
		return constant.ModeExpense
	}
	if strings.Contains(rawInput, "Income") {
		return constant.ModeIncome
	}
	return constant.ModeTransfer
}

func (r *addRunner) parseDate(dateStr string) (int64, error) {
	if dateStr == "" {
		return time.Now().Unix(), nil
	}
	t, err := time.Parse(constant.DateFormat, dateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid date format, use %s: %w", constant.DateFormat, err)
	}
	return t.Unix(), nil
}
