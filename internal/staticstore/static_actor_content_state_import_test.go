package staticstore

import (
	"context"
	"database/sql"
	"errors"
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
