package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/ui/prompts"
	"github.com/hance08/kea/internal/utils"
)

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
	amountCents, err := utils.ParseAmount(r.flags.Amount)
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
	accounts, err := r.accSvc.GetAllAccounts()
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

	amountCents, err := utils.ParseAmount(amountStr)
	if err != nil {
		return addTransactionInput{}, fmt.Errorf("invalid amount: %w", err)
	}

	// Step 4 & 5: Select accounts based on mode
	rule, err := r.txSvc.GetTransactionRule(mode)
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

	toAccount, err := r.selectAccount(accounts, rule.DestTypes, uiConf.Dst, mode != model.TxTypeExpense)
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
		balanceGetter = r.accSvc.GetAccountBalanceFormatted
	}

	return prompts.PromptAccountSelection(accounts, allowedTypes, message, showBalance, balanceGetter)
}

func (r *addRunner) determineMode(rawInput string) model.TransactionType {
	if strings.Contains(rawInput, "Expense") {
		return model.TxTypeExpense
	}
	if strings.Contains(rawInput, "Income") {
		return model.TxTypeIncome
	}
	return model.TxTypeTransfer
}

func (r *addRunner) parseDate(dateStr string) (int64, error) {
	if dateStr == "" {
		return time.Now().Unix(), nil
	}
	t, err := time.Parse(model.DateFormat, dateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid date format, use %s: %w", model.DateFormat, err)
	}
	return t.Unix(), nil
}
