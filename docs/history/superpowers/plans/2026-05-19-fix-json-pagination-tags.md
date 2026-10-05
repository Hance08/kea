# Fix Missing JSON Tags on Pagination Types — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add snake_case JSON struct tags to `ListResult` and `ListOptions` so API output is consistent with every other model type.

**Architecture:** Single-file change in `internal/model/pagination.go`. No new files, no interface changes.

**Tech Stack:** Go struct tags

---

## File Structure

- Modify: `internal/model/pagination.go` — add `json:"..."` tags to both structs

---

### Task 1: Add JSON tags to ListOptions and ListResult

**Files:**
- Modify: `internal/model/pagination.go:9-21`

- [ ] **Step 1: Add JSON tags to both structs**

Replace the two structs in `internal/model/pagination.go` with:

```go
type ListOptions struct {
	Limit        int  `json:"limit"`
	Offset       int  `json:"offset"`
	IncludeCount bool `json:"include_count,omitempty"`
}

type ListResult[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"total_count"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
}
```

Conventions match existing model types: snake_case, `omitempty` on the boolean field that defaults to false.

- [ ] **Step 2: Run tests to verify nothing breaks**

Run: `go test ./...`
Expected: all tests PASS — struct tags don't affect Go code, only JSON marshalling.

- [ ] **Step 3: Commit**

```bash
git add internal/model/pagination.go
git commit -m "fix: add missing JSON tags to ListResult and ListOptions (#116)"
```
