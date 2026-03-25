package views

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hance08/kea/internal/model"
)

// WriteJSON encodes v as indented JSON to stdout.
func WriteJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// CentsToUnit converts int64 cents to float64 currency units (÷ model.CentsPerUnit).
func CentsToUnit(cents int64) float64 {
	return float64(cents) / float64(model.CentsPerUnit)
}
