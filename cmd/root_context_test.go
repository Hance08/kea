package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestExecuteContextPropagatesContext(t *testing.T) {
	type ctxKey string
	key := ctxKey("test-signal")

	ctx := context.WithValue(context.Background(), key, "present")

	var receivedCtx context.Context
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{
		Use: "child",
		RunE: func(cmd *cobra.Command, args []string) error {
			receivedCtx = cmd.Context()
			return nil
		},
	}
	root.AddCommand(child)
	root.SetArgs([]string{"child"})

	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	if receivedCtx == nil {
		t.Fatal("child command did not receive a context")
	}
	val, ok := receivedCtx.Value(key).(string)
	if !ok || val != "present" {
		t.Errorf("child context missing test value: got %q, want %q", val, "present")
	}
}
