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
	if len(plan.Pending) < 8 {
		t.Fatalf("expected account/character roster, character item-state, character quest-state, item-template-state, safebox-reject, auth login-ticket handoff, and static actor content-state migrations in built-in pending plan, got %#v", plan.Pending)
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
