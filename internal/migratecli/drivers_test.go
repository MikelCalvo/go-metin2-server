package migratecli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type sqlDriversGot struct {
	Format  string          `json:"format"`
	Drivers json.RawMessage `json:"drivers"`
}

func TestRunDriversWritesEnvelopeWithoutOpeningDatabase(t *testing.T) {
	name := registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"drivers"}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected drivers to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr on drivers success, got %q", stderr.String())
	}
	got := decodeSQLDriversEnvelope(t, stdout.Bytes())
	if !sqlDriversListContains(t, got.Drivers, name) {
		t.Fatalf("expected linked test driver %q in drivers list, got %s", name, stdout.String())
	}
	body := stdout.String()
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL", "memory://", "postgres://", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("drivers must not expose %q, got %s", forbidden, body)
		}
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("drivers must not open a database target, got events %#v", events)
	}
}

func TestRunDriversRequireDriverAcceptsRegisteredName(t *testing.T) {
	name := registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"drivers", "--require-driver", name}, nil, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected required linked driver to succeed, exit=%d stderr=%q", code, stderr.String())
	}
	got := decodeSQLDriversEnvelope(t, stdout.Bytes())
	if !sqlDriversListContains(t, got.Drivers, name) {
		t.Fatalf("expected full linked list to include %q, got %s", name, stdout.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("drivers --require-driver must not open a database target, got events %#v", events)
	}
}

func TestRunDriversRequireDriverRejectsUnknownName(t *testing.T) {
	_ = registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"drivers", "--require-driver", "go_metin2_missing_driver"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected missing required driver to exit %d, got %d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout JSON on missing required driver, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "database driver is unavailable") || !strings.Contains(stderr.String(), "go_metin2_missing_driver") {
		t.Fatalf("expected unavailable-driver stderr, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("drivers --require-driver must not open a database target, got events %#v", events)
	}
}

func TestRunDriversRequireDriverRejectsEmptyName(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"drivers", "--require-driver", "  "}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected empty --require-driver to exit %d, got %d stdout=%q stderr=%q", exitUsage, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on empty --require-driver, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "drivers usage:") || !strings.Contains(stderr.String(), "--require-driver") {
		t.Fatalf("expected drivers usage for empty --require-driver, got %q", stderr.String())
	}
}

func TestRunDriversRequireEmptyStockReleaseRejectsLinkedDriver(t *testing.T) {
	name := registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"drivers", "--require-empty-stock-release"}, nil, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected linked stock-release gate to exit %d, got %d stdout=%q stderr=%q", exitError, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout JSON on nonempty stock-release gate, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stock release must not register a production database driver") {
		t.Fatalf("expected no-stock-driver stderr, got %q", stderr.String())
	}
	if !strings.Contains(name, "go_metin2_migratecli_test_") {
		t.Fatalf("expected a package-local test driver name, got %q", name)
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("drivers --require-empty-stock-release must not open a database target, got events %#v", events)
	}
}

func TestRunDriversRejectsRequireEmptyStockReleaseWithRequireDriver(t *testing.T) {
	name := registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"drivers", "--require-empty-stock-release", "--require-driver", name}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected combined stock-release and require-driver flags to exit %d, got %d stdout=%q stderr=%q", exitUsage, code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on combined flags, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "drivers usage:") || !strings.Contains(stderr.String(), "--require-empty-stock-release") {
		t.Fatalf("expected drivers usage for combined flags, got %q", stderr.String())
	}
}

func TestStockMetin2MigrateBinaryKeepsEmptyDriverList(t *testing.T) {
	migrateBin := mustBuildStockMetin2Migrate(t, t.TempDir())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(migrateBin, "drivers", "--require-empty-stock-release")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("stock metin2-migrate drivers --require-empty-stock-release: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr from stock empty-driver gate, got %q", stderr.String())
	}
	got := decodeSQLDriversEnvelope(t, stdout.Bytes())
	var drivers []string
	if err := json.Unmarshal(got.Drivers, &drivers); err != nil {
		t.Fatalf("decode stock drivers list: %v raw=%s", err, got.Drivers)
	}
	if drivers == nil || len(drivers) != 0 {
		t.Fatalf("stock release must keep drivers empty, got %s", stdout.String())
	}
	body := stdout.String()
	for _, forbidden := range []string{"sqlite", "sqlite3", "mysql", "postgres", "pgx", "CREATE TABLE", "password="} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("stock drivers envelope must not mention %q, got %s", forbidden, body)
		}
	}

	stdout.Reset()
	stderr.Reset()
	requireCmd := exec.Command(migrateBin, "drivers", "--require-driver", "sqlite")
	requireCmd.Stdout = &stdout
	requireCmd.Stderr = &stderr
	err := requireCmd.Run()
	if err == nil {
		t.Fatalf("stock metin2-migrate drivers --require-driver sqlite must fail closed, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout JSON when stock binary lacks sqlite, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "database driver is unavailable") || !strings.Contains(stderr.String(), "sqlite") {
		t.Fatalf("expected unavailable-driver stderr for stock sqlite require, got %q", stderr.String())
	}
}

func mustBuildStockMetin2Migrate(t *testing.T, binDir string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	out := filepath.Join(binDir, "metin2-migrate")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/metin2-migrate")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build ./cmd/metin2-migrate: %v stderr=%s", err, stderr.String())
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat built metin2-migrate: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable metin2-migrate at %s mode=%v", out, info.Mode())
	}
	return out
}

func TestRunDriversRejectsExtraArgsAndUnknownFlags(t *testing.T) {
	cases := [][]string{
		{"drivers", "extra"},
		{"drivers", "--nope"},
	}
	for _, args := range cases {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run(args, nil, &stdout, &stderr)
		if code != exitUsage {
			t.Fatalf("expected usage exit %d for %v, got %d stderr=%q", exitUsage, args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "drivers usage:") {
			t.Fatalf("expected drivers usage for %v, got %q", args, stderr.String())
		}
	}
}

func TestRunHelpListsDrivers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "  drivers ") {
		t.Fatalf("expected help to list drivers as its own command, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "drivers usage:") {
		t.Fatalf("expected drivers usage block, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--require-driver") {
		t.Fatalf("expected --require-driver in usage, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--require-empty-stock-release") {
		t.Fatalf("expected --require-empty-stock-release in usage, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "status                 ") || !strings.Contains(stdout.String(), "migration-run-retention") {
		t.Fatalf("expected usage to list drivers beside status / migration-run-retention, got %q", stdout.String())
	}
}

func TestRunRejectsUnknownCommandMentionsDrivers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"not-a-real-command"}, nil, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d", exitUsage, code)
	}
	if !strings.Contains(stderr.String(), "  drivers ") {
		t.Fatalf("expected usage to mention drivers as its own command, got %q", stderr.String())
	}
}

func decodeSQLDriversEnvelope(t *testing.T, raw []byte) sqlDriversGot {
	t.Helper()
	var got sqlDriversGot
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode drivers JSON: %v\nbody:\n%s", err, raw)
	}
	if got.Format != "go-metin2-sql-drivers-v1" {
		t.Fatalf("unexpected drivers format: %#v body=%s", got, raw)
	}
	if len(got.Drivers) == 0 || string(got.Drivers) == "null" {
		t.Fatalf("drivers must be a JSON array, got %s", raw)
	}
	return got
}

func sqlDriversListContains(t *testing.T, raw json.RawMessage, name string) bool {
	t.Helper()
	var drivers []string
	if err := json.Unmarshal(raw, &drivers); err != nil {
		t.Fatalf("decode drivers list: %v raw=%s", err, raw)
	}
	if drivers == nil {
		t.Fatalf("drivers array must not be null, raw=%s", raw)
	}
	for _, got := range drivers {
		if got == name {
			return true
		}
	}
	return false
}
