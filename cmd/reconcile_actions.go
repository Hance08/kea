package cmd

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
	"github.com/hance08/kea/ui/prompts"
	reconcileui "github.com/hance08/kea/ui/reconcile"
	"github.com/hance08/kea/ui/views"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func (r *reconcileRunner) Run(cmd *cobra.Command, args []string) error {
	accountName := args[0]

	// Resolve account.
	acc, err := r.accSvc.GetAccountByName(accountName)
	if err != nil {
		return fmt.Errorf("account %q not found: %w", accountName, err)
	}

	nonInteractive := cmd.Flags().Changed("balance") && cmd.Flags().Changed("ids")

	if nonInteractive {
		return r.runNonInteractive(acc)
	}
	return r.runInteractive(acc)
}

// ── Non-interactive (agent / script) mode ────────────────────────────────────

func (r *reconcileRunner) runNonInteractive(acc *model.Account) error {
	statementBalance, err := utils.ParseAmount(r.flags.Balance)
	if err != nil {
		return fmt.Errorf("invalid --balance value %q: %w", r.flags.Balance, err)
	}

	txIDs, err := parseIDs(r.flags.IDs)
	if err != nil {
		return fmt.Errorf("invalid --ids value %q: %w", r.flags.IDs, err)
	}

	diff, err := r.txSvc.ReconcileTransactions(acc.ID, statementBalance, txIDs)
	if err != nil {
		return err
	}

	if diff != 0 && !r.flags.Force {
		return fmt.Errorf(
			"balance mismatch: off by $%s — use --force to reconcile anyway",
			utils.FormatAmount(abs64(diff)),
		)
	}

	if r.flags.JSON {
		return writeReconcileJSON(acc.Name, len(txIDs), diff)
	}
	pterm.Success.Printf(
		"Reconciled %d transaction(s) on %q. Difference: $%s\n",
		len(txIDs), acc.Name, utils.FormatAmount(abs64(diff)),
	)
	return nil
}

// ── Interactive mode ──────────────────────────────────────────────────────────

func (r *reconcileRunner) runInteractive(acc *model.Account) error {
	// 1. Prompt for statement balance.
	balanceStr, err := prompts.PromptAmount(
		"Statement ending balance:",
		"Enter the closing balance from your bank statement (e.g. 2450.00)",
		prompts.ValidateAmountFormat(false),
	)
	if err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}

	statementBalance, err := utils.ParseAmount(balanceStr)
	if err != nil {
		return fmt.Errorf("invalid balance: %w", err)
	}

	// 2. Load unreconciled entries.
	entries, err := r.txSvc.GetUnreconciledByAccount(acc.ID)
	if err != nil {
		return err
	}

	// 3. Run bubbletea TUI.
	m := reconcileui.NewModel(acc.Name, statementBalance, entries)
	prog := tea.NewProgram(m)
	finalRaw, err := prog.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	finalModel := finalRaw.(reconcileui.Model)

	if finalModel.Cancelled() {
		pterm.Info.Println("Reconciliation cancelled.")
		return nil
	}

	selectedIDs := finalModel.SelectedIDs()
	if len(selectedIDs) == 0 {
		pterm.Info.Println("No transactions selected — nothing reconciled.")
		return nil
	}

	// 4. Persist.
	diff, err := r.txSvc.ReconcileTransactions(acc.ID, statementBalance, selectedIDs)
	if err != nil {
		return err
	}

	if r.flags.JSON {
		return writeReconcileJSON(acc.Name, len(selectedIDs), diff)
	}

	if diff == 0 {
		pterm.Success.Printf("Reconciled %d transaction(s) on %q — balanced!\n", len(selectedIDs), acc.Name)
	} else {
		pterm.Warning.Printf(
			"Reconciled %d transaction(s) on %q with a remaining difference of $%s.\n",
			len(selectedIDs), acc.Name, utils.FormatAmount(abs64(diff)),
		)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseIDs(s string) ([]int64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("empty ID list")
	}
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func writeReconcileJSON(account string, count int, diff int64) error {
	return views.WriteJSON(map[string]any{
		"account":          account,
		"reconciled_count": count,
		"difference":       diff,
	})
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
