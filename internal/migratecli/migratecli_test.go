package migratecli

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestRunCatalogWritesMetadataOnlySummary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful catalog command, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	var summary dbmigrations.CatalogSummaryPayload
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("decode catalog JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if summary.Format != dbmigrations.CatalogSummaryFormat {
		t.Fatalf("unexpected catalog summary format: %#v", summary)
	}
	if summary.LatestVersion < 9 || len(summary.Migrations) != summary.LatestVersion {
		t.Fatalf("unexpected catalog summary size: %#v", summary)
	}
	if summary.Migrations[0].Name != "bootstrap_schema_migrations" || summary.Migrations[0].UpPath != "0001_bootstrap_schema_migrations.up.sql" {
		t.Fatalf("unexpected first migration summary: %#v", summary.Migrations[0])
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "UpSQL") || strings.Contains(body, "DownSQL") {
		t.Fatalf("catalog CLI must not expose executable SQL, got %s", body)
	}
}

func TestRunPlanUsesOfflineLedgerSnapshotAndTargetVersion(t *testing.T) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	snapshot := dbmigrations.LedgerSnapshot{
		Format: dbmigrations.LedgerSnapshotFormat,
		Entries: []dbmigrations.LedgerEntry{
			{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256},
		},
	}
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful plan command, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if plan.CurrentVersion != 1 || plan.LatestVersion != len(catalog) || plan.UpToDate {
		t.Fatalf("unexpected rollback preflight plan: %#v", plan)
	}
	if len(plan.Pending) != 1 {
		t.Fatalf("expected one rollback step, got %#v", plan.Pending)
	}
	step := plan.Pending[0]
	if step.Version != 1 || step.Name != "bootstrap_schema_migrations" || step.Direction != dbmigrations.DirectionDown || step.Path != "0001_bootstrap_schema_migrations.down.sql" || step.SHA256 != catalog[0].DownSHA256 {
		t.Fatalf("unexpected rollback step: %#v", step)
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "UpSQL") || strings.Contains(body, "DownSQL") {
		t.Fatalf("plan CLI must not expose executable SQL, got %s", body)
	}
}

func TestRunPlanReadsOfflineLedgerSnapshotFromPath(t *testing.T) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	snapshot := dbmigrations.LedgerSnapshot{Format: dbmigrations.LedgerSnapshotFormat, Entries: []dbmigrations.LedgerEntry{}}
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal ledger snapshot: %v", err)
	}
	snapshotPath := t.TempDir() + "/ledger-snapshot.json"
	if err := os.WriteFile(snapshotPath, rawSnapshot, 0o600); err != nil {
		t.Fatalf("write ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan", "--ledger-snapshot", snapshotPath, "--target-version", "1"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful plan command, exit=%d stderr=%q", code, stderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if plan.CurrentVersion != 0 || plan.LatestVersion != len(catalog) || plan.UpToDate {
		t.Fatalf("unexpected path-backed plan: %#v", plan)
	}
	if len(plan.Pending) != 1 || plan.Pending[0].Version != 1 || plan.Pending[0].Direction != dbmigrations.DirectionUp {
		t.Fatalf("unexpected path-backed pending steps: %#v", plan.Pending)
	}
}

func TestRunPlanAcceptsLatestTargetVersion(t *testing.T) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	snapshot := dbmigrations.LedgerSnapshot{Format: dbmigrations.LedgerSnapshotFormat, Entries: []dbmigrations.LedgerEntry{}}
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "latest"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected latest target plan to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode latest target plan JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if plan.CurrentVersion != 0 || plan.LatestVersion != len(catalog) || plan.UpToDate {
		t.Fatalf("unexpected latest target plan versions: %#v", plan)
	}
	if len(plan.Pending) != len(catalog) {
		t.Fatalf("expected latest target to plan every migration from empty ledger, got %#v", plan.Pending)
	}
	last := plan.Pending[len(plan.Pending)-1]
	if last.Version != len(catalog) || last.Direction != dbmigrations.DirectionUp {
		t.Fatalf("expected last latest target step to reach catalog latest, got %#v", last)
	}
}

func TestRunPlanRejectsOversizedLedgerSnapshotBeforePlanning(t *testing.T) {
	oversizedSnapshot := `{"format":"` + dbmigrations.LedgerSnapshotFormat + `","entries":[]}` + strings.Repeat(" ", 70*1024)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "0"}, strings.NewReader(oversizedSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected oversized snapshot to exit 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected oversized snapshot not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "ledger snapshot exceeds") {
		t.Fatalf("expected bounded snapshot error on stderr, got %q", stderr.String())
	}
}

func TestRunPlanRejectsInvalidSnapshotBeforeWritingPlan(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "0"}, strings.NewReader(`{"format":"manual","entries":[]}`), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected invalid snapshot to exit 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected invalid snapshot not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid migration ledger snapshot") {
		t.Fatalf("expected snapshot validation error on stderr, got %q", stderr.String())
	}
}

func TestRunApplyExecutesMigrationPlanAgainstRegisteredDriver(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://migrate", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful apply command, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 0 || result.CurrentVersion != 1 || result.LatestVersion < 1 {
		t.Fatalf("unexpected apply result versions: %#v", result)
	}
	if len(result.Applied) != 1 || result.Applied[0].Version != 1 || result.Applied[0].Direction != dbmigrations.DirectionUp {
		t.Fatalf("unexpected applied steps: %#v", result.Applied)
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "-- go-metin2 migration") || strings.Contains(body, "UpSQL") || strings.Contains(body, "DownSQL") || strings.Contains(body, "memory://") {
		t.Fatalf("apply CLI must not expose executable SQL or DSN text, got %s", body)
	}

	driver := currentMigrateCLITestDriver(t)
	events := driver.eventsSnapshot()
	for _, want := range []string{"open:memory://migrate", "begin", "exec:INSERT INTO schema_migrations", "query:SELECT version, name, up_sha256 FROM schema_migrations ORDER BY version ASC", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(events, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, events)
		}
	}
	if !containsMigrateCLITestEventContaining(events, "CREATE TABLE schema_migrations") {
		t.Fatalf("expected apply to execute migration SQL before ledger insert, got events %#v", events)
	}
	if containsMigrateCLITestEventContaining(events, "DROP TABLE") {
		t.Fatalf("expected apply to run only up statements, got events %#v", events)
	}
}

func TestRunApplyUsesOfflineLedgerSnapshotForRollbackTarget(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot(applied)
	if err != nil {
		t.Fatalf("marshal ledger snapshot: %v", err)
	}
	driver := currentMigrateCLITestDriver(t)
	driver.setLedger(applied)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful rollback apply, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode rollback result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 1 || result.CurrentVersion != 0 || result.LatestVersion < 1 {
		t.Fatalf("unexpected rollback result versions: %#v", result)
	}
	if len(result.Applied) != 1 || result.Applied[0].Version != 1 || result.Applied[0].Direction != dbmigrations.DirectionDown {
		t.Fatalf("unexpected rollback steps: %#v", result.Applied)
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "-- go-metin2 migration") || strings.Contains(body, "memory://") {
		t.Fatalf("rollback apply CLI must stay metadata-only, got %s", body)
	}
	events := driver.eventsSnapshot()
	deleteIndex := migrateCLITestEventIndexContaining(events, "exec:DELETE FROM schema_migrations")
	dropIndex := migrateCLITestEventIndexContaining(events, "DROP TABLE schema_migrations")
	if deleteIndex < 0 || dropIndex < 0 || deleteIndex > dropIndex {
		t.Fatalf("expected ledger delete before down SQL, got events %#v", events)
	}
	if !containsMigrateCLITestEventPrefix(events, "commit") {
		t.Fatalf("expected rollback command to commit, got events %#v", events)
	}
}

func TestRunApplyRejectsMissingLedgerSnapshotForRollbackTarget(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback", "--target-version", "0"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected rollback without snapshot to be a usage error, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected missing rollback snapshot not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--ledger-snapshot is required") {
		t.Fatalf("expected missing rollback snapshot usage guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected usage error before opening database, got events %#v", got)
	}
}

func TestRunApplyRejectsDriverWithoutDSNAsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", "sqlite3", "--target-version", "latest"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected missing DSN usage exit 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected usage error not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--driver and --dsn are required") {
		t.Fatalf("expected DSN usage guidance, got %q", stderr.String())
	}
}

func TestRunApplyRejectsOversizedLedgerSnapshotBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	oversizedSnapshot := `{"format":"` + dbmigrations.LedgerSnapshotFormat + `","entries":[]}` + strings.Repeat(" ", 70*1024)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://migrate", "--ledger-snapshot", "-", "--target-version", "1"}, strings.NewReader(oversizedSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected oversized snapshot to exit 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected oversized snapshot not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "ledger snapshot exceeds") {
		t.Fatalf("expected bounded snapshot error on stderr, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected oversized snapshot to fail before opening DB, got events %#v", got)
	}
}

func TestRunApplyRedactsDSNFromRuntimeErrors(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	secretDSN := "memory://secret-password@db/migrate"
	currentMigrateCLITestDriver(t).setError(fmt.Errorf("dial %s refused", secretDSN))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", secretDSN, "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected driver runtime error to exit 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected failed apply not to write stdout, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), secretDSN) {
		t.Fatalf("expected apply errors to redact DSN, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "<redacted-dsn>") {
		t.Fatalf("expected redacted DSN marker in error, got %q", stderr.String())
	}
}

func TestRunApplyResolvesLatestTargetVersion(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://latest", "--ledger-snapshot", "-", "--target-version", "latest"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected latest apply command, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode latest apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.CurrentVersion != result.LatestVersion || result.LatestVersion < 8 {
		t.Fatalf("expected latest apply to reach catalog tip, got %#v", result)
	}
	if len(result.Applied) != result.LatestVersion {
		t.Fatalf("expected latest apply from empty ledger to execute every migration, got %#v", result.Applied)
	}
	last := result.Applied[len(result.Applied)-1]
	if last.Version != result.LatestVersion || last.Direction != dbmigrations.DirectionUp {
		t.Fatalf("expected last latest step to reach catalog tip, got %#v", last)
	}
}

func TestRunEmptyLedgerSnapshotWritesStrictEmptySnapshot(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"empty-ledger-snapshot"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful empty-ledger-snapshot command, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	var snapshot dbmigrations.LedgerSnapshot
	if err := json.Unmarshal(stdout.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode empty ledger snapshot JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if snapshot.Format != dbmigrations.LedgerSnapshotFormat {
		t.Fatalf("unexpected ledger snapshot format: %#v", snapshot)
	}
	if snapshot.Entries == nil || len(snapshot.Entries) != 0 {
		t.Fatalf("expected explicit empty entries array, got %#v", snapshot.Entries)
	}
	body := stdout.String()
	if !strings.Contains(body, `"entries": []`) {
		t.Fatalf("expected explicit empty entries array in JSON, got %s", body)
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "-- go-metin2 migration") || strings.Contains(body, "memory://") {
		t.Fatalf("empty ledger snapshot CLI must not expose executable SQL or DSN text, got %s", body)
	}
}

func TestRunEmptyLedgerSnapshotOutputCanFeedPlan(t *testing.T) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	var snapshotStdout bytes.Buffer
	var snapshotStderr bytes.Buffer
	if code := Run([]string{"empty-ledger-snapshot"}, nil, &snapshotStdout, &snapshotStderr); code != 0 {
		t.Fatalf("expected empty-ledger-snapshot success, exit=%d stderr=%q", code, snapshotStderr.String())
	}
	var planStdout bytes.Buffer
	var planStderr bytes.Buffer

	code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "latest"}, bytes.NewReader(snapshotStdout.Bytes()), &planStdout, &planStderr)

	if code != 0 {
		t.Fatalf("expected plan from empty snapshot to succeed, exit=%d stderr=%q", code, planStderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(planStdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan JSON: %v\nbody:\n%s", err, planStdout.String())
	}
	if plan.CurrentVersion != 0 || plan.LatestVersion != len(catalog) || plan.UpToDate || len(plan.Pending) != len(catalog) {
		t.Fatalf("unexpected plan from generated empty snapshot: %#v", plan)
	}
}

func TestRunEmptyLedgerSnapshotRejectsUnexpectedArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"empty-ledger-snapshot", "extra"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected unexpected empty-ledger-snapshot argument to exit 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected usage error not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "empty-ledger-snapshot does not accept arguments") {
		t.Fatalf("expected empty-ledger-snapshot usage guidance, got %q", stderr.String())
	}
}

func TestRunLedgerSnapshotExportsMetadataOnlySQLLedgerSnapshot(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	currentMigrateCLITestDriver(t).setLedger(applied)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"ledger-snapshot", "--driver", driverName, "--dsn", "memory://snapshot"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful ledger-snapshot command, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	var snapshot dbmigrations.LedgerSnapshot
	if err := json.Unmarshal(stdout.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode ledger snapshot JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if snapshot.Format != dbmigrations.LedgerSnapshotFormat {
		t.Fatalf("unexpected ledger snapshot format: %#v", snapshot)
	}
	if len(snapshot.Entries) != 1 || snapshot.Entries[0] != applied[0] {
		t.Fatalf("unexpected SQL ledger snapshot entries: %#v", snapshot.Entries)
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "-- go-metin2 migration") || strings.Contains(body, "memory://") {
		t.Fatalf("ledger-snapshot CLI must not expose executable SQL or DSN text, got %s", body)
	}

	events := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:memory://snapshot", "query:SELECT version, name, up_sha256 FROM schema_migrations ORDER BY version ASC", "close"} {
		if !containsMigrateCLITestEventPrefix(events, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, events)
		}
	}
	for _, forbidden := range []string{"begin", "exec:", "commit", "rollback"} {
		if containsMigrateCLITestEventPrefix(events, forbidden) {
			t.Fatalf("ledger-snapshot command must be read-only; unexpected %q event in %#v", forbidden, events)
		}
	}
}

func TestRunLedgerSnapshotRejectsMissingDriverOrDSNAsUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"ledger-snapshot", "--driver", "sqlite3"},
		{"ledger-snapshot", "--dsn", "file:metin2.db"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(args, nil, &stdout, &stderr)

			if code != 2 {
				t.Fatalf("expected missing driver/DSN usage exit 2, got %d", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected usage error not to write stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "--driver and --dsn are required") {
				t.Fatalf("expected driver/DSN usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunLedgerSnapshotRedactsDSNFromRuntimeErrors(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	secretDSN := "memory://secret-password@db/snapshot"
	currentMigrateCLITestDriver(t).setError(fmt.Errorf("query %s refused", secretDSN))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"ledger-snapshot", "--driver", driverName, "--dsn", secretDSN}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected ledger-snapshot runtime error to exit 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected failed ledger-snapshot not to write stdout, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), secretDSN) {
		t.Fatalf("expected ledger-snapshot errors to redact DSN, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "<redacted-dsn>") {
		t.Fatalf("expected redacted DSN marker in error, got %q", stderr.String())
	}
}

func TestRunStatusReadsDatabaseLedgerAndWritesMetadataOnlyPlan(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	currentMigrateCLITestDriver(t).setLedger(applied)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status", "--driver", driverName, "--dsn", "memory://status", "--target-version", "latest"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful status command, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode status plan JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if plan.CurrentVersion != 1 || plan.LatestVersion != len(catalog) || plan.UpToDate {
		t.Fatalf("unexpected status plan versions: %#v", plan)
	}
	if len(plan.Pending) != len(catalog)-1 {
		t.Fatalf("expected pending steps from version 1 to latest, got %#v", plan.Pending)
	}
	if len(plan.Pending) > 0 && (plan.Pending[0].Version != 2 || plan.Pending[0].Direction != dbmigrations.DirectionUp) {
		t.Fatalf("expected first pending status step to be migration 0002 up, got %#v", plan.Pending[0])
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "-- go-metin2 migration") || strings.Contains(body, "memory://") {
		t.Fatalf("status CLI must not expose executable SQL or DSN text, got %s", body)
	}

	events := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:memory://status", "query:SELECT version, name, up_sha256 FROM schema_migrations ORDER BY version ASC", "close"} {
		if !containsMigrateCLITestEventPrefix(events, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, events)
		}
	}
	for _, forbidden := range []string{"begin", "exec:", "commit", "rollback"} {
		if containsMigrateCLITestEventPrefix(events, forbidden) {
			t.Fatalf("status command must be read-only; unexpected %q event in %#v", forbidden, events)
		}
	}
}

func TestRunStatusDefaultsToLatestTargetVersion(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status", "--driver", driverName, "--dsn", "memory://status-default"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful default status command, exit=%d stderr=%q", code, stderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode default status plan JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if plan.CurrentVersion != 0 || plan.LatestVersion != len(catalog) || plan.UpToDate {
		t.Fatalf("expected default status to plan empty ledger to latest, got %#v", plan)
	}
	if len(plan.Pending) != len(catalog) {
		t.Fatalf("expected default status to include every pending migration, got %#v", plan.Pending)
	}
}

func TestRunStatusAcceptsRollbackTargetVersion(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	currentMigrateCLITestDriver(t).setLedger(applied)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status", "--driver", driverName, "--dsn", "memory://status-rollback", "--target-version", "0"}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful rollback status command, exit=%d stderr=%q", code, stderr.String())
	}
	var plan dbmigrations.Plan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode rollback status plan JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if plan.CurrentVersion != 1 || len(plan.Pending) != 1 {
		t.Fatalf("expected one rollback status step from version 1, got %#v", plan)
	}
	if plan.Pending[0].Version != 1 || plan.Pending[0].Direction != dbmigrations.DirectionDown {
		t.Fatalf("unexpected rollback status step: %#v", plan.Pending[0])
	}
}

func TestRunStatusRejectsMissingDriverOrDSNAsUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--driver", "sqlite3"},
		{"status", "--dsn", "file:metin2.db"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(args, nil, &stdout, &stderr)

			if code != 2 {
				t.Fatalf("expected missing driver/DSN usage exit 2, got %d", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected usage error not to write stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "--driver and --dsn are required") {
				t.Fatalf("expected driver/DSN usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunStatusRedactsDSNFromRuntimeErrors(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	secretDSN := "memory://secret-password@db/status"
	currentMigrateCLITestDriver(t).setError(fmt.Errorf("query %s refused", secretDSN))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status", "--driver", driverName, "--dsn", secretDSN}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected status runtime error to exit 1, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected failed status not to write stdout, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), secretDSN) {
		t.Fatalf("expected status errors to redact DSN, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "<redacted-dsn>") {
		t.Fatalf("expected redacted DSN marker in error, got %q", stderr.String())
	}
}

func TestRunRejectsUnknownCommandAsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"frobnicate"}, nil, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected unknown command usage exit 2, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected usage errors not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command") || !strings.Contains(stderr.String(), "catalog") || !strings.Contains(stderr.String(), "plan") {
		t.Fatalf("expected usage guidance for unknown command, got %q", stderr.String())
	}
}

var migrateCLITestDriverRegistry struct {
	sync.Mutex
	current *migrateCLITestDriver
	next    int
}

func registerMigrateCLITestSQLDriver(t *testing.T) string {
	t.Helper()
	driver := &migrateCLITestDriver{}
	migrateCLITestDriverRegistry.Lock()
	migrateCLITestDriverRegistry.next++
	name := fmt.Sprintf("go_metin2_migratecli_test_%d_%s", migrateCLITestDriverRegistry.next, strings.NewReplacer("/", "_", "-", "_").Replace(t.Name()))
	migrateCLITestDriverRegistry.current = driver
	migrateCLITestDriverRegistry.Unlock()
	if err := registerSQLDriverOnce(name, driver); err != nil {
		t.Fatalf("register migration CLI test driver: %v", err)
	}
	t.Cleanup(func() {
		migrateCLITestDriverRegistry.Lock()
		if migrateCLITestDriverRegistry.current == driver {
			migrateCLITestDriverRegistry.current = nil
		}
		migrateCLITestDriverRegistry.Unlock()
	})
	return name
}

func currentMigrateCLITestDriver(t *testing.T) *migrateCLITestDriver {
	t.Helper()
	migrateCLITestDriverRegistry.Lock()
	defer migrateCLITestDriverRegistry.Unlock()
	if migrateCLITestDriverRegistry.current == nil {
		t.Fatalf("migration CLI test driver is not registered")
	}
	return migrateCLITestDriverRegistry.current
}

var registeredMigrateCLITestDrivers struct {
	sync.Mutex
	names map[string]struct{}
}

func registerSQLDriverOnce(name string, driver driver.Driver) (err error) {
	registeredMigrateCLITestDrivers.Lock()
	defer registeredMigrateCLITestDrivers.Unlock()
	if registeredMigrateCLITestDrivers.names == nil {
		registeredMigrateCLITestDrivers.names = make(map[string]struct{})
	}
	if _, ok := registeredMigrateCLITestDrivers.names[name]; ok {
		return nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("sql.Register(%q): %v", name, recovered)
		}
	}()
	sql.Register(name, driver)
	registeredMigrateCLITestDrivers.names[name] = struct{}{}
	return nil
}

type migrateCLITestDriver struct {
	mu     sync.Mutex
	events []string
	ledger []dbmigrations.LedgerEntry
	err    error
}

func (d *migrateCLITestDriver) Open(name string) (driver.Conn, error) {
	d.record("open:" + name)
	return &migrateCLITestConn{driver: d}, nil
}

func (d *migrateCLITestDriver) setLedger(entries []dbmigrations.LedgerEntry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ledger = append([]dbmigrations.LedgerEntry(nil), entries...)
}

func (d *migrateCLITestDriver) eventsSnapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.events...)
}

func (d *migrateCLITestDriver) record(event string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.events = append(d.events, event)
}

func (d *migrateCLITestDriver) setError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.err = err
}

func (d *migrateCLITestDriver) errorSnapshot() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

func (d *migrateCLITestDriver) ledgerSnapshot() []dbmigrations.LedgerEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]dbmigrations.LedgerEntry(nil), d.ledger...)
}

func (d *migrateCLITestDriver) appendLedgerFromInsert(query string) {
	values := valuesFromSQLInsert(query)
	if len(values) != 3 {
		return
	}
	version, err := strconvAtoiForTest(values[0])
	if err != nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ledger = append(d.ledger, dbmigrations.LedgerEntry{Version: version, Name: values[1], UpSHA256: values[2]})
}

func (d *migrateCLITestDriver) deleteLedgerFromDelete(query string) {
	version, name, sum, ok := ledgerDeleteCriteriaFromSQL(query)
	if !ok {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	filtered := d.ledger[:0]
	for _, entry := range d.ledger {
		if entry.Version == version && entry.Name == name && entry.UpSHA256 == sum {
			continue
		}
		filtered = append(filtered, entry)
	}
	d.ledger = filtered
}

type migrateCLITestConn struct {
	driver *migrateCLITestDriver
}

func (c *migrateCLITestConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not supported by migration CLI test driver")
}

func (c *migrateCLITestConn) Close() error {
	c.driver.record("close")
	return nil
}

func (c *migrateCLITestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *migrateCLITestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.driver.record("begin")
	return &migrateCLITestTx{driver: c.driver}, nil
}

func (c *migrateCLITestConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return (&migrateCLITestTx{driver: c.driver}).ExecContext(ctx, query, args)
}

func (c *migrateCLITestConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return (&migrateCLITestTx{driver: c.driver}).QueryContext(ctx, query, args)
}

func (c *migrateCLITestConn) CheckNamedValue(*driver.NamedValue) error {
	return nil
}

type migrateCLITestTx struct {
	driver *migrateCLITestDriver
}

func (tx *migrateCLITestTx) Commit() error {
	tx.driver.record("commit")
	return nil
}

func (tx *migrateCLITestTx) Rollback() error {
	tx.driver.record("rollback")
	return nil
}

func (tx *migrateCLITestTx) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	normalized := normalizeMigrateCLITestSQL(query)
	tx.driver.record("exec:" + normalized)
	if err := tx.driver.errorSnapshot(); err != nil {
		return nil, err
	}
	if strings.HasPrefix(normalized, "INSERT INTO schema_migrations") {
		tx.driver.appendLedgerFromInsert(query)
	}
	if strings.HasPrefix(normalized, "DELETE FROM schema_migrations") {
		tx.driver.deleteLedgerFromDelete(query)
	}
	return driver.RowsAffected(1), nil
}

func (tx *migrateCLITestTx) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	tx.driver.record("query:" + normalizeMigrateCLITestSQL(query))
	if err := tx.driver.errorSnapshot(); err != nil {
		return nil, err
	}
	return &migrateCLITestRows{entries: tx.driver.ledgerSnapshot()}, nil
}

type migrateCLITestRows struct {
	entries []dbmigrations.LedgerEntry
	index   int
}

func (r *migrateCLITestRows) Columns() []string {
	return []string{"version", "name", "up_sha256"}
}

func (r *migrateCLITestRows) Close() error {
	return nil
}

func (r *migrateCLITestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.entries) {
		return io.EOF
	}
	entry := r.entries[r.index]
	r.index++
	dest[0] = int64(entry.Version)
	dest[1] = entry.Name
	dest[2] = entry.UpSHA256
	return nil
}

func containsMigrateCLITestEventPrefix(events []string, prefix string) bool {
	for _, event := range events {
		if strings.HasPrefix(event, prefix) {
			return true
		}
	}
	return false
}

func containsMigrateCLITestEventContaining(events []string, fragment string) bool {
	return migrateCLITestEventIndexContaining(events, fragment) >= 0
}

func migrateCLITestEventIndexContaining(events []string, fragment string) int {
	for i, event := range events {
		if strings.Contains(event, fragment) {
			return i
		}
	}
	return -1
}

func normalizeMigrateCLITestSQL(query string) string {
	fields := strings.Fields(query)
	return strings.Join(fields, " ")
}

func valuesFromSQLInsert(query string) []string {
	start := strings.Index(query, "VALUES (")
	end := strings.LastIndex(query, ");")
	if start < 0 || end <= start {
		return nil
	}
	body := query[start+len("VALUES (") : end]
	parts := strings.Split(body, ", ")
	if len(parts) != 3 {
		return nil
	}
	return []string{strings.TrimSpace(parts[0]), unquoteMigrateCLITestSQLText(parts[1]), unquoteMigrateCLITestSQLText(parts[2])}
}

func ledgerDeleteCriteriaFromSQL(query string) (int, string, string, bool) {
	normalized := normalizeMigrateCLITestSQL(query)
	versionPrefix := "WHERE version = "
	versionStart := strings.Index(normalized, versionPrefix)
	if versionStart < 0 {
		return 0, "", "", false
	}
	rest := normalized[versionStart+len(versionPrefix):]
	versionText, rest, ok := strings.Cut(rest, " AND name = ")
	if !ok {
		return 0, "", "", false
	}
	nameText, sumText, ok := strings.Cut(rest, " AND up_sha256 = ")
	if !ok {
		return 0, "", "", false
	}
	version, err := strconvAtoiForTest(strings.TrimSpace(versionText))
	if err != nil {
		return 0, "", "", false
	}
	return version, unquoteMigrateCLITestSQLText(strings.TrimSuffix(strings.TrimSpace(nameText), ";")), unquoteMigrateCLITestSQLText(strings.TrimSuffix(strings.TrimSpace(sumText), ";")), true
}

func unquoteMigrateCLITestSQLText(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimSuffix(trimmed, ";")
	if len(trimmed) >= 2 && trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	return strings.ReplaceAll(trimmed, "''", "'")
}

func strconvAtoiForTest(value string) (int, error) {
	var result int
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &result)
	return result, err
}
