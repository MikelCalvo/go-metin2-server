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
	periodicWeeklyPath := filepath.Join(base, "periodic", "weekly", "900.metin2-artifact-retention-gc-print.sample")
	periodicConfPath := filepath.Join(base, "periodic", "periodic.conf.sample")
	readmePath := filepath.Join(base, "README.md")

	helper := mustReadContribSample(t, helperPath)
	service := mustReadContribSample(t, servicePath)
	timer := mustReadContribSample(t, timerPath)
	cron := mustReadContribSample(t, cronPath)
	periodicWeekly := mustReadContribSample(t, periodicWeeklyPath)
	periodicConf := mustReadContribSample(t, periodicConfPath)
	readme := mustReadContribSample(t, readmePath)

	for _, want := range []string{
		"artifact-retention-gc",
		"migration-run-retention",
		`>"$OUT/migration-run-retention.sh"`,
		"export-quarantine-drill",
		`>"$OUT/export-quarantine-drill.sh"`,
		"--build-info \"$OUT/build-info.json\"",
		"METIN2_RUNTIME_CONFIG",
		"METIN2_GAMED_LOG_PATH",
		"METIN2_AUTHD_LOG_PATH",
		"METIN2_IMPORT_EXPORT_TREE",
		"METIN2_IMPORT_DRIVER",
		"METIN2_IMPORT_DSN_ENV",
		"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE",
		"METIN2_GC_ASIDE_MIN_AGE_DAYS",
		"METIN2_GC_ASIDE_NOW",
		"METIN2_MIGRATION_TARGET_VERSION",
		"METIN2_MIGRATION_ALLOW_ROLLBACK",
		"--target-version",
		"--allow-rollback",
		"--gamed-log-path \"$GAMED_LOG\"",
		"--authd-log-path \"$AUTHD_LOG\"",
		"/var/log/metin2/gamed.log",
		"/var/log/metin2/authd.log",
		"backup-restore-drill",
		`>"$OUT/backup-restore-drill.sh"`,
		"import-export-drill",
		`>"$OUT/import-export-drill.sh"`,
		"--i-confirm-print-sql-import-drill",
		"--export-tree \"$IMPORT_EXPORT_TREE\"",
		"--driver \"$IMPORT_DRIVER\"",
		"--dsn-env \"$IMPORT_DSN_ENV\"",
		"artifact-gc-aside-purge",
		"--i-confirm-lab-gc-aside-purge",
		`>"$OUT/artifact-gc-aside-purge-backups.sh"`,
		`>"$OUT/artifact-gc-aside-purge-migration-runs.sh"`,
		`>"$OUT/artifact-gc-aside-purge-exports.sh"`,
		`>"$OUT/artifact-retention-gc-exports.sh"`,
		"/var/metin2/ops-prints",
		"/var/metin2/backups",
		"/var/metin2/migration-runs",
		"/var/metin2/exports",
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
		"weekly_metin2_artifact_retention_gc_print_enable",
		"/usr/local/libexec/metin2-print-retention-gc.sh",
		"source_periodic_confs",
		"[Yy][Ee][Ss]",
	} {
		if !strings.Contains(periodicWeekly, want) {
			t.Fatalf("periodic weekly sample missing %q", want)
		}
	}
	for _, line := range strings.Split(periodicWeekly, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "| /bin/sh") || strings.Contains(trimmed, "|/bin/sh") {
			t.Fatalf("periodic weekly sample must not pipe into a shell: %q", trimmed)
		}
		if strings.Contains(trimmed, "curl ") {
			t.Fatalf("periodic weekly sample must not live-fetch ops JSON: %q", trimmed)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "periodic-weekly", periodicWeekly, map[string]struct{}{
		`# Explicit non-goals: no "| /bin/sh", no curl of ops JSON, no rm`: {},
		`# of retention / .gc-aside trees, no DSN / SQL embedding.`:        {},
		`# Never pipe helper output into a shell and`:                      {},
		`# never execute the printed *.sh files from this script.`:         {},
	})

	if !strings.Contains(periodicConf, `weekly_metin2_artifact_retention_gc_print_enable="NO"`) {
		t.Fatalf("periodic.conf sample must default enable to NO, got:\n%s", periodicConf)
	}
	if strings.Contains(periodicConf, `weekly_metin2_artifact_retention_gc_print_enable="YES"`) {
		t.Fatalf("periodic.conf sample must not default enable to YES")
	}
	for _, forbiddenLive := range []string{
		"curl ",
		"| /bin/sh",
		"|/bin/sh",
		"password=",
		"METIN2_DB_DSN",
		"CREATE TABLE",
		"DROP TABLE",
	} {
		if strings.Contains(periodicConf, forbiddenLive) {
			t.Fatalf("periodic.conf sample must not contain %q:\n%s", forbiddenLive, periodicConf)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "periodic-conf", periodicConf, nil)

	for _, want := range []string{
		"disabled-by-default",
		"Never pipe printer stdout",
		"systemctl enable --now",
		"docs/workflow/lab-retention-gc-unit-samples.md",
		"migration-run-retention.sh",
		"export-quarantine-drill.sh",
		"artifact-retention-gc-exports.sh",
		"/var/metin2/exports",
		"METIN2_RUNTIME_CONFIG",
		"METIN2_GAMED_LOG_PATH",
		"METIN2_AUTHD_LOG_PATH",
		"METIN2_IMPORT_EXPORT_TREE",
		"METIN2_IMPORT_DRIVER",
		"METIN2_IMPORT_DSN_ENV",
		"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE",
		"METIN2_GC_ASIDE_MIN_AGE_DAYS",
		"METIN2_GC_ASIDE_NOW",
		"METIN2_MIGRATION_TARGET_VERSION",
		"METIN2_MIGRATION_ALLOW_ROLLBACK",
		"/var/log/metin2/gamed.log",
		"/var/log/metin2/authd.log",
		"backup-restore-drill.sh",
		"import-export-drill.sh",
		"artifact-gc-aside-purge-backups.sh",
		"artifact-gc-aside-purge-migration-runs.sh",
		"artifact-gc-aside-purge-exports.sh",
		"metin2-runtime-config.env.sample",
		"EnvironmentFile=",
		"periodic/weekly/900.metin2-artifact-retention-gc-print.sample",
		"periodic/periodic.conf.sample",
		"weekly_metin2_artifact_retention_gc_print_enable",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("contrib README missing %q", want)
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "readme", readme, map[string]struct{}{
		"Never `ExecStart` / cron-run / periodic-run `rm`, `rmdir`, `unlink`,":           {},
		"`find -delete`, or aside-rename of retention trees from these samples.":         {},
		"- automatic / scheduled `rm` of aged aside-renamed retention trees (print-only": {},
	})

	envSamplePath := filepath.Join(base, "env", "metin2-runtime-config.env.sample")
	dropInPath := filepath.Join(base, "systemd", "metin2-artifact-retention-gc-print.service.d", "runtime-config.conf.sample")
	envSample := mustReadContribSample(t, envSamplePath)
	dropIn := mustReadContribSample(t, dropInPath)
	for _, want := range []string{
		"METIN2_RUNTIME_CONFIG=",
		"/var/metin2/ops-prints/",
		"METIN2_GAMED_LOG_PATH=",
		"METIN2_AUTHD_LOG_PATH=",
		"/var/log/metin2/gamed.log",
		"/var/log/metin2/authd.log",
		"METIN2_IMPORT_EXPORT_TREE=",
		"METIN2_IMPORT_DRIVER=",
		"METIN2_IMPORT_DSN_ENV=",
		"METIN2_IMPORT_DSN",
		"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=",
		"METIN2_GC_ASIDE_MIN_AGE_DAYS=",
		"METIN2_GC_ASIDE_NOW=",
		"METIN2_MIGRATION_TARGET_VERSION=",
		"METIN2_MIGRATION_ALLOW_ROLLBACK=",
	} {
		if !strings.Contains(envSample, want) {
			t.Fatalf("env sample missing %q", want)
		}
	}
	for _, want := range []string{
		"METIN2_GAMED_LOG_PATH=",
		"METIN2_AUTHD_LOG_PATH=",
		"/var/log/metin2/gamed.log",
		"/var/log/metin2/authd.log",
		"METIN2_IMPORT_EXPORT_TREE=",
		"METIN2_IMPORT_DRIVER=",
		"METIN2_IMPORT_DSN_ENV=",
		"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=",
		"METIN2_GC_ASIDE_MIN_AGE_DAYS=",
		"METIN2_GC_ASIDE_NOW=",
		"METIN2_MIGRATION_TARGET_VERSION=",
		"METIN2_MIGRATION_ALLOW_ROLLBACK=",
	} {
		if !strings.Contains(periodicConf, want) {
			t.Fatalf("periodic.conf sample missing %q", want)
		}
	}
	if !strings.Contains(dropIn, "EnvironmentFile=") {
		t.Fatalf("drop-in sample must set EnvironmentFile=, got:\n%s", dropIn)
	}
	if !strings.Contains(dropIn, "metin2-runtime-config.env") {
		t.Fatalf("drop-in sample must point at the runtime-config env file, got:\n%s", dropIn)
	}
	for _, body := range []string{envSample, dropIn} {
		for _, forbiddenLive := range []string{
			"curl ",
			"http://127.0.0.1:6060",
			"http://127.0.0.1:6061",
			"| /bin/sh",
			"|/bin/sh",
			"password=",
			"METIN2_DB_DSN",
			"CREATE TABLE",
			"DROP TABLE",
		} {
			if strings.Contains(body, forbiddenLive) {
				t.Fatalf("runtime-config sample must not contain %q:\n%s", forbiddenLive, body)
			}
		}
	}
	assertNoForbiddenRetentionGCMarkers(t, "env-sample", envSample, nil)
	assertNoForbiddenRetentionGCMarkers(t, "drop-in-sample", dropIn, nil)
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
  export-quarantine-drill)
    printf '%s\n' "# stub export-quarantine-drill"
    printf '%s\n' "$*"
    ;;
  backup-restore-drill)
    printf '%s\n' "# stub backup-restore-drill"
    printf '%s\n' "$*"
    ;;
  import-export-drill)
    printf '%s\n' "# stub import-export-drill"
    printf '%s\n' "$*"
    ;;
  artifact-gc-aside-purge)
    printf '%s\n' "# stub artifact-gc-aside-purge"
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
			"artifact-retention-gc-exports.sh",
			"migration-run-retention.sh",
			"export-quarantine-drill.sh",
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
		if _, err := os.Stat(filepath.Join(outDir, "import-export-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("import-export-drill.sh must be absent without METIN2_IMPORT_EXPORT_TREE, err=%v", err)
		}
		for _, name := range []string{
			"artifact-gc-aside-purge-backups.sh",
			"artifact-gc-aside-purge-migration-runs.sh",
			"artifact-gc-aside-purge-exports.sh",
		} {
			if _, err := os.Stat(filepath.Join(outDir, name)); !os.IsNotExist(err) {
				t.Fatalf("%s must be absent without METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=YES, err=%v", name, err)
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "backup-restore-drill=skipped") {
			t.Fatalf("notes must record drill skip, got %q", notes)
		}
		if !strings.Contains(notes, "export-quarantine-drill=printed from build-info") {
			t.Fatalf("notes must record export-quarantine-drill print, got %q", notes)
		}
		if !strings.Contains(notes, "import-export-drill=skipped") {
			t.Fatalf("notes must record import-export-drill skip, got %q", notes)
		}
		if !strings.Contains(notes, "artifact-gc-aside-purge=skipped") {
			t.Fatalf("notes must record artifact-gc-aside-purge skip, got %q", notes)
		}
		if !strings.Contains(notes, "migration-run-retention=printed with CLI default target (latest)") {
			t.Fatalf("notes must record default migration-run-retention posture, got %q", notes)
		}
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		if !strings.Contains(migrationBody, "# stub migration-run-retention") {
			t.Fatalf("migration-run-retention.sh must come from stub printer, got %q", migrationBody)
		}
		if strings.Contains(migrationBody, "--target-version") {
			t.Fatalf("default migration-run-retention argv must omit --target-version, got %q", migrationBody)
		}
		if strings.Contains(migrationBody, "--allow-rollback") {
			t.Fatalf("default migration-run-retention argv must omit --allow-rollback, got %q", migrationBody)
		}
		exportBody := mustReadContribSample(t, filepath.Join(outDir, "export-quarantine-drill.sh"))
		if !strings.Contains(exportBody, "# stub export-quarantine-drill") {
			t.Fatalf("export-quarantine-drill.sh must come from stub printer, got %q", exportBody)
		}
		exportsGCBody := mustReadContribSample(t, filepath.Join(outDir, "artifact-retention-gc-exports.sh"))
		if !strings.Contains(exportsGCBody, "/var/metin2/exports") {
			t.Fatalf("artifact-retention-gc-exports.sh must target /var/metin2/exports, got %q", exportsGCBody)
		}
		for _, body := range []string{migrationBody, exportBody} {
			for _, want := range []string{
				"--gamed-log-path",
				"/var/log/metin2/gamed.log",
				"--authd-log-path",
				"/var/log/metin2/authd.log",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("printer argv must include default log path marker %q, got %q", want, body)
				}
			}
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
		for _, want := range []string{
			"--gamed-log-path",
			"/var/log/metin2/gamed.log",
			"--authd-log-path",
			"/var/log/metin2/authd.log",
		} {
			if !strings.Contains(drillBody, want) {
				t.Fatalf("backup-restore-drill argv must include default log path marker %q, got %q", want, drillBody)
			}
		}
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		exportBody := mustReadContribSample(t, filepath.Join(outDir, "export-quarantine-drill.sh"))
		for _, body := range []string{migrationBody, exportBody} {
			for _, want := range []string{
				"--gamed-log-path",
				"/var/log/metin2/gamed.log",
				"--authd-log-path",
				"/var/log/metin2/authd.log",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("printer argv must include default log path marker %q, got %q", want, body)
				}
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "backup-restore-drill=printed from METIN2_RUNTIME_CONFIG") {
			t.Fatalf("notes must record drill print, got %q", notes)
		}
		if !strings.Contains(notes, "export-quarantine-drill=printed from build-info") {
			t.Fatalf("notes must record export-quarantine-drill print, got %q", notes)
		}
		if !strings.Contains(notes, "import-export-drill=skipped") {
			t.Fatalf("notes must still record import-export-drill skip without import env, got %q", notes)
		}
		if _, err := os.Stat(filepath.Join(outDir, "import-export-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("import-export-drill.sh must stay absent without import env, err=%v", err)
		}
	})

	t.Run("prints_import_export_drill_from_retained_tree", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-print")
		mustMkdir(t, runPrints)
		exportTree := filepath.Join(root, "exports", "20260827T120000Z-abcdef012345")
		mustMkdir(t, exportTree)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=" + exportTree,
			"METIN2_IMPORT_DRIVER=sqlite3",
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
		importPath := filepath.Join(outDir, "import-export-drill.sh")
		importBody := mustReadContribSample(t, importPath)
		if !strings.Contains(importBody, "# stub import-export-drill") {
			t.Fatalf("import-export-drill.sh must come from stub printer, got %q", importBody)
		}
		for _, want := range []string{
			"--export-tree",
			exportTree,
			"--driver",
			"sqlite3",
			"--dsn-env",
			"METIN2_IMPORT_DSN",
			"--i-confirm-print-sql-import-drill",
		} {
			if !strings.Contains(importBody, want) {
				t.Fatalf("import-export-drill argv must include %q, got %q", want, importBody)
			}
		}
		for _, forbidden := range []string{
			"postgres://",
			"memory://",
			"CREATE TABLE",
			"DROP TABLE",
			"password=",
			"--i-confirm-print-scoped-replace",
		} {
			if strings.Contains(importBody, forbidden) {
				t.Fatalf("import-export-drill print must not embed %q, got %q", forbidden, importBody)
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "import-export-drill=printed from METIN2_IMPORT_EXPORT_TREE") {
			t.Fatalf("notes must record import-export-drill print, got %q", notes)
		}
		if strings.Contains(notes, "scoped-replace opt-in") {
			t.Fatalf("default import print must stay insert-only, got %q", notes)
		}
	})

	t.Run("prints_import_export_drill_scoped_replace_when_env_set", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-scoped-replace")
		mustMkdir(t, runPrints)
		exportTree := filepath.Join(root, "exports", "20260903T120000Z-abcdef012345")
		mustMkdir(t, exportTree)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=" + exportTree,
			"METIN2_IMPORT_DRIVER=sqlite3",
			"METIN2_IMPORT_PRINT_SCOPED_REPLACE=YES",
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
		importPath := filepath.Join(outDir, "import-export-drill.sh")
		importBody := mustReadContribSample(t, importPath)
		for _, want := range []string{
			"--export-tree",
			exportTree,
			"--driver",
			"sqlite3",
			"--dsn-env",
			"METIN2_IMPORT_DSN",
			"--i-confirm-print-sql-import-drill",
			"--i-confirm-print-scoped-replace",
		} {
			if !strings.Contains(importBody, want) {
				t.Fatalf("scoped-replace import-export-drill argv must include %q, got %q", want, importBody)
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "import-export-drill=printed from METIN2_IMPORT_EXPORT_TREE (scoped-replace opt-in)") {
			t.Fatalf("notes must record scoped-replace opt-in print, got %q", notes)
		}
	})

	t.Run("prints_import_export_drill_two_phase_when_env_set", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-two-phase")
		mustMkdir(t, runPrints)
		exportTree := filepath.Join(root, "exports", "20260903T181500Z-abcdef012345")
		mustMkdir(t, exportTree)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=" + exportTree,
			"METIN2_IMPORT_DRIVER=sqlite3",
			"METIN2_IMPORT_PRINT_TWO_PHASE_WIPE_ROSTER=YES",
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
		importPath := filepath.Join(outDir, "import-export-drill.sh")
		importBody := mustReadContribSample(t, importPath)
		for _, want := range []string{
			"--export-tree",
			exportTree,
			"--driver",
			"sqlite3",
			"--dsn-env",
			"METIN2_IMPORT_DSN",
			"--i-confirm-print-sql-import-drill",
			"--i-confirm-print-two-phase-wipe-roster-reimport",
		} {
			if !strings.Contains(importBody, want) {
				t.Fatalf("two-phase import-export-drill argv must include %q, got %q", want, importBody)
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "import-export-drill=printed from METIN2_IMPORT_EXPORT_TREE (two-phase wipe→roster→reimport opt-in)") {
			t.Fatalf("notes must record two-phase opt-in print, got %q", notes)
		}
	})

	t.Run("skips_incomplete_import_export_env", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-incomplete")
		mustMkdir(t, runPrints)
		exportTree := filepath.Join(root, "exports", "20260827T130000Z-abcdef012345")
		mustMkdir(t, exportTree)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=" + exportTree,
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
		if _, err := os.Stat(filepath.Join(outDir, "import-export-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("incomplete import env must skip import-export-drill.sh, err=%v", err)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "METIN2_IMPORT_DRIVER is required") {
			t.Fatalf("notes must record missing driver skip, got %q", notes)
		}
	})

	t.Run("skips_relative_import_export_tree", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-relative")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=relative/exports/20260827T120000Z-abcdef012345",
			"METIN2_IMPORT_DRIVER=sqlite3",
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
		if _, err := os.Stat(filepath.Join(outDir, "import-export-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("relative METIN2_IMPORT_EXPORT_TREE must skip import-export-drill.sh, err=%v", err)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "METIN2_IMPORT_EXPORT_TREE must be an absolute path") {
			t.Fatalf("notes must record relative tree skip, got %q", notes)
		}
	})

	t.Run("honors_custom_import_dsn_env_name", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-custom-dsn-env")
		mustMkdir(t, runPrints)
		exportTree := filepath.Join(root, "exports", "20260827T140000Z-abcdef012345")
		mustMkdir(t, exportTree)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=" + exportTree,
			"METIN2_IMPORT_DRIVER=sqlite3",
			"METIN2_IMPORT_DSN_ENV=LAB_SQL_IMPORT_DSN",
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
		importBody := mustReadContribSample(t, filepath.Join(outDir, "import-export-drill.sh"))
		if !strings.Contains(importBody, "LAB_SQL_IMPORT_DSN") {
			t.Fatalf("custom METIN2_IMPORT_DSN_ENV must appear in stub argv, got %q", importBody)
		}
		if strings.Contains(importBody, "postgres://") || strings.Contains(importBody, "password=") {
			t.Fatalf("custom dsn-env must remain a name only, got %q", importBody)
		}
	})

	t.Run("honors_custom_daemon_log_paths", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "custom-logs")
		mustMkdir(t, runPrints)
		runtimeConfig := filepath.Join(root, "retained-runtime-config-custom.json")
		mustWriteFile(t, runtimeConfig, []byte(`{"ok":true}`+"\n"))
		gamedLog := filepath.Join(root, "custom-gamed.log")
		authdLog := filepath.Join(root, "custom-authd.log")
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_RUNTIME_CONFIG=" + runtimeConfig,
			"METIN2_GAMED_LOG_PATH=" + gamedLog,
			"METIN2_AUTHD_LOG_PATH=" + authdLog,
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
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		exportBody := mustReadContribSample(t, filepath.Join(outDir, "export-quarantine-drill.sh"))
		drillBody := mustReadContribSample(t, filepath.Join(outDir, "backup-restore-drill.sh"))
		for _, body := range []string{migrationBody, exportBody, drillBody} {
			for _, want := range []string{
				"--gamed-log-path",
				gamedLog,
				"--authd-log-path",
				authdLog,
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("printer argv must honor custom log path marker %q, got %q", want, body)
				}
			}
			if strings.Contains(body, "/var/log/metin2/gamed.log") || strings.Contains(body, "/var/log/metin2/authd.log") {
				t.Fatalf("custom log env must replace lab defaults, got %q", body)
			}
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
		if _, err := os.Stat(filepath.Join(outDir, "export-quarantine-drill.sh")); err != nil {
			t.Fatalf("export-quarantine-drill.sh must still print when runtime-config is a symlink: %v", err)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "must not be a symlink") {
			t.Fatalf("notes must record symlink skip, got %q", notes)
		}
		if !strings.Contains(notes, "export-quarantine-drill=printed from build-info") {
			t.Fatalf("notes must still record export-quarantine-drill print, got %q", notes)
		}
	})

	t.Run("skips_symlink_import_export_tree", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "import-symlink")
		mustMkdir(t, runPrints)
		target := filepath.Join(root, "exports", "20260827T150000Z-abcdef012345")
		mustMkdir(t, target)
		linkPath := filepath.Join(root, "exports", "import-tree-link")
		if err := os.Symlink(target, linkPath); err != nil {
			t.Fatalf("symlink import export tree: %v", err)
		}
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_IMPORT_EXPORT_TREE=" + linkPath,
			"METIN2_IMPORT_DRIVER=sqlite3",
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
		if _, err := os.Stat(filepath.Join(outDir, "import-export-drill.sh")); !os.IsNotExist(err) {
			t.Fatalf("symlink METIN2_IMPORT_EXPORT_TREE must skip import-export-drill printer, err=%v", err)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "METIN2_IMPORT_EXPORT_TREE must not be a symlink") {
			t.Fatalf("notes must record import tree symlink skip, got %q", notes)
		}
	})

	t.Run("prints_artifact_gc_aside_purge_when_enabled", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "aside-purge-print")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=YES",
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
		for _, name := range []string{
			"artifact-gc-aside-purge-backups.sh",
			"artifact-gc-aside-purge-migration-runs.sh",
			"artifact-gc-aside-purge-exports.sh",
		} {
			body := mustReadContribSample(t, filepath.Join(outDir, name))
			if !strings.Contains(body, "# stub artifact-gc-aside-purge") {
				t.Fatalf("%s must come from stub printer, got %q", name, body)
			}
			for _, want := range []string{
				"--i-confirm-lab-gc-aside-purge",
				"--min-aside-age-days",
				"7",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("%s argv must include %q, got %q", name, want, body)
				}
			}
		}
		backupsBody := mustReadContribSample(t, filepath.Join(outDir, "artifact-gc-aside-purge-backups.sh"))
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "artifact-gc-aside-purge-migration-runs.sh"))
		exportsBody := mustReadContribSample(t, filepath.Join(outDir, "artifact-gc-aside-purge-exports.sh"))
		if !strings.Contains(backupsBody, "/var/metin2/backups") {
			t.Fatalf("backups purge argv must target /var/metin2/backups, got %q", backupsBody)
		}
		if !strings.Contains(migrationBody, "/var/metin2/migration-runs") {
			t.Fatalf("migration-runs purge argv must target /var/metin2/migration-runs, got %q", migrationBody)
		}
		if !strings.Contains(exportsBody, "/var/metin2/exports") {
			t.Fatalf("exports purge argv must target /var/metin2/exports, got %q", exportsBody)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "artifact-gc-aside-purge=printed for backups/migration-runs/exports") {
			t.Fatalf("notes must record artifact-gc-aside-purge print, got %q", notes)
		}
	})

	t.Run("skips_invalid_aside_min_age_days", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "aside-purge-invalid-age")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=YES",
			"METIN2_GC_ASIDE_MIN_AGE_DAYS=0",
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
		for _, name := range []string{
			"artifact-gc-aside-purge-backups.sh",
			"artifact-gc-aside-purge-migration-runs.sh",
			"artifact-gc-aside-purge-exports.sh",
		} {
			if _, err := os.Stat(filepath.Join(outDir, name)); !os.IsNotExist(err) {
				t.Fatalf("invalid min age must skip %s, err=%v", name, err)
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "METIN2_GC_ASIDE_MIN_AGE_DAYS must be an integer >= 1") {
			t.Fatalf("notes must record invalid min-age skip, got %q", notes)
		}
	})

	t.Run("honors_custom_aside_min_age_and_now", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "aside-purge-custom")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_PRINT_ARTIFACT_GC_ASIDE_PURGE=yes",
			"METIN2_GC_ASIDE_MIN_AGE_DAYS=14",
			"METIN2_GC_ASIDE_NOW=20260827T120000Z",
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
		body := mustReadContribSample(t, filepath.Join(outDir, "artifact-gc-aside-purge-backups.sh"))
		for _, want := range []string{
			"--min-aside-age-days",
			"14",
			"--now",
			"20260827T120000Z",
			"--i-confirm-lab-gc-aside-purge",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("custom aside purge argv must include %q, got %q", want, body)
			}
		}
	})

	t.Run("forwards_intermediate_migration_target_version", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "migration-target-forward")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_MIGRATION_TARGET_VERSION=7",
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
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		for _, want := range []string{
			"--target-version",
			"7",
		} {
			if !strings.Contains(migrationBody, want) {
				t.Fatalf("intermediate forward argv must include %q, got %q", want, migrationBody)
			}
		}
		if strings.Contains(migrationBody, "--allow-rollback") {
			t.Fatalf("intermediate forward argv must omit --allow-rollback, got %q", migrationBody)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "migration-run-retention=printed with --target-version 7") {
			t.Fatalf("notes must record intermediate forward posture, got %q", notes)
		}
		if strings.Contains(notes, "--allow-rollback") {
			t.Fatalf("forward notes must not claim --allow-rollback, got %q", notes)
		}
	})

	t.Run("forwards_rollback_migration_target_version", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "migration-target-rollback")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_MIGRATION_TARGET_VERSION=8",
			"METIN2_MIGRATION_ALLOW_ROLLBACK=yes",
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
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		for _, want := range []string{
			"--target-version",
			"8",
			"--allow-rollback",
		} {
			if !strings.Contains(migrationBody, want) {
				t.Fatalf("intermediate rollback argv must include %q, got %q", want, migrationBody)
			}
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "migration-run-retention=printed with --target-version 8 --allow-rollback") {
			t.Fatalf("notes must record intermediate rollback posture, got %q", notes)
		}
	})

	t.Run("skips_allow_rollback_without_non_latest_target", func(t *testing.T) {
		runPrints := filepath.Join(printsRoot, "migration-target-rollback-skip")
		mustMkdir(t, runPrints)
		cmd := exec.Command("/bin/sh", helperPath)
		cmd.Env = []string{
			"PATH=/usr/bin:/bin",
			"METIN2_MIGRATE_BIN=" + stubPath,
			"METIN2_OPS_PRINTS_ROOT=" + runPrints,
			"METIN2_RETENTION_KEEP_DAYS=14",
			"METIN2_MIGRATION_ALLOW_ROLLBACK=YES",
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
		migrationBody := mustReadContribSample(t, filepath.Join(outDir, "migration-run-retention.sh"))
		if strings.Contains(migrationBody, "--allow-rollback") {
			t.Fatalf("rollback without non-latest target must omit --allow-rollback, got %q", migrationBody)
		}
		if strings.Contains(migrationBody, "--target-version") {
			t.Fatalf("rollback without target must omit --target-version, got %q", migrationBody)
		}
		notes := mustReadContribSample(t, filepath.Join(outDir, "notes.md"))
		if !strings.Contains(notes, "--allow-rollback skipped (requires METIN2_MIGRATION_TARGET_VERSION non-empty and not latest)") {
			t.Fatalf("notes must record rollback skip reason, got %q", notes)
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
