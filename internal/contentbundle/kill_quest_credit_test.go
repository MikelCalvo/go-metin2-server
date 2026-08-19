package contentbundle

import (
	"errors"
	"reflect"
	"testing"

	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestCanonicalizeAcceptsSpawnGroupKillQuestCredit(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.kill_quest_mob",
			Name:            "KillQuestMob",
			MapIndex:        42,
			X:               1800,
			Y:               2900,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestFrom: 0,
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize kill quest credit spawn group: %v", err)
	}
	want := SpawnGroup{
		Ref:             "practice.kill_quest_mob",
		Name:            "KillQuestMob",
		MapIndex:        42,
		X:               1800,
		Y:               2900,
		RaceNum:         101,
		CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
		RewardQuestRef:  "quest:first_steps",
		RewardQuestFlag: "killed_qa_mob",
		RewardQuestTo:   1,
		RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
	}
	if len(bundle.SpawnGroups) != 1 || !reflect.DeepEqual(bundle.SpawnGroups[0], want) {
		t.Fatalf("unexpected canonical kill quest credit spawn group:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}

func TestCanonicalizeRejectsPartialSpawnGroupKillQuestCredit(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.partial_kill_quest_mob",
			Name:            "PartialKillQuestMob",
			MapIndex:        42,
			X:               1800,
			Y:               2900,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for partial kill quest credit, got %v", err)
	}
}

func TestCanonicalizeRejectsKillQuestCreditWithFromEqualsTo(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.noop_kill_quest_mob",
			Name:            "NoopKillQuestMob",
			MapIndex:        42,
			X:               1800,
			Y:               2900,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestFrom: 1,
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated.",
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle when kill quest from equals to, got %v", err)
	}
}

func TestCanonicalizeExpandsRegenSpawnKillQuestCredit(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		RegenSpawns: []RegenSpawn{{
			Ref:             "practice.regen_kill_quest_mob",
			Name:            "RegenKillQuestMob",
			MapIndex:        1,
			X:               469900,
			Y:               964200,
			RaceNum:         20350,
			Count:           1,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize regen kill quest credit: %v", err)
	}
	want := []SpawnGroup{{
		Ref:             "practice.regen_kill_quest_mob",
		Name:            "RegenKillQuestMob",
		MapIndex:        1,
		X:               469900,
		Y:               964200,
		RaceNum:         20350,
		CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
		RewardQuestRef:  "quest:first_steps",
		RewardQuestFlag: "killed_qa_mob",
		RewardQuestTo:   1,
		RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
	}}
	if len(bundle.RegenSpawns) != 0 || !reflect.DeepEqual(bundle.SpawnGroups, want) {
		t.Fatalf("unexpected regen kill quest expansion:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}

func TestCanonicalizeExpandsAuthoringDropTableKillQuestCredit(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		DropTables: []DropTable{{
			Ref:              "loot.qa_kill_quest_reward",
			RewardExperience: 75,
			RewardGold:       60,
			DropVnums:        []uint32{27002, 27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
		SpawnGroups: []SpawnGroup{{
			Ref:                "practice.drop_table_kill_quest_mob",
			Name:               "DropTableKillQuestMob",
			MapIndex:           42,
			X:                  1785,
			Y:                  2885,
			RaceNum:            101,
			CombatProfile:      worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropTableRef: "loot.qa_kill_quest_reward",
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize drop-table kill quest credit: %v", err)
	}
	want := SpawnGroup{
		Ref:              "practice.drop_table_kill_quest_mob",
		Name:             "DropTableKillQuestMob",
		MapIndex:         42,
		X:                1785,
		Y:                2885,
		RaceNum:          101,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardExperience: 75,
		RewardGold:       60,
		RewardDropVnums:  []uint32{27001, 27002},
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
	}
	if len(bundle.DropTables) != 0 || len(bundle.SpawnGroups) != 1 || !reflect.DeepEqual(bundle.SpawnGroups[0], want) {
		t.Fatalf("unexpected drop-table kill quest expansion:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}

func TestCanonicalizeRejectsConflictingSpawnGroupDropTableKillQuestCredit(t *testing.T) {
	_, err := Canonicalize(Bundle{
		DropTables: []DropTable{{
			Ref:              "loot.qa_kill_quest_reward",
			RewardExperience: 75,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		SpawnGroups: []SpawnGroup{{
			Ref:                "practice.conflicting_drop_table_kill_quest_mob",
			Name:               "ConflictingDropTableKillQuestMob",
			MapIndex:           42,
			X:                  1785,
			Y:                  2885,
			RaceNum:            101,
			CombatProfile:      worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropTableRef: "loot.qa_kill_quest_reward",
			RewardQuestRef:     "quest:first_steps",
			RewardQuestFlag:    "killed_qa_mob",
			RewardQuestTo:      1,
			RewardQuestText:    "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for conflicting drop-table kill quest credit, got %v", err)
	}
}

func TestCanonicalizeRejectsPartialDropTableKillQuestCredit(t *testing.T) {
	_, err := Canonicalize(Bundle{
		DropTables: []DropTable{{
			Ref:              "loot.qa_partial_kill_quest_reward",
			RewardExperience: 75,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
		}},
		SpawnGroups: []SpawnGroup{{
			Ref:                "practice.partial_drop_table_kill_quest_mob",
			Name:               "PartialDropTableKillQuestMob",
			MapIndex:           42,
			X:                  1785,
			Y:                  2885,
			RaceNum:            101,
			CombatProfile:      worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropTableRef: "loot.qa_partial_kill_quest_reward",
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for partial drop-table kill quest credit, got %v", err)
	}
}

func TestCanonicalizeExpandsRegenSpawnDropTableKillQuestCredit(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		DropTables: []DropTable{{
			Ref:              "loot.qa_regen_kill_quest_reward",
			RewardExperience: 90,
			RewardGold:       45,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		}},
		RegenSpawns: []RegenSpawn{{
			Ref:                "practice.regen_drop_table_kill_quest_mob",
			Name:               "RegenDropTableKillQuestMob",
			MapIndex:           1,
			X:                  469900,
			Y:                  964200,
			RaceNum:            20350,
			Count:              1,
			RewardDropTableRef: "loot.qa_regen_kill_quest_reward",
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize regen drop-table kill quest credit: %v", err)
	}
	want := []SpawnGroup{{
		Ref:              "practice.regen_drop_table_kill_quest_mob",
		Name:             "RegenDropTableKillQuestMob",
		MapIndex:         1,
		X:                469900,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardExperience: 90,
		RewardGold:       45,
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
	}}
	if len(bundle.DropTables) != 0 || len(bundle.RegenSpawns) != 0 || !reflect.DeepEqual(bundle.SpawnGroups, want) {
		t.Fatalf("unexpected regen drop-table kill quest expansion:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}

func TestCanonicalizeAcceptsSpawnGroupKillQuestRequireGate(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.gated_kill_quest_mob",
			Name:             "GatedKillQuestMob",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize gated kill quest credit: %v", err)
	}
	want := SpawnGroup{
		Ref:              "practice.gated_kill_quest_mob",
		Name:             "GatedKillQuestMob",
		MapIndex:         42,
		X:                1800,
		Y:                2900,
		RaceNum:          101,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		RequireQuestRef:  "quest:first_steps",
		RequireQuestFlag: "met_guide",
		RequireQuestFrom: 1,
	}
	if len(bundle.SpawnGroups) != 1 || !reflect.DeepEqual(bundle.SpawnGroups[0], want) {
		t.Fatalf("unexpected gated kill quest credit:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}

func TestCanonicalizeRejectsPartialSpawnGroupKillQuestRequireGate(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:             "practice.partial_gated_kill_quest_mob",
			Name:            "PartialGatedKillQuestMob",
			MapIndex:        42,
			X:               1800,
			Y:               2900,
			RaceNum:         101,
			CombatProfile:   worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:  "quest:first_steps",
			RewardQuestFlag: "killed_qa_mob",
			RewardQuestTo:   1,
			RewardQuestText: "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef: "quest:first_steps",
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for partial require gate, got %v", err)
	}
}

func TestCanonicalizeRejectsOrphanRequireQuestFromWithoutGate(t *testing.T) {
	_, err := Canonicalize(Bundle{
		SpawnGroups: []SpawnGroup{{
			Ref:              "practice.orphan_require_from_mob",
			Name:             "OrphanRequireFromMob",
			MapIndex:         42,
			X:                1800,
			Y:                2900,
			RaceNum:          101,
			CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestFrom: 1,
		}},
	})
	if !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("expected ErrInvalidBundle for orphan require_quest_from, got %v", err)
	}
}

func TestCanonicalizeExpandsDropTableKillQuestRequireGate(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		DropTables: []DropTable{{
			Ref:              "loot.qa_gated_kill_quest_reward",
			RewardExperience: 75,
			RewardGold:       60,
			DropVnums:        []uint32{27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		SpawnGroups: []SpawnGroup{{
			Ref:                "practice.gated_table_kill_quest_mob",
			Name:               "GatedTableKillQuestMob",
			MapIndex:           1,
			X:                  469850,
			Y:                  964200,
			RaceNum:            20350,
			CombatProfile:      worldruntime.StaticActorCombatProfilePracticeMob,
			RewardDropTableRef: "loot.qa_gated_kill_quest_reward",
		}},
		ItemTemplates: []itemcatalog.Template{{
			Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200,
		}},
	})
	if err != nil {
		t.Fatalf("canonicalize drop-table gated kill quest credit: %v", err)
	}
	want := SpawnGroup{
		Ref:              "practice.gated_table_kill_quest_mob",
		Name:             "GatedTableKillQuestMob",
		MapIndex:         1,
		X:                469850,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardExperience: 75,
		RewardGold:       60,
		RewardDropVnums:  []uint32{27001},
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		RequireQuestRef:  "quest:first_steps",
		RequireQuestFlag: "met_guide",
		RequireQuestFrom: 1,
	}
	if len(bundle.DropTables) != 0 || len(bundle.SpawnGroups) != 1 || !reflect.DeepEqual(bundle.SpawnGroups[0], want) {
		t.Fatalf("unexpected gated drop-table expansion:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}

func TestCanonicalizeExpandsRegenSpawnDropTableKillQuestRequireGate(t *testing.T) {
	bundle, err := Canonicalize(Bundle{
		DropTables: []DropTable{{
			Ref:              "loot.qa_regen_gated_kill_quest_reward",
			RewardExperience: 90,
			RewardGold:       45,
			DropVnums:        []uint32{27002, 27001},
			RewardQuestRef:   "quest:first_steps",
			RewardQuestFlag:  "killed_qa_mob",
			RewardQuestTo:    1,
			RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
			RequireQuestRef:  "quest:first_steps",
			RequireQuestFlag: "met_guide",
			RequireQuestFrom: 1,
		}},
		RegenSpawns: []RegenSpawn{{
			Ref:                "practice.regen_gated_table_kill_quest_mob",
			Name:               "RegenGatedTableKillQuestMob",
			MapIndex:           1,
			X:                  469900,
			Y:                  964200,
			RaceNum:            20350,
			Count:              1,
			RewardDropTableRef: "loot.qa_regen_gated_kill_quest_reward",
		}},
		ItemTemplates: []itemcatalog.Template{
			{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200},
			{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200},
		},
	})
	if err != nil {
		t.Fatalf("canonicalize regen drop-table gated kill quest credit: %v", err)
	}
	want := []SpawnGroup{{
		Ref:              "practice.regen_gated_table_kill_quest_mob",
		Name:             "RegenGatedTableKillQuestMob",
		MapIndex:         1,
		X:                469900,
		Y:                964200,
		RaceNum:          20350,
		CombatProfile:    worldruntime.StaticActorCombatProfilePracticeMob,
		RewardExperience: 90,
		RewardGold:       45,
		RewardDropVnums:  []uint32{27001, 27002},
		RewardQuestRef:   "quest:first_steps",
		RewardQuestFlag:  "killed_qa_mob",
		RewardQuestTo:    1,
		RewardQuestText:  "Quest updated: first_steps.killed_qa_mob = 1.",
		RequireQuestRef:  "quest:first_steps",
		RequireQuestFlag: "met_guide",
		RequireQuestFrom: 1,
	}}
	if len(bundle.DropTables) != 0 || len(bundle.RegenSpawns) != 0 || !reflect.DeepEqual(bundle.SpawnGroups, want) {
		t.Fatalf("unexpected regen gated drop-table expansion:\n got: %#v\nwant: %#v", bundle.SpawnGroups, want)
	}
}
