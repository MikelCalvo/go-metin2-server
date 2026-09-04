package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExportTreeStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-export-tree")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"export-tree-status", "--export-tree", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing export-tree-status not to write stderr, got %q", stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != exportTreeStatusFormat || got.Present || got.ExportTree != "" || len(got.Kinds) != 0 {
		t.Fatalf("unexpected missing export-tree status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusReportsEmptyPresentTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T120000Z-abcdef012345")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatalf("mkdir export-tree: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected empty present export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode empty export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != exportTreeStatusFormat || !got.Present || got.ExportTree != tree {
		t.Fatalf("unexpected empty export-tree status envelope: %#v", got)
	}
	if got.KindCount != len(exportQuarantineKinds) || len(got.Kinds) != len(exportQuarantineKinds) {
		t.Fatalf("unexpected kind count: %#v", got)
	}
	if got.QuarantineComplete || got.TwoPhaseWipeArtifactsComplete || got.ImportResultArtifactsComplete || got.WipeImportArtifactsComplete || got.QuarantinePresentCount != 0 || got.WipeQuarantinePresentCount != 0 || got.ImportResultPresentCount != 0 || got.ImportResultStatusPresentCount != 0 || got.WipeImportResultPresentCount != 0 || got.WipeImportResultStatusPresentCount != 0 {
		t.Fatalf("unexpected empty-tree aggregates: %#v", got)
	}
	for i, kind := range exportQuarantineKinds {
		entry := got.Kinds[i]
		if entry.Kind != kind {
			t.Fatalf("kind[%d]=%q want %q", i, entry.Kind, kind)
		}
		if entry.Quarantine.Present || entry.ImportResult.Present || entry.ImportResultStatus.Present {
			t.Fatalf("expected absent artifacts for %s, got %#v", kind, entry)
		}
		wantWipe := false
		for _, wipeKind := range importExportDrillWipeKinds {
			if wipeKind == kind {
				wantWipe = true
				break
			}
		}
		if entry.WipeKind != wantWipe {
			t.Fatalf("wipe_kind for %s = %v want %v", kind, entry.WipeKind, wantWipe)
		}
		if wantWipe {
			if entry.WipeQuarantine == nil || entry.WipeQuarantine.Present || entry.WipeQuarantineStatus == nil || entry.WipeQuarantineStatus.Present {
				t.Fatalf("expected absent wipe artifacts for %s, got %#v", kind, entry)
			}
		} else if entry.WipeQuarantine != nil || entry.WipeQuarantineStatus != nil || entry.WipeImportResult != nil || entry.WipeImportResultStatus != nil {
			t.Fatalf("non-wipe kind %s must omit wipe fields, got %#v", kind, entry)
		}
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("export-tree-status must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusReportsCompleteQuarantineAndTwoPhaseArtifacts(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T130000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)

	wipePayloads := map[string]string{
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_ids":[11],"points":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_ids":[11],"unit_prices":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_ids":[11],"flags":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_ids":[11],"passwords":[],"items":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","vids":[117440556],"ground_items":[]}`,
	}
	for kind, payload := range wipePayloads {
		wipePath := filepath.Join(tree, kind, "wipe-quarantine.json")
		if err := os.WriteFile(wipePath, []byte(payload), 0o600); err != nil {
			t.Fatalf("write wipe-quarantine for %s: %v", kind, err)
		}
		var wipeStdout bytes.Buffer
		var wipeStderr bytes.Buffer
		code := Run([]string{"synthesize-wipe-export-status", "--kind", kind, "--wipe-export", wipePath}, nil, &wipeStdout, &wipeStderr)
		if code != exitOK {
			t.Fatalf("synthesize-wipe-export-status for %s: exit=%d stderr=%q", kind, code, wipeStderr.String())
		}
		if err := os.WriteFile(filepath.Join(tree, kind, "wipe-quarantine-status.json"), wipeStdout.Bytes(), 0o600); err != nil {
			t.Fatalf("write wipe-quarantine-status for %s: %v", kind, err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected complete export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode complete export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || !got.QuarantineComplete || !got.TwoPhaseWipeArtifactsComplete {
		t.Fatalf("expected complete aggregates, got %#v", got)
	}
	if got.ImportResultArtifactsComplete || got.WipeImportArtifactsComplete || got.ImportResultPresentCount != 0 || got.ImportResultStatusPresentCount != 0 || got.WipeImportResultPresentCount != 0 || got.WipeImportResultStatusPresentCount != 0 {
		t.Fatalf("expected absent import/wipe-import completeness before import artifacts, got %#v", got)
	}
	if got.QuarantinePresentCount != len(exportQuarantineKinds) || got.WipeQuarantinePresentCount != len(importExportDrillWipeKinds) {
		t.Fatalf("unexpected present counts: %#v", got)
	}
	for _, entry := range got.Kinds {
		if !entry.Quarantine.Present || entry.Quarantine.SHA256 == "" {
			t.Fatalf("expected present quarantine for %s, got %#v", entry.Kind, entry.Quarantine)
		}
		if entry.WipeKind {
			if entry.WipeQuarantine == nil || !entry.WipeQuarantine.Present || entry.WipeQuarantine.ScopeCount < 1 {
				t.Fatalf("expected present wipe quarantine for %s, got %#v", entry.Kind, entry.WipeQuarantine)
			}
			if entry.WipeQuarantineStatus == nil || !entry.WipeQuarantineStatus.Present {
				t.Fatalf("expected present wipe quarantine status for %s, got %#v", entry.Kind, entry.WipeQuarantineStatus)
			}
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusReportsImportAndWipeImportArtifactCompleteness(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T150000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)

	wipePayloads := map[string]string{
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_ids":[11],"points":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_ids":[11],"unit_prices":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_ids":[11],"flags":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_ids":[11],"passwords":[],"items":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","vids":[117440556],"ground_items":[]}`,
	}
	for kind, payload := range wipePayloads {
		wipePath := filepath.Join(tree, kind, "wipe-quarantine.json")
		if err := os.WriteFile(wipePath, []byte(payload), 0o600); err != nil {
			t.Fatalf("write wipe-quarantine for %s: %v", kind, err)
		}
		var wipeStdout bytes.Buffer
		var wipeStderr bytes.Buffer
		code := Run([]string{"synthesize-wipe-export-status", "--kind", kind, "--wipe-export", wipePath}, nil, &wipeStdout, &wipeStderr)
		if code != exitOK {
			t.Fatalf("synthesize-wipe-export-status for %s: exit=%d stderr=%q", kind, code, wipeStderr.String())
		}
		if err := os.WriteFile(filepath.Join(tree, kind, "wipe-quarantine-status.json"), wipeStdout.Bytes(), 0o600); err != nil {
			t.Fatalf("write wipe-quarantine-status for %s: %v", kind, err)
		}
	}

	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected import-complete export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode import-complete export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || !got.QuarantineComplete || !got.TwoPhaseWipeArtifactsComplete || !got.ImportResultArtifactsComplete || !got.WipeImportArtifactsComplete {
		t.Fatalf("expected import/wipe-import complete aggregates, got %#v", got)
	}
	if got.ImportResultPresentCount != len(exportQuarantineKinds) || got.ImportResultStatusPresentCount != len(exportQuarantineKinds) {
		t.Fatalf("unexpected import-result counts: %#v", got)
	}
	if got.WipeImportResultPresentCount != len(importExportDrillWipeKinds) || got.WipeImportResultStatusPresentCount != len(importExportDrillWipeKinds) {
		t.Fatalf("unexpected wipe-import-result counts: %#v", got)
	}
	for _, entry := range got.Kinds {
		if !entry.ImportResult.Present || !entry.ImportResultStatus.Present {
			t.Fatalf("expected present import artifacts for %s, got %#v", entry.Kind, entry)
		}
		if entry.WipeKind {
			if entry.WipeImportResult == nil || !entry.WipeImportResult.Present || entry.WipeImportResultStatus == nil || !entry.WipeImportResultStatus.Present {
				t.Fatalf("expected present wipe-import artifacts for %s, got %#v", entry.Kind, entry)
			}
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusRequireFlagsSucceedOnCompleteTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T160000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)

	wipePayloads := map[string]string{
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_ids":[11],"points":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_ids":[11],"unit_prices":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_ids":[11],"flags":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_ids":[11],"passwords":[],"items":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","vids":[117440556],"ground_items":[]}`,
	}
	for kind, payload := range wipePayloads {
		wipePath := filepath.Join(tree, kind, "wipe-quarantine.json")
		if err := os.WriteFile(wipePath, []byte(payload), 0o600); err != nil {
			t.Fatalf("write wipe-quarantine for %s: %v", kind, err)
		}
		var wipeStdout bytes.Buffer
		var wipeStderr bytes.Buffer
		code := Run([]string{"synthesize-wipe-export-status", "--kind", kind, "--wipe-export", wipePath}, nil, &wipeStdout, &wipeStderr)
		if code != exitOK {
			t.Fatalf("synthesize-wipe-export-status for %s: exit=%d stderr=%q", kind, code, wipeStderr.String())
		}
		if err := os.WriteFile(filepath.Join(tree, kind, "wipe-quarantine-status.json"), wipeStdout.Bytes(), 0o600); err != nil {
			t.Fatalf("write wipe-quarantine-status for %s: %v", kind, err)
		}
	}
	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"export-tree-status",
		"--export-tree", tree,
		"--require-quarantine-complete",
		"--require-two-phase-wipe-artifacts-complete",
		"--require-import-result-artifacts-complete",
		"--require-wipe-import-artifacts-complete",
	}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected gated complete export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on gated success, got %q", stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode gated complete export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || !got.QuarantineComplete || !got.TwoPhaseWipeArtifactsComplete || !got.ImportResultArtifactsComplete || !got.WipeImportArtifactsComplete {
		t.Fatalf("expected complete gated aggregates, got %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusRejectsInvalidContracts(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T140000Z-abcdef012345")
	if err := os.MkdirAll(filepath.Join(tree, "character-item-state"), 0o755); err != nil {
		t.Fatalf("mkdir kind: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tree, "character-item-state", "quarantine.json"), []byte(`{"migration_version":3,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[],"extra":true}`), 0o600); err != nil {
		t.Fatalf("write invalid quarantine: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected invalid quarantine to fail closed, exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "character-item-state/quarantine.json") {
		t.Fatalf("expected stderr to name invalid artifact path, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusRejectsRelativeSymlinkAndFileTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	fileTree := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(fileTree, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write file tree: %v", err)
	}
	realTree := filepath.Join(dir, "real-tree")
	if err := os.MkdirAll(realTree, 0o755); err != nil {
		t.Fatalf("mkdir real-tree: %v", err)
	}
	linkTree := filepath.Join(dir, "link-tree")
	if err := os.Symlink(realTree, linkTree); err != nil {
		t.Fatalf("symlink export-tree: %v", err)
	}

	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{
			name: "relative",
			args: []string{"export-tree-status", "--export-tree", "relative/exports/tree"},
			code: exitError,
			want: "absolute path",
		},
		{
			name: "file",
			args: []string{"export-tree-status", "--export-tree", fileTree},
			code: exitError,
			want: "directory",
		},
		{
			name: "symlink",
			args: []string{"export-tree-status", "--export-tree", linkTree},
			code: exitError,
			want: "symlink",
		},
		{
			name: "usage",
			args: []string{"export-tree-status"},
			code: exitUsage,
			want: "--export-tree is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != tc.code {
				t.Fatalf("exit=%d want %d stderr=%q", code, tc.code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsExportTreeStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
	if !strings.Contains(stderr.String(), "export-tree-status") {
		t.Fatalf("expected usage to mention export-tree-status, got %q", stderr.String())
	}
}

func TestRunExportTreeStatusRequireFlagsFailClosedOnIncompleteOrAbsentTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)

	emptyTree := filepath.Join(t.TempDir(), "exports", "20260904T140000Z-abcdef012345")
	if err := os.MkdirAll(emptyTree, 0o755); err != nil {
		t.Fatalf("mkdir empty export-tree: %v", err)
	}
	missingTree := filepath.Join(t.TempDir(), "missing-export-tree")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "empty-tree-require-quarantine",
			args: []string{"export-tree-status", "--export-tree", emptyTree, "--require-quarantine-complete"},
			want: "--require-quarantine-complete",
		},
		{
			name: "empty-tree-require-two-phase-wipe",
			args: []string{"export-tree-status", "--export-tree", emptyTree, "--require-two-phase-wipe-artifacts-complete"},
			want: "--require-two-phase-wipe-artifacts-complete",
		},
		{
			name: "empty-tree-require-import-result",
			args: []string{"export-tree-status", "--export-tree", emptyTree, "--require-import-result-artifacts-complete"},
			want: "--require-import-result-artifacts-complete",
		},
		{
			name: "empty-tree-require-wipe-import",
			args: []string{"export-tree-status", "--export-tree", emptyTree, "--require-wipe-import-artifacts-complete"},
			want: "--require-wipe-import-artifacts-complete",
		},
		{
			name: "absent-tree-require-quarantine",
			args: []string{"export-tree-status", "--export-tree", missingTree, "--require-quarantine-complete"},
			want: "--require-quarantine-complete",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected require-gate exit %d, got %d stderr=%q stdout=%q", exitError, code, stderr.String(), stdout.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on require failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to name %q, got %q", tc.want, stderr.String())
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunExportTreeStatusReportsInsertOnlyImportResultOutcomes(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T170000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)
	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected insert-only outcome export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode insert-only outcome export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || !got.ImportResultArtifactsComplete || !got.ImportResultOutcomesComplete {
		t.Fatalf("expected complete import-result outcomes, got %#v", got)
	}
	if got.ImportResultReplacedCount != 0 || got.ImportResultAllReplaced || got.ImportResultRowCountTotal != 0 {
		t.Fatalf("unexpected insert-only outcome aggregates: %#v", got)
	}
	if len(got.Kinds) != len(exportQuarantineKinds) {
		t.Fatalf("unexpected kind count: %#v", got)
	}
	for _, entry := range got.Kinds {
		if entry.ImportResultOutcome == nil {
			t.Fatalf("expected import_result_outcome for %s", entry.Kind)
		}
		if entry.ImportResultOutcome.Replaced || entry.ImportResultOutcome.RowCount != 0 {
			t.Fatalf("unexpected insert-only outcome for %s: %#v", entry.Kind, entry.ImportResultOutcome)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusReportsMixedAndAllReplacedImportResultOutcomes(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T171000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)
	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)

	replacedPayloads := map[string]string{
		"account-character-roster":     `{"migration_version":2,"migration_name":"account_character_roster","account_count":1,"character_count":2,"account_ids":[7],"character_ids":[11,12],"replaced":true}`,
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_count":1,"inventory_item_count":3,"equipment_item_count":1,"quickslot_count":2,"character_ids":[11],"replaced":true}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_count":1,"point_row_count":4,"character_ids":[11],"replaced":true}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_count":1,"price_row_count":5,"character_ids":[11],"replaced":true}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_count":1,"flag_count":6,"character_ids":[11],"replaced":true}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_count":1,"password_count":1,"item_count":7,"character_ids":[11],"replaced":true}`,
		"auth-login-ticket-handoff":    `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","ticket_count":8,"active_ticket_count":1,"login_keys":[16909060],"replaced":true}`,
		"item-template-state":          `{"migration_version":9,"migration_name":"item_template_refine_info","template_count":9,"socket_count":0,"attribute_count":0,"use_effect_count":0,"equip_effect_count":0,"refine_info_count":0,"refine_material_count":0,"vnums":[19],"replaced":true}`,
		"static-actor-content-state":   `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definition_count":0,"merchant_catalog_entry_count":0,"quest_flag_reward_item_count":0,"quest_flag_consume_item_count":0,"static_actor_count":10,"reward_drop_count":0,"combat_profile_count":0,"combat_profile_death_reward_drop_count":0,"entity_ids":[101],"interaction_kinds":[],"combat_profiles":[],"replaced":true}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_item_count":11,"item_shaped_count":11,"gold_shaped_count":0,"vids":[117440556],"replaced":true}`,
	}
	wantRows := map[string]int{
		"account-character-roster":     2,
		"character-item-state":         6,
		"character-point-state":        4,
		"character-myshop-unit-prices": 5,
		"character-quest-state":        6,
		"character-safebox-state":      7,
		"auth-login-ticket-handoff":    8,
		"item-template-state":          9,
		"static-actor-content-state":   10,
		"bootstrap-ground-item-state":  11,
	}

	// Mixed: only character-item-state replaced with non-zero primary counts.
	mustRewriteExportTreeImportResult(t, tree, "character-item-state", replacedPayloads["character-item-state"])

	var mixedStdout bytes.Buffer
	var mixedStderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &mixedStdout, &mixedStderr)
	if code != exitOK {
		t.Fatalf("expected mixed outcome export-tree-status to succeed, exit=%d stderr=%q", code, mixedStderr.String())
	}
	var mixed exportTreeStatus
	if err := json.Unmarshal(mixedStdout.Bytes(), &mixed); err != nil {
		t.Fatalf("decode mixed outcome export-tree status JSON: %v\nbody:\n%s", err, mixedStdout.String())
	}
	if !mixed.ImportResultOutcomesComplete || mixed.ImportResultReplacedCount != 1 || mixed.ImportResultAllReplaced || mixed.ImportResultRowCountTotal != wantRows["character-item-state"] {
		t.Fatalf("unexpected mixed outcome aggregates: %#v", mixed)
	}
	for _, entry := range mixed.Kinds {
		if entry.ImportResultOutcome == nil {
			t.Fatalf("expected import_result_outcome for %s", entry.Kind)
		}
		if entry.Kind == "character-item-state" {
			if !entry.ImportResultOutcome.Replaced || entry.ImportResultOutcome.RowCount != wantRows[entry.Kind] {
				t.Fatalf("unexpected replaced outcome for %s: %#v", entry.Kind, entry.ImportResultOutcome)
			}
			continue
		}
		if entry.ImportResultOutcome.Replaced || entry.ImportResultOutcome.RowCount != 0 {
			t.Fatalf("unexpected insert-only outcome for %s: %#v", entry.Kind, entry.ImportResultOutcome)
		}
	}

	// All tip kinds replaced.
	for kind, payload := range replacedPayloads {
		mustRewriteExportTreeImportResult(t, tree, kind, payload)
	}
	var allStdout bytes.Buffer
	var allStderr bytes.Buffer
	code = Run([]string{"export-tree-status", "--export-tree", tree}, nil, &allStdout, &allStderr)
	if code != exitOK {
		t.Fatalf("expected all-replaced outcome export-tree-status to succeed, exit=%d stderr=%q", code, allStderr.String())
	}
	var all exportTreeStatus
	if err := json.Unmarshal(allStdout.Bytes(), &all); err != nil {
		t.Fatalf("decode all-replaced outcome export-tree status JSON: %v\nbody:\n%s", err, allStdout.String())
	}
	wantTotal := 0
	for _, rows := range wantRows {
		wantTotal += rows
	}
	if !all.ImportResultOutcomesComplete || all.ImportResultReplacedCount != len(exportQuarantineKinds) || !all.ImportResultAllReplaced || all.ImportResultRowCountTotal != wantTotal {
		t.Fatalf("unexpected all-replaced outcome aggregates: %#v", all)
	}
	for _, entry := range all.Kinds {
		if entry.ImportResultOutcome == nil || !entry.ImportResultOutcome.Replaced || entry.ImportResultOutcome.RowCount != wantRows[entry.Kind] {
			t.Fatalf("unexpected all-replaced outcome for %s: %#v", entry.Kind, entry.ImportResultOutcome)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusOmitsImportResultOutcomeWhenImportResultAbsent(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T172000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected quarantine-only export-tree-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got exportTreeStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode quarantine-only export-tree status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.ImportResultOutcomesComplete || got.ImportResultAllReplaced || got.ImportResultReplacedCount != 0 || got.ImportResultRowCountTotal != 0 {
		t.Fatalf("unexpected absent-outcome aggregates: %#v", got)
	}
	for _, entry := range got.Kinds {
		if entry.ImportResultOutcome != nil {
			t.Fatalf("expected omitted import_result_outcome for %s, got %#v", entry.Kind, entry.ImportResultOutcome)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusRejectsInvalidImportResultBeforeOutcomeAggregation(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	tree := filepath.Join(t.TempDir(), "exports", "20260904T173000Z-abcdef012345")
	mustMaterializeEmptyExportTreeStatusFixtures(t, tree)
	mustMaterializeEmptyExportTreeImportResultFixtures(t, tree)
	mustWriteFile(t, filepath.Join(tree, "character-item-state", "import-result.json"), []byte(`{"migration_version":3,"migration_name":"character_item_state","character_count":0,"inventory_item_count":0,"equipment_item_count":0,"quickslot_count":0,"character_ids":[],"extra":true}`+"\n"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status", "--export-tree", tree}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected invalid import-result to fail closed, exit=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on invalid import-result, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "character-item-state/import-result.json") {
		t.Fatalf("expected stderr to name invalid import-result path, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("export-tree-status must not open a database target, got events %#v", events)
	}
}

func TestRunExportTreeStatusUsageListsRequireFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-tree-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	body := stderr.String()
	for _, want := range []string{
		"--require-quarantine-complete",
		"--require-two-phase-wipe-artifacts-complete",
		"--require-import-result-artifacts-complete",
		"--require-wipe-import-artifacts-complete",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected usage to list %q, got %q", want, body)
		}
	}
}

func mustMaterializeEmptyExportTreeStatusFixtures(t *testing.T, exportTree string) {
	t.Helper()
	payloads := map[string]string{
		"account-character-roster":     `{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`,
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","points":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","unit_prices":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","flags":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","passwords":[],"items":[]}`,
		"auth-login-ticket-handoff":    `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","tickets":[]}`,
		"item-template-state":          `{"migration_version":9,"migration_name":"item_template_refine_info","templates":[],"sockets":[],"attributes":[],"use_effects":[],"equip_effects":[],"refine_infos":[],"refine_materials":[]}`,
		"static-actor-content-state":   `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definitions":[],"merchant_catalog_entries":[],"quest_flag_reward_items":[],"quest_flag_consume_items":[],"static_actors":[],"reward_drops":[],"combat_profiles":[],"combat_profile_death_reward_drops":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_items":[]}`,
	}
	for _, kind := range exportQuarantineKinds {
		payload, ok := payloads[kind]
		if !ok {
			t.Fatalf("missing empty quarantine payload for kind %q", kind)
		}
		dir := filepath.Join(exportTree, kind)
		mustMkdir(t, dir)
		mustWriteFile(t, filepath.Join(dir, "quarantine.json"), []byte(payload+"\n"))
	}
}

func mustMaterializeEmptyExportTreeImportResultFixtures(t *testing.T, exportTree string) {
	t.Helper()
	payloads := map[string]string{
		"account-character-roster":     `{"migration_version":2,"migration_name":"account_character_roster","account_count":0,"character_count":0,"account_ids":[],"character_ids":[]}`,
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","character_count":0,"inventory_item_count":0,"equipment_item_count":0,"quickslot_count":0,"character_ids":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","character_count":0,"point_row_count":0,"character_ids":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_count":0,"price_row_count":0,"character_ids":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","character_count":0,"flag_count":0,"character_ids":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","character_count":0,"password_count":0,"item_count":0,"character_ids":[]}`,
		"auth-login-ticket-handoff":    `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","ticket_count":0,"active_ticket_count":0,"login_keys":[]}`,
		"item-template-state":          `{"migration_version":9,"migration_name":"item_template_refine_info","template_count":0,"socket_count":0,"attribute_count":0,"use_effect_count":0,"equip_effect_count":0,"refine_info_count":0,"refine_material_count":0,"vnums":[]}`,
		"static-actor-content-state":   `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definition_count":0,"merchant_catalog_entry_count":0,"quest_flag_reward_item_count":0,"quest_flag_consume_item_count":0,"static_actor_count":0,"reward_drop_count":0,"combat_profile_count":0,"combat_profile_death_reward_drop_count":0,"entity_ids":[],"interaction_kinds":[],"combat_profiles":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_item_count":0,"item_shaped_count":0,"gold_shaped_count":0,"vids":[]}`,
	}
	wipeKindSet := make(map[string]struct{}, len(importExportDrillWipeKinds))
	for _, kind := range importExportDrillWipeKinds {
		wipeKindSet[kind] = struct{}{}
	}
	for _, kind := range exportQuarantineKinds {
		payload, ok := payloads[kind]
		if !ok {
			t.Fatalf("missing empty import-result payload for kind %q", kind)
		}
		dir := filepath.Join(exportTree, kind)
		mustMkdir(t, dir)
		resultPath := filepath.Join(dir, "import-result.json")
		mustWriteFile(t, resultPath, []byte(payload+"\n"))
		var statusStdout bytes.Buffer
		var statusStderr bytes.Buffer
		code := Run([]string{"import-export-status", "--kind", kind, "--import-result", resultPath}, nil, &statusStdout, &statusStderr)
		if code != exitOK {
			t.Fatalf("import-export-status for %s: exit=%d stderr=%q", kind, code, statusStderr.String())
		}
		mustWriteFile(t, filepath.Join(dir, "import-result-status.json"), statusStdout.Bytes())

		if _, isWipe := wipeKindSet[kind]; !isWipe {
			continue
		}
		wipeResultPath := filepath.Join(dir, "wipe-import-result.json")
		mustWriteFile(t, wipeResultPath, []byte(payload+"\n"))
		var wipeStatusStdout bytes.Buffer
		var wipeStatusStderr bytes.Buffer
		code = Run([]string{"import-export-status", "--kind", kind, "--import-result", wipeResultPath}, nil, &wipeStatusStdout, &wipeStatusStderr)
		if code != exitOK {
			t.Fatalf("import-export-status wipe result for %s: exit=%d stderr=%q", kind, code, wipeStatusStderr.String())
		}
		mustWriteFile(t, filepath.Join(dir, "wipe-import-result-status.json"), wipeStatusStdout.Bytes())
	}
}

func mustRewriteExportTreeImportResult(t *testing.T, exportTree, kind, payload string) {
	t.Helper()
	dir := filepath.Join(exportTree, kind)
	mustMkdir(t, dir)
	resultPath := filepath.Join(dir, "import-result.json")
	mustWriteFile(t, resultPath, []byte(payload+"\n"))
	var statusStdout bytes.Buffer
	var statusStderr bytes.Buffer
	code := Run([]string{"import-export-status", "--kind", kind, "--import-result", resultPath}, nil, &statusStdout, &statusStderr)
	if code != exitOK {
		t.Fatalf("import-export-status rewrite for %s: exit=%d stderr=%q", kind, code, statusStderr.String())
	}
	mustWriteFile(t, filepath.Join(dir, "import-result-status.json"), statusStdout.Bytes())
}
