package reconcileui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hance08/kea/internal/model"
	"github.com/hance08/kea/internal/utils"
)

type listItem struct {
	entry   *model.ReconcileEntry
	checked bool
}

// Model is the bubbletea model for the reconciliation TUI.
// After tea.Program.Run() returns, inspect Cancelled() and SelectedIDs()
// to determine the outcome.
type Model struct {
	accountName      string
	statementBalance int64
	items            []listItem
	cursor           int
	confirmPending   bool // waiting for y/n after Enter with non-zero diff
	cancelled        bool
	done             bool
	keys             keyMap
}

// NewModel constructs the initial reconciliation model.
func NewModel(accountName string, statementBalance int64, entries []*model.ReconcileEntry) Model {
	items := make([]listItem, len(entries))
	for i, e := range entries {
		items[i] = listItem{entry: e}
	}
	return Model{
		accountName:      accountName,
		statementBalance: statementBalance,
		items:            items,
		keys:             defaultKeyMap(),
	}
}

// Cancelled reports whether the user quit without confirming.
func (m Model) Cancelled() bool { return m.cancelled }

// SelectedIDs returns the transaction IDs the user checked off.
func (m Model) SelectedIDs() []int64 {
	var ids []int64
	for _, it := range m.items {
		if it.checked {
			ids = append(ids, it.entry.ID)
		}
	}
	return ids
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmPending {
			switch {
			case key.Matches(msg, m.keys.Yes):
				m.done = true
				return m, tea.Quit
			case key.Matches(msg, m.keys.No):
				m.confirmPending = false
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Toggle):
			if len(m.items) > 0 {
				m.items[m.cursor].checked = !m.items[m.cursor].checked
			}
		case key.Matches(msg, m.keys.Confirm):
			if len(m.items) == 0 {
				return m, nil
			}
			if m.difference() != 0 {
				m.confirmPending = true
			} else {
				m.done = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.items) == 0 {
		return "No unreconciled transactions for this account.\n\nPress q to quit.\n"
	}

	var sb strings.Builder

	// ── Header ──────────────────────────────────────────
	diff := m.difference()
	var badge string
	if diff == 0 {
		badge = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("22")).
			Foreground(lipgloss.Color("15")).
			Render("BALANCED")
	} else {
		badge = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("58")).
			Foreground(lipgloss.Color("15")).
			Render(fmt.Sprintf("OFF BY $%s", utils.FormatAmount(abs(diff))))
	}

	accountStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("78"))
	sb.WriteString(fmt.Sprintf("%-40s %s\n\n", accountStyle.Render(m.accountName), badge))

	stmtStr := utils.FormatAmount(m.statementBalance)
	sb.WriteString(fmt.Sprintf("STATEMENT: $%s · %d UNRECONCILED\n", stmtStr, len(m.items)))
	sb.WriteString(strings.Repeat("─", 52) + "\n")

	// ── Transaction list ────────────────────────────────
	for i, it := range m.items {
		cursorMark := "  "
		if i == m.cursor {
			cursorMark = "▶ "
		}

		checkbox := "[ ]"
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		if it.checked {
			checkbox = "[✓]"
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
		}

		date := time.Unix(it.entry.Timestamp, 0).Format("Jan 02")
		amt := fmt.Sprintf("$%s", utils.FormatAmount(it.entry.Amount))
		line := fmt.Sprintf("%s%s %s  %-26s %10s", cursorMark, checkbox, date, truncate(it.entry.Description, 26), amt)

		if i == m.cursor {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("255")).
				Render(line)
		} else {
			line = rowStyle.Render(line)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString(strings.Repeat("─", 52) + "\n")

	// ── Footer balance ───────────────────────────────────
	clearedStr := utils.FormatAmount(m.clearedBalance())
	diffStr := utils.FormatAmount(abs(diff))
	remainingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	if diff == 0 {
		remainingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	}
	sb.WriteString(fmt.Sprintf(
		"Cleared $%s · %s\n",
		clearedStr,
		remainingStyle.Render(fmt.Sprintf("Remaining $%s", diffStr)),
	))

	// ── Prompt or hint ───────────────────────────────────
	if m.confirmPending {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		sb.WriteString(fmt.Sprintf("\n%s\n",
			warnStyle.Render(fmt.Sprintf("You're off by $%s. Confirm anyway? (y/n)", diffStr)),
		))
	} else {
		sb.WriteString("\nspace toggle · enter finish · ↑↓ navigate · q quit\n")
	}

	return sb.String()
}

// ── helpers ─────────────────────────────────────────────

func (m Model) clearedBalance() int64 {
	var total int64
	for _, it := range m.items {
		if it.checked {
			total += it.entry.Amount
		}
	}
	return total
}

func (m Model) difference() int64 {
	return m.statementBalance - m.clearedBalance()
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
