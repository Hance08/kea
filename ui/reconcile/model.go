// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

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
	rw "github.com/mattn/go-runewidth"
)

// overheadLines is the number of non-item lines in the view:
// account+badge (1) · blank (1) · statement (1) · last-reconciled (1) ·
// top-sep (1) · bottom-sep (1) · cleared (1) · blank (1) · hint (1) = 9
const overheadLines = 9

// Column widths in terminal display columns (accounts for double-wide CJK chars).
const (
	colOffset = 40 // other-side account
	colDesc   = 12 // description
	colAmt    = 12 // amount, right-aligned
)

// sepWidth is the total display width of separator lines.
// cursor(2) + checkbox(3) + sp(1) + date(8) + sp(2) +
// offset(40) + sp(2) + desc(12) + sp(2) + amt(12) = 84
const sepWidth = 84

type listItem struct {
	entry   *model.ReconcileEntry
	checked bool
}

// Model is the bubbletea model for the reconciliation TUI.
// After tea.Program.Run() returns, inspect Cancelled() and SelectedIDs()
// to determine the outcome.
type Model struct {
	accountName           string
	statementBalance      int64
	lastReconciledBalance int64
	items                 []listItem
	cursor                int
	viewportOffset        int  // index of the first visible item
	height                int  // terminal height (0 = unknown, show all)
	confirmPending        bool // waiting for y/n after Enter with non-zero diff
	cancelled             bool
	done                  bool
	keys                  keyMap
}

// NewModel constructs the initial reconciliation model.
func NewModel(accountName string, statementBalance int64, lastReconciledBalance int64, entries []*model.ReconcileEntry) Model {
	items := make([]listItem, len(entries))
	for i, e := range entries {
		items[i] = listItem{entry: e}
	}
	return Model{
		accountName:           accountName,
		statementBalance:      statementBalance,
		lastReconciledBalance: lastReconciledBalance,
		items:                 items,
		keys:                  defaultKeyMap(),
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

// visibleCount returns how many item rows fit in the current terminal height.
// Returns len(items) when height is unknown (0) so nothing is clipped.
func (m Model) visibleCount() int {
	if m.height == 0 {
		return len(m.items)
	}
	n := m.height - overheadLines
	if n < 1 {
		n = 1
	}
	return n
}

// clampViewport adjusts viewportOffset so the cursor row is always visible.
func clampViewport(cursor, viewportOffset, visibleCount, totalItems int) int {
	if cursor < viewportOffset {
		viewportOffset = cursor
	}
	if cursor >= viewportOffset+visibleCount {
		viewportOffset = cursor - visibleCount + 1
	}
	maxOffset := totalItems - visibleCount
	if maxOffset < 0 {
		maxOffset = 0
	}
	if viewportOffset > maxOffset {
		viewportOffset = maxOffset
	}
	if viewportOffset < 0 {
		viewportOffset = 0
	}
	return viewportOffset
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.viewportOffset = clampViewport(m.cursor, m.viewportOffset, m.visibleCount(), len(m.items))
		return m, nil

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
				m.viewportOffset = clampViewport(m.cursor, m.viewportOffset, m.visibleCount(), len(m.items))
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.viewportOffset = clampViewport(m.cursor, m.viewportOffset, m.visibleCount(), len(m.items))
			}
		case key.Matches(msg, m.keys.Toggle):
			if len(m.items) > 0 {
				m.items[m.cursor].checked = !m.items[m.cursor].checked
			}
		case key.Matches(msg, m.keys.SelectAll):
			// If every item is already checked, uncheck all; otherwise check all.
			allChecked := true
			for _, it := range m.items {
				if !it.checked {
					allChecked = false
					break
				}
			}
			for i := range m.items {
				m.items[i].checked = !allChecked
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
	} else if diff > 0 {
		badge = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("58")).
			Foreground(lipgloss.Color("15")).
			Render(fmt.Sprintf("SHORT $%s", utils.FormatAmount(diff)))
	} else {
		badge = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("130")).
			Foreground(lipgloss.Color("15")).
			Render(fmt.Sprintf("OVER $%s", utils.FormatAmount(abs(diff))))
	}

	accountStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("78"))
	sb.WriteString(fmt.Sprintf("%-40s %s\n\n", accountStyle.Render(m.accountName), badge))

	stmtStr := utils.FormatAmount(m.statementBalance)
	lastStr := utils.FormatAmount(m.lastReconciledBalance)
	sb.WriteString(fmt.Sprintf("STATEMENT: $%s · %d UNRECONCILED\n", stmtStr, len(m.items)))
	sb.WriteString(fmt.Sprintf("LAST RECONCILED: $%s\n", lastStr))

	// ── Viewport calculation ─────────────────────────────
	vis := m.visibleCount()
	start := m.viewportOffset
	end := start + vis
	if end > len(m.items) {
		end = len(m.items)
	}
	above := start
	below := len(m.items) - end

	// ── Top separator (with scroll-up indicator) ─────────
	if above > 0 {
		indicator := fmt.Sprintf("↑ %d more  ", above)
		sb.WriteString(indicator + strings.Repeat("─", sepWidth-len(indicator)) + "\n")
	} else {
		sb.WriteString(strings.Repeat("─", sepWidth) + "\n")
	}

	// ── Transaction list ─────────────────────────────────
	// Column order: date · account · offset account · description · amount
	for i := start; i < end; i++ {
		it := m.items[i]

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

		date := time.Unix(it.entry.Timestamp, 0).Format("06-01-02")
		amt := fmt.Sprintf("%*s", colAmt, "$"+utils.FormatAmount(it.entry.Amount))

		line := cursorMark + checkbox + " " + date + "  " +
			padRight(it.entry.OffsetAccount, colOffset) + "  " +
			padRight(it.entry.Description, colDesc) + "  " +
			amt

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

	// ── Bottom separator (with scroll-down indicator) ────
	if below > 0 {
		indicator := fmt.Sprintf("  ↓ %d more", below)
		sb.WriteString(strings.Repeat("─", sepWidth-len(indicator)) + indicator + "\n")
	} else {
		sb.WriteString(strings.Repeat("─", sepWidth) + "\n")
	}

	// ── Footer balance ───────────────────────────────────
	clearedStr := utils.FormatAmount(m.clearedBalance())
	diffStr := utils.FormatAmount(abs(diff))
	var diffLabel string
	var diffStyle lipgloss.Style
	switch {
	case diff == 0:
		diffLabel = "Balanced"
		diffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	case diff > 0:
		diffLabel = fmt.Sprintf("Short $%s", diffStr)
		diffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	default: // diff < 0: cleared more than statement
		diffLabel = fmt.Sprintf("Over $%s", diffStr)
		diffStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	}
	sb.WriteString(fmt.Sprintf(
		"Cleared $%s · %s\n",
		clearedStr,
		diffStyle.Render(diffLabel),
	))

	// ── Prompt or hint ───────────────────────────────────
	if m.confirmPending {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		direction := "short"
		if diff < 0 {
			direction = "over"
		}
		sb.WriteString(fmt.Sprintf("\n%s\n",
			warnStyle.Render(fmt.Sprintf("$%s %s. Confirm anyway? (y/n)", diffStr, direction)),
		))
	} else {
		sb.WriteString("\nspace toggle · a select all · enter finish · ↑↓ navigate · q quit\n")
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
	return m.statementBalance - (m.lastReconciledBalance + m.clearedBalance())
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// padRight pads s to exactly `width` terminal display columns, correctly
// accounting for double-wide CJK characters. Strings wider than `width`
// are truncated and suffixed with "…".
func padRight(s string, width int) string {
	w := rw.StringWidth(s)
	if w >= width {
		// Truncate: fill width-1 columns then append "…" (1 column).
		truncated := rw.Truncate(s, width-1, "")
		tw := rw.StringWidth(truncated)
		return truncated + strings.Repeat(" ", width-1-tw) + "…"
	}
	return s + strings.Repeat(" ", width-w)
}
