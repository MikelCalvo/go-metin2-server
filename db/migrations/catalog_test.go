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
	testManifestFilename                     = "migrations.manifest.json"
	expectedBootstrapUpSHA256                = "76ab086217590515cb9b1eb72d78f49abf766da977998c4c60b41825c8e92f78"
	expectedBootstrapDownSHA256              = "140e8ba3c7a1c89cd942c13ef40160c74df5619093fe8c287c69cb978dba822d"
	expectedAccountCharacterRosterUpSHA256   = "5385c65b2f00b6c64567d604176f99f84b39afae62d840939e49ab2994b053af"
	expectedAccountCharacterRosterDownSHA256 = "cd8877ab1e88c4fe9a55d350bd5a89e1961ac88bd01423c5c1a1b0b8af37dc94"
	expectedCharacterItemStateUpSHA256       = "122e94f3d39975a6d1cf7e2d9321177a408e195be484e5ea2ffd5a8fa61c9a24"
	expectedCharacterItemStateDownSHA256     = "1a4dbc6d32c52a85eab837e00a9a63cc6c811b153a6054e5b568bdc3027592ee"
	expectedCharacterQuestStateUpSHA256      = "d67b53bc4f6aeaf74e9721f760ab05279037293f4de9e7b0079813984de56862"
	expectedCharacterQuestStateDownSHA256    = "70d2a9c4db6a47acd6574975c96449efd4eb6d3076db53c3eb6e21221936282f"
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

	if len(catalog) < 2 {
		t.Fatalf("expected account/character roster migration after bootstrap ledger, got %d migrations", len(catalog))
	}
	second := catalog[1]
	if second.Version != 2 || second.Name != "account_character_roster" {
		t.Fatalf("unexpected second migration: %#v", second)
	}
	if second.UpPath != "0002_account_character_roster.up.sql" {
		t.Fatalf("unexpected second up path: %q", second.UpPath)
	}
	if second.DownPath != "0002_account_character_roster.down.sql" {
		t.Fatalf("unexpected second down path: %q", second.DownPath)
	}
	if second.UpSHA256 != expectedAccountCharacterRosterUpSHA256 {
		t.Fatalf("unexpected account/character roster up checksum: got %q want %q", second.UpSHA256, expectedAccountCharacterRosterUpSHA256)
	}
	if second.DownSHA256 != expectedAccountCharacterRosterDownSHA256 {
		t.Fatalf("unexpected account/character roster down checksum: got %q want %q", second.DownSHA256, expectedAccountCharacterRosterDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE accounts",
		"CREATE TABLE characters",
		"CREATE UNIQUE INDEX accounts_login_normalized_unique",
		"CREATE UNIQUE INDEX characters_account_slot_unique",
		"CREATE UNIQUE INDEX characters_name_normalized_unique",
		"FOREIGN KEY (account_id) REFERENCES accounts(id)",
		"CHECK (slot >= 0 AND slot < 4)",
	} {
		if !strings.Contains(second.UpSQL, want) {
			t.Fatalf("expected account/character roster migration to contain %q, got:\n%s", want, second.UpSQL)
		}
	}
	for _, want := range []string{
		"DROP TABLE characters",
		"DROP TABLE accounts",
	} {
		if !strings.Contains(second.DownSQL, want) {
			t.Fatalf("expected account/character roster down migration to contain %q, got:\n%s", want, second.DownSQL)
		}
	}

	if len(catalog) < 3 {
		t.Fatalf("expected character item-state migration after account/character roster, got %d migrations", len(catalog))
	}
	third := catalog[2]
	if third.Version != 3 || third.Name != "character_item_state" {
		t.Fatalf("unexpected third migration: %#v", third)
	}
	if third.UpPath != "0003_character_item_state.up.sql" {
		t.Fatalf("unexpected third up path: %q", third.UpPath)
	}
	if third.DownPath != "0003_character_item_state.down.sql" {
		t.Fatalf("unexpected third down path: %q", third.DownPath)
	}
	if third.UpSHA256 != expectedCharacterItemStateUpSHA256 {
		t.Fatalf("unexpected character item-state up checksum: got %q want %q", third.UpSHA256, expectedCharacterItemStateUpSHA256)
	}
	if third.DownSHA256 != expectedCharacterItemStateDownSHA256 {
		t.Fatalf("unexpected character item-state down checksum: got %q want %q", third.DownSHA256, expectedCharacterItemStateDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE character_inventory_items",
		"CREATE TABLE character_equipment_items",
		"CREATE TABLE character_quickslots",
		"FOREIGN KEY (character_id) REFERENCES characters(id)",
		"CREATE UNIQUE INDEX character_inventory_items_character_slot_unique",
		"CREATE UNIQUE INDEX character_equipment_items_character_slot_unique",
		"CREATE UNIQUE INDEX character_quickslots_character_position_unique",
		"CHECK (slot >= 0 AND slot < 90)",
		"CHECK (position >= 0 AND position < 36)",
	} {
		if !strings.Contains(third.UpSQL, want) {
			t.Fatalf("expected character item-state migration to contain %q, got:\n%s", want, third.UpSQL)
		}
	}
	for _, want := range []string{
		"DROP TABLE character_quickslots",
		"DROP TABLE character_equipment_items",
		"DROP TABLE character_inventory_items",
	} {
		if !strings.Contains(third.DownSQL, want) {
			t.Fatalf("expected character item-state down migration to contain %q, got:\n%s", want, third.DownSQL)
		}
	}

	if len(catalog) < 4 {
		t.Fatalf("expected character quest-state migration after item-state, got %d migrations", len(catalog))
	}
	fourth := catalog[3]
	if fourth.Version != 4 || fourth.Name != "character_quest_state" {
		t.Fatalf("unexpected fourth migration: %#v", fourth)
	}
	if fourth.UpPath != "0004_character_quest_state.up.sql" {
		t.Fatalf("unexpected fourth up path: %q", fourth.UpPath)
	}
	if fourth.DownPath != "0004_character_quest_state.down.sql" {
		t.Fatalf("unexpected fourth down path: %q", fourth.DownPath)
	}
	if fourth.UpSHA256 != expectedCharacterQuestStateUpSHA256 {
		t.Fatalf("unexpected character quest-state up checksum: got %q want %q", fourth.UpSHA256, expectedCharacterQuestStateUpSHA256)
	}
	if fourth.DownSHA256 != expectedCharacterQuestStateDownSHA256 {
		t.Fatalf("unexpected character quest-state down checksum: got %q want %q", fourth.DownSHA256, expectedCharacterQuestStateDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE character_quest_flags",
		"FOREIGN KEY (character_id) REFERENCES characters(id)",
		"PRIMARY KEY (character_id, quest_ref, flag_name)",
		"CREATE INDEX character_quest_flags_quest_ref_index",
		"CHECK (character_id > 0)",
		"CHECK (value > 0)",
	} {
		if !strings.Contains(fourth.UpSQL, want) {
			t.Fatalf("expected character quest-state migration to contain %q, got:\n%s", want, fourth.UpSQL)
		}
	}
	if !strings.Contains(fourth.DownSQL, "DROP TABLE character_quest_flags") {
		t.Fatalf("expected character quest-state down migration to drop quest flags, got:\n%s", fourth.DownSQL)
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
