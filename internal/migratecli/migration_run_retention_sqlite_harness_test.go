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
		"migration-catalog-status.json",
		"sql-drivers.json",
		"ledger-snapshot.json",
		"ledger-snapshot-status.json",
		"migration-plan-artifact.json",
		"plan-artifact-status.json",
		"apply-preflight.json",
		"apply-preflight-status.json",
		"migration-apply-audit.json",
		"apply-audit-status.json",
		"post-apply-status.json",
		"post-apply-status-status.json",
		"persistence-status-after.json",
		"persistence-status-before-status.json",
		"persistence-status-after-status.json",
	} {
		assertRegularFileExists(t, filepath.Join(runDir, name))
	}
	assertMigrationRunRetentionPersistenceStatusStatus(t, runDir)
	if _, err := os.Lstat(filepath.Join(runDir, "migration-apply.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected successful apply to remove lock file, lstat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "apply-lock-status.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no apply-lock-status.json on successful path, lstat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "apply-lock-aside-status.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no apply-lock-aside-status.json on successful path, lstat err=%v", err)
	}

	assertSQLiteLedgerAtCatalogTip(t, dsn)
	assertPostStatusCurrentVersion(t, filepath.Join(runDir, "post-apply-status.json"), catalogTipVersion(t))
	assertMigrationRunRetentionStatusStatus(t, runDir, "post-apply-status.json", "post-apply-status-status.json", catalogTipVersion(t))
	assertMigrationRunRetentionDaemonMigrationsStatus(t, runDir)
	assertCatalogStatusMatchesRetainedCatalog(t, runDir)
	assertMigrationRunRetentionSQLDrivers(t, runDir)
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
		"migration-catalog.json",
		"migration-catalog-status.json",
		"sql-drivers.json",
		"rollback-plan-artifact.json",
		"rollback-plan-artifact-status.json",
		"rollback-apply-preflight.json",
		"rollback-apply-preflight-status.json",
		"migration-rollback-audit.json",
		"rollback-apply-audit-status.json",
		"post-rollback-status.json",
		"post-rollback-status-status.json",
		"persistence-status-after.json",
		"persistence-status-before-status.json",
		"persistence-status-after-status.json",
	} {
		assertRegularFileExists(t, filepath.Join(runDir, name))
	}
	assertMigrationRunRetentionPersistenceStatusStatus(t, runDir)
	if _, err := os.Lstat(filepath.Join(runDir, "migration-rollback.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected successful rollback to remove lock file, lstat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "apply-lock-aside-status.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no apply-lock-aside-status.json on successful path, lstat err=%v", err)
	}

	assertSQLiteLedgerEmpty(t, dsn)
	assertPostStatusCurrentVersion(t, filepath.Join(runDir, "post-rollback-status.json"), 0)
	assertMigrationRunRetentionStatusStatus(t, runDir, "post-rollback-status.json", "post-rollback-status-status.json", 0)
	assertMigrationRunRetentionDaemonMigrationsStatus(t, runDir)
	assertCatalogStatusMatchesRetainedCatalog(t, runDir)
	assertMigrationRunRetentionSQLDrivers(t, runDir)
}

func TestMigrationRunRetentionSQLiteHermeticPrintedScriptAppliesToIntermediateTarget(t *testing.T) {
	binDir := t.TempDir()
	_ = mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)
	mustInstallMigrationRunRetentionCurlStub(t, binDir)

	runsBase := filepath.Join(t.TempDir(), "migration-runs-intermediate-forward")
	if err := os.MkdirAll(runsBase, 0o755); err != nil {
		t.Fatalf("mkdir migration-runs base: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "migration-run-retention-intermediate-forward.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"

	buildInfoPath := filepath.Join(t.TempDir(), "build-info.json")
	mustWriteFile(t, buildInfoPath, []byte(`{
  "version": "v0.1.0-retention",
  "commit": "interfwd0123456789abcd",
  "build_date": "2026-08-28T16:00:00Z"
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
			"--target-version", "7",
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
	for _, want := range []string{
		`TARGET_VERSION='7'`,
		`LOCK_FILE='migration-apply.lock'`,
		`> "$RUN/migration-plan-artifact.json"`,
		`> "$RUN/apply-preflight.json"`,
		`--audit-file "$RUN/migration-apply-audit.json"`,
		`> "$RUN/post-apply-status.json"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in intermediate forward script:\n%s", want, script)
		}
	}
	if strings.Contains(script, `--allow-rollback`) {
		t.Fatalf("intermediate forward script must omit --allow-rollback:\n%s", script)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DRIVER=sqlite",
		"DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, script, env)
	if code != 0 {
		t.Fatalf("expected printed intermediate forward retention script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "No leftover lock at") {
		t.Fatalf("expected successful-path leftover-lock note in script stdout, got %q", stdout)
	}

	runDir := mustFindSingleRetentionTree(t, runsBase, "interfwd0123")
	for _, name := range []string{
		"migration-catalog.json",
		"migration-catalog-status.json",
		"sql-drivers.json",
		"ledger-snapshot.json",
		"ledger-snapshot-status.json",
		"migration-plan-artifact.json",
		"plan-artifact-status.json",
		"apply-preflight.json",
		"apply-preflight-status.json",
		"migration-apply-audit.json",
		"apply-audit-status.json",
		"post-apply-status.json",
		"post-apply-status-status.json",
		"persistence-status-after.json",
		"persistence-status-before-status.json",
		"persistence-status-after-status.json",
	} {
		assertRegularFileExists(t, filepath.Join(runDir, name))
	}
	assertMigrationRunRetentionPersistenceStatusStatus(t, runDir)
	if _, err := os.Lstat(filepath.Join(runDir, "migration-apply.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected successful apply to remove lock file, lstat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "apply-lock-aside-status.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no apply-lock-aside-status.json on successful path, lstat err=%v", err)
	}

	assertSQLiteLedgerAtVersion(t, dsn, 7, "auth_login_ticket_handoff")
	assertPostStatusCurrentVersion(t, filepath.Join(runDir, "post-apply-status.json"), 7)
	assertMigrationRunRetentionStatusStatus(t, runDir, "post-apply-status.json", "post-apply-status-status.json", 7)
	assertMigrationRunRetentionDaemonMigrationsStatus(t, runDir)
	assertCatalogStatusMatchesRetainedCatalog(t, runDir)
	assertMigrationRunRetentionSQLDrivers(t, runDir)
}

func TestMigrationRunRetentionSQLiteHermeticPrintedScriptRollsBackToIntermediateTarget(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)
	mustInstallMigrationRunRetentionCurlStub(t, binDir)

	runsBase := filepath.Join(t.TempDir(), "migration-runs-intermediate-rollback")
	if err := os.MkdirAll(runsBase, 0o755); err != nil {
		t.Fatalf("mkdir migration-runs base: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "migration-run-retention-intermediate-rollback.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"
	mustApplyCatalogToTipWithSQLiteMigrate(t, migrateBin, dsn)
	assertSQLiteLedgerAtCatalogTip(t, dsn)

	buildInfoPath := filepath.Join(t.TempDir(), "build-info.json")
	mustWriteFile(t, buildInfoPath, []byte(`{
  "version": "v0.1.0-retention",
  "commit": "interrollb0123456789ab",
  "build_date": "2026-08-28T16:30:00Z"
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
			"--target-version", "8",
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
		`TARGET_VERSION='8'`,
		`LOCK_FILE='migration-rollback.lock'`,
		`> "$RUN/rollback-plan-artifact.json"`,
		`> "$RUN/rollback-apply-preflight.json"`,
		`--allow-rollback`,
		`--audit-file "$RUN/migration-rollback-audit.json"`,
		`> "$RUN/post-rollback-status.json"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in intermediate rollback script:\n%s", want, script)
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
		t.Fatalf("expected printed intermediate rollback retention script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "No leftover lock at") {
		t.Fatalf("expected successful-path leftover-lock note in script stdout, got %q", stdout)
	}

	runDir := mustFindSingleRetentionTree(t, runsBase, "interrollb01")
	for _, name := range []string{
		"migration-catalog.json",
		"migration-catalog-status.json",
		"sql-drivers.json",
		"rollback-plan-artifact.json",
		"rollback-plan-artifact-status.json",
		"rollback-apply-preflight.json",
		"rollback-apply-preflight-status.json",
		"migration-rollback-audit.json",
		"rollback-apply-audit-status.json",
		"post-rollback-status.json",
		"post-rollback-status-status.json",
		"persistence-status-after.json",
		"persistence-status-before-status.json",
		"persistence-status-after-status.json",
	} {
		assertRegularFileExists(t, filepath.Join(runDir, name))
	}
	assertMigrationRunRetentionPersistenceStatusStatus(t, runDir)
	if _, err := os.Lstat(filepath.Join(runDir, "migration-rollback.lock")); !os.IsNotExist(err) {
		t.Fatalf("expected successful rollback to remove lock file, lstat err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "apply-lock-aside-status.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no apply-lock-aside-status.json on successful path, lstat err=%v", err)
	}

	assertSQLiteLedgerAtVersion(t, dsn, 8, "static_actor_content_state")
	assertPostStatusCurrentVersion(t, filepath.Join(runDir, "post-rollback-status.json"), 8)
	assertMigrationRunRetentionStatusStatus(t, runDir, "post-rollback-status.json", "post-rollback-status-status.json", 8)
	assertMigrationRunRetentionDaemonMigrationsStatus(t, runDir)
	assertCatalogStatusMatchesRetainedCatalog(t, runDir)
	assertMigrationRunRetentionSQLDrivers(t, runDir)
}

func compactEmptyLedgerPlanJSON() string {
	plan, err := dbmigrations.PlanUpToLatest(nil)
	if err != nil {
		panic(err)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		panic(err)
	}
	if bytes.Contains(raw, []byte("'")) {
		panic("empty-ledger plan JSON contains single quotes")
	}
	return string(raw)
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
    body='` + compactEmptyPersistenceStatusJSON() + `'
    ;;
  */local/db/migrations/status)
    body='` + compactEmptyLedgerPlanJSON() + `'
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

func assertSQLiteLedgerAtVersion(t *testing.T, dsn string, wantVersion int, wantName string) {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	defer db.Close()

	ledger, err := dbmigrations.ReadSQLLedgerEntries(context.Background(), db)
	if err != nil {
		t.Fatalf("ReadSQLLedgerEntries: %v", err)
	}
	if len(ledger) != wantVersion {
		t.Fatalf("ledger rows = %d, want %d", len(ledger), wantVersion)
	}
	tip := ledger[len(ledger)-1]
	if tip.Version != wantVersion || tip.Name != wantName {
		t.Fatalf("ledger tip = %d/%s, want %d/%s", tip.Version, tip.Name, wantVersion, wantName)
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

func assertCatalogStatusMatchesRetainedCatalog(t *testing.T, runDir string) {
	t.Helper()
	catalogRaw, err := os.ReadFile(filepath.Join(runDir, "migration-catalog.json"))
	if err != nil {
		t.Fatalf("read retained catalog: %v", err)
	}
	statusRaw, err := os.ReadFile(filepath.Join(runDir, "migration-catalog-status.json"))
	if err != nil {
		t.Fatalf("read retained catalog-status: %v", err)
	}
	var got catalogStatusGot
	if err := json.Unmarshal(statusRaw, &got); err != nil {
		t.Fatalf("decode retained catalog-status: %v\nbody:\n%s", err, statusRaw)
	}
	if got.Format != catalogStatusFormat || !got.Present || got.Catalog == nil || !got.MatchesEmbedded {
		t.Fatalf("unexpected retained catalog-status: %#v", got)
	}
	if got.CatalogSHA256 != sha256Hex(catalogRaw) {
		t.Fatalf("catalog-status checksum mismatch: got %s want %s", got.CatalogSHA256, sha256Hex(catalogRaw))
	}
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

func assertMigrationRunRetentionPersistenceStatusStatus(t *testing.T, runDir string) {
	t.Helper()
	assertOneMigrationRunRetentionPersistenceStatusStatus(t, runDir, "persistence-status-before.json", "persistence-status-before-status.json")
	assertOneMigrationRunRetentionPersistenceStatusStatus(t, runDir, "persistence-status-after.json", "persistence-status-after-status.json")
}

func assertOneMigrationRunRetentionPersistenceStatusStatus(t *testing.T, runDir, statusName, companionName string) {
	t.Helper()
	statusRaw, err := os.ReadFile(filepath.Join(runDir, statusName))
	if err != nil {
		t.Fatalf("read %s: %v", statusName, err)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, companionName))
	if err != nil {
		t.Fatalf("read %s: %v", companionName, err)
	}
	var got persistenceStatusStatusGot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode %s: %v\nbody:\n%s", companionName, err, raw)
	}
	if got.Format != persistenceStatusStatusFormat || !got.Present || len(got.Status) == 0 || got.PersistenceStatusSHA256 != sha256Hex(statusRaw) {
		t.Fatalf("unexpected %s envelope: %#v", companionName, got)
	}
	var inner struct {
		OK                         bool `json:"ok"`
		LiveSelectedCharacterCount int  `json:"live_selected_character_count"`
	}
	if err := json.Unmarshal(got.Status, &inner); err != nil {
		t.Fatalf("decode inner %s: %v\ninner:\n%s", companionName, err, got.Status)
	}
	if !inner.OK || inner.LiveSelectedCharacterCount != 0 {
		t.Fatalf("expected drained ok inner snapshot in %s, got %#v", companionName, inner)
	}
	body := string(raw)
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("%s must not expose %q, got %s", companionName, forbidden, body)
		}
	}
}

func assertMigrationRunRetentionStatusStatus(t *testing.T, runDir, statusName, companionName string, wantCurrent int) {
	t.Helper()
	statusRaw, err := os.ReadFile(filepath.Join(runDir, statusName))
	if err != nil {
		t.Fatalf("read %s: %v", statusName, err)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, companionName))
	if err != nil {
		t.Fatalf("read %s: %v", companionName, err)
	}
	var got statusStatusGot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode %s: %v\nbody:\n%s", companionName, err, raw)
	}
	if got.Format != statusStatusFormat || !got.Present || got.Plan == nil || got.StatusSHA256 != sha256Hex(statusRaw) || !got.MatchesEmbeddedLatest {
		t.Fatalf("unexpected %s envelope: %#v", companionName, got)
	}
	if !got.Plan.UpToDate || got.Plan.CurrentVersion != wantCurrent || len(got.Plan.Pending) != 0 {
		t.Fatalf("expected up-to-date inner plan current_version=%d in %s, got %#v", wantCurrent, companionName, got.Plan)
	}
	body := string(raw)
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("%s must not expose %q, got %s", companionName, forbidden, body)
		}
	}
}

func assertMigrationRunRetentionDaemonMigrationsStatus(t *testing.T, runDir string) {
	t.Helper()
	statusPath := filepath.Join(runDir, "daemon-migrations-status.json")
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read daemon-migrations-status.json: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"status-status", "--status", statusPath}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected ungated status-status of daemon-migrations-status.json to succeed, exit=%d stderr=%q raw=%s", code, stderr.String(), raw)
	}
	var got statusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode daemon-migrations status-status: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != statusStatusFormat || !got.Present || got.Plan == nil || !got.MatchesEmbeddedLatest {
		t.Fatalf("expected inspectable empty-ledger daemon-migrations-status, got %#v raw=%s", got, raw)
	}
	if got.Plan.CurrentVersion != 0 || got.Plan.UpToDate || len(got.Plan.Pending) == 0 {
		t.Fatalf("expected empty-ledger pending daemon plan, got %#v", got.Plan)
	}
	if _, err := os.Lstat(filepath.Join(runDir, "daemon-migrations-status-status.json")); !os.IsNotExist(err) {
		t.Fatalf("printer must not emit daemon-migrations-status-status.json, lstat err=%v", err)
	}
}

func assertMigrationRunRetentionSQLDrivers(t *testing.T, runDir string) {
	t.Helper()
	path := filepath.Join(runDir, "sql-drivers.json")
	assertRegularFileExists(t, path)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sql-drivers.json: %v", err)
	}
	var got sqlDriversGot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode sql-drivers.json: %v\nbody:\n%s", err, raw)
	}
	if got.Format != "go-metin2-sql-drivers-v1" {
		t.Fatalf("unexpected sql-drivers format: %#v body=%s", got, raw)
	}
	if !sqlDriversListContains(t, got.Drivers, "sqlite") {
		t.Fatalf("expected sqlite in retained sql-drivers.json, got %s", raw)
	}
	body := string(raw)
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("sql-drivers.json must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Lstat(filepath.Join(runDir, "sql-drivers-status.json")); !os.IsNotExist(err) {
		t.Fatalf("printer must not emit sql-drivers-status.json, lstat err=%v", err)
	}
}
