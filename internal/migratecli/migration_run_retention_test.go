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
		`AUTH_OPS='http://127.0.0.1:6061'`,
		`RUNS_BASE='/var/metin2/migration-runs'`,
		`GAMED_LOG='/var/log/metin2/gamed.log'`,
		`AUTHD_LOG='/var/log/metin2/authd.log'`,
		`TARGET_VERSION='latest'`,
		`LOCK_FILE='migration-apply.lock'`,
		`COMMIT12='abcdef012345'`,
		`RUN="${RUNS_BASE}/${TS}-${COMMIT12}"`,
		`mkdir -p "$RUN"`,
		`curl -sS "$OPS/local/build-info" > "$RUN/gamed-build-info.json"`,
		`curl -sS "$AUTH_OPS/local/build-info" > "$RUN/authd-build-info.json"`,
		`curl -sS "$OPS/local/runtime-config" > "$RUN/runtime-config.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"`,
		`metin2-migrate persistence-status-status`,
		`--persistence-status "$RUN/persistence-status-before.json"`,
		`> "$RUN/persistence-status-before-status.json"`,
		`curl -sS "$OPS/local/db/migrations/status" > "$RUN/daemon-migrations-status.json"`,
		`if [ -f "$GAMED_LOG" ]; then cp -p "$GAMED_LOG" "$RUN/gamed.log"; fi`,
		`if [ -f "$AUTHD_LOG" ]; then cp -p "$AUTHD_LOG" "$RUN/authd.log"; fi`,
		`cat > "$RUN/notes.md" <<'EOF'`,
		`metin2-migrate catalog > "$RUN/migration-catalog.json"`,
		`metin2-migrate catalog-status`,
		`--catalog "$RUN/migration-catalog.json"`,
		`--require-matches-embedded`,
		`> "$RUN/migration-catalog-status.json"`,
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
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"`,
		`--persistence-status "$RUN/persistence-status-after.json"`,
		`> "$RUN/persistence-status-after-status.json"`,
		`if [ -e "$RUN/$LOCK_FILE" ]; then`,
		`  metin2-migrate apply-lock-status --lock-file "$RUN/$LOCK_FILE" > "$RUN/apply-lock-status.json"`,
		`  echo "  metin2-migrate apply-lock-aside --lock-file \"$RUN/$LOCK_FILE\" --i-confirm-lab-aside-rename > \"$RUN/apply-lock-aside.json\""`,
		`  echo "  metin2-migrate apply-lock-aside-status --aside \"$RUN/apply-lock-aside.json\" > \"$RUN/apply-lock-aside-status.json\""`,
		`  echo "No leftover lock at $RUN/$LOCK_FILE (expected after successful apply)."`,
		`# Successful apply removes the lock; do not fail the runbook script on that path.`,
		`# require operator-exported DRIVER/DSN; printer never embeds a DSN`,
		`docs/workflow/migration-apply-runbook.md`,
		`docs/workflow/lab-deployment-topology.md`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\nmetin2-migrate apply-lock-aside --lock-file") {
		t.Fatalf("successful-path printer must not auto-run apply-lock-aside under set -eu, got:\n%s", body)
	}
	if strings.Contains(body, "\nmetin2-migrate apply-lock-aside-status --aside") {
		t.Fatalf("successful-path printer must not auto-run apply-lock-aside-status under set -eu, got:\n%s", body)
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "password=") || strings.Contains(body, "memory://") {
		t.Fatalf("migration-run-retention must not expose SQL or concrete DSN text, got %s", body)
	}
	idxMkdir := strings.Index(body, `mkdir -p "$RUN"`)
	idxAuthd := strings.Index(body, `curl -sS "$AUTH_OPS/local/build-info" > "$RUN/authd-build-info.json"`)
	idxRuntime := strings.Index(body, `curl -sS "$OPS/local/runtime-config" > "$RUN/runtime-config.json"`)
	idxStatusBefore := strings.Index(body, `> "$RUN/persistence-status-before.json"`)
	idxStatusBeforeStatus := strings.Index(body, `> "$RUN/persistence-status-before-status.json"`)
	idxGamedLog := strings.Index(body, `cp -p "$GAMED_LOG" "$RUN/gamed.log"`)
	idxAuthdLog := strings.Index(body, `cp -p "$AUTHD_LOG" "$RUN/authd.log"`)
	idxNotes := strings.Index(body, `cat > "$RUN/notes.md" <<'EOF'`)
	idxCatalog := strings.Index(body, `metin2-migrate catalog > "$RUN/migration-catalog.json"`)
	idxPreflight := strings.Index(body, `> "$RUN/apply-preflight.json"`)
	idxApply := strings.Index(body, `metin2-migrate apply \`)
	idxPostStatus := strings.Index(body, `> "$RUN/post-apply-status.json"`)
	idxStatusAfter := strings.Index(body, `> "$RUN/persistence-status-after.json"`)
	idxStatusAfterStatus := strings.Index(body, `> "$RUN/persistence-status-after-status.json"`)
	idxLockStatus := strings.Index(body, `apply-lock-status --lock-file "$RUN/$LOCK_FILE"`)
	if idxMkdir < 0 || idxAuthd < 0 || idxRuntime < 0 || idxStatusBefore < 0 || idxStatusBeforeStatus < 0 || idxGamedLog < 0 || idxAuthdLog < 0 || idxNotes < 0 || idxCatalog < 0 || idxPreflight < 0 || idxApply < 0 || idxPostStatus < 0 || idxStatusAfter < 0 || idxStatusAfterStatus < 0 || idxLockStatus < 0 {
		t.Fatalf("missing expected ordering markers in stdout:\n%s", body)
	}
	if !(idxMkdir < idxAuthd && idxAuthd < idxRuntime && idxRuntime < idxStatusBefore && idxStatusBefore < idxStatusBeforeStatus && idxStatusBeforeStatus < idxGamedLog && idxGamedLog < idxAuthdLog && idxAuthdLog < idxNotes && idxNotes < idxCatalog && idxCatalog < idxPreflight && idxPreflight < idxApply && idxApply < idxPostStatus && idxPostStatus < idxStatusAfter && idxStatusAfter < idxStatusAfterStatus && idxStatusAfterStatus < idxLockStatus) {
		t.Fatalf("expected mkdir -> authd/runtime/status-before -> before-status-status -> daemon logs -> notes -> catalog -> preflight -> apply -> post-status -> status-after -> after-status-status -> conditional lock triage ordering, got idxs mkdir=%d authd=%d runtime=%d before=%d beforeStatus=%d gamedLog=%d authdLog=%d notes=%d catalog=%d preflight=%d apply=%d post=%d after=%d afterStatus=%d lock=%d\n%s",
			idxMkdir, idxAuthd, idxRuntime, idxStatusBefore, idxStatusBeforeStatus, idxGamedLog, idxAuthdLog, idxNotes, idxCatalog, idxPreflight, idxApply, idxPostStatus, idxStatusAfter, idxStatusAfterStatus, idxLockStatus, body)
	}
	assertMigrationRunRetentionPrintsUngatedPersistenceStatusStatus(t, body)
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
			if !strings.Contains(stderr.String(), "--allow-rollback") {
				t.Fatalf("expected usage to list --allow-rollback, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), "--authd-ops-base-url") {
				t.Fatalf("expected usage to list --authd-ops-base-url, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), "--gamed-log-path") || !strings.Contains(stderr.String(), "--authd-log-path") {
				t.Fatalf("expected usage to list daemon log path flags, got %q", stderr.String())
			}
		})
	}
}

func TestRunMigrationRunRetentionRejectsInvalidAuthdOpsBaseURL(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--authd-ops-base-url", "ftp://127.0.0.1:6061",
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
	if !strings.Contains(stderr.String(), "authd-ops-base-url") {
		t.Fatalf("expected authd-ops-base-url error, got %q", stderr.String())
	}
}

func TestRunMigrationRunRetentionHonorsCustomAuthdOpsBaseURL(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--authd-ops-base-url", "http://127.0.0.1:17061/",
		},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	if !strings.Contains(body, `AUTH_OPS='http://127.0.0.1:17061'`) {
		t.Fatalf("expected normalized custom AUTH_OPS, got:\n%s", body)
	}
	if !strings.Contains(body, `curl -sS "$AUTH_OPS/local/build-info" > "$RUN/authd-build-info.json"`) {
		t.Fatalf("expected authd build-info retain, got:\n%s", body)
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

func TestRunMigrationRunRetentionPrintsRollbackTreeCommands(t *testing.T) {
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
			"--target-version", "0",
			"--allow-rollback",
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
		`AUTH_OPS='http://127.0.0.1:6061'`,
		`RUNS_BASE='/var/metin2/migration-runs'`,
		`TARGET_VERSION='0'`,
		`LOCK_FILE='migration-rollback.lock'`,
		`COMMIT12='abcdef012345'`,
		`RUN="${RUNS_BASE}/${TS}-${COMMIT12}"`,
		`mkdir -p "$RUN"`,
		`curl -sS "$AUTH_OPS/local/build-info" > "$RUN/authd-build-info.json"`,
		`curl -sS "$OPS/local/runtime-config" > "$RUN/runtime-config.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"`,
		`metin2-migrate persistence-status-status`,
		`--persistence-status "$RUN/persistence-status-before.json"`,
		`> "$RUN/persistence-status-before-status.json"`,
		`cat > "$RUN/notes.md" <<'EOF'`,
		`metin2-migrate catalog > "$RUN/migration-catalog.json"`,
		`metin2-migrate catalog-status`,
		`--catalog "$RUN/migration-catalog.json"`,
		`--require-matches-embedded`,
		`> "$RUN/migration-catalog-status.json"`,
		`> "$RUN/ledger-snapshot.json"`,
		`> "$RUN/ledger-snapshot-status.json"`,
		`> "$RUN/rollback-plan-artifact.json"`,
		`> "$RUN/rollback-plan-artifact-status.json"`,
		`> "$RUN/rollback-apply-preflight.json"`,
		`> "$RUN/rollback-apply-preflight-status.json"`,
		`--allow-rollback`,
		`--audit-file "$RUN/migration-rollback-audit.json"`,
		`> "$RUN/rollback-apply-audit-status.json"`,
		`> "$RUN/post-rollback-status.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"`,
		`--persistence-status "$RUN/persistence-status-after.json"`,
		`> "$RUN/persistence-status-after-status.json"`,
		`if [ -e "$RUN/$LOCK_FILE" ]; then`,
		`  metin2-migrate apply-lock-status --lock-file "$RUN/$LOCK_FILE" > "$RUN/apply-lock-status.json"`,
		`  echo "  metin2-migrate apply-lock-aside --lock-file \"$RUN/$LOCK_FILE\" --i-confirm-lab-aside-rename > \"$RUN/apply-lock-aside.json\""`,
		`  echo "  metin2-migrate apply-lock-aside-status --aside \"$RUN/apply-lock-aside.json\" > \"$RUN/apply-lock-aside-status.json\""`,
		`  echo "No leftover lock at $RUN/$LOCK_FILE (expected after successful apply)."`,
		`# Successful apply removes the lock; do not fail the runbook script on that path.`,
		`# require operator-exported DRIVER/DSN; printer never embeds a DSN`,
		`docs/workflow/migration-apply-runbook.md`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\nmetin2-migrate apply-lock-aside --lock-file") {
		t.Fatalf("successful-path printer must not auto-run apply-lock-aside under set -eu, got:\n%s", body)
	}
	if strings.Contains(body, "\nmetin2-migrate apply-lock-aside-status --aside") {
		t.Fatalf("successful-path printer must not auto-run apply-lock-aside-status under set -eu, got:\n%s", body)
	}
	for _, banned := range []string{
		`> "$RUN/migration-plan-artifact.json"`,
		`> "$RUN/plan-artifact-status.json"`,
		`> "$RUN/apply-preflight.json"`,
		`> "$RUN/apply-preflight-status.json"`,
		`--audit-file "$RUN/migration-apply-audit.json"`,
		`> "$RUN/apply-audit-status.json"`,
		`> "$RUN/post-apply-status.json"`,
		`LOCK_FILE='migration-apply.lock'`,
		`CREATE TABLE`,
		`DROP TABLE`,
		`password=`,
		`memory://`,
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("rollback retention must not contain %q, got:\n%s", banned, body)
		}
	}
	preflightIdx := strings.Index(body, `metin2-migrate apply-preflight`)
	applyIdx := strings.Index(body, `metin2-migrate apply \`)
	if preflightIdx < 0 || applyIdx < 0 {
		t.Fatalf("missing preflight/apply markers:\n%s", body)
	}
	preflightBlock := body[preflightIdx:applyIdx]
	applyBlock := body[applyIdx:]
	if !strings.Contains(preflightBlock, `--allow-rollback`) {
		t.Fatalf("expected --allow-rollback on apply-preflight block:\n%s", preflightBlock)
	}
	if !strings.Contains(applyBlock, `--allow-rollback`) {
		t.Fatalf("expected --allow-rollback on apply block:\n%s", applyBlock)
	}
	assertMigrationRunRetentionPrintsUngatedPersistenceStatusStatus(t, body)
}

func TestRunMigrationRunRetentionRejectsAllowRollbackWithLatestTarget(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "explicit-latest",
			args: []string{"migration-run-retention", "--build-info", "-", "--target-version", "latest", "--allow-rollback"},
		},
		{
			name: "default-latest",
			args: []string{"migration-run-retention", "--build-info", "-", "--allow-rollback"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(payload), &stdout, &stderr)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on contract failure, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "allow-rollback") || !strings.Contains(stderr.String(), "target-version") {
				t.Fatalf("expected allow-rollback/target-version error, got %q", stderr.String())
			}
		})
	}
}

func TestRunMigrationRunRetentionForwardPathOmitsAllowRollback(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{"migration-run-retention", "--build-info", "-"},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	if strings.Contains(body, `--allow-rollback`) {
		t.Fatalf("forward retention must omit --allow-rollback, got:\n%s", body)
	}
	if strings.Contains(body, `rollback-plan-artifact.json`) || strings.Contains(body, `migration-rollback.lock`) {
		t.Fatalf("forward retention must omit rollback artifact names, got:\n%s", body)
	}
	if !strings.Contains(body, `LOCK_FILE='migration-apply.lock'`) {
		t.Fatalf("expected forward default lock file, got:\n%s", body)
	}
}

func TestRunMigrationRunRetentionPrintsIntermediateForwardTarget(t *testing.T) {
	payload := `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789deadbeef",
  "build_date": "2026-08-28T16:00:00Z"
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--ops-base-url", "http://127.0.0.1:6060",
			"--migration-runs-base", "/var/metin2/migration-runs",
			"--target-version", "7",
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
		`TARGET_VERSION='7'`,
		`LOCK_FILE='migration-apply.lock'`,
		`> "$RUN/migration-plan-artifact.json"`,
		`> "$RUN/apply-preflight.json"`,
		`--audit-file "$RUN/migration-apply-audit.json"`,
		`> "$RUN/post-apply-status.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"`,
		`--persistence-status "$RUN/persistence-status-before.json"`,
		`> "$RUN/persistence-status-before-status.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"`,
		`--persistence-status "$RUN/persistence-status-after.json"`,
		`> "$RUN/persistence-status-after-status.json"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in intermediate forward stdout:\n%s", want, body)
		}
	}
	for _, banned := range []string{
		`--allow-rollback`,
		`rollback-plan-artifact.json`,
		`migration-rollback.lock`,
		`post-rollback-status.json`,
		`CREATE TABLE`,
		`DROP TABLE`,
		`password=`,
		`memory://`,
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("intermediate forward retention must not contain %q, got:\n%s", banned, body)
		}
	}
	assertMigrationRunRetentionPrintsUngatedPersistenceStatusStatus(t, body)
}

func TestRunMigrationRunRetentionPrintsIntermediateRollbackTarget(t *testing.T) {
	payload := `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789deadbeef",
  "build_date": "2026-08-28T16:15:00Z"
}`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--ops-base-url", "http://127.0.0.1:6060",
			"--migration-runs-base", "/var/metin2/migration-runs",
			"--target-version", "8",
			"--allow-rollback",
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
		`TARGET_VERSION='8'`,
		`LOCK_FILE='migration-rollback.lock'`,
		`> "$RUN/rollback-plan-artifact.json"`,
		`> "$RUN/rollback-apply-preflight.json"`,
		`--allow-rollback`,
		`--audit-file "$RUN/migration-rollback-audit.json"`,
		`> "$RUN/post-rollback-status.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"`,
		`--persistence-status "$RUN/persistence-status-before.json"`,
		`> "$RUN/persistence-status-before-status.json"`,
		`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"`,
		`--persistence-status "$RUN/persistence-status-after.json"`,
		`> "$RUN/persistence-status-after-status.json"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in intermediate rollback stdout:\n%s", want, body)
		}
	}
	for _, banned := range []string{
		`> "$RUN/migration-plan-artifact.json"`,
		`> "$RUN/apply-preflight.json"`,
		`--audit-file "$RUN/migration-apply-audit.json"`,
		`> "$RUN/post-apply-status.json"`,
		`LOCK_FILE='migration-apply.lock'`,
		`CREATE TABLE`,
		`DROP TABLE`,
		`password=`,
		`memory://`,
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("intermediate rollback retention must not contain %q, got:\n%s", banned, body)
		}
	}
	assertMigrationRunRetentionPrintsUngatedPersistenceStatusStatus(t, body)
}

func TestRunMigrationRunRetentionHonorsCustomDaemonLogPaths(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"migration-run-retention",
			"--build-info", "-",
			"--gamed-log-path", "/tmp/custom-gamed.jsonl",
			"--authd-log-path", "/tmp/custom-authd.jsonl",
		},
		strings.NewReader(payload),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	for _, want := range []string{
		`GAMED_LOG='/tmp/custom-gamed.jsonl'`,
		`AUTHD_LOG='/tmp/custom-authd.jsonl'`,
		`cp -p "$GAMED_LOG" "$RUN/gamed.log"`,
		`cp -p "$AUTHD_LOG" "$RUN/authd.log"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
}

func TestRunMigrationRunRetentionRejectsRelativeDaemonLogPaths(t *testing.T) {
	payload := `{"version":"v0.1.0","commit":"abcdef012345","build_date":"2026-08-21T15:30:45Z"}`
	cases := []struct {
		name string
		flag string
		path string
	}{
		{name: "relative-gamed", flag: "--gamed-log-path", path: "var/log/metin2/gamed.log"},
		{name: "blank-authd", flag: "--authd-log-path", path: "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{"migration-run-retention", "--build-info", "-", tc.flag, tc.path},
				strings.NewReader(payload),
				&stdout,
				&stderr,
			)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), strings.TrimPrefix(tc.flag, "--")) {
				t.Fatalf("expected %s reason, got %q", tc.flag, stderr.String())
			}
		})
	}
}

func assertMigrationRunRetentionPrintsUngatedPersistenceStatusStatus(t *testing.T, body string) {
	t.Helper()
	beforeRetain := `curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"`
	afterRetain := `curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"`
	beforeInspect := "metin2-migrate persistence-status-status \\\n  --persistence-status \"$RUN/persistence-status-before.json\" \\\n  > \"$RUN/persistence-status-before-status.json\""
	afterInspect := "metin2-migrate persistence-status-status \\\n  --persistence-status \"$RUN/persistence-status-after.json\" \\\n  > \"$RUN/persistence-status-after-status.json\""
	idxBeforeRetain := strings.Index(body, beforeRetain)
	idxBeforeInspect := strings.Index(body, beforeInspect)
	idxAfterRetain := strings.Index(body, afterRetain)
	idxAfterInspect := strings.Index(body, afterInspect)
	idxLockTriage := strings.Index(body, "echo '== optional lab stale-lock triage / aside-rename =='")
	if idxBeforeRetain < 0 || idxBeforeInspect < 0 || idxAfterRetain < 0 || idxAfterInspect < 0 || idxLockTriage < 0 {
		t.Fatalf("expected ungated persistence-status-status companions beside retained persistence-status JSON, got:\n%s", body)
	}
	if !(idxBeforeRetain < idxBeforeInspect && idxBeforeInspect < idxAfterRetain && idxAfterRetain < idxAfterInspect && idxAfterInspect < idxLockTriage) {
		t.Fatalf("expected before-retain -> before-status-status -> after-retain -> after-status-status -> leftover-lock triage, got idxs beforeRetain=%d beforeInspect=%d afterRetain=%d afterInspect=%d lock=%d\n%s",
			idxBeforeRetain, idxBeforeInspect, idxAfterRetain, idxAfterInspect, idxLockTriage, body)
	}
	beforeBlock := body[idxBeforeInspect:idxAfterRetain]
	afterBlock := body[idxAfterInspect:idxLockTriage]
	for _, banned := range []string{"--require-ok", "--require-drained", "--require-no-crash-temps"} {
		if strings.Contains(beforeBlock, banned) || strings.Contains(afterBlock, banned) {
			t.Fatalf("migration-run-retention persistence-status-status redirects must omit %q, got:\n%s", banned, body)
		}
	}
}
