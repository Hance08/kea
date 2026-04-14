package backup

import (
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
