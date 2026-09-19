package migratecli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
)

type applyBoundaryGot struct {
	Format                      string   `json:"format"`
	CLIApply                    string   `json:"cli_apply"`
	CLIRollbackFlag             string   `json:"cli_rollback_flag"`
	CLIApplyIsMutating          bool     `json:"cli_apply_is_mutating"`
	DaemonOpsMutatingPaths      []string `json:"daemon_ops_mutating_paths"`
	DaemonOpsMutatingRegistered bool     `json:"daemon_ops_mutating_registered"`
	DaemonOpsReadOnlyPaths      []string `json:"daemon_ops_read_only_paths"`
}

func TestRunApplyBoundaryWritesMetadataOnlyContractWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-boundary"}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected apply-boundary to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on apply-boundary success, got %q", stderr.String())
	}
	got := decodeApplyBoundary(t, stdout.Bytes())
	if got.Format != applyBoundaryFormat {
		t.Fatalf("unexpected apply-boundary format: %#v", got)
	}
	if got.CLIApply != applyBoundaryCLIApply || !got.CLIApplyIsMutating {
		t.Fatalf("expected CLI apply to stay the mutating surface, got %#v", got)
	}
	if got.CLIRollbackFlag != applyBoundaryCLIRollbackFlag {
		t.Fatalf("expected CLI rollback acknowledgement %q, got %#v", applyBoundaryCLIRollbackFlag, got)
	}
	if got.DaemonOpsMutatingRegistered {
		t.Fatalf("daemon ops must not register apply/rollback, got %#v", got)
	}
	if !reflect.DeepEqual(got.DaemonOpsMutatingPaths, applyBoundaryMutatingPaths) {
		t.Fatalf("unexpected mutating paths: %#v", got.DaemonOpsMutatingPaths)
	}
	if !reflect.DeepEqual(got.DaemonOpsReadOnlyPaths, applyBoundaryReadOnlyPaths) {
		t.Fatalf("unexpected read-only paths: %#v", got.DaemonOpsReadOnlyPaths)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL", "memory://", "postgres://", "password=", `"dsn"`, "DSN"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("apply-boundary must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-boundary must not open a database target, got events %#v", events)
	}
}

func TestRunApplyBoundaryRejectsExtraArgsAsUsage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-boundary", "--remote-apply"}, nil, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stdout=%q stderr=%q", exitUsage, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on apply-boundary usage error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "apply-boundary usage:") {
		t.Fatalf("expected apply-boundary usage, got %q", stderr.String())
	}
}

func TestRunHelpListsApplyBoundary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "  apply-boundary ") {
		t.Fatalf("expected help to list apply-boundary as its own command, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "apply-boundary usage:") {
		t.Fatalf("expected apply-boundary usage block, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "  apply ") || !strings.Contains(stdout.String(), "  catalog ") {
		t.Fatalf("expected usage to list apply-boundary beside apply / catalog, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsApplyBoundary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "  apply-boundary ") {
		t.Fatalf("expected usage to mention apply-boundary as its own command, got %q", stderr.String())
	}
}

func TestApplyBoundaryCLIPathsMatchOpsConstants(t *testing.T) {
	if !reflect.DeepEqual(applyBoundaryReadOnlyPaths, ops.LocalDBMigrationReadOnlyPaths()) {
		t.Fatalf("CLI read-only paths drifted from ops: cli=%#v ops=%#v", applyBoundaryReadOnlyPaths, ops.LocalDBMigrationReadOnlyPaths())
	}
	if !reflect.DeepEqual(applyBoundaryMutatingPaths, ops.LocalDBMigrationMutatingPaths()) {
		t.Fatalf("CLI mutating paths drifted from ops: cli=%#v ops=%#v", applyBoundaryMutatingPaths, ops.LocalDBMigrationMutatingPaths())
	}
}

func TestApplyBoundaryLeavesDaemonApplyUnregisteredOnReadOnlyMux(t *testing.T) {
	mux := newReadOnlyMigrationMux(t)
	loopback := "127.0.0.1:12345"

	for _, path := range applyBoundaryReadOnlyPaths {
		req := httptest.NewRequest(readOnlyMigrationMethod(path), registeredReadOnlyMigrationURL(path), nil)
		req.RemoteAddr = loopback
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("read-only path %s must stay registered, got %d", path, rec.Code)
		}
		if rec.Code == http.StatusForbidden {
			t.Fatalf("loopback caller must reach read-only path %s, got %d", path, rec.Code)
		}
	}

	remote := httptest.NewRequest(http.MethodGet, ops.LocalDBMigrationsCatalogPath, nil)
	remote.RemoteAddr = "198.51.100.10:12345"
	remoteRec := httptest.NewRecorder()
	mux.ServeHTTP(remoteRec, remote)
	if remoteRec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback catalog GET must stay 403, got %d", remoteRec.Code)
	}

	for _, path := range applyBoundaryMutatingPaths {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
			req := httptest.NewRequest(method, path, nil)
			req.RemoteAddr = loopback
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("daemon %s %s must stay unregistered, got %d body=%q", method, path, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestApplyBoundaryLeavesDaemonApplyUnregisteredOnDefaultMux(t *testing.T) {
	mux := ops.NewPprofMux("gamed")
	for _, path := range applyBoundaryMutatingPaths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"dsn":"postgres://secret"}`))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("default mux must not register %s, got %d body=%q", path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "postgres://") || strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("unregistered apply path must not echo a DSN, got %q", rec.Body.String())
		}
	}
}

func decodeApplyBoundary(t *testing.T, raw []byte) applyBoundaryGot {
	t.Helper()
	var got applyBoundaryGot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode apply-boundary JSON: %v\nbody:\n%s", err, raw)
	}
	if got.DaemonOpsMutatingPaths == nil || got.DaemonOpsReadOnlyPaths == nil {
		t.Fatalf("path arrays must not be null, got %#v body=%s", got, raw)
	}
	return got
}

func newReadOnlyMigrationMux(t *testing.T) *http.ServeMux {
	t.Helper()
	plan := dbmigrations.Plan{CurrentVersion: 0, LatestVersion: 1, UpToDate: false, Pending: []dbmigrations.PlanStep{}}
	catalog := dbmigrations.CatalogSummaryPayload{Format: dbmigrations.CatalogSummaryFormat, LatestVersion: 1, Migrations: []dbmigrations.CatalogSummaryEntry{}}
	snapshot := dbmigrations.LedgerSnapshot{Format: dbmigrations.LedgerSnapshotFormat, Entries: []dbmigrations.LedgerEntry{}}
	mux := ops.NewPprofMux("gamed")
	mux = ops.RegisterLocalMigrationCatalogEndpoint(mux, func() (dbmigrations.CatalogSummaryPayload, error) { return catalog, nil })
	mux = ops.RegisterLocalMigrationStatusEndpoint(mux, func() (dbmigrations.Plan, error) { return plan, nil })
	mux = ops.RegisterLocalMigrationPlanEndpoint(mux, func(int) (dbmigrations.Plan, error) { return plan, nil })
	mux = ops.RegisterLocalMigrationLedgerSnapshotEndpoint(mux, func() (dbmigrations.LedgerSnapshot, error) { return snapshot, nil })
	mux = ops.RegisterLocalMigrationLedgerSnapshotPlanEndpoint(mux, func(dbmigrations.LedgerSnapshot, int) (dbmigrations.Plan, error) { return plan, nil })
	return mux
}

func readOnlyMigrationMethod(path string) string {
	if path == ops.LocalDBMigrationsPlanFromLedgerSnapshotPath {
		return http.MethodPost
	}
	return http.MethodGet
}

func registeredReadOnlyMigrationURL(path string) string {
	switch path {
	case ops.LocalDBMigrationsPlanPath, ops.LocalDBMigrationsPlanFromLedgerSnapshotPath:
		return path + "?target_version=0"
	default:
		return path
	}
}
