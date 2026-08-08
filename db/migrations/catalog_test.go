package migrations

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuiltInCatalogIsValid(t *testing.T) {
	catalog, err := Catalog()
	if err != nil {
		t.Fatalf("expected built-in migration catalog to validate: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("expected at least one built-in migration")
	}

	first := catalog[0]
	if first.Version != 1 || first.Name != "bootstrap_schema_migrations" {
		t.Fatalf("unexpected first migration: %#v", first)
	}
	if first.UpPath != "0001_bootstrap_schema_migrations.up.sql" {
		t.Fatalf("unexpected first up path: %q", first.UpPath)
	}
	if first.DownPath != "0001_bootstrap_schema_migrations.down.sql" {
		t.Fatalf("unexpected first down path: %q", first.DownPath)
	}
	if !strings.Contains(strings.ToLower(first.UpSQL), "create table") || !strings.Contains(first.UpSQL, "schema_migrations") {
		t.Fatalf("expected first migration to create schema_migrations ledger, got:\n%s", first.UpSQL)
	}

	for i, migration := range catalog {
		wantVersion := i + 1
		if migration.Version != wantVersion {
			t.Fatalf("catalog is not contiguous at index %d: got version %d want %d", i, migration.Version, wantVersion)
		}
		if strings.TrimSpace(migration.UpSQL) == "" || strings.TrimSpace(migration.DownSQL) == "" {
			t.Fatalf("migration %04d has empty SQL body", migration.Version)
		}
	}
}

func TestLoadCatalogRejectsInvalidStates(t *testing.T) {
	validUp := "-- go-metin2 migration: 0001 bootstrap_schema_migrations up\nCREATE TABLE schema_migrations (version INTEGER PRIMARY KEY);\n"
	validDown := "-- go-metin2 migration: 0001 bootstrap_schema_migrations down\nDROP TABLE schema_migrations;\n"

	cases := []struct {
		name string
		fsys fstest.MapFS
	}{
		{
			name: "malformed sql filename",
			fsys: fstest.MapFS{
				"1_bootstrap_schema_migrations.up.sql":      {Data: []byte(validUp)},
				"0001_bootstrap_schema_migrations.down.sql": {Data: []byte(validDown)},
			},
		},
		{
			name: "missing down pair",
			fsys: fstest.MapFS{
				"0001_bootstrap_schema_migrations.up.sql": {Data: []byte(validUp)},
			},
		},
		{
			name: "mismatched pair names",
			fsys: fstest.MapFS{
				"0001_bootstrap_schema_migrations.up.sql": {Data: []byte(validUp)},
				"0001_other_name.down.sql":                {Data: []byte(validDown)},
			},
		},
		{
			name: "version gap",
			fsys: fstest.MapFS{
				"0001_bootstrap_schema_migrations.up.sql":   {Data: []byte(validUp)},
				"0001_bootstrap_schema_migrations.down.sql": {Data: []byte(validDown)},
				"0003_future_table.up.sql":                  {Data: []byte("-- go-metin2 migration: 0003 future_table up\nCREATE TABLE future_table (id INTEGER);\n")},
				"0003_future_table.down.sql":                {Data: []byte("-- go-metin2 migration: 0003 future_table down\nDROP TABLE future_table;\n")},
			},
		},
		{
			name: "empty sql body",
			fsys: fstest.MapFS{
				"0001_bootstrap_schema_migrations.up.sql":   {Data: []byte("\n\t ")},
				"0001_bootstrap_schema_migrations.down.sql": {Data: []byte(validDown)},
			},
		},
		{
			name: "missing header",
			fsys: fstest.MapFS{
				"0001_bootstrap_schema_migrations.up.sql":   {Data: []byte("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY);\n")},
				"0001_bootstrap_schema_migrations.down.sql": {Data: []byte(validDown)},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadCatalog(tc.fsys)
			if !errors.Is(err, ErrInvalidCatalog) {
				t.Fatalf("expected ErrInvalidCatalog, got %v", err)
			}
		})
	}
}

func TestLoadCatalogReturnsDeterministicOrder(t *testing.T) {
	catalog, err := LoadCatalog(fstest.MapFS{
		"0002_accounts.up.sql":                      {Data: []byte("-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n")},
		"0001_bootstrap_schema_migrations.up.sql":   {Data: []byte("-- go-metin2 migration: 0001 bootstrap_schema_migrations up\nCREATE TABLE schema_migrations (version INTEGER PRIMARY KEY);\n")},
		"0002_accounts.down.sql":                    {Data: []byte("-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n")},
		"0001_bootstrap_schema_migrations.down.sql": {Data: []byte("-- go-metin2 migration: 0001 bootstrap_schema_migrations down\nDROP TABLE schema_migrations;\n")},
	})
	if err != nil {
		t.Fatalf("expected catalog to validate: %v", err)
	}
	if len(catalog) != 2 {
		t.Fatalf("unexpected catalog length: %d", len(catalog))
	}
	if catalog[0].Version != 1 || catalog[0].Name != "bootstrap_schema_migrations" {
		t.Fatalf("unexpected first migration: %#v", catalog[0])
	}
	if catalog[1].Version != 2 || catalog[1].Name != "accounts" {
		t.Fatalf("unexpected second migration: %#v", catalog[1])
	}
}
