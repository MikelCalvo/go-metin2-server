package migratecli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
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
