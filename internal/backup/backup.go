package backup

import (
	"fmt"
	"io"
	"os"
	"time"
)

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Run backs up dbPath if any tier is due. It is a no-op when dbPath does not
// exist. Errors are non-fatal: the caller should log and continue startup.
func Run(dbPath string) error {
	return run(dbPath, realClock{})
}

func run(dbPath string, clk clock) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}
	return nil // remaining logic added in later tasks
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

// copyFile copies src to dst atomically via a .tmp intermediate file.
func copyFile(src, dst string) error {
	tmp := dst + ".tmp"

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}
