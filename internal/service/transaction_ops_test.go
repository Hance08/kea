package service

import (
	"errors"
	"testing"
	"time"

	"github.com/hance08/kea/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
// Test helper: standard account fixture
// ──────────────────────────────────────────────

func setupStandardAccounts(accRepo *mockAccountRepo) {
	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank", Type: model.AccountTypeAsset, Currency: ""})
	accRepo.addAccount(&model.Account{ID: 2, Name: "Expenses:Food", Type: model.AccountTypeExpense, Currency: ""})
	accRepo.addAccount(&model.Account{ID: 3, Name: "Revenue:Salary", Type: model.AccountTypeRevenue, Currency: ""})
	accRepo.addAccount(&model.Account{ID: 4, Name: "Assets:Cash", Type: model.AccountTypeAsset, Currency: "TWD"})
}

// ──────────────────────────────────────────────
// CreateTransaction
// ──────────────────────────────────────────────

func TestCreateTransaction(t *testing.T) {
	t.Run("valid double-entry transaction created", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Description: "Lunch",
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(input)
		require.NoError(t, err)
		assert.Greater(t, id, int64(0))
	})

	t.Run("fewer than 2 splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Assets:Bank", Amount: 1000},
			},
		}
		_, err := svc.CreateTransaction(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("empty splits rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateTransaction(model.TransactionDetail{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("unbalanced splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -400}, // off by 100
			},
		}
		_, err := svc.CreateTransaction(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "balance")
	})

	t.Run("unknown account rejected with split number in error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		svc := newTestTransactionService(accRepo, newMockTransactionRepo())

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Assets:Bank", Amount: -500},
				{AccountName: "NonExistent:Account", Amount: 500},
			},
		}
		_, err := svc.CreateTransaction(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "#2")
	})

	t.Run("zero timestamp is set to current time", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		before := time.Now().Unix()
		input := model.TransactionDetail{
			Timestamp: 0,
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(input)
		require.NoError(t, err)

		tx := txRepo.transactions[id]
		assert.GreaterOrEqual(t, tx.Timestamp, before)
	})

	t.Run("account currency overrides system default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Cash", Amount: -500}, // Currency: TWD
			},
		}
		id, err := svc.CreateTransaction(input)
		require.NoError(t, err)

		splits := txRepo.splits[id]
		require.Len(t, splits, 2)
		// Assets:Cash (ID:4) has Currency="TWD"
		var cashSplit *model.Split
		for _, s := range splits {
			if s.AccountID == 4 {
				cashSplit = s
			}
		}
		require.NotNil(t, cashSplit)
		assert.Equal(t, "TWD", cashSplit.Currency)
	})

	t.Run("account without currency uses system default", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		id, err := svc.CreateTransaction(input)
		require.NoError(t, err)

		for _, s := range txRepo.splits[id] {
			assert.Equal(t, "USD", s.Currency)
		}
	})

	t.Run("db failure returns error", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.createErr = errors.New("db write failed")
		svc := newTestTransactionService(accRepo, txRepo)

		input := model.TransactionDetail{
			Splits: []model.SplitDetail{
				{AccountName: "Expenses:Food", Amount: 500},
				{AccountName: "Assets:Bank", Amount: -500},
			},
		}
		_, err := svc.CreateTransaction(input)
		assert.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// CreateSimpleTransaction
// ──────────────────────────────────────────────

func TestCreateSimpleTransaction(t *testing.T) {
	t.Run("creates two balanced splits with correct direction", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		detail, err := svc.CreateSimpleTransaction(
			"Assets:Bank", "Expenses:Food", 1000, "Dinner", 0, model.StatusPending,
		)
		require.NoError(t, err)
		assert.Greater(t, detail.ID, int64(0))

		// to account = +amount, from account = -amount
		require.Len(t, detail.Splits, 2)
		amounts := map[string]int64{}
		for _, s := range detail.Splits {
			amounts[s.AccountName] = s.Amount
		}
		assert.Equal(t, int64(1000), amounts["Expenses:Food"])
		assert.Equal(t, int64(-1000), amounts["Assets:Bank"])
	})

	t.Run("splits sum to zero", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		svc := newTestTransactionService(accRepo, txRepo)

		detail, err := svc.CreateSimpleTransaction(
			"Assets:Bank", "Expenses:Food", 750, "Coffee", 0, model.StatusPending,
		)
		require.NoError(t, err)

		var total int64
		for _, s := range detail.Splits {
			total += s.Amount
		}
		assert.Equal(t, int64(0), total)
	})

	t.Run("same account rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateSimpleTransaction(
			"Assets:Bank", "Assets:Bank", 1000, "Self", 0, model.StatusPending,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "same")
	})

	t.Run("zero amount rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateSimpleTransaction(
			"Assets:Bank", "Expenses:Food", 0, "Zero", 0, model.StatusPending,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positive")
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		_, err := svc.CreateSimpleTransaction(
			"Assets:Bank", "Expenses:Food", -100, "Negative", 0, model.StatusPending,
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positive")
	})
}

// ──────────────────────────────────────────────
// DeleteTransaction
// ──────────────────────────────────────────────

func TestDeleteTransaction(t *testing.T) {
	t.Run("pending transaction deleted successfully", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(accRepo, txRepo)

		err := svc.DeleteTransaction(5)
		require.NoError(t, err)
		_, exists := txRepo.transactions[5]
		assert.False(t, exists, "transaction should not exist after deletion")
	})

	t.Run("cleared transaction deleted successfully", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 6, Status: model.StatusCleared}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.DeleteTransaction(6)
		require.NoError(t, err)
	})

	t.Run("opening balance transaction (ID=1) rejected", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.DeleteTransaction(model.OpeningBalanceTransactionID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotEditable), "expected ErrNotEditable, got: %v", err)
	})

	t.Run("non-existent transaction returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.DeleteTransaction(999)
		require.Error(t, err)
	})

	t.Run("reconciled transaction rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 7, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.DeleteTransaction(7)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReconciled), "expected ErrReconciled, got: %v", err)
	})

	t.Run("ErrReconciled detectable via errors.Is through error chain", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 8, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.DeleteTransaction(8)
		// wrapped with %w — errors.Is must unwrap correctly
		assert.True(t, errors.Is(err, ErrReconciled))
		assert.False(t, errors.Is(err, ErrNotEditable))
	})
}

// ──────────────────────────────────────────────
// UpdateTransactionStatus
// ──────────────────────────────────────────────

func TestUpdateTransactionStatus(t *testing.T) {
	t.Run("pending to cleared is valid", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(5, model.StatusCleared)
		require.NoError(t, err)
		assert.Equal(t, model.StatusCleared, txRepo.transactions[5].Status)
	})

	t.Run("cleared to pending is valid", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusCleared}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(5, model.StatusPending)
		require.NoError(t, err)
	})

	t.Run("setting status to reconciled(2) rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(5, model.StatusReconciled)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("invalid status(99) rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(5, model.TransactionStatus(99))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("modifying reconciled transaction rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionStatus(5, model.StatusCleared)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReconciled))
	})

	t.Run("non-existent transaction returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.UpdateTransactionStatus(999, model.StatusCleared)
		require.Error(t, err)
	})
}

// ──────────────────────────────────────────────
// UpdateTransactionComplete
// ──────────────────────────────────────────────

func TestUpdateTransactionComplete(t *testing.T) {
	makeExistingSplits := func(ids ...int64) []*model.Split {
		result := make([]*model.Split, len(ids))
		for i, id := range ids {
			result[i] = &model.Split{ID: id, AccountID: int64(i + 1), Amount: 0}
		}
		return result
	}

	t.Run("valid complete update succeeds", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			makeExistingSplits(10, 11),
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, Amount: -800, Currency: "USD"},
			{ID: 11, AccountID: 2, Amount: 800, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(5, "Updated desc", 0, model.StatusCleared, splits)
		require.NoError(t, err)
	})

	t.Run("invalid status(99) rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionComplete(5, "", 0, model.TransactionStatus(99), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("non-existent transaction returns error", func(t *testing.T) {
		svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())
		err := svc.UpdateTransactionComplete(999, "", 0, model.StatusPending,
			[]model.SplitDetail{{AccountID: 1, Amount: 0}, {AccountID: 2, Amount: 0}})
		require.Error(t, err)
	})

	t.Run("reconciled transaction rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusReconciled}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusCleared,
			[]model.SplitDetail{{AccountID: 1, Amount: 500}, {AccountID: 2, Amount: -500}})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReconciled))
	})

	t.Run("fewer than 2 splits rejected", func(t *testing.T) {
		txRepo := newMockTransactionRepo()
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(newMockAccountRepo(), txRepo)

		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusPending,
			[]model.SplitDetail{{AccountID: 1, Amount: 0}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2")
	})

	t.Run("unbalanced splits rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(accRepo, txRepo)

		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusPending,
			[]model.SplitDetail{
				{AccountID: 1, Amount: -500},
				{AccountID: 2, Amount: 400}, // off by 100
			})
		require.Error(t, err)
	})

	t.Run("non-existent account rejected", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Bank"})
		txRepo.addTransaction(&model.Transaction{ID: 5, Status: model.StatusPending}, nil)
		svc := newTestTransactionService(accRepo, txRepo)

		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusPending,
			[]model.SplitDetail{
				{AccountID: 1, Amount: -500},
				{AccountID: 99, Amount: 500}, // does not exist
			})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "99")
	})

	t.Run("removed split triggers DeleteSplit call", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		// original has 3 splits; new payload only has 2
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			[]*model.Split{
				{ID: 10, AccountID: 1, Amount: -1000},
				{ID: 11, AccountID: 2, Amount: 600},
				{ID: 12, AccountID: 3, Amount: 400}, // this split will be removed
			},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, Amount: -1000, Currency: "USD"},
			{ID: 11, AccountID: 2, Amount: 1000, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusPending, splits)
		require.NoError(t, err)
		assert.Contains(t, txRepo.deleteSplitCalls, int64(12), "split ID 12 should be deleted")
	})

	t.Run("split with ID=0 triggers CreateSplit call", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			[]*model.Split{{ID: 10, AccountID: 1, Amount: -1000}},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, Amount: -1000, Currency: "USD"},
			{ID: 0, AccountID: 2, Amount: 1000, Currency: "USD"}, // new split
		}
		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusPending, splits)
		require.NoError(t, err)
		assert.Len(t, txRepo.createSplitCalls, 1, "CreateSplit should be called once")
	})

	t.Run("existing split (ID≠0) triggers UpdateSplit call", func(t *testing.T) {
		accRepo := newMockAccountRepo()
		txRepo := newMockTransactionRepo()
		setupStandardAccounts(accRepo)
		txRepo.addTransaction(
			&model.Transaction{ID: 5, Status: model.StatusPending},
			[]*model.Split{
				{ID: 10, AccountID: 1, Amount: -1000},
				{ID: 11, AccountID: 2, Amount: 1000},
			},
		)
		svc := newTestTransactionService(accRepo, txRepo)

		splits := []model.SplitDetail{
			{ID: 10, AccountID: 1, Amount: -800, Currency: "USD"},
			{ID: 11, AccountID: 2, Amount: 800, Currency: "USD"},
		}
		err := svc.UpdateTransactionComplete(5, "", 0, model.StatusPending, splits)
		require.NoError(t, err)
		assert.Contains(t, txRepo.updateSplitCalls, int64(10))
		assert.Contains(t, txRepo.updateSplitCalls, int64(11))
	})
}

// ──────────────────────────────────────────────
// IsEditable
// ──────────────────────────────────────────────

func TestIsEditable(t *testing.T) {
	svc := newTestTransactionService(newMockAccountRepo(), newMockTransactionRepo())

	t.Run("regular transaction is editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 5}
		assert.True(t, svc.IsEditable(detail))
	})

	t.Run("opening balance transaction (ID=1) is not editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: model.OpeningBalanceTransactionID}
		assert.False(t, svc.IsEditable(detail))
	})

	t.Run("ID=2 is editable", func(t *testing.T) {
		detail := &model.TransactionDetail{ID: 2}
		assert.True(t, svc.IsEditable(detail))
	})
}
