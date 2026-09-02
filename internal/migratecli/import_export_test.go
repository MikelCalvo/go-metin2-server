package migratecli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRunImportExportRejectsUsageErrorsWithoutOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing-flags",
			args: []string{"import-export"},
			want: "--kind, --export, --driver, and --dsn are required",
		},
		{
			name: "missing-confirm",
			args: []string{
				"import-export",
				"--kind", "account-character-roster",
				"--export", "-",
				"--driver", driverName,
				"--dsn", "memory://import-missing-confirm",
			},
			want: "--i-confirm-sql-import",
		},
		{
			name: "scoped-replace-wrong-kind",
			args: []string{
				"import-export",
				"--kind", "auth-login-ticket-handoff",
				"--export", "-",
				"--driver", driverName,
				"--dsn", "memory://import-replace-wrong-kind",
				"--i-confirm-sql-import",
				"--i-confirm-scoped-replace",
			},
			want: "--i-confirm-scoped-replace is only supported for kind account-character-roster, character-item-state, character-quest-state, character-point-state, character-safebox-state, character-myshop-unit-prices, bootstrap-ground-item-state, or item-template-state",
		},
		{
			name: "unsupported-kind",
			args: []string{
				"import-export",
				"--kind", "not-a-kind",
				"--export", "-",
				"--driver", driverName,
				"--dsn", "memory://import-bad-kind",
				"--i-confirm-sql-import",
			},
			want: "unsupported import-export kind",
		},
		{
			name: "unexpected-arg",
			args: []string{
				"import-export",
				"--kind", "account-character-roster",
				"--export", "-",
				"--driver", driverName,
				"--dsn", "memory://import-extra",
				"--i-confirm-sql-import",
				"extra",
			},
			want: "unexpected import-export argument",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tc.args, strings.NewReader(`{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`), &stdout, &stderr)
			if code != exitUsage {
				t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected no stdout on usage error, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("expected %q in stderr %q", tc.want, stderr.String())
			}
			if !strings.Contains(stderr.String(), "import-export usage:") {
				t.Fatalf("expected import-export usage guidance, got %q", stderr.String())
			}
			if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
				t.Fatalf("usage errors must not open the database, got events %#v", events)
			}
		})
	}
}

func TestRunImportExportRejectsInvalidExportBeforeOpeningDatabase(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "account-character-roster",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-invalid-export",
			"--i-confirm-sql-import",
		},
		strings.NewReader(`{"migration_version":99,"migration_name":"not-roster","accounts":[],"characters":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitError {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitError, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on invalid export, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "import-export:") {
		t.Fatalf("expected import-export error prefix, got %q", stderr.String())
	}
	if events := currentMigrateCLITestDriver(t).eventsSnapshot(); len(events) != 0 {
		t.Fatalf("invalid export must fail before sql.Open, got events %#v", events)
	}
}

func TestRunImportExportImportsEmptyExportsAgainstRegisteredDriver(t *testing.T) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}

	cases := []struct {
		kind    string
		payload string
		version int
		want    string
	}{
		{
			kind:    "account-character-roster",
			payload: `{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`,
			version: 2,
			want:    `"account_count": 0`,
		},
		{
			kind:    "character-item-state",
			payload: `{"migration_version":3,"migration_name":"character_item_state","inventory_items":[],"equipment_items":[],"quickslots":[]}`,
			version: 3,
			want:    `"inventory_item_count": 0`,
		},
		{
			kind:    "character-point-state",
			payload: `{"migration_version":11,"migration_name":"character_point_state","points":[]}`,
			version: 11,
			want:    `"point_row_count": 0`,
		},
		{
			kind:    "character-myshop-unit-prices",
			payload: `{"migration_version":23,"migration_name":"character_myshop_unit_prices","unit_prices":[]}`,
			version: 23,
			want:    `"price_row_count": 0`,
		},
		{
			kind:    "character-quest-state",
			payload: `{"migration_version":4,"migration_name":"character_quest_state","flags":[]}`,
			version: 4,
			want:    `"flag_count": 0`,
		},
		{
			kind:    "character-safebox-state",
			payload: `{"migration_version":15,"migration_name":"character_safebox_money","passwords":[],"items":[]}`,
			version: 15,
			want:    `"password_count": 0`,
		},
		{
			kind:    "auth-login-ticket-handoff",
			payload: `{"migration_version":7,"migration_name":"auth_login_ticket_handoff","tickets":[]}`,
			version: 7,
			want:    `"ticket_count": 0`,
		},
		{
			kind:    "item-template-state",
			payload: `{"migration_version":9,"migration_name":"item_template_refine_info","templates":[],"sockets":[],"attributes":[],"use_effects":[],"equip_effects":[],"refine_infos":[],"refine_materials":[]}`,
			version: 9,
			want:    `"template_count": 0`,
		},
		{
			kind:    "static-actor-content-state",
			payload: `{"migration_version":13,"migration_name":"static_actor_combat_profile_state","interaction_definitions":[],"merchant_catalog_entries":[],"quest_flag_reward_items":[],"quest_flag_consume_items":[],"static_actors":[],"reward_drops":[],"combat_profiles":[],"combat_profile_death_reward_drops":[]}`,
			version: 13,
			want:    `"static_actor_count": 0`,
		},
		{
			kind:    "bootstrap-ground-item-state",
			payload: `{"migration_version":10,"migration_name":"bootstrap_ground_item_state","ground_items":[]}`,
			version: 10,
			want:    `"ground_item_count": 0`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			driverName := registerMigrateCLITestSQLDriver(t)
			driver := currentMigrateCLITestDriver(t)
			// Static-actor SQL import keeps tip-0013 export identity but requires
			// additive 0016 chase_delay_ms, 0017 return_delay_ms, 0018
			// homeward_delay_ms, 0019 max_step, and 0020 reaction_delay_ms before INSERT.
			// Item-template SQL import keeps tip-0009 export identity but requires
			// additive 0021 keep_on_fail and 0022 fail_result_vnum before INSERT.
			// Character item-state SQL import keeps tip-0003 export identity but
			// requires additive 0024 instance sockets and 0027 instance attributes before INSERT.
			// Character safebox-state SQL import keeps tip-0015 export identity but
			// requires additive 0025 instance sockets and 0028 instance attributes before INSERT.
			// Bootstrap ground-item-state SQL import keeps tip-0010 export identity but
			// requires additive 0026 instance sockets and 0029 instance attributes before INSERT.
			ledger := []dbmigrations.LedgerEntry{ledgerEntry(tc.version)}
			if tc.kind == "static-actor-content-state" {
				ledger = []dbmigrations.LedgerEntry{
					ledgerEntry(staticstore.StaticActorContentStateMigrationVersion),
					ledgerEntry(staticstore.StaticActorCombatProfileChaseDelayMigrationVersion),
					ledgerEntry(staticstore.StaticActorCombatProfileReturnDelayMigrationVersion),
					ledgerEntry(staticstore.StaticActorCombatProfileHomewardDelayMigrationVersion),
					ledgerEntry(staticstore.StaticActorCombatProfileMaxStepMigrationVersion),
					ledgerEntry(staticstore.StaticActorCombatProfileReactionDelayMigrationVersion),
				}
			}
			if tc.kind == "item-template-state" {
				ledger = []dbmigrations.LedgerEntry{
					ledgerEntry(itemstore.ItemTemplateStateMigrationVersion),
					ledgerEntry(itemstore.ItemTemplateRefineKeepOnFailMigrationVersion),
					ledgerEntry(itemstore.ItemTemplateRefineFailResultVnumMigrationVersion),
				}
			}
			if tc.kind == "character-item-state" {
				ledger = []dbmigrations.LedgerEntry{
					ledgerEntry(accountstore.CharacterItemStateMigrationVersion),
					ledgerEntry(accountstore.CharacterItemInstanceSocketsMigrationVersion),
					ledgerEntry(accountstore.CharacterItemInstanceAttributesMigrationVersion),
				}
			}
			if tc.kind == "character-safebox-state" {
				ledger = []dbmigrations.LedgerEntry{
					ledgerEntry(safeboxstore.CharacterSafeboxStateMigrationVersion),
					ledgerEntry(safeboxstore.CharacterSafeboxItemInstanceSocketsMigrationVersion),
					ledgerEntry(safeboxstore.CharacterSafeboxItemInstanceAttributesMigrationVersion),
				}
			}
			if tc.kind == "bootstrap-ground-item-state" {
				ledger = []dbmigrations.LedgerEntry{
					ledgerEntry(worldruntime.BootstrapGroundItemStateMigrationVersion),
					ledgerEntry(worldruntime.BootstrapGroundItemInstanceSocketsMigrationVersion),
					ledgerEntry(worldruntime.BootstrapGroundItemInstanceAttributesMigrationVersion),
				}
			}
			driver.setLedger(ledger)

			dsn := "memory://import-export-" + tc.kind
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(
				[]string{
					"import-export",
					"--kind", tc.kind,
					"--export", "-",
					"--driver", driverName,
					"--dsn", dsn,
					"--i-confirm-sql-import",
				},
				strings.NewReader(tc.payload),
				&stdout,
				&stderr,
			)
			if code != exitOK {
				t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no stderr, got %q", stderr.String())
			}
			body := stdout.String()
			if !strings.Contains(body, tc.want) {
				t.Fatalf("expected %q in stdout %s", tc.want, body)
			}
			if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "INSERT INTO") || strings.Contains(body, dsn) || strings.Contains(body, "memory://") {
				t.Fatalf("import-export must not expose SQL or DSN text, got %s", body)
			}
			events := driver.eventsSnapshot()
			for _, want := range []string{
				"open:" + dsn,
				"begin",
				"query:SELECT version, name, up_sha256 FROM schema_migrations ORDER BY version ASC",
				"commit",
				"close",
			} {
				if !containsMigrateCLITestEventPrefix(events, want) {
					t.Fatalf("expected event prefix %q in events %#v", want, events)
				}
			}
		})
	}
}

func TestRunImportExportRedactsDSNFromRuntimeErrors(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	secretDSN := "memory://secret-password@db/import-export"
	currentMigrateCLITestDriver(t).setOpenHook(func() error {
		return fmt.Errorf("open %s refused", secretDSN)
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "account-character-roster",
			"--export", "-",
			"--driver", driverName,
			"--dsn", secretDSN,
			"--i-confirm-sql-import",
		},
		strings.NewReader(`{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitError {
		t.Fatalf("expected exit %d, got %d stderr=%q", exitError, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on runtime error, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), secretDSN) {
		t.Fatalf("expected DSN redaction, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "<redacted-dsn>") {
		t.Fatalf("expected redacted DSN marker, got %q", stderr.String())
	}
}

func TestRunHelpListsImportExport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, nil, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("expected help exit 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "import-export") {
		t.Fatalf("expected help to list import-export, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "import-export usage:") {
		t.Fatalf("expected import-export usage block, got %q", stdout.String())
	}
}

func TestRunImportExportDecodesRosterResultShape(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	var entry dbmigrations.LedgerEntry
	for _, migration := range catalog {
		if migration.Version == accountstore.AccountCharacterRosterMigrationVersion {
			entry = dbmigrations.LedgerEntry{
				Version:  migration.Version,
				Name:     migration.Name,
				UpSHA256: migration.UpSHA256,
			}
			break
		}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{entry})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "account-character-roster",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-roster-shape",
			"--i-confirm-sql-import",
		},
		strings.NewReader(`{"migration_version":2,"migration_name":"account_character_roster","accounts":[],"characters":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result accountstore.AccountCharacterRosterImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if result.MigrationVersion != 2 || result.MigrationName != "account_character_roster" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if result.AccountCount != 0 || result.CharacterCount != 0 {
		t.Fatalf("unexpected counts: %#v", result)
	}
}

func TestRunImportExportCharacterItemStateScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(accountstore.CharacterItemStateMigrationVersion),
		ledgerEntry(accountstore.CharacterItemInstanceSocketsMigrationVersion),
		ledgerEntry(accountstore.CharacterItemInstanceAttributesMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "character-item-state",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-item-state-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":3,"migration_name":"character_item_state","character_ids":[],"inventory_items":[],"equipment_items":[],"quickslots":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result accountstore.CharacterItemStateImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 3 || result.MigrationName != "character_item_state" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportCharacterSafeboxStateScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(safeboxstore.CharacterSafeboxStateMigrationVersion),
		ledgerEntry(safeboxstore.CharacterSafeboxItemInstanceSocketsMigrationVersion),
		ledgerEntry(safeboxstore.CharacterSafeboxItemInstanceAttributesMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "character-safebox-state",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-safebox-state-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":15,"migration_name":"character_safebox_money","character_ids":[],"passwords":[],"items":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result safeboxstore.CharacterSafeboxStateImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 15 || result.MigrationName != "character_safebox_money" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportCharacterQuestStateScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(queststate.CharacterQuestStateMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "character-quest-state",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-quest-state-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":4,"migration_name":"character_quest_state","character_ids":[],"flags":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result queststate.CharacterQuestStateImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 4 || result.MigrationName != "character_quest_state" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportCharacterPointStateScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(accountstore.CharacterPointStateMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "character-point-state",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-point-state-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":11,"migration_name":"character_point_state","character_ids":[],"points":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result accountstore.CharacterPointStateImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 11 || result.MigrationName != "character_point_state" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportCharacterMyShopUnitPricesScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(accountstore.CharacterMyShopUnitPricesMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "character-myshop-unit-prices",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-myshop-unit-prices-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":23,"migration_name":"character_myshop_unit_prices","character_ids":[],"unit_prices":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result accountstore.CharacterMyShopUnitPricesImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 23 || result.MigrationName != "character_myshop_unit_prices" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportAccountCharacterRosterScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(accountstore.AccountCharacterRosterMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "account-character-roster",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-roster-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":2,"migration_name":"account_character_roster","account_ids":[],"accounts":[],"characters":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result accountstore.AccountCharacterRosterImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 2 || result.MigrationName != "account_character_roster" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportBootstrapGroundItemStateScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(worldruntime.BootstrapGroundItemStateMigrationVersion),
		ledgerEntry(worldruntime.BootstrapGroundItemInstanceSocketsMigrationVersion),
		ledgerEntry(worldruntime.BootstrapGroundItemInstanceAttributesMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "bootstrap-ground-item-state",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-ground-item-state-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":10,"migration_name":"bootstrap_ground_item_state","vids":[],"ground_items":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result worldruntime.BootstrapGroundItemStateImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 10 || result.MigrationName != "bootstrap_ground_item_state" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}

func TestRunImportExportItemTemplateStateScopedReplaceSetsReplaced(t *testing.T) {
	driverName := registerMigrateCLITestSQLDriver(t)
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	ledgerEntry := func(version int) dbmigrations.LedgerEntry {
		t.Helper()
		for _, migration := range catalog {
			if migration.Version == version {
				return dbmigrations.LedgerEntry{
					Version:  migration.Version,
					Name:     migration.Name,
					UpSHA256: migration.UpSHA256,
				}
			}
		}
		t.Fatalf("catalog missing version %d", version)
		return dbmigrations.LedgerEntry{}
	}
	currentMigrateCLITestDriver(t).setLedger([]dbmigrations.LedgerEntry{
		ledgerEntry(itemstore.ItemTemplateStateMigrationVersion),
		ledgerEntry(itemstore.ItemTemplateRefineKeepOnFailMigrationVersion),
		ledgerEntry(itemstore.ItemTemplateRefineFailResultVnumMigrationVersion),
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		[]string{
			"import-export",
			"--kind", "item-template-state",
			"--export", "-",
			"--driver", driverName,
			"--dsn", "memory://import-item-template-state-replace",
			"--i-confirm-sql-import",
			"--i-confirm-scoped-replace",
		},
		strings.NewReader(`{"migration_version":9,"migration_name":"item_template_refine_info","vnums":[],"templates":[],"sockets":[],"attributes":[],"use_effects":[],"equip_effects":[],"refine_infos":[],"refine_materials":[]}`),
		&stdout,
		&stderr,
	)
	if code != exitOK {
		t.Fatalf("expected exit 0, got %d stderr=%q", code, stderr.String())
	}
	var result itemstore.ItemTemplateStateImportResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v\nbody:\n%s", err, stdout.String())
	}
	if !result.Replaced {
		t.Fatalf("expected replaced=true, got %#v", result)
	}
	if result.MigrationVersion != 9 || result.MigrationName != "item_template_refine_info" {
		t.Fatalf("unexpected migration identity: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"replaced": true`) {
		t.Fatalf("expected replaced field in stdout JSON, got %q", stdout.String())
	}
}
