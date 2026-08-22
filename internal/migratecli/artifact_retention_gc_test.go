package migratecli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunArtifactRetentionGCPrintsAsideTriageScript(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-retention-gc",
			"--retention-base", "/var/metin2/backups",
			"--keep-days", "14",
			"--now", "2026-08-22T12:00:00Z",
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr, got %q", stderr.String())
	}

	body := stdout.String()
	for _, want := range []string{
		`# read-only printer: does not delete retention trees or open a database`,
		`RETENTION_BASE='/var/metin2/backups'`,
		`KEEP_DAYS='14'`,
		`NOW_UTC='20260822T120000Z'`,
		`ASIDE_SUFFIX='gc-aside-20260822T120000Z'`,
		`YYYYMMDDTHHMMSSZ-`,
		`docs/workflow/lab-deployment-topology.md`,
		`"$RETENTION_BASE/${name}.${ASIDE_SUFFIX}"`,
		`if [ -e "$aside" ]; then`,
		`# skip trees younger than KEEP_DAYS relative to NOW_UTC`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	for _, banned := range []string{
		" rm ",
		"\nrm ",
		"rmdir",
		"unlink",
		"find -delete",
		"CREATE TABLE",
		"DROP TABLE",
		"password=",
		"memory://",
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("artifact-retention-gc must not contain %q, got:\n%s", banned, body)
		}
	}
}

func TestRunArtifactRetentionGCHonorsMigrationRunsBaseAndCompactNow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-retention-gc",
			"--retention-base", "/var/metin2/migration-runs",
			"--keep-days", "7",
			"--now", "20260822T153045Z",
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	for _, want := range []string{
		`RETENTION_BASE='/var/metin2/migration-runs'`,
		`KEEP_DAYS='7'`,
		`NOW_UTC='20260822T153045Z'`,
		`ASIDE_SUFFIX='gc-aside-20260822T153045Z'`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
}

func TestRunArtifactRetentionGCRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "relative-base", args: []string{"artifact-retention-gc", "--retention-base", "var/metin2/backups", "--keep-days", "14", "--now", "2026-08-22T12:00:00Z"}},
		{name: "keep-days-zero", args: []string{"artifact-retention-gc", "--retention-base", "/var/metin2/backups", "--keep-days", "0", "--now", "2026-08-22T12:00:00Z"}},
		{name: "keep-days-negative", args: []string{"artifact-retention-gc", "--retention-base", "/var/metin2/backups", "--keep-days", "-1", "--now", "2026-08-22T12:00:00Z"}},
		{name: "keep-days-non-integer", args: []string{"artifact-retention-gc", "--retention-base", "/var/metin2/backups", "--keep-days", "fourteen", "--now", "2026-08-22T12:00:00Z"}},
		{name: "invalid-now", args: []string{"artifact-retention-gc", "--retention-base", "/var/metin2/backups", "--keep-days", "14", "--now", "not-a-timestamp"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "artifact-retention-gc") {
				t.Fatalf("expected stderr to mention artifact-retention-gc, got %q", stderr.String())
			}
		})
	}
}

func TestRunArtifactRetentionGCUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing-flag", args: []string{"artifact-retention-gc"}},
		{name: "unexpected-arg", args: []string{"artifact-retention-gc", "--retention-base", "/var/metin2/backups", "extra"}},
		{name: "unknown-flag", args: []string{"artifact-retention-gc", "--nope", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "artifact-retention-gc usage:") {
				t.Fatalf("expected usage text, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), "--retention-base") || !strings.Contains(stderr.String(), "--keep-days") {
				t.Fatalf("expected usage to mention --retention-base and --keep-days, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsArtifactRetentionGC(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "artifact-retention-gc") {
		t.Fatalf("expected usage to list artifact-retention-gc, got %q", stderr.String())
	}
}
