// Package store implements the repository interfaces on SQLite and runs schema
// migrations. All methods take a context and work over either *sql.DB or
// *sql.Tx.
package store
