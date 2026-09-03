package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRunSynthesizeWipeExportStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	wipePath := filepath.Join(t.TempDir(), "missing-wipe-quarantine.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"synthesize-wipe-export-status", "--kind", "character-item-state", "--wipe-export", wipePath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing synthesize-wipe-export-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing synthesize-wipe-export-status not to write stderr, got %q", stderr.String())
	}
	var got synthesizeWipeExportStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing wipe-export status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != synthesizeWipeExportStatusFormat || got.Present || got.Export != nil || got.Kind != "" || got.WipeExportSHA256 != "" || got.ScopeKey != "" || got.ScopeCount != 0 || len(got.ScopeIDs) != 0 {
		t.Fatalf("unexpected missing wipe-export status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("synthesize-wipe-export-status must not open a database target, got events %#v", events)
	}
}

func TestRunSynthesizeWipeExportStatusReadsValidCharacterItemWipe(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := []byte(`{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`)
	wipePath := filepath.Join(t.TempDir(), "wipe-quarantine.json")
	if err := os.WriteFile(wipePath, raw, 0o600); err != nil {
		t.Fatalf("write wipe-export: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"synthesize-wipe-export-status", "--kind", "character-item-state", "--wipe-export", wipePath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected synthesize-wipe-export-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on synthesize-wipe-export-status success, got %q", stderr.String())
	}
	var got synthesizeWipeExportStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode wipe-export status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != synthesizeWipeExportStatusFormat || !got.Present || got.Kind != "character-item-state" {
		t.Fatalf("unexpected wipe-export status envelope: %#v", got)
	}
	wantSHA := sha256Hex(raw)
	if got.WipeExportSHA256 != wantSHA {
		t.Fatalf("unexpected wipe_export_sha256: got %s want %s", got.WipeExportSHA256, wantSHA)
	}
	if got.ScopeKey != "character_ids" || got.ScopeCount != 1 || len(got.ScopeIDs) != 1 || got.ScopeIDs[0] != 11 {
		t.Fatalf("unexpected wipe scope: %#v", got)
	}
	exportBytes, err := json.Marshal(got.Export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	var export accountstore.CharacterItemStateExport
	if err := json.Unmarshal(exportBytes, &export); err != nil {
		t.Fatalf("decode nested export: %v\nbody:\n%s", err, string(exportBytes))
	}
	if export.MigrationVersion != accountstore.CharacterItemStateMigrationVersion || export.MigrationName != accountstore.CharacterItemStateMigrationName {
		t.Fatalf("unexpected nested migration identity: %#v", export)
	}
	if len(export.CharacterIDs) != 1 || export.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected nested character_ids: %#v", export.CharacterIDs)
	}
	if export.InventoryItems == nil || export.EquipmentItems == nil || export.Quickslots == nil {
		t.Fatalf("expected present empty row arrays, got %#v", export)
	}
	if len(export.InventoryItems) != 0 || len(export.EquipmentItems) != 0 || len(export.Quickslots) != 0 {
		t.Fatalf("expected empty wipe rows, got %#v", export)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("synthesize-wipe-export-status must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(wipePath); err != nil {
		t.Fatalf("synthesize-wipe-export-status must not remove the inspected file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("synthesize-wipe-export-status must not open a database target, got events %#v", events)
	}
}

func TestRunSynthesizeWipeExportStatusReadsValidGroundVIDWipe(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := []byte(`{"migration_version":10,"migration_name":"bootstrap_ground_item_state","vids":[117440556],"ground_items":[]}`)
	wipePath := filepath.Join(t.TempDir(), "wipe-quarantine.json")
	if err := os.WriteFile(wipePath, raw, 0o600); err != nil {
		t.Fatalf("write wipe-export: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"synthesize-wipe-export-status", "--kind", "bootstrap-ground-item-state", "--wipe-export", wipePath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected synthesize-wipe-export-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got synthesizeWipeExportStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode wipe-export status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.ScopeKey != "vids" || got.ScopeCount != 1 || len(got.ScopeIDs) != 1 || got.ScopeIDs[0] != 117440556 {
		t.Fatalf("unexpected wipe scope: %#v", got)
	}
	exportBytes, err := json.Marshal(got.Export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	var export worldruntime.BootstrapGroundItemStateExport
	if err := json.Unmarshal(exportBytes, &export); err != nil {
		t.Fatalf("decode nested export: %v\nbody:\n%s", err, string(exportBytes))
	}
	if len(export.VIDs) != 1 || export.VIDs[0] != 117440556 {
		t.Fatalf("unexpected nested vids: %#v", export.VIDs)
	}
	if export.GroundItems == nil || len(export.GroundItems) != 0 {
		t.Fatalf("expected empty ground_items, got %#v", export.GroundItems)
	}
}

func TestRunSynthesizeWipeExportStatusRejectsInvalidContracts(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	cases := []struct {
		name string
		kind string
		raw  string
		want string
	}{
		{
			name: "empty-scope",
			kind: "character-item-state",
			raw:  `{"migration_version":3,"migration_name":"character_item_state","character_ids":[],"inventory_items":[],"equipment_items":[],"quickslots":[]}`,
			want: "wipe scope",
		},
		{
			name: "non-empty-rows",
			kind: "character-item-state",
			raw:  `{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[{"id":1001,"character_id":11,"slot":0,"vnum":27001,"count":1}],"equipment_items":[],"quickslots":[]}`,
			want: "wipe export row arrays must be empty",
		},
		{
			name: "wrapped-quarantine",
			kind: "character-item-state",
			raw:  `{"summary":{"character_count":1,"inventory_item_count":0,"equipment_item_count":0,"quickslot_count":0,"character_ids":[11]},"export":{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}}`,
			want: "bare",
		},
		{
			name: "wrong-migration-identity",
			kind: "character-item-state",
			raw:  `{"migration_version":4,"migration_name":"character_quest_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`,
			want: "migration",
		},
		{
			name: "unknown-field",
			kind: "character-item-state",
			raw:  `{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[],"extra":true}`,
			want: "decode",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wipePath := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(wipePath, []byte(tc.raw), 0o600); err != nil {
				t.Fatalf("write wipe-export: %v", err)
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"synthesize-wipe-export-status", "--kind", tc.kind, "--wipe-export", wipePath}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected exit %d, got %d stderr=%q", exitError, code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
			}
			if !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(tc.want)) {
				t.Fatalf("expected stderr to mention %q, got %q", tc.want, stderr.String())
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("synthesize-wipe-export-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunSynthesizeWipeExportStatusRejectsSymlinkAndUnsupportedKind(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	raw := []byte(`{"migration_version":3,"migration_name":"character_item_state","character_ids":[11],"inventory_items":[],"equipment_items":[],"quickslots":[]}`)
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "wipe-quarantine.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"synthesize-wipe-export-status", "--kind", "character-item-state", "--wipe-export", link}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected symlink reject exit %d, got %d stderr=%q", exitError, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on symlink reject, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("expected symlink error, got %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"synthesize-wipe-export-status", "--kind", "account-character-roster", "--wipe-export", target}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected unsupported kind usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported") {
		t.Fatalf("expected unsupported kind stderr, got %q", stderr.String())
	}
}

func TestRunRejectsUnknownCommandMentionsSynthesizeWipeExportStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "synthesize-wipe-export-status") {
		t.Fatalf("expected usage to mention synthesize-wipe-export-status, got %q", stderr.String())
	}
}
