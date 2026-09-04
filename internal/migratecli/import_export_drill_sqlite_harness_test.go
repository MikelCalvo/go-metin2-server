//go:build sqlite_harness

package migratecli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
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
		"account-character-roster":     `"account_count": 0`,
		"character-item-state":         `"inventory_item_count": 0`,
		"character-point-state":        `"point_row_count": 0`,
		"character-myshop-unit-prices": `"price_row_count": 0`,
		"character-quest-state":        `"flag_count": 0`,
		"character-safebox-state":      `"password_count": 0`,
		"auth-login-ticket-handoff":    `"ticket_count": 0`,
		"item-template-state":          `"template_count": 0`,
		"static-actor-content-state":   `"static_actor_count": 0`,
		"bootstrap-ground-item-state":  `"ground_item_count": 0`,
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
	assertImportExportDrillStatusArtifacts(t, exportTree, wantMarkers, dsn)
}

func TestImportExportDrillSQLiteHermeticPrintedScriptImportsSeededTipKinds(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)

	exportTree := filepath.Join(t.TempDir(), "20260827T130000Z-seededabcdef")
	mustMaterializeSeededImportExportQuarantineTree(t, exportTree)

	dbPath := filepath.Join(t.TempDir(), "import-export-drill-seeded.sqlite")
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
		"account-character-roster":     `"account_count": 1`,
		"character-item-state":         `"inventory_item_count": 1`,
		"character-point-state":        `"point_row_count": 255`,
		"character-myshop-unit-prices": `"price_row_count": 2`,
		"character-quest-state":        `"flag_count": 1`,
		"character-safebox-state":      `"password_count": 1`,
		"auth-login-ticket-handoff":    `"ticket_count": 1`,
		"item-template-state":          `"refine_info_count": 2`,
		"static-actor-content-state":   `"static_actor_count": 1`,
		"bootstrap-ground-item-state":  `"ground_item_count": 1`,
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
	assertImportExportDrillStatusArtifacts(t, exportTree, wantMarkers, dsn)

	assertSeededImportExportDrillSQLiteRows(t, dsn)
}

func TestImportExportDrillSQLiteHermeticPrintedScriptScopedReplaceImportsEmptyTipKinds(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)

	exportTree := filepath.Join(t.TempDir(), "20260903T141000Z-scopedreplace-empty")
	mustMaterializeEmptyImportExportQuarantineTree(t, exportTree)

	dbPath := filepath.Join(t.TempDir(), "import-export-drill-scoped-replace-empty.sqlite")
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
			"--i-confirm-print-scoped-replace",
		},
		nil,
		&printStdout,
		&printStderr,
	)
	if printCode != exitOK {
		t.Fatalf("expected scoped-replace import-export-drill exit %d, got %d stderr=%q", exitOK, printCode, printStderr.String())
	}
	if printStderr.Len() != 0 {
		t.Fatalf("expected no stderr from scoped-replace import-export-drill, got %q", printStderr.String())
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
			t.Fatalf("scoped-replace import-export-drill must not expose %q, got %s", forbidden, script)
		}
	}
	if !strings.Contains(script, "opt-in scoped replace") {
		t.Fatalf("expected scoped-replace comment in opt-in drill stdout:\n%s", script)
	}
	if !strings.Contains(script, "FK-safe") {
		t.Fatalf("expected FK-safe ordering comment in opt-in drill stdout:\n%s", script)
	}
	for _, kind := range importExportDrillScopedReplaceKinds {
		want := fmt.Sprintf(
			`metin2-migrate import-export --kind %s --export "$EXPORT_TREE/%s/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import --i-confirm-scoped-replace > "$EXPORT_TREE/%s/import-result.json"`,
			kind, kind, kind,
		)
		if !strings.Contains(script, want) {
			t.Fatalf("expected scoped-replace import line for %s:\nwant %q\nbody:\n%s", kind, want, script)
		}
	}
	idxItem := strings.Index(script, `import-export --kind character-item-state`)
	idxRoster := strings.Index(script, `import-export --kind account-character-roster`)
	if !(idxItem >= 0 && idxRoster >= 0 && idxItem < idxRoster) {
		t.Fatalf("expected FK-safe scoped-replace order with roster after character-item-state, got item=%d roster=%d\n%s", idxItem, idxRoster, script)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METIN2_IMPORT_DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, script, env)
	if code != 0 {
		t.Fatalf("expected scoped-replace printed import-export-drill script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}

	wantMarkers := map[string]string{
		"account-character-roster":     `"account_count": 0`,
		"character-item-state":         `"inventory_item_count": 0`,
		"character-point-state":        `"point_row_count": 0`,
		"character-myshop-unit-prices": `"price_row_count": 0`,
		"character-quest-state":        `"flag_count": 0`,
		"character-safebox-state":      `"password_count": 0`,
		"auth-login-ticket-handoff":    `"ticket_count": 0`,
		"item-template-state":          `"template_count": 0`,
		"static-actor-content-state":   `"static_actor_count": 0`,
		"bootstrap-ground-item-state":  `"ground_item_count": 0`,
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
		if !strings.Contains(body, `"replaced": true`) && !strings.Contains(compactJSONForAssert(body), `"replaced":true`) {
			t.Fatalf("kind %s import-result missing replaced:true, got %s", kind, body)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", dsn} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("import-result for %s must not contain %q, got %s", kind, forbidden, body)
			}
		}
	}
	assertImportExportDrillStatusArtifacts(t, exportTree, wantMarkers, dsn)
	assertImportExportDrillStatusReplaced(t, exportTree, dsn)
}

func TestImportExportDrillSQLiteHermeticPrintedScriptScopedReplaceReimportsSeededTipKindsOmittingRoster(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)

	exportTree := filepath.Join(t.TempDir(), "20260903T140000Z-scopedreplace")
	mustMaterializeSeededImportExportQuarantineTree(t, exportTree)

	dbPath := filepath.Join(t.TempDir(), "import-export-drill-scoped-replace.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"

	mustApplyCatalogToTipWithSQLiteMigrate(t, migrateBin, dsn)

	var insertPrintStdout bytes.Buffer
	var insertPrintStderr bytes.Buffer
	insertPrintCode := Run(
		[]string{
			"import-export-drill",
			"--export-tree", exportTree,
			"--driver", "sqlite",
			"--i-confirm-print-sql-import-drill",
		},
		nil,
		&insertPrintStdout,
		&insertPrintStderr,
	)
	if insertPrintCode != exitOK {
		t.Fatalf("expected insert-only import-export-drill exit %d, got %d stderr=%q", exitOK, insertPrintCode, insertPrintStderr.String())
	}
	if insertPrintStderr.Len() != 0 {
		t.Fatalf("expected no stderr from insert-only import-export-drill, got %q", insertPrintStderr.String())
	}
	insertScript := insertPrintStdout.String()
	if strings.Contains(insertScript, "--i-confirm-scoped-replace") {
		t.Fatalf("insert-only import-export-drill must omit --i-confirm-scoped-replace, got %s", insertScript)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METIN2_IMPORT_DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, insertScript, env)
	if code != 0 {
		t.Fatalf("expected insert-only printed import-export-drill script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertSeededImportExportDrillSQLiteRows(t, dsn)

	var replacePrintStdout bytes.Buffer
	var replacePrintStderr bytes.Buffer
	replacePrintCode := Run(
		[]string{
			"import-export-drill",
			"--export-tree", exportTree,
			"--driver", "sqlite",
			"--i-confirm-print-sql-import-drill",
			"--i-confirm-print-scoped-replace",
		},
		nil,
		&replacePrintStdout,
		&replacePrintStderr,
	)
	if replacePrintCode != exitOK {
		t.Fatalf("expected scoped-replace import-export-drill exit %d, got %d stderr=%q", exitOK, replacePrintCode, replacePrintStderr.String())
	}
	if replacePrintStderr.Len() != 0 {
		t.Fatalf("expected no stderr from scoped-replace import-export-drill, got %q", replacePrintStderr.String())
	}
	replaceScript := replacePrintStdout.String()
	for _, forbidden := range []string{
		dsn,
		"postgres://",
		"memory://",
		"CREATE TABLE",
		"--dsn 'sqlite'",
		"--dsn sqlite",
	} {
		if strings.Contains(replaceScript, forbidden) {
			t.Fatalf("scoped-replace import-export-drill must not expose %q, got %s", forbidden, replaceScript)
		}
	}
	if !strings.Contains(replaceScript, "opt-in scoped replace") {
		t.Fatalf("expected scoped-replace comment in opt-in drill stdout:\n%s", replaceScript)
	}
	if !strings.Contains(replaceScript, "Seeded full-tree single-pass") {
		t.Fatalf("expected seeded tip-0002 limitation comment in opt-in drill stdout:\n%s", replaceScript)
	}
	for _, kind := range importExportDrillScopedReplaceKinds {
		want := fmt.Sprintf(
			`metin2-migrate import-export --kind %s --export "$EXPORT_TREE/%s/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import --i-confirm-scoped-replace > "$EXPORT_TREE/%s/import-result.json"`,
			kind, kind, kind,
		)
		if !strings.Contains(replaceScript, want) {
			t.Fatalf("expected scoped-replace import line for %s:\nwant %q\nbody:\n%s", kind, want, replaceScript)
		}
	}
	idxItem := strings.Index(replaceScript, `import-export --kind character-item-state`)
	idxRoster := strings.Index(replaceScript, `import-export --kind account-character-roster`)
	if !(idxItem >= 0 && idxRoster >= 0 && idxItem < idxRoster) {
		t.Fatalf("expected FK-safe scoped-replace order with roster after character-item-state, got item=%d roster=%d\n%s", idxItem, idxRoster, replaceScript)
	}

	// Seeded full-tree single-pass still fails closed on tip-0002 while child tip
	// rows remain (child tip replace re-inserts FK dependents). Prove the operator
	// omit-roster pass that re-backfills every non-roster tip kind.
	omitRosterScript := omitImportExportDrillKindLines(replaceScript, "account-character-roster")
	if strings.Contains(omitRosterScript, `import-export --kind account-character-roster`) {
		t.Fatalf("omit-roster script still contains account-character-roster import-export:\n%s", omitRosterScript)
	}
	stdout, stderr, code = runPrintedShellScriptWithEnv(t, omitRosterScript, env)
	if code != 0 {
		t.Fatalf("expected omit-roster scoped-replace printed script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}

	wantMarkers := map[string]string{
		"character-item-state":         `"inventory_item_count": 1`,
		"character-point-state":        `"point_row_count": 255`,
		"character-myshop-unit-prices": `"price_row_count": 2`,
		"character-quest-state":        `"flag_count": 1`,
		"character-safebox-state":      `"password_count": 1`,
		"auth-login-ticket-handoff":    `"ticket_count": 1`,
		"item-template-state":          `"refine_info_count": 2`,
		"static-actor-content-state":   `"static_actor_count": 1`,
		"bootstrap-ground-item-state":  `"ground_item_count": 1`,
	}
	for kind, want := range wantMarkers {
		resultPath := filepath.Join(exportTree, kind, "import-result.json")
		body := mustReadFileString(t, resultPath)
		if !strings.Contains(body, want) && !strings.Contains(compactJSONForAssert(body), compactJSONForAssert(want)) {
			t.Fatalf("kind %s import-result missing %s, got %s", kind, want, body)
		}
		if !strings.Contains(body, `"replaced": true`) && !strings.Contains(compactJSONForAssert(body), `"replaced":true`) {
			t.Fatalf("kind %s import-result missing replaced:true, got %s", kind, body)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", dsn} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("import-result for %s must not contain %q, got %s", kind, forbidden, body)
			}
		}
	}
	assertImportExportDrillStatusArtifactsForKinds(t, exportTree, wantMarkers, dsn)
	assertImportExportDrillStatusReplacedForKinds(t, exportTree, wantMarkers, dsn)
	assertSeededImportExportDrillSQLiteRows(t, dsn)
}

func TestImportExportDrillSQLiteHermeticPrintedScriptTwoPhaseWipeRosterReimportsSeededTipKinds(t *testing.T) {
	binDir := t.TempDir()
	migrateBin := mustBuildMetin2MigrateWithSQLiteHarness(t, binDir)

	exportTree := filepath.Join(t.TempDir(), "20260903T181500Z-twophase")
	mustMaterializeSeededImportExportQuarantineTree(t, exportTree)

	dbPath := filepath.Join(t.TempDir(), "import-export-drill-two-phase.sqlite")
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)"

	mustApplyCatalogToTipWithSQLiteMigrate(t, migrateBin, dsn)

	var insertPrintStdout bytes.Buffer
	var insertPrintStderr bytes.Buffer
	insertPrintCode := Run(
		[]string{
			"import-export-drill",
			"--export-tree", exportTree,
			"--driver", "sqlite",
			"--i-confirm-print-sql-import-drill",
		},
		nil,
		&insertPrintStdout,
		&insertPrintStderr,
	)
	if insertPrintCode != exitOK {
		t.Fatalf("expected insert-only import-export-drill exit %d, got %d stderr=%q", exitOK, insertPrintCode, insertPrintStderr.String())
	}
	insertScript := insertPrintStdout.String()
	if strings.Contains(insertScript, "--i-confirm-scoped-replace") {
		t.Fatalf("insert-only import-export-drill must omit --i-confirm-scoped-replace, got %s", insertScript)
	}

	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METIN2_IMPORT_DSN="+dsn,
	)
	stdout, stderr, code := runPrintedShellScriptWithEnv(t, insertScript, env)
	if code != 0 {
		t.Fatalf("expected insert-only printed import-export-drill script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertSeededImportExportDrillSQLiteRows(t, dsn)

	var twoPhasePrintStdout bytes.Buffer
	var twoPhasePrintStderr bytes.Buffer
	twoPhasePrintCode := Run(
		[]string{
			"import-export-drill",
			"--export-tree", exportTree,
			"--driver", "sqlite",
			"--i-confirm-print-sql-import-drill",
			"--i-confirm-print-two-phase-wipe-roster-reimport",
		},
		nil,
		&twoPhasePrintStdout,
		&twoPhasePrintStderr,
	)
	if twoPhasePrintCode != exitOK {
		t.Fatalf("expected two-phase import-export-drill exit %d, got %d stderr=%q", exitOK, twoPhasePrintCode, twoPhasePrintStderr.String())
	}
	if twoPhasePrintStderr.Len() != 0 {
		t.Fatalf("expected no stderr from two-phase import-export-drill, got %q", twoPhasePrintStderr.String())
	}
	twoPhaseScript := twoPhasePrintStdout.String()
	for _, want := range []string{
		"two-phase wipe → roster → omit-roster reimport",
		`metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" > "$EXPORT_TREE/export-tree-status-before.json"`,
		"phase 1: synthesize wipe-quarantine.json artifacts",
		"phase 2: wipe character-FK tip kinds",
		"phase 3: scoped-replace tip-0002 account-character-roster",
		"phase 4: scoped-replace reimport non-roster tip kinds",
		"synthesize-wipe-export --kind character-item-state",
		"wipe-quarantine.json",
		"wipe-import-result.json",
		`metin2-migrate export-tree-status --export-tree "$EXPORT_TREE" --require-quarantine-complete --require-two-phase-wipe-artifacts-complete --require-import-result-artifacts-complete --require-wipe-import-artifacts-complete > "$EXPORT_TREE/export-tree-status-after.json"`,
	} {
		if !strings.Contains(twoPhaseScript, want) {
			t.Fatalf("expected %q in two-phase drill stdout:\n%s", want, twoPhaseScript)
		}
	}
	for _, forbidden := range []string{
		dsn,
		"postgres://",
		"memory://",
		"CREATE TABLE",
		"--dsn 'sqlite'",
		"--dsn sqlite",
	} {
		if strings.Contains(twoPhaseScript, forbidden) {
			t.Fatalf("two-phase import-export-drill must not expose %q, got %s", forbidden, twoPhaseScript)
		}
	}
	idxSynthesize := strings.Index(twoPhaseScript, "phase 1: synthesize wipe-quarantine.json artifacts")
	idxWipe := strings.Index(twoPhaseScript, "phase 2: wipe character-FK tip kinds")
	idxRoster := strings.Index(twoPhaseScript, "phase 3: scoped-replace tip-0002 account-character-roster")
	idxReimport := strings.Index(twoPhaseScript, "phase 4: scoped-replace reimport non-roster tip kinds")
	if !(idxSynthesize >= 0 && idxWipe > idxSynthesize && idxRoster > idxWipe && idxReimport > idxRoster) {
		t.Fatalf("expected two-phase ordering synthesize<%d wipe<%d roster<%d reimport<%d\n%s", idxSynthesize, idxWipe, idxRoster, idxReimport, twoPhaseScript)
	}
	if strings.Count(twoPhaseScript, `import-export --kind account-character-roster --export "$EXPORT_TREE/account-character-roster/quarantine.json"`) != 1 {
		t.Fatalf("expected exactly one roster quarantine import line, got:\n%s", twoPhaseScript)
	}

	stdout, stderr, code = runPrintedShellScriptWithEnv(t, twoPhaseScript, env)
	if code != 0 {
		t.Fatalf("expected two-phase printed import-export-drill script exit 0, got %d stdout=%q stderr=%q", code, stdout, stderr)
	}

	for _, kind := range importExportDrillWipeKinds {
		wipePath := filepath.Join(exportTree, kind, "wipe-quarantine.json")
		wipeBody := mustReadFileString(t, wipePath)
		if strings.Contains(wipeBody, `"summary"`) {
			t.Fatalf("wipe-quarantine.json for %s must be bare export JSON, got %s", kind, wipeBody)
		}
		wipeStatusPath := filepath.Join(exportTree, kind, "wipe-quarantine-status.json")
		wipeStatusRaw, err := os.ReadFile(wipeStatusPath)
		if err != nil {
			t.Fatalf("read wipe-quarantine-status for %s: %v", kind, err)
		}
		var wipeStatus synthesizeWipeExportStatus
		if err := json.Unmarshal(wipeStatusRaw, &wipeStatus); err != nil {
			t.Fatalf("decode wipe-quarantine-status for %s: %v body=%s", kind, err, string(wipeStatusRaw))
		}
		if wipeStatus.Format != synthesizeWipeExportStatusFormat || !wipeStatus.Present || wipeStatus.Kind != kind {
			t.Fatalf("unexpected wipe-quarantine-status for %s: %#v", kind, wipeStatus)
		}
		wipeRaw, err := os.ReadFile(wipePath)
		if err != nil {
			t.Fatalf("read wipe-quarantine for %s: %v", kind, err)
		}
		wantWipeSHA := sha256Hex(wipeRaw)
		if wipeStatus.WipeExportSHA256 != wantWipeSHA {
			t.Fatalf("wipe-quarantine-status sha mismatch for %s: got %s want %s", kind, wipeStatus.WipeExportSHA256, wantWipeSHA)
		}
		if wipeStatus.ScopeCount <= 0 || len(wipeStatus.ScopeIDs) != wipeStatus.ScopeCount {
			t.Fatalf("wipe-quarantine-status scope for %s invalid: %#v", kind, wipeStatus)
		}
		wantScopeKey := "character_ids"
		if kind == "bootstrap-ground-item-state" {
			wantScopeKey = "vids"
		}
		if wipeStatus.ScopeKey != wantScopeKey {
			t.Fatalf("wipe-quarantine-status scope_key for %s: got %q want %q", kind, wipeStatus.ScopeKey, wantScopeKey)
		}
		wipeResultPath := filepath.Join(exportTree, kind, "wipe-import-result.json")
		wipeResult := mustReadFileString(t, wipeResultPath)
		if !strings.Contains(wipeResult, `"replaced": true`) && !strings.Contains(compactJSONForAssert(wipeResult), `"replaced":true`) {
			t.Fatalf("wipe import-result for %s missing replaced:true, got %s", kind, wipeResult)
		}
	}

	for _, name := range []string{"export-tree-status-before.json", "export-tree-status-after.json"} {
		statusPath := filepath.Join(exportTree, name)
		statusRaw, err := os.ReadFile(statusPath)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var treeStatus exportTreeStatus
		if err := json.Unmarshal(statusRaw, &treeStatus); err != nil {
			t.Fatalf("decode %s: %v body=%s", name, err, string(statusRaw))
		}
		if treeStatus.Format != exportTreeStatusFormat || !treeStatus.Present || treeStatus.ExportTree != exportTree {
			t.Fatalf("unexpected %s envelope: %#v", name, treeStatus)
		}
		if !treeStatus.QuarantineComplete {
			t.Fatalf("%s must report quarantine_complete=true, got %#v", name, treeStatus)
		}
		if name == "export-tree-status-after.json" && !treeStatus.TwoPhaseWipeArtifactsComplete {
			t.Fatalf("export-tree-status-after.json must report two_phase_wipe_artifacts_complete=true, got %#v", treeStatus)
		}
		if name == "export-tree-status-after.json" && !treeStatus.ImportResultArtifactsComplete {
			t.Fatalf("export-tree-status-after.json must report import_result_artifacts_complete=true, got %#v", treeStatus)
		}
		if name == "export-tree-status-after.json" && !treeStatus.WipeImportArtifactsComplete {
			t.Fatalf("export-tree-status-after.json must report wipe_import_artifacts_complete=true, got %#v", treeStatus)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", "DROP TABLE", dsn, "password="} {
			if strings.Contains(string(statusRaw), forbidden) {
				t.Fatalf("%s must not contain %q, got %s", name, forbidden, string(statusRaw))
			}
		}
	}

	wantMarkers := map[string]string{
		"account-character-roster":     `"character_count": 1`,
		"character-item-state":         `"inventory_item_count": 1`,
		"character-point-state":        `"point_row_count": 255`,
		"character-myshop-unit-prices": `"price_row_count": 2`,
		"character-quest-state":        `"flag_count": 1`,
		"character-safebox-state":      `"password_count": 1`,
		"auth-login-ticket-handoff":    `"ticket_count": 1`,
		"item-template-state":          `"refine_info_count": 2`,
		"static-actor-content-state":   `"static_actor_count": 1`,
		"bootstrap-ground-item-state":  `"ground_item_count": 1`,
	}
	for kind, want := range wantMarkers {
		resultPath := filepath.Join(exportTree, kind, "import-result.json")
		body := mustReadFileString(t, resultPath)
		if !strings.Contains(body, want) && !strings.Contains(compactJSONForAssert(body), compactJSONForAssert(want)) {
			t.Fatalf("kind %s import-result missing %s, got %s", kind, want, body)
		}
		if !strings.Contains(body, `"replaced": true`) && !strings.Contains(compactJSONForAssert(body), `"replaced":true`) {
			t.Fatalf("kind %s import-result missing replaced:true, got %s", kind, body)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", dsn} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("import-result for %s must not contain %q, got %s", kind, forbidden, body)
			}
		}
	}
	assertImportExportDrillStatusArtifactsForKinds(t, exportTree, wantMarkers, dsn)
	assertImportExportDrillStatusReplacedForKinds(t, exportTree, wantMarkers, dsn)
	assertSeededImportExportDrillSQLiteRows(t, dsn)
}

func omitImportExportDrillKindLines(script, kind string) string {
	var kept []string
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "/"+kind+"/") || strings.Contains(line, "--kind "+kind+" ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func assertImportExportDrillStatusArtifactsForKinds(t *testing.T, exportTree string, wantMarkers map[string]string, dsn string) {
	t.Helper()
	for kind, want := range wantMarkers {
		resultPath := filepath.Join(exportTree, kind, "import-result.json")
		statusPath := filepath.Join(exportTree, kind, "import-result-status.json")
		resultRaw, err := os.ReadFile(resultPath)
		if err != nil {
			t.Fatalf("read import-result for %s: %v", kind, err)
		}
		statusBody := mustReadFileString(t, statusPath)
		var got importExportStatus
		if err := json.Unmarshal([]byte(statusBody), &got); err != nil {
			t.Fatalf("decode import-result-status for %s: %v\nbody:\n%s", kind, err, statusBody)
		}
		if got.Format != importExportStatusFormat || !got.Present || got.Kind != kind {
			t.Fatalf("unexpected import-result-status envelope for %s: %#v", kind, got)
		}
		wantSHA := sha256Hex(resultRaw)
		if got.ImportResultSHA256 != wantSHA {
			t.Fatalf("unexpected import_result_sha256 for %s: got %s want %s", kind, got.ImportResultSHA256, wantSHA)
		}
		resultBytes, err := json.Marshal(got.Result)
		if err != nil {
			t.Fatalf("marshal nested status result for %s: %v", kind, err)
		}
		resultJSON := string(resultBytes)
		if !strings.Contains(resultJSON, want) && !strings.Contains(compactJSONForAssert(resultJSON), compactJSONForAssert(want)) {
			t.Fatalf("kind %s import-result-status missing %s, got %s", kind, want, statusBody)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", "DROP TABLE", dsn, "password="} {
			if strings.Contains(statusBody, forbidden) {
				t.Fatalf("import-result-status for %s must not contain %q, got %s", kind, forbidden, statusBody)
			}
		}
	}
}

func assertImportExportDrillStatusReplaced(t *testing.T, exportTree string, dsn string) {
	t.Helper()
	kinds := make(map[string]string, len(exportQuarantineKinds))
	for _, kind := range exportQuarantineKinds {
		kinds[kind] = ""
	}
	assertImportExportDrillStatusReplacedForKinds(t, exportTree, kinds, dsn)
}

func assertImportExportDrillStatusReplacedForKinds(t *testing.T, exportTree string, kinds map[string]string, dsn string) {
	t.Helper()
	for kind := range kinds {
		statusPath := filepath.Join(exportTree, kind, "import-result-status.json")
		statusBody := mustReadFileString(t, statusPath)
		if !strings.Contains(statusBody, `"replaced": true`) && !strings.Contains(compactJSONForAssert(statusBody), `"replaced":true`) {
			t.Fatalf("kind %s import-result-status missing replaced:true, got %s", kind, statusBody)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", "DROP TABLE", dsn, "password="} {
			if strings.Contains(statusBody, forbidden) {
				t.Fatalf("import-result-status for %s must not contain %q, got %s", kind, forbidden, statusBody)
			}
		}
	}
}

func mustMaterializeEmptyImportExportQuarantineTree(t *testing.T, exportTree string) {
	t.Helper()
	payloads := map[string]string{
		"account-character-roster":     `{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`,
		"character-item-state":         `{"migration_version":3,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[]}`,
		"character-point-state":        `{"migration_version":11,"migration_name":"character_point_state","points":[]}`,
		"character-myshop-unit-prices": `{"migration_version":23,"migration_name":"character_myshop_unit_prices","unit_prices":[]}`,
		"character-quest-state":        `{"migration_version":4,"migration_name":"character_quest_state","flags":[]}`,
		"character-safebox-state":      `{"migration_version":15,"migration_name":"character_safebox_money","passwords":[],"items":[]}`,
		"auth-login-ticket-handoff":    `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","tickets":[]}`,
		"item-template-state":          `{"migration_version":9,"migration_name":"item_template_refine_info","templates":[],"sockets":[],"attributes":[],"use_effects":[],"equip_effects":[],"refine_infos":[],"refine_materials":[]}`,
		"static-actor-content-state":   `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definitions":[],"merchant_catalog_entries":[],"quest_flag_reward_items":[],"quest_flag_consume_items":[],"static_actors":[],"reward_drops":[],"combat_profiles":[],"combat_profile_death_reward_drops":[]}`,
		"bootstrap-ground-item-state":  `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_items":[]}`,
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

func assertImportExportDrillStatusArtifacts(t *testing.T, exportTree string, wantMarkers map[string]string, dsn string) {
	t.Helper()
	for _, kind := range exportQuarantineKinds {
		resultPath := filepath.Join(exportTree, kind, "import-result.json")
		statusPath := filepath.Join(exportTree, kind, "import-result-status.json")
		resultRaw, err := os.ReadFile(resultPath)
		if err != nil {
			t.Fatalf("read import-result for %s: %v", kind, err)
		}
		statusBody := mustReadFileString(t, statusPath)
		var got importExportStatus
		if err := json.Unmarshal([]byte(statusBody), &got); err != nil {
			t.Fatalf("decode import-result-status for %s: %v\nbody:\n%s", kind, err, statusBody)
		}
		if got.Format != importExportStatusFormat || !got.Present || got.Kind != kind {
			t.Fatalf("unexpected import-result-status envelope for %s: %#v", kind, got)
		}
		wantSHA := sha256Hex(resultRaw)
		if got.ImportResultSHA256 != wantSHA {
			t.Fatalf("unexpected import_result_sha256 for %s: got %s want %s", kind, got.ImportResultSHA256, wantSHA)
		}
		want, ok := wantMarkers[kind]
		if !ok {
			t.Fatalf("missing expected marker for kind %q", kind)
		}
		resultBytes, err := json.Marshal(got.Result)
		if err != nil {
			t.Fatalf("marshal nested status result for %s: %v", kind, err)
		}
		resultJSON := string(resultBytes)
		if !strings.Contains(resultJSON, want) && !strings.Contains(compactJSONForAssert(resultJSON), compactJSONForAssert(want)) {
			t.Fatalf("kind %s import-result-status missing %s, got %s", kind, want, statusBody)
		}
		for _, forbidden := range []string{"postgres://", "CREATE TABLE", "DROP TABLE", dsn, "password="} {
			if strings.Contains(statusBody, forbidden) {
				t.Fatalf("import-result-status for %s must not contain %q, got %s", kind, forbidden, statusBody)
			}
		}
	}
}

func compactJSONForAssert(raw string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		return strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), "\n", "")
	}
	return buf.String()
}

func mustMaterializeSeededImportExportQuarantineTree(t *testing.T, exportTree string) {
	t.Helper()

	const (
		characterID   = uint32(11)
		accountLogin  = "Alpha"
		characterName = "AlphaWar"
	)

	zeroSockets := inventory.SocketValues{}
	activeSockets := inventory.SocketValues{1, 0, 7}
	zeroAttributes := inventory.AttributeValues{}
	activeAttributes := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 4, Value: -5}}
	character := loginticket.Character{
		ID:       characterID,
		Name:     characterName,
		Level:    1,
		MapIndex: 1,
		Empire:   1,
		Inventory: []inventory.ItemInstance{
			{ID: 1001, Vnum: 27001, Count: 1, Slot: 5, Sockets: &zeroSockets, Attributes: &zeroAttributes},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon, Sockets: &activeSockets, Attributes: &activeAttributes},
		},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
		MyShopUnitPrices: []loginticket.MyShopUnitPrice{
			{Vnum: 27002, UnitPrice: 200},
			{Vnum: 27001, UnitPrice: 500},
		},
	}
	character.Points[0] = 12

	accounts := []accountstore.Account{{
		Login:  accountLogin,
		Empire: 1,
		Characters: []loginticket.Character{
			character,
		},
	}}

	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	itemExport, err := accountstore.ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	pointExport, err := accountstore.ExportCharacterPointState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterPointState: %v", err)
	}
	myShopExport, err := accountstore.ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	questExport, err := queststate.ExportCharacterQuestState(queststate.Snapshot{
		Flags: []queststate.Flag{{
			Character: characterName,
			QuestRef:  "quest:first_steps",
			Name:      "met_guide",
			Value:     1,
		}},
	}, map[string]uint32{characterName: characterID})
	if err != nil {
		t.Fatalf("ExportCharacterQuestState: %v", err)
	}
	safeboxExport, err := safeboxstore.ExportCharacterSafeboxState(safeboxstore.Snapshot{
		Characters: []safeboxstore.CharacterRow{{
			Login:       accountLogin,
			CharacterID: characterID,
			Password:    "secret",
			Money:       1500,
			Cells: []safeboxstore.Cell{{
				Cell:          0,
				ID:            3001,
				Vnum:          27001,
				Count:         2,
				HasSockets:    true,
				Socket0:       1,
				Socket1:       0,
				Socket2:       7,
				HasAttributes: true,
				Attributes:    &activeAttributes,
			}},
		}},
	})
	if err != nil {
		t.Fatalf("ExportCharacterSafeboxState: %v", err)
	}
	ticketExport, err := loginticket.ExportAuthLoginTicketHandoff([]loginticket.Ticket{{
		Login:    accountLogin,
		LoginKey: 0x01020304,
		Empire:   1,
		IssuedAt: time.Date(2026, 8, 12, 9, 30, 0, 123456789, time.UTC),
		Characters: []loginticket.Character{{
			ID:       characterID,
			Name:     characterName,
			Level:    1,
			MapIndex: 1,
		}},
	}})
	if err != nil {
		t.Fatalf("ExportAuthLoginTicketHandoff: %v", err)
	}
	templateExport, err := itemstore.ExportItemTemplateState(itemstore.Snapshot{
		Templates: []itemstore.Template{
			{
				Vnum:      27001,
				Name:      "Small Red Potion",
				Stackable: true,
				MaxCount:  200,
			},
			{
				Vnum:      11199,
				Name:      "Downgrade Blade",
				Stackable: false,
				MaxCount:  1,
			},
			{
				Vnum:       11200,
				Name:       "Wooden Sword",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				EquipSlot:  "weapon",
				RefineInfo: &itemstore.RefineInfo{
					ResultVnum:  11201,
					Cost:        2500,
					Probability: 75,
					KeepOnFail:  true,
					Materials:   []itemstore.RefineMaterial{{Vnum: 27001, Count: 2}},
				},
			},
			{
				Vnum:       11300,
				Name:       "Downgrade Source Blade",
				Stackable:  false,
				MaxCount:   1,
				Refineable: true,
				EquipSlot:  "weapon",
				RefineInfo: &itemstore.RefineInfo{
					ResultVnum:     11301,
					Cost:           1800,
					Probability:    60,
					FailResultVnum: 11199,
					Materials:      []itemstore.RefineMaterial{{Vnum: 27001, Count: 1}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportItemTemplateState: %v", err)
	}
	staticExport, err := staticstore.ExportStaticActorContentState(
		staticstore.Snapshot{
			StaticActors: []staticstore.StaticActor{{
				EntityID:        11,
				Name:            "Warehouse",
				MapIndex:        1,
				X:               100,
				Y:               200,
				RaceNum:         20010,
				InteractionKind: interactionstore.KindTalk,
				InteractionRef:  "npc:village_guard",
			}},
		},
		interactionstore.Snapshot{
			Definitions: []interactionstore.Definition{{
				Kind: interactionstore.KindTalk,
				Ref:  "npc:village_guard",
				Text: "Keep your blade sharp.",
			}},
		},
	)
	if err != nil {
		t.Fatalf("ExportStaticActorContentState: %v", err)
	}
	groundCount := uint16(1)
	groundExport, err := worldruntime.ExportBootstrapGroundItemState([]worldruntime.GroundItemSnapshot{{
		VID:              0x0700002c,
		Vnum:             3001,
		Count:            groundCount,
		OwnerLogin:       accountLogin,
		OwnerCharacterID: characterID,
		OwnerVID:         0x02040000 + characterID,
		OwnerName:        characterName,
		MapIndex:         1,
		X:                1100,
		Y:                2100,
		Z:                2,
		PickupRange:      450,
		HasSockets:       true,
		Socket0:          1,
		Socket1:          0,
		Socket2:          7,
		HasAttributes:    true,
		Attr0Type:        1,
		Attr0Value:       25,
		Attr1Type:        4,
		Attr1Value:       -5,
	}})
	if err != nil {
		t.Fatalf("ExportBootstrapGroundItemState: %v", err)
	}

	payloads := map[string]any{
		"account-character-roster":     rosterExport,
		"character-item-state":         itemExport,
		"character-point-state":        pointExport,
		"character-myshop-unit-prices": myShopExport,
		"character-quest-state":        questExport,
		"character-safebox-state":      safeboxExport,
		"auth-login-ticket-handoff":    ticketExport,
		"item-template-state":          templateExport,
		"static-actor-content-state":   staticExport,
		"bootstrap-ground-item-state":  groundExport,
	}
	for _, kind := range exportQuarantineKinds {
		payload, ok := payloads[kind]
		if !ok {
			t.Fatalf("missing seeded quarantine payload for kind %q", kind)
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s: %v", kind, err)
		}
		dir := filepath.Join(exportTree, kind)
		mustMkdir(t, dir)
		mustWriteFile(t, filepath.Join(dir, "quarantine.json"), append(raw, '\n'))
	}
}

func assertSeededImportExportDrillSQLiteRows(t *testing.T, dsn string) {
	t.Helper()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite for assertions: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM accounts`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM characters`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_inventory_items`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_equipment_items`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_quickslots`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_points`, 255)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`, 2)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_quest_flags`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_safebox_passwords`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM character_safebox_items`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM auth_login_tickets`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM item_templates`, 4)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM item_template_refine_infos`, 2)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM item_template_refine_materials`, 2)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM interaction_definitions`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM static_actors`, 1)
	mustCountExact(t, db, ctx, `SELECT COUNT(*) FROM bootstrap_ground_items`, 1)

	var (
		gotAccountID int64
		gotLogin     string
	)
	if err := db.QueryRowContext(ctx, `SELECT id, login FROM accounts`).Scan(&gotAccountID, &gotLogin); err != nil {
		t.Fatalf("select account: %v", err)
	}
	if gotAccountID != 776349473104011307 || gotLogin != "Alpha" {
		t.Fatalf("account row = id=%d login=%q, want id=776349473104011307 login=Alpha", gotAccountID, gotLogin)
	}

	var (
		gotCharacterID int64
		gotName        string
		gotMapIndex    int
	)
	if err := db.QueryRowContext(ctx, `SELECT id, name, map_index FROM characters`).Scan(&gotCharacterID, &gotName, &gotMapIndex); err != nil {
		t.Fatalf("select character: %v", err)
	}
	if gotCharacterID != 11 || gotName != "AlphaWar" || gotMapIndex != 1 {
		t.Fatalf("character row = id=%d name=%q map=%d, want 11/AlphaWar/1", gotCharacterID, gotName, gotMapIndex)
	}

	var (
		gotInvHasSockets int
		gotInvSocket0    int64
		gotInvSocket1    int64
		gotInvSocket2    int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_sockets, socket0, socket1, socket2
FROM character_inventory_items WHERE id = 1001`).Scan(
		&gotInvHasSockets, &gotInvSocket0, &gotInvSocket1, &gotInvSocket2,
	); err != nil {
		t.Fatalf("select inventory sockets: %v", err)
	}
	if gotInvHasSockets != 1 || gotInvSocket0 != 0 || gotInvSocket1 != 0 || gotInvSocket2 != 0 {
		t.Fatalf("inventory sockets = has_sockets=%d sockets=(%d,%d,%d), want 1/(0,0,0)",
			gotInvHasSockets, gotInvSocket0, gotInvSocket1, gotInvSocket2)
	}

	var (
		gotEquipHasSockets int
		gotEquipSocket0    int64
		gotEquipSocket1    int64
		gotEquipSocket2    int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_sockets, socket0, socket1, socket2
FROM character_equipment_items WHERE id = 2001`).Scan(
		&gotEquipHasSockets, &gotEquipSocket0, &gotEquipSocket1, &gotEquipSocket2,
	); err != nil {
		t.Fatalf("select equipment sockets: %v", err)
	}
	if gotEquipHasSockets != 1 || gotEquipSocket0 != 1 || gotEquipSocket1 != 0 || gotEquipSocket2 != 7 {
		t.Fatalf("equipment sockets = has_sockets=%d sockets=(%d,%d,%d), want 1/(1,0,7)",
			gotEquipHasSockets, gotEquipSocket0, gotEquipSocket1, gotEquipSocket2)
	}

	var (
		gotInvHasAttributes int
		gotInvAttr0Type     int
		gotInvAttr0Value    int
		gotInvAttr1Type     int
		gotInvAttr1Value    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_inventory_items WHERE id = 1001`).Scan(
		&gotInvHasAttributes, &gotInvAttr0Type, &gotInvAttr0Value, &gotInvAttr1Type, &gotInvAttr1Value,
	); err != nil {
		t.Fatalf("select inventory attributes: %v", err)
	}
	if gotInvHasAttributes != 1 || gotInvAttr0Type != 0 || gotInvAttr0Value != 0 || gotInvAttr1Type != 0 || gotInvAttr1Value != 0 {
		t.Fatalf("inventory attributes = has_attributes=%d attrs=(%d/%d,%d/%d), want 1/(0/0,0/0)",
			gotInvHasAttributes, gotInvAttr0Type, gotInvAttr0Value, gotInvAttr1Type, gotInvAttr1Value)
	}

	var (
		gotEquipHasAttributes int
		gotEquipAttr0Type     int
		gotEquipAttr0Value    int
		gotEquipAttr1Type     int
		gotEquipAttr1Value    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_equipment_items WHERE id = 2001`).Scan(
		&gotEquipHasAttributes, &gotEquipAttr0Type, &gotEquipAttr0Value, &gotEquipAttr1Type, &gotEquipAttr1Value,
	); err != nil {
		t.Fatalf("select equipment attributes: %v", err)
	}
	if gotEquipHasAttributes != 1 || gotEquipAttr0Type != 1 || gotEquipAttr0Value != 25 || gotEquipAttr1Type != 4 || gotEquipAttr1Value != -5 {
		t.Fatalf("equipment attributes = has_attributes=%d attrs=(%d/%d,%d/%d), want 1/(1/25,4/-5)",
			gotEquipHasAttributes, gotEquipAttr0Type, gotEquipAttr0Value, gotEquipAttr1Type, gotEquipAttr1Value)
	}

	var pointValue int64
	if err := db.QueryRowContext(ctx, `SELECT value FROM character_points WHERE character_id = 11 AND point_index = 0`).Scan(&pointValue); err != nil {
		t.Fatalf("select point 0: %v", err)
	}
	if pointValue != 12 {
		t.Fatalf("point 0 = %d, want 12", pointValue)
	}

	var (
		gotPriceVnum int64
		gotUnitPrice int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT vnum, unit_price FROM character_myshop_unit_prices
WHERE character_id = 11 ORDER BY vnum LIMIT 1`).Scan(&gotPriceVnum, &gotUnitPrice); err != nil {
		t.Fatalf("select myshop unit price: %v", err)
	}
	if gotPriceVnum != 27001 || gotUnitPrice != 500 {
		t.Fatalf("myshop unit price = vnum=%d unit_price=%d, want 27001/500", gotPriceVnum, gotUnitPrice)
	}

	var (
		gotQuestRef string
		gotFlagName string
		gotFlagVal  int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT quest_ref, flag_name, value
FROM character_quest_flags WHERE character_id = 11`).Scan(&gotQuestRef, &gotFlagName, &gotFlagVal); err != nil {
		t.Fatalf("select quest flag: %v", err)
	}
	if gotQuestRef != "quest:first_steps" || gotFlagName != "met_guide" || gotFlagVal != 1 {
		t.Fatalf("quest flag = %s/%s/%d, want quest:first_steps/met_guide/1", gotQuestRef, gotFlagName, gotFlagVal)
	}

	var (
		gotPassword string
		gotMoney    int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT password, money FROM character_safebox_passwords WHERE character_id = 11`).Scan(&gotPassword, &gotMoney); err != nil {
		t.Fatalf("select safebox password: %v", err)
	}
	if gotPassword != "secret" || gotMoney != 1500 {
		t.Fatalf("safebox password/money = %q/%d, want secret/1500", gotPassword, gotMoney)
	}

	var (
		gotSafeboxHasSockets int
		gotSafeboxSocket0    int64
		gotSafeboxSocket1    int64
		gotSafeboxSocket2    int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_sockets, socket0, socket1, socket2
FROM character_safebox_items WHERE id = 3001`).Scan(
		&gotSafeboxHasSockets, &gotSafeboxSocket0, &gotSafeboxSocket1, &gotSafeboxSocket2,
	); err != nil {
		t.Fatalf("select safebox item sockets: %v", err)
	}
	if gotSafeboxHasSockets != 1 || gotSafeboxSocket0 != 1 || gotSafeboxSocket1 != 0 || gotSafeboxSocket2 != 7 {
		t.Fatalf("safebox item sockets = has_sockets=%d sockets=(%d,%d,%d), want 1/(1,0,7)",
			gotSafeboxHasSockets, gotSafeboxSocket0, gotSafeboxSocket1, gotSafeboxSocket2)
	}

	var (
		gotSafeboxHasAttributes int
		gotSafeboxAttr0Type     int
		gotSafeboxAttr0Value    int
		gotSafeboxAttr1Type     int
		gotSafeboxAttr1Value    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_safebox_items WHERE id = 3001`).Scan(
		&gotSafeboxHasAttributes, &gotSafeboxAttr0Type, &gotSafeboxAttr0Value, &gotSafeboxAttr1Type, &gotSafeboxAttr1Value,
	); err != nil {
		t.Fatalf("select safebox item attributes: %v", err)
	}
	if gotSafeboxHasAttributes != 1 || gotSafeboxAttr0Type != 1 || gotSafeboxAttr0Value != 25 || gotSafeboxAttr1Type != 4 || gotSafeboxAttr1Value != -5 {
		t.Fatalf("safebox item attributes = has_attributes=%d attrs=(%d/%d,%d/%d), want 1/(1/25,4/-5)",
			gotSafeboxHasAttributes, gotSafeboxAttr0Type, gotSafeboxAttr0Value, gotSafeboxAttr1Type, gotSafeboxAttr1Value)
	}

	var (
		gotTemplateName string
		gotStackable    int
		gotMaxCount     int
	)
	if err := db.QueryRowContext(ctx, `
SELECT name, stackable, max_count FROM item_templates WHERE vnum = 27001`).Scan(&gotTemplateName, &gotStackable, &gotMaxCount); err != nil {
		t.Fatalf("select item template: %v", err)
	}
	if gotTemplateName != "Small Red Potion" || gotStackable != 1 || gotMaxCount != 200 {
		t.Fatalf("item template = %q/%d/%d, want Small Red Potion/1/200", gotTemplateName, gotStackable, gotMaxCount)
	}

	var (
		gotKeepResult      int64
		gotKeepCost        int
		gotKeepProbability int
		gotKeepOnFail      int
		gotKeepFailResult  int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT result_vnum, cost, probability, keep_on_fail, fail_result_vnum
FROM item_template_refine_infos WHERE vnum = 11200`).Scan(
		&gotKeepResult, &gotKeepCost, &gotKeepProbability, &gotKeepOnFail, &gotKeepFailResult,
	); err != nil {
		t.Fatalf("select keep_on_fail refine info: %v", err)
	}
	if gotKeepResult != 11201 || gotKeepCost != 2500 || gotKeepProbability != 75 || gotKeepOnFail != 1 || gotKeepFailResult != 0 {
		t.Fatalf("keep_on_fail refine = result=%d cost=%d prob=%d keep=%d fail=%d, want 11201/2500/75/1/0",
			gotKeepResult, gotKeepCost, gotKeepProbability, gotKeepOnFail, gotKeepFailResult)
	}

	var (
		gotFailResult      int64
		gotFailCost        int
		gotFailProbability int
		gotFailKeepOnFail  int
		gotFailResultVnum  int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT result_vnum, cost, probability, keep_on_fail, fail_result_vnum
FROM item_template_refine_infos WHERE vnum = 11300`).Scan(
		&gotFailResult, &gotFailCost, &gotFailProbability, &gotFailKeepOnFail, &gotFailResultVnum,
	); err != nil {
		t.Fatalf("select fail_result_vnum refine info: %v", err)
	}
	if gotFailResult != 11301 || gotFailCost != 1800 || gotFailProbability != 60 || gotFailKeepOnFail != 0 || gotFailResultVnum != 11199 {
		t.Fatalf("fail_result_vnum refine = result=%d cost=%d prob=%d keep=%d fail=%d, want 11301/1800/60/0/11199",
			gotFailResult, gotFailCost, gotFailProbability, gotFailKeepOnFail, gotFailResultVnum)
	}

	var (
		gotKeepMaterialVnum  int64
		gotKeepMaterialCount int
	)
	if err := db.QueryRowContext(ctx, `
SELECT item_vnum, count FROM item_template_refine_materials
WHERE vnum = 11200 AND position = 0`).Scan(&gotKeepMaterialVnum, &gotKeepMaterialCount); err != nil {
		t.Fatalf("select keep_on_fail refine material: %v", err)
	}
	if gotKeepMaterialVnum != 27001 || gotKeepMaterialCount != 2 {
		t.Fatalf("keep_on_fail material = vnum=%d count=%d, want 27001/2", gotKeepMaterialVnum, gotKeepMaterialCount)
	}

	var (
		gotInteractionKind string
		gotInteractionRef  string
		gotActorName       string
	)
	if err := db.QueryRowContext(ctx, `
SELECT kind, ref FROM interaction_definitions`).Scan(&gotInteractionKind, &gotInteractionRef); err != nil {
		t.Fatalf("select interaction definition: %v", err)
	}
	if gotInteractionKind != "talk" || gotInteractionRef != "npc:village_guard" {
		t.Fatalf("interaction = %s/%s, want talk/npc:village_guard", gotInteractionKind, gotInteractionRef)
	}
	if err := db.QueryRowContext(ctx, `SELECT name FROM static_actors WHERE entity_id = 11`).Scan(&gotActorName); err != nil {
		t.Fatalf("select static actor: %v", err)
	}
	if gotActorName != "Warehouse" {
		t.Fatalf("static actor name = %q, want Warehouse", gotActorName)
	}

	var (
		gotGroundVID           int64
		gotGroundVnum          int64
		gotItemCount           sql.NullInt64
		gotGoldAmount          sql.NullInt64
		gotOwnerID             int64
		gotGroundHasSockets    int
		gotGroundSocket0       int64
		gotGroundSocket1       int64
		gotGroundSocket2       int64
		gotGroundHasAttributes int
		gotGroundAttr0Type     int64
		gotGroundAttr0Value    int64
		gotGroundAttr1Type     int64
		gotGroundAttr1Value    int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT vid, vnum, item_count, gold_amount, owner_character_id,
       has_sockets, socket0, socket1, socket2,
       has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM bootstrap_ground_items`).Scan(
		&gotGroundVID, &gotGroundVnum, &gotItemCount, &gotGoldAmount, &gotOwnerID,
		&gotGroundHasSockets, &gotGroundSocket0, &gotGroundSocket1, &gotGroundSocket2,
		&gotGroundHasAttributes, &gotGroundAttr0Type, &gotGroundAttr0Value, &gotGroundAttr1Type, &gotGroundAttr1Value,
	); err != nil {
		t.Fatalf("select ground item: %v", err)
	}
	if gotGroundVID != 0x0700002c || gotGroundVnum != 3001 || !gotItemCount.Valid || gotItemCount.Int64 != 1 || gotGoldAmount.Valid || gotOwnerID != 11 {
		t.Fatalf("ground item = vid=%d vnum=%d count=%v gold=%v owner=%d", gotGroundVID, gotGroundVnum, gotItemCount, gotGoldAmount, gotOwnerID)
	}
	if gotGroundHasSockets != 1 || gotGroundSocket0 != 1 || gotGroundSocket1 != 0 || gotGroundSocket2 != 7 {
		t.Fatalf("ground item sockets = has_sockets=%d sockets=(%d,%d,%d), want 1/(1,0,7)",
			gotGroundHasSockets, gotGroundSocket0, gotGroundSocket1, gotGroundSocket2)
	}
	if gotGroundHasAttributes != 1 || gotGroundAttr0Type != 1 || gotGroundAttr0Value != 25 || gotGroundAttr1Type != 4 || gotGroundAttr1Value != -5 {
		t.Fatalf("ground item attributes = has_attributes=%d attrs=(%d/%d,%d/%d), want 1/(1/25,4/-5)",
			gotGroundHasAttributes, gotGroundAttr0Type, gotGroundAttr0Value, gotGroundAttr1Type, gotGroundAttr1Value)
	}
}

func mustCountExact(t *testing.T, db *sql.DB, ctx context.Context, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", query, got, want)
	}
}
