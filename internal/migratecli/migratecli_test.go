package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
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
	if summary.LatestVersion < 7 || len(summary.Migrations) != summary.LatestVersion {
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

func TestRunRejectsUnknownCommandAsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply"}, nil, &stdout, &stderr)

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
