package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
)

type applyLockAsideStatusGot struct {
	Format          string                   `json:"format"`
	Present         bool                     `json:"present"`
	AsideSHA256     string                   `json:"aside_sha256"`
	AsidePathExists bool                     `json:"aside_path_exists"`
	LockFileExists  bool                     `json:"lock_file_exists"`
	Aside           *migrationApplyLockAside `json:"aside"`
}

func TestRunApplyLockAsideStatusReportsMissingWithoutOpeningDatabase(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-apply-lock-aside.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", missing}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected missing apply-lock-aside-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected missing apply-lock-aside-status not to write stderr, got %q", stderr.String())
	}
	var got applyLockAsideStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode missing apply-lock-aside-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-apply-lock-aside-status-v1" || got.Present || got.Aside != nil || got.AsideSHA256 != "" || got.AsidePathExists || got.LockFileExists {
		t.Fatalf("unexpected missing apply-lock-aside-status: %#v", got)
	}
	for _, field := range []string{`"aside"`, `"aside_sha256"`, `"aside_path_exists"`, `"lock_file_exists"`} {
		if strings.Contains(stdout.String(), field) {
			t.Fatalf("missing apply-lock-aside-status must omit inner fields, got %s", stdout.String())
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-aside-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockAsideStatusReadsRetainedAsideWithoutOpeningDatabase(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", fixture.jsonPath}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected apply-lock-aside-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on apply-lock-aside-status success, got %q", stderr.String())
	}
	var got applyLockAsideStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode apply-lock-aside-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Format != "go-metin2-migration-apply-lock-aside-status-v1" || !got.Present || got.Aside == nil {
		t.Fatalf("unexpected apply-lock-aside-status envelope: %#v", got)
	}
	if got.AsideSHA256 != sha256Hex(fixture.raw) {
		t.Fatalf("unexpected aside_sha256: got %s want %s", got.AsideSHA256, sha256Hex(fixture.raw))
	}
	if !got.AsidePathExists || got.LockFileExists {
		t.Fatalf("expected aside path present and original lock absent, got %#v", got)
	}
	if got.Aside.Format != migrationApplyLockAsideFormat || got.Aside.LockFile != fixture.lockPath || got.Aside.AsidePath != fixture.asidePath {
		t.Fatalf("unexpected inner aside: %#v", got.Aside)
	}
	if got.Aside.ManualClearCandidate == nil || !*got.Aside.ManualClearCandidate {
		t.Fatalf("expected retained candidate true, got %#v", got.Aside.ManualClearCandidate)
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("apply-lock-aside-status must not expose %q, got %s", forbidden, body)
		}
	}
	if _, err := os.Stat(fixture.jsonPath); err != nil {
		t.Fatalf("apply-lock-aside-status must not remove the inspected aside JSON: %v", err)
	}
	if _, err := os.Stat(fixture.asidePath); err != nil {
		t.Fatalf("apply-lock-aside-status must not remove the renamed lock: %v", err)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-aside-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockAsideStatusRequireGatesSucceedWhenAsidePresentAndLockAbsent(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"apply-lock-aside-status",
		"--aside", fixture.jsonPath,
		"--require-aside-path-exists",
		"--require-lock-file-absent",
	}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected gated apply-lock-aside-status to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	var got applyLockAsideStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode gated apply-lock-aside-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if !got.Present || !got.AsidePathExists || got.LockFileExists {
		t.Fatalf("unexpected gated apply-lock-aside-status: %#v", got)
	}
}

func TestRunApplyLockAsideStatusRequireAsidePathExistsRejectsMissingStaleCopy(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	if err := os.Remove(fixture.asidePath); err != nil {
		t.Fatalf("remove aside path: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", fixture.jsonPath, "--require-aside-path-exists"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-aside-path-exists miss to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-aside-path-exists miss, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-aside-path-exists") && !strings.Contains(stderr.String(), "aside_path") {
		t.Fatalf("expected require-aside-path-exists guidance, got %q", stderr.String())
	}
}

func TestRunApplyLockAsideStatusRequireLockFileAbsentRejectsRestoredLock(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	if err := os.WriteFile(fixture.lockPath, []byte("restored-lock\n"), 0o600); err != nil {
		t.Fatalf("restore original lock path: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", fixture.jsonPath, "--require-lock-file-absent"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-lock-file-absent collision to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-lock-file-absent collision, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "require-lock-file-absent") && !strings.Contains(stderr.String(), "lock_file") {
		t.Fatalf("expected require-lock-file-absent guidance, got %q", stderr.String())
	}
}

func TestRunApplyLockAsideStatusRequireGatesRejectMissingJSON(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	missing := filepath.Join(t.TempDir(), "missing-apply-lock-aside.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", missing, "--require-aside-path-exists"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected require-gate missing aside JSON to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on require-gate miss, got %q", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-aside-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockAsideStatusRejectsAsidePathMismatch(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	aside := fixture.decoded
	aside.AsidePath = fixture.lockPath + ".stale-19990101T000000Z"
	assertApplyLockAsideStatusRejectsInvalidFile(t, mustMarshalApplyLockAside(t, aside), "aside_path")
}

func TestRunApplyLockAsideStatusRejectsFalseCandidate(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	aside := fixture.decoded
	falseCandidate := false
	aside.ManualClearCandidate = &falseCandidate
	assertApplyLockAsideStatusRejectsInvalidFile(t, mustMarshalApplyLockAside(t, aside), "manual_clear_candidate")
}

func TestRunApplyLockAsideStatusRejectsUnknownField(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	raw := bytes.TrimSpace(mustMarshalApplyLockAside(t, fixture.decoded))
	if !bytes.HasSuffix(raw, []byte("}")) {
		t.Fatalf("expected marshaled aside JSON object, got %s", raw)
	}
	raw = append(raw[:len(raw)-1], []byte(`,"extra":true}`)...)
	assertApplyLockAsideStatusRejectsInvalidFile(t, raw, "unknown field")
}

func TestRunApplyLockAsideStatusRejectsOwnStatusEnvelope(t *testing.T) {
	raw := []byte(`{"format":"go-metin2-migration-apply-lock-aside-status-v1","present":false}`)
	assertApplyLockAsideStatusRejectsInvalidFile(t, raw, "format")
}

func TestRunApplyLockAsideStatusRejectsLockFileEnvelope(t *testing.T) {
	raw := []byte(`{"format":"go-metin2-migration-apply-lock-v1","created_at":"2026-08-17T00:00:00Z","pid":1,"hostname":"lab-host","build_version":"dev","build_commit":"none","build_date":"unknown","driver":"driver","dsn_configured":true,"target_version":1,"target_latest":false,"plan_sha256":"` + strings.Repeat("0", 64) + `","ledger_snapshot_sha256":"` + strings.Repeat("1", 64) + `"}`)
	assertApplyLockAsideStatusRejectsInvalidFile(t, raw, "format")
}

func TestRunApplyLockAsideStatusRejectsLockStatusEnvelope(t *testing.T) {
	raw := []byte(`{"format":"go-metin2-migration-apply-lock-status-v1","present":false}`)
	assertApplyLockAsideStatusRejectsInvalidFile(t, raw, "format")
}

func TestRunApplyLockAsideStatusRejectsSymlink(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	linkPath := filepath.Join(filepath.Dir(fixture.jsonPath), "apply-lock-aside-link.json")
	if err := os.Symlink(fixture.jsonPath, linkPath); err != nil {
		t.Fatalf("create symlink aside JSON: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", linkPath}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected symlink apply-lock-aside-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected symlink apply-lock-aside-status not to write stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("expected symlink rejection guidance, got %q", stderr.String())
	}
}

func TestRunApplyLockAsideStatusRejectsOversized(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	raw := bytes.Repeat([]byte("a"), 16*1024+1)
	asidePath := filepath.Join(t.TempDir(), "apply-lock-aside.json")
	if err := os.WriteFile(asidePath, raw, 0o600); err != nil {
		t.Fatalf("write oversized aside JSON: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", asidePath}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected oversized apply-lock-aside-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected oversized apply-lock-aside-status not to write stdout, got %q", stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-aside-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockAsideStatusDoesNotReadStdinDash(t *testing.T) {
	fixture := mustCaptureApplyLockAside(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"apply-lock-aside-status", "--aside", "-"}, bytes.NewReader(fixture.raw), &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected apply-lock-aside-status --aside - to treat a missing file named -, exit=%d stderr=%q", code, stderr.String())
	}
	var got applyLockAsideStatusGot
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode dash-path apply-lock-aside-status JSON: %v\nbody:\n%s", err, stdout.String())
	}
	if got.Present {
		t.Fatalf("apply-lock-aside-status must not treat - as stdin aside, got %#v", got)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-aside-status must not open a database target, got events %#v", events)
	}
}

func TestRunApplyLockAsideStatusUsageRequiresAsideFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"apply-lock-aside-status"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected apply-lock-aside-status usage exit %d, got %d stdout=%q stderr=%q", exitUsage, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--aside") {
		t.Fatalf("expected --aside usage guidance, got %q", stderr.String())
	}
}

func TestRunApplyLockAsideStatusHelpListsCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "apply-lock-aside-status") {
		t.Fatalf("expected help to list apply-lock-aside-status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "apply-lock-aside-status usage:") {
		t.Fatalf("expected apply-lock-aside-status usage block, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--require-aside-path-exists") || !strings.Contains(stdout.String(), "--require-lock-file-absent") {
		t.Fatalf("expected require flags in usage, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsApplyLockAsideStatus(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "apply-lock-aside-status") {
		t.Fatalf("expected usage to mention apply-lock-aside-status, got %q", stderr.String())
	}
}

func assertApplyLockAsideStatusRejectsInvalidFile(t *testing.T, raw []byte, wantErr string) {
	t.Helper()
	_ = registerMigrateCLITestSQLDriver(t)
	asidePath := filepath.Join(t.TempDir(), "apply-lock-aside.json")
	if err := os.WriteFile(asidePath, raw, 0o600); err != nil {
		t.Fatalf("write invalid aside JSON: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"apply-lock-aside-status", "--aside", asidePath}, nil, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected invalid apply-lock-aside-status to exit %d, got exit=%d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected invalid apply-lock-aside-status not to write stdout, got %q", stdout.String())
	}
	if wantErr != "" && !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(wantErr)) {
		t.Fatalf("expected stderr to mention %q, got %q", wantErr, stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("apply-lock-aside-status must not open a database target, got events %#v", events)
	}
}

type applyLockAsideFixture struct {
	lockPath  string
	asidePath string
	jsonPath  string
	raw       []byte
	decoded   migrationApplyLockAside
}

func mustCaptureApplyLockAside(t *testing.T) applyLockAsideFixture {
	t.Helper()
	_ = registerMigrateCLITestSQLDriver(t)
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "migration-apply.lock")
	createdAt := "2026-08-17T00:00:00Z"
	inspectAt := time.Date(2026, 8, 17, 1, 0, 0, 0, time.UTC)
	restore := setApplyLockStatusNow(t, inspectAt)
	t.Cleanup(restore)
	identity := buildinfo.Current()
	writeLabStaleApplyLockForTest(t, lockPath, migrationApplyLock{
		Format:               migrationApplyLockFormat,
		CreatedAt:            createdAt,
		PID:                  findAbsentLocalPID(t),
		Hostname:             mustLocalHostname(t),
		BuildVersion:         identity.Version,
		BuildCommit:          identity.Commit,
		BuildDate:            identity.BuildDate,
		Driver:               "example-driver",
		DSNConfigured:        true,
		TargetVersion:        1,
		TargetLatest:         false,
		PlanSHA256:           strings.Repeat("a", 64),
		LedgerSnapshotSHA256: strings.Repeat("b", 64),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"apply-lock-aside", "--lock-file", lockPath, "--i-confirm-lab-aside-rename"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected apply-lock-aside fixture to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	jsonPath := filepath.Join(dir, "apply-lock-aside.json")
	if err := os.WriteFile(jsonPath, stdout.Bytes(), 0o600); err != nil {
		t.Fatalf("retain aside JSON: %v", err)
	}
	var decoded migrationApplyLockAside
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode aside fixture: %v\nbody:\n%s", err, stdout.String())
	}
	return applyLockAsideFixture{
		lockPath:  lockPath,
		asidePath: lockAsidePath(lockPath, inspectAt),
		jsonPath:  jsonPath,
		raw:       stdout.Bytes(),
		decoded:   decoded,
	}
}

func mustMarshalApplyLockAside(t *testing.T, aside migrationApplyLockAside) []byte {
	t.Helper()
	raw, err := json.Marshal(aside)
	if err != nil {
		t.Fatalf("marshal aside JSON: %v", err)
	}
	return raw
}
