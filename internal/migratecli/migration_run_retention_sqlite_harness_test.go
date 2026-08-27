//go:build sqlite_harness

package migratecli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestMigrationRunRetentionSQLiteHermeticPrintedScriptAppliesToTip(t *testing.T) {
	binDir := t.TempDir()
	_ = mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)
	mustInstallMigrationRunRetentionCurlStub(t, binDir)

	runsBase := filepath.Join(t.TempDir(), "migration-runs")
	if err := os.MkdirAll(runsBase, 0o755); err != nil {
		t.Fatalf("mkdir migration-runs base: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "migration-run-retention-forward.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"

	buildInfoPath := filepath.Join(t.TempDir(), "build-info.json")
	mustWriteFile(t, buildInfoPath, []byte(`{
  "version": "v0.1.0-retention",
  "commit": "retention0123456789abcdef",
  "build_date": "2026-08-28T12:00:00Z"
}
`))

	var printStdout bytes.Buffer
	var printStderr bytes.Buffer
	printCode := Run(
		[]string{
			"migration-run-retention",
			"--build-info", buildInfoPath,
			"--ops-base-url", "http://127.0.0.1:6060",
			"--authd-ops-base-url", "http://127.0.0.1:6061",
			"--migration-runs-base", runsBase,
			"--target-version", "latest",
			"--gamed-log-path", filepath.Join(t.TempDir(), "missing-gamed.log"),
			"--authd-log-path", filepath.Join(t.TempDir(), "missing-authd.log"),
		},
		nil,
		&printStdout,
		&printStderr,
	)
	if printCode != exitOK {
		t.Fatalf("expected migration-run-retention exit %d, got %d stderr=%q", exitOK, printCode, printStderr.String())
	}
	if printStderr.Len() != 0 {
		t.Fatalf("expected no stderr from migration-run-retention, got %q", printStderr.String())
	}
	script := printStdout.String()
	assertMigrationRunRetentionScriptOmitsDSN(t, script, dsn)
	if !strings.Contains(script, `if [ -e "$RUN/$LOCK_FILE" ]; then`) {
		t.Fatalf("expected conditional leftover-lock triage in printed script:\n%s", script)
	}
	if strings.Contains(script, "\nmetin2-migrate apply-lock-aside --lock-file") {
		t.Fatalf("printed script must not auto-run apply-lock-aside:\n%s", script)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DRIVER=sqlite",
		"DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, script, env)
	if code != 0 {
		t.Fatalf("expected printed forward retention script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "No leftover lock at") {
		t.Fatalf("expected successful-path leftover-lock note in script stdout, got %q", stdout)
	}

	runDir := mustFindSingleRetentionTree(t, runsBase, "retention012")
	for _, name := range []string{
		"gamed-build-info.json",
		"authd-build-info.json",
		"runtime-config.json",
		"persistence-status-before.json",
		"daemon-migrations-status.json",
		"notes.md",
		"migration-catalog.json",
		"ledger-snapshot.json",
		"ledger-snapshot-status.json",
		"migration-plan-artifact.json",
		"plan-artifact-status.json",
		"apply-preflight.json",
		"apply-preflight-status.json",
		"migration-apply-audit.json",
		"apply-audit-status.json",
		"post-apply-status.json",
		"persistence-status-after.json",
	} {
		assertRegularFileExists(t, filepath.Join(runDir, name))
	}
	if _, err := os.Lstat(filepath.Join(runDir, "migration-apply.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected successful apply to remove lock file, lstat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "apply-lock-status.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no apply-lock-status.json on successful path, lstat err=%v", err)
	}

	assertSQLiteLedgerAtCatalogTip(t, dsn)
	assertPostStatusCurrentVersion(t, filepath.Join(runDir, "post-apply-status.json"), catalogTipVersion(t))
}

func TestMigrationRunRetentionSQLiteHermeticPrintedScriptRollsBackToZero(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)
	mustInstallMigrationRunRetentionCurlStub(t, binDir)

	runsBase := filepath.Join(t.TempDir(), "migration-runs-rollback")
	if err := os.MkdirAll(runsBase, 0o755); err != nil {
		t.Fatalf("mkdir migration-runs base: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "migration-run-retention-rollback.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"
	mustApplyCatalogToTipWithSQLiteMigrate(t, migrateBin, dsn)
	assertSQLiteLedgerAtCatalogTip(t, dsn)

	buildInfoPath := filepath.Join(t.TempDir(), "build-info.json")
	mustWriteFile(t, buildInfoPath, []byte(`{
  "version": "v0.1.0-retention",
  "commit": "rollback0123456789abcdef",
  "build_date": "2026-08-28T12:30:00Z"
}
`))

	var printStdout bytes.Buffer
	var printStderr bytes.Buffer
	printCode := Run(
		[]string{
			"migration-run-retention",
			"--build-info", buildInfoPath,
			"--ops-base-url", "http://127.0.0.1:6060",
			"--authd-ops-base-url", "http://127.0.0.1:6061",
			"--migration-runs-base", runsBase,
			"--target-version", "0",
			"--allow-rollback",
			"--gamed-log-path", filepath.Join(t.TempDir(), "missing-gamed.log"),
			"--authd-log-path", filepath.Join(t.TempDir(), "missing-authd.log"),
		},
		nil,
		&printStdout,
		&printStderr,
	)
	if printCode != exitOK {
		t.Fatalf("expected migration-run-retention exit %d, got %d stderr=%q", exitOK, printCode, printStderr.String())
	}
	script := printStdout.String()
	assertMigrationRunRetentionScriptOmitsDSN(t, script, dsn)
	for _, want := range []string{
		`TARGET_VERSION='0'`,
		`LOCK_FILE='migration-rollback.lock'`,
		`> "$RUN/rollback-plan-artifact.json"`,
		`> "$RUN/rollback-apply-preflight.json"`,
		`--audit-file "$RUN/migration-rollback-audit.json"`,
		`> "$RUN/post-rollback-status.json"`,
		`--allow-rollback`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in rollback script:\n%s", want, script)
		}
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DRIVER=sqlite",
		"DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, script, env)
	if code != 0 {
		t.Fatalf("expected printed rollback retention script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "No leftover lock at") {
		t.Fatalf("expected successful-path leftover-lock note in script stdout, got %q", stdout)
	}

	runDir := mustFindSingleRetentionTree(t, runsBase, "rollback0123")
	for _, name := range []string{
		"rollback-plan-artifact.json",
		"rollback-plan-artifact-status.json",
		"rollback-apply-preflight.json",
		"rollback-apply-preflight-status.json",
		"migration-rollback-audit.json",
		"rollback-apply-audit-status.json",
		"post-rollback-status.json",
		"persistence-status-after.json",
	} {
		assertRegularFileExists(t, filepath.Join(runDir, name))
	}
	if _, err := os.Lstat(filepath.Join(runDir, "migration-rollback.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected successful rollback to remove lock file, lstat err=%v", err)
	}

	assertSQLiteLedgerEmpty(t, dsn)
	assertPostStatusCurrentVersion(t, filepath.Join(runDir, "post-rollback-status.json"), 0)
}

func mustInstallMigrationRunRetentionCurlStub(t *testing.T, binDir string) {
	t.Helper()
	path := filepath.Join(binDir, "curl")
	script := `#!/bin/sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o|--output)
      shift
      out=${1:-}
      ;;
    -sS|-s|-S|-f|-L|--silent|--show-error|--fail|--location)
      ;;
    -*)
      ;;
    *)
      url=$1
      ;;
  esac
  shift
done
if [ -z "$url" ]; then
  echo "curl stub: missing url" >&2
  exit 1
fi
body='{"ok":true,"stub":"migration-run-retention-curl"}'
case "$url" in
  */local/build-info)
    body='{"version":"v0.1.0-retention","commit":"retention0123456789abcdef","build_date":"2026-08-28T12:00:00Z"}'
    ;;
  */local/runtime-config)
    body='{"service":"gamed","stub":true}'
    ;;
  */local/persistence/status)
    body='{"ok":true,"live_selected_character_count":0}'
    ;;
  */local/db/migrations/status)
    body='{"current_version":0,"latest_version":0,"up_to_date":false,"pending":[]}'
    ;;
esac
if [ -n "$out" ]; then
  printf '%s\n' "$body" > "$out"
else
  printf '%s\n' "$body"
fi
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write curl stub: %v", err)
	}
}

func assertMigrationRunRetentionScriptOmitsDSN(t *testing.T, script, dsn string) {
	t.Helper()
	for _, forbidden := range []string{
		dsn,
		"postgres://",
		"memory://",
		"CREATE TABLE",
		"DROP TABLE",
		"--dsn 'sqlite'",
		"--dsn sqlite",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("migration-run-retention must not expose %q, got %s", forbidden, script)
		}
	}
}

func assertSQLiteLedgerAtCatalogTip(t *testing.T, dsn string) {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	defer db.Close()

	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	wantTip := catalog[len(catalog)-1].Version
	ledger, err := dbmigrations.ReadSQLLedgerEntries(context.Background(), db)
	if err != nil {
		t.Fatalf("ReadSQLLedgerEntries: %v", err)
	}
	if len(ledger) != wantTip {
		t.Fatalf("ledger rows = %d, want tip %d", len(ledger), wantTip)
	}
	if ledger[len(ledger)-1].Version != wantTip {
		t.Fatalf("ledger tip = %d, want %d", ledger[len(ledger)-1].Version, wantTip)
	}
}

func assertSQLiteLedgerEmpty(t *testing.T, dsn string) {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	defer db.Close()

	ledger, err := dbmigrations.ReadSQLLedgerEntries(context.Background(), db)
	if err != nil {
		t.Fatalf("ReadSQLLedgerEntries after rollback: %v", err)
	}
	if len(ledger) != 0 {
		t.Fatalf("ledger after rollback = %#v, want empty", ledger)
	}
}

func catalogTipVersion(t *testing.T) int {
	t.Helper()
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	return catalog[len(catalog)-1].Version
}

func assertPostStatusCurrentVersion(t *testing.T, path string, want int) {
	t.Helper()
	body := mustReadFileString(t, path)
	var status struct {
		CurrentVersion int `json:"current_version"`
	}
	if err := json.Unmarshal([]byte(body), &status); err != nil {
		t.Fatalf("decode post status %s: %v body=%s", path, err, body)
	}
	if status.CurrentVersion != want {
		t.Fatalf("%s current_version = %d, want %d body=%s", path, status.CurrentVersion, want, body)
	}
}

func mustFindSingleRetentionTree(t *testing.T, base, commit12 string) string {
	t.Helper()
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("readdir %s: %v", base, err)
	}
	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "-"+commit12) {
			matches = append(matches, filepath.Join(base, name))
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one retention tree ending in -%s under %s, got %#v", commit12, base, matches)
	}
	return matches[0]
}

func assertRegularFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("expected regular file %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected regular file %s, mode=%v", path, info.Mode())
	}
}
