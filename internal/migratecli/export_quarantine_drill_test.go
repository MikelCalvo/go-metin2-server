package migratecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExportQuarantineDrillPrintsLabRetentionCommands(t *testing.T) {
	buildInfoPath := writeTempJSON(t, "build-info.json", `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789deadbeef",
  "build_date": "2026-08-25T12:00:00Z"
}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"export-quarantine-drill",
			"--build-info", buildInfoPath,
			"--ops-base-url", "http://127.0.0.1:6060",
			"--export-base", "/var/metin2/exports",
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
		`# read-only printer: does not execute export/quarantine`,
		`docs/workflow/lab-deployment-topology.md`,
		`docs/plans/2026-08-25-hermetic-export-quarantine-offline-cli-proof.md`,
		`OPS='http://127.0.0.1:6060'`,
		`AUTH_OPS='http://127.0.0.1:6061'`,
		`EXPORTS_BASE='/var/metin2/exports'`,
		`GAMED_LOG='/var/log/metin2/gamed.log'`,
		`AUTHD_LOG='/var/log/metin2/authd.log'`,
		`COMMIT12='abcdef012345'`,
		`TS=$(date -u +%Y%m%dT%H%M%SZ)`,
		`BASE="${EXPORTS_BASE}/${TS}-${COMMIT12}"`,
		`mkdir -p "$BASE"/account-character-roster "$BASE"/character-item-state "$BASE"/character-point-state "$BASE"/character-myshop-unit-prices "$BASE"/auth-login-ticket-handoff "$BASE"/character-quest-state "$BASE"/character-safebox-state "$BASE"/item-template-state "$BASE"/static-actor-content-state "$BASE"/bootstrap-ground-item-state`,
		`curl -sS "$OPS/local/build-info" > "$BASE/gamed-build-info.json"`,
		`curl -sS "$AUTH_OPS/local/build-info" > "$BASE/authd-build-info.json"`,
		`curl -sS "$OPS/local/runtime-config" > "$BASE/runtime-config.json"`,
		`curl -sS "$OPS/local/db/migrations/catalog" > "$BASE/migration-catalog.json"`,
		`if [ -f "$GAMED_LOG" ]; then cp -p "$GAMED_LOG" "$BASE/gamed.log"; fi`,
		`if [ -f "$AUTHD_LOG" ]; then cp -p "$AUTHD_LOG" "$BASE/authd.log"; fi`,
		`cat > "$BASE/notes.md" <<'EOF'`,
		`# Export/quarantine drill notes`,
		`Do not paste DSNs, passwords, login keys, tickets, or executable SQL here.`,
		`"$OPS/local/account-store/exports/account-character-roster"`,
		`"$BASE/account-character-roster/export.json"`,
		`metin2-migrate quarantine-export --kind account-character-roster --export "$BASE/account-character-roster/export.json"`,
		`"$BASE/account-character-roster/quarantine.json"`,
		`"$OPS/local/account-store/exports/character-item-state"`,
		`metin2-migrate quarantine-export --kind character-item-state --export "$BASE/character-item-state/export.json"`,
		`"$OPS/local/account-store/exports/character-point-state"`,
		`metin2-migrate quarantine-export --kind character-point-state --export "$BASE/character-point-state/export.json"`,
		`"$OPS/local/account-store/exports/character-myshop-unit-prices"`,
		`metin2-migrate quarantine-export --kind character-myshop-unit-prices --export "$BASE/character-myshop-unit-prices/export.json"`,
		`"$OPS/local/login-tickets/exports/auth-login-ticket-handoff"`,
		`metin2-migrate quarantine-export --kind auth-login-ticket-handoff --export "$BASE/auth-login-ticket-handoff/export.json"`,
		`"$OPS/local/quest-state/exports/character-quest-state"`,
		`metin2-migrate quarantine-export --kind character-quest-state --export "$BASE/character-quest-state/export.json"`,
		`"$OPS/local/safebox-store/exports/character-safebox-state"`,
		`metin2-migrate quarantine-export --kind character-safebox-state --export "$BASE/character-safebox-state/export.json"`,
		`"$OPS/local/item-templates/exports/item-template-state"`,
		`metin2-migrate quarantine-export --kind item-template-state --export "$BASE/item-template-state/export.json"`,
		`"$OPS/local/static-actors/exports/static-actor-content-state"`,
		`metin2-migrate quarantine-export --kind static-actor-content-state --export "$BASE/static-actor-content-state/export.json"`,
		`"$OPS/local/ground-items/exports/bootstrap-ground-item-state"`,
		`metin2-migrate quarantine-export --kind bootstrap-ground-item-state --export "$BASE/bootstrap-ground-item-state/export.json"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "DROP TABLE") || strings.Contains(body, "postgres://") || strings.Contains(body, "memory://") {
		t.Fatalf("export-quarantine-drill must not expose SQL or DSN text, got %s", body)
	}
	if strings.Contains(body, "does not execute export/quarantine") && strings.Contains(body, "executes export") {
		t.Fatalf("printer must remain read-only wording only")
	}

	idxGamedBuild := strings.Index(body, `curl -sS "$OPS/local/build-info" > "$BASE/gamed-build-info.json"`)
	idxAuthdBuild := strings.Index(body, `curl -sS "$AUTH_OPS/local/build-info" > "$BASE/authd-build-info.json"`)
	idxRuntime := strings.Index(body, `> "$BASE/runtime-config.json"`)
	idxCatalog := strings.Index(body, `> "$BASE/migration-catalog.json"`)
	idxNotes := strings.Index(body, `cat > "$BASE/notes.md" <<'EOF'`)
	idxRosterExport := strings.Index(body, `"$OPS/local/account-store/exports/account-character-roster"`)
	idxRosterQuarantine := strings.Index(body, `quarantine-export --kind account-character-roster`)
	idxSafeboxExport := strings.Index(body, `"$OPS/local/safebox-store/exports/character-safebox-state"`)
	idxGroundExport := strings.Index(body, `"$OPS/local/ground-items/exports/bootstrap-ground-item-state"`)
	if idxGamedBuild < 0 || idxAuthdBuild < 0 || idxRuntime < 0 || idxCatalog < 0 || idxNotes < 0 || idxRosterExport < 0 || idxRosterQuarantine < 0 || idxSafeboxExport < 0 || idxGroundExport < 0 {
		t.Fatalf("missing expected ordering markers in stdout:\n%s", body)
	}
	if !(idxGamedBuild < idxAuthdBuild && idxAuthdBuild < idxRuntime && idxRuntime < idxCatalog && idxCatalog < idxNotes && idxNotes < idxRosterExport && idxRosterExport < idxRosterQuarantine && idxRosterQuarantine < idxSafeboxExport && idxSafeboxExport < idxGroundExport) {
		t.Fatalf("expected identity -> runtime/catalog -> notes -> roster export/quarantine -> later kinds ordering, got idxs gamed=%d authd=%d runtime=%d catalog=%d notes=%d rosterExport=%d rosterQ=%d safebox=%d ground=%d\n%s",
			idxGamedBuild, idxAuthdBuild, idxRuntime, idxCatalog, idxNotes, idxRosterExport, idxRosterQuarantine, idxSafeboxExport, idxGroundExport, body)
	}
}

func TestRunExportQuarantineDrillReadsStdinBuildInfo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"export-quarantine-drill",
			"--build-info", "-",
			"--ops-base-url", "https://127.0.0.1:7060/ops",
			"--authd-ops-base-url", "https://127.0.0.1:7061/ops",
			"--export-base", "/tmp/metin2-exports",
			"--gamed-log-path", "/tmp/gamed.log",
			"--authd-log-path", "/tmp/authd.log",
		},
		strings.NewReader(`{
  "version": "dev",
  "commit": "0123456789abcdef",
  "build_date": "2026-08-25T01:02:03Z"
}`),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	body := stdout.String()
	for _, want := range []string{
		`OPS='https://127.0.0.1:7060/ops'`,
		`AUTH_OPS='https://127.0.0.1:7061/ops'`,
		`EXPORTS_BASE='/tmp/metin2-exports'`,
		`GAMED_LOG='/tmp/gamed.log'`,
		`AUTHD_LOG='/tmp/authd.log'`,
		`COMMIT12='0123456789ab'`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
}

func TestRunExportQuarantineDrillRejectsBlankCommit(t *testing.T) {
	path := writeTempJSON(t, "build-info.json", `{
  "version": "v0.1.0",
  "commit": "   ",
  "build_date": "2026-08-25T12:00:00Z"
}`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-quarantine-drill", "--build-info", path}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "commit is required") {
		t.Fatalf("expected commit error, got %q", stderr.String())
	}
}

func TestRunExportQuarantineDrillRejectsRelativeExportBase(t *testing.T) {
	path := writeTempJSON(t, "build-info.json", `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789",
  "build_date": "2026-08-25T12:00:00Z"
}`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-quarantine-drill", "--build-info", path, "--export-base", "exports"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "export-base must be an absolute path") {
		t.Fatalf("expected absolute path error, got %q", stderr.String())
	}
}

func TestRunExportQuarantineDrillRejectsInvalidOpsBaseURL(t *testing.T) {
	path := writeTempJSON(t, "build-info.json", `{
  "version": "v0.1.0",
  "commit": "abcdef0123456789",
  "build_date": "2026-08-25T12:00:00Z"
}`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-quarantine-drill", "--build-info", path, "--ops-base-url", "ftp://127.0.0.1:6060"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "ops-base-url") {
		t.Fatalf("expected ops-base-url error, got %q", stderr.String())
	}
}

func TestRunExportQuarantineDrillRejectsMalformedAndOversizedInput(t *testing.T) {
	oversized := "{" + strings.Repeat(`"x":1,`, 20_000) + `"version":"v","commit":"abc","build_date":"d"}`
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "malformed", payload: `{"version":`, want: "decode build-info"},
		{name: "invalid-utf8", payload: "{\x80\"commit\":\"abc\"}", want: "not valid UTF-8"},
		{name: "empty", payload: "   ", want: "build-info is empty"},
		{name: "null", payload: "null", want: "build-info is empty"},
		{name: "oversized", payload: oversized, want: "exceeds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "build-info.json")
			if err := os.WriteFile(path, []byte(tc.payload), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"export-quarantine-drill", "--build-info", path}, nil, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected %q in stderr, got %q", tc.want, stderr.String())
			}
		})
	}
}

func TestRunExportQuarantineDrillRejectsSymlinkBuildInfo(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.json")
	if err := os.WriteFile(target, []byte(`{"version":"v","commit":"abcdef012345","build_date":"2026-08-25T00:00:00Z"}`), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export-quarantine-drill", "--build-info", link}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "must not be a symlink") {
		t.Fatalf("expected symlink error, got %q", stderr.String())
	}
}

func TestRunExportQuarantineDrillUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing-build-info", args: []string{"export-quarantine-drill"}},
		{name: "unexpected-arg", args: []string{"export-quarantine-drill", "--build-info", "-", "extra"}},
		{name: "unknown-flag", args: []string{"export-quarantine-drill", "--nope"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(`{"version":"v","commit":"abcdef012345","build_date":"d"}`), &stdout, &stderr)
			if code != 2 {
				t.Fatalf("expected exit 2, got %d stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "export-quarantine-drill usage:") {
				t.Fatalf("expected usage text, got %q", stderr.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommandMentionsExportQuarantineDrill(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "export-quarantine-drill") {
		t.Fatalf("expected usage to mention export-quarantine-drill, got %q", stderr.String())
	}
}
