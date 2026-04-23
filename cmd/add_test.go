package cmd

import (
	"testing"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// mockTransactionProvider
// ──────────────────────────────────────────────

type mockTransactionProvider struct {
	createSimpleErr error
	createSplitsErr error
	lastSplitsInput []model.SplitDetail
}

func (m *mockTransactionProvider) GetTransactionRule(mode model.TransactionType) (model.TransactionRule, error) {
	return model.TransactionRule{}, nil
}

func (m *mockTransactionProvider) CreateSimpleTransaction(fromAccount, toAccount string, amount int64, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
	if m.createSimpleErr != nil {
		return model.TransactionDetail{}, m.createSimpleErr
	}
	return model.TransactionDetail{}, nil
}

func (m *mockTransactionProvider) CreateTransactionFromSplits(splits []model.SplitDetail, desc string, timestamp int64, status model.TransactionStatus, txType model.TransactionType) (model.TransactionDetail, error) {
	m.lastSplitsInput = splits
	if m.createSplitsErr != nil {
		return model.TransactionDetail{}, m.createSplitsErr
	}
	return model.TransactionDetail{ID: 1}, nil
}

// ──────────────────────────────────────────────
// TestParseSplitFlag
// ──────────────────────────────────────────────

func TestParseSplitFlag(t *testing.T) {
	t.Run("parses positive amount", func(t *testing.T) {
		got, err := parseSplitFlag("Expenses:Food:Lunch=2000")
		require.NoError(t, err)
		assert.Equal(t, "Expenses:Food:Lunch", got.AccountName)
		assert.Equal(t, int64(200000), got.Amount)
	})

	t.Run("parses negative amount", func(t *testing.T) {
		got, err := parseSplitFlag("Assets:Bank:Bank1=-1000")
		require.NoError(t, err)
		assert.Equal(t, "Assets:Bank:Bank1", got.AccountName)
		assert.Equal(t, int64(-100000), got.Amount)
	})

	t.Run("parses decimal amount", func(t *testing.T) {
		got, err := parseSplitFlag("Assets:Cash=10.50")
		require.NoError(t, err)
		assert.Equal(t, int64(1050), got.Amount)
	})

	t.Run("missing equals returns error", func(t *testing.T) {
		_, err := parseSplitFlag("Assets:Bank")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected format")
	})

	t.Run("empty account name returns error", func(t *testing.T) {
		_, err := parseSplitFlag("=1000")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account name")
	})

	t.Run("invalid amount returns error", func(t *testing.T) {
		_, err := parseSplitFlag("Assets:Bank=abc")
		require.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// TestRunFromFlags_SplitMode
// ──────────────────────────────────────────────

func TestRunFromFlags_SplitMode(t *testing.T) {
	runner := &addRunner{}

	t.Run("valid two-split expense", func(t *testing.T) {
		flags := &addFlags{
			Type:        "expense",
			Description: "team lunch",
			Splits:      []string{"Assets:Bank=-1000", "Expenses:Food=1000"},
		}
		input, err := runner.runFromFlags(flags)
		require.NoError(t, err)
		require.Len(t, input.Splits, 2)
		assert.Equal(t, "Assets:Bank", input.Splits[0].AccountName)
		assert.Equal(t, int64(-100000), input.Splits[0].Amount)
		assert.Equal(t, "Expenses:Food", input.Splits[1].AccountName)
		assert.Equal(t, int64(100000), input.Splits[1].Amount)
		assert.Equal(t, model.TxTypeExpense, input.Type)
		assert.Equal(t, "team lunch", input.Description)
	})

	t.Run("three-split expense", func(t *testing.T) {
		flags := &addFlags{
			Type:   "expense",
			Splits: []string{"Assets:Bank:Bank1=-1000", "Assets:Bank:Bank2=-1000", "Expenses:Food:Lunch=2000"},
		}
		input, err := runner.runFromFlags(flags)
		require.NoError(t, err)
		assert.Len(t, input.Splits, 3)
		assert.Equal(t, int64(200000), input.Splits[2].Amount)
	})

	t.Run("missing --type returns error", func(t *testing.T) {
		flags := &addFlags{
			Splits: []string{"Assets:Bank=-1000", "Expenses:Food=1000"},
		}
		_, err := runner.runFromFlags(flags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--type is required")
	})

	t.Run("single split returns error", func(t *testing.T) {
		flags := &addFlags{
			Type:   "expense",
			Splits: []string{"Assets:Bank=-1000"},
		}
		_, err := runner.runFromFlags(flags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("invalid type returns error", func(t *testing.T) {
		flags := &addFlags{
			Type:   "bogus",
			Splits: []string{"Assets:Bank=-1000", "Expenses:Food=1000"},
		}
		_, err := runner.runFromFlags(flags)
		require.Error(t, err)
	})

	t.Run("malformed split returns error", func(t *testing.T) {
		flags := &addFlags{
			Type:   "expense",
			Splits: []string{"Assets:Bank=-1000", "Expenses:Food"},
		}
		_, err := runner.runFromFlags(flags)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected format")
	})

	t.Run("empty description defaults to dash", func(t *testing.T) {
		flags := &addFlags{
			Type:   "expense",
			Splits: []string{"Assets:Bank=-500", "Expenses:Food=500"},
		}
		input, err := runner.runFromFlags(flags)
		require.NoError(t, err)
		assert.Equal(t, "-", input.Description)
	})
}
