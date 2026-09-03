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
	if got.QuarantineComplete || got.TwoPhaseWipeArtifactsComplete || got.QuarantinePresentCount != 0 || got.WipeQuarantinePresentCount != 0 || got.ImportResultPresentCount != 0 {
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
