# Deduplicate determineMode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the duplicated `determineMode` logic from `cmd/add_actions.go` and `cmd/transaction/edit_actions.go` into a single `ParseTransactionTypeLabel` function in `internal/model/types.go`.

**Architecture:** Add a new public function `ParseTransactionTypeLabel(s string) TransactionType` to the model package that uses fuzzy substring matching (existing `determineMode` behavior). Both cmd call sites replace their method calls with `model.ParseTransactionTypeLabel(rawType)`. The existing `ParseTransactionType` (strict exact-match) remains unchanged.

**Tech Stack:** Go standard library (`strings`)

---

### Task 1: Add `ParseTransactionTypeLabel` with tests

**Files:**
- Modify: `internal/model/types.go:163` (insert after `ParseTransactionType`)
- Modify: `internal/model/types_test.go:56` (insert after `TestParseTransactionType`)

- [ ] **Step 1: Write the failing test**

Add to `internal/model/types_test.go` after line 56 (after `TestParseTransactionType`):

```go
func TestParseTransactionTypeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  TransactionType
	}{
		{"Expense (pay a bill...)", TxTypeExpense},
		{"expense", TxTypeExpense},
		{"EXPENSE", TxTypeExpense},
		{"Income (receive payment...)", TxTypeIncome},
		{"income", TxTypeIncome},
		{"Transfer (move between accounts)", TxTypeTransfer},
		{"transfer", TxTypeTransfer},
		{"something else", TxTypeTransfer},
		{"", TxTypeTransfer},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseTransactionTypeLabel(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/model/ -run TestParseTransactionTypeLabel -v`
Expected: compilation error — `ParseTransactionTypeLabel` undefined.

- [ ] **Step 3: Implement `ParseTransactionTypeLabel`**

Add to `internal/model/types.go` after `ParseTransactionType` (after line 163):

```go
func ParseTransactionTypeLabel(s string) TransactionType {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "expense") {
		return TxTypeExpense
	}
	if strings.Contains(lower, "income") {
		return TxTypeIncome
	}
	return TxTypeTransfer
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/model/ -run TestParseTransactionTypeLabel -v`
Expected: PASS (all 9 cases).

- [ ] **Step 5: Run all model tests to check for regressions**

Run: `go test ./internal/model/ -v`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/model/types.go internal/model/types_test.go
git commit -m "feat: add ParseTransactionTypeLabel for fuzzy type matching (#112)"
```

---

### Task 2: Replace `determineMode` in `cmd/add_actions.go`

**Files:**
- Modify: `cmd/add_actions.go:86` (change call site)
- Modify: `cmd/add_actions.go:191-200` (delete method)

- [ ] **Step 1: Replace the call site**

In `cmd/add_actions.go` at line 86, change:

```go
mode := r.determineMode(rawType)
```

to:

```go
mode := model.ParseTransactionTypeLabel(rawType)
```

- [ ] **Step 2: Delete the `determineMode` method**

Remove lines 191–200 from `cmd/add_actions.go`:

```go
func (r *addRunner) determineMode(rawInput string) model.TransactionType {
	lower := strings.ToLower(rawInput)
	if strings.Contains(lower, "expense") {
		return model.TxTypeExpense
	}
	if strings.Contains(lower, "income") {
		return model.TxTypeIncome
	}
	return model.TxTypeTransfer
}
```

- [ ] **Step 3: Remove unused `strings` import if needed**

Check whether `cmd/add_actions.go` still uses `strings` elsewhere. If not, remove the import. (It likely still uses it — verify before removing.)

- [ ] **Step 4: Verify it compiles and tests pass**

Run: `go build ./cmd/... && go test ./cmd/ -v`
Expected: builds clean, all cmd tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/add_actions.go
git commit -m "refactor: use model.ParseTransactionTypeLabel in add command (#112)"
```

---

### Task 3: Replace `determineMode` in `cmd/transaction/edit_actions.go`

**Files:**
- Modify: `cmd/transaction/edit_actions.go:264` (change call site)
- Modify: `cmd/transaction/edit_actions.go:277-286` (delete method)

- [ ] **Step 1: Replace the call site**

In `cmd/transaction/edit_actions.go` at line 264, change:

```go
newType := r.determineMode(rawType)
```

to:

```go
newType := model.ParseTransactionTypeLabel(rawType)
```

- [ ] **Step 2: Delete the `determineMode` method**

Remove lines 277–286 from `cmd/transaction/edit_actions.go`:

```go
func (r *editRunner) determineMode(rawInput string) model.TransactionType {
	lower := strings.ToLower(rawInput)
	if strings.Contains(lower, "expense") {
		return model.TxTypeExpense
	}
	if strings.Contains(lower, "income") {
		return model.TxTypeIncome
	}
	return model.TxTypeTransfer
}
```

- [ ] **Step 3: Remove unused `strings` import if needed**

Check whether `cmd/transaction/edit_actions.go` still uses `strings` elsewhere. If not, remove the import.

- [ ] **Step 4: Verify it compiles and tests pass**

Run: `go build ./cmd/... && go test ./cmd/transaction/ -v`
Expected: builds clean, all transaction cmd tests pass.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./...`
Expected: all tests pass, no regressions.

- [ ] **Step 6: Commit**

```bash
git add cmd/transaction/edit_actions.go
git commit -m "refactor: use model.ParseTransactionTypeLabel in edit command (#112)"
```
