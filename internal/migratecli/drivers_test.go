package migratecli

import (
	"bytes"
	"encoding/json"
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
