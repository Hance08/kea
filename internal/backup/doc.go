// Package backup takes tiered (daily, weekly, monthly) copies of the ledger
// database at startup, using the SQLite online backup API, so a bad run can be
// rolled back.
package backup
