// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Run backs up dbPath if any tier is due. It is a no-op when dbPath does not
// exist. When db is non-nil, the SQLite online backup API is used for a
// consistent snapshot; when nil, the DB file is copied directly (CLI startup
// path where the database is not yet open). Errors are non-fatal: the caller
// should log and continue startup.
func Run(dbPath string, db *sql.DB) error {
	return run(dbPath, realClock{}, db)
}

func run(dbPath string, clk clock, db *sql.DB) error {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	dir := filepath.Dir(dbPath)
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	ext := filepath.Ext(dbPath)
	dbBase := strings.TrimSuffix(filepath.Base(dbPath), ext)
	now := clk.Now()

	for _, t := range tiers {
		label := t.label(now)
		dst := filepath.Join(backupDir, backupFilename(dbBase, t.name, label, ext))

		if _, err := os.Stat(dst); err == nil {
			continue // backup for this period already exists
		}

		if err := doBackup(dbPath, dst, db); err != nil {
			return fmt.Errorf("backup %s: %w", t.name, err)
		}

		if err := rotate(backupDir, dbBase, t.name, ext, t.retention); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: backup rotation failed for %s: %v\n", t.name, err)
		}
	}

	return nil
}

type tier struct {
	name      string
	label     func(time.Time) string
	retention int
}

var tiers = []tier{
	{name: "daily", label: dailyLabel, retention: 7},
	{name: "weekly", label: weeklyLabel, retention: 4},
	{name: "monthly", label: monthlyLabel, retention: 12},
}

func dailyLabel(t time.Time) string {
	return t.Format("2006-01-02")
}

func weeklyLabel(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

func monthlyLabel(t time.Time) string {
	return t.Format("2006-01")
}

// backupFilename builds the backup file name.
// e.g. backupFilename("kea", "daily", "2026-04-14", ".db") → "kea_daily_2026-04-14.db"
func backupFilename(dbBase, tierName, label, ext string) string {
	return fmt.Sprintf("%s_%s_%s%s", dbBase, tierName, label, ext)
}

// rotate removes the oldest backups for a given tier beyond the retention limit.
// Files are identified by prefix "<dbBase>_<tierName>_" and suffix ext.
func rotate(backupDir, dbBase, tierName, ext string, retention int) error {
	prefix := fmt.Sprintf("%s_%s_", dbBase, tierName)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ext) {
			files = append(files, e.Name())
		}
	}

	sort.Strings(files) // lexicographic = chronological given the naming scheme

	for len(files) > retention {
		if err := os.Remove(filepath.Join(backupDir, files[0])); err != nil {
			return err
		}
		files = files[1:]
	}

	return nil
}

// doBackup dispatches to the appropriate backup strategy. When db is non-nil
// the SQLite online backup API is used for a consistent snapshot; otherwise
// the source file is copied directly.
func doBackup(dbPath, dst string, db *sql.DB) error {
	if db != nil {
		return backupOnline(context.Background(), db, dst)
	}

	tmpDB, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open for backup: %w", err)
	}
	defer tmpDB.Close()

	if err := tmpDB.Ping(); err != nil {
		return fmt.Errorf("ping for backup: %w", err)
	}

	return backupOnline(context.Background(), tmpDB, dst)
}
