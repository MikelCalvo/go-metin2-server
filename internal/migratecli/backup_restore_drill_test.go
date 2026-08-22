package migratecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validBackupRestoreRuntimeConfig() string {
	return `{
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
    "quest_state_store_path": "/var/metin2/quest-state/quest-state.json",
    "ground_item_store_path": "/var/metin2/ground-items/ground-items.json",
    "safebox_store_path": "/var/metin2/safebox/safebox.json"
  },
  "database": {
    "configured": false,
    "dsn_configured": false
  }
}`
}

func writeTempJSON(t *testing.T, name, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestRunBackupRestoreDrillPrintsLabRetentionCommands(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789deadbeef",
  "build_date": "2026-08-21T15:30:45Z"
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"backup-restore-drill",
			"--runtime-config", "-",
			"--build-info", buildInfoPath,
			"--ops-base-url", "http://127.0.0.1:6060",
			"--backup-base", "/var/metin2/backups",
		},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
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
		`# read-only printer: does not execute backup/restore`,
		`# crash-temps/cleanup mutates only hidden crash-temp residue after validate`,
		`docs/workflow/file-store-backup-restore-drill.md`,
		`docs/workflow/lab-deployment-topology.md`,
		`OPS='http://127.0.0.1:6060'`,
		`AUTH_OPS='http://127.0.0.1:6061'`,
		`BACKUPS_BASE='/var/metin2/backups'`,
		`COMMIT12='abcdef012345'`,
		`ACCOUNT_STORE_DIR='/var/metin2/accounts'`,
		`LOGIN_TICKET_STORE_DIR='/var/metin2/login-tickets'`,
		`ITEM_TEMPLATE_STORE_PATH='/var/metin2/item-templates/item-templates.json'`,
		`INTERACTION_STORE_PATH='/var/metin2/interactions/interaction-definitions.json'`,
		`STATIC_ACTOR_STORE_PATH='/var/metin2/static-actors/static-actors.json'`,
		`QUEST_STATE_STORE_PATH='/var/metin2/quest-state/quest-state.json'`,
		`GROUND_ITEM_STORE_PATH='/var/metin2/ground-items/ground-items.json'`,
		`SAFEBOX_STORE_PATH='/var/metin2/safebox/safebox.json'`,
		`TS=$(date -u +%Y%m%dT%H%M%SZ)`,
		`BASE="${BACKUPS_BASE}/${TS}-${COMMIT12}"`,
		`mkdir -p "$BASE"/accounts "$BASE"/login-tickets "$BASE"/item-templates "$BASE"/interaction-store "$BASE"/static-actors "$BASE"/quest-state "$BASE"/ground-items "$BASE"/safebox`,
		`curl -sS "$OPS/local/build-info" > "$BASE/gamed-build-info.json"`,
		`curl -sS "$AUTH_OPS/local/build-info" > "$BASE/authd-build-info.json"`,
		`curl -sS "$OPS/local/runtime-config" > "$BASE/runtime-config.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$BASE/persistence-status-before.json"`,
		`cat > "$BASE/notes.md" <<'EOF'`,
		`# Backup/restore drill notes`,
		`Do not paste DSNs, passwords, login keys, tickets, or executable SQL here.`,
		`curl -sS "$OPS/local/persistence/status" > "$BASE/persistence-status-after.json"`,
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
		`"$OPS/local/ground-item-store/validate"`,
		`"$OPS/local/ground-item-store/crash-temps/cleanup"`,
		`"$OPS/local/safebox-store/validate"`,
		`"$OPS/local/safebox-store/crash-temps/cleanup"`,
		`"$OPS/local/account-store/backup"`,
		`$BASE/accounts`,
		`"$OPS/local/login-tickets/backup"`,
		`$BASE/login-tickets`,
		`"$OPS/local/item-templates/backup"`,
		`$BASE/item-templates`,
		`"$OPS/local/interaction-store/backup"`,
		`$BASE/interaction-store`,
		`"$OPS/local/static-actors/backup"`,
		`$BASE/static-actors`,
		`"$OPS/local/quest-state/backup"`,
		`$BASE/quest-state`,
		`"$OPS/local/ground-item-store/backup"`,
		`$BASE/ground-items`,
		`"$OPS/local/safebox-store/backup"`,
		`$BASE/safebox`,
		`"$OPS/local/account-store/backup/validate"`,
		`"$OPS/local/safebox-store/backup/validate"`,
		`"$OPS/local/item-templates/restore"`,
		`"$OPS/local/ground-item-store/restore"`,
		`"$OPS/local/safebox-store/restore"`,
		`"$OPS/local/login-tickets/restore"`,
		`mv "$ACCOUNT_STORE_DIR"`,
		`mv "$(dirname "$ITEM_TEMPLATE_STORE_PATH")"`,
		`mv "$(dirname "$SAFEBOX_STORE_PATH")"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "memory://") {
		t.Fatalf("backup-restore-drill must not expose SQL or DSN text, got %s", body)
	}
	if strings.Contains(body, `/var/metin2/backups/drill`) || strings.Contains(body, `$BASE/account"`) || strings.Contains(body, `$BASE/interactions`) {
		t.Fatalf("expected lab retention paths, not legacy drill naming, got %s", body)
	}
	if strings.Contains(body, `BASE="${BASE}-${TS}"`) || strings.Contains(body, `TS=$(date +%Y%m%dT%H%M%S)`) {
		t.Fatalf("expected UTC commit12 retention tree, got legacy local-time suffix:\n%s", body)
	}

	idxGamedBuild := strings.Index(body, `curl -sS "$OPS/local/build-info" > "$BASE/gamed-build-info.json"`)
	idxAuthdBuild := strings.Index(body, `curl -sS "$AUTH_OPS/local/build-info" > "$BASE/authd-build-info.json"`)
	idxRuntimeRetain := strings.Index(body, `> "$BASE/runtime-config.json"`)
	idxStatusBefore := strings.Index(body, `> "$BASE/persistence-status-before.json"`)
	idxNotes := strings.Index(body, `cat > "$BASE/notes.md" <<'EOF'`)
	idxStoreValidate := strings.Index(body, `"$OPS/local/account-store/validate"`)
	idxCrashCleanup := strings.Index(body, `"$OPS/local/account-store/crash-temps/cleanup"`)
	idxBackup := strings.Index(body, `"$OPS/local/account-store/backup"`)
	idxBackupValidate := strings.Index(body, `"$OPS/local/account-store/backup/validate"`)
	idxRestore := strings.Index(body, `"$OPS/local/item-templates/restore"`)
	idxStatusAfter := strings.Index(body, `> "$BASE/persistence-status-after.json"`)
	if idxGamedBuild < 0 || idxAuthdBuild < 0 || idxRuntimeRetain < 0 || idxStatusBefore < 0 || idxNotes < 0 || idxStoreValidate < 0 || idxCrashCleanup < 0 || idxBackup < 0 || idxBackupValidate < 0 || idxRestore < 0 || idxStatusAfter < 0 {
		t.Fatalf("missing expected ordering markers in stdout:\n%s", body)
	}
	if !(idxGamedBuild < idxAuthdBuild && idxAuthdBuild < idxRuntimeRetain && idxRuntimeRetain < idxStatusBefore && idxStatusBefore < idxNotes && idxNotes < idxStoreValidate && idxStoreValidate < idxCrashCleanup && idxCrashCleanup < idxBackup && idxBackup < idxBackupValidate && idxBackupValidate < idxRestore && idxRestore < idxStatusAfter) {
		t.Fatalf("expected gamed/authd build-info -> runtime-config -> status-before -> notes -> validate -> cleanup -> backup -> backup validate -> restore -> status-after ordering, got idxs gamed=%d authd=%d runtime=%d before=%d notes=%d validate=%d cleanup=%d backup=%d backupValidate=%d restore=%d after=%d\n%s",
			idxGamedBuild, idxAuthdBuild, idxRuntimeRetain, idxStatusBefore, idxNotes, idxStoreValidate, idxCrashCleanup, idxBackup, idxBackupValidate, idxRestore, idxStatusAfter, body)
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

func TestRunBackupRestoreDrillReadsRegularFiles(t *testing.T) {
	runtimePath := writeTempJSON(t, "runtime-config.json", `{
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
    "quest_state_store_path": "/state/quests/quest-state.json",
    "ground_item_store_path": "/state/ground-items/ground-items.json",
    "safebox_store_path": "/state/safebox/safebox.json"
  },
  "database": {"configured": false, "dsn_configured": false}
}`)
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"deadbeefcafe","build_date":"2026-08-21T15:30:45Z"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", runtimePath, "--build-info", buildInfoPath},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	if !strings.Contains(body, `ACCOUNT_STORE_DIR='/state/accounts'`) {
		t.Fatalf("expected account path in stdout:\n%s", body)
	}
	if !strings.Contains(body, `COMMIT12='deadbeefcafe'`) {
		t.Fatalf("expected short commit in stdout:\n%s", body)
	}
	if !strings.Contains(body, `BACKUPS_BASE='/var/metin2/backups'`) {
		t.Fatalf("expected default backups base in stdout:\n%s", body)
	}
}

func TestRunBackupRestoreDrillRejectsSharedFileStoreParents(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)
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
    "quest_state_store_path": "/tmp/shared/quest-state.json",
    "ground_item_store_path": "/tmp/ground-items/ground-items.json",
    "safebox_store_path": "/tmp/safebox/safebox.json"
  },
  "database": {"configured": false, "dsn_configured": false}
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", "-", "--build-info", buildInfoPath},
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
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)
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
    "quest_state_store_path": "/var/metin2/quest-state/quest-state.json",
    "ground_item_store_path": "/var/metin2/ground-items/ground-items.json",
    "safebox_store_path": "/var/metin2/safebox/safebox.json"
  },
  "database": {"configured": false, "dsn_configured": false}
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", "-", "--build-info", buildInfoPath},
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

func TestRunBackupRestoreDrillRejectsBlankCommit(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"   ","build_date":"2026-08-21T15:30:45Z"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", "-", "--build-info", buildInfoPath},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "commit") {
		t.Fatalf("expected commit reason, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillRejectsDualStdin(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"backup-restore-drill", "--runtime-config", "-", "--build-info", "-"},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stdin") {
		t.Fatalf("expected dual-stdin reason, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillRejectsRelativeBackupBase(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"backup-restore-drill",
			"--runtime-config", "-",
			"--build-info", buildInfoPath,
			"--backup-base", "relative/backups",
		},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
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

func TestRunBackupRestoreDrillRejectsInvalidOpsBaseURL(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"backup-restore-drill",
			"--runtime-config", "-",
			"--build-info", buildInfoPath,
			"--ops-base-url", "ftp://127.0.0.1:6060",
		},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "ops-base-url") {
		t.Fatalf("expected ops-base-url reason, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillRejectsInvalidAuthdOpsBaseURL(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"backup-restore-drill",
			"--runtime-config", "-",
			"--build-info", buildInfoPath,
			"--authd-ops-base-url", "ftp://127.0.0.1:6061",
		},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "authd-ops-base-url") {
		t.Fatalf("expected authd-ops-base-url reason, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillHonorsCustomAuthdOpsBaseURL(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"backup-restore-drill",
			"--runtime-config", "-",
			"--build-info", buildInfoPath,
			"--authd-ops-base-url", "http://127.0.0.1:7061",
		},
		strings.NewReader(validBackupRestoreRuntimeConfig()),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	if !strings.Contains(body, `AUTH_OPS='http://127.0.0.1:7061'`) {
		t.Fatalf("expected custom AUTH_OPS in stdout:\n%s", body)
	}
	if !strings.Contains(body, `curl -sS "$AUTH_OPS/local/build-info" > "$BASE/authd-build-info.json"`) {
		t.Fatalf("expected authd build-info retain in stdout:\n%s", body)
	}
}

func TestRunBackupRestoreDrillRejectsMalformedAndOversizedInput(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`)
	cases := []struct {
		name    string
		payload string
	}{
		{name: "invalid-json", payload: `{"persistence":`},
		{name: "invalid-utf8", payload: "{\x80"},
		{name: "null", payload: "null"},
		{name: "unknown-field", payload: `{"local_channel_id":1,"visibility_mode":"whole_map","visibility_radius":0,"visibility_sector_size":0,"persistence":{"login_ticket_store_dir":"/a","account_store_dir":"/b","static_actor_store_path":"/c/s.json","interaction_store_path":"/d/i.json","item_template_store_path":"/e/t.json","quest_state_store_path":"/f/q.json","ground_item_store_path":"/g/gi.json","safebox_store_path":"/h/s.json"},"database":{"configured":false,"dsn_configured":false},"extra":true}`},
		{name: "oversized", payload: strings.Repeat("a", maxRuntimeConfigBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"backup-restore-drill", "--runtime-config", "-", "--build-info", buildInfoPath},
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
		{name: "missing-build-info", args: []string{"backup-restore-drill", "--runtime-config", "-"}},
		{name: "unexpected-arg", args: []string{"backup-restore-drill", "--runtime-config", "-", "--build-info", "-", "extra"}},
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
			if !strings.Contains(stderr.String(), "--build-info") || !strings.Contains(stderr.String(), "/var/metin2/backups") {
				t.Fatalf("expected usage to mention --build-info and /var/metin2/backups default, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), "--authd-ops-base-url") {
				t.Fatalf("expected usage to list --authd-ops-base-url, got %q", stderr.String())
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
