package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

type catalogStatusGot struct {
	Format          string                              `json:"format"`
	Present         bool                                `json:"present"`
	CatalogSHA256   string                              `json:"catalog_sha256"`
	MatchesEmbedded bool                                `json:"matches_embedded"`
	Catalog         *dbmigrations.CatalogSummaryPayload `json:"catalog"`
}

func TestRunCatalogStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-migration-catalog.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing catalog-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing catalog-status not to write stderr, got %q", stderr.String())
	}
	var got catalogStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing catalog-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-catalog-status-v1" || got.Present || got.Catalog != nil || got.CatalogSHA256 != "" || got.MatchesEmbedded {
		t.Fatalf("unexpected missing catalog-status: %#v", got)
	}
	if strings.Contains(stdout.String(), `"catalog"`) || strings.Contains(stdout.String(), `"catalog_sha256"`) || strings.Contains(stdout.String(), `"matches_embedded"`) {
		t.Fatalf("missing catalog-status must omit inner fields, got %s", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusReadsEmbeddedCatalogWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustCaptureCatalogJSON(t)
	catalogPath := filepath.Join(t.TempDir(), "migration-catalog.json")
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", catalogPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected catalog-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on catalog-status success, got %q", stderr.String())
	}
	var got catalogStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode catalog-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-catalog-status-v1" || !got.Present || got.Catalog == nil {
		t.Fatalf("unexpected catalog-status envelope: %#v", got)
	}
	if got.CatalogSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected catalog_sha256: got %s want %s", got.CatalogSHA256, sha256Hex(raw))
	}
	if !got.MatchesEmbedded {
		t.Fatalf("expected matches_embedded true for current catalog, got %#v", got)
	}
	if !strings.Contains(stdout.String(), `"matches_embedded": true`) {
		t.Fatalf("expected matches_embedded true in JSON, got %s", stdout.String())
	}
	summary, err := dbmigrations.BuiltInCatalogSummary()
	if err != nil {
		t.Fatalf("built-in catalog summary: %v", err)
	}
	if got.Catalog.Format != dbmigrations.CatalogSummaryFormat || got.Catalog.LatestVersion != summary.LatestVersion || len(got.Catalog.Migrations) != len(summary.Migrations) {
		t.Fatalf("unexpected inner catalog: %#v", got.Catalog)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("catalog-status must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(catalogPath); err != nil {
		t.Fatalf("catalog-status must not remove the inspected catalog file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusReportsDriftedInternallyConsistentCatalog(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustMarshalCatalogSummary(t, driftedSingleRowCatalog(t))
	catalogPath := filepath.Join(t.TempDir(), "migration-catalog.json")
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatalf("write drifted catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", catalogPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected drifted catalog-status to succeed ungated, exit=%d stderr=%q", code, stderr.String())
	}
	var got catalogStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode drifted catalog-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || got.MatchesEmbedded || got.Catalog == nil || got.Catalog.LatestVersion != 1 {
		t.Fatalf("expected present drifted catalog with matches_embedded false, got %#v", got)
	}
	if got.CatalogSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected catalog_sha256: got %s want %s", got.CatalogSHA256, sha256Hex(raw))
	}
	if !strings.Contains(stdout.String(), `"matches_embedded": false`) {
		t.Fatalf("expected matches_embedded false in JSON, got %s", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusRequireMatchesEmbeddedRejectsMissing(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-migration-catalog.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", missing, "--require-matches-embedded"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-matches-embedded missing catalog-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-matches-embedded miss, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-matches-embedded") && !strings.Contains(stderr.String(), "absent") && !strings.Contains(stderr.String(), "present") {
		t.Fatalf("expected require-matches-embedded/absent guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusRequireMatchesEmbeddedRejectsDrifted(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustMarshalCatalogSummary(t, driftedSingleRowCatalog(t))
	catalogPath := filepath.Join(t.TempDir(), "migration-catalog.json")
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatalf("write drifted catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", catalogPath, "--require-matches-embedded"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-matches-embedded drifted catalog-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-matches-embedded drift, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-matches-embedded") && !strings.Contains(stderr.String(), "matches_embedded") {
		t.Fatalf("expected require-matches-embedded guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusRejectsVersionGap(t *testing.T) {
	summary := driftedSingleRowCatalog(t)
	second := summary.Migrations[0]
	second.Version = 3
	second.Name = "future_table"
	second.UpPath = "0003_future_table.up.sql"
	second.DownPath = "0003_future_table.down.sql"
	summary.LatestVersion = 2
	summary.Migrations = append(summary.Migrations, second)
	assertCatalogStatusRejectsInvalidFile(t, mustMarshalCatalogSummary(t, summary), "version")
}

func TestRunCatalogStatusRejectsLatestVersionMismatch(t *testing.T) {
	summary := driftedSingleRowCatalog(t)
	summary.LatestVersion = 2
	assertCatalogStatusRejectsInvalidFile(t, mustMarshalCatalogSummary(t, summary), "latest_version")
}

func TestRunCatalogStatusRejectsPathMismatch(t *testing.T) {
	summary := driftedSingleRowCatalog(t)
	summary.Migrations[0].UpPath = "0001_wrong.up.sql"
	assertCatalogStatusRejectsInvalidFile(t, mustMarshalCatalogSummary(t, summary), "up_path")
}

func TestRunCatalogStatusRejectsUppercaseChecksum(t *testing.T) {
	summary := driftedSingleRowCatalog(t)
	summary.Migrations[0].UpSHA256 = strings.ToUpper(summary.Migrations[0].UpSHA256)
	assertCatalogStatusRejectsInvalidFile(t, mustMarshalCatalogSummary(t, summary), "sha256")
}

func TestRunCatalogStatusRejectsUnknownField(t *testing.T) {
	raw := []byte(`{"format":"go-metin2-migration-catalog-summary-v1","latest_version":1,"migrations":[],"extra":true}`)
	assertCatalogStatusRejectsInvalidFile(t, raw, "unknown field")
}

func TestRunCatalogStatusRejectsOwnStatusEnvelope(t *testing.T) {
	raw := []byte(`{"format":"go-metin2-migration-catalog-status-v1","present":false}`)
	assertCatalogStatusRejectsInvalidFile(t, raw, "format")
}

func TestRunCatalogStatusRejectsSymlink(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target-migration-catalog.json")
	if err := os.WriteFile(targetPath, mustCaptureCatalogJSON(t), 0o600); err != nil {
		t.Fatalf("write symlink catalog target: %v", err)
	}
	catalogPath := filepath.Join(dir, "migration-catalog.json")
	if err := os.Symlink(targetPath, catalogPath); err != nil {
		t.Fatalf("create symlink catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", catalogPath}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected symlink catalog-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected symlink catalog-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("expected symlink rejection guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusRejectsOversized(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := bytes.Repeat([]byte("a"), 64*1024+1)
	catalogPath := filepath.Join(t.TempDir(), "migration-catalog.json")
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatalf("write oversized catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", catalogPath}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected oversized catalog-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected oversized catalog-status not to write stdout, got %q", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusDoesNotReadStdinDash(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"catalog-status", "--catalog", "-"}, bytes.NewReader(mustCaptureCatalogJSON(t)), &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected catalog-status --catalog - to treat a missing file named -, exit=%d stderr=%q", code, stderr.String())
	}
	var got catalogStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode dash-path catalog-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Present {
		t.Fatalf("catalog-status must not treat - as stdin catalog, got %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func TestRunCatalogStatusUsageRequiresCatalogFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"catalog-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected catalog-status usage exit %d, got %d stdout=%q stderr=%q", exitUsage, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--catalog") {
		t.Fatalf("expected --catalog usage guidance, got %q", stderr.String())
	}
}

func TestRunCatalogStatusHelpListsCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "catalog-status") {
		t.Fatalf("expected help to list catalog-status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "catalog-status usage:") {
		t.Fatalf("expected catalog-status usage block, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--require-matches-embedded") {
		t.Fatalf("expected --require-matches-embedded in usage, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsCatalogStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "catalog-status") {
		t.Fatalf("expected usage to mention catalog-status, got %q", stderr.String())
	}
}

func assertCatalogStatusRejectsInvalidFile(t *testing.T, raw []byte, wantErr string) {
	t.Helper()
	_ = registerMigrateCLITestSQLDriver(t)
	catalogPath := filepath.Join(t.TempDir(), "migration-catalog.json")
	if err := os.WriteFile(catalogPath, raw, 0o600); err != nil {
		t.Fatalf("write invalid catalog: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"catalog-status", "--catalog", catalogPath}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected invalid catalog-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected invalid catalog-status not to write stdout, got %q", stdout.String())
	}
	if wantErr != "" && !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(wantErr)) {
		t.Fatalf("expected stderr to mention %q, got %q", wantErr, stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("catalog-status must not open a database target, got events %#v", events)
	}
}

func mustCaptureCatalogJSON(t *testing.T) []byte {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"catalog"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected catalog command to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	return stdout.Bytes()
}

func driftedSingleRowCatalog(t *testing.T) dbmigrations.CatalogSummaryPayload {
	t.Helper()
	summary, err := dbmigrations.BuiltInCatalogSummary()
	if err != nil {
		t.Fatalf("built-in catalog summary: %v", err)
	}
	if len(summary.Migrations) < 2 {
		t.Fatalf("expected built-in catalog to have more than one row, got %#v", summary)
	}
	return dbmigrations.CatalogSummaryPayload{
		Format:        dbmigrations.CatalogSummaryFormat,
		LatestVersion: 1,
		Migrations:    []dbmigrations.CatalogSummaryEntry{summary.Migrations[0]},
	}
}

func mustMarshalCatalogSummary(t *testing.T, summary dbmigrations.CatalogSummaryPayload) []byte {
	t.Helper()
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal catalog summary: %v", err)
	}
	return raw
}
