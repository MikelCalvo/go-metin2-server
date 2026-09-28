package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRunAdvisoryLockProbeDisabledWithoutConfirmation(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	secretDSN := "memory://secret-password@db/advisory-disabled"
	lockPath := filepath.Join(t.TempDir(), "migration-apply.lock")
	if err := os.WriteFile(lockPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"advisory-lock-probe", "--driver", driverName, "--dsn", secretDSN}, nil, &stdout, &stderr)

	if code != exitUsage {
		t.Fatalf("expected disabled probe to exit %d, got %d stdout=%q stderr=%q", exitUsage, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout when the probe stays disabled, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--i-confirm-read-only-advisory-probe is required") {
		t.Fatalf("expected confirmation guidance, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("disabled probe must not open a database, got %#v", events)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("disabled probe must leave the lock file in place: %v", err)
	}
}

func TestRunAdvisoryLockProbeReportsHeldWithoutClearingLock(t *testing.T) {
	driverName := registerPostgresAdvisoryLockProbeDriver(t)
	currentMigrateCLITestDriver(t).setAdvisoryHeld(true)
	secretDSN := "memory://secret-password@db/advisory-held"
	lockPath := filepath.Join(t.TempDir(), "migration-apply.lock")
	asidePath := filepath.Join(t.TempDir(), "apply-lock-aside.json")
	if err := os.WriteFile(lockPath, []byte("live-lock\n"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if err := os.WriteFile(asidePath, []byte("retained-aside\n"), 0o600); err != nil {
		t.Fatalf("write aside: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"advisory-lock-probe",
		"--driver", driverName,
		"--dsn", secretDSN,
		"--i-confirm-read-only-advisory-probe",
	}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected held probe to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on success, got %q", stderr.String())
	}
	got := decodeAdvisoryLockProbe(t, stdout.Bytes())
	if got.Format != advisoryLockProbeFormat || !got.Configured || !got.Held || got.Probe != advisoryLockProbeName || got.Engine != advisoryLockProbeEngine {
		t.Fatalf("unexpected held probe: %#v", got)
	}
	assertAdvisoryProbeReadOnly(t, secretDSN, stdout.String(), lockPath, asidePath)
	assertPostgresAdvisoryLockQuery(t)
}

func TestRunAdvisoryLockProbeReportsFreeWithoutProvingAside(t *testing.T) {
	driverName := registerPostgresAdvisoryLockProbeDriver(t)
	currentMigrateCLITestDriver(t).setAdvisoryHeld(false)
	secretDSN := "memory://secret-password@db/advisory-free"
	lockPath := filepath.Join(t.TempDir(), "migration-apply.lock")
	asidePath := filepath.Join(t.TempDir(), "apply-lock-aside.json")
	if err := os.WriteFile(lockPath, []byte("live-lock\n"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if err := os.WriteFile(asidePath, []byte("retained-aside\n"), 0o600); err != nil {
		t.Fatalf("write aside: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"advisory-lock-probe",
		"--driver", driverName,
		"--dsn", secretDSN,
		"--i-confirm-read-only-advisory-probe",
	}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected free probe to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	got := decodeAdvisoryLockProbe(t, stdout.Bytes())
	if got.Held {
		t.Fatalf("expected held=false, got %#v", got)
	}
	if got.Format != advisoryLockProbeFormat || !got.Configured || got.Probe != advisoryLockProbeName || got.Engine != advisoryLockProbeEngine {
		t.Fatalf("unexpected free probe: %#v", got)
	}
	assertAdvisoryProbeReadOnly(t, secretDSN, stdout.String(), lockPath, asidePath)
	if currentMigrateCLITestDriver(t).advisoryProbeCount() != 1 {
		t.Fatalf("expected one read-only probe, got %d events %#v", currentMigrateCLITestDriver(t).advisoryProbeCount(), currentMigrateCLITestDriver(t).eventsSnapshot())
	}
	assertPostgresAdvisoryLockQuery(t)
}

func TestRunAdvisoryLockProbeRejectsUnlinkedDriverBeforeOpen(t *testing.T) {
	secretDSN := "memory://secret-password@db/advisory-unlinked"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"advisory-lock-probe",
		"--driver", "go_metin2_unlinked_advisory",
		"--dsn", secretDSN,
		"--i-confirm-read-only-advisory-probe",
	}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected unlinked driver exit %d, got %d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout for an unlinked driver, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "database driver is unavailable") || strings.Contains(stderr.String(), secretDSN) {
		t.Fatalf("expected redacted unavailable-driver error, got %q", stderr.String())
	}
}

func TestRunAdvisoryLockProbeRejectsNonPostgresEngineBeforeOpen(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	secretDSN := "memory://secret-password@db/advisory-sqlite"
	lockPath := filepath.Join(t.TempDir(), "migration-apply.lock")
	if err := os.WriteFile(lockPath, []byte("live-lock\n"), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"advisory-lock-probe",
		"--driver", driverName,
		"--dsn", secretDSN,
		"--i-confirm-read-only-advisory-probe",
	}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected unsupported engine exit %d, got %d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout for an unsupported engine, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "engine unsupported") || !strings.Contains(stderr.String(), "pg_locks") || strings.Contains(stderr.String(), secretDSN) {
		t.Fatalf("expected redacted unsupported-engine error, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("unsupported engine must not open a database, got %#v", events)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("unsupported engine must leave the lock file in place: %v", err)
	}
	if strings.Contains(advisoryLockProbeQuery(), "go_metin2_session_advisory_lock") {
		t.Fatal("probe query must not select the absent stand-in table")
	}
}

func TestRunAdvisoryLockProbeRedactsDSNOnProbeFailure(t *testing.T) {
	driverName := registerPostgresAdvisoryLockProbeDriver(t)
	secretDSN := "memory://secret-password@db/advisory-error"
	currentMigrateCLITestDriver(t).setError(os.ErrInvalid)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"advisory-lock-probe",
		"--driver", driverName,
		"--dsn", secretDSN,
		"--i-confirm-read-only-advisory-probe",
	}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected probe failure exit %d, got %d", exitError, code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on probe failure, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), secretDSN) || !strings.Contains(stderr.String(), "<redacted-dsn>") {
		t.Fatalf("expected redacted probe error, got %q", stderr.String())
	}
	for _, forbidden := range []string{"UNLOCK", "pg_advisory_unlock", "RELEASE_LOCK", "rm ", "unlink"} {
		if strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(forbidden)) {
			t.Fatalf("probe failure must not suggest clearing a lock via %q, got %q", forbidden, stderr.String())
		}
	}
}

func TestRunHelpListsAdvisoryLockProbe(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "  advisory-lock-probe ") || !strings.Contains(stdout.String(), "advisory-lock-probe usage:") {
		t.Fatalf("expected help to list the disabled probe beside apply-lock-aside, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--i-confirm-read-only-advisory-probe") || !strings.Contains(stdout.String(), "pg_locks") {
		t.Fatalf("expected confirmation flag and pg_locks usage, got %q", stdout.String())
	}
}

var postgresAdvisoryProbeDriver struct {
	sync.Once
	driver *migrateCLITestDriver
}

func registerPostgresAdvisoryLockProbeDriver(t *testing.T) string {
	t.Helper()
	// database/sql names are process-global and cannot be unregistered, so the
	// postgres double is registered once. sql.Open returns that same object,
	// and each test swaps its held/error state. This is not a production driver import.
	name := advisoryLockProbeEngine
	postgresAdvisoryProbeDriver.Once.Do(func() {
		postgresAdvisoryProbeDriver.driver = &migrateCLITestDriver{}
	})
	driver := postgresAdvisoryProbeDriver.driver
	driver.resetAdvisoryProbeState()
	migrateCLITestDriverRegistry.Lock()
	migrateCLITestDriverRegistry.current = driver
	migrateCLITestDriverRegistry.Unlock()
	if err := registerSQLDriverOnce(name, driver); err != nil {
		t.Fatalf("register postgres advisory-lock probe double: %v", err)
	}
	t.Cleanup(func() {
		migrateCLITestDriverRegistry.Lock()
		if migrateCLITestDriverRegistry.current == driver {
			migrateCLITestDriverRegistry.current = nil
		}
		migrateCLITestDriverRegistry.Unlock()
		driver.resetAdvisoryProbeState()
	})
	return name
}

func decodeAdvisoryLockProbe(t *testing.T, raw []byte) advisoryLockProbe {
	t.Helper()
	var got advisoryLockProbe
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode advisory-lock probe: %v\nbody:\n%s", err, raw)
	}
	return got
}

func assertPostgresAdvisoryLockQuery(t *testing.T) {
	t.Helper()
	query := advisoryLockProbeQuery()
	for _, forbidden := range []string{"go_metin2_session_advisory_lock", "pg_advisory_lock", "pg_try_advisory_lock", "pg_advisory_unlock", "GET_LOCK", "IS_USED_LOCK", "RELEASE_LOCK"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("probe query must stay a read-only pg_locks catalog read, found %q in %s", forbidden, query)
		}
	}
	if !strings.Contains(query, "pg_locks") || !strings.Contains(query, "locktype = 'advisory'") {
		t.Fatalf("probe query must read pg_locks advisory rows, got %s", query)
	}
	events := currentMigrateCLITestDriver(t).eventsSnapshot()
	joined := strings.Join(events, "\n")
	if !strings.Contains(joined, "pg_locks") {
		t.Fatalf("expected one pg_locks query, got %#v", events)
	}
}

func assertAdvisoryProbeReadOnly(t *testing.T, secretDSN, body, lockPath, asidePath string) {
	t.Helper()
	for _, forbidden := range []string{secretDSN, "CREATE TABLE", "DROP TABLE", "UNLOCK", "pg_advisory_unlock", "RELEASE_LOCK", "rm ", "unlink", "go_metin2_session_advisory_lock"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("probe output must not contain %q, got %s", forbidden, body)
		}
	}
	lockRaw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("probe must leave the live lock in place: %v", err)
	}
	if string(lockRaw) != "live-lock\n" {
		t.Fatalf("probe rewrote the live lock: %q", lockRaw)
	}
	asideRaw, err := os.ReadFile(asidePath)
	if err != nil {
		t.Fatalf("probe must leave the retained aside in place: %v", err)
	}
	if string(asideRaw) != "retained-aside\n" {
		t.Fatalf("probe treated the retained aside as mutable: %q", asideRaw)
	}
	events := currentMigrateCLITestDriver(t).eventsSnapshot()
	for _, event := range events {
		lower := strings.ToLower(event)
		if strings.Contains(lower, "unlock") || strings.Contains(lower, "delete") || strings.Contains(lower, "insert") || strings.Contains(lower, "update") {
			t.Fatalf("probe must stay read-only, got events %#v", events)
		}
	}
}
