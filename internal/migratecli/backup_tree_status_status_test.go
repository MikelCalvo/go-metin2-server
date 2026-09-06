package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type backupTreeStatusStatusGot struct {
	Format                 string            `json:"format"`
	Present                bool              `json:"present"`
	BackupTreeStatusSHA256 string            `json:"backup_tree_status_sha256,omitempty"`
	Status                 *backupTreeStatus `json:"status,omitempty"`
}

func TestRunBackupTreeStatusStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-backup-tree-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"backup-tree-status-status", "--backup-tree-status", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing backup-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing backup-tree-status-status not to write stderr, got %q", stderr.String())
	}
	var got backupTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing backup-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-backup-tree-status-status-v1" || got.Present || got.Status != nil || got.BackupTreeStatusSHA256 != "" {
		t.Fatalf("unexpected missing backup-tree-status-status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusStatusReadsValidAbsentInnerSnapshot(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustCaptureBackupTreeStatusJSON(t, filepath.Join(t.TempDir(), "missing-backup-tree"))
	statusPath := filepath.Join(t.TempDir(), "backup-tree-status.json")
	mustWriteFile(t, statusPath, raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status-status", "--backup-tree-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected absent-inner backup-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on absent-inner success, got %q", stderr.String())
	}
	var got backupTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode absent-inner backup-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-backup-tree-status-status-v1" || !got.Present || got.Status == nil {
		t.Fatalf("unexpected absent-inner envelope: %#v", got)
	}
	if got.BackupTreeStatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected backup_tree_status_sha256: got %s want %s", got.BackupTreeStatusSHA256, sha256Hex(raw))
	}
	if got.Status.Format != backupTreeStatusFormat || got.Status.Present || got.Status.BackupTree != "" || len(got.Status.Stores) != 0 {
		t.Fatalf("unexpected inner absent snapshot: %#v", got.Status)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusStatusReadsValidPresentSnapshotWithoutWalkingTree(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T180000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, true)
	raw := mustCaptureBackupTreeStatusJSON(t, tree)
	if err := os.RemoveAll(tree); err != nil {
		t.Fatalf("remove original backup-tree: %v", err)
	}
	statusPath := filepath.Join(t.TempDir(), "backup-tree-status.json")
	mustWriteFile(t, statusPath, raw)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status-status", "--backup-tree-status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected present backup-tree-status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got backupTreeStatusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode present backup-tree-status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-backup-tree-status-status-v1" || !got.Present || got.Status == nil || got.BackupTreeStatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected present envelope: %#v", got)
	}
	if got.Status.Format != backupTreeStatusFormat || !got.Status.Present || got.Status.StoreCount != 8 || got.Status.StorePresentCount != 8 || got.Status.StoresComplete == nil || !*got.Status.StoresComplete {
		t.Fatalf("unexpected inner present snapshot: %#v", got.Status)
	}
	body := stdout.String()
	for _, forbidden := range []string{
		"mkmk", "MkmkWar", "CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password=",
		`".account-crashed.json"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("backup-tree-status-status must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("backup-tree-status-status must not open a database target, got events %#v", events)
	}
}

func TestRunBackupTreeStatusStatusRejectsInconsistentInnerSnapshots(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	disableBackupTreeStatusDurableSync(t)
	tree := filepath.Join(t.TempDir(), "backups", "20260906T181000Z-abcdef012345")
	mustMaterializeCompleteBackupTree(t, tree, false)
	raw := mustCaptureBackupTreeStatusJSON(t, tree)
	var base backupTreeStatus
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatalf("decode captured backup-tree-status: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*backupTreeStatus)
		want   string
	}{
		{
			name: "kind-order",
			mutate: func(status *backupTreeStatus) {
				status.Stores[0], status.Stores[1] = status.Stores[1], status.Stores[0]
			},
			want: "kind",
		},
		{
			name: "stores-complete-drift",
			mutate: func(status *backupTreeStatus) {
				status.StoresComplete = boolPtr(false)
			},
			want: "stores_complete",
		},
		{
			name: "present-invalid",
			mutate: func(status *backupTreeStatus) {
				status.Stores[0].Valid = false
			},
			want: "valid",
		},
		{
			name: "wrong-kind-count",
			mutate: func(status *backupTreeStatus) {
				status.Stores[1].AccountCount = 1
			},
			want: "account_count",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := base
			status.Stores = append([]backupTreeStoreStatus(nil), base.Stores...)
			tc.mutate(&status)
			mutated, err := json.Marshal(status)
			if err != nil {
				t.Fatalf("marshal mutated status: %v", err)
			}
			statusPath := filepath.Join(t.TempDir(), "backup-tree-status.json")
			mustWriteFile(t, statusPath, mutated)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"backup-tree-status-status", "--backup-tree-status", statusPath}, nil, &stdout, &stderr)
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

func TestRunBackupTreeStatusStatusRequireStoresCompleteFailClosedOnAbsentPathOrInnerPresentFalse(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-backup-tree-status.json")
	absentInner := mustCaptureBackupTreeStatusJSON(t, filepath.Join(t.TempDir(), "missing-backup-tree"))
	absentPath := filepath.Join(t.TempDir(), "backup-tree-status.json")
	mustWriteFile(t, absentPath, absentInner)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing-path",
			args: []string{"backup-tree-status-status", "--backup-tree-status", missing, "--require-stores-complete"},
			want: "require-stores-complete",
		},
		{
			name: "inner-absent",
			args: []string{"backup-tree-status-status", "--backup-tree-status", absentPath, "--require-stores-complete"},
			want: "require-stores-complete",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected require-gate to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on require-gate failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunBackupTreeStatusStatusRejectsSymlinkOversizedUnknownFieldAndWrongFormat(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	targetPath := filepath.Join(dir, "target-backup-tree-status.json")
	mustWriteFile(t, targetPath, []byte("{}\n"))
	symlinkPath := filepath.Join(dir, "backup-tree-status.json")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("create symlink backup-tree-status: %v", err)
	}

	oversizedPath := filepath.Join(dir, "oversized-backup-tree-status.json")
	mustWriteFile(t, oversizedPath, bytes.Repeat([]byte("a"), 128*1024+1))

	unknownFieldPath := filepath.Join(dir, "unknown-field-backup-tree-status.json")
	mustWriteFile(t, unknownFieldPath, []byte(`{"format":"go-metin2-backup-tree-status-v1","present":false,"extra":true}`+"\n"))

	wrongFormatPath := filepath.Join(dir, "wrong-format-backup-tree-status.json")
	mustWriteFile(t, wrongFormatPath, []byte(`{"format":"go-metin2-backup-tree-status-status-v1","present":false}`+"\n"))

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "symlink", path: symlinkPath, want: "must not be a symlink"},
		{name: "oversized", path: oversizedPath, want: "exceeds"},
		{name: "unknown-field", path: unknownFieldPath, want: "unknown field"},
		{name: "wrong-format", path: wrongFormatPath, want: "format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"backup-tree-status-status", "--backup-tree-status", tc.path}, nil, &stdout, &stderr)
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
				t.Fatalf("backup-tree-status-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunBackupTreeStatusStatusRejectsUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-flag", args: []string{"backup-tree-status-status"}, want: "--backup-tree-status"},
		{name: "unexpected-arg", args: []string{"backup-tree-status-status", "--backup-tree-status", "/tmp/x.json", "extra"}, want: "unexpected backup-tree-status-status argument"},
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
			if !strings.Contains(stderr.String(), "backup-tree-status-status usage:") {
				t.Fatalf("expected backup-tree-status-status usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunBackupTreeStatusStatusUsageListsRequireFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--require-stores-complete") {
		t.Fatalf("expected usage to list --require-stores-complete, got %q", stderr.String())
	}
}

func TestRunHelpListsBackupTreeStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "backup-tree-status-status") {
		t.Fatalf("expected help to list backup-tree-status-status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "backup-tree-status-status usage:") {
		t.Fatalf("expected backup-tree-status-status usage block, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsBackupTreeStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "backup-tree-status-status") {
		t.Fatalf("expected usage to mention backup-tree-status-status, got %q", stderr.String())
	}
}

func TestRunBackupRestoreDrillPrintsBackupTreeStatusStatusRedirect(t *testing.T) {
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
	wantStatus := `metin2-migrate backup-tree-status --backup-tree "$BASE"`
	wantStatusStatus := `metin2-migrate backup-tree-status-status --backup-tree-status "$BASE/backup-tree-status.json"`
	wantRedirect := `> "$BASE/backup-tree-status-status.json"`
	if !strings.Contains(body, wantStatusStatus) || !strings.Contains(body, wantRedirect) {
		t.Fatalf("expected backup-tree-status-status redirect in printed drill:\n%s", body)
	}
	idxStatus := strings.Index(body, wantStatus)
	idxStatusStatus := strings.Index(body, wantStatusStatus)
	idxAside := strings.Index(body, `mv "$ACCOUNT_STORE_DIR"`)
	if idxStatus < 0 || idxStatusStatus < 0 || idxAside < 0 {
		t.Fatalf("missing ordering markers in printed drill:\n%s", body)
	}
	if !(idxStatus < idxStatusStatus && idxStatusStatus < idxAside) {
		t.Fatalf("expected backup-tree-status -> backup-tree-status-status -> aside-rename, got status=%d status-status=%d aside=%d", idxStatus, idxStatusStatus, idxAside)
	}
}

func mustCaptureBackupTreeStatusJSON(t *testing.T, tree string) []byte {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"backup-tree-status", "--backup-tree", tree}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("backup-tree-status capture: exit=%d stderr=%q", code, stderr.String())
	}
	return stdout.Bytes()
}
