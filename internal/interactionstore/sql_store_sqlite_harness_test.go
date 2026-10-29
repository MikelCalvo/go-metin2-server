//go:build sqlite_harness

package interactionstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	_ "modernc.org/sqlite"
)

func interactionSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "interactions.sqlite"))+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func applyInteractionSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, 13); err != nil {
		t.Fatal(err)
	}
}

func authoredInteractionSQLSnapshot() Snapshot {
	return Snapshot{Definitions: []Definition{
		{Kind: KindInfo, Ref: "npc:info", Text: "Welcome", QuestRef: "quest:welcome", QuestFlag: "step", QuestFrom: 2},
		{Kind: KindTalk, Ref: "npc:talk", Text: "Hello"},
		{Kind: KindWarp, Ref: "npc:warp", Text: "Travel", MapIndex: 1, X: -12, Y: 42},
		{Kind: KindShopPreview, Ref: "npc:shop", Title: "Goods", Catalog: []MerchantCatalogEntry{{Slot: 0, ItemVnum: 123, Price: 33, Count: 2}}},
		{Kind: KindOpenSafebox, Ref: "npc:box", Text: "Store", Size: 2},
		{Kind: KindQuestFlag, Ref: "npc:quest", Text: "Complete", QuestRef: "quest:welcome", QuestFlag: "step", QuestFrom: 2, QuestTo: 3, RewardGold: 5, RewardItems: []RewardItemEntry{{ItemVnum: 7, Count: 2}}, ConsumeItems: []RewardItemEntry{{ItemVnum: 8, Count: 1}}},
	}}
}

func TestSQLiteHarnessInteractionSQLStoreRoundTripReplaceAndClear(t *testing.T) {
	db := interactionSQLiteDB(t)
	applyInteractionSchema(t, db)
	store := NewSQLStore(db)
	if err := store.Save(authoredInteractionSQLSnapshot()); err != nil {
		t.Fatal(err)
	}
	got, err := NewSQLStore(db).Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := normalizeSnapshot(authoredInteractionSQLSnapshot()); !reflect.DeepEqual(got, want) {
		t.Fatalf("roundtrip got %#v want %#v", got, want)
	}
	file := NewMemoryStore()
	if err := file.Save(Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:file", Text: "From file"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := SelectRematerializeStore(file, nil).Load(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:replacement", Text: "Replacement"}}}); err != nil {
		t.Fatal(err)
	}
	got, err = NewSQLStore(db).Load()
	if err != nil || len(got.Definitions) != 1 || got.Definitions[0].Ref != "npc:replacement" {
		t.Fatalf("replacement: %+v, %v", got, err)
	}
	if err := store.Save(Snapshot{}); err != nil {
		t.Fatal(err)
	}
	got, err = store.Load()
	if err != nil || len(got.Definitions) != 0 {
		t.Fatalf("empty: %+v, %v", got, err)
	}
	fromFile, err := file.Load()
	if err != nil || fromFile.Definitions[0].Ref != "npc:file" {
		t.Fatalf("file changed: %+v, %v", fromFile, err)
	}
}

func TestSQLiteHarnessInteractionSQLStoreSchemaAndRollback(t *testing.T) {
	beforeTip := interactionSQLiteDB(t)
	if _, err := dbmigrations.ApplyToVersion(context.Background(), beforeTip, nil, 12); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSQLStore(beforeTip).Load(); !errors.Is(err, ErrSQLSchemaRequired) {
		t.Fatalf("tip-0012 load: %v", err)
	}
	if err := NewSQLStore(beforeTip).Save(Snapshot{}); !errors.Is(err, ErrSQLSchemaRequired) {
		t.Fatalf("tip-0012 save: %v", err)
	}
	db := interactionSQLiteDB(t)
	store := NewSQLStore(db)
	if _, err := store.Load(); !errors.Is(err, ErrSQLSchemaRequired) {
		t.Fatalf("missing ledger load: %v", err)
	}
	if err := store.Save(Snapshot{}); !errors.Is(err, ErrSQLSchemaRequired) {
		t.Fatalf("missing ledger save: %v", err)
	}
	applyInteractionSchema(t, db)
	if err := store.Save(Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:kept", Text: "Kept"}}}); err != nil {
		t.Fatal(err)
	}
	// The existing schema owns the reference; never delete or cascade static actors.
	_, err := db.Exec(`INSERT INTO static_actors (entity_id, name, map_index, x, y, race_num, interaction_kind, interaction_ref) VALUES (9, 'Guide', 1, 1, 1, 1, 'talk', 'npc:kept')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Snapshot{}); !errors.Is(err, ErrSQLDefinitionsReferenced) {
		t.Fatalf("deleted referenced definition: %v", err)
	}
	// Even a replacement with the same key cannot quietly bypass an actor FK.
	if err := store.Save(Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:kept", Text: "Changed"}}}); !errors.Is(err, ErrSQLDefinitionsReferenced) {
		t.Fatalf("replaced referenced definition without actor migration policy: %v", err)
	}
	got, err := store.Load()
	if err != nil || len(got.Definitions) != 1 || got.Definitions[0].Ref != "npc:kept" {
		t.Fatalf("rollback: %+v, %v", got, err)
	}
}

func TestSQLiteHarnessInteractionSQLStoreSaveRollsBackAfterChildFailure(t *testing.T) {
	db := interactionSQLiteDB(t)
	applyInteractionSchema(t, db)
	store := NewSQLStore(db)
	seed := Snapshot{Definitions: []Definition{{Kind: KindTalk, Ref: "npc:kept", Text: "Kept"}}}
	if err := store.Save(seed); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_catalog_insert BEFORE INSERT ON interaction_merchant_catalog_entries BEGIN SELECT RAISE(ABORT, 'rejected catalog'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(authoredInteractionSQLSnapshot()); err == nil {
		t.Fatal("child insert unexpectedly succeeded")
	}
	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, normalizeSnapshot(seed)) {
		t.Fatalf("failed child insert did not preserve snapshot: %+v, %v", got, err)
	}
}

func TestSQLiteHarnessInteractionSQLStoreRejectsMalformedRows(t *testing.T) {
	db := interactionSQLiteDB(t)
	applyInteractionSchema(t, db)
	// The SQL CHECK rejects a partially populated warp coordinate group.
	if _, err := db.Exec(`INSERT INTO interaction_definitions (kind, ref, text, map_index, x, y) VALUES ('warp', 'npc:broken', 'hello', 1, NULL, 2)`); err == nil {
		t.Fatal("schema accepted incomplete warp coordinates")
	}
	if _, err := db.Exec(`INSERT INTO interaction_definitions (kind, ref, text) VALUES ('talk', 'npc:broken', '  ')`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSQLStore(db).Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("invalid loaded text: %v", err)
	}
	if err := NewSQLStore(db).Save(authoredInteractionSQLSnapshot()); err != nil {
		t.Fatalf("recover malformed content: %v", err)
	}
	if _, err := db.Exec(`UPDATE interaction_merchant_catalog_entries SET slot = 2`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSQLStore(db).Load(); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("noncontiguous catalog: %v", err)
	}
	if err := NewSQLStore(db).Save(Snapshot{}); err != nil {
		t.Fatal(err)
	}
}
