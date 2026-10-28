package migratecli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// These are invented test-only credentials, never operator DSNs.
func TestWriteMigrationCommandErrorScrubsDriverCredentialFragments(t *testing.T) {
	const dsn = "postgres://sample:whole-value@db.invalid/app"
	for _, tc := range []struct {
		name, driverError string
		forbidden         []string
		preserved         string
	}{
		{"exact DSN", "dial " + dsn + " refused", []string{dsn, "whole-value"}, "dial"},
		{"kv unquoted", "dial failed: password=fragment-value host=db.invalid", []string{"password=fragment-value", "fragment-value"}, "host=db.invalid"},
		{"kv mixed case", "dial failed: PASSWORD=fragment-value host=db.invalid", []string{"fragment-value"}, "host=db.invalid"},
		{"kv quoted", "dial failed: password='two words' host=db.invalid", []string{"two words", "password='two words'"}, "host=db.invalid"},
		{"kv URL query", "dial failed: password=fragment-value&sslmode=disable", []string{"fragment-value"}, "sslmode=disable"},
		{"userinfo URL", "dial postgres://sample:fragment-value@db.invalid/app refused", []string{"fragment-value", "sample:fragment-value@"}, "db.invalid/app"},
		{"userinfo bare", "dial sample:fragment-value@db.invalid refused", []string{"fragment-value", "sample:fragment-value@"}, "db.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			writeMigrationCommandError(&stderr, dsn, "migration status: %s", tc.driverError)
			got := stderr.String()
			if !strings.Contains(got, "<redacted-dsn>") || !strings.Contains(got, tc.preserved) {
				t.Fatalf("expected redacted marker and non-secret context, got %q", got)
			}
			for _, value := range tc.forbidden {
				if strings.Contains(got, value) {
					t.Fatalf("credential fragment leaked on stderr: %q", got)
				}
			}
		})
	}
}

func TestRunStatusScrubsDriverCredentialFragments(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	const dsn = "postgres://sample:whole-value@db.invalid/app"
	currentMigrateCLITestDriver(t).setError(fmt.Errorf("dial failed: password=fragment-value via sample:other-value@db.invalid"))
	var stdout, stderr bytes.Buffer
	code := Run([]string{"status", "--driver", driverName, "--dsn", dsn}, nil, &stdout, &stderr)
	if code != exitError || stdout.Len() != 0 {
		t.Fatalf("expected failed status and no stdout, code=%d stdout=%q", code, stdout.String())
	}
	got := stderr.String()
	for _, secret := range []string{"fragment-value", "other-value", "whole-value"} {
		if strings.Contains(got, secret) {
			t.Fatalf("driver credential leaked on stderr: %q", got)
		}
	}
	if !strings.Contains(got, "<redacted-dsn>") || !strings.Contains(got, "db.invalid") {
		t.Fatalf("expected scrubbed driver error with context, got %q", got)
	}
}

func TestWriteMigrationCommandErrorEmptyDSNPath(t *testing.T) {
	for _, dsn := range []string{"", "  \t"} {
		var stderr bytes.Buffer
		writeMigrationCommandError(&stderr, dsn, "--driver and --dsn are required")
		if got, want := stderr.String(), "--driver and --dsn are required\n"; got != want {
			t.Fatalf("empty DSN error path changed: got %q, want %q", got, want)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"status", "--driver", "unlinked", "--dsn", ""}, nil, &stdout, &stderr); code != exitUsage || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--driver and --dsn are required") {
		t.Fatalf("missing DSN must follow usage path, code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
