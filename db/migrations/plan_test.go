package migrations

import (
	"encoding/json"
	"errors"
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
		t.Fatal("expected at least the bootstrap schema migration to be pending for an empty ledger")
	}
	if plan.Pending[0].Version != 1 || plan.Pending[0].Name != "bootstrap_schema_migrations" || plan.Pending[0].Direction != DirectionUp {
		t.Fatalf("unexpected first built-in pending step: %#v", plan.Pending[0])
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
