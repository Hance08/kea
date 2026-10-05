# Fix Non-Interactive Reconcile --force Check (Issue #40)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the balance-mismatch decision before any write in non-interactive reconcile mode, so `--force` is checked before mutating the ledger.

**Architecture:** Extract the validation/diff-computation logic from `ReconcileTransactions` into a new `PreviewReconcile` method that shares the same selection logic but performs no writes. The non-interactive command path calls `PreviewReconcile` first, checks the diff against `--force`, and only then calls `ReconcileTransactions`.

**Tech Stack:** Go, standard library testing

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/service/reconcile_ops.go` | Modify | Add `PreviewReconcile` method |
| `internal/service/reconcile_ops_test.go` | Modify | Add tests for `PreviewReconcile` |
| `cmd/reconcile.go:20-23` | Modify | Add `PreviewReconcile` to `reconcileTxProvider` interface |
| `cmd/reconcile_actions.go` | Modify | Call `PreviewReconcile` before `ReconcileTransactions` in non-interactive path |

---

### Task 1: Add PreviewReconcile service method with tests

**Files:**
- Modify: `internal/service/reconcile_ops.go:46`
- Test: `internal/service/reconcile_ops_test.go`

- [ ] **Step 1: Write failing tests for PreviewReconcile**

Add these tests at the end of `internal/service/reconcile_ops_test.go`:

```go
// ── PreviewReconcile ─────────────────────────────────────────────────────────

func TestPreviewReconcile_ZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
		{ID: 11, Amount: -50000},
	})

	diff, err := svc.PreviewReconcile(context.Background(), 1, 50000, []int64{10, 11})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
}

func TestPreviewReconcile_NonZeroDifference(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	diff, err := svc.PreviewReconcile(context.Background(), 1, 120000, []int64{10})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 20000 {
		t.Errorf("expected diff 20000, got %d", diff)
	}
}

func TestPreviewReconcile_InvalidID(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 100000},
	})

	_, err := svc.PreviewReconcile(context.Background(), 1, 100000, []int64{10, 99})

	if err == nil {
		t.Fatal("expected error for unknown ID, got nil")
	}
}

func TestPreviewReconcile_EmptyIDs(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})

	_, err := svc.PreviewReconcile(context.Background(), 1, 100000, []int64{})

	if err == nil {
		t.Fatal("expected error for empty IDs, got nil")
	}
}

func TestPreviewReconcile_AccountNotFound(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	_, err := svc.PreviewReconcile(context.Background(), 99, 100000, []int64{10})

	if err == nil {
		t.Fatal("expected error for missing account, got nil")
	}
}

func TestPreviewReconcile_WithPriorBalance(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 20, Amount: 3000},
	})
	txRepo.lastReconciledBalances[1] = 2000

	// diff = statementBalance - (lastBalance + cleared) = 5000 - (2000 + 3000) = 0
	diff, err := svc.PreviewReconcile(context.Background(), 1, 5000, []int64{20})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != 0 {
		t.Errorf("expected diff 0, got %d", diff)
	}
}

func TestPreviewReconcile_DoesNotMutate(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	svc := newTestTransactionService(accRepo, txRepo)

	accRepo.addAccount(&model.Account{ID: 1, Name: "Assets:Checking", Type: model.AccountTypeAsset})
	seedUnreconciled(txRepo, 1, []*model.ReconcileEntry{
		{ID: 10, Amount: 50000},
	})

	_, err := svc.PreviewReconcile(context.Background(), 1, 50000, []int64{10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no writes happened: markSplitsReconciledCalls should be empty.
	if len(txRepo.markSplitsReconciledCalls) != 0 {
		t.Errorf("expected no markSplits calls, got %d", len(txRepo.markSplitsReconciledCalls))
	}
	// lastReconciledBalances should be unchanged (still 0 / absent).
	if bal := txRepo.lastReconciledBalances[1]; bal != 0 {
		t.Errorf("expected last reconciled balance unchanged at 0, got %d", bal)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/service/ -run TestPreviewReconcile -v`
Expected: FAIL — `svc.PreviewReconcile` does not exist yet.

- [ ] **Step 3: Implement PreviewReconcile**

Add this method to `internal/service/reconcile_ops.go`, before `ReconcileTransactions`:

```go
// PreviewReconcile validates the given transaction IDs against the
// unreconciled set for accountID and computes the balance difference without
// persisting any changes. Use this to check whether --force is needed before
// committing to a write.
//
// The returned difference is:
//
//	statementBalance − (lastReconciledBalance + clearedBalance)
func (ts *TransactionService) PreviewReconcile(ctx context.Context, accountID int64, statementBalance int64, txIDs []int64) (int64, error) {
	if len(txIDs) == 0 {
		return 0, fmt.Errorf("no transactions selected for reconciliation")
	}

	if _, err := ts.accRepo.GetAccountByID(ctx, accountID); err != nil {
		return 0, fmt.Errorf("account not found: %w", err)
	}

	lastBalance, err := ts.txRepo.GetLastReconciledBalance(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load last reconciled balance: %w", err)
	}

	entries, err := ts.txRepo.GetUnreconciledTransactionsByAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("failed to load unreconciled transactions: %w", err)
	}

	validAmounts := make(map[int64]int64, len(entries))
	for _, e := range entries {
		validAmounts[e.ID] = e.Amount
	}

	var clearedBalance int64
	for _, id := range txIDs {
		amount, ok := validAmounts[id]
		if !ok {
			return 0, fmt.Errorf("transaction ID %d is not in the unreconciled set for this account", id)
		}
		clearedBalance += amount
	}

	return statementBalance - (lastBalance + clearedBalance), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/service/ -run TestPreviewReconcile -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/reconcile_ops.go internal/service/reconcile_ops_test.go
git commit -m "feat(service): add PreviewReconcile for dry-run diff computation"
```

---

### Task 2: Fix non-interactive command path to check before writing

**Files:**
- Modify: `cmd/reconcile_actions.go:42-63`

- [ ] **Step 1: Update runNonInteractive to call PreviewReconcile before ReconcileTransactions**

Replace the `runNonInteractive` method body in `cmd/reconcile_actions.go`:

```go
func (r *reconcileRunner) runNonInteractive(ctx context.Context, acc *model.Account) error {
	statementBalance, err := utils.ParseAmount(r.flags.Balance)
	if err != nil {
		return fmt.Errorf("invalid --balance value %q: %w", r.flags.Balance, err)
	}

	txIDs, err := parseIDs(r.flags.IDs)
	if err != nil {
		return fmt.Errorf("invalid --ids value %q: %w", r.flags.IDs, err)
	}

	// Check balance mismatch BEFORE writing anything.
	diff, err := r.txSvc.PreviewReconcile(ctx, acc.ID, statementBalance, txIDs)
	if err != nil {
		return err
	}

	if diff != 0 && !r.flags.Force {
		return fmt.Errorf(
			"balance mismatch: off by $%s — use --force to reconcile anyway",
			utils.FormatAmount(abs64(diff)),
		)
	}

	// Validation passed (or --force): persist the reconciliation.
	diff, err = r.txSvc.ReconcileTransactions(ctx, acc.ID, statementBalance, txIDs)
	if err != nil {
		return err
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
```

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/reconcile_actions.go
git commit -m "fix(cmd): check --force before writing in non-interactive reconcile

Closes #40"
```

---

### Task 3: Verify end-to-end behavior

- [ ] **Step 1: Build the binary**

Run: `make build`
Expected: Clean build, no errors.

- [ ] **Step 2: Run full test suite one final time**

Run: `go test ./...`
Expected: All PASS.
