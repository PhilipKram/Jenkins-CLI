package pipeline

import (
	"testing"
)

// TestStatusIsAliasForStages verifies that "pipeline status" resolves to
// the stages subcommand.
func TestStatusIsAliasForStages(t *testing.T) {
	// Verify the alias is registered on stagesCmd
	found := false
	for _, alias := range stagesCmd.Aliases {
		if alias == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'status' to be registered as an alias for 'stages'")
	}

	// Verify the parent command can resolve "status"
	cmd, _, err := Cmd.Find([]string{"status"})
	if err != nil {
		t.Fatalf("failed to find 'status' subcommand: %v", err)
	}
	if cmd.Name() != "stages" {
		t.Errorf("expected 'status' to resolve to 'stages', got %q", cmd.Name())
	}
}
