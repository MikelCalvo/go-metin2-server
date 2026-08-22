package migratecli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

func TestArtifactRetentionGCPrintedScriptAsidesAgedTreesAndPreservesYoungOnes(t *testing.T) {
	retentionBase := t.TempDir()
	agedName := "20260801T120000Z-abcdef012345"
	youngName := "20260820T120000Z-fedcba543210"
	nonMatchingName := "notes-not-a-retention-tree"
	alreadyAsideName := "20260701T120000Z-deadbeefcafe.gc-aside-20260810T000000Z"

	mustMkdir(t, filepath.Join(retentionBase, agedName))
	mustMkdir(t, filepath.Join(retentionBase, youngName))
	mustMkdir(t, filepath.Join(retentionBase, nonMatchingName))
	mustMkdir(t, filepath.Join(retentionBase, alreadyAsideName))
	mustWriteFile(t, filepath.Join(retentionBase, "README.txt"), []byte("leave me alone\n"))

	script := mustPrintArtifactRetentionGCScript(t, retentionBase, "14", "2026-08-22T12:00:00Z")
	stdout, stderr, code := runPrintedShellScript(t, script)
	if code != 0 {
		t.Fatalf("expected printed script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "aside-renamed "+agedName+" -> "+agedName+".gc-aside-20260822T120000Z") {
		t.Fatalf("expected aged aside rename in stdout, got %q", stdout)
	}
	if strings.Contains(stdout, youngName) {
		t.Fatalf("young tree must not be aside-renamed, got stdout %q", stdout)
	}

	assertDirExists(t, filepath.Join(retentionBase, agedName+".gc-aside-20260822T120000Z"))
	assertDirMissing(t, filepath.Join(retentionBase, agedName))
	assertDirExists(t, filepath.Join(retentionBase, youngName))
	assertDirExists(t, filepath.Join(retentionBase, nonMatchingName))
	assertDirExists(t, filepath.Join(retentionBase, alreadyAsideName))
	if _, err := os.Stat(filepath.Join(retentionBase, "README.txt")); err != nil {
		t.Fatalf("non-directory child must remain: %v", err)
	}
}

func TestArtifactRetentionGCPrintedScriptFailsClosedOnAsideDestinationCollision(t *testing.T) {
	retentionBase := t.TempDir()
	agedName := "20260801T120000Z-abcdef012345"
	asideName := agedName + ".gc-aside-20260822T120000Z"

	mustMkdir(t, filepath.Join(retentionBase, agedName))
	mustMkdir(t, filepath.Join(retentionBase, asideName))

	script := mustPrintArtifactRetentionGCScript(t, retentionBase, "14", "2026-08-22T12:00:00Z")
	stdout, stderr, code := runPrintedShellScript(t, script)
	if code == 0 {
		t.Fatalf("expected non-zero exit on aside destination collision, stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "artifact-retention-gc") || !strings.Contains(stderr, asideName) {
		t.Fatalf("expected stderr to mention artifact-retention-gc and aside path, got %q", stderr)
	}
	assertDirExists(t, filepath.Join(retentionBase, agedName))
	assertDirExists(t, filepath.Join(retentionBase, asideName))
}

func mustPrintArtifactRetentionGCScript(t *testing.T, retentionBase, keepDays, now string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"artifact-retention-gc",
			"--retention-base", retentionBase,
			"--keep-days", keepDays,
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

func runPrintedShellScript(t *testing.T, script string) (stdout string, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError from printed script, got %v", err)
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode()
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory %s to exist: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", path)
	}
}

func assertDirMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected directory %s to be missing, got err=%v", path, err)
	}
}
