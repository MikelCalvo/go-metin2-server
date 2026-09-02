package staticstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestImportStaticActorContentStateRejectsNilExecutor(t *testing.T) {
	export := sampleStaticActorContentStateExport()

	_, err := ImportStaticActorContentState(context.Background(), nil, export)
	if !errors.Is(err, ErrStaticActorContentStateImportExecutorRequired) {
		t.Fatalf("ImportStaticActorContentState(nil) error = %v, want %v", err, ErrStaticActorContentStateImportExecutorRequired)
	}
}

func TestImportStaticActorContentStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := StaticActorContentStateExport{
		MigrationVersion:              99,
		MigrationName:                 "not-static-actor-content-state",
		InteractionDefinitions:        []InteractionDefinitionRow{},
		MerchantCatalogEntries:        []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:          []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:         []InteractionQuestFlagItemRow{},
		StaticActors:                  []StaticActorContentStateRow{},
		RewardDrops:                   []StaticActorRewardDropRow{},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}

	_, err := ImportStaticActorContentState(context.Background(), failingStaticActorContentStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("ImportStaticActorContentState(invalid) error = %v, want %v", err, ErrInvalidStaticActorContentStateExport)
	}
}

func TestImportStaticActorContentStateRejectsNilCollectionsBeforeOpeningTransaction(t *testing.T) {
	export := StaticActorContentStateExport{
		MigrationVersion:              StaticActorContentStateMigrationVersion,
		MigrationName:                 StaticActorContentStateMigrationName,
		InteractionDefinitions:        nil,
		MerchantCatalogEntries:        []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:          []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:         []InteractionQuestFlagItemRow{},
		StaticActors:                  []StaticActorContentStateRow{},
		RewardDrops:                   []StaticActorRewardDropRow{},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}

	_, err := ImportStaticActorContentState(context.Background(), failingStaticActorContentStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("ImportStaticActorContentState(nil collections) error = %v, want %v", err, ErrInvalidStaticActorContentStateExport)
	}
}

type failingStaticActorContentStateImportExecutor struct{}

func (failingStaticActorContentStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid static-actor content-state exports")
}

func TestImportStaticActorContentStateRejectsMoreThanOneOptionsValue(t *testing.T) {
	export := StaticActorContentStateExport{
		MigrationVersion:              StaticActorContentStateMigrationVersion,
		MigrationName:                 StaticActorContentStateMigrationName,
		InteractionDefinitions:        []InteractionDefinitionRow{},
		MerchantCatalogEntries:        []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:          []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:         []InteractionQuestFlagItemRow{},
		StaticActors:                  []StaticActorContentStateRow{},
		RewardDrops:                   []StaticActorRewardDropRow{},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}
	_, err := ImportStaticActorContentState(
		context.Background(),
		failingStaticActorContentStateImportExecutor{},
		export,
		ImportStaticActorContentStateOptions{Replace: true},
		ImportStaticActorContentStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportStaticActorContentState(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineStaticActorContentStateExportMergesDeclaredScopes(t *testing.T) {
	export := StaticActorContentStateExport{
		MigrationVersion:              StaticActorContentStateMigrationVersion,
		MigrationName:                 StaticActorContentStateMigrationName,
		EntityIDs:                     []uint64{7},
		InteractionDefinitionKeys:     []InteractionDefinitionKey{{Kind: "talk", Ref: "npc:village_guard"}},
		CombatProfileNames:            []string{"practice_static_store_import_wolf"},
		InteractionDefinitions:        []InteractionDefinitionRow{},
		MerchantCatalogEntries:        []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:          []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:         []InteractionQuestFlagItemRow{},
		StaticActors:                  []StaticActorContentStateRow{},
		RewardDrops:                   []StaticActorRewardDropRow{},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}

	canonical, summary, err := QuarantineStaticActorContentStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.StaticActorCount != 0 || summary.InteractionDefinitionCount != 0 || summary.CombatProfileCount != 0 {
		t.Fatalf("declared wipe should keep zero tip-0013 counts: %#v", summary)
	}
	if len(summary.EntityIDs) != 1 || summary.EntityIDs[0] != 7 {
		t.Fatalf("unexpected declared wipe summary entity ids: %#v", summary.EntityIDs)
	}
	if len(canonical.EntityIDs) != 1 || canonical.EntityIDs[0] != 7 {
		t.Fatalf("unexpected canonical entity ids: %#v", canonical.EntityIDs)
	}
	if len(canonical.InteractionDefinitionKeys) != 1 ||
		canonical.InteractionDefinitionKeys[0].Kind != "talk" ||
		canonical.InteractionDefinitionKeys[0].Ref != "npc:village_guard" {
		t.Fatalf("unexpected canonical interaction definition keys: %#v", canonical.InteractionDefinitionKeys)
	}
	if len(canonical.CombatProfileNames) != 1 || canonical.CombatProfileNames[0] != "practice_static_store_import_wolf" {
		t.Fatalf("unexpected canonical combat profile names: %#v", canonical.CombatProfileNames)
	}
}

func TestQuarantineStaticActorContentStateExportRejectsInvalidDeclaredScopes(t *testing.T) {
	base := StaticActorContentStateExport{
		MigrationVersion:              StaticActorContentStateMigrationVersion,
		MigrationName:                 StaticActorContentStateMigrationName,
		InteractionDefinitions:        []InteractionDefinitionRow{},
		MerchantCatalogEntries:        []InteractionMerchantCatalogEntryRow{},
		QuestFlagRewardItems:          []InteractionQuestFlagItemRow{},
		QuestFlagConsumeItems:         []InteractionQuestFlagItemRow{},
		StaticActors:                  []StaticActorContentStateRow{},
		RewardDrops:                   []StaticActorRewardDropRow{},
		CombatProfiles:                []StaticActorCombatProfileRow{},
		CombatProfileDeathRewardDrops: []StaticActorCombatProfileDropRow{},
	}

	zeroEntity := base
	zeroEntity.EntityIDs = []uint64{0}
	if _, _, err := QuarantineStaticActorContentStateExport(zeroEntity); err == nil || !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("zero entity_ids error = %v, want invalid export", err)
	}

	dupEntity := base
	dupEntity.EntityIDs = []uint64{7, 7}
	if _, _, err := QuarantineStaticActorContentStateExport(dupEntity); err == nil || !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("duplicate entity_ids error = %v, want invalid export", err)
	}

	emptyDefKey := base
	emptyDefKey.InteractionDefinitionKeys = []InteractionDefinitionKey{{Kind: "talk", Ref: ""}}
	if _, _, err := QuarantineStaticActorContentStateExport(emptyDefKey); err == nil || !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("empty interaction definition key error = %v, want invalid export", err)
	}

	emptyProfile := base
	emptyProfile.CombatProfileNames = []string{""}
	if _, _, err := QuarantineStaticActorContentStateExport(emptyProfile); err == nil || !errors.Is(err, ErrInvalidStaticActorContentStateExport) {
		t.Fatalf("empty combat_profile_names error = %v, want invalid export", err)
	}
}
