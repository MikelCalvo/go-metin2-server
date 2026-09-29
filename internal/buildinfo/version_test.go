package buildinfo

import (
	"encoding/json"
	"testing"
)

func TestDefaultBuildInfoIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}

	if Commit == "" {
		t.Fatal("Commit should not be empty")
	}

	if BuildDate == "" {
		t.Fatal("BuildDate should not be empty")
	}
}

func TestCurrentReturnsPackageIdentity(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuildDate := BuildDate
	originalWorkflowRunID := WorkflowRunID
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuildDate = originalBuildDate
		WorkflowRunID = originalWorkflowRunID
	})

	Version = "v0.1.0-test"
	Commit = "abc1234"
	BuildDate = "2026-08-19T12:00:00Z"
	WorkflowRunID = "9876543210"

	got := Current()
	if got.Version != Version || got.Commit != Commit || got.BuildDate != BuildDate || got.WorkflowRunID != WorkflowRunID {
		t.Fatalf("unexpected snapshot %#v", got)
	}
}

func TestSnapshotJSONCarriesWorkflowRunIDAndKeepsMissingValueEmpty(t *testing.T) {
	for _, tc := range []struct {
		name          string
		workflowRunID string
	}{
		{name: "stamped", workflowRunID: "9876543210"},
		{name: "missing", workflowRunID: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(Snapshot{
				Version:       "v0.1.0-test",
				Commit:        "abc1234",
				BuildDate:     "2026-08-19T12:00:00Z",
				WorkflowRunID: tc.workflowRunID,
			})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			got, ok := decoded["workflow_run_id"]
			if !ok {
				t.Fatalf("workflow_run_id missing from %s", raw)
			}
			if got != tc.workflowRunID {
				t.Fatalf("workflow_run_id = %#v, want %q", got, tc.workflowRunID)
			}
		})
	}
}
