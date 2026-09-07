package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type persistenceStatusStatusGot struct {
	Format                  string          `json:"format"`
	Present                 bool            `json:"present"`
	PersistenceStatusSHA256 string          `json:"persistence_status_sha256,omitempty"`
	Status                  json.RawMessage `json:"status,omitempty"`
}

func validDrainedPersistenceStatusJSON() []byte {
	return []byte(`{
  "ok": true,
  "live_selected_character_count": 0,
  "account_store": {
    "path": "/state/accounts",
    "valid": true,
    "summary": {"account_count": 1, "character_count": 1, "logins": ["mkmk"]},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "login_ticket_store": {
    "path": "/state/tickets",
    "valid": true,
    "summary": {"ticket_count": 0, "character_count": 0, "logins": [], "login_keys": []},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "item_template_store": {
    "path": "/state/item-templates.json",
    "valid": true,
    "summary": {"template_count": 1, "vnums": [27001]},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "static_actor_store": {
    "path": "/state/static-actors.json",
    "valid": true,
    "summary": {"actor_count": 1, "actor_ids": [7], "actor_names": ["TrainingDummy"]},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "interaction_store": {
    "path": "/state/interaction-definitions.json",
    "valid": true,
    "summary": {"definition_count": 1, "definition_keys": ["info:lore:alchemist"]},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "quest_state_store": {
    "path": "/state/quest-state.json",
    "valid": true,
    "summary": {"flag_count": 0, "characters": [], "quest_refs": [], "flag_keys": []},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "ground_item_store": {
    "path": "/state/ground-items.json",
    "valid": true,
    "summary": {"ground_item_count": 0, "item_shaped_count": 0, "gold_shaped_count": 0, "vids": []},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  },
  "safebox_store": {
    "path": "/state/safebox.json",
    "valid": true,
    "summary": {"character_count": 1, "cell_count": 2, "logins": ["mkmk"], "character_keys": ["mkmk:1"]},
    "backup_manifest": {"present": false},
    "restore_blocked_by_live_sessions": false
  }
}
`)
}

func invalidAccountPersistenceStatusJSON() []byte {
	raw := validDrainedPersistenceStatusJSON()
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		panic(err)
	}
	snapshot["ok"] = false
	account := snapshot["account_store"].(map[string]any)
	account["valid"] = false
	account["error"] = "account snapshot is corrupt"
	account["summary"] = map[string]any{"account_count": 0, "character_count": 0, "logins": []string{}}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		panic(err)
	}
	return append(encoded, '\n')
}

func mustWritePersistenceStatusFile(t *testing.T, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "persistence-status.json")
	mustWriteFile(t, path, raw)
	return path
}

func TestRunPersistenceStatusStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-persistence-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"persistence-status-status", "--persistence-status", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing persistence-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing persistence-status-status not to write stderr, got %q", stderr.String())
	}
	var got persistenceStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing persistence-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-persistence-status-status-v1" || got.Present || len(got.Status) != 0 || got.PersistenceStatusSHA256 != "" {
		t.Fatalf("unexpected missing persistence-status-status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("persistence-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunPersistenceStatusStatusReadsValidDrainedSnapshot(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := validDrainedPersistenceStatusJSON()
	statusPath := mustWritePersistenceStatusFile(t, raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"persistence-status-status", "--persistence-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected valid persistence-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on valid success, got %q", stderr.String())
	}
	var got persistenceStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode valid persistence-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-persistence-status-status-v1" || !got.Present || len(got.Status) == 0 {
		t.Fatalf("unexpected valid envelope: %#v", got)
	}
	if got.PersistenceStatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected persistence_status_sha256: got %s want %s", got.PersistenceStatusSHA256, sha256Hex(raw))
	}
	var inner struct {
		OK                         bool `json:"ok"`
		LiveSelectedCharacterCount int  `json:"live_selected_character_count"`
	}
	if err := json.Unmarshal(got.Status, &inner); err != nil {
		t.Fatalf("decode inner status: %v\ninner:\n%s", err, got.Status)
	}
	if !inner.OK || inner.LiveSelectedCharacterCount != 0 {
		t.Fatalf("unexpected inner snapshot: %#v", inner)
	}
	body := stdout.String()
	if !strings.Contains(body, `"logins": [`) && !strings.Contains(body, `"logins":[`) {
		t.Fatalf("expected inner status to echo identity slices, got %s", body)
	}
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("persistence-status-status must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("persistence-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunPersistenceStatusStatusAllowsUngatedOkFalse(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	statusPath := mustWritePersistenceStatusFile(t, invalidAccountPersistenceStatusJSON())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"persistence-status-status", "--persistence-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected ungated ok=false to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got persistenceStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode ungated ok=false JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || !strings.Contains(string(got.Status), `"ok": false`) && !strings.Contains(string(got.Status), `"ok":false`) {
		t.Fatalf("expected inner ok=false to be echoed, got %s", stdout.String())
	}
}

func TestRunPersistenceStatusStatusRequireOkFailClosed(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-persistence-status.json")
	invalidPath := mustWritePersistenceStatusFile(t, invalidAccountPersistenceStatusJSON())

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing-path",
			args: []string{"persistence-status-status", "--persistence-status", missing, "--require-ok"},
			want: "--require-ok failed: persistence-status is absent",
		},
		{
			name: "ok-false",
			args: []string{"persistence-status-status", "--persistence-status", invalidPath, "--require-ok"},
			want: "--require-ok failed: ok=false",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected require-ok to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on require-ok failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunPersistenceStatusStatusRequireDrainedFailClosed(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := validDrainedPersistenceStatusJSON()
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	snapshot["live_selected_character_count"] = 1
	for _, key := range []string{
		"account_store", "login_ticket_store", "item_template_store", "static_actor_store",
		"interaction_store", "quest_state_store", "ground_item_store", "safebox_store",
	} {
		store := snapshot[key].(map[string]any)
		store["restore_blocked_by_live_sessions"] = true
	}
	mutated, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal live-session snapshot: %v", err)
	}
	statusPath := mustWritePersistenceStatusFile(t, append(mutated, '\n'))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"persistence-status-status", "--persistence-status", statusPath, "--require-drained"}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected require-drained to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-drained failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-drained failed: live_selected_character_count=1") {
		t.Fatalf("expected require-drained/count guidance, got %q", stderr.String())
	}
}

func TestRunPersistenceStatusStatusRequireNoCrashTempsFailClosed(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := validDrainedPersistenceStatusJSON()
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	account := snapshot["account_store"].(map[string]any)
	summary := account["summary"].(map[string]any)
	summary["crash_temp_count"] = 1
	summary["crash_temp_files"] = []string{".account-crashed.json"}
	mutated, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal crash-temp snapshot: %v", err)
	}
	statusPath := mustWritePersistenceStatusFile(t, append(mutated, '\n'))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"persistence-status-status", "--persistence-status", statusPath, "--require-no-crash-temps"}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected require-no-crash-temps to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-no-crash-temps failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-no-crash-temps failed: crash_temp_count>0 on account_store") {
		t.Fatalf("expected require-no-crash-temps/account_store guidance, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), ".account-crashed.json") {
		t.Fatalf("require-no-crash-temps must not expose crash-temp filenames, got %q", stderr.String())
	}
}

func TestRunPersistenceStatusStatusRequireOkFirstOnAbsentPath(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-persistence-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"persistence-status-status",
		"--persistence-status", missing,
		"--require-ok",
		"--require-drained",
		"--require-no-crash-temps",
	}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected combined require gates to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on combined require-gate failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-ok failed: persistence-status is absent") {
		t.Fatalf("expected --require-ok to be reported first, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "--require-drained") || strings.Contains(stderr.String(), "--require-no-crash-temps") {
		t.Fatalf("expected later gates not to be reported after --require-ok, got %q", stderr.String())
	}
}

func TestRunPersistenceStatusStatusRejectsInconsistentInnerSnapshots(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	base := validDrainedPersistenceStatusJSON()
	var snapshot map[string]any
	if err := json.Unmarshal(base, &snapshot); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{
			name: "ok-valid-mismatch",
			mutate: func(status map[string]any) {
				account := status["account_store"].(map[string]any)
				account["valid"] = false
				account["error"] = "corrupt"
			},
			want: "ok",
		},
		{
			name: "restore-blocked-mismatch",
			mutate: func(status map[string]any) {
				account := status["account_store"].(map[string]any)
				account["restore_blocked_by_live_sessions"] = true
			},
			want: "restore_blocked_by_live_sessions",
		},
		{
			name: "account-count-logins-mismatch",
			mutate: func(status map[string]any) {
				account := status["account_store"].(map[string]any)
				summary := account["summary"].(map[string]any)
				summary["account_count"] = 2
			},
			want: "account_count",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cloned := map[string]any{}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatalf("marshal base: %v", err)
			}
			if err := json.Unmarshal(raw, &cloned); err != nil {
				t.Fatalf("clone base: %v", err)
			}
			tc.mutate(cloned)
			mutated, err := json.Marshal(cloned)
			if err != nil {
				t.Fatalf("marshal mutated status: %v", err)
			}
			statusPath := mustWritePersistenceStatusFile(t, mutated)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"persistence-status-status", "--persistence-status", statusPath}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected inconsistent snapshot to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on inconsistent snapshot, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to mention %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunPersistenceStatusStatusRejectsWrappingStatusStatusFormat(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	statusPath := mustWritePersistenceStatusFile(t, []byte(`{"format":"go-metin2-persistence-status-status-v1","present":false}`+"\n"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"persistence-status-status", "--persistence-status", statusPath}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected wrapping status-status format to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on wrapping format, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "format") && !strings.Contains(stderr.String(), "unknown field") {
		t.Fatalf("expected format/unknown-field guidance, got %q", stderr.String())
	}
}

func TestRunPersistenceStatusStatusRejectsSymlinkOversizedUnknownField(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	targetPath := filepath.Join(dir, "target-persistence-status.json")
	mustWriteFile(t, targetPath, validDrainedPersistenceStatusJSON())
	symlinkPath := filepath.Join(dir, "persistence-status.json")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("create symlink persistence-status: %v", err)
	}

	oversizedPath := filepath.Join(dir, "oversized-persistence-status.json")
	mustWriteFile(t, oversizedPath, bytes.Repeat([]byte("a"), 128*1024+1))

	unknownFieldPath := filepath.Join(dir, "unknown-field-persistence-status.json")
	mustWriteFile(t, unknownFieldPath, []byte(`{"ok":true,"live_selected_character_count":0,"extra":true}`+"\n"))

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "symlink", path: symlinkPath, want: "must not be a symlink"},
		{name: "oversized", path: oversizedPath, want: "exceeds"},
		{name: "unknown-field", path: unknownFieldPath, want: "unknown field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"persistence-status-status", "--persistence-status", tc.path}, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected %s to fail closed, exit=%d stdout=%q stderr=%q", tc.name, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on %s rejection, got %q", tc.name, stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("persistence-status-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunPersistenceStatusStatusRejectsUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-flag", args: []string{"persistence-status-status"}, want: "--persistence-status"},
		{name: "unexpected-arg", args: []string{"persistence-status-status", "--persistence-status", "/tmp/x.json", "extra"}, want: "unexpected persistence-status-status argument"},
		{name: "unknown-flag", args: []string{"persistence-status-status", "--persistence-status", "/tmp/x.json", "--nope"}, want: "flag provided but not defined"},
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
			if !strings.Contains(stderr.String(), "persistence-status-status usage:") {
				t.Fatalf("expected persistence-status-status usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunPersistenceStatusStatusUsageListsRequireFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"persistence-status-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	for _, want := range []string{"--persistence-status", "--require-ok", "--require-drained", "--require-no-crash-temps"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected usage to list %s, got %q", want, stderr.String())
		}
	}
}

func TestRunHelpListsPersistenceStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "persistence-status-status") {
		t.Fatalf("expected help to list persistence-status-status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "persistence-status-status usage:") {
		t.Fatalf("expected persistence-status-status usage block, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "backup-tree-status-status") || !strings.Contains(stdout.String(), "backup-restore-drill") {
		t.Fatalf("expected usage to list persistence-status-status beside backup-tree-status-status / backup-restore-drill, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsPersistenceStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "persistence-status-status") {
		t.Fatalf("expected usage to mention persistence-status-status, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillPrintsPersistenceStatusStatusRedirect(t *testing.T) {
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

	body := stdout.String()
	wantBeforeStatus := `metin2-migrate persistence-status-status --persistence-status "$BASE/persistence-status-before.json"`
	wantBeforeRedirect := `> "$BASE/persistence-status-before-status.json"`
	wantAfterStatus := `metin2-migrate persistence-status-status --persistence-status "$BASE/persistence-status-after.json"`
	wantAfterRedirect := `> "$BASE/persistence-status-after-status.json"`
	if !strings.Contains(body, wantBeforeStatus) || !strings.Contains(body, wantBeforeRedirect) {
		t.Fatalf("expected ungated before persistence-status-status redirect in printed drill:\n%s", body)
	}
	if !strings.Contains(body, wantAfterStatus) || !strings.Contains(body, wantAfterRedirect) {
		t.Fatalf("expected gated after persistence-status-status redirect in printed drill:\n%s", body)
	}

	idxBeforeJSON := strings.Index(body, `> "$BASE/persistence-status-before.json"`)
	idxBeforeStatus := strings.Index(body, wantBeforeStatus)
	idxBeforeRedirect := strings.Index(body, wantBeforeRedirect)
	idxGamedLog := strings.Index(body, `cp -p "$GAMED_LOG" "$BASE/gamed.log"`)
	idxAfterJSON := strings.Index(body, `> "$BASE/persistence-status-after.json"`)
	idxAfterStatus := strings.Index(body, wantAfterStatus)
	idxAfterRedirect := strings.Index(body, wantAfterRedirect)
	if idxBeforeJSON < 0 || idxBeforeStatus < 0 || idxBeforeRedirect < 0 || idxGamedLog < 0 || idxAfterJSON < 0 || idxAfterStatus < 0 || idxAfterRedirect < 0 {
		t.Fatalf("missing persistence-status-status ordering markers in printed drill:\n%s", body)
	}
	if !(idxBeforeJSON < idxBeforeStatus && idxBeforeStatus < idxBeforeRedirect && idxBeforeRedirect < idxGamedLog) {
		t.Fatalf("expected before.json -> before-status-status -> daemon logs, got beforeJSON=%d beforeStatus=%d beforeRedirect=%d gamedLog=%d", idxBeforeJSON, idxBeforeStatus, idxBeforeRedirect, idxGamedLog)
	}
	if !(idxAfterJSON < idxAfterStatus && idxAfterStatus < idxAfterRedirect) {
		t.Fatalf("expected after.json -> after-status-status, got afterJSON=%d afterStatus=%d afterRedirect=%d", idxAfterJSON, idxAfterStatus, idxAfterRedirect)
	}
	afterBlock := body[idxAfterStatus:]
	if !strings.Contains(afterBlock, "--require-ok") || !strings.Contains(afterBlock, "--require-drained") {
		t.Fatalf("expected after-status-status to print --require-ok --require-drained, got %s", afterBlock)
	}
	beforeBlock := body[idxBeforeStatus:idxGamedLog]
	if strings.Contains(beforeBlock, "--require-no-crash-temps") {
		t.Fatalf("before persistence-status-status must not print --require-no-crash-temps, got %s", beforeBlock)
	}
	end := idxAfterRedirect + len(wantAfterRedirect)
	if end > len(body) {
		end = len(body)
	}
	afterCommand := body[idxAfterStatus:end]
	if strings.Contains(afterCommand, "--require-no-crash-temps") {
		t.Fatalf("after persistence-status-status must not print --require-no-crash-temps, got %s", afterCommand)
	}
}
