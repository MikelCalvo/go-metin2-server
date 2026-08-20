package migratecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBackupRestoreDrillPrintsPathAwareCommands(t *testing.T) {
	payload := `{
  "local_channel_id": 1,
  "visibility_mode": "radius",
  "visibility_radius": 400,
  "visibility_sector_size": 200,
  "persistence": {
    "login_ticket_store_dir": "/var/metin2/login-tickets",
    "account_store_dir": "/var/metin2/accounts",
    "static_actor_store_path": "/var/metin2/static-actors/static-actors.json",
    "interaction_store_path": "/var/metin2/interactions/interaction-definitions.json",
    "item_template_store_path": "/var/metin2/item-templates/item-templates.json",
    "quest_state_store_path": "/var/metin2/quest-state/quest-state.json"
  },
  "database": {
    "configured": false,
    "dsn_configured": false
  }
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"backup-restore-drill",
			"--runtime-config", "-",
			"--ops-base-url", "http://127.0.0.1:6060",
			"--backup-base", "/var/metin2/backups/drill",
		},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	body := stdout.String()
	for _, want := range []string{
		`OPS='http://127.0.0.1:6060'`,
		`BASE='/var/metin2/backups/drill'`,
		`ACCOUNT_STORE_DIR='/var/metin2/accounts'`,
		`LOGIN_TICKET_STORE_DIR='/var/metin2/login-tickets'`,
		`ITEM_TEMPLATE_STORE_PATH='/var/metin2/item-templates/item-templates.json'`,
		`INTERACTION_STORE_PATH='/var/metin2/interactions/interaction-definitions.json'`,
		`STATIC_ACTOR_STORE_PATH='/var/metin2/static-actors/static-actors.json'`,
		`QUEST_STATE_STORE_PATH='/var/metin2/quest-state/quest-state.json'`,
		`curl -sS "$OPS/healthz"`,
		`curl -sS "$OPS/local/runtime-config"`,
		`curl -sS "$OPS/local/persistence/status"`,
		`echo '== store validate / crash-temp triage =='`,
		`"$OPS/local/account-store/validate"`,
		`"$OPS/local/account-store/crash-temps/cleanup"`,
		`"$OPS/local/login-tickets/validate"`,
		`"$OPS/local/login-tickets/crash-temps/cleanup"`,
		`"$OPS/local/item-templates/validate"`,
		`"$OPS/local/item-templates/crash-temps/cleanup"`,
		`"$OPS/local/interaction-store/validate"`,
		`"$OPS/local/interaction-store/crash-temps/cleanup"`,
		`"$OPS/local/static-actor-store/validate"`,
		`"$OPS/local/static-actor-store/crash-temps/cleanup"`,
		`"$OPS/local/quest-state/validate"`,
		`"$OPS/local/quest-state/crash-temps/cleanup"`,
		`"$OPS/local/account-store/backup"`,
		`"$OPS/local/login-tickets/backup"`,
		`"$OPS/local/item-templates/backup"`,
		`"$OPS/local/interaction-store/backup"`,
		`"$OPS/local/static-actors/backup"`,
		`"$OPS/local/quest-state/backup"`,
		`"$OPS/local/account-store/backup/validate"`,
		`"$OPS/local/item-templates/restore"`,
		`"$OPS/local/login-tickets/restore"`,
		`mv "$ACCOUNT_STORE_DIR"`,
		`mv "$(dirname "$ITEM_TEMPLATE_STORE_PATH")"`,
		`# read-only printer: does not execute backup/restore`,
		`# crash-temps/cleanup mutates only hidden crash-temp residue after validate`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "memory://") {
		t.Fatalf("backup-restore-drill must not expose SQL or DSN text, got %s", body)
	}
	idxStatus := strings.Index(body, `curl -sS "$OPS/local/persistence/status"`)
	idxStoreValidate := strings.Index(body, `"$OPS/local/account-store/validate"`)
	idxCrashCleanup := strings.Index(body, `"$OPS/local/account-store/crash-temps/cleanup"`)
	idxBackup := strings.Index(body, `"$OPS/local/account-store/backup"`)
	idxBackupValidate := strings.Index(body, `"$OPS/local/account-store/backup/validate"`)
	idxRestore := strings.Index(body, `"$OPS/local/item-templates/restore"`)
	if idxStatus < 0 || idxStoreValidate < 0 || idxCrashCleanup < 0 || idxBackup < 0 || idxBackupValidate < 0 || idxRestore < 0 {
		t.Fatalf("missing expected ordering markers in stdout:\n%s", body)
	}
	if !(idxStatus < idxStoreValidate && idxStoreValidate < idxCrashCleanup && idxCrashCleanup < idxBackup && idxBackup < idxBackupValidate && idxBackupValidate < idxRestore) {
		t.Fatalf("expected status -> store validate -> crash-temp cleanup -> backup -> backup validate -> restore ordering, got idxs status=%d validate=%d cleanup=%d backup=%d backupValidate=%d restore=%d\n%s",
			idxStatus, idxStoreValidate, idxCrashCleanup, idxBackup, idxBackupValidate, idxRestore, body)
	}
	idxStaticActorValidate := strings.Index(body, `"$OPS/local/static-actor-store/validate"`)
	idxStaticActorsBackup := strings.Index(body, `"$OPS/local/static-actors/backup"`)
	if idxStaticActorValidate < 0 || idxStaticActorsBackup < 0 {
		t.Fatalf("missing static-actor validate/backup markers in stdout:\n%s", body)
	}
	if idxStaticActorValidate >= idxStaticActorsBackup {
		t.Fatalf("expected static-actor-store validate before static-actors backup")
	}
}

func TestRunBackupRestoreDrillReadsRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime-config.json")
	payload := `{
  "local_channel_id": 1,
  "visibility_mode": "whole_map",
  "visibility_radius": 0,
  "visibility_sector_size": 0,
  "persistence": {
    "login_ticket_store_dir": "/state/login-tickets",
    "account_store_dir": "/state/accounts",
    "static_actor_store_path": "/state/static/static-actors.json",
    "interaction_store_path": "/state/interactions/interaction-definitions.json",
    "item_template_store_path": "/state/items/item-templates.json",
    "quest_state_store_path": "/state/quests/quest-state.json"
  },
  "database": {"configured": false, "dsn_configured": false}
}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write runtime-config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", path},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `ACCOUNT_STORE_DIR='/state/accounts'`) {
		t.Fatalf("expected account path in stdout:\n%s", stdout.String())
	}
}

func TestRunBackupRestoreDrillRejectsSharedFileStoreParents(t *testing.T) {
	payload := `{
  "local_channel_id": 1,
  "visibility_mode": "whole_map",
  "visibility_radius": 0,
  "visibility_sector_size": 0,
  "persistence": {
    "login_ticket_store_dir": "/var/metin2/login-tickets",
    "account_store_dir": "/var/metin2/accounts",
    "static_actor_store_path": "/tmp/shared/static-actors.json",
    "interaction_store_path": "/tmp/shared/interaction-definitions.json",
    "item_template_store_path": "/tmp/shared/item-templates.json",
    "quest_state_store_path": "/tmp/shared/quest-state.json"
  },
  "database": {"configured": false, "dsn_configured": false}
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", "-"},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "shared") {
		t.Fatalf("expected shared-parent reason, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillRejectsBlankPersistencePath(t *testing.T) {
	payload := `{
  "local_channel_id": 1,
  "visibility_mode": "whole_map",
  "visibility_radius": 0,
  "visibility_sector_size": 0,
  "persistence": {
    "login_ticket_store_dir": "/var/metin2/login-tickets",
    "account_store_dir": " ",
    "static_actor_store_path": "/var/metin2/static-actors/static-actors.json",
    "interaction_store_path": "/var/metin2/interactions/interaction-definitions.json",
    "item_template_store_path": "/var/metin2/item-templates/item-templates.json",
    "quest_state_store_path": "/var/metin2/quest-state/quest-state.json"
  },
  "database": {"configured": false, "dsn_configured": false}
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", "-"},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
}

func TestRunBackupRestoreDrillRejectsMalformedAndOversizedInput(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{name: "invalid-json", payload: `{"persistence":`},
		{name: "invalid-utf8", payload: "{\x80"},
		{name: "null", payload: "null"},
		{name: "unknown-field", payload: `{"local_channel_id":1,"visibility_mode":"whole_map","visibility_radius":0,"visibility_sector_size":0,"persistence":{"login_ticket_store_dir":"/a","account_store_dir":"/b","static_actor_store_path":"/c/s.json","interaction_store_path":"/d/i.json","item_template_store_path":"/e/t.json","quest_state_store_path":"/f/q.json"},"database":{"configured":false,"dsn_configured":false},"extra":true}`},
		{name: "oversized", payload: strings.Repeat("a", maxRuntimeConfigBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"backup-restore-drill", "--runtime-config", "-"},
				strings.NewReader(tc.payload),
				&stdout,
				&stderr,
			)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
		})
	}
}

func TestRunBackupRestoreDrillUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing-flag", args: []string{"backup-restore-drill"}},
		{name: "unexpected-arg", args: []string{"backup-restore-drill", "--runtime-config", "-", "extra"}},
		{name: "unknown-flag", args: []string{"backup-restore-drill", "--nope", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader("{}"), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "backup-restore-drill usage:") {
				t.Fatalf("expected usage text, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsBackupRestoreDrill(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "backup-restore-drill") {
		t.Fatalf("expected usage to list backup-restore-drill, got %q", stderr.String())
	}
}
