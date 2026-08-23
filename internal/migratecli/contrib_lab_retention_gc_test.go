package migratecli

import (
	"os"
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
		"/var/metin2/ops-prints",
		"/var/metin2/backups",
		"/var/metin2/migration-runs",
		`trap 'rm -f "$TMP_BUILD"'`,
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("helper missing %q", want)
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
