//go:build sqlite_harness

package queststate

import (
	"context"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessOptInSQLStoreRematerializeSurvivesReloadBesideFileStore(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion: %v", err)
	}
	accounts := []accountstore.Account{{
		Login: "Alpha", Empire: 1,
		Characters: []loginticket.Character{
			questStateImportCharacter(11, "AlphaWar"),
			questStateImportCharacter(22, "BravoNinja"),
		},
	}}
	roster, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("export roster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, roster); err != nil {
		t.Fatalf("seed roster: %v", err)
	}

	fileStore := NewMemoryStore()
	if err := fileStore.Save(Snapshot{Flags: []Flag{{
		Character: "AlphaWar", QuestRef: "quest:first_steps", Name: "step", Value: 2,
	}}}); err != nil {
		t.Fatalf("seed file store: %v", err)
	}
	sqlStore := NewSQLStore(db)
	selected := SelectRematerializeStore(fileStore, sqlStore)
	if selected != sqlStore {
		t.Fatal("opt-in selector did not return the SQLStore")
	}
	if err := rematerializeQuestFlag(selected, Transition{
		Character: "AlphaWar",
		QuestRef:  "quest:first_steps",
		Flag:      "met_guide",
		From:      0,
		To:        1,
	}); err != nil {
		t.Fatalf("opt-in transition: %v", err)
	}

	reloaded := SelectRematerializeStore(NewMemoryStore(), NewSQLStore(db))
	got, err := rematerializeQuestFlagValue(reloaded, "AlphaWar", "quest:first_steps", "met_guide")
	if err != nil {
		t.Fatalf("opt-in reload: %v", err)
	}
	if got != 1 {
		t.Fatalf("opt-in reloaded flag = %d, want 1", got)
	}
	fileValue, err := rematerializeQuestFlagValue(fileStore, "AlphaWar", "quest:first_steps", "step")
	if err != nil {
		t.Fatalf("file reload: %v", err)
	}
	if fileValue != 2 {
		t.Fatalf("opt-in transition changed the FileStore step flag to %d", fileValue)
	}
	if leaked, err := rematerializeQuestFlagValue(fileStore, "AlphaWar", "quest:first_steps", "met_guide"); err != nil || leaked != 0 {
		t.Fatalf("opt-in transition leaked onto the FileStore: value=%d err=%v", leaked, err)
	}

	if err := rematerializeQuestFlag(selected, Transition{
		Character: "BravoNinja",
		QuestRef:  "quest:kill_qa_mob",
		Flag:      "killed_qa_mob",
		From:      0,
		To:        3,
	}); err != nil {
		t.Fatalf("second character transition: %v", err)
	}
	alpha, err := rematerializeQuestFlagValue(NewSQLStore(db), "AlphaWar", "quest:first_steps", "met_guide")
	if err != nil {
		t.Fatalf("reload alpha after scoped replace: %v", err)
	}
	bravo, err := rematerializeQuestFlagValue(NewSQLStore(db), "BravoNinja", "quest:kill_qa_mob", "killed_qa_mob")
	if err != nil {
		t.Fatalf("reload bravo after scoped replace: %v", err)
	}
	if alpha != 1 || bravo != 3 {
		t.Fatalf("scoped replace alpha=%d bravo=%d, want 1 and 3", alpha, bravo)
	}

	exported, err := NewSQLStore(db).ExportCharacterQuestState(map[string]uint32{
		"AlphaWar":   11,
		"BravoNinja": 22,
	})
	if err != nil {
		t.Fatalf("export SQL flags: %v", err)
	}
	if exported.MigrationVersion != CharacterQuestStateMigrationVersion || exported.MigrationName != CharacterQuestStateMigrationName || len(exported.Flags) != 2 {
		t.Fatalf("unexpected SQL export: %+v", exported)
	}
}

func TestSQLiteHarnessOptInSQLStoreRejectsUnknownCharacterAndEmptySaveIsNoop(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion: %v", err)
	}
	accounts := []accountstore.Account{{
		Login: "Alpha", Empire: 1,
		Characters: []loginticket.Character{questStateImportCharacter(11, "AlphaWar")},
	}}
	roster, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("export roster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, roster); err != nil {
		t.Fatalf("seed roster: %v", err)
	}
	store := NewSQLStore(db)
	if err := store.Save(Snapshot{Flags: []Flag{{
		Character: "AlphaWar", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1,
	}}}); err != nil {
		t.Fatalf("seed SQL flag: %v", err)
	}
	if err := store.Save(Snapshot{Flags: []Flag{}}); err != nil {
		t.Fatalf("empty save: %v", err)
	}
	kept, err := rematerializeQuestFlagValue(store, "AlphaWar", "quest:first_steps", "met_guide")
	if err != nil {
		t.Fatalf("reload after empty save: %v", err)
	}
	if kept != 1 {
		t.Fatalf("empty save truncated the table, flag=%d", kept)
	}
	err = store.Save(Snapshot{Flags: []Flag{{
		Character: "MissingHero", QuestRef: "quest:first_steps", Name: "step", Value: 1,
	}}})
	if err == nil {
		t.Fatal("expected unknown character save to fail closed")
	}
	kept, err = rematerializeQuestFlagValue(NewSQLStore(db), "AlphaWar", "quest:first_steps", "met_guide")
	if err != nil {
		t.Fatalf("reload after rejected save: %v", err)
	}
	if kept != 1 {
		t.Fatalf("rejected save changed the committed flag to %d", kept)
	}
}

func TestSQLiteHarnessOptInSQLStoreClearsLastFlagWithoutTouchingSibling(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion: %v", err)
	}
	accounts := []accountstore.Account{{
		Login: "Alpha", Empire: 1,
		Characters: []loginticket.Character{
			questStateImportCharacter(11, "AlphaWar"),
			questStateImportCharacter(22, "BravoNinja"),
		},
	}}
	roster, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("export roster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, roster); err != nil {
		t.Fatalf("seed roster: %v", err)
	}
	store := NewSQLStore(db)
	if err := store.Save(Snapshot{Flags: []Flag{
		{Character: "AlphaWar", QuestRef: "quest:first_steps", Name: "met_guide", Value: 1},
		{Character: "BravoNinja", QuestRef: "quest:kill_qa_mob", Name: "killed_qa_mob", Value: 2},
	}}); err != nil {
		t.Fatalf("seed flags: %v", err)
	}
	if err := rematerializeQuestFlag(store, Transition{
		Character: "AlphaWar",
		QuestRef:  "quest:first_steps",
		Flag:      "met_guide",
		From:      1,
		To:        0,
	}); err != nil {
		t.Fatalf("clear last alpha flag: %v", err)
	}
	alpha, err := rematerializeQuestFlagValue(NewSQLStore(db), "AlphaWar", "quest:first_steps", "met_guide")
	if err != nil {
		t.Fatalf("reload alpha: %v", err)
	}
	bravo, err := rematerializeQuestFlagValue(NewSQLStore(db), "BravoNinja", "quest:kill_qa_mob", "killed_qa_mob")
	if err != nil {
		t.Fatalf("reload bravo: %v", err)
	}
	if alpha != 0 || bravo != 2 {
		t.Fatalf("after clear alpha=%d bravo=%d, want 0 and 2", alpha, bravo)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quest_flags WHERE character_id = 11`).Scan(&rows); err != nil {
		t.Fatalf("count alpha rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("cleared character still has %d SQL rows", rows)
	}
}
