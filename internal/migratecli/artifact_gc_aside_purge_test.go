package migratecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunArtifactGCAsidePurgePrintsConfirmationGatedPurgeScript(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-gc-aside-purge",
			"--retention-base", "/var/metin2/backups",
			"--i-confirm-lab-gc-aside-purge",
			"--min-aside-age-days", "7",
			"--now", "2026-08-25T12:00:00Z",
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
		`# confirmation-gated printer: does not execute purge or open a database`,
		`RETENTION_BASE='/var/metin2/backups'`,
		`MIN_ASIDE_AGE_DAYS='7'`,
		`NOW_UTC='20260825T120000Z'`,
		`.gc-aside-`,
		`docs/workflow/lab-deployment-topology.md`,
		`rm -rf -- "$path"`,
		`# skip aside trees younger than MIN_ASIDE_AGE_DAYS relative to NOW_UTC`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	for _, banned := range []string{
		"CREATE TABLE",
		"DROP TABLE",
		"password=",
		"memory://",
	} {
		if strings.Contains(body, banned) {
			t.Fatalf("artifact-gc-aside-purge must not contain %q, got:\n%s", banned, body)
		}
	}
}

func TestRunArtifactGCAsidePurgeHonorsExportsBaseAndCompactNow(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-gc-aside-purge",
			"--retention-base", "/var/metin2/exports",
			"--i-confirm-lab-gc-aside-purge",
			"--min-aside-age-days", "3",
			"--now", "20260825T153045Z",
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
		`RETENTION_BASE='/var/metin2/exports'`,
		`MIN_ASIDE_AGE_DAYS='3'`,
		`NOW_UTC='20260825T153045Z'`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
}

func TestRunArtifactGCAsidePurgeRequiresConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-gc-aside-purge",
			"--retention-base", "/var/metin2/backups",
			"--min-aside-age-days", "7",
			"--now", "2026-08-25T12:00:00Z",
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout without confirmation, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--i-confirm-lab-gc-aside-purge") {
		t.Fatalf("expected confirmation flag in stderr, got %q", stderr.String())
	}
}

func TestRunArtifactGCAsidePurgeRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "relative-base", args: []string{"artifact-gc-aside-purge", "--retention-base", "var/metin2/backups", "--i-confirm-lab-gc-aside-purge", "--min-aside-age-days", "7", "--now", "2026-08-25T12:00:00Z"}},
		{name: "age-zero", args: []string{"artifact-gc-aside-purge", "--retention-base", "/var/metin2/backups", "--i-confirm-lab-gc-aside-purge", "--min-aside-age-days", "0", "--now", "2026-08-25T12:00:00Z"}},
		{name: "age-negative", args: []string{"artifact-gc-aside-purge", "--retention-base", "/var/metin2/backups", "--i-confirm-lab-gc-aside-purge", "--min-aside-age-days", "-1", "--now", "2026-08-25T12:00:00Z"}},
		{name: "age-non-integer", args: []string{"artifact-gc-aside-purge", "--retention-base", "/var/metin2/backups", "--i-confirm-lab-gc-aside-purge", "--min-aside-age-days", "seven", "--now", "2026-08-25T12:00:00Z"}},
		{name: "invalid-now", args: []string{"artifact-gc-aside-purge", "--retention-base", "/var/metin2/backups", "--i-confirm-lab-gc-aside-purge", "--min-aside-age-days", "7", "--now", "not-a-timestamp"}},
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
			if !strings.Contains(stderr.String(), "artifact-gc-aside-purge") {
				t.Fatalf("expected stderr to mention artifact-gc-aside-purge, got %q", stderr.String())
			}
		})
	}
}

func TestRunArtifactGCAsidePurgeUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing-flag", args: []string{"artifact-gc-aside-purge"}},
		{name: "unexpected-arg", args: []string{"artifact-gc-aside-purge", "--retention-base", "/var/metin2/backups", "--i-confirm-lab-gc-aside-purge", "extra"}},
		{name: "unknown-flag", args: []string{"artifact-gc-aside-purge", "--nope", "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "artifact-gc-aside-purge usage:") {
				t.Fatalf("expected usage text, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), "--retention-base") || !strings.Contains(stderr.String(), "--i-confirm-lab-gc-aside-purge") {
				t.Fatalf("expected usage to mention required flags, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsArtifactGCAsidePurge(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "artifact-gc-aside-purge") {
		t.Fatalf("expected usage to list artifact-gc-aside-purge, got %q", stderr.String())
	}
}

func TestArtifactGCAsidePurgePrintedScriptRemovesAgedAsidesAndPreservesLiveTrees(t *testing.T) {
	retentionBase := t.TempDir()
	liveName := "20260820T120000Z-fedcba543210"
	agedAsideName := "20260801T120000Z-abcdef012345.gc-aside-20260810T120000Z"
	youngAsideName := "20260818T120000Z-deadbeefcafe.gc-aside-20260822T120000Z"
	nonMatchingName := "notes-not-a-retention-tree"

	mustMkdir(t, filepath.Join(retentionBase, liveName))
	mustMkdir(t, filepath.Join(retentionBase, agedAsideName))
	mustMkdir(t, filepath.Join(retentionBase, youngAsideName))
	mustMkdir(t, filepath.Join(retentionBase, nonMatchingName))
	mustWriteFile(t, filepath.Join(retentionBase, "README.txt"), []byte("leave me alone\n"))
	mustWriteFile(t, filepath.Join(retentionBase, agedAsideName, "marker.txt"), []byte("purge me\n"))

	script := mustPrintArtifactGCAsidePurgeScript(t, retentionBase, "7", "2026-08-25T12:00:00Z")
	stdout, stderr, code := runPrintedShellScript(t, script)
	if code != 0 {
		t.Fatalf("expected printed script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "purged "+agedAsideName) {
		t.Fatalf("expected aged purge in stdout, got %q", stdout)
	}
	if strings.Contains(stdout, youngAsideName) || strings.Contains(stdout, liveName) {
		t.Fatalf("young aside / live tree must not be purged, got stdout %q", stdout)
	}

	assertDirMissing(t, filepath.Join(retentionBase, agedAsideName))
	assertDirExists(t, filepath.Join(retentionBase, youngAsideName))
	assertDirExists(t, filepath.Join(retentionBase, liveName))
	assertDirExists(t, filepath.Join(retentionBase, nonMatchingName))
	if _, err := os.Stat(filepath.Join(retentionBase, "README.txt")); err != nil {
		t.Fatalf("non-directory child must remain: %v", err)
	}
}

func mustPrintArtifactGCAsidePurgeScript(t *testing.T, retentionBase, minAsideAgeDays, now string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-gc-aside-purge",
			"--retention-base", retentionBase,
			"--i-confirm-lab-gc-aside-purge",
			"--min-aside-age-days", minAsideAgeDays,
			"--now", now,
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected printer exit 0, got %d stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no printer stderr, got %q", stderr.String())
	}
	body := stdout.String()
	if body == "" {
		t.Fatal("expected non-empty printed script")
	}
	return body
}
