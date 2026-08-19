package contentbundle

import (
	"errors"
	"reflect"
	"testing"

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
