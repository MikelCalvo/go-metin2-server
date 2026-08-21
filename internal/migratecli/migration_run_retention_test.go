package migratecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMigrationRunRetentionPrintsLabTreeCommands(t *testing.T) {
	payload := `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789deadbeef",
  "build_date": "2026-08-21T15:30:45Z"
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--ops-base-url", "http://127.0.0.1:6060",
			"--migration-runs-base", "/var/metin2/migration-runs",
			"--target-version", "latest",
			"--lock-file", "migration-apply.lock",
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
		`# read-only printer: does not execute migration apply/rollback`,
		`OPS='http://127.0.0.1:6060'`,
		`RUNS_BASE='/var/metin2/migration-runs'`,
		`TARGET_VERSION='latest'`,
		`LOCK_FILE='migration-apply.lock'`,
		`COMMIT12='abcdef012345'`,
		`RUN="${RUNS_BASE}/${TS}-${COMMIT12}"`,
		`mkdir -p "$RUN"`,
		`metin2-migrate catalog > "$RUN/migration-catalog.json"`,
		`metin2-migrate ledger-snapshot`,
		`> "$RUN/ledger-snapshot.json"`,
		`metin2-migrate ledger-snapshot-status`,
		`> "$RUN/ledger-snapshot-status.json"`,
		`metin2-migrate plan-artifact`,
		`> "$RUN/migration-plan-artifact.json"`,
		`metin2-migrate plan-artifact-status`,
		`> "$RUN/plan-artifact-status.json"`,
		`metin2-migrate apply-preflight`,
		`> "$RUN/apply-preflight.json"`,
		`metin2-migrate apply-preflight-status`,
		`> "$RUN/apply-preflight-status.json"`,
		`metin2-migrate apply`,
		`--lock-file "$RUN/$LOCK_FILE"`,
		`--audit-file "$RUN/migration-apply-audit.json"`,
		`metin2-migrate apply-audit-status`,
		`> "$RUN/apply-audit-status.json"`,
		`metin2-migrate status`,
		`> "$RUN/post-apply-status.json"`,
		`metin2-migrate apply-lock-status --lock-file "$RUN/$LOCK_FILE" > "$RUN/apply-lock-status.json"`,
		`metin2-migrate apply-lock-aside --lock-file "$RUN/$LOCK_FILE" --i-confirm-lab-aside-rename > "$RUN/apply-lock-aside.json"`,
		`curl -sS "$OPS/local/build-info" > "$RUN/gamed-build-info.json"`,
		`curl -sS "$OPS/local/db/migrations/status" > "$RUN/daemon-migrations-status.json"`,
		`# require operator-exported DRIVER/DSN; printer never embeds a DSN`,
		`docs/workflow/migration-apply-runbook.md`,
		`docs/workflow/lab-deployment-topology.md`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "password=") || strings.Contains(body, "memory://") {
		t.Fatalf("migration-run-retention must not expose SQL or concrete DSN text, got %s", body)
	}
	idxMkdir := strings.Index(body, `mkdir -p "$RUN"`)
	idxCatalog := strings.Index(body, `metin2-migrate catalog > "$RUN/migration-catalog.json"`)
	idxPreflight := strings.Index(body, `> "$RUN/apply-preflight.json"`)
	idxApply := strings.Index(body, `metin2-migrate apply \`)
	idxPostStatus := strings.Index(body, `> "$RUN/post-apply-status.json"`)
	idxLockStatus := strings.Index(body, `apply-lock-status --lock-file "$RUN/$LOCK_FILE"`)
	if idxMkdir < 0 || idxCatalog < 0 || idxPreflight < 0 || idxApply < 0 || idxPostStatus < 0 || idxLockStatus < 0 {
		t.Fatalf("missing expected ordering markers in stdout:\n%s", body)
	}
	if !(idxMkdir < idxCatalog && idxCatalog < idxPreflight && idxPreflight < idxApply && idxApply < idxPostStatus) {
		t.Fatalf("expected mkdir -> catalog -> preflight -> apply -> post-status ordering, got idxs mkdir=%d catalog=%d preflight=%d apply=%d post=%d\n%s",
			idxMkdir, idxCatalog, idxPreflight, idxApply, idxPostStatus, body)
	}
}

func TestRunMigrationRunRetentionReadsRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build-info.json")
	payload := `{"version":"v0.1.0","commit":"deadbeefcafe","build_date":"2026-08-21T15:30:45Z"}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write build-info: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"migration-run-retention", "--build-info", path},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `COMMIT12='deadbeefcafe'`) {
		t.Fatalf("expected short commit in stdout:\n%s", stdout.String())
	}
}

func TestRunMigrationRunRetentionRejectsBlankCommit(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"   ","build_date":"2026-08-21T15:30:45Z"}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"migration-run-retention", "--build-info", "-"},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "commit") {
		t.Fatalf("expected commit error, got %q", stderr.String())
	}
}

func TestRunMigrationRunRetentionRejectsRelativeRunsBase(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--migration-runs-base", "relative/runs",
		},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
	}
}

func TestRunMigrationRunRetentionRejectsMalformedAndOversizedInput(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{name: "malformed", payload: `{"version":`},
		{name: "null", payload: `null`},
		{name: "invalid-utf8", payload: "{\x80"},
		{name: "oversized", payload: `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z","padding":"` + strings.Repeat("x", 64*1024) + `"}`},
		{name: "unknown-field", payload: `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z","extra":true}`},
		{name: "trailing-json", payload: `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"migration-run-retention", "--build-info", "-"},
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

func TestRunMigrationRunRetentionRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "build-info.json")
	link := filepath.Join(dir, "build-info.link")
	if err := os.WriteFile(target, []byte(`{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"migration-run-retention", "--build-info", link},
		nil,
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("expected symlink error, got %q", stderr.String())
	}
}

func TestRunMigrationRunRetentionUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing-flag", args: []string{"migration-run-retention"}},
		{name: "unexpected-arg", args: []string{"migration-run-retention", "--build-info", "-", "extra"}},
		{name: "unknown-flag", args: []string{"migration-run-retention", "--nope", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(`{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "migration-run-retention usage:") {
				t.Fatalf("expected usage text, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsMigrationRunRetention(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "migration-run-retention") {
		t.Fatalf("expected usage to list migration-run-retention, got %q", stderr.String())
	}
}
