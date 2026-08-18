package migratecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
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
	if summary.LatestVersion < 11 || len(summary.Migrations) != summary.LatestVersion {
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
		t.Fatalf("expected last latest target step to reach catalog tip, got %#v", last)
	}
}

func TestRunPlanArtifactWritesChecksumForExactPlanJSON(t *testing.T) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	snapshot := dbmigrations.LedgerSnapshot{Format: dbmigrations.LedgerSnapshotFormat, Entries: []dbmigrations.LedgerEntry{}}
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal ledger snapshot: %v", err)
	}
	var planStdout bytes.Buffer
	var planStderr bytes.Buffer
	if code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "latest"}, bytes.NewReader(rawSnapshot), &planStdout, &planStderr); code != 0 {
		t.Fatalf("expected plan command to succeed, exit=%d stderr=%q", code, planStderr.String())
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "latest"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(stdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if artifact.Format != migrationPlanArtifactFormat {
		t.Fatalf("unexpected plan artifact format: %#v", artifact)
	}
	if artifact.PlanSHA256 != testSHA256HexBytes(planStdout.Bytes()) {
		t.Fatalf("expected artifact checksum over exact plan JSON, got %q want %q", artifact.PlanSHA256, testSHA256HexBytes(planStdout.Bytes()))
	}
	if artifact.Plan.CurrentVersion != 0 || artifact.Plan.LatestVersion != len(catalog) || artifact.Plan.UpToDate || len(artifact.Plan.Pending) != len(catalog) {
		t.Fatalf("unexpected embedded plan artifact: %#v", artifact.Plan)
	}
	body := stdout.String()
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "-- go-metin2 migration") || strings.Contains(body, "memory://") {
		t.Fatalf("plan-artifact CLI must not expose executable SQL or DSN text, got %s", body)
	}
}

func TestRunPlanArtifactStatusReportsMissingArtifactWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	artifactPath := t.TempDir() + "/missing-migration-plan-artifact.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact-status", "--plan-artifact", artifactPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected missing plan-artifact-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing plan-artifact-status not to write stderr, got %q", stderr.String())
	}
	var got migrationPlanArtifactStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing plan artifact status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationPlanArtifactStatusFormat || got.Present || got.Artifact != nil {
		t.Fatalf("unexpected missing plan artifact status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("plan-artifact-status must not open a database target, got events %#v", events)
	}
}

func TestRunPlanArtifactStatusReadsMetadataOnlyArtifactFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	var want migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &want); err != nil {
		t.Fatalf("decode written plan artifact: %v\nbody:\n%s", err, artifactStdout.String())
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact-status", "--plan-artifact", artifactPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected plan-artifact-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on plan-artifact-status success, got %q", stderr.String())
	}
	var got migrationPlanArtifactStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode plan artifact status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationPlanArtifactStatusFormat || !got.Present || got.Artifact == nil {
		t.Fatalf("unexpected plan artifact status envelope: %#v", got)
	}
	if got.Artifact.Format != migrationPlanArtifactFormat || got.Artifact.PlanSHA256 != want.PlanSHA256 {
		t.Fatalf("unexpected plan artifact status metadata: %#v", got.Artifact)
	}
	if got.Artifact.Plan.CurrentVersion != 0 || got.Artifact.Plan.LatestVersion < 1 || len(got.Artifact.Plan.Pending) != 1 || got.Artifact.Plan.Pending[0].Direction != dbmigrations.DirectionUp {
		t.Fatalf("unexpected plan artifact status plan: %#v", got.Artifact.Plan)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "-- go-metin2 migration", "memory://"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("plan artifact status output must stay metadata-only, exposed %q in %s", forbidden, body)
		}
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("plan-artifact-status must not remove the inspected artifact file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("plan-artifact-status must not open a database target, got events %#v", events)
	}
}

func TestRunPlanArtifactStatusRejectsMalformedArtifactFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	malformedArtifact := `{"format":"` + migrationPlanArtifactFormat + `","plan_sha256":"` + strings.Repeat("0", 64) + `","plan":{"current_version":0,"latest_version":1,"up_to_date":false,"pending":[]},"extra":true}`
	if err := os.WriteFile(artifactPath, []byte(malformedArtifact), 0o600); err != nil {
		t.Fatalf("write malformed plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact-status", "--plan-artifact", artifactPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected malformed plan-artifact-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected malformed plan-artifact-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply plan confirmation failed") || !strings.Contains(stderr.String(), "unknown field") {
		t.Fatalf("expected strict malformed-plan-artifact guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("malformed plan-artifact-status must not open a database target, got events %#v", events)
	}
}

func TestRunPlanArtifactStatusRejectsNonContiguousStepSequence(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	driftedPlan := dbmigrations.Plan{
		CurrentVersion: 0,
		LatestVersion:  len(catalog),
		UpToDate:       false,
		Pending: []dbmigrations.PlanStep{
			{Version: 2, Name: catalog[1].Name, Direction: dbmigrations.DirectionUp, Path: catalog[1].UpPath, SHA256: catalog[1].UpSHA256},
		},
	}
	planSHA256, err := planSHA256(driftedPlan)
	if err != nil {
		t.Fatalf("hash drifted plan: %v", err)
	}
	rawArtifact, err := json.MarshalIndent(migrationPlanArtifact{Format: migrationPlanArtifactFormat, PlanSHA256: planSHA256, Plan: driftedPlan}, "", "  ")
	if err != nil {
		t.Fatalf("marshal drifted plan artifact: %v", err)
	}
	rawArtifact = append(rawArtifact, '\n')
	if err := os.WriteFile(artifactPath, rawArtifact, 0o600); err != nil {
		t.Fatalf("write drifted plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact-status", "--plan-artifact", artifactPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected non-contiguous plan-artifact-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected non-contiguous plan-artifact-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply plan confirmation failed") || !strings.Contains(stderr.String(), "does not continue") {
		t.Fatalf("expected non-contiguous-step guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("non-contiguous plan-artifact-status must not open a database target, got events %#v", events)
	}
}

func TestRunPlanArtifactStatusRejectsSymlinkArtifactFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	targetPath := dir + "/target-plan-artifact.json"
	if err := os.WriteFile(targetPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write symlink plan artifact target: %v", err)
	}
	artifactPath := dir + "/migration-plan-artifact.json"
	if err := os.Symlink(targetPath, artifactPath); err != nil {
		t.Fatalf("create symlink plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact-status", "--plan-artifact", artifactPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected symlink plan-artifact-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected symlink plan-artifact-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plan artifact must not be a symlink") {
		t.Fatalf("expected symlink rejection guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("symlink plan-artifact-status must not open a database target, got events %#v", events)
	}
}

func TestRunPlanArtifactOutputCanFeedApplyPlanConfirmation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, artifactStdout.String())
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://artifact-confirmed-plan", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", artifact.PlanSHA256}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply with plan-artifact checksum to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode plan-artifact confirmed apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 0 || result.CurrentVersion != 1 || len(result.Applied) != 1 {
		t.Fatalf("unexpected plan-artifact confirmed apply result: %#v", result)
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); !containsMigrateCLITestEventPrefix(got, "open:memory://artifact-confirmed-plan") {
		t.Fatalf("expected artifact-confirmed apply to open migration target, got events %#v", got)
	}
}

func TestRunApplyPreflightValidatesPlanArtifactWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, artifactStdout.String())
	}
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply-preflight to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on successful apply-preflight, got %q", stderr.String())
	}
	var got struct {
		Format        string            `json:"format"`
		TargetVersion int               `json:"target_version"`
		TargetLatest  bool              `json:"target_latest"`
		PlanSHA256    string            `json:"plan_sha256"`
		Plan          dbmigrations.Plan `json:"plan"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode apply-preflight JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-apply-preflight-v1" {
		t.Fatalf("unexpected apply-preflight format: %#v", got)
	}
	if got.TargetVersion != 1 || got.TargetLatest || got.PlanSHA256 != artifact.PlanSHA256 {
		t.Fatalf("unexpected apply-preflight metadata: %#v", got)
	}
	if got.Plan.CurrentVersion != 0 || got.Plan.LatestVersion < 1 || len(got.Plan.Pending) != 1 || got.Plan.Pending[0].Direction != dbmigrations.DirectionUp {
		t.Fatalf("unexpected apply-preflight plan: %#v", got.Plan)
	}
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "-- go-metin2 migration", "memory://"} {
		if strings.Contains(stdout.String(), forbidden) || strings.Contains(stderr.String(), forbidden) {
			t.Fatalf("apply-preflight output must stay metadata-only, stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
	if gotEvents := currentMigrateCLITestDriver(t).eventsSnapshot(); len(gotEvents) != 0 {
		t.Fatalf("apply-preflight must not open a database target, got events %#v", gotEvents)
	}
}

func TestRunApplyPreflightReportsLedgerSnapshotSHA256ForRunbookAudit(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply-preflight to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got struct {
		LedgerSnapshotSHA256 string `json:"ledger_snapshot_sha256"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode apply-preflight JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.LedgerSnapshotSHA256 != testSHA256HexBytes(rawSnapshot) {
		t.Fatalf("expected apply-preflight to report ledger snapshot checksum %q, got %#v", testSHA256HexBytes(rawSnapshot), got)
	}
	if strings.Contains(stdout.String(), "CREATE TABLE") || strings.Contains(stdout.String(), "DROP TABLE") || strings.Contains(stdout.String(), "memory://") {
		t.Fatalf("apply-preflight ledger checksum output must stay metadata-only, got %s", stdout.String())
	}
	if gotEvents := currentMigrateCLITestDriver(t).eventsSnapshot(); len(gotEvents) != 0 {
		t.Fatalf("apply-preflight must not open a database target, got events %#v", gotEvents)
	}
}

func TestRunApplyPreflightRejectsRollbackWithoutConfirmationBeforeOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot(applied)
	if err != nil {
		t.Fatalf("marshal rollback ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected unconfirmed rollback apply-preflight to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected unconfirmed rollback apply-preflight not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--allow-rollback") {
		t.Fatalf("expected rollback apply-preflight direction acknowledgement guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected rollback apply-preflight guard before opening database, got events %#v", got)
	}
}

func TestRunApplyPreflightAcceptsConfirmedRollbackPlanArtifactWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot(applied)
	if err != nil {
		t.Fatalf("marshal rollback ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected rollback plan-artifact command, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/rollback-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write rollback plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "0", "--plan-artifact", artifactPath, "--allow-rollback"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected confirmed rollback apply-preflight to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got struct {
		Format        string            `json:"format"`
		TargetVersion int               `json:"target_version"`
		TargetLatest  bool              `json:"target_latest"`
		PlanSHA256    string            `json:"plan_sha256"`
		Plan          dbmigrations.Plan `json:"plan"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode rollback apply-preflight JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-apply-preflight-v1" || got.TargetVersion != 0 || got.TargetLatest {
		t.Fatalf("unexpected rollback apply-preflight metadata: %#v", got)
	}
	if len(got.Plan.Pending) != 1 || got.Plan.Pending[0].Direction != dbmigrations.DirectionDown {
		t.Fatalf("expected one down migration in rollback apply-preflight, got %#v", got.Plan)
	}
	if gotEvents := currentMigrateCLITestDriver(t).eventsSnapshot(); len(gotEvents) != 0 {
		t.Fatalf("confirmed rollback apply-preflight must not open a database target, got events %#v", gotEvents)
	}
}

func TestRunApplyPreflightRejectsPlanArtifactAndPlanSHA256TogetherAsUsageError(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	artifactPath := t.TempDir() + "/unused-plan-artifact.json"
	if err := os.WriteFile(artifactPath, []byte(`{"format":"unused"}`), 0o600); err != nil {
		t.Fatalf("write unused artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", strings.Repeat("0", 64), "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected plan artifact plus checksum to exit 2, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected apply-preflight usage error not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--plan-sha256 and --plan-artifact cannot be used together") {
		t.Fatalf("expected mutually exclusive plan confirmation guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected apply-preflight usage guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyPreflightRejectsMismatchedPlanArtifact(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected mismatched plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/mismatched-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write mismatched plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected mismatched plan artifact to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected mismatched plan artifact not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plan artifact does not match") {
		t.Fatalf("expected plan artifact mismatch guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected apply-preflight artifact mismatch guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyPreflightStatusReportsMissingPreflightWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	preflightPath := t.TempDir() + "/missing-apply-preflight.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight-status", "--apply-preflight", preflightPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected missing apply-preflight-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing apply-preflight-status not to write stderr, got %q", stderr.String())
	}
	var got migrationApplyPreflightStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing apply preflight status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationApplyPreflightStatusFormat || got.Present || got.Preflight != nil {
		t.Fatalf("unexpected missing apply preflight status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-preflight-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyPreflightStatusReadsMetadataOnlyPreflightFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected apply-preflight command to succeed, exit=%d stderr=%q", code, preflightStderr.String())
	}
	preflightPath := t.TempDir() + "/apply-preflight.json"
	if err := os.WriteFile(preflightPath, preflightStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write apply preflight: %v", err)
	}
	var want migrationApplyPreflight
	if err := json.Unmarshal(preflightStdout.Bytes(), &want); err != nil {
		t.Fatalf("decode written apply preflight: %v\nbody:\n%s", err, preflightStdout.String())
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight-status", "--apply-preflight", preflightPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply-preflight-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on apply-preflight-status success, got %q", stderr.String())
	}
	var got migrationApplyPreflightStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode apply preflight status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationApplyPreflightStatusFormat || !got.Present || got.Preflight == nil {
		t.Fatalf("unexpected apply preflight status envelope: %#v", got)
	}
	if got.Preflight.Format != migrationApplyPreflightFormat || got.Preflight.TargetVersion != want.TargetVersion || got.Preflight.TargetLatest != want.TargetLatest {
		t.Fatalf("unexpected apply preflight status metadata: %#v", got.Preflight)
	}
	if got.Preflight.LedgerSnapshotSHA256 != testSHA256HexBytes(rawSnapshot) || got.Preflight.PlanSHA256 != want.PlanSHA256 {
		t.Fatalf("unexpected apply preflight checksums: %#v", got.Preflight)
	}
	if got.Preflight.Plan.CurrentVersion != 0 || got.Preflight.Plan.LatestVersion < 1 || len(got.Preflight.Plan.Pending) != 1 || got.Preflight.Plan.Pending[0].Direction != dbmigrations.DirectionUp {
		t.Fatalf("unexpected apply preflight plan: %#v", got.Preflight.Plan)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "-- go-metin2 migration", "memory://"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("apply preflight status output must stay metadata-only, exposed %q in %s", forbidden, body)
		}
	}
	if _, err := os.Stat(preflightPath); err != nil {
		t.Fatalf("apply-preflight-status must not remove the inspected preflight file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-preflight-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyPreflightStatusRejectsMalformedPreflightFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	preflightPath := t.TempDir() + "/apply-preflight.json"
	malformedPreflight := `{"format":"` + migrationApplyPreflightFormat + `","target_version":1,"target_latest":false,"ledger_snapshot_sha256":"` + strings.Repeat("0", 64) + `","plan_sha256":"` + strings.Repeat("1", 64) + `","plan":{"current_version":0,"latest_version":1,"up_to_date":false,"pending":[]},"extra":true}`
	if err := os.WriteFile(preflightPath, []byte(malformedPreflight), 0o600); err != nil {
		t.Fatalf("write malformed apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight-status", "--apply-preflight", preflightPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected malformed apply-preflight-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected malformed apply-preflight-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply preflight failed") || !strings.Contains(stderr.String(), "unknown field") {
		t.Fatalf("expected strict malformed-preflight guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("malformed apply-preflight-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyPreflightStatusRejectsPlanChecksumDrift(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected apply-preflight command to succeed, exit=%d stderr=%q", code, preflightStderr.String())
	}
	var preflight migrationApplyPreflight
	if err := json.Unmarshal(preflightStdout.Bytes(), &preflight); err != nil {
		t.Fatalf("decode apply preflight: %v\nbody:\n%s", err, preflightStdout.String())
	}
	preflight.PlanSHA256 = strings.Repeat("0", 64)
	rawDriftedPreflight, err := json.MarshalIndent(preflight, "", "  ")
	if err != nil {
		t.Fatalf("marshal drifted apply preflight: %v", err)
	}
	rawDriftedPreflight = append(rawDriftedPreflight, '\n')
	preflightPath := t.TempDir() + "/apply-preflight.json"
	if err := os.WriteFile(preflightPath, rawDriftedPreflight, 0o600); err != nil {
		t.Fatalf("write drifted apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight-status", "--apply-preflight", preflightPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected checksum-drift apply-preflight-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected checksum-drift apply-preflight-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply preflight failed") || !strings.Contains(stderr.String(), "plan_sha256 mismatch") {
		t.Fatalf("expected preflight plan checksum guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("checksum-drift apply-preflight-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyPreflightStatusRejectsTargetDrift(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected apply-preflight command to succeed, exit=%d stderr=%q", code, preflightStderr.String())
	}
	var preflight migrationApplyPreflight
	if err := json.Unmarshal(preflightStdout.Bytes(), &preflight); err != nil {
		t.Fatalf("decode apply preflight: %v\nbody:\n%s", err, preflightStdout.String())
	}
	preflight.TargetVersion = 0
	rawDriftedPreflight, err := json.MarshalIndent(preflight, "", "  ")
	if err != nil {
		t.Fatalf("marshal target-drift apply preflight: %v", err)
	}
	rawDriftedPreflight = append(rawDriftedPreflight, '\n')
	preflightPath := t.TempDir() + "/apply-preflight.json"
	if err := os.WriteFile(preflightPath, rawDriftedPreflight, 0o600); err != nil {
		t.Fatalf("write target-drift apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight-status", "--apply-preflight", preflightPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected target-drift apply-preflight-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected target-drift apply-preflight-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply preflight failed") || !strings.Contains(stderr.String(), "target_version") {
		t.Fatalf("expected preflight target drift guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("target-drift apply-preflight-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyPreflightStatusRejectsSymlinkPreflightFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	targetPath := dir + "/target-apply-preflight.json"
	if err := os.WriteFile(targetPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write symlink preflight target: %v", err)
	}
	preflightPath := dir + "/apply-preflight.json"
	if err := os.Symlink(targetPath, preflightPath); err != nil {
		t.Fatalf("create symlink apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-preflight-status", "--apply-preflight", preflightPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected symlink apply-preflight-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected symlink apply-preflight-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "preflight file must not be a symlink") {
		t.Fatalf("expected symlink rejection guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("symlink apply-preflight-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyUsesApplyPreflightArtifactBeforeMutation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected apply-preflight command to succeed, exit=%d stderr=%q", code, preflightStderr.String())
	}
	preflightPath := t.TempDir() + "/apply-preflight.json"
	if err := os.WriteFile(preflightPath, preflightStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://apply-preflight", "--ledger-snapshot", "-", "--target-version", "1", "--apply-preflight", preflightPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply with preflight artifact to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode apply-preflight apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 0 || result.CurrentVersion != 1 || len(result.Applied) != 1 {
		t.Fatalf("unexpected apply-preflight result: %#v", result)
	}
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "-- go-metin2 migration", "memory://apply-preflight"} {
		if strings.Contains(stdout.String(), forbidden) || strings.Contains(stderr.String(), forbidden) {
			t.Fatalf("apply-preflight apply output must stay metadata-only and redacted, stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
	got := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:memory://apply-preflight", "begin", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(got, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, got)
		}
	}
}

func TestRunApplyWritesApplyPreflightPlanSHA256IntoAuditFile(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected apply-preflight command to succeed, exit=%d stderr=%q", code, preflightStderr.String())
	}
	var preflight migrationApplyPreflight
	if err := json.Unmarshal(preflightStdout.Bytes(), &preflight); err != nil {
		t.Fatalf("decode apply preflight JSON: %v\nbody:\n%s", err, preflightStdout.String())
	}
	preflightPath := t.TempDir() + "/apply-preflight.json"
	if err := os.WriteFile(preflightPath, preflightStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write apply preflight: %v", err)
	}
	auditPath := t.TempDir() + "/apply-preflight-audit.json"
	secretDSN := "memory://apply-preflight-audit"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", secretDSN, "--ledger-snapshot", "-", "--target-version", "1", "--apply-preflight", preflightPath, "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected audited apply with preflight artifact to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read apply-preflight audit file: %v", err)
	}
	var audit migrationApplyAudit
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode apply-preflight audit JSON: %v\nbody:\n%s", err, string(rawAudit))
	}
	if audit.ConfirmedPlanSHA256 != preflight.PlanSHA256 {
		t.Fatalf("expected audit to carry apply preflight plan checksum %q, got %#v", preflight.PlanSHA256, audit)
	}
	body := string(rawAudit)
	if strings.Contains(body, secretDSN) || strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") {
		t.Fatalf("apply-preflight audit file must stay metadata-only, got %s", body)
	}
}

func TestRunApplyRejectsMismatchedApplyPreflightBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected mismatched apply-preflight command to succeed, exit=%d stderr=%q", code, preflightStderr.String())
	}
	preflightPath := t.TempDir() + "/mismatched-apply-preflight.json"
	if err := os.WriteFile(preflightPath, preflightStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write mismatched apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://preflight-mismatch", "--ledger-snapshot", "-", "--target-version", "1", "--apply-preflight", preflightPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected mismatched apply preflight to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected mismatched apply preflight not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply preflight failed") || !strings.Contains(stderr.String(), "target_version") {
		t.Fatalf("expected apply preflight target mismatch guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected apply preflight mismatch guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyRejectsInvalidApplyPreflightBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	preflightPath := t.TempDir() + "/invalid-apply-preflight.json"
	invalidPreflight := `{"format":"` + migrationApplyPreflightFormat + `","target_version":1,"target_latest":false,"ledger_snapshot_sha256":"` + strings.Repeat("0", 64) + `","plan_sha256":"` + strings.Repeat("1", 64) + `","plan":{"current_version":0,"latest_version":1,"up_to_date":false,"pending":[]},"extra":true}`
	if err := os.WriteFile(preflightPath, []byte(invalidPreflight), 0o600); err != nil {
		t.Fatalf("write invalid apply preflight: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://invalid-preflight", "--ledger-snapshot", "-", "--target-version", "1", "--apply-preflight", preflightPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected invalid apply preflight to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected invalid apply preflight not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply preflight failed") || !strings.Contains(stderr.String(), "unknown field") {
		t.Fatalf("expected invalid preflight guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected invalid apply preflight guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyUsesApplyPreflightForRollbackTarget(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot(applied)
	if err != nil {
		t.Fatalf("marshal rollback ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected rollback plan-artifact command, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/rollback-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write rollback plan artifact: %v", err)
	}
	var preflightStdout bytes.Buffer
	var preflightStderr bytes.Buffer
	if code := Run([]string{"apply-preflight", "--ledger-snapshot", "-", "--target-version", "0", "--plan-artifact", artifactPath, "--allow-rollback"}, bytes.NewReader(rawSnapshot), &preflightStdout, &preflightStderr); code != 0 {
		t.Fatalf("expected rollback apply-preflight command, exit=%d stderr=%q", code, preflightStderr.String())
	}
	preflightPath := t.TempDir() + "/rollback-apply-preflight.json"
	if err := os.WriteFile(preflightPath, preflightStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write rollback apply preflight: %v", err)
	}
	driver := currentMigrateCLITestDriver(t)
	driver.setLedger(applied)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback-preflight", "--ledger-snapshot", "-", "--target-version", "0", "--allow-rollback", "--apply-preflight", preflightPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected preflight-confirmed rollback apply, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode preflight-confirmed rollback result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 1 || result.CurrentVersion != 0 || len(result.Applied) != 1 || result.Applied[0].Direction != dbmigrations.DirectionDown {
		t.Fatalf("unexpected preflight-confirmed rollback result: %#v", result)
	}
	for _, want := range []string{"open:memory://rollback-preflight", "begin", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(driver.eventsSnapshot(), want) {
			t.Fatalf("expected event prefix %q in events %#v", want, driver.eventsSnapshot())
		}
	}
}

func TestRunApplyAcceptsPlanArtifactBeforeMutation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://artifact-path", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply with plan artifact path to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode plan-artifact apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 0 || result.CurrentVersion != 1 || len(result.Applied) != 1 {
		t.Fatalf("unexpected plan-artifact apply result: %#v", result)
	}
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "-- go-metin2 migration", "memory://artifact-path"} {
		if strings.Contains(stdout.String(), forbidden) || strings.Contains(stderr.String(), forbidden) {
			t.Fatalf("plan-artifact apply output must stay metadata-only and redacted, stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
	got := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:memory://artifact-path", "begin", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(got, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, got)
		}
	}
}

func TestRunApplyWritesPlanArtifactSHA256IntoAuditFile(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, artifactStdout.String())
	}
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	auditPath := t.TempDir() + "/plan-artifact-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://artifact-audit", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath, "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected audited apply with plan artifact to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read plan-artifact audit file: %v", err)
	}
	var audit migrationApplyAudit
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode plan-artifact audit JSON: %v\nbody:\n%s", err, string(rawAudit))
	}
	if audit.ConfirmedPlanSHA256 != artifact.PlanSHA256 {
		t.Fatalf("expected audit to carry plan artifact checksum %q, got %#v", artifact.PlanSHA256, audit)
	}
	body := string(rawAudit)
	if strings.Contains(body, "memory://artifact-audit") || strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") {
		t.Fatalf("plan-artifact audit file must stay metadata-only, got %s", body)
	}
}

func TestRunApplyRejectsMismatchedPlanArtifactBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected rollback plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/mismatched-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write mismatched plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://artifact-mismatch", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected mismatched plan artifact to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected mismatched plan artifact not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plan artifact does not match") {
		t.Fatalf("expected plan artifact mismatch guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected plan artifact mismatch guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyRejectsInvalidPlanArtifactBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	artifactPath := t.TempDir() + "/invalid-plan-artifact.json"
	invalidArtifact := `{"format":"` + migrationPlanArtifactFormat + `","plan_sha256":"` + strings.Repeat("0", 64) + `","plan":{"current_version":0,"latest_version":1,"up_to_date":false,"pending":[]}}`
	if err := os.WriteFile(artifactPath, []byte(invalidArtifact), 0o600); err != nil {
		t.Fatalf("write invalid plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://invalid-artifact", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected invalid plan artifact to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected invalid plan artifact not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plan artifact checksum mismatch") {
		t.Fatalf("expected plan artifact checksum validation guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected invalid plan artifact guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyRejectsPlanArtifactAndPlanSHA256TogetherAsUsageError(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	artifactPath := t.TempDir() + "/unused-plan-artifact.json"
	if err := os.WriteFile(artifactPath, []byte(`{"format":"unused"}`), 0o600); err != nil {
		t.Fatalf("write unused artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://artifact-and-sha", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", strings.Repeat("0", 64), "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected plan artifact plus checksum to exit 2, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected usage error not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--plan-sha256 and --plan-artifact cannot be used together") {
		t.Fatalf("expected mutually exclusive plan confirmation guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected plan confirmation usage guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyRejectsApplyPreflightWithPlanConfirmationTogetherAsUsageError(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	preflightPath := t.TempDir() + "/unused-apply-preflight.json"
	if err := os.WriteFile(preflightPath, []byte(`{"format":"unused"}`), 0o600); err != nil {
		t.Fatalf("write unused apply preflight: %v", err)
	}
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "plan-sha256",
			args: []string{"apply", "--driver", driverName, "--dsn", "memory://preflight-and-sha", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", strings.Repeat("0", 64), "--apply-preflight", preflightPath},
		},
		{
			name: "plan-artifact",
			args: []string{"apply", "--driver", driverName, "--dsn", "memory://preflight-and-artifact", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", preflightPath, "--apply-preflight", preflightPath},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(tc.args, bytes.NewReader(rawSnapshot), &stdout, &stderr)

			if code != 2 {
				t.Fatalf("expected apply-preflight plus %s to exit 2, got exit=%d stdout=%q stderr=%q", tc.name, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected usage error not to write stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "--apply-preflight cannot be used together with --plan-sha256 or --plan-artifact") {
				t.Fatalf("expected mutually exclusive apply-preflight guidance, got %q", stderr.String())
			}
			if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
				t.Fatalf("expected apply-preflight usage guard before opening DB, got events %#v", got)
			}
		})
	}
}

func TestRunApplyRejectsOversizedPlanArtifactBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	artifactPath := t.TempDir() + "/oversized-plan-artifact.json"
	if err := os.WriteFile(artifactPath, []byte(strings.Repeat(" ", maxMigrationPlanArtifactBytes+1)), 0o600); err != nil {
		t.Fatalf("write oversized plan artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://oversized-artifact", "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected oversized plan artifact to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected oversized plan artifact not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plan artifact exceeds") {
		t.Fatalf("expected bounded plan artifact guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected oversized plan artifact guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyLockStatusReportsMissingLockWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	lockPath := t.TempDir() + "/missing-migration-apply.lock"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-status", "--lock-file", lockPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected missing apply-lock-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing apply-lock-status not to write stderr, got %q", stderr.String())
	}
	var got migrationApplyLockStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing lock status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationApplyLockStatusFormat || got.Present || got.Lock != nil {
		t.Fatalf("unexpected missing lock status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockStatusReadsMetadataOnlyLockFile(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, artifactStdout.String())
	}
	lockPath := t.TempDir() + "/migration-apply.lock"
	lock := migrationApplyLock{
		Format:               migrationApplyLockFormat,
		CreatedAt:            "2026-08-17T00:00:00Z",
		PID:                  1234,
		Driver:               driverName,
		DSNConfigured:        true,
		TargetVersion:        1,
		TargetLatest:         false,
		PlanSHA256:           artifact.PlanSHA256,
		ConfirmedPlanSHA256:  artifact.PlanSHA256,
		LedgerSnapshotSHA256: testSHA256HexBytes(rawSnapshot),
	}
	rawLock, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatalf("marshal lock JSON: %v", err)
	}
	rawLock = append(rawLock, '\n')
	if err := os.WriteFile(lockPath, rawLock, 0o600); err != nil {
		t.Fatalf("write lock JSON: %v", err)
	}
	secretDSN := "memory://lock-status-secret"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-status", "--lock-file", lockPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply-lock-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on apply-lock-status success, got %q", stderr.String())
	}
	var got migrationApplyLockStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode lock status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationApplyLockStatusFormat || !got.Present || got.Lock == nil {
		t.Fatalf("unexpected lock status envelope: %#v", got)
	}
	if got.Lock.Format != migrationApplyLockFormat || got.Lock.CreatedAt == "" || got.Lock.PID <= 0 {
		t.Fatalf("unexpected lock process metadata: %#v", got.Lock)
	}
	if got.Lock.Driver != driverName || !got.Lock.DSNConfigured {
		t.Fatalf("unexpected lock database metadata: %#v", got.Lock)
	}
	if got.Lock.TargetVersion != 1 || got.Lock.TargetLatest {
		t.Fatalf("unexpected lock target metadata: %#v", got.Lock)
	}
	if got.Lock.PlanSHA256 != artifact.PlanSHA256 || got.Lock.ConfirmedPlanSHA256 != artifact.PlanSHA256 {
		t.Fatalf("unexpected lock plan metadata: %#v", got.Lock)
	}
	if got.Lock.LedgerSnapshotSHA256 != testSHA256HexBytes(rawSnapshot) {
		t.Fatalf("unexpected lock ledger checksum: %#v", got.Lock)
	}
	body := stdout.String()
	for _, forbidden := range []string{secretDSN, "CREATE TABLE", "DROP TABLE", "-- go-metin2 migration"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("lock status output must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("apply-lock-status must not remove the inspected lock file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockStatusRejectsMalformedLockFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	lockPath := t.TempDir() + "/migration-apply.lock"
	malformedLock := `{"format":"go-metin2-migration-apply-lock-v1","created_at":"2026-08-17T00:00:00Z","pid":1,"driver":"driver","dsn_configured":true,"target_version":1,"target_latest":false,"plan_sha256":"` + strings.Repeat("0", 64) + `","ledger_snapshot_sha256":"` + strings.Repeat("1", 64) + `","extra":true}`
	if err := os.WriteFile(lockPath, []byte(malformedLock), 0o600); err != nil {
		t.Fatalf("write malformed lock file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-status", "--lock-file", lockPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected malformed apply-lock-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected malformed apply-lock-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply lock failed") || !strings.Contains(stderr.String(), "unknown field") {
		t.Fatalf("expected strict malformed-lock guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("malformed apply-lock-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockStatusRejectsSymlinkLockFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	targetPath := dir + "/target.lock"
	if err := os.WriteFile(targetPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	lockPath := dir + "/migration-apply.lock"
	if err := os.Symlink(targetPath, lockPath); err != nil {
		t.Fatalf("create symlink lock: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-status", "--lock-file", lockPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected symlink apply-lock-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected symlink apply-lock-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "lock file must not be a symlink") {
		t.Fatalf("expected symlink rejection guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("symlink apply-lock-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyAuditStatusReportsMissingAuditWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	auditPath := t.TempDir() + "/missing-migration-apply-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-audit-status", "--audit-file", auditPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected missing apply-audit-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing apply-audit-status not to write stderr, got %q", stderr.String())
	}
	var got migrationApplyAuditStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing audit status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationApplyAuditStatusFormat || got.Present || got.Audit != nil {
		t.Fatalf("unexpected missing audit status: %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-audit-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyAuditStatusReadsMetadataOnlyAuditFile(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, artifactStdout.String())
	}
	auditPath := t.TempDir() + "/migration-apply-audit.json"
	artifactPath := t.TempDir() + "/migration-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	secretDSN := "memory://apply-audit-secret"
	var applyStdout bytes.Buffer
	var applyStderr bytes.Buffer
	code := Run([]string{"apply", "--driver", driverName, "--dsn", secretDSN, "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath, "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &applyStdout, &applyStderr)
	if code != 0 {
		t.Fatalf("expected audited apply to succeed, exit=%d stdout=%q stderr=%q", code, applyStdout.String(), applyStderr.String())
	}
	eventsBeforeStatus := currentMigrateCLITestDriver(t).eventsSnapshot()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code = Run([]string{"apply-audit-status", "--audit-file", auditPath}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply-audit-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on apply-audit-status success, got %q", stderr.String())
	}
	var got migrationApplyAuditStatus
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode audit status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != migrationApplyAuditStatusFormat || !got.Present || got.Audit == nil {
		t.Fatalf("unexpected audit status envelope: %#v", got)
	}
	if got.Audit.Format != migrationApplyAuditFormat || got.Audit.AppliedAt == "" {
		t.Fatalf("unexpected audit timestamp metadata: %#v", got.Audit)
	}
	if got.Audit.Driver != driverName || !got.Audit.DSNConfigured {
		t.Fatalf("unexpected audit database metadata: %#v", got.Audit)
	}
	if got.Audit.TargetVersion != 1 || got.Audit.TargetLatest {
		t.Fatalf("unexpected audit target metadata: %#v", got.Audit)
	}
	if got.Audit.PlanSHA256 != artifact.PlanSHA256 || got.Audit.ConfirmedPlanSHA256 != artifact.PlanSHA256 {
		t.Fatalf("unexpected audit plan metadata: %#v", got.Audit)
	}
	if got.Audit.LedgerSnapshotSHA256 != testSHA256HexBytes(rawSnapshot) {
		t.Fatalf("unexpected audit ledger checksum: %#v", got.Audit)
	}
	if got.Audit.Result.PreviousVersion != 0 || got.Audit.Result.CurrentVersion != 1 || len(got.Audit.Result.Applied) != 1 {
		t.Fatalf("unexpected audit apply result: %#v", got.Audit.Result)
	}
	body := stdout.String()
	for _, forbidden := range []string{secretDSN, "CREATE TABLE", "DROP TABLE", "-- go-metin2 migration"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("audit status output must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(auditPath); err != nil {
		t.Fatalf("apply-audit-status must not remove the inspected audit file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != len(eventsBeforeStatus) {
		t.Fatalf("apply-audit-status must not open a database target, before=%#v after=%#v", eventsBeforeStatus, events)
	}
}

func TestRunApplyAuditStatusRejectsPlanChecksumDrift(t *testing.T) {
	auditPath, eventsBeforeStatus := writeValidApplyAuditFileForStatusTest(t, "memory://audit-drift")
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var audit migrationApplyAudit
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode audit file: %v\nbody:\n%s", err, string(rawAudit))
	}
	audit.PlanSHA256 = strings.Repeat("0", 64)
	rawDriftedAudit, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		t.Fatalf("marshal drifted audit: %v", err)
	}
	rawDriftedAudit = append(rawDriftedAudit, '\n')
	if err := os.WriteFile(auditPath, rawDriftedAudit, 0o600); err != nil {
		t.Fatalf("write drifted audit file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-audit-status", "--audit-file", auditPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected checksum-drift apply-audit-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected checksum-drift apply-audit-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply audit failed") || !strings.Contains(stderr.String(), "plan_sha256 mismatch") {
		t.Fatalf("expected audit plan checksum guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != len(eventsBeforeStatus) {
		t.Fatalf("checksum-drift apply-audit-status must not open a database target, before=%#v after=%#v", eventsBeforeStatus, events)
	}
}

func TestRunApplyAuditStatusRejectsTargetResultDrift(t *testing.T) {
	auditPath, eventsBeforeStatus := writeValidApplyAuditFileForStatusTest(t, "memory://audit-target-drift")
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var audit migrationApplyAudit
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode audit file: %v\nbody:\n%s", err, string(rawAudit))
	}
	audit.TargetVersion = 0
	rawDriftedAudit, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		t.Fatalf("marshal target-drift audit: %v", err)
	}
	rawDriftedAudit = append(rawDriftedAudit, '\n')
	if err := os.WriteFile(auditPath, rawDriftedAudit, 0o600); err != nil {
		t.Fatalf("write target-drift audit file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-audit-status", "--audit-file", auditPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected target-drift apply-audit-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected target-drift apply-audit-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply audit failed") || !strings.Contains(stderr.String(), "target_version") {
		t.Fatalf("expected audit target drift guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != len(eventsBeforeStatus) {
		t.Fatalf("target-drift apply-audit-status must not open a database target, before=%#v after=%#v", eventsBeforeStatus, events)
	}
}

func writeValidApplyAuditFileForStatusTest(t *testing.T, dsn string) (string, []string) {
	t.Helper()
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	auditPath := t.TempDir() + "/migration-apply-audit.json"
	var applyStdout bytes.Buffer
	var applyStderr bytes.Buffer
	code := Run([]string{"apply", "--driver", driverName, "--dsn", dsn, "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &applyStdout, &applyStderr)
	if code != 0 {
		t.Fatalf("expected audited apply to succeed, exit=%d stdout=%q stderr=%q", code, applyStdout.String(), applyStderr.String())
	}
	return auditPath, currentMigrateCLITestDriver(t).eventsSnapshot()
}

func TestRunApplyAuditStatusRejectsMalformedAuditFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	auditPath := t.TempDir() + "/migration-apply-audit.json"
	malformedAudit := `{"format":"go-metin2-migration-apply-audit-v1","applied_at":"2026-08-17T00:00:00Z","driver":"driver","dsn_configured":true,"target_version":1,"target_latest":false,"plan_sha256":"` + strings.Repeat("0", 64) + `","ledger_snapshot_sha256":"` + strings.Repeat("1", 64) + `","result":{"previous_version":0,"current_version":1,"latest_version":1,"applied":[{"version":1,"name":"bootstrap_schema_migrations","direction":"up","path":"0001_bootstrap_schema_migrations.up.sql","sha256":"` + strings.Repeat("2", 64) + `"}]},"extra":true}`
	if err := os.WriteFile(auditPath, []byte(malformedAudit), 0o600); err != nil {
		t.Fatalf("write malformed audit file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-audit-status", "--audit-file", auditPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected malformed apply-audit-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected malformed apply-audit-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply audit failed") || !strings.Contains(stderr.String(), "unknown field") {
		t.Fatalf("expected strict malformed-audit guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("malformed apply-audit-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyAuditStatusRejectsSymlinkAuditFile(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	targetPath := dir + "/target-audit.json"
	if err := os.WriteFile(targetPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write symlink audit target: %v", err)
	}
	auditPath := dir + "/migration-apply-audit.json"
	if err := os.Symlink(targetPath, auditPath); err != nil {
		t.Fatalf("create symlink audit: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-audit-status", "--audit-file", auditPath}, nil, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected symlink apply-audit-status to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected symlink apply-audit-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "audit file must not be a symlink") {
		t.Fatalf("expected symlink rejection guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("symlink apply-audit-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyRejectsExistingLockFileBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	lockPath := t.TempDir() + "/migration-apply.lock"
	if err := os.WriteFile(lockPath, []byte("already locked\n"), 0o600); err != nil {
		t.Fatalf("write existing lock file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://locked", "--ledger-snapshot", "-", "--target-version", "1", "--lock-file", lockPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected existing lock to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected existing lock not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration apply lock failed") || !strings.Contains(stderr.String(), "lock file already exists") {
		t.Fatalf("expected lock failure guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected existing lock guard before opening DB, got events %#v", got)
	}
	rawLock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read existing lock file: %v", err)
	}
	if string(rawLock) != "already locked\n" {
		t.Fatalf("expected existing lock file to remain untouched, got %q", string(rawLock))
	}
}

func TestRunApplyWritesMetadataOnlyLockFileBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected plan-artifact command to succeed, exit=%d stderr=%q", code, artifactStderr.String())
	}
	var artifact migrationPlanArtifact
	if err := json.Unmarshal(artifactStdout.Bytes(), &artifact); err != nil {
		t.Fatalf("decode plan artifact JSON: %v\nbody:\n%s", err, artifactStdout.String())
	}
	dir := t.TempDir()
	artifactPath := dir + "/migration-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	lockPath := dir + "/migration-apply.lock"
	secretDSN := "memory://lock-metadata-secret"
	currentMigrateCLITestDriver(t).setOpenHook(func() error {
		rawLock, err := os.ReadFile(lockPath)
		if err != nil {
			return fmt.Errorf("read reserved lock file before opening DB: %w", err)
		}
		var got struct {
			Format               string `json:"format"`
			CreatedAt            string `json:"created_at"`
			PID                  int    `json:"pid"`
			Driver               string `json:"driver"`
			DSNConfigured        bool   `json:"dsn_configured"`
			TargetVersion        int    `json:"target_version"`
			TargetLatest         bool   `json:"target_latest"`
			PlanSHA256           string `json:"plan_sha256"`
			ConfirmedPlanSHA256  string `json:"confirmed_plan_sha256"`
			LedgerSnapshotSHA256 string `json:"ledger_snapshot_sha256"`
		}
		if err := json.Unmarshal(rawLock, &got); err != nil {
			return fmt.Errorf("decode reserved lock JSON: %w; body=%q", err, string(rawLock))
		}
		if got.Format != "go-metin2-migration-apply-lock-v1" {
			return fmt.Errorf("unexpected lock format: %#v", got)
		}
		if got.CreatedAt == "" || got.PID <= 0 {
			return fmt.Errorf("expected lock to include local process metadata: %#v", got)
		}
		if got.Driver != driverName || !got.DSNConfigured {
			return fmt.Errorf("unexpected lock database metadata: %#v", got)
		}
		if got.TargetVersion != 1 || got.TargetLatest {
			return fmt.Errorf("unexpected lock target metadata: %#v", got)
		}
		if got.PlanSHA256 != artifact.PlanSHA256 || got.ConfirmedPlanSHA256 != artifact.PlanSHA256 {
			return fmt.Errorf("unexpected lock plan metadata: %#v", got)
		}
		if got.LedgerSnapshotSHA256 != testSHA256HexBytes(rawSnapshot) {
			return fmt.Errorf("unexpected lock ledger checksum: %#v", got)
		}
		body := string(rawLock)
		for _, forbidden := range []string{secretDSN, "CREATE TABLE", "DROP TABLE", "-- go-metin2 migration"} {
			if strings.Contains(body, forbidden) {
				return fmt.Errorf("reserved lock file must not expose %q, got %s", forbidden, body)
			}
		}
		return nil
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", secretDSN, "--ledger-snapshot", "-", "--target-version", "1", "--plan-artifact", artifactPath, "--lock-file", lockPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected locked apply to succeed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected successful apply to remove metadata lock file, got %v", err)
	}
}

func TestRunApplyRemovesLockFileAfterSuccessfulMutation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	lockPath := t.TempDir() + "/migration-apply.lock"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://lock-success", "--ledger-snapshot", "-", "--target-version", "1", "--lock-file", lockPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected locked apply to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected successful apply to remove lock file, got %v", err)
	}
	got := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:memory://lock-success", "begin", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(got, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, got)
		}
	}
	body := stdout.String()
	if strings.Contains(body, "migration-apply.lock") || strings.Contains(body, "memory://lock-success") || strings.Contains(body, "CREATE TABLE") {
		t.Fatalf("locked apply output must stay metadata-only, got %s", body)
	}
}

func TestRunApplyRemovesReservedLockFileWhenApplyFails(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	secretDSN := "memory://lock-failed-secret"
	currentMigrateCLITestDriver(t).setError(fmt.Errorf("apply failed for %s", secretDSN))
	lockPath := t.TempDir() + "/migration-apply.lock"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", secretDSN, "--ledger-snapshot", "-", "--target-version", "1", "--lock-file", lockPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failed locked apply to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected failed locked apply not to write stdout, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), secretDSN) || !strings.Contains(stderr.String(), "<redacted-dsn>") {
		t.Fatalf("expected failed locked apply to redact DSN, got %q", stderr.String())
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected failed apply to remove reserved lock file, got %v", err)
	}
	got := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:" + secretDSN, "begin", "rollback", "close"} {
		if !containsMigrateCLITestEventPrefix(got, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, got)
		}
	}
}

func TestRunPlanArtifactRejectsOversizedLedgerSnapshotBeforePlanning(t *testing.T) {
	oversizedSnapshot := `{"format":"` + dbmigrations.LedgerSnapshotFormat + `","entries":[]}` + strings.Repeat(" ", 70*1024)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "0"}, strings.NewReader(oversizedSnapshot), &stdout, &stderr)

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
	var planStdout bytes.Buffer
	var planStderr bytes.Buffer
	if code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &planStdout, &planStderr); code != 0 {
		t.Fatalf("expected rollback plan command, exit=%d stderr=%q", code, planStderr.String())
	}
	planSHA256 := testSHA256HexBytes(planStdout.Bytes())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback", "--ledger-snapshot", "-", "--target-version", "0", "--allow-rollback", "--plan-sha256", planSHA256}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

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

func TestRunApplyUsesPlanArtifactForRollbackTarget(t *testing.T) {
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
	var artifactStdout bytes.Buffer
	var artifactStderr bytes.Buffer
	if code := Run([]string{"plan-artifact", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &artifactStdout, &artifactStderr); code != 0 {
		t.Fatalf("expected rollback plan-artifact command, exit=%d stderr=%q", code, artifactStderr.String())
	}
	artifactPath := t.TempDir() + "/rollback-plan-artifact.json"
	if err := os.WriteFile(artifactPath, artifactStdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write rollback plan artifact: %v", err)
	}
	driver := currentMigrateCLITestDriver(t)
	driver.setLedger(applied)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback-artifact", "--ledger-snapshot", "-", "--target-version", "0", "--allow-rollback", "--plan-artifact", artifactPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful artifact-confirmed rollback apply, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode artifact-confirmed rollback result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 1 || result.CurrentVersion != 0 || len(result.Applied) != 1 || result.Applied[0].Direction != dbmigrations.DirectionDown {
		t.Fatalf("unexpected artifact-confirmed rollback result: %#v", result)
	}
	for _, want := range []string{"open:memory://rollback-artifact", "begin", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(driver.eventsSnapshot(), want) {
			t.Fatalf("expected event prefix %q in events %#v", want, driver.eventsSnapshot())
		}
	}
}

func TestRunApplyRejectsRollbackWithoutPlanConfirmationBeforeOpeningDatabase(t *testing.T) {
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
	currentMigrateCLITestDriver(t).setLedger(applied)
	dir := t.TempDir()
	lockPath := dir + "/migration-rollback.lock"
	auditPath := dir + "/migration-rollback-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback-unconfirmed-plan", "--ledger-snapshot", "-", "--target-version", "0", "--allow-rollback", "--lock-file", lockPath, "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected rollback without plan confirmation to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected unconfirmed rollback not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--plan-sha256") || !strings.Contains(stderr.String(), "--plan-artifact") || !strings.Contains(stderr.String(), "--apply-preflight") {
		t.Fatalf("expected rollback plan confirmation guidance, got %q", stderr.String())
	}
	for _, path := range []string{lockPath, auditPath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected unconfirmed rollback not to reserve %s, got %v", path, err)
		}
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected rollback plan confirmation guard before opening database, got events %#v", got)
	}
}

func TestRunApplyRejectsRollbackTargetWithoutAllowRollbackBeforeOpeningDatabase(t *testing.T) {
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
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback-unconfirmed", "--ledger-snapshot", "-", "--target-version", "0"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected unconfirmed rollback apply to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected unconfirmed rollback not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--allow-rollback") {
		t.Fatalf("expected rollback confirmation guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected rollback confirmation guard before opening database, got events %#v", got)
	}
}

func TestRunApplyRejectsRollbackBeforePlanArtifactLockAuditTouch(t *testing.T) {
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
	dir := t.TempDir()
	missingPlanArtifactPath := dir + "/missing-plan-artifact.json"
	lockPath := dir + "/migration-apply.lock"
	auditPath := dir + "/migration-rollback-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://rollback-guard-order", "--ledger-snapshot", "-", "--target-version", "0", "--plan-artifact", missingPlanArtifactPath, "--lock-file", lockPath, "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected unconfirmed rollback apply to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--allow-rollback") {
		t.Fatalf("expected rollback confirmation guidance before artifact/lock/audit handling, got %q", stderr.String())
	}
	for _, path := range []string{lockPath, auditPath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected unconfirmed rollback not to reserve %s, got %v", path, err)
		}
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected rollback confirmation guard before opening database, got events %#v", got)
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

func TestRunApplyAcceptsConfirmedPlanSHA256BeforeMutation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var planStdout bytes.Buffer
	var planStderr bytes.Buffer
	if code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &planStdout, &planStderr); code != 0 {
		t.Fatalf("expected plan command for apply confirmation to succeed, exit=%d stderr=%q", code, planStderr.String())
	}
	planSHA256 := testSHA256HexBytes(planStdout.Bytes())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://confirmed-plan", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", planSHA256}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected apply with confirmed plan checksum to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode confirmed-plan apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if result.PreviousVersion != 0 || result.CurrentVersion != 1 || len(result.Applied) != 1 {
		t.Fatalf("unexpected confirmed-plan apply result: %#v", result)
	}
	events := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, want := range []string{"open:memory://confirmed-plan", "begin", "commit", "close"} {
		if !containsMigrateCLITestEventPrefix(events, want) {
			t.Fatalf("expected event prefix %q in events %#v", want, events)
		}
	}
}

func TestRunApplyWritesConfirmedPlanSHA256IntoAuditFile(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var planStdout bytes.Buffer
	var planStderr bytes.Buffer
	if code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &planStdout, &planStderr); code != 0 {
		t.Fatalf("expected plan command for audited apply confirmation to succeed, exit=%d stderr=%q", code, planStderr.String())
	}
	planSHA256 := testSHA256HexBytes(planStdout.Bytes())
	auditPath := t.TempDir() + "/confirmed-plan-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://confirmed-plan-audit", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", planSHA256, "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected audited apply with confirmed plan checksum to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read confirmed-plan audit file: %v", err)
	}
	var audit migrationApplyAudit
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode confirmed-plan audit JSON: %v\nbody:\n%s", err, string(rawAudit))
	}
	if audit.ConfirmedPlanSHA256 != planSHA256 {
		t.Fatalf("expected audit to carry confirmed plan checksum %q, got %#v", planSHA256, audit)
	}
	body := string(rawAudit)
	if strings.Contains(body, "memory://confirmed-plan-audit") || strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") {
		t.Fatalf("confirmed-plan audit file must stay metadata-only, got %s", body)
	}
}

func TestRunApplyWritesComputedPlanSHA256IntoAuditFile(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var planStdout bytes.Buffer
	var planStderr bytes.Buffer
	if code := Run([]string{"plan", "--ledger-snapshot", "-", "--target-version", "1"}, bytes.NewReader(rawSnapshot), &planStdout, &planStderr); code != 0 {
		t.Fatalf("expected plan command for audited apply checksum to succeed, exit=%d stderr=%q", code, planStderr.String())
	}
	planSHA256 := testSHA256HexBytes(planStdout.Bytes())
	auditPath := t.TempDir() + "/computed-plan-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://computed-plan-audit", "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected audited apply to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read computed-plan audit file: %v", err)
	}
	var audit struct {
		PlanSHA256          string `json:"plan_sha256"`
		ConfirmedPlanSHA256 string `json:"confirmed_plan_sha256"`
	}
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode computed-plan audit JSON: %v\nbody:\n%s", err, string(rawAudit))
	}
	if audit.PlanSHA256 != planSHA256 {
		t.Fatalf("expected audit to carry computed plan checksum %q, got %#v", planSHA256, audit)
	}
	if audit.ConfirmedPlanSHA256 != "" {
		t.Fatalf("expected unconfirmed apply audit to leave confirmed_plan_sha256 empty, got %#v", audit)
	}
	body := string(rawAudit)
	if strings.Contains(body, "memory://computed-plan-audit") || strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") {
		t.Fatalf("computed-plan audit file must stay metadata-only, got %s", body)
	}
}

func TestRunApplyRejectsMismatchedPlanSHA256BeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://plan-mismatch", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", strings.Repeat("0", 64)}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected plan checksum mismatch to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected plan checksum mismatch not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plan sha256 mismatch") {
		t.Fatalf("expected plan checksum mismatch guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected plan checksum mismatch guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyRejectsMalformedPlanSHA256AsUsageError(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://plan-malformed", "--ledger-snapshot", "-", "--target-version", "1", "--plan-sha256", "not-a-sha"}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected malformed plan checksum to exit 2, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected malformed plan checksum not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid --plan-sha256") {
		t.Fatalf("expected malformed plan checksum guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected malformed plan checksum guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyWritesAuditFileAfterSuccessfulMutation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	auditPath := t.TempDir() + "/migration-apply-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://audit", "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected audited apply command, exit=%d stderr=%q", code, stderr.String())
	}
	var result dbmigrations.ApplyResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode apply result JSON: %v\nbody:\n%s", err, stdout.String())
	}
	rawAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	var audit migrationApplyAudit
	if err := json.Unmarshal(rawAudit, &audit); err != nil {
		t.Fatalf("decode audit JSON: %v\nbody:\n%s", err, string(rawAudit))
	}
	if audit.Format != migrationApplyAuditFormat {
		t.Fatalf("unexpected audit format: %#v", audit)
	}
	if audit.Driver != driverName || audit.DSNConfigured != true {
		t.Fatalf("unexpected audit database metadata: %#v", audit)
	}
	if audit.TargetVersion != 1 || audit.TargetLatest {
		t.Fatalf("unexpected audit target metadata: %#v", audit)
	}
	if audit.Result.PreviousVersion != result.PreviousVersion || audit.Result.CurrentVersion != result.CurrentVersion || audit.Result.LatestVersion != result.LatestVersion || len(audit.Result.Applied) != len(result.Applied) {
		t.Fatalf("audit result does not match stdout result: audit=%#v stdout=%#v", audit.Result, result)
	}
	if audit.LedgerSnapshotSHA256 != testSHA256HexBytes(rawSnapshot) {
		t.Fatalf("unexpected audit ledger snapshot checksum: got %q want %q", audit.LedgerSnapshotSHA256, testSHA256HexBytes(rawSnapshot))
	}
	if audit.AppliedAt == "" {
		t.Fatalf("expected audit to include applied_at timestamp: %#v", audit)
	}
	body := string(rawAudit)
	for _, forbidden := range []string{"memory://audit", "CREATE TABLE", "DROP TABLE", "-- go-metin2 migration"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("audit file must not expose %q, got %s", forbidden, body)
		}
	}
}

func TestRunApplyRejectsAuditFileWhenNoMigrationWouldRun(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	applied := []dbmigrations.LedgerEntry{{Version: catalog[0].Version, Name: catalog[0].Name, UpSHA256: catalog[0].UpSHA256}}
	currentMigrateCLITestDriver(t).setLedger(applied)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot(applied)
	if err != nil {
		t.Fatalf("marshal ledger snapshot: %v", err)
	}
	auditPath := t.TempDir() + "/noop-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://noop-audit", "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected no-op audited apply to fail closed, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected rejected no-op audit not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "audit file requires at least one applied migration") {
		t.Fatalf("expected no-op audit guidance, got %q", stderr.String())
	}
	if _, err := os.Stat(auditPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no audit file for rejected no-op apply, got %v", err)
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected no-op audit guard before opening DB, got events %#v", got)
	}
}

func TestRunApplyRejectsAuditFileOverwriteBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	auditPath := t.TempDir() + "/existing-audit.json"
	if err := os.WriteFile(auditPath, []byte("existing\n"), 0o600); err != nil {
		t.Fatalf("seed existing audit file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://overwrite-audit", "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected audit overwrite to fail closed, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected audit overwrite failure not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "audit file already exists") {
		t.Fatalf("expected audit overwrite guidance, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected audit overwrite guard before opening DB, got events %#v", got)
	}
	if raw, err := os.ReadFile(auditPath); err != nil || string(raw) != "existing\n" {
		t.Fatalf("expected existing audit file to remain unchanged, raw=%q err=%v", string(raw), err)
	}
}

func TestRunApplyRemovesReservedAuditFileWhenApplyFails(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	auditPath := t.TempDir() + "/failed-audit.json"
	currentMigrateCLITestDriver(t).setError(fmt.Errorf("migration target refused write"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://failed-audit", "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failed audited apply to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected failed audited apply not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "migration target refused write") {
		t.Fatalf("expected apply failure on stderr, got %q", stderr.String())
	}
	if _, err := os.Stat(auditPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected reserved audit file to be removed after failed apply, got %v", err)
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); !containsMigrateCLITestEventPrefix(got, "rollback") {
		t.Fatalf("expected failed apply to roll back, got events %#v", got)
	}
}

func TestRunApplyRejectsAuditFileWhenParentDirectoryIsMissing(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	rawSnapshot, err := dbmigrations.MarshalJSONLedgerSnapshot([]dbmigrations.LedgerEntry{})
	if err != nil {
		t.Fatalf("marshal empty ledger snapshot: %v", err)
	}
	auditPath := t.TempDir() + "/missing/failed-audit.json"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply", "--driver", driverName, "--dsn", "memory://missing-parent-audit", "--ledger-snapshot", "-", "--target-version", "1", "--audit-file", auditPath}, bytes.NewReader(rawSnapshot), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected missing audit parent to exit 1, got exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected missing audit parent not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "create audit file") {
		t.Fatalf("expected audit create error, got %q", stderr.String())
	}
	if got := currentMigrateCLITestDriver(t).eventsSnapshot(); len(got) != 0 {
		t.Fatalf("expected missing audit parent guard before opening DB, got events %#v", got)
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
	mu       sync.Mutex
	events   []string
	ledger   []dbmigrations.LedgerEntry
	err      error
	openHook func() error
}

func (d *migrateCLITestDriver) Open(name string) (driver.Conn, error) {
	d.record("open:" + name)
	if err := d.runOpenHook(); err != nil {
		return nil, err
	}
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

func (d *migrateCLITestDriver) setOpenHook(hook func() error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.openHook = hook
}

func (d *migrateCLITestDriver) runOpenHook() error {
	d.mu.Lock()
	hook := d.openHook
	d.mu.Unlock()
	if hook == nil {
		return nil
	}
	return hook()
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

func testSHA256HexBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
