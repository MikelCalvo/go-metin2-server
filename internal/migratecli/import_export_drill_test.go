package migratecli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRunImportExportDrillPrintsConfirmationGatedImportCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/var/metin2/exports/20260827T120000Z-abcdef012345",
			"--driver", "sqlite3",
			"--i-confirm-print-sql-import-drill",
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
		`# confirmation-gated printer: does not execute import-export or open a database`,
		`docs/workflow/migration-apply-runbook.md`,
		`docs/plans/2026-08-27-cli-import-export.md`,
		`EXPORT_TREE='/var/metin2/exports/20260827T120000Z-abcdef012345'`,
		`DRIVER='sqlite3'`,
		`DSN_ENV='METIN2_IMPORT_DSN'`,
		`DSN="${METIN2_IMPORT_DSN:?METIN2_IMPORT_DSN must be set to the import target DSN}"`,
		`test -f "$EXPORT_TREE/account-character-roster/quarantine.json"`,
		`metin2-migrate import-export --kind account-character-roster --export "$EXPORT_TREE/account-character-roster/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/account-character-roster/import-result.json"`,
		`metin2-migrate import-export-status --kind account-character-roster --import-result "$EXPORT_TREE/account-character-roster/import-result.json" > "$EXPORT_TREE/account-character-roster/import-result-status.json"`,
		`test -f "$EXPORT_TREE/character-item-state/quarantine.json"`,
		`metin2-migrate import-export --kind character-item-state --export "$EXPORT_TREE/character-item-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/character-item-state/import-result.json"`,
		`metin2-migrate import-export-status --kind character-item-state --import-result "$EXPORT_TREE/character-item-state/import-result.json" > "$EXPORT_TREE/character-item-state/import-result-status.json"`,
		`test -f "$EXPORT_TREE/character-point-state/quarantine.json"`,
		`metin2-migrate import-export --kind character-point-state --export "$EXPORT_TREE/character-point-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/character-point-state/import-result.json"`,
		`metin2-migrate import-export-status --kind character-point-state --import-result "$EXPORT_TREE/character-point-state/import-result.json" > "$EXPORT_TREE/character-point-state/import-result-status.json"`,
		`test -f "$EXPORT_TREE/character-myshop-unit-prices/quarantine.json"`,
		`metin2-migrate import-export --kind character-myshop-unit-prices --export "$EXPORT_TREE/character-myshop-unit-prices/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/character-myshop-unit-prices/import-result.json"`,
		`metin2-migrate import-export-status --kind character-myshop-unit-prices --import-result "$EXPORT_TREE/character-myshop-unit-prices/import-result.json" > "$EXPORT_TREE/character-myshop-unit-prices/import-result-status.json"`,
		`test -f "$EXPORT_TREE/character-quest-state/quarantine.json"`,
		`metin2-migrate import-export --kind character-quest-state --export "$EXPORT_TREE/character-quest-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/character-quest-state/import-result.json"`,
		`metin2-migrate import-export-status --kind character-quest-state --import-result "$EXPORT_TREE/character-quest-state/import-result.json" > "$EXPORT_TREE/character-quest-state/import-result-status.json"`,
		`test -f "$EXPORT_TREE/character-safebox-state/quarantine.json"`,
		`metin2-migrate import-export --kind character-safebox-state --export "$EXPORT_TREE/character-safebox-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/character-safebox-state/import-result.json"`,
		`metin2-migrate import-export-status --kind character-safebox-state --import-result "$EXPORT_TREE/character-safebox-state/import-result.json" > "$EXPORT_TREE/character-safebox-state/import-result-status.json"`,
		`test -f "$EXPORT_TREE/auth-login-ticket-handoff/quarantine.json"`,
		`metin2-migrate import-export --kind auth-login-ticket-handoff --export "$EXPORT_TREE/auth-login-ticket-handoff/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/auth-login-ticket-handoff/import-result.json"`,
		`metin2-migrate import-export-status --kind auth-login-ticket-handoff --import-result "$EXPORT_TREE/auth-login-ticket-handoff/import-result.json" > "$EXPORT_TREE/auth-login-ticket-handoff/import-result-status.json"`,
		`test -f "$EXPORT_TREE/item-template-state/quarantine.json"`,
		`metin2-migrate import-export --kind item-template-state --export "$EXPORT_TREE/item-template-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/item-template-state/import-result.json"`,
		`metin2-migrate import-export-status --kind item-template-state --import-result "$EXPORT_TREE/item-template-state/import-result.json" > "$EXPORT_TREE/item-template-state/import-result-status.json"`,
		`test -f "$EXPORT_TREE/static-actor-content-state/quarantine.json"`,
		`metin2-migrate import-export --kind static-actor-content-state --export "$EXPORT_TREE/static-actor-content-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/static-actor-content-state/import-result.json"`,
		`metin2-migrate import-export-status --kind static-actor-content-state --import-result "$EXPORT_TREE/static-actor-content-state/import-result.json" > "$EXPORT_TREE/static-actor-content-state/import-result-status.json"`,
		`test -f "$EXPORT_TREE/bootstrap-ground-item-state/quarantine.json"`,
		`metin2-migrate import-export --kind bootstrap-ground-item-state --export "$EXPORT_TREE/bootstrap-ground-item-state/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import > "$EXPORT_TREE/bootstrap-ground-item-state/import-result.json"`,
		`metin2-migrate import-export-status --kind bootstrap-ground-item-state --import-result "$EXPORT_TREE/bootstrap-ground-item-state/import-result.json" > "$EXPORT_TREE/bootstrap-ground-item-state/import-result-status.json"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	for _, forbidden := range []string{
		"CREATE TABLE",
		"DROP TABLE",
		"postgres://",
		"memory://",
		"sqlite://",
		"--dsn 'sqlite3'",
		"--dsn sqlite",
		"password=",
		"--i-confirm-scoped-replace",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("import-export-drill must not expose %q, got %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "remains insert-only") {
		t.Fatalf("expected insert-only comment in default drill stdout:\n%s", body)
	}

	idxRoster := strings.Index(body, `import-export --kind account-character-roster`)
	idxItem := strings.Index(body, `import-export --kind character-item-state`)
	idxPoint := strings.Index(body, `import-export --kind character-point-state`)
	idxMyShop := strings.Index(body, `import-export --kind character-myshop-unit-prices`)
	idxQuest := strings.Index(body, `import-export --kind character-quest-state`)
	idxSafebox := strings.Index(body, `import-export --kind character-safebox-state`)
	idxTicket := strings.Index(body, `import-export --kind auth-login-ticket-handoff`)
	idxTemplate := strings.Index(body, `import-export --kind item-template-state`)
	idxStatic := strings.Index(body, `import-export --kind static-actor-content-state`)
	idxGround := strings.Index(body, `import-export --kind bootstrap-ground-item-state`)
	if !(idxRoster < idxItem && idxItem < idxPoint && idxPoint < idxMyShop && idxMyShop < idxQuest && idxQuest < idxSafebox && idxSafebox < idxTicket && idxTicket < idxTemplate && idxTemplate < idxStatic && idxStatic < idxGround) {
		t.Fatalf("expected tip-kind import ordering, got idxs roster=%d item=%d point=%d myshop=%d quest=%d safebox=%d ticket=%d template=%d static=%d ground=%d\n%s",
			idxRoster, idxItem, idxPoint, idxMyShop, idxQuest, idxSafebox, idxTicket, idxTemplate, idxStatic, idxGround, body)
	}
}

func TestRunImportExportDrillHonorsCustomDSNEnv(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/tmp/metin2-exports/tree",
			"--driver", "pgx",
			"--dsn-env", "LAB_SQL_IMPORT_DSN",
			"--i-confirm-print-sql-import-drill",
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
		`EXPORT_TREE='/tmp/metin2-exports/tree'`,
		`DRIVER='pgx'`,
		`DSN_ENV='LAB_SQL_IMPORT_DSN'`,
		`DSN="${LAB_SQL_IMPORT_DSN:?LAB_SQL_IMPORT_DSN must be set to the import target DSN}"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in stdout:\n%s", want, body)
		}
	}
	if strings.Contains(body, "METIN2_IMPORT_DSN") {
		t.Fatalf("custom dsn-env must replace the default env name, got %s", body)
	}
}

func TestRunImportExportDrillPrintsOptInScopedReplace(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/var/metin2/exports/20260903T120000Z-abcdef012345",
			"--driver", "sqlite3",
			"--i-confirm-print-sql-import-drill",
			"--i-confirm-print-scoped-replace",
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
	for _, kind := range importExportDrillScopedReplaceKinds {
		want := fmt.Sprintf(
			`metin2-migrate import-export --kind %s --export "$EXPORT_TREE/%s/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import --i-confirm-scoped-replace > "$EXPORT_TREE/%s/import-result.json"`,
			kind, kind, kind,
		)
		if !strings.Contains(body, want) {
			t.Fatalf("expected scoped-replace import line for %s:\nwant %q\nbody:\n%s", kind, want, body)
		}
	}
	if !strings.Contains(body, "opt-in scoped replace") {
		t.Fatalf("expected scoped-replace comment in opt-in drill stdout:\n%s", body)
	}
	if !strings.Contains(body, "FK-safe") {
		t.Fatalf("expected FK-safe ordering comment in opt-in drill stdout:\n%s", body)
	}
	if !strings.Contains(body, "Seeded full-tree single-pass") {
		t.Fatalf("expected seeded tip-0002 limitation comment in opt-in drill stdout:\n%s", body)
	}
	if strings.Contains(body, "remains insert-only") {
		t.Fatalf("opt-in drill must not claim insert-only default wording, got:\n%s", body)
	}

	idxItem := strings.Index(body, `import-export --kind character-item-state`)
	idxPoint := strings.Index(body, `import-export --kind character-point-state`)
	idxMyShop := strings.Index(body, `import-export --kind character-myshop-unit-prices`)
	idxQuest := strings.Index(body, `import-export --kind character-quest-state`)
	idxSafebox := strings.Index(body, `import-export --kind character-safebox-state`)
	idxGround := strings.Index(body, `import-export --kind bootstrap-ground-item-state`)
	idxTicket := strings.Index(body, `import-export --kind auth-login-ticket-handoff`)
	idxTemplate := strings.Index(body, `import-export --kind item-template-state`)
	idxStatic := strings.Index(body, `import-export --kind static-actor-content-state`)
	idxRoster := strings.Index(body, `import-export --kind account-character-roster`)
	if !(idxItem < idxPoint && idxPoint < idxMyShop && idxMyShop < idxQuest && idxQuest < idxSafebox && idxSafebox < idxGround && idxGround < idxTicket && idxTicket < idxTemplate && idxTemplate < idxStatic && idxStatic < idxRoster) {
		t.Fatalf("expected FK-safe scoped-replace tip-kind import ordering, got idxs item=%d point=%d myshop=%d quest=%d safebox=%d ground=%d ticket=%d template=%d static=%d roster=%d\n%s",
			idxItem, idxPoint, idxMyShop, idxQuest, idxSafebox, idxGround, idxTicket, idxTemplate, idxStatic, idxRoster, body)
	}
}

func TestRunImportExportDrillScopedReplaceAloneRequiresPrintConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/var/metin2/exports/tree",
			"--driver", "sqlite3",
			"--i-confirm-print-scoped-replace",
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout without print confirmation, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--i-confirm-print-sql-import-drill") {
		t.Fatalf("expected print confirmation error, got %q", stderr.String())
	}
}

func TestRunImportExportDrillPrintsTwoPhaseWipeRosterReimport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/var/metin2/exports/20260903T180000Z-abcdef012345",
			"--driver", "sqlite3",
			"--i-confirm-print-sql-import-drill",
			"--i-confirm-print-two-phase-wipe-roster-reimport",
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
		"two-phase wipe → roster → omit-roster reimport",
		"phase 1: synthesize wipe-quarantine.json artifacts",
		"phase 2: wipe character-FK tip kinds",
		"phase 3: scoped-replace tip-0002 account-character-roster",
		"phase 4: scoped-replace reimport non-roster tip kinds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in two-phase drill stdout:\n%s", want, body)
		}
	}
	for _, kind := range importExportDrillWipeKinds {
		wantSynthesize := fmt.Sprintf(
			`metin2-migrate synthesize-wipe-export --kind %s --export "$EXPORT_TREE/%s/quarantine.json" > "$EXPORT_TREE/%s/wipe-quarantine.json"`,
			kind, kind, kind,
		)
		wantWipeStatus := fmt.Sprintf(
			`metin2-migrate synthesize-wipe-export-status --kind %s --wipe-export "$EXPORT_TREE/%s/wipe-quarantine.json" > "$EXPORT_TREE/%s/wipe-quarantine-status.json"`,
			kind, kind, kind,
		)
		wantWipe := fmt.Sprintf(
			`metin2-migrate import-export --kind %s --export "$EXPORT_TREE/%s/wipe-quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import --i-confirm-scoped-replace > "$EXPORT_TREE/%s/wipe-import-result.json"`,
			kind, kind, kind,
		)
		if !strings.Contains(body, wantSynthesize) {
			t.Fatalf("expected synthesize line for %s:\nwant %q\nbody:\n%s", kind, wantSynthesize, body)
		}
		if !strings.Contains(body, wantWipeStatus) {
			t.Fatalf("expected wipe-export status line for %s:\nwant %q\nbody:\n%s", kind, wantWipeStatus, body)
		}
		if !strings.Contains(body, wantWipe) {
			t.Fatalf("expected wipe import line for %s:\nwant %q\nbody:\n%s", kind, wantWipe, body)
		}
	}
	wantRoster := `metin2-migrate import-export --kind account-character-roster --export "$EXPORT_TREE/account-character-roster/quarantine.json" --driver "$DRIVER" --dsn "$DSN" --i-confirm-sql-import --i-confirm-scoped-replace > "$EXPORT_TREE/account-character-roster/import-result.json"`
	if !strings.Contains(body, wantRoster) {
		t.Fatalf("expected roster replace line:\nwant %q\nbody:\n%s", wantRoster, body)
	}
	idxSynthesize := strings.Index(body, "phase 1: synthesize wipe-quarantine.json artifacts")
	idxWipe := strings.Index(body, "phase 2: wipe character-FK tip kinds")
	idxRosterPhase := strings.Index(body, "phase 3: scoped-replace tip-0002 account-character-roster")
	idxReimport := strings.Index(body, "phase 4: scoped-replace reimport non-roster tip kinds")
	idxRosterImport := strings.Index(body, `import-export --kind account-character-roster`)
	idxItemReimport := strings.LastIndex(body, `import-export --kind character-item-state --export "$EXPORT_TREE/character-item-state/quarantine.json"`)
	if !(idxSynthesize >= 0 && idxWipe > idxSynthesize && idxRosterPhase > idxWipe && idxReimport > idxRosterPhase) {
		t.Fatalf("expected phase ordering synthesize<%d wipe<%d roster<%d reimport<%d\n%s", idxSynthesize, idxWipe, idxRosterPhase, idxReimport, body)
	}
	if !(idxRosterImport >= 0 && idxItemReimport >= 0 && idxRosterImport < idxItemReimport) {
		t.Fatalf("expected roster replace before non-roster reimport, got roster=%d item=%d\n%s", idxRosterImport, idxItemReimport, body)
	}
	if strings.Contains(body, `import-export --kind account-character-roster --export "$EXPORT_TREE/account-character-roster/quarantine.json"`) {
		count := strings.Count(body, `import-export --kind account-character-roster --export "$EXPORT_TREE/account-character-roster/quarantine.json"`)
		if count != 1 {
			t.Fatalf("expected exactly one roster quarantine import line, got %d\n%s", count, body)
		}
	}
	for _, forbidden := range []string{
		"CREATE TABLE",
		"postgres://",
		"memory://",
		"--dsn 'sqlite3'",
		"--dsn sqlite",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("two-phase drill must not expose %q, got %s", forbidden, body)
		}
	}
}

func TestRunImportExportDrillTwoPhaseAloneRequiresPrintConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/var/metin2/exports/tree",
			"--driver", "sqlite3",
			"--i-confirm-print-two-phase-wipe-roster-reimport",
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout without print confirmation, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--i-confirm-print-sql-import-drill") {
		t.Fatalf("expected print confirmation error, got %q", stderr.String())
	}
}

func TestRunImportExportDrillRequiresConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "/var/metin2/exports/tree",
			"--driver", "sqlite3",
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
	if !strings.Contains(stderr.String(), "--i-confirm-print-sql-import-drill") {
		t.Fatalf("expected confirmation error, got %q", stderr.String())
	}
}

func TestRunImportExportDrillRejectsRelativeExportTree(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export-drill",
			"--export-tree", "exports/tree",
			"--driver", "sqlite3",
			"--i-confirm-print-sql-import-drill",
		},
		nil,
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on failure, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "export-tree must be an absolute path") {
		t.Fatalf("expected absolute path error, got %q", stderr.String())
	}
}

func TestRunImportExportDrillUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing-flags", args: []string{"import-export-drill"}, want: "--export-tree and --driver are required"},
		{name: "missing-driver", args: []string{"import-export-drill", "--export-tree", "/var/metin2/exports/tree"}, want: "--export-tree and --driver are required"},
		{name: "unexpected-arg", args: []string{"import-export-drill", "--export-tree", "/var/metin2/exports/tree", "--driver", "sqlite3", "--i-confirm-print-sql-import-drill", "extra"}, want: "unexpected import-export-drill argument"},
		{name: "unknown-flag", args: []string{"import-export-drill", "--nope"}, want: "import-export-drill usage:"},
		{name: "blank-dsn-env", args: []string{"import-export-drill", "--export-tree", "/var/metin2/exports/tree", "--driver", "sqlite3", "--dsn-env", "   ", "--i-confirm-print-sql-import-drill"}, want: "dsn-env"},
		{name: "unsafe-dsn-env-metachar", args: []string{"import-export-drill", "--export-tree", "/var/metin2/exports/tree", "--driver", "sqlite3", "--dsn-env", "FOO;rm", "--i-confirm-print-sql-import-drill"}, want: "shell-safe environment variable name"},
		{name: "unsafe-dsn-env-leading-digit", args: []string{"import-export-drill", "--export-tree", "/var/metin2/exports/tree", "--driver", "sqlite3", "--dsn-env", "1BAD", "--i-confirm-print-sql-import-drill"}, want: "shell-safe environment variable name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, nil, &stdout, &stderr)
			wantCode := 2
			switch tc.name {
			case "blank-dsn-env", "unsafe-dsn-env-metachar", "unsafe-dsn-env-leading-digit":
				wantCode = 1
			}
			if code != wantCode {
				t.Fatalf("expected exit %d, got %d stderr=%q", wantCode, code, stderr.String())
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

func TestRunRejectsUnknownCommandMentionsImportExportDrill(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"nope"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "import-export-drill") {
		t.Fatalf("expected usage to mention import-export-drill, got %q", stderr.String())
	}
}
