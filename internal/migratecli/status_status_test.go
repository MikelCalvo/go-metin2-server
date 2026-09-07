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

type statusStatusGot struct {
	Format                string             `json:"format"`
	Present               bool               `json:"present"`
	StatusSHA256          string             `json:"status_sha256"`
	MatchesEmbeddedLatest bool               `json:"matches_embedded_latest"`
	Plan                  *dbmigrations.Plan `json:"plan"`
}

func TestRunStatusStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-post-apply-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing status-status not to write stderr, got %q", stderr.String())
	}
	var got statusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-status-status-v1" || got.Present || got.Plan != nil || got.StatusSHA256 != "" || got.MatchesEmbeddedLatest {
		t.Fatalf("unexpected missing status-status: %#v", got)
	}
	for _, field := range []string{`"plan"`, `"status_sha256"`, `"matches_embedded_latest"`} {
		if strings.Contains(stdout.String(), field) {
			t.Fatalf("missing status-status must omit inner fields, got %s", stdout.String())
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("status-status must not open a database target, got events %#v", events)
	}
}

func TestRunStatusStatusReadsUpToDatePlanWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustMarshalPlanJSON(t, mustUpToDatePlan(t))
	statusPath := mustWriteStatusFile(t, raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", statusPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on status-status success, got %q", stderr.String())
	}
	var got statusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-status-status-v1" || !got.Present || got.Plan == nil {
		t.Fatalf("unexpected status-status envelope: %#v", got)
	}
	if got.StatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected status_sha256: got %s want %s", got.StatusSHA256, sha256Hex(raw))
	}
	if !got.MatchesEmbeddedLatest {
		t.Fatalf("expected matches_embedded_latest true for current catalog, got %#v", got)
	}
	if !got.Plan.UpToDate || got.Plan.CurrentVersion != got.Plan.LatestVersion || len(got.Plan.Pending) != 0 {
		t.Fatalf("expected up-to-date inner plan, got %#v", got.Plan)
	}
	if !strings.Contains(stdout.String(), `"matches_embedded_latest": true`) {
		t.Fatalf("expected matches_embedded_latest true in JSON, got %s", stdout.String())
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("status-status must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(statusPath); err != nil {
		t.Fatalf("status-status must not remove the inspected status file: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("status-status must not open a database target, got events %#v", events)
	}
}

func TestRunStatusStatusReportsEmptyLedgerPlanUngated(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustMarshalPlanJSON(t, mustEmptyLedgerPlan(t))
	statusPath := mustWriteStatusFile(t, raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", statusPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected empty-ledger status-status to succeed ungated, exit=%d stderr=%q", code, stderr.String())
	}
	var got statusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode empty-ledger status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || got.Plan == nil || got.Plan.UpToDate || got.Plan.CurrentVersion != 0 || !got.MatchesEmbeddedLatest {
		t.Fatalf("expected present empty-ledger plan with up_to_date false, got %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("status-status must not open a database target, got events %#v", events)
	}
}

func TestRunStatusStatusReportsDriftedInternallyConsistentPlan(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustMarshalPlanJSON(t, driftedSingleStepPlan(t))
	statusPath := mustWriteStatusFile(t, raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", statusPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected drifted status-status to succeed ungated, exit=%d stderr=%q", code, stderr.String())
	}
	var got statusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode drifted status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || got.MatchesEmbeddedLatest || got.Plan == nil || got.Plan.LatestVersion != 1 || got.Plan.UpToDate {
		t.Fatalf("expected present drifted plan with matches_embedded_latest false, got %#v", got)
	}
	if got.StatusSHA256 != sha256Hex(raw) {
		t.Fatalf("unexpected status_sha256: got %s want %s", got.StatusSHA256, sha256Hex(raw))
	}
	if !strings.Contains(stdout.String(), `"matches_embedded_latest": false`) {
		t.Fatalf("expected matches_embedded_latest false in JSON, got %s", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("status-status must not open a database target, got events %#v", events)
	}
}

func TestRunStatusStatusRequireUpToDateRejectsMissing(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-post-apply-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", missing, "--require-up-to-date"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-up-to-date missing status-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-up-to-date miss, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-up-to-date failed: status is absent") {
		t.Fatalf("expected require-up-to-date/absent guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("status-status must not open a database target, got events %#v", events)
	}
}

func TestRunStatusStatusRequireUpToDateRejectsPendingPlan(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	statusPath := mustWriteStatusFile(t, mustMarshalPlanJSON(t, mustEmptyLedgerPlan(t)))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", statusPath, "--require-up-to-date"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-up-to-date pending status-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-up-to-date pending plan, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-up-to-date failed: up_to_date=false") {
		t.Fatalf("expected require-up-to-date/up_to_date=false guidance, got %q", stderr.String())
	}
}

func TestRunStatusStatusRequireMatchesEmbeddedLatestRejectsMissing(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-post-apply-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", missing, "--require-matches-embedded-latest"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-matches-embedded-latest missing status-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-matches-embedded-latest miss, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-matches-embedded-latest failed: status is absent") {
		t.Fatalf("expected require-matches-embedded-latest/absent guidance, got %q", stderr.String())
	}
}

func TestRunStatusStatusRequireMatchesEmbeddedLatestRejectsDrifted(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	statusPath := mustWriteStatusFile(t, mustMarshalPlanJSON(t, driftedSingleStepPlan(t)))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", statusPath, "--require-matches-embedded-latest"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-matches-embedded-latest drifted status-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-matches-embedded-latest drift, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-matches-embedded-latest failed: matches_embedded_latest=false") {
		t.Fatalf("expected require-matches-embedded-latest guidance, got %q", stderr.String())
	}
}

func TestRunStatusStatusRequireUpToDateFirstOnAbsentPath(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-post-apply-status.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"status-status",
		"--status", missing,
		"--require-up-to-date",
		"--require-matches-embedded-latest",
	}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected combined require gates to fail closed, exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on combined require-gate failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--require-up-to-date failed: status is absent") {
		t.Fatalf("expected --require-up-to-date to be reported first, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "--require-matches-embedded-latest") {
		t.Fatalf("expected later gates not to be reported after --require-up-to-date, got %q", stderr.String())
	}
}

func TestRunStatusStatusDashPathDoesNotReadStdin(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := mustMarshalPlanJSON(t, mustUpToDatePlan(t))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"status-status", "--status", "-"}, bytes.NewReader(raw), &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing '-' status-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got statusStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode dash-path status-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Present {
		t.Fatalf("status-status must treat '-' as a filesystem path, not stdin, got %#v body=%s", got, stdout.String())
	}
}

func TestRunStatusStatusRejectsWrappingAndMalformedPlans(t *testing.T) {
	emptyLedger := mustEmptyLedgerPlan(t)
	emptyLedger.UpToDate = true
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "wrapping-status-status",
			raw:  []byte(`{"format":"go-metin2-migration-status-status-v1","present":false}` + "\n"),
			want: "format",
		},
		{
			name: "plan-artifact-envelope",
			raw:  []byte(`{"format":"` + migrationPlanArtifactFormat + `","plan_sha256":"` + strings.Repeat("0", 64) + `","plan":{"current_version":0,"latest_version":1,"up_to_date":true,"pending":[]}}` + "\n"),
			want: "format",
		},
		{
			name: "unknown-field",
			raw:  []byte(`{"current_version":0,"latest_version":1,"up_to_date":true,"pending":[],"extra":true}` + "\n"),
			want: "unknown field",
		},
		{
			name: "latest-version-zero",
			raw:  []byte(`{"current_version":0,"latest_version":0,"up_to_date":true,"pending":[]}` + "\n"),
			want: "latest_version",
		},
		{
			name: "up-to-date-pending-mismatch",
			raw:  mustMarshalPlanJSON(t, emptyLedger),
			want: "up_to_date",
		},
		{
			name: "empty",
			raw:  []byte("   \n"),
			want: "empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStatusStatusRejectsInvalidFile(t, tc.raw, tc.want)
		})
	}
}

func TestRunStatusStatusRejectsSymlinkOversizedUnknownField(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()

	targetPath := filepath.Join(dir, "target-post-apply-status.json")
	mustWriteFile(t, targetPath, mustMarshalPlanJSON(t, mustUpToDatePlan(t)))
	symlinkPath := filepath.Join(dir, "post-apply-status.json")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("create symlink status: %v", err)
	}

	oversizedPath := filepath.Join(dir, "oversized-post-apply-status.json")
	mustWriteFile(t, oversizedPath, bytes.Repeat([]byte("a"), 64*1024+1))

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "symlink", path: symlinkPath, want: "must not be a symlink"},
		{name: "oversized", path: oversizedPath, want: "exceeds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"status-status", "--status", tc.path}, nil, &stdout, &stderr)
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
				t.Fatalf("status-status must not open a database target, got events %#v", events)
			}
		})
	}
}

func TestRunStatusStatusRejectsUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-flag", args: []string{"status-status"}, want: "--status"},
		{name: "unexpected-arg", args: []string{"status-status", "--status", "/tmp/x.json", "extra"}, want: "unexpected status-status argument"},
		{name: "unknown-flag", args: []string{"status-status", "--status", "/tmp/x.json", "--nope"}, want: "flag provided but not defined"},
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
			if !strings.Contains(stderr.String(), "status-status usage:") {
				t.Fatalf("expected status-status usage guidance, got %q", stderr.String())
			}
		})
	}
}

func TestRunStatusStatusUsageListsRequireFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"status-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	for _, want := range []string{"--status", "--require-up-to-date", "--require-matches-embedded-latest"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected usage to list %s, got %q", want, stderr.String())
		}
	}
}

func TestRunHelpListsStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "  status-status ") {
		t.Fatalf("expected help to list status-status as its own command, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "status-status usage:") {
		t.Fatalf("expected status-status usage block, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "status                 ") || !strings.Contains(stdout.String(), "plan-artifact-status") || !strings.Contains(stdout.String(), "migration-run-retention") {
		t.Fatalf("expected usage to list status-status beside status / plan-artifact-status / migration-run-retention, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsStatusStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "  status-status ") {
		t.Fatalf("expected usage to mention status-status as its own command, got %q", stderr.String())
	}
}

func assertStatusStatusRejectsInvalidFile(t *testing.T, raw []byte, wantErr string) {
	t.Helper()
	_ = registerMigrateCLITestSQLDriver(t)
	statusPath := mustWriteStatusFile(t, raw)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"status-status", "--status", statusPath}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected invalid status-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected invalid status-status not to write stdout, got %q", stdout.String())
	}
	if wantErr != "" && !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(wantErr)) {
		t.Fatalf("expected stderr to mention %q, got %q", wantErr, stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("status-status must not open a database target, got events %#v", events)
	}
}

func mustWriteStatusFile(t *testing.T, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "post-apply-status.json")
	mustWriteFile(t, path, raw)
	return path
}

func mustMarshalPlanJSON(t *testing.T, plan dbmigrations.Plan) []byte {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		t.Fatalf("encode plan JSON: %v", err)
	}
	return buf.Bytes()
}

func mustUpToDatePlan(t *testing.T) dbmigrations.Plan {
	t.Helper()
	plan, err := dbmigrations.PlanUpToLatest(catalogTipLedger(t))
	if err != nil {
		t.Fatalf("plan up to latest: %v", err)
	}
	if !plan.UpToDate || plan.CurrentVersion != plan.LatestVersion {
		t.Fatalf("expected up-to-date catalog-tip plan, got %#v", plan)
	}
	return plan
}

func mustEmptyLedgerPlan(t *testing.T) dbmigrations.Plan {
	t.Helper()
	plan, err := dbmigrations.PlanUpToLatest(nil)
	if err != nil {
		t.Fatalf("plan empty ledger: %v", err)
	}
	if plan.UpToDate || plan.CurrentVersion != 0 || len(plan.Pending) == 0 {
		t.Fatalf("expected empty-ledger pending plan, got %#v", plan)
	}
	return plan
}

func driftedSingleStepPlan(t *testing.T) dbmigrations.Plan {
	t.Helper()
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if len(catalog) < 2 {
		t.Fatalf("expected catalog to have more than one migration, got %d", len(catalog))
	}
	first := catalog[0]
	return dbmigrations.Plan{
		CurrentVersion: 0,
		LatestVersion:  1,
		UpToDate:       false,
		Pending: []dbmigrations.PlanStep{{
			Version:   first.Version,
			Name:      first.Name,
			Direction: dbmigrations.DirectionUp,
			Path:      first.UpPath,
			SHA256:    first.UpSHA256,
		}},
	}
}

func catalogTipLedger(t *testing.T) []dbmigrations.LedgerEntry {
	t.Helper()
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	entries := make([]dbmigrations.LedgerEntry, len(catalog))
	for i, migration := range catalog {
		entries[i] = dbmigrations.LedgerEntry{
			Version:  migration.Version,
			Name:     migration.Name,
			UpSHA256: migration.UpSHA256,
		}
	}
	return entries
}
