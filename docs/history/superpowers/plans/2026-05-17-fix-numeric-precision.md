# Fix Numeric Precision (Issues #31, #67) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two numeric precision bugs in `internal/utils/`: `AbsInt64(math.MinInt64)` silently overflows (#31), and `FormatAmount` loses precision for large cent values due to float64 conversion (#67).

**Architecture:** Pure integer arithmetic for `FormatAmount` (divide/mod by 100, manual comma insertion). Explicit panic in `AbsInt64` for `math.MinInt64`. Both are leaf utility functions with no cross-cutting changes needed.

**Tech Stack:** Go stdlib (`math`, `strings`, `strconv`)

---

## File Structure

- Modify: `internal/utils/math.go` — add `math.MinInt64` guard to `AbsInt64`
- Modify: `internal/utils/amount.go` — rewrite `FormatAmount` using integer arithmetic, remove `humanize` import
- Modify: `internal/utils/amount_test.go` — add large-amount and boundary test cases for `FormatAmount`
- Create: `internal/utils/math_test.go` — unit tests for `AbsInt64` including boundary behavior

---

### Task 1: AbsInt64 — Add Failing Test for math.MinInt64

**Files:**
- Create: `internal/utils/math_test.go`

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils_test

import (
	"math"
	"testing"

	"github.com/hance08/kea/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAbsInt64(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want int64
	}{
		{"positive", 42, 42},
		{"negative", -42, 42},
		{"zero", 0, 0},
		{"max int64", math.MaxInt64, math.MaxInt64},
		{"negative one", -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, utils.AbsInt64(tt.n))
		})
	}
}

func TestAbsInt64_MinInt64_Panics(t *testing.T) {
	assert.Panics(t, func() {
		utils.AbsInt64(math.MinInt64)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/utils/ -run TestAbsInt64_MinInt64_Panics -v`
Expected: FAIL — no panic occurs, `AbsInt64(math.MinInt64)` returns silently.

- [ ] **Step 3: Commit**

```bash
git add internal/utils/math_test.go
git commit -m "test: add AbsInt64 tests including math.MinInt64 panic expectation (closes #31)"
```

---

### Task 2: AbsInt64 — Fix the Overflow

**Files:**
- Modify: `internal/utils/math.go`

- [ ] **Step 1: Implement the fix**

Replace the entire body of `internal/utils/math.go` with:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils

import "math"

func AbsInt64(n int64) int64 {
	if n == math.MinInt64 {
		panic("utils.AbsInt64: undefined for math.MinInt64 (overflow)")
	}
	if n < 0 {
		return -n
	}
	return n
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/utils/ -run TestAbsInt64 -v`
Expected: All PASS (both `TestAbsInt64` and `TestAbsInt64_MinInt64_Panics`).

- [ ] **Step 3: Commit**

```bash
git add internal/utils/math.go
git commit -m "fix: panic on AbsInt64(math.MinInt64) instead of silent overflow (closes #31)"
```

---

### Task 3: FormatAmount — Add Failing Tests for Large Amounts

**Files:**
- Modify: `internal/utils/amount_test.go`

- [ ] **Step 1: Add large-amount test cases to `TestFormatAmount`**

Add these entries to the `tests` slice in `TestFormatAmount`:

```go
{"large amount near float64 precision limit", 9007199254740993, "90,071,992,547,409.93"},
{"max practical (90 trillion)", 9000000000000000, "90,000,000,000,000"},
{"negative large", -9007199254740993, "-90,071,992,547,409.93"},
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/utils/ -run TestFormatAmount -v`
Expected: FAIL — the float64-based implementation rounds `9007199254740993` to `9007199254740992`, producing incorrect output.

- [ ] **Step 3: Commit**

```bash
git add internal/utils/amount_test.go
git commit -m "test: add large-amount cases to FormatAmount exposing float64 precision loss (#67)"
```

---

### Task 4: FormatAmount — Rewrite with Integer Arithmetic

**Files:**
- Modify: `internal/utils/amount.go`

- [ ] **Step 1: Rewrite FormatAmount**

Replace the `FormatAmount` function and update imports. The new implementation uses integer division/mod and manual comma insertion:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hance08/kea/internal/model"
)

func FormatAmount(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}

	whole := cents / int64(model.CentsPerUnit)
	frac := cents % int64(model.CentsPerUnit)

	wholeStr := insertCommas(strconv.FormatInt(whole, 10))

	var result string
	if frac == 0 {
		result = wholeStr
	} else if frac%10 == 0 {
		result = fmt.Sprintf("%s.%d", wholeStr, frac/10)
	} else {
		result = fmt.Sprintf("%s.%02d", wholeStr, frac)
	}

	if negative {
		return "-" + result
	}
	return result
}

func insertCommas(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}

	var buf strings.Builder
	firstGroup := n % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	buf.WriteString(s[:firstGroup])
	for i := firstGroup; i < n; i += 3 {
		buf.WriteByte(',')
		buf.WriteString(s[i : i+3])
	}
	return buf.String()
}
```

- [ ] **Step 2: Run all utils tests**

Run: `go test ./internal/utils/ -v`
Expected: All PASS — existing tests continue to pass, new large-amount tests pass.

- [ ] **Step 3: Remove unused `humanize` dependency if no other file uses it**

Run: `grep -r "go-humanize" --include="*.go" . | grep -v "_test.go" | grep -v "amount.go"` to check. If no other file imports it, run `go mod tidy`.

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/utils/amount.go go.mod go.sum
git commit -m "fix: rewrite FormatAmount with integer arithmetic to eliminate float64 precision loss (closes #67)"
```

---

### Task 5: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS.

- [ ] **Step 2: Build**

Run: `make build`
Expected: Clean build with no errors.

---

## Notes

- The `humanize` library (`github.com/dustin/go-humanize`) may still be used elsewhere (check before removing from `go.mod`).
- The `MaxSafeBalanceFloat` constant in `model/types.go` becomes dead code after this fix. It can be removed in a separate cleanup PR if desired.
