package migratecli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContribLabRetentionGCSamplesStayPrintOnly(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	base := filepath.Join(repoRoot, "contrib", "lab-retention-gc")

	helperPath := filepath.Join(base, "metin2-print-retention-gc.sh")
	servicePath := filepath.Join(base, "systemd", "metin2-artifact-retention-gc-print.service.sample")
	timerPath := filepath.Join(base, "systemd", "metin2-artifact-retention-gc-print.timer.sample")
	cronPath := filepath.Join(base, "cron.d", "metin2-artifact-retention-gc-print.sample")
	readmePath := filepath.Join(base, "README.md")

	helper := mustReadContribSample(t, helperPath)
	service := mustReadContribSample(t, servicePath)
	timer := mustReadContribSample(t, timerPath)
	cron := mustReadContribSample(t, cronPath)
	readme := mustReadContribSample(t, readmePath)

	for _, want := range []string{
		"artifact-retention-gc",
		"migration-run-retention",
		`>"$OUT/migration-run-retention.sh"`,
		"--build-info \"$OUT/build-info.json\"",
		"METIN2_RUNTIME_CONFIG",
		"backup-restore-drill",
		`>"$OUT/backup-restore-drill.sh"`,
		"/var/metin2/ops-prints",
		"/var/metin2/backups",
		"/var/metin2/migration-runs",
		`trap 'rm -f "$TMP_BUILD"'`,
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("helper missing %q", want)
		}
	}
	for _, forbiddenLive := range []string{
		"curl ",
		"http://127.0.0.1:6060",
		"http://127.0.0.1:6061",
		"/local/runtime-config",
	} {
		if strings.Contains(helper, forbiddenLive) {
			t.Fatalf("helper must not live-fetch ops JSON (%q):\n%s", forbiddenLive, helper)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "helper", helper, map[string]struct{}{
		`trap 'rm -f "$TMP_BUILD"' EXIT INT TERM`: {},
	})

	if !strings.Contains(service, "ExecStart=/usr/local/libexec/metin2-print-retention-gc.sh") {
		t.Fatalf("service must ExecStart only the print helper, got:\n%s", service)
	}
	for _, line := range strings.Split(service, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "Environment=") {
			t.Fatalf("service must not set Environment=: %q", trimmed)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "service", service, nil)
	if strings.Contains(service, "| /bin/sh") || strings.Contains(service, "|/bin/sh") {
		t.Fatalf("service must not pipe into a shell")
	}

	if !strings.Contains(timer, "Unit=metin2-artifact-retention-gc-print.service") {
		t.Fatalf("timer must point at the print service, got:\n%s", timer)
	}
	assertNoForbiddenRetentionGCMarkers(t, "timer", timer, nil)

	if !strings.Contains(cron, "/usr/local/libexec/metin2-print-retention-gc.sh") {
		t.Fatalf("cron sample must invoke the print helper, got:\n%s", cron)
	}
	for _, line := range strings.Split(cron, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "| /bin/sh") || strings.Contains(trimmed, "|/bin/sh") {
			t.Fatalf("cron sample must not pipe into a shell: %q", trimmed)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "cron", cron, map[string]struct{}{
		`# Do NOT append "| /bin/sh" to the helper output or invoke the printed *.sh files here.`: {},
	})

	for _, want := range []string{
		"disabled-by-default",
		"Never pipe printer stdout",
		"systemctl enable --now",
		"docs/workflow/lab-retention-gc-unit-samples.md",
		"migration-run-retention.sh",
		"METIN2_RUNTIME_CONFIG",
		"backup-restore-drill.sh",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("contrib README missing %q", want)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "readme", readme, map[string]struct{}{
		"Never `ExecStart` / cron-run `rm`, `rmdir`, `unlink`, `find -delete`, or": {},
		"- `rm` of `.gc-aside-*` trees":                                            {},
	})
}

func TestContribLabRetentionGCHelperHermeticExecution(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	helperPath := filepath.Join(repoRoot, "contrib", "lab-retention-gc", "metin2-print-retention-gc.sh")

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	printsRoot := filepath.Join(root, "ops-prints")
	mustMkdir(t, binDir)
	mustMkdir(t, printsRoot)

	stubPath := filepath.Join(binDir, "metin2-migrate")
	mustWriteFile(t, stubPath, []byte(`#!/bin/sh
set -eu
cmd="${1:-}"
case "$cmd" in
  version)
    printf '%s\n' '{"version":"test","commit":"abcdef0123456789","build_date":"2026-08-23T00:00:00Z"}'
    ;;
  artifact-retention-gc)
    printf '%s\n' "# stub artifact-retention-gc"
    printf '%s\n' "$*"
    ;;
  migration-run-retention)
    printf '%s\n' "# stub migration-run-retention"
    printf '%s\n' "$*"
    ;;
  backup-restore-drill)
    printf '%s\n' "# stub backup-restore-drill"
    printf '%s\n' "$*"
    ;;
  *)
    printf 'unexpected metin2-migrate command: %s\n' "$*" >&2
    exit 2
    ;;
esac
`))
	if err := os.Chmod(stubPath, 0o750); err != nil {
		t.Fatalf("chmod stub: %v", err)
	}

	t.Run("skips_drill_without_runtime_config", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "skip")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"HOME=" + root,
			"TMPDIR=" + root,
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("helper exit error: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
		outDir := strings.TrimSpace(stdout.String())
		if outDir == "" {
			t.Fatalf("expected helper to print OUT path, stderr=%q", stderr.String())
		}
		for _, name := range []string{
			"build-info.json",
			"artifact-retention-gc-backups.sh",
			"artifact-retention-gc-migration-runs.sh",
			"migration-run-retention.sh",
			"notes.md",
		} {
			path := filepath.Join(outDir, name)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected %s: %v", path, err)
			}
		}
		if _, err := os.Stat(filepath.Join(outDir, "backup-restore-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("backup-restore-drill.sh must be absent without METIN2_RUNTIME_CONFIG, err=%v", err)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "backup-restore-drill=skipped") {
			t.Fatalf("notes must record drill skip, got %q", notes)
		}
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		if !strings.Contains(migrationBody, "# stub migration-run-retention") {
			t.Fatalf("migration-run-retention.sh must come from stub printer, got %q", migrationBody)
		}
	})

	t.Run("prints_drill_from_retained_runtime_config", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "print")
		mustMkdir(t, runPrints)
		runtimeConfig := filepath.Join(root, "retained-runtime-config.json")
		mustWriteFile(t, runtimeConfig, []byte(`{"ok":true}`+"\n"))
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_RUNTIME_CONFIG=" + runtimeConfig,
			"HOME=" + root,
			"TMPDIR=" + root,
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("helper exit error: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
		outDir := strings.TrimSpace(stdout.String())
		drillPath := filepath.Join(outDir, "backup-restore-drill.sh")
		drillBody := mustReadContribSample(t, drillPath)
		if !strings.Contains(drillBody, "# stub backup-restore-drill") {
			t.Fatalf("backup-restore-drill.sh must come from stub printer, got %q", drillBody)
		}
		if !strings.Contains(drillBody, runtimeConfig) {
			t.Fatalf("stub drill argv must include runtime-config path %q, got %q", runtimeConfig, drillBody)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "backup-restore-drill=printed from METIN2_RUNTIME_CONFIG") {
			t.Fatalf("notes must record drill print, got %q", notes)
		}
	})

	t.Run("skips_symlink_runtime_config", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "symlink")
		mustMkdir(t, runPrints)
		target := filepath.Join(root, "runtime-config-target.json")
		mustWriteFile(t, target, []byte(`{"ok":true}`+"\n"))
		linkPath := filepath.Join(root, "runtime-config-link.json")
		if err := os.Symlink(target, linkPath); err != nil {
			t.Fatalf("symlink runtime-config: %v", err)
		}
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_RUNTIME_CONFIG=" + linkPath,
			"HOME=" + root,
			"TMPDIR=" + root,
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("helper exit error: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
		outDir := strings.TrimSpace(stdout.String())
		if _, err := os.Stat(filepath.Join(outDir, "backup-restore-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("symlink METIN2_RUNTIME_CONFIG must skip drill printer, err=%v", err)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "must not be a symlink") {
			t.Fatalf("notes must record symlink skip, got %q", notes)
		}
	})
}

func mustReadContribSample(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(raw) == 0 {
		t.Fatalf("%s is empty", path)
	}
	return string(raw)
}

func assertNoForbiddenRetentionGCMarkers(t *testing.T, label, body string, allowExact map[string]struct{}) {
	t.Helper()
	forbidden := []string{
		"| /bin/sh",
		"|/bin/sh",
		"| /bin/bash",
		"|/bin/bash",
		"| bash",
		"|bash",
		"find -delete",
		"unlink ",
		"rmdir ",
		".gc-aside-",
		"DROP TABLE",
		"CREATE TABLE",
		"METIN2_DB_DSN",
		"password=",
	}
	for _, marker := range forbidden {
		if !strings.Contains(body, marker) {
			continue
		}
		if allowExact != nil {
			allowed := false
			for exact := range allowExact {
				if strings.Contains(exact, marker) && strings.Contains(body, exact) {
					allowed = true
					break
				}
			}
			if allowed {
				continue
			}
		}
		// Helper may mention rm only inside the mktemp trap line.
		if marker == "unlink " || marker == "rmdir " || marker == "find -delete" {
			t.Fatalf("%s contains forbidden marker %q", label, marker)
		}
		if marker == "| /bin/sh" || marker == "|/bin/sh" || marker == "| /bin/bash" || marker == "|/bin/bash" || marker == "| bash" || marker == "|bash" {
			t.Fatalf("%s contains forbidden shell pipe %q", label, marker)
		}
		if marker == "DROP TABLE" || marker == "CREATE TABLE" || marker == "METIN2_DB_DSN" || marker == "password=" {
			t.Fatalf("%s contains forbidden secret/SQL marker %q", label, marker)
		}
		if marker == ".gc-aside-" {
			t.Fatalf("%s must not auto-run aside-rename of .gc-aside trees", label)
		}
	}

	// Bare `rm` of retention trees is forbidden outside the helper's mktemp trap
	// and documentation non-goal sentences that mention `rm` as something not to do.
	if label == "helper" {
		for _, line := range strings.Split(body, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.Contains(trimmed, "rm ") && !strings.HasPrefix(trimmed, "rm ") && !strings.Contains(trimmed, "rm -") {
				continue
			}
			if strings.Contains(trimmed, `trap 'rm -f "$TMP_BUILD"'`) {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			t.Fatalf("helper line must not rm retention trees: %q", trimmed)
		}
	}
}
