package migratecli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
)

func TestRunImportExportStatusReportsMissingResultWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	resultPath := filepath.Join(t.TempDir(), "missing-import-result.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"import-export-status", "--kind", "account-character-roster", "--import-result", resultPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing import-export-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing import-export-status not to write stderr, got %q", stderr.String())
	}
	var got importExportStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing import-export status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != importExportStatusFormat || got.Present || got.Result != nil || got.Kind != "" || got.ImportResultSHA256 != "" {
		t.Fatalf("unexpected missing import-export status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("import-export-status must not open a database target, got events %#v", events)
	}
}

func TestRunImportExportStatusReadsValidRosterResult(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := []byte(`{"migration_version":2,"migration_name":"account_character_roster","account_count":0,"character_count":0,"account_ids":[],"character_ids":[]}`)
	resultPath := filepath.Join(t.TempDir(), "import-result.json")
	if err := os.WriteFile(resultPath, raw, 0o600); err != nil {
		t.Fatalf("write import-result: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"import-export-status", "--kind", "account-character-roster", "--import-result", resultPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected import-export-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on import-export-status success, got %q", stderr.String())
	}
	var got importExportStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode import-export status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != importExportStatusFormat || !got.Present || got.Kind != "account-character-roster" {
		t.Fatalf("unexpected import-export status envelope: %#v", got)
	}
	wantSHA := sha256Hex(raw)
	if got.ImportResultSHA256 != wantSHA {
		t.Fatalf("unexpected import_result_sha256: got %s want %s", got.ImportResultSHA256, wantSHA)
	}
	resultBytes, err := json.Marshal(got.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var result accountstore.AccountCharacterRosterImportResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("decode nested result: %v\nbody:\n%s", err, string(resultBytes))
	}
	if result.MigrationVersion != accountstore.AccountCharacterRosterMigrationVersion || result.MigrationName != accountstore.AccountCharacterRosterMigrationName {
		t.Fatalf("unexpected nested migration identity: %#v", result)
	}
	if result.AccountCount != 0 || result.CharacterCount != 0 || len(result.AccountIDs) != 0 || len(result.CharacterIDs) != 0 {
		t.Fatalf("unexpected nested counts: %#v", result)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("import-export-status must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(resultPath); err != nil {
		t.Fatalf("import-export-status must not remove the inspected file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("import-export-status must not open a database target, got events %#v", events)
	}
}

func TestRunImportExportStatusAcceptsScopedReplaceScopeSlices(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	cases := []struct {
		name string
		kind string
		raw  string
	}{
		{
			name: "auth-login-ticket-wipe",
			kind: "auth-login-ticket-handoff",
			raw:  `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","ticket_count":0,"active_ticket_count":0,"login_keys":[16909060],"replaced":true}`,
		},
		{
			name: "auth-login-ticket-multi-history",
			kind: "auth-login-ticket-handoff",
			raw:  `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","ticket_count":2,"active_ticket_count":1,"login_keys":[16909060],"replaced":true}`,
		},
		{
			name: "item-template-wipe",
			kind: "item-template-state",
			raw:  `{"migration_version":9,"migration_name":"item_template_refine_info","template_count":0,"socket_count":0,"attribute_count":0,"use_effect_count":0,"equip_effect_count":0,"refine_info_count":0,"refine_material_count":0,"vnums":[11200],"replaced":true}`,
		},
		{
			name: "ground-item-wipe",
			kind: "bootstrap-ground-item-state",
			raw:  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_item_count":0,"item_shaped_count":0,"gold_shaped_count":0,"vids":[117440556],"replaced":true}`,
		},
		{
			name: "static-actor-wipe",
			kind: "static-actor-content-state",
			raw:  `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definition_count":0,"merchant_catalog_entry_count":0,"quest_flag_reward_item_count":0,"quest_flag_consume_item_count":0,"static_actor_count":0,"reward_drop_count":0,"combat_profile_count":0,"combat_profile_death_reward_drop_count":0,"entity_ids":[7],"interaction_kinds":[],"combat_profiles":["practice_static_store_import_wolf"],"replaced":true}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
				t.Fatalf("write import-result: %v", err)
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"import-export-status", "--kind", tc.kind, "--import-result", path}, nil, &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("expected scoped-replace status to succeed, exit=%d stderr=%q", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr on scoped-replace status success, got %q", stderr.String())
			}
			var got importExportStatus
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("decode scoped-replace status JSON: %v\nbody:\n%s", err, stdout.String())
			}
			if got.Format != importExportStatusFormat || !got.Present || got.Kind != tc.kind {
				t.Fatalf("unexpected scoped-replace status envelope: %#v", got)
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("import-export-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunImportExportStatusRejectsContractFailures(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	cases := []struct {
		name string
		kind string
		raw  string
		want string
	}{
		{
			name: "wrong-migration-identity",
			kind: "account-character-roster",
			raw:  `{"migration_version":3,"migration_name":"character_item_state","account_count":0,"character_count":0,"account_ids":[],"character_ids":[]}`,
			want: "unexpected migration identity",
		},
		{
			name: "unknown-field",
			kind: "account-character-roster",
			raw:  `{"migration_version":2,"migration_name":"account_character_roster","account_count":0,"character_count":0,"account_ids":[],"character_ids":[],"extra":true}`,
			want: "decode import-result",
		},
		{
			name: "count-slice-mismatch",
			kind: "account-character-roster",
			raw:  `{"migration_version":2,"migration_name":"account_character_roster","account_count":1,"character_count":0,"account_ids":[],"character_ids":[]}`,
			want: "account_ids length",
		},
		{
			name: "wrong-kind-for-payload",
			kind: "character-item-state",
			raw:  `{"migration_version":2,"migration_name":"account_character_roster","account_count":0,"character_count":0,"account_ids":[],"character_ids":[]}`,
			want: "decode import-result",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
				t.Fatalf("write import-result: %v", err)
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"import-export-status", "--kind", tc.kind, "--import-result", path}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected exit %d, got %d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected %q in stderr %q", tc.want, stderr.String())
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("import-export-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunImportExportStatusRejectsSymlink(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	raw := []byte(`{"migration_version":2,"migration_name":"account_character_roster","account_count":0,"character_count":0,"account_ids":[],"character_ids":[]}`)
	if err := os.WriteFile(target, raw, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"import-export-status", "--kind", "account-character-roster", "--import-result", link}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected exit %d, got %d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on symlink rejection, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "must not be a symlink") {
		t.Fatalf("expected symlink rejection, got %q", stderr.String())
	}
}

func TestRunImportExportStatusRejectsUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-flags", args: []string{"import-export-status"}, want: "--kind and --import-result are required"},
		{name: "unsupported-kind", args: []string{"import-export-status", "--kind", "not-a-kind", "--import-result", "/tmp/x.json"}, want: "unsupported import-export-status kind"},
		{name: "unexpected-arg", args: []string{"import-export-status", "--kind", "account-character-roster", "--import-result", "/tmp/x.json", "extra"}, want: "unexpected import-export-status argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != exitUsage {
				t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on usage error, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected %q in stderr %q", tc.want, stderr.String())
			}
			if !strings.Contains(stderr.String(), "import-export-status usage:") {
				t.Fatalf("expected import-export-status usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunHelpListsImportExportStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "import-export-status") {
		t.Fatalf("expected help to list import-export-status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "import-export-status usage:") {
		t.Fatalf("expected import-export-status usage block, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsImportExportStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "import-export-status") {
		t.Fatalf("expected usage to mention import-export-status, got %q", stderr.String())
	}
}

func TestImportExportStatusChecksumMatchesRawBytes(t *testing.T) {
	raw := []byte(`{"migration_version":2,"migration_name":"account_character_roster","account_count":0,"character_count":0,"account_ids":[],"character_ids":[]}`)
	sum := sha256.Sum256(raw)
	if got := sha256Hex(raw); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256Hex mismatch: got %s want %s", got, hex.EncodeToString(sum[:]))
	}
}
