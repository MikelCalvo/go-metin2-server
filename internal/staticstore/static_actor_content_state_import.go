package staticstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// ErrStaticActorContentStateImportExecutorRequired reports that the SQL import
// primitive was called without a transaction-capable executor.
var ErrStaticActorContentStateImportExecutorRequired = errors.New("static-actor content-state import executor is required")

// ErrStaticActorContentStateImportSchemaRequired reports that the target
// database has not applied the 0013_static_actor_combat_profile_state migration
// boundary yet.
var ErrStaticActorContentStateImportSchemaRequired = errors.New("static-actor content-state schema is not applied")

// ErrStaticActorContentStateImportRowCount reports that an INSERT affected an
// unexpected number of rows during static-actor content-state backfill.
var ErrStaticActorContentStateImportRowCount = errors.New("static-actor content-state import row count mismatch")

// StaticActorContentStateImportResult is the metadata-only outcome of importing
// a quarantined 0013 static-actor content-state export. It never includes
// content payloads, SQL text, DSNs, or FileStore snapshot bytes.
type StaticActorContentStateImportResult struct {
	MigrationVersion                  int      `json:"migration_version"`
	MigrationName                     string   `json:"migration_name"`
	InteractionDefinitionCount        int      `json:"interaction_definition_count"`
	MerchantCatalogEntryCount         int      `json:"merchant_catalog_entry_count"`
	QuestFlagRewardItemCount          int      `json:"quest_flag_reward_item_count"`
	QuestFlagConsumeItemCount         int      `json:"quest_flag_consume_item_count"`
	StaticActorCount                  int      `json:"static_actor_count"`
	RewardDropCount                   int      `json:"reward_drop_count"`
	CombatProfileCount                int      `json:"combat_profile_count"`
	CombatProfileDeathRewardDropCount int      `json:"combat_profile_death_reward_drop_count"`
	EntityIDs                         []uint64 `json:"entity_ids"`
	InteractionKinds                  []string `json:"interaction_kinds"`
	CombatProfiles                    []string `json:"combat_profiles"`
}

// ImportStaticActorContentState validates a retained 0013 static-actor
// content-state export through the existing quarantine contract and inserts the
// canonicalized rows into the tip-0013 interaction / static-actor / combat-profile
// tables inside one transaction.
//
// The caller still owns driver selection and DSN loading. This primitive does
// not mutate bootstrap file stores or live content indexes, does not rewrite
// schema_migrations, and does not invent upsert / merge policy: duplicate
// primary keys or unique-index collisions fail closed and roll the transaction
// back.
func ImportStaticActorContentState(ctx context.Context, executor dbmigrations.SQLMigrationExecutor, export StaticActorContentStateExport) (StaticActorContentStateImportResult, error) {
	if staticActorContentStateImportExecutorIsNil(executor) {
		return StaticActorContentStateImportResult{}, ErrStaticActorContentStateImportExecutorRequired
	}

	canonical, summary, err := QuarantineStaticActorContentStateExport(export)
	if err != nil {
		return StaticActorContentStateImportResult{}, err
	}

	result := StaticActorContentStateImportResult{
		MigrationVersion:                  StaticActorContentStateMigrationVersion,
		MigrationName:                     StaticActorContentStateMigrationName,
		InteractionDefinitionCount:        summary.InteractionDefinitionCount,
		MerchantCatalogEntryCount:         summary.MerchantCatalogEntryCount,
		QuestFlagRewardItemCount:          summary.QuestFlagRewardItemCount,
		QuestFlagConsumeItemCount:         summary.QuestFlagConsumeItemCount,
		StaticActorCount:                  summary.StaticActorCount,
		RewardDropCount:                   summary.RewardDropCount,
		CombatProfileCount:                summary.CombatProfileCount,
		CombatProfileDeathRewardDropCount: summary.CombatProfileDeathRewardDropCount,
		EntityIDs:                         append([]uint64(nil), summary.EntityIDs...),
		InteractionKinds:                  append([]string(nil), summary.InteractionKinds...),
		CombatProfiles:                    append([]string(nil), summary.CombatProfiles...),
	}
	if result.EntityIDs == nil {
		result.EntityIDs = []uint64{}
	}
	if result.InteractionKinds == nil {
		result.InteractionKinds = []string{}
	}
	if result.CombatProfiles == nil {
		result.CombatProfiles = []string{}
	}

	tx, err := executor.BeginTx(ctx, nil)
	if err != nil {
		return StaticActorContentStateImportResult{}, fmt.Errorf("begin static-actor content-state import transaction: %w", err)
	}

	if err := requireStaticActorContentStateSchema(ctx, tx); err != nil {
		return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
	}

	for _, row := range canonical.InteractionDefinitions {
		if err := insertInteractionDefinition(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.MerchantCatalogEntries {
		if err := insertInteractionMerchantCatalogEntry(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.QuestFlagRewardItems {
		if err := insertInteractionQuestFlagRewardItem(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.QuestFlagConsumeItems {
		if err := insertInteractionQuestFlagConsumeItem(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.StaticActors {
		if err := insertStaticActor(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.RewardDrops {
		if err := insertStaticActorRewardDrop(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.CombatProfiles {
		if err := insertStaticActorCombatProfile(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}
	for _, row := range canonical.CombatProfileDeathRewardDrops {
		if err := insertStaticActorCombatProfileDeathRewardDrop(ctx, tx, row); err != nil {
			return StaticActorContentStateImportResult{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return StaticActorContentStateImportResult{}, fmt.Errorf("commit static-actor content-state import transaction: %w", err)
	}
	return result, nil
}

func requireStaticActorContentStateSchema(ctx context.Context, querier dbmigrations.SQLLedgerQuerier) error {
	ledger, err := dbmigrations.ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return fmt.Errorf("%w: read schema_migrations: %v", ErrStaticActorContentStateImportSchemaRequired, err)
	}
	for _, entry := range ledger {
		if entry.Version == StaticActorContentStateMigrationVersion && entry.Name == StaticActorContentStateMigrationName {
			return nil
		}
	}
	latest := 0
	for _, entry := range ledger {
		if entry.Version > latest {
			latest = entry.Version
		}
	}
	return fmt.Errorf("%w: ledger tip %d missing version %d %q", ErrStaticActorContentStateImportSchemaRequired, latest, StaticActorContentStateMigrationVersion, StaticActorContentStateMigrationName)
}

func insertInteractionDefinition(ctx context.Context, tx *sql.Tx, row InteractionDefinitionRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO interaction_definitions (
    kind, ref, text, title, map_index, x, y, size, quest_ref, quest_flag, quest_from, quest_to,
    reward_experience, reward_gold, consume_gold, consume_experience
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Kind,
		row.Ref,
		row.Text,
		row.Title,
		nullableUint32SQL(row.MapIndex),
		nullableInt32SQL(row.X),
		nullableInt32SQL(row.Y),
		int(row.Size),
		row.QuestRef,
		row.QuestFlag,
		int64(row.QuestFrom),
		int64(row.QuestTo),
		int64(row.RewardExperience),
		int64(row.RewardGold),
		int64(row.ConsumeGold),
		int64(row.ConsumeExperience),
	)
	if err != nil {
		return fmt.Errorf("insert interaction definition %s:%s: %w", row.Kind, row.Ref, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert interaction definition", int64(len(row.Kind)+len(row.Ref)))
}

func insertInteractionMerchantCatalogEntry(ctx context.Context, tx *sql.Tx, row InteractionMerchantCatalogEntryRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO interaction_merchant_catalog_entries (
    definition_kind, definition_ref, slot, item_vnum, price, count
) VALUES (?, ?, ?, ?, ?, ?)`,
		row.DefinitionKind,
		row.DefinitionRef,
		int(row.Slot),
		int64(row.ItemVnum),
		int64(row.Price),
		int(row.Count),
	)
	if err != nil {
		return fmt.Errorf("insert merchant catalog entry %s:%s slot %d: %w", row.DefinitionKind, row.DefinitionRef, row.Slot, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert merchant catalog entry", int64(row.Slot))
}

func insertInteractionQuestFlagRewardItem(ctx context.Context, tx *sql.Tx, row InteractionQuestFlagItemRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO interaction_quest_flag_reward_items (
    definition_kind, definition_ref, position, item_vnum, count
) VALUES (?, ?, ?, ?, ?)`,
		row.DefinitionKind,
		row.DefinitionRef,
		int(row.Position),
		int64(row.ItemVnum),
		int(row.Count),
	)
	if err != nil {
		return fmt.Errorf("insert quest-flag reward item %s:%s position %d: %w", row.DefinitionKind, row.DefinitionRef, row.Position, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert quest-flag reward item", int64(row.Position))
}

func insertInteractionQuestFlagConsumeItem(ctx context.Context, tx *sql.Tx, row InteractionQuestFlagItemRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO interaction_quest_flag_consume_items (
    definition_kind, definition_ref, position, item_vnum, count
) VALUES (?, ?, ?, ?, ?)`,
		row.DefinitionKind,
		row.DefinitionRef,
		int(row.Position),
		int64(row.ItemVnum),
		int(row.Count),
	)
	if err != nil {
		return fmt.Errorf("insert quest-flag consume item %s:%s position %d: %w", row.DefinitionKind, row.DefinitionRef, row.Position, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert quest-flag consume item", int64(row.Position))
}

func insertStaticActor(ctx context.Context, tx *sql.Tx, row StaticActorContentStateRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO static_actors (
    entity_id, name, map_index, x, y, race_num,
    spawn_home_map_index, spawn_home_x, spawn_home_y,
    combat_profile, interaction_kind, interaction_ref, spawn_group_ref,
    reward_experience, reward_gold, reward_quest_ref, reward_quest_flag, reward_quest_from, reward_quest_to, reward_quest_text,
    require_quest_ref, require_quest_flag, require_quest_from
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(row.EntityID),
		row.Name,
		int64(row.MapIndex),
		int64(row.X),
		int64(row.Y),
		int64(row.RaceNum),
		nullableUint32SQL(row.SpawnHomeMapIndex),
		nullableInt32SQL(row.SpawnHomeX),
		nullableInt32SQL(row.SpawnHomeY),
		row.CombatProfile,
		nullableNonEmptyStringSQL(row.InteractionKind),
		nullableNonEmptyStringSQL(row.InteractionRef),
		nullableNonEmptyStringSQL(row.SpawnGroupRef),
		int64(row.RewardExperience),
		int64(row.RewardGold),
		row.RewardQuestRef,
		row.RewardQuestFlag,
		int64(row.RewardQuestFrom),
		int64(row.RewardQuestTo),
		row.RewardQuestText,
		row.RequireQuestRef,
		row.RequireQuestFlag,
		int64(row.RequireQuestFrom),
	)
	if err != nil {
		return fmt.Errorf("insert static actor entity_id %d: %w", row.EntityID, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert static actor", int64(row.EntityID))
}

func insertStaticActorRewardDrop(ctx context.Context, tx *sql.Tx, row StaticActorRewardDropRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO static_actor_reward_drops (
    entity_id, position, item_vnum
) VALUES (?, ?, ?)`,
		int64(row.EntityID),
		int(row.Position),
		int64(row.ItemVnum),
	)
	if err != nil {
		return fmt.Errorf("insert static actor reward drop entity_id %d position %d: %w", row.EntityID, row.Position, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert static actor reward drop", int64(row.EntityID)*1000+int64(row.Position))
}

func insertStaticActorCombatProfile(ctx context.Context, tx *sql.Tx, row StaticActorCombatProfileRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO static_actor_combat_profiles (
    profile, max_hp, damage_per_normal_attack, attack_value, defense_value, level, rank,
    respawn_delay_ms, aggro_radius, leash_radius, retaliation_point_delta, death_reward_experience, death_reward_gold
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Profile,
		int(row.MaxHP),
		int(row.DamagePerNormalAttack),
		int(row.AttackValue),
		int(row.DefenseValue),
		int(row.Level),
		int(row.Rank),
		row.RespawnDelayMs,
		int64(row.AggroRadius),
		int64(row.LeashRadius),
		int64(row.RetaliationPointDelta),
		int64(row.DeathRewardExperience),
		int64(row.DeathRewardGold),
	)
	if err != nil {
		return fmt.Errorf("insert static actor combat profile %q: %w", row.Profile, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert static actor combat profile", int64(len(row.Profile)))
}

func insertStaticActorCombatProfileDeathRewardDrop(ctx context.Context, tx *sql.Tx, row StaticActorCombatProfileDropRow) error {
	result, err := tx.ExecContext(ctx, `
INSERT INTO static_actor_combat_profile_death_reward_drops (
    profile, position, item_vnum
) VALUES (?, ?, ?)`,
		row.Profile,
		int(row.Position),
		int64(row.ItemVnum),
	)
	if err != nil {
		return fmt.Errorf("insert combat-profile death-reward drop %q position %d: %w", row.Profile, row.Position, err)
	}
	return requireExactStaticActorContentStateImportRows(result, "insert combat-profile death-reward drop", int64(row.Position))
}

func nullableUint32SQL(value *uint32) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func nullableInt32SQL(value *int32) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func nullableNonEmptyStringSQL(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func requireExactStaticActorContentStateImportRows(result sql.Result, action string, id int64) error {
	if result == nil {
		return fmt.Errorf("%w: %s id %d returned nil result", ErrStaticActorContentStateImportRowCount, action, id)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %s id %d returned unknown row count: %v", ErrStaticActorContentStateImportRowCount, action, id, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%w: %s id %d affected %d rows", ErrStaticActorContentStateImportRowCount, action, id, rowsAffected)
	}
	return nil
}

func rollbackAfterStaticActorContentStateImportFailure(tx *sql.Tx, importErr error) error {
	if tx == nil {
		return importErr
	}
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(importErr, fmt.Errorf("rollback static-actor content-state import transaction: %w", rollbackErr))
	}
	return importErr
}

func staticActorContentStateImportExecutorIsNil(executor dbmigrations.SQLMigrationExecutor) bool {
	if executor == nil {
		return true
	}
	value := reflect.ValueOf(executor)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
