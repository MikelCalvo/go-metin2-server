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
	testManifestFilename                        = "migrations.manifest.json"
	expectedBootstrapUpSHA256                   = "76ab086217590515cb9b1eb72d78f49abf766da977998c4c60b41825c8e92f78"
	expectedBootstrapDownSHA256                 = "140e8ba3c7a1c89cd942c13ef40160c74df5619093fe8c287c69cb978dba822d"
	expectedAccountCharacterRosterUpSHA256      = "5385c65b2f00b6c64567d604176f99f84b39afae62d840939e49ab2994b053af"
	expectedAccountCharacterRosterDownSHA256    = "cd8877ab1e88c4fe9a55d350bd5a89e1961ac88bd01423c5c1a1b0b8af37dc94"
	expectedCharacterItemStateUpSHA256          = "122e94f3d39975a6d1cf7e2d9321177a408e195be484e5ea2ffd5a8fa61c9a24"
	expectedCharacterItemStateDownSHA256        = "1a4dbc6d32c52a85eab837e00a9a63cc6c811b153a6054e5b568bdc3027592ee"
	expectedCharacterQuestStateUpSHA256         = "d67b53bc4f6aeaf74e9721f760ab05279037293f4de9e7b0079813984de56862"
	expectedCharacterQuestStateDownSHA256       = "70d2a9c4db6a47acd6574975c96449efd4eb6d3076db53c3eb6e21221936282f"
	expectedItemTemplateStateUpSHA256           = "6b615d308f7a0b3a0c8a67ebd16661a3fe7d7c5e608ee397127398f4e6fa2e4c"
	expectedItemTemplateStateDownSHA256         = "28d0adc265466bcfccaa683b7a777a3fbfb5aff146c962709532b0bb40bf3fce"
	expectedItemTemplateSafeboxRejectUpSHA256   = "83b5af7214706ffe8884d1ec841a190c2f6bf220b3899f11aa3850340643c280"
	expectedItemTemplateSafeboxRejectDownSHA256 = "7f04a66fc85f5e5b70be54c7ad8afae47d1b4e63004716e8814fdf141d3f1d81"
	expectedAuthLoginTicketHandoffUpSHA256      = "e42ae108f6b12938f4f622cc6c71f1d091ad5fc51c9892df78c6f05f3207eae9"
	expectedAuthLoginTicketHandoffDownSHA256    = "eec9767c316afeefe6319861e0a193df7b77c8e9eac6b42a2d6cf8f396127268"
	expectedStaticActorContentStateUpSHA256     = "303d4608766de8147c676e4d93f27e53a3744bf09343b060ec662d9c2378d9ad"
	expectedStaticActorContentStateDownSHA256   = "8a58559911600f73c9f8c0e23bd4b4df8919a0c0dbe19c2ede6a2771ac43a2d7"
	expectedItemTemplateRefineInfoUpSHA256      = "89ff5fd8c8e7f4c97a580b59d5b80196d5200aa0f19e1a3281691104e906788d"
	expectedItemTemplateRefineInfoDownSHA256    = "446ab6e77951ed82c7ca5eadb41c27855cf7eb10ad7c807939e58f4f23450ec6"
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

	if len(catalog) < 6 {
		t.Fatalf("expected item-template-state and safebox-reject migrations after quest-state, got %d migrations", len(catalog))
	}
	fifth := catalog[4]
	if fifth.Version != 5 || fifth.Name != "item_template_state" {
		t.Fatalf("unexpected fifth migration: %#v", fifth)
	}
	if fifth.UpPath != "0005_item_template_state.up.sql" {
		t.Fatalf("unexpected fifth up path: %q", fifth.UpPath)
	}
	if fifth.DownPath != "0005_item_template_state.down.sql" {
		t.Fatalf("unexpected fifth down path: %q", fifth.DownPath)
	}
	if fifth.UpSHA256 != expectedItemTemplateStateUpSHA256 {
		t.Fatalf("unexpected item-template-state up checksum: got %q want %q", fifth.UpSHA256, expectedItemTemplateStateUpSHA256)
	}
	if fifth.DownSHA256 != expectedItemTemplateStateDownSHA256 {
		t.Fatalf("unexpected item-template-state down checksum: got %q want %q", fifth.DownSHA256, expectedItemTemplateStateDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE item_templates",
		"CREATE TABLE item_template_sockets",
		"CREATE TABLE item_template_attributes",
		"CREATE TABLE item_template_use_effects",
		"CREATE TABLE item_template_equip_effects",
		"PRIMARY KEY (vnum, position)",
		"FOREIGN KEY (vnum) REFERENCES item_templates(vnum)",
		"CHECK (max_count > 0 AND max_count <= 255)",
		"CHECK (shop_buy_price >= 0 AND shop_buy_price <= 4294967295)",
		"CHECK (equip_slot IN ('', 'body', 'weapon', 'head', 'hair', 'shield', 'wrist', 'shoes', 'neck', 'ear', 'unique1', 'unique2', 'arrow'))",
	} {
		if !strings.Contains(fifth.UpSQL, want) {
			t.Fatalf("expected item-template-state migration to contain %q, got:\n%s", want, fifth.UpSQL)
		}
	}
	for _, want := range []string{
		"DROP TABLE item_template_equip_effects",
		"DROP TABLE item_template_use_effects",
		"DROP TABLE item_template_attributes",
		"DROP TABLE item_template_sockets",
		"DROP TABLE item_templates",
	} {
		if !strings.Contains(fifth.DownSQL, want) {
			t.Fatalf("expected item-template-state down migration to contain %q, got:\n%s", want, fifth.DownSQL)
		}
	}

	sixth := catalog[5]
	if sixth.Version != 6 || sixth.Name != "item_template_safebox_reject_message" {
		t.Fatalf("unexpected sixth migration: %#v", sixth)
	}
	if sixth.UpPath != "0006_item_template_safebox_reject_message.up.sql" {
		t.Fatalf("unexpected sixth up path: %q", sixth.UpPath)
	}
	if sixth.DownPath != "0006_item_template_safebox_reject_message.down.sql" {
		t.Fatalf("unexpected sixth down path: %q", sixth.DownPath)
	}
	if sixth.UpSHA256 != expectedItemTemplateSafeboxRejectUpSHA256 {
		t.Fatalf("unexpected item-template-safebox-reject up checksum: got %q want %q", sixth.UpSHA256, expectedItemTemplateSafeboxRejectUpSHA256)
	}
	if sixth.DownSHA256 != expectedItemTemplateSafeboxRejectDownSHA256 {
		t.Fatalf("unexpected item-template-safebox-reject down checksum: got %q want %q", sixth.DownSHA256, expectedItemTemplateSafeboxRejectDownSHA256)
	}
	for _, want := range []string{
		"ALTER TABLE item_templates ADD COLUMN safebox_reject_message",
		"CHECK (safebox_reject_message = '' OR anti_safebox = 1)",
	} {
		if !strings.Contains(sixth.UpSQL, want) {
			t.Fatalf("expected item-template-safebox-reject migration to contain %q, got:\n%s", want, sixth.UpSQL)
		}
	}
	if !strings.Contains(sixth.DownSQL, "ALTER TABLE item_templates DROP COLUMN safebox_reject_message") {
		t.Fatalf("expected item-template-safebox-reject down migration to drop column, got:\n%s", sixth.DownSQL)
	}

	if len(catalog) < 7 {
		t.Fatalf("expected auth login-ticket handoff migration after item-template safebox reject, got %d", len(catalog))
	}
	seventh := catalog[6]
	if seventh.Version != 7 || seventh.Name != "auth_login_ticket_handoff" {
		t.Fatalf("unexpected seventh migration: %#v", seventh)
	}
	if seventh.UpPath != "0007_auth_login_ticket_handoff.up.sql" {
		t.Fatalf("unexpected seventh up path: %q", seventh.UpPath)
	}
	if seventh.DownPath != "0007_auth_login_ticket_handoff.down.sql" {
		t.Fatalf("unexpected seventh down path: %q", seventh.DownPath)
	}
	if seventh.UpSHA256 != expectedAuthLoginTicketHandoffUpSHA256 {
		t.Fatalf("unexpected auth-login-ticket-handoff up checksum: got %q want %q", seventh.UpSHA256, expectedAuthLoginTicketHandoffUpSHA256)
	}
	if seventh.DownSHA256 != expectedAuthLoginTicketHandoffDownSHA256 {
		t.Fatalf("unexpected auth-login-ticket-handoff down checksum: got %q want %q", seventh.DownSHA256, expectedAuthLoginTicketHandoffDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE auth_login_tickets",
		"login_key BIGINT NOT NULL",
		"issued_at TEXT NOT NULL",
		"consumed_at TEXT",
		"characters_snapshot_json TEXT NOT NULL",
		"CHECK (login_key > 0 AND login_key <= 4294967295)",
		"CHECK (consumed_at IS NULL OR consumed_at >= issued_at)",
		"CHECK (characters_snapshot_json <> '')",
		"CREATE UNIQUE INDEX auth_login_tickets_active_login_key_unique",
		"WHERE consumed_at IS NULL",
	} {
		if !strings.Contains(seventh.UpSQL, want) {
			t.Fatalf("expected auth-login-ticket-handoff migration to contain %q, got:\n%s", want, seventh.UpSQL)
		}
	}
	if !strings.Contains(seventh.DownSQL, "DROP TABLE auth_login_tickets") {
		t.Fatalf("expected auth-login-ticket-handoff down migration to drop ticket table, got:\n%s", seventh.DownSQL)
	}

	if len(catalog) < 8 {
		t.Fatalf("expected static actor content-state migration after auth login-ticket handoff, got %d", len(catalog))
	}
	eighth := catalog[7]
	if eighth.Version != 8 || eighth.Name != "static_actor_content_state" {
		t.Fatalf("unexpected eighth migration: %#v", eighth)
	}
	if eighth.UpPath != "0008_static_actor_content_state.up.sql" {
		t.Fatalf("unexpected eighth up path: %q", eighth.UpPath)
	}
	if eighth.DownPath != "0008_static_actor_content_state.down.sql" {
		t.Fatalf("unexpected eighth down path: %q", eighth.DownPath)
	}
	if eighth.UpSHA256 != expectedStaticActorContentStateUpSHA256 {
		t.Fatalf("unexpected static-actor-content-state up checksum: got %q want %q", eighth.UpSHA256, expectedStaticActorContentStateUpSHA256)
	}
	if eighth.DownSHA256 != expectedStaticActorContentStateDownSHA256 {
		t.Fatalf("unexpected static-actor-content-state down checksum: got %q want %q", eighth.DownSHA256, expectedStaticActorContentStateDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE static_actors",
		"CREATE TABLE static_actor_reward_drops",
		"CREATE TABLE interaction_definitions",
		"CREATE TABLE interaction_merchant_catalog_entries",
		"FOREIGN KEY (entity_id) REFERENCES static_actors(entity_id)",
		"CHECK (race_num > 0 AND race_num <= 65535)",
		"CHECK (kind IN ('info', 'talk', 'warp', 'shop_preview'))",
		"CHECK (price > 0 AND price <= 4294967295)",
		"CREATE UNIQUE INDEX static_actors_spawn_group_ref_unique",
	} {
		if !strings.Contains(eighth.UpSQL, want) {
			t.Fatalf("expected static-actor-content-state migration to contain %q, got:\n%s", want, eighth.UpSQL)
		}
	}
	for _, want := range []string{
		"DROP TABLE interaction_merchant_catalog_entries",
		"DROP TABLE interaction_definitions",
		"DROP TABLE static_actor_reward_drops",
		"DROP TABLE static_actors",
	} {
		if !strings.Contains(eighth.DownSQL, want) {
			t.Fatalf("expected static-actor-content-state down migration to contain %q, got:\n%s", want, eighth.DownSQL)
		}
	}

	if len(catalog) < 9 {
		t.Fatalf("expected item-template refine-info migration after static actor content-state, got %d", len(catalog))
	}
	ninth := catalog[8]
	if ninth.Version != 9 || ninth.Name != "item_template_refine_info" {
		t.Fatalf("unexpected ninth migration: %#v", ninth)
	}
	if ninth.UpPath != "0009_item_template_refine_info.up.sql" {
		t.Fatalf("unexpected ninth up path: %q", ninth.UpPath)
	}
	if ninth.DownPath != "0009_item_template_refine_info.down.sql" {
		t.Fatalf("unexpected ninth down path: %q", ninth.DownPath)
	}
	if ninth.UpSHA256 != expectedItemTemplateRefineInfoUpSHA256 {
		t.Fatalf("unexpected item-template-refine-info up checksum: got %q want %q", ninth.UpSHA256, expectedItemTemplateRefineInfoUpSHA256)
	}
	if ninth.DownSHA256 != expectedItemTemplateRefineInfoDownSHA256 {
		t.Fatalf("unexpected item-template-refine-info down checksum: got %q want %q", ninth.DownSHA256, expectedItemTemplateRefineInfoDownSHA256)
	}
	for _, want := range []string{
		"CREATE TABLE item_template_refine_infos",
		"CREATE TABLE item_template_refine_materials",
		"FOREIGN KEY (vnum) REFERENCES item_templates(vnum)",
		"FOREIGN KEY (vnum) REFERENCES item_template_refine_infos(vnum)",
		"CHECK (result_vnum > 0 AND result_vnum <= 4294967295)",
		"CHECK (cost >= 0 AND cost <= 2147483647)",
		"CHECK (probability >= 0 AND probability <= 100)",
		"CHECK (position >= 0 AND position < 5)",
		"CHECK (count > 0 AND count <= 2147483647)",
	} {
		if !strings.Contains(ninth.UpSQL, want) {
			t.Fatalf("expected item-template-refine-info migration to contain %q, got:\n%s", want, ninth.UpSQL)
		}
	}
	for _, want := range []string{
		"DROP TABLE item_template_refine_materials",
		"DROP TABLE item_template_refine_infos",
	} {
		if !strings.Contains(ninth.DownSQL, want) {
			t.Fatalf("expected item-template-refine-info down migration to contain %q, got:\n%s", want, ninth.DownSQL)
		}
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

func TestCatalogSummaryReturnsMetadataOnlyDeterministicRows(t *testing.T) {
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

	summary, err := CatalogSummary(catalog)
	if err != nil {
		t.Fatalf("catalog summary: %v", err)
	}
	if summary.Format != CatalogSummaryFormat {
		t.Fatalf("unexpected catalog summary format: %#v", summary)
	}
	if summary.LatestVersion != 2 {
		t.Fatalf("expected latest version 2, got %#v", summary)
	}
	if len(summary.Migrations) != 2 {
		t.Fatalf("expected two catalog summary rows, got %#v", summary.Migrations)
	}
	first := summary.Migrations[0]
	if first.Version != 1 || first.Name != "bootstrap_schema_migrations" || first.UpPath != "0001_bootstrap_schema_migrations.up.sql" || first.DownPath != "0001_bootstrap_schema_migrations.down.sql" || first.UpSHA256 != catalog[0].UpSHA256 || first.DownSHA256 != catalog[0].DownSHA256 {
		t.Fatalf("unexpected first catalog summary row: %#v", first)
	}
	second := summary.Migrations[1]
	if second.Version != 2 || second.Name != "accounts" || second.UpPath != "0002_accounts.up.sql" || second.DownPath != "0002_accounts.down.sql" || second.UpSHA256 != catalog[1].UpSHA256 || second.DownSHA256 != catalog[1].DownSHA256 {
		t.Fatalf("unexpected second catalog summary row: %#v", second)
	}

	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal catalog summary: %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("catalog summary must not expose executable SQL marker %q, got %s", forbidden, body)
		}
	}
}

func TestCatalogSummaryUsesBuiltInCatalog(t *testing.T) {
	summary, err := BuiltInCatalogSummary()
	if err != nil {
		t.Fatalf("built-in catalog summary: %v", err)
	}
	if summary.Format != CatalogSummaryFormat || summary.LatestVersion < 9 {
		t.Fatalf("unexpected built-in catalog summary: %#v", summary)
	}
	if len(summary.Migrations) != summary.LatestVersion {
		t.Fatalf("expected one row per built-in migration, got latest=%d rows=%d", summary.LatestVersion, len(summary.Migrations))
	}
	if summary.Migrations[0].Version != 1 || summary.Migrations[0].Name != "bootstrap_schema_migrations" {
		t.Fatalf("unexpected first built-in catalog summary row: %#v", summary.Migrations[0])
	}
	latest := summary.Migrations[len(summary.Migrations)-1]
	if latest.Version != summary.LatestVersion || latest.Name != "item_template_refine_info" {
		t.Fatalf("unexpected latest built-in catalog summary row: %#v", latest)
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
