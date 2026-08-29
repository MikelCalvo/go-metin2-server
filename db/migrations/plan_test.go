package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPlanCatalogUpToLatestReturnsPendingStepsFromValidatedLedger(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
		testMigration{
			version:  3,
			name:     "characters",
			upPath:   "0003_characters.up.sql",
			downPath: "0003_characters.down.sql",
			upSQL:    "-- go-metin2 migration: 0003 characters up\nCREATE TABLE characters (id INTEGER PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0003 characters down\nDROP TABLE characters;\n",
		},
	)

	plan, err := PlanCatalogUpToLatest(catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)})
	if err != nil {
		t.Fatalf("expected dry-run plan to validate: %v", err)
	}

	if plan.CurrentVersion != 1 || plan.LatestVersion != 3 {
		t.Fatalf("unexpected plan versions: %#v", plan)
	}
	if plan.UpToDate {
		t.Fatalf("expected plan to have pending migrations: %#v", plan)
	}
	if len(plan.Pending) != 2 {
		t.Fatalf("expected two pending steps, got %#v", plan.Pending)
	}
	first := plan.Pending[0]
	if first.Version != 2 || first.Name != "accounts" || first.Direction != DirectionUp || first.Path != "0002_accounts.up.sql" {
		t.Fatalf("unexpected first pending step: %#v", first)
	}
	if first.SHA256 != catalog[1].UpSHA256 {
		t.Fatalf("unexpected first pending checksum: got %q want %q", first.SHA256, catalog[1].UpSHA256)
	}
}

func TestPlanCatalogToVersionReturnsRollbackStepsInReverseOrder(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
		testMigration{
			version:  3,
			name:     "characters",
			upPath:   "0003_characters.up.sql",
			downPath: "0003_characters.down.sql",
			upSQL:    "-- go-metin2 migration: 0003 characters up\nCREATE TABLE characters (id INTEGER PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0003 characters down\nDROP TABLE characters;\n",
		},
	)

	plan, err := PlanCatalogToVersion(catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 1),
		ledgerEntryFor(t, catalog, 2),
		ledgerEntryFor(t, catalog, 3),
	}, 1)
	if err != nil {
		t.Fatalf("expected rollback dry-run plan to validate: %v", err)
	}

	if plan.CurrentVersion != 3 || plan.LatestVersion != 3 || plan.UpToDate {
		t.Fatalf("unexpected rollback plan versions: %#v", plan)
	}
	if len(plan.Pending) != 2 {
		t.Fatalf("expected two rollback steps, got %#v", plan.Pending)
	}
	first := plan.Pending[0]
	if first.Version != 3 || first.Name != "characters" || first.Direction != DirectionDown || first.Path != "0003_characters.down.sql" || first.SHA256 != catalog[2].DownSHA256 {
		t.Fatalf("unexpected first rollback step: %#v", first)
	}
	second := plan.Pending[1]
	if second.Version != 2 || second.Name != "accounts" || second.Direction != DirectionDown || second.Path != "0002_accounts.down.sql" || second.SHA256 != catalog[1].DownSHA256 {
		t.Fatalf("unexpected second rollback step: %#v", second)
	}
}

func TestPlanCatalogToVersionSupportsZeroTargetRollback(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())

	plan, err := PlanCatalogToVersion(catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, 0)
	if err != nil {
		t.Fatalf("expected rollback-to-zero dry-run plan to validate: %v", err)
	}
	if plan.CurrentVersion != 1 || plan.LatestVersion != 1 || plan.UpToDate {
		t.Fatalf("unexpected rollback-to-zero plan: %#v", plan)
	}
	if len(plan.Pending) != 1 {
		t.Fatalf("expected one rollback-to-zero step, got %#v", plan.Pending)
	}
	step := plan.Pending[0]
	if step.Version != 1 || step.Name != "bootstrap_schema_migrations" || step.Direction != DirectionDown || step.Path != "0001_bootstrap_schema_migrations.down.sql" || step.SHA256 != catalog[0].DownSHA256 {
		t.Fatalf("unexpected rollback-to-zero step: %#v", step)
	}
}

func TestPlanCatalogToVersionDetectsTargetAlreadyApplied(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())

	plan, err := PlanCatalogToVersion(catalog, []LedgerEntry{ledgerEntryFor(t, catalog, 1)}, 1)
	if err != nil {
		t.Fatalf("expected target-equal-current plan to validate: %v", err)
	}
	if plan.CurrentVersion != 1 || plan.LatestVersion != 1 || !plan.UpToDate {
		t.Fatalf("unexpected up-to-date target plan: %#v", plan)
	}
	if len(plan.Pending) != 0 {
		t.Fatalf("expected no pending steps at target, got %#v", plan.Pending)
	}
}

func TestPlanCatalogToVersionRejectsTargetsOutsideCatalog(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())

	for _, target := range []int{-1, 2} {
		t.Run(fmt.Sprintf("target_%d", target), func(t *testing.T) {
			_, err := PlanCatalogToVersion(catalog, nil, target)
			if !errors.Is(err, ErrInvalidMigrationTarget) {
				t.Fatalf("expected ErrInvalidMigrationTarget for target %d, got %v", target, err)
			}
		})
	}
}

func TestPlanCatalogUpToLatestAcceptsUnorderedLedgerAndDetectsUpToDate(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)

	plan, err := PlanCatalogUpToLatest(catalog, []LedgerEntry{
		ledgerEntryFor(t, catalog, 2),
		ledgerEntryFor(t, catalog, 1),
	})
	if err != nil {
		t.Fatalf("expected unordered ledger to validate: %v", err)
	}
	if plan.CurrentVersion != 2 || plan.LatestVersion != 2 {
		t.Fatalf("unexpected plan versions: %#v", plan)
	}
	if !plan.UpToDate {
		t.Fatalf("expected plan to be up to date: %#v", plan)
	}
	if len(plan.Pending) != 0 {
		t.Fatalf("expected no pending migrations, got %#v", plan.Pending)
	}
}

func TestPlanUpToLatestUsesBuiltInCatalog(t *testing.T) {
	plan, err := PlanUpToLatest(nil)
	if err != nil {
		t.Fatalf("expected built-in dry-run plan to validate: %v", err)
	}
	if plan.CurrentVersion != 0 || plan.LatestVersion < 1 {
		t.Fatalf("unexpected built-in plan versions: %#v", plan)
	}
	if len(plan.Pending) == 0 {
		t.Fatal("expected built-in catalog migrations to be pending for an empty ledger")
	}
	if plan.Pending[0].Version != 1 || plan.Pending[0].Name != "bootstrap_schema_migrations" || plan.Pending[0].Direction != DirectionUp {
		t.Fatalf("unexpected first built-in pending step: %#v", plan.Pending[0])
	}
	if len(plan.Pending) < 14 {
		t.Fatalf("expected account/character roster, character item-state, character quest-state, item-template-state, safebox-reject, auth login-ticket handoff, static actor content-state, item-template refine-info, bootstrap ground-item state, character point-state, static-actor PvE interaction-state, static-actor combat-profile-state, and character safebox-state migrations in built-in pending plan, got %#v", plan.Pending)
	}
	if plan.Pending[1].Version != 2 || plan.Pending[1].Name != "account_character_roster" || plan.Pending[1].Direction != DirectionUp || plan.Pending[1].Path != "0002_account_character_roster.up.sql" {
		t.Fatalf("unexpected second built-in pending step: %#v", plan.Pending[1])
	}
	if plan.Pending[2].Version != 3 || plan.Pending[2].Name != "character_item_state" || plan.Pending[2].Direction != DirectionUp || plan.Pending[2].Path != "0003_character_item_state.up.sql" {
		t.Fatalf("unexpected third built-in pending step: %#v", plan.Pending[2])
	}
	if plan.Pending[3].Version != 4 || plan.Pending[3].Name != "character_quest_state" || plan.Pending[3].Direction != DirectionUp || plan.Pending[3].Path != "0004_character_quest_state.up.sql" {
		t.Fatalf("unexpected fourth built-in pending step: %#v", plan.Pending[3])
	}
	if plan.Pending[4].Version != 5 || plan.Pending[4].Name != "item_template_state" || plan.Pending[4].Direction != DirectionUp || plan.Pending[4].Path != "0005_item_template_state.up.sql" {
		t.Fatalf("unexpected fifth built-in pending step: %#v", plan.Pending[4])
	}
	if plan.Pending[5].Version != 6 || plan.Pending[5].Name != "item_template_safebox_reject_message" || plan.Pending[5].Direction != DirectionUp || plan.Pending[5].Path != "0006_item_template_safebox_reject_message.up.sql" {
		t.Fatalf("unexpected sixth built-in pending step: %#v", plan.Pending[5])
	}
	if plan.Pending[6].Version != 7 || plan.Pending[6].Name != "auth_login_ticket_handoff" || plan.Pending[6].Direction != DirectionUp || plan.Pending[6].Path != "0007_auth_login_ticket_handoff.up.sql" {
		t.Fatalf("unexpected seventh built-in pending step: %#v", plan.Pending[6])
	}
	if plan.Pending[7].Version != 8 || plan.Pending[7].Name != "static_actor_content_state" || plan.Pending[7].Direction != DirectionUp || plan.Pending[7].Path != "0008_static_actor_content_state.up.sql" {
		t.Fatalf("unexpected eighth built-in pending step: %#v", plan.Pending[7])
	}
	if plan.Pending[8].Version != 9 || plan.Pending[8].Name != "item_template_refine_info" || plan.Pending[8].Direction != DirectionUp || plan.Pending[8].Path != "0009_item_template_refine_info.up.sql" {
		t.Fatalf("unexpected ninth built-in pending step: %#v", plan.Pending[8])
	}
	if plan.Pending[9].Version != 10 || plan.Pending[9].Name != "bootstrap_ground_item_state" || plan.Pending[9].Direction != DirectionUp || plan.Pending[9].Path != "0010_bootstrap_ground_item_state.up.sql" {
		t.Fatalf("unexpected tenth built-in pending step: %#v", plan.Pending[9])
	}
	if plan.Pending[10].Version != 11 || plan.Pending[10].Name != "character_point_state" || plan.Pending[10].Direction != DirectionUp || plan.Pending[10].Path != "0011_character_point_state.up.sql" {
		t.Fatalf("unexpected eleventh built-in pending step: %#v", plan.Pending[10])
	}
	if plan.Pending[11].Version != 12 || plan.Pending[11].Name != "static_actor_pve_interaction_state" || plan.Pending[11].Direction != DirectionUp || plan.Pending[11].Path != "0012_static_actor_pve_interaction_state.up.sql" {
		t.Fatalf("unexpected twelfth built-in pending step: %#v", plan.Pending[11])
	}
	if plan.Pending[12].Version != 13 || plan.Pending[12].Name != "static_actor_combat_profile_state" || plan.Pending[12].Direction != DirectionUp || plan.Pending[12].Path != "0013_static_actor_combat_profile_state.up.sql" {
		t.Fatalf("unexpected thirteenth built-in pending step: %#v", plan.Pending[12])
	}
	if plan.Pending[13].Version != 14 || plan.Pending[13].Name != "character_safebox_state" || plan.Pending[13].Direction != DirectionUp || plan.Pending[13].Path != "0014_character_safebox_state.up.sql" {
		t.Fatalf("unexpected fourteenth built-in pending step: %#v", plan.Pending[13])
	}
	if plan.Pending[14].Version != 15 || plan.Pending[14].Name != "character_safebox_money" || plan.Pending[14].Direction != DirectionUp || plan.Pending[14].Path != "0015_character_safebox_money.up.sql" {
		t.Fatalf("unexpected fifteenth built-in pending step: %#v", plan.Pending[14])
	}
	if plan.Pending[15].Version != 16 || plan.Pending[15].Name != "static_actor_combat_profile_chase_delay" || plan.Pending[15].Direction != DirectionUp || plan.Pending[15].Path != "0016_static_actor_combat_profile_chase_delay.up.sql" {
		t.Fatalf("unexpected sixteenth built-in pending step: %#v", plan.Pending[15])
	}
	if plan.Pending[16].Version != 17 || plan.Pending[16].Name != "static_actor_combat_profile_return_delay" || plan.Pending[16].Direction != DirectionUp || plan.Pending[16].Path != "0017_static_actor_combat_profile_return_delay.up.sql" {
		t.Fatalf("unexpected seventeenth built-in pending step: %#v", plan.Pending[16])
	}
	if plan.Pending[17].Version != 18 || plan.Pending[17].Name != "static_actor_combat_profile_homeward_delay" || plan.Pending[17].Direction != DirectionUp || plan.Pending[17].Path != "0018_static_actor_combat_profile_homeward_delay.up.sql" {
		t.Fatalf("unexpected eighteenth built-in pending step: %#v", plan.Pending[17])
	}
	if plan.Pending[18].Version != 19 || plan.Pending[18].Name != "static_actor_combat_profile_max_step" || plan.Pending[18].Direction != DirectionUp || plan.Pending[18].Path != "0019_static_actor_combat_profile_max_step.up.sql" {
		t.Fatalf("unexpected nineteenth built-in pending step: %#v", plan.Pending[18])
	}
	if plan.Pending[19].Version != 20 || plan.Pending[19].Name != "static_actor_combat_profile_reaction_delay" || plan.Pending[19].Direction != DirectionUp || plan.Pending[19].Path != "0020_static_actor_combat_profile_reaction_delay.up.sql" {
		t.Fatalf("unexpected twentieth built-in pending step: %#v", plan.Pending[19])
	}
	if plan.Pending[20].Version != 21 || plan.Pending[20].Name != "item_template_refine_keep_on_fail" || plan.Pending[20].Direction != DirectionUp || plan.Pending[20].Path != "0021_item_template_refine_keep_on_fail.up.sql" {
		t.Fatalf("unexpected twenty-first built-in pending step: %#v", plan.Pending[20])
	}
	if plan.Pending[21].Version != 22 || plan.Pending[21].Name != "item_template_refine_fail_result_vnum" || plan.Pending[21].Direction != DirectionUp || plan.Pending[21].Path != "0022_item_template_refine_fail_result_vnum.up.sql" {
		t.Fatalf("unexpected twenty-second built-in pending step: %#v", plan.Pending[21])
	}
}

func TestPlanJSONShapeIsStableForFuturePreflightOutput(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	plan, err := PlanCatalogUpToLatest(catalog, nil)
	if err != nil {
		t.Fatalf("expected dry-run plan to validate: %v", err)
	}

	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}

	body := string(raw)
	for _, want := range []string{`"current_version":0`, `"latest_version":1`, `"up_to_date":false`, `"pending":[`, `"direction":"up"`, `"path":"0001_bootstrap_schema_migrations.up.sql"`, `"sha256":"` + catalog[0].UpSHA256 + `"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected JSON plan to contain %s, got %s", want, body)
		}
	}
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "UpSQL") {
		t.Fatalf("expected JSON plan to omit executable SQL, got %s", body)
	}
}

func TestPlanCatalogUpToLatestRejectsMutatedCatalog(t *testing.T) {
	catalog := testCatalog(t, bootstrapSchemaMigration())
	catalog[0].UpSQL += "\n-- accidental historical edit\n"

	_, err := PlanCatalogUpToLatest(catalog, nil)
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("expected ErrInvalidCatalog, got %v", err)
	}
}

func TestPlanCatalogUpToLatestRejectsInvalidLedgers(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)

	validFirst := ledgerEntryFor(t, catalog, 1)
	validSecond := ledgerEntryFor(t, catalog, 2)

	cases := []struct {
		name   string
		ledger []LedgerEntry
	}{
		{
			name: "duplicate version",
			ledger: []LedgerEntry{
				validFirst,
				validFirst,
			},
		},
		{
			name: "gap",
			ledger: []LedgerEntry{
				validSecond,
			},
		},
		{
			name: "unknown future version",
			ledger: []LedgerEntry{
				validFirst,
				validSecond,
				{Version: 3, Name: "future", UpSHA256: strings.Repeat("0", 64)},
			},
		},
		{
			name: "name drift",
			ledger: []LedgerEntry{
				{Version: validFirst.Version, Name: "renamed", UpSHA256: validFirst.UpSHA256},
			},
		},
		{
			name: "checksum drift",
			ledger: []LedgerEntry{
				{Version: validFirst.Version, Name: validFirst.Name, UpSHA256: strings.Repeat("f", 64)},
			},
		},
		{
			name: "zero version",
			ledger: []LedgerEntry{
				{Version: 0, Name: validFirst.Name, UpSHA256: validFirst.UpSHA256},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PlanCatalogUpToLatest(catalog, tc.ledger)
			if !errors.Is(err, ErrInvalidLedger) {
				t.Fatalf("expected ErrInvalidLedger, got %v", err)
			}
		})
	}
}

func testCatalog(t *testing.T, migrations ...testMigration) []Migration {
	t.Helper()
	catalog, err := LoadCatalog(mapFSFor(migrations...))
	if err != nil {
		t.Fatalf("build test migration catalog: %v", err)
	}
	return catalog
}

func ledgerEntryFor(t *testing.T, catalog []Migration, version int) LedgerEntry {
	t.Helper()
	if version <= 0 || version > len(catalog) {
		t.Fatalf("test requested invalid catalog version %d", version)
	}
	migration := catalog[version-1]
	return LedgerEntry{
		Version:  migration.Version,
		Name:     migration.Name,
		UpSHA256: migration.UpSHA256,
	}
}
