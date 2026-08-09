package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	testManifestFilename        = "migrations.manifest.json"
	expectedBootstrapUpSHA256   = "76ab086217590515cb9b1eb72d78f49abf766da977998c4c60b41825c8e92f78"
	expectedBootstrapDownSHA256 = "140e8ba3c7a1c89cd942c13ef40160c74df5619093fe8c287c69cb978dba822d"
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
	if first.UpSHA256 != expectedBootstrapUpSHA256 {
		t.Fatalf("unexpected first up checksum: got %q want %q", first.UpSHA256, expectedBootstrapUpSHA256)
	}
	if first.DownSHA256 != expectedBootstrapDownSHA256 {
		t.Fatalf("unexpected first down checksum: got %q want %q", first.DownSHA256, expectedBootstrapDownSHA256)
	}
	if !strings.Contains(strings.ToLower(first.UpSQL), "create table") || !strings.Contains(first.UpSQL, "schema_migrations") {
		t.Fatalf("expected first migration to create schema_migrations ledger, got:\n%s", first.UpSQL)
	}
	if !strings.Contains(first.UpSQL, "up_sha256") {
		t.Fatalf("expected schema_migrations ledger to pin applied up checksums, got:\n%s", first.UpSQL)
	}

	for i, migration := range catalog {
		wantVersion := i + 1
		if migration.Version != wantVersion {
			t.Fatalf("catalog is not contiguous at index %d: got version %d want %d", i, migration.Version, wantVersion)
		}
		if strings.TrimSpace(migration.UpSQL) == "" || strings.TrimSpace(migration.DownSQL) == "" {
			t.Fatalf("migration %04d has empty SQL body", migration.Version)
		}
		if migration.UpSHA256 != testSHA256Hex(migration.UpSQL) {
			t.Fatalf("migration %04d has stale up checksum", migration.Version)
		}
		if migration.DownSHA256 != testSHA256Hex(migration.DownSQL) {
			t.Fatalf("migration %04d has stale down checksum", migration.Version)
		}
	}
}

func TestLoadCatalogRejectsInvalidStates(t *testing.T) {
	base := bootstrapSchemaMigration()
	future := testMigration{
		version:  3,
		name:     "future_table",
		upPath:   "0003_future_table.up.sql",
		downPath: "0003_future_table.down.sql",
		upSQL:    "-- go-metin2 migration: 0003 future_table up\nCREATE TABLE future_table (id INTEGER);\n",
		downSQL:  "-- go-metin2 migration: 0003 future_table down\nDROP TABLE future_table;\n",
	}

	cases := []struct {
		name string
		fsys fstest.MapFS
	}{
		{
			name: "missing manifest",
			fsys: fstest.MapFS{
				base.upPath:   {Data: []byte(base.upSQL)},
				base.downPath: {Data: []byte(base.downSQL)},
			},
		},
		{
			name: "malformed manifest json",
			fsys: withManifestData(mapFSFor(base), []byte("{not-json")),
		},
		{
			name: "manifest unknown field",
			fsys: withManifestData(mapFSFor(base), []byte(`{"format":"go-metin2-migration-manifest-v1","migrations":[],"extra":true}`)),
		},
		{
			name: "manifest trailing json",
			fsys: withManifestData(mapFSFor(base), []byte(strings.TrimSpace(manifestFor(base))+"\n{}\n")),
		},
		{
			name: "manifest checksum mismatch",
			fsys: withManifestData(mapFSFor(base), []byte(strings.Replace(manifestFor(base), testSHA256Hex(base.upSQL), strings.Repeat("0", 64), 1))),
		},
		{
			name: "manifest missing catalog entry",
			fsys: withManifestData(mapFSFor(base), []byte(manifestFor())),
		},
		{
			name: "malformed sql filename",
			fsys: withExtraFile(mapFSFor(base), "1_bootstrap_schema_migrations.up.sql", base.upSQL),
		},
		{
			name: "missing down pair",
			fsys: withDeletedFile(mapFSFor(base), base.downPath),
		},
		{
			name: "mismatched pair names",
			fsys: mismatchedPairFS(),
		},
		{
			name: "version gap",
			fsys: mapFSFor(base, future),
		},
		{
			name: "empty sql body",
			fsys: mapFSFor(testMigration{
				version:  1,
				name:     base.name,
				upPath:   base.upPath,
				downPath: base.downPath,
				upSQL:    "\n\t ",
				downSQL:  base.downSQL,
			}),
		},
		{
			name: "missing header",
			fsys: mapFSFor(testMigration{
				version:  1,
				name:     base.name,
				upPath:   base.upPath,
				downPath: base.downPath,
				upSQL:    "CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY);\n",
				downSQL:  base.downSQL,
			}),
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
	catalog, err := LoadCatalog(mapFSFor(
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	))
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
	if catalog[1].UpSHA256 != testSHA256Hex(catalog[1].UpSQL) {
		t.Fatalf("unexpected second migration up checksum: %q", catalog[1].UpSHA256)
	}
}

type testMigration struct {
	version  int
	name     string
	upPath   string
	downPath string
	upSQL    string
	downSQL  string
}

func bootstrapSchemaMigration() testMigration {
	return testMigration{
		version:  1,
		name:     "bootstrap_schema_migrations",
		upPath:   "0001_bootstrap_schema_migrations.up.sql",
		downPath: "0001_bootstrap_schema_migrations.down.sql",
		upSQL: "-- go-metin2 migration: 0001 bootstrap_schema_migrations up\n" +
			"CREATE TABLE schema_migrations (\n" +
			"    version INTEGER PRIMARY KEY,\n" +
			"    name TEXT NOT NULL,\n" +
			"    up_sha256 TEXT NOT NULL,\n" +
			"    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP\n" +
			");\n",
		downSQL: "-- go-metin2 migration: 0001 bootstrap_schema_migrations down\nDROP TABLE schema_migrations;\n",
	}
}

func mapFSFor(migrations ...testMigration) fstest.MapFS {
	fsys := make(fstest.MapFS, len(migrations)*2+1)
	for _, migration := range migrations {
		fsys[migration.upPath] = &fstest.MapFile{Data: []byte(migration.upSQL)}
		fsys[migration.downPath] = &fstest.MapFile{Data: []byte(migration.downSQL)}
	}
	fsys[testManifestFilename] = &fstest.MapFile{Data: []byte(manifestFor(migrations...))}
	return fsys
}

func manifestFor(migrations ...testMigration) string {
	type entry struct {
		Version    int    `json:"version"`
		Name       string `json:"name"`
		UpPath     string `json:"up_path"`
		DownPath   string `json:"down_path"`
		UpSHA256   string `json:"up_sha256"`
		DownSHA256 string `json:"down_sha256"`
	}
	payload := struct {
		Format     string  `json:"format"`
		Migrations []entry `json:"migrations"`
	}{
		Format: "go-metin2-migration-manifest-v1",
	}
	for _, migration := range migrations {
		payload.Migrations = append(payload.Migrations, entry{
			Version:    migration.version,
			Name:       migration.name,
			UpPath:     migration.upPath,
			DownPath:   migration.downPath,
			UpSHA256:   testSHA256Hex(migration.upSQL),
			DownSHA256: testSHA256Hex(migration.downSQL),
		})
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal test migration manifest: %v", err))
	}
	return string(raw) + "\n"
}

func withManifestData(fsys fstest.MapFS, data []byte) fstest.MapFS {
	fsys[testManifestFilename] = &fstest.MapFile{Data: data}
	return fsys
}

func withExtraFile(fsys fstest.MapFS, path, body string) fstest.MapFS {
	fsys[path] = &fstest.MapFile{Data: []byte(body)}
	return fsys
}

func withDeletedFile(fsys fstest.MapFS, path string) fstest.MapFS {
	delete(fsys, path)
	return fsys
}

func mismatchedPairFS() fstest.MapFS {
	up := "-- go-metin2 migration: 0001 bootstrap_schema_migrations up\nCREATE TABLE schema_migrations (version INTEGER PRIMARY KEY);\n"
	down := "-- go-metin2 migration: 0001 other_name down\nDROP TABLE schema_migrations;\n"
	migration := testMigration{
		version:  1,
		name:     "bootstrap_schema_migrations",
		upPath:   "0001_bootstrap_schema_migrations.up.sql",
		downPath: "0001_other_name.down.sql",
		upSQL:    up,
		downSQL:  down,
	}
	return mapFSFor(migration)
}

func testSHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
