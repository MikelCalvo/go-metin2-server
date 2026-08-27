//go:build sqlite_harness

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

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestImportExportDrillSQLiteHermeticPrintedScriptImportsEmptyTipKinds(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)

	exportTree := filepath.Join(t.TempDir(), "20260827T120000Z-abcdef012345")
	mustMaterializeEmptyImportExportQuarantineTree(t, exportTree)

	dbPath := filepath.Join(t.TempDir(), "import-export-drill.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"

	mustApplyCatalogToTipWithSQLiteMigrate(t, migrateBin, dsn)

	var printStdout bytes.Buffer
	var printStderr bytes.Buffer
	printCode := Run(
		[]string{
			"import-export-drill",
			"--export-tree", exportTree,
			"--driver", "sqlite",
			"--i-confirm-print-sql-import-drill",
		},
		nil,
		&printStdout,
		&printStderr,
	)
	if printCode != exitOK {
		t.Fatalf("expected import-export-drill exit %d, got %d stderr=%q", exitOK, printCode, printStderr.String())
	}
	if printStderr.Len() != 0 {
		t.Fatalf("expected no stderr from import-export-drill, got %q", printStderr.String())
	}
	script := printStdout.String()
	for _, forbidden := range []string{
		dsn,
		"postgres://",
		"memory://",
		"CREATE TABLE",
		"--dsn 'sqlite'",
		"--dsn sqlite",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("import-export-drill must not expose %q, got %s", forbidden, script)
		}
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METIN2_IMPORT_DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, script, env)
	if code != 0 {
		t.Fatalf("expected printed import-export-drill script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}

	wantMarkers := map[string]string{
		"account-character-roster":    `"account_count": 0`,
		"character-item-state":        `"inventory_item_count": 0`,
		"character-point-state":       `"point_row_count": 0`,
		"character-quest-state":       `"flag_count": 0`,
		"character-safebox-state":     `"password_count": 0`,
		"auth-login-ticket-handoff":   `"ticket_count": 0`,
		"item-template-state":         `"template_count": 0`,
		"static-actor-content-state":  `"static_actor_count": 0`,
		"bootstrap-ground-item-state": `"ground_item_count": 0`,
	}
	for _, kind := range exportQuarantineKinds {
		resultPath := filepath.Join(exportTree, kind, "import-result.json")
		body := mustReadFileString(t, resultPath)
		want, ok := wantMarkers[kind]
		if !ok {
			t.Fatalf("missing expected marker for kind %q", kind)
		}
		if !strings.Contains(body, want) && !strings.Contains(compactJSONForAssert(body), compactJSONForAssert(want)) {
			t.Fatalf("kind %s import-result missing %s, got %s", kind, want, body)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", dsn} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("import-result for %s must not contain %q, got %s", kind, forbidden, body)
			}
		}
	}
}

func mustMaterializeEmptyImportExportQuarantineTree(t *testing.T, exportTree string) {
	t.Helper()
	payloads := map[string]string{
		"account-character-roster":    `{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`,
		"character-item-state":        `{"migration_version":3,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":       `{"migration_version":11,"migration_name":"character_point_state","points":[]}`,
		"character-quest-state":       `{"migration_version":4,"migration_name":"character_quest_state","flags":[]}`,
		"character-safebox-state":     `{"migration_version":15,"migration_name":"character_safebox_money","passwords":[],"items":[]}`,
		"auth-login-ticket-handoff":   `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","tickets":[]}`,
		"item-template-state":         `{"migration_version":9,"migration_name":"item_template_refine_info","templates":[],"sockets":[],"attributes":[],"use_effects":[],"equip_effects":[],"refine_infos":[],"refine_materials":[]}`,
		"static-actor-content-state":  `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definitions":[],"merchant_catalog_entries":[],"quest_flag_reward_items":[],"quest_flag_consume_items":[],"static_actors":[],"reward_drops":[],"combat_profiles":[],"combat_profile_death_reward_drops":[]}`,
		"bootstrap-ground-item-state": `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_items":[]}`,
	}
	for _, kind := range exportQuarantineKinds {
		payload, ok := payloads[kind]
		if !ok {
			t.Fatalf("missing empty quarantine payload for kind %q", kind)
		}
		dir := filepath.Join(exportTree, kind)
		mustMkdir(t, dir)
		mustWriteFile(t, filepath.Join(dir, "quarantine.json"), []byte(payload+"\n"))
	}
}

func mustApplyCatalogToTipWithSQLiteMigrate(t *testing.T, migrateBin, dsn string) {
	t.Helper()

	var snapshotStdout bytes.Buffer
	var snapshotStderr bytes.Buffer
	if code := Run([]string{"empty-ledger-snapshot"}, nil, &snapshotStdout, &snapshotStderr); code != exitOK {
		t.Fatalf("empty-ledger-snapshot: exit=%d stderr=%q", code, snapshotStderr.String())
	}
	snapshotPath := filepath.Join(t.TempDir(), "empty-ledger-snapshot.json")
	mustWriteFile(t, snapshotPath, snapshotStdout.Bytes())

	cmd := exec.Command(
		migrateBin,
		"apply",
		"--driver", "sqlite",
		"--dsn", dsn,
		"--ledger-snapshot", snapshotPath,
		"--target-version", "latest",
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("metin2-migrate apply to tip: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}

	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	wantTip := catalog[len(catalog)-1].Version
	var applyResult struct {
		CurrentVersion int `json:"current_version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &applyResult); err != nil {
		t.Fatalf("decode apply result: %v body=%s", err, stdout.String())
	}
	if applyResult.CurrentVersion != wantTip {
		t.Fatalf("apply CurrentVersion = %d, want tip %d body=%s", applyResult.CurrentVersion, wantTip, stdout.String())
	}
}

func mustBuildMetin2MigrateWithSQLiteHarness(t *testing.T, binDir string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	out := filepath.Join(binDir, "metin2-migrate")
	cmd := exec.Command("go", "build", "-tags=sqlite_harness", "-o", out, "./cmd/metin2-migrate")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build -tags=sqlite_harness ./cmd/metin2-migrate: %v stderr=%s", err, stderr.String())
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

func runPrintedShellScriptWithEnv(t *testing.T, script string, env []string) (stdout string, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = env
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

func mustReadFileString(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func compactJSONForAssert(raw string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		return strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), "\n", "")
	}
	return buf.String()
}
