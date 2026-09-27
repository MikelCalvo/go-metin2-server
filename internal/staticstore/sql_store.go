package staticstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
)

// SQLStore is the first live DB-backed static-actor repository. It implements
// Store Load/Save and StaticActorContentStateExporter against the already-owned
// tip-0013 static-actor / interaction / combat-profile tables through a
// caller-supplied database/sql executor. Export identity stays tip-0013.
// Schema preflight requires the ledger entries for version 13 /
// static_actor_combat_profile_state plus additive 0016 chase_delay_ms, 0017
// return_delay_ms, 0018 homeward_delay_ms, 0019 max_step, and 0020
// reaction_delay_ms.
//
// Save replaces the entire authored static-actor snapshot: it deletes every
// tip-0013 static-actor and combat-profile parent and child row, then inserts
// the canonicalized snapshot. An empty snapshot clears those tables.
// Interaction-definition rows are not part of the static-actor snapshot and
// are left untouched. That is the FileStore whole-snapshot posture for actors,
// not insert-only import, not the opt-in per-identity scoped replace, and not
// an upsert.
//
// Tip-0013 has no columns for combat_current_hp, respawn_ready_at, or
// proximity_suppress_vids. A snapshot that carries any of those FileStore-only
// fields fails closed before any SQL mutation so the live repository cannot
// silently drop them. Load and Save never call RegisterStaticActorCombatProfile.
//
// The package does not select a driver, load a DSN, embed secrets, or register
// a production engine. Stock gamed rematerialize stays on FileStore unless a
// caller passes SQLStore into the explicit opt-in selector.
type SQLStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

var _ Store = (*SQLStore)(nil)
var _ StaticActorContentStateExporter = (*SQLStore)(nil)

// NewSQLStore returns an opt-in SQL static-actor repository. A nil executor
// fails closed on Load/Save/Export rather than at construction.
func NewSQLStore(executor dbmigrations.SQLMigrationExecutor) *SQLStore {
	return &SQLStore{executor: executor}
}

// Load projects interaction definitions, static actors, reward drops, combat
// profiles, and death-reward drops onto an authored Snapshot. An empty table
// is an empty snapshot, not a missing FileStore. Schema preflight requires
// tip-0013 plus additive 0016 through 0020. Rows that cannot reconstruct a
// valid authored snapshot fail closed.
func (s *SQLStore) Load() (Snapshot, error) {
	if s == nil || staticActorContentStateImportExecutorIsNil(s.executor) {
		return Snapshot{}, ErrStaticActorContentStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin static-actor SQL load transaction: %w", err)
	}
	snapshot, err := loadStaticActorSQLSnapshot(ctx, tx)
	if err != nil {
		return Snapshot{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit static-actor SQL load transaction: %w", err)
	}
	return snapshot, nil
}

// Save replaces every tip-0013 static-actor and combat-profile row with the
// supplied snapshot inside one transaction. A nil or empty actor slice clears
// those tables. Interaction-definition rows are left untouched: they stay
// owned by the interaction FileStore and by tip-0013 import. FileStore-only
// combat persistence fields are rejected before the transaction opens. Save
// does not register combat profiles.
func (s *SQLStore) Save(snapshot Snapshot) error {
	if s == nil || staticActorContentStateImportExecutorIsNil(s.executor) {
		return ErrStaticActorContentStateImportExecutorRequired
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate static actor snapshot", err)
	}
	if err := rejectFileStoreOnlyStaticActorFields(normalized); err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin static-actor SQL save transaction: %w", err)
	}
	if err := requireStaticActorContentStateSchema(ctx, tx); err != nil {
		return rollbackAfterStaticActorContentStateImportFailure(tx, err)
	}
	if err := replaceStaticActorSQLSnapshot(ctx, tx, normalized); err != nil {
		return rollbackAfterStaticActorContentStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit static-actor SQL save transaction: %w", err)
	}
	return nil
}

// ExportStaticActorContentState projects the committed SQL rows onto the
// tip-0013 migration export. An empty table yields an empty export. The
// interactions argument is ignored: tip-0013 already stores the paired
// interaction definitions beside the static actors.
func (s *SQLStore) ExportStaticActorContentState(interactionstore.Store) (StaticActorContentStateExport, error) {
	if s == nil || staticActorContentStateImportExecutorIsNil(s.executor) {
		return StaticActorContentStateExport{}, ErrStaticActorContentStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("begin static-actor SQL export transaction: %w", err)
	}
	export, err := queryStaticActorContentStateExport(ctx, tx)
	if err != nil {
		return StaticActorContentStateExport{}, rollbackAfterStaticActorContentStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("commit static-actor SQL export transaction: %w", err)
	}
	canonical, _, err := QuarantineStaticActorContentStateExport(export)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	return canonical, nil
}

// SelectRematerializeStore chooses the static-actor Load/Save seam. A nil SQL
// store keeps the FileStore. A non-nil SQL store is the explicit opt-in for
// that one caller. Nil file store with a nil opt-in stays nil. This does not
// select a driver, load a DSN, upsert, auto-run, register combat profiles, or
// replace stock gamed rematerialize.
func SelectRematerializeStore(fileStore Store, sqlStore *SQLStore) Store {
	if sqlStore != nil {
		return sqlStore
	}
	return fileStore
}

func rejectFileStoreOnlyStaticActorFields(snapshot Snapshot) error {
	for _, actor := range snapshot.StaticActors {
		if actor.CombatCurrentHP != nil || (actor.RespawnReadyAt != nil && !actor.RespawnReadyAt.IsZero()) || len(actor.ProximitySuppressVIDs) > 0 {
			return fmt.Errorf("%w: static actor %d combat_current_hp, respawn_ready_at, and proximity_suppress_vids are not tip-0013 columns", ErrInvalidSnapshot, actor.EntityID)
		}
	}
	return nil
}

func replaceStaticActorSQLSnapshot(ctx context.Context, tx *sql.Tx, snapshot Snapshot) error {
	// Child tables declare FOREIGN KEY without ON DELETE CASCADE. Delete reward
	// drops before actors, and death-reward drops before combat profiles. This
	// is the whole static-actor snapshot, not an identity scope. Interaction
	// definitions and their merchant / quest-flag children stay owned by the
	// interaction FileStore and by tip-0013 import, so Save does not delete them.
	statements := []struct {
		table string
		query string
	}{
		{"static_actor_reward_drops", `DELETE FROM static_actor_reward_drops`},
		{"static_actors", `DELETE FROM static_actors`},
		{"static_actor_combat_profile_death_reward_drops", `DELETE FROM static_actor_combat_profile_death_reward_drops`},
		{"static_actor_combat_profiles", `DELETE FROM static_actor_combat_profiles`},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query); err != nil {
			return fmt.Errorf("delete %s: %w", statement.table, err)
		}
	}
	export, err := ExportStaticActorContentState(snapshot, interactionstore.Snapshot{})
	if err != nil {
		return err
	}
	for _, row := range export.StaticActors {
		if err := insertStaticActor(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.RewardDrops {
		if err := insertStaticActorRewardDrop(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.CombatProfiles {
		if err := insertStaticActorCombatProfile(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.CombatProfileDeathRewardDrops {
		if err := insertStaticActorCombatProfileDeathRewardDrop(ctx, tx, row); err != nil {
			return err
		}
	}
	return nil
}

func loadStaticActorSQLSnapshot(ctx context.Context, tx *sql.Tx) (Snapshot, error) {
	export, err := queryStaticActorContentStateExport(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	canonical, _, err := QuarantineStaticActorContentStateExport(export)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, _, err := snapshotsFromStaticActorContentStateExport(canonical)
	if err != nil {
		return Snapshot{}, err
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate loaded static actor snapshot", err)
	}
	return normalized, nil
}

func queryStaticActorContentStateExport(ctx context.Context, tx *sql.Tx) (StaticActorContentStateExport, error) {
	if err := requireStaticActorContentStateSchema(ctx, tx); err != nil {
		return StaticActorContentStateExport{}, err
	}
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

	definitionRows, err := tx.QueryContext(ctx, `
SELECT kind, ref, text, title, map_index, x, y, size, quest_ref, quest_flag, quest_from, quest_to,
       reward_experience, reward_gold, consume_gold, consume_experience
FROM interaction_definitions
ORDER BY kind ASC, ref ASC`)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("query interaction definitions: %w", err)
	}
	defer definitionRows.Close()
	for definitionRows.Next() {
		row, err := scanInteractionDefinitionRow(definitionRows)
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		export.InteractionDefinitions = append(export.InteractionDefinitions, row)
	}
	if err := definitionRows.Err(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("iterate interaction definitions: %w", err)
	}

	catalogRows, err := tx.QueryContext(ctx, `
SELECT definition_kind, definition_ref, slot, item_vnum, price, count
FROM interaction_merchant_catalog_entries
ORDER BY definition_kind ASC, definition_ref ASC, slot ASC`)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("query merchant catalog entries: %w", err)
	}
	defer catalogRows.Close()
	for catalogRows.Next() {
		var (
			kind, ref       string
			slot, count     int64
			itemVnum, price int64
		)
		if err := catalogRows.Scan(&kind, &ref, &slot, &itemVnum, &price, &count); err != nil {
			return StaticActorContentStateExport{}, fmt.Errorf("scan merchant catalog entry: %w", err)
		}
		rowSlot, err := sqlStaticActorUint16(slot, "merchant catalog slot")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		rowVnum, err := sqlStaticActorUint32(itemVnum, "merchant catalog item_vnum")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		rowPrice, err := sqlStaticActorUint64(price, "merchant catalog price")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		rowCount, err := sqlStaticActorUint16(count, "merchant catalog count")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		export.MerchantCatalogEntries = append(export.MerchantCatalogEntries, InteractionMerchantCatalogEntryRow{
			DefinitionKind: kind,
			DefinitionRef:  ref,
			Slot:           rowSlot,
			ItemVnum:       rowVnum,
			Price:          rowPrice,
			Count:          rowCount,
		})
	}
	if err := catalogRows.Err(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("iterate merchant catalog entries: %w", err)
	}

	rewardItems, err := queryQuestFlagItemRows(ctx, tx, "interaction_quest_flag_reward_items", "quest-flag reward item")
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	export.QuestFlagRewardItems = rewardItems
	consumeItems, err := queryQuestFlagItemRows(ctx, tx, "interaction_quest_flag_consume_items", "quest-flag consume item")
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	export.QuestFlagConsumeItems = consumeItems

	actorRows, err := tx.QueryContext(ctx, `
SELECT entity_id, name, map_index, x, y, race_num,
       spawn_home_map_index, spawn_home_x, spawn_home_y,
       combat_profile, interaction_kind, interaction_ref, spawn_group_ref,
       reward_experience, reward_gold, reward_quest_ref, reward_quest_flag, reward_quest_from, reward_quest_to, reward_quest_text,
       require_quest_ref, require_quest_flag, require_quest_from
FROM static_actors
ORDER BY name ASC, entity_id ASC`)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("query static actors: %w", err)
	}
	defer actorRows.Close()
	for actorRows.Next() {
		row, err := scanStaticActorContentStateRow(actorRows)
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		export.StaticActors = append(export.StaticActors, row)
	}
	if err := actorRows.Err(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("iterate static actors: %w", err)
	}

	dropRows, err := tx.QueryContext(ctx, `
SELECT entity_id, position, item_vnum FROM static_actor_reward_drops
ORDER BY entity_id ASC, position ASC`)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("query static actor reward drops: %w", err)
	}
	defer dropRows.Close()
	for dropRows.Next() {
		var entityID, position, itemVnum int64
		if err := dropRows.Scan(&entityID, &position, &itemVnum); err != nil {
			return StaticActorContentStateExport{}, fmt.Errorf("scan static actor reward drop: %w", err)
		}
		rowEntity, err := sqlStaticActorUint64(entityID, "reward drop entity_id")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		rowPosition, err := sqlStaticActorUint8(position, "reward drop position")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		rowVnum, err := sqlStaticActorUint32(itemVnum, "reward drop item_vnum")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		export.RewardDrops = append(export.RewardDrops, StaticActorRewardDropRow{
			EntityID: rowEntity,
			Position: rowPosition,
			ItemVnum: rowVnum,
		})
	}
	if err := dropRows.Err(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("iterate static actor reward drops: %w", err)
	}

	profileRows, err := tx.QueryContext(ctx, `
SELECT profile, max_hp, damage_per_normal_attack, attack_value, defense_value, level, rank,
       respawn_delay_ms, aggro_radius, leash_radius, chase_delay_ms, return_delay_ms, homeward_delay_ms,
       max_step, reaction_delay_ms, retaliation_point_delta, death_reward_experience, death_reward_gold
FROM static_actor_combat_profiles
ORDER BY profile ASC`)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("query static actor combat profiles: %w", err)
	}
	defer profileRows.Close()
	for profileRows.Next() {
		row, err := scanStaticActorCombatProfileRow(profileRows)
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		export.CombatProfiles = append(export.CombatProfiles, row)
	}
	if err := profileRows.Err(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("iterate static actor combat profiles: %w", err)
	}

	profileDropRows, err := tx.QueryContext(ctx, `
SELECT profile, position, item_vnum FROM static_actor_combat_profile_death_reward_drops
ORDER BY profile ASC, position ASC`)
	if err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("query combat-profile death-reward drops: %w", err)
	}
	defer profileDropRows.Close()
	for profileDropRows.Next() {
		var (
			profile            string
			position, itemVnum int64
		)
		if err := profileDropRows.Scan(&profile, &position, &itemVnum); err != nil {
			return StaticActorContentStateExport{}, fmt.Errorf("scan combat-profile death-reward drop: %w", err)
		}
		rowPosition, err := sqlStaticActorUint8(position, "combat-profile drop position")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		rowVnum, err := sqlStaticActorUint32(itemVnum, "combat-profile drop item_vnum")
		if err != nil {
			return StaticActorContentStateExport{}, err
		}
		export.CombatProfileDeathRewardDrops = append(export.CombatProfileDeathRewardDrops, StaticActorCombatProfileDropRow{
			Profile:  profile,
			Position: rowPosition,
			ItemVnum: rowVnum,
		})
	}
	if err := profileDropRows.Err(); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("iterate combat-profile death-reward drops: %w", err)
	}
	return export, nil
}

func queryQuestFlagItemRows(ctx context.Context, tx *sql.Tx, table string, label string) ([]InteractionQuestFlagItemRow, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT definition_kind, definition_ref, position, item_vnum, count
FROM `+table+`
ORDER BY definition_kind ASC, definition_ref ASC, position ASC`)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", label, err)
	}
	defer rows.Close()
	items := []InteractionQuestFlagItemRow{}
	for rows.Next() {
		var (
			kind, ref             string
			position, count, vnum int64
		)
		if err := rows.Scan(&kind, &ref, &position, &vnum, &count); err != nil {
			return nil, fmt.Errorf("scan %s: %w", label, err)
		}
		rowPosition, err := sqlStaticActorUint8(position, label+" position")
		if err != nil {
			return nil, err
		}
		rowVnum, err := sqlStaticActorUint32(vnum, label+" item_vnum")
		if err != nil {
			return nil, err
		}
		rowCount, err := sqlStaticActorUint16(count, label+" count")
		if err != nil {
			return nil, err
		}
		items = append(items, InteractionQuestFlagItemRow{
			DefinitionKind: kind,
			DefinitionRef:  ref,
			Position:       rowPosition,
			ItemVnum:       rowVnum,
			Count:          rowCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", label, err)
	}
	return items, nil
}

type staticActorSQLScanner interface {
	Scan(dest ...any) error
}

func scanInteractionDefinitionRow(scanner staticActorSQLScanner) (InteractionDefinitionRow, error) {
	var (
		kind, ref, text, title, questRef, questFlag string
		mapIndex, x, y                              sql.NullInt64
		size                                        int64
		questFrom, questTo                          int64
		rewardExperience, rewardGold                int64
		consumeGold, consumeExperience              int64
	)
	if err := scanner.Scan(
		&kind, &ref, &text, &title, &mapIndex, &x, &y, &size, &questRef, &questFlag, &questFrom, &questTo,
		&rewardExperience, &rewardGold, &consumeGold, &consumeExperience,
	); err != nil {
		return InteractionDefinitionRow{}, fmt.Errorf("scan interaction definition: %w", err)
	}
	rowSize, err := sqlStaticActorUint8(size, "interaction size")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowQuestFrom, err := sqlStaticActorUint32(questFrom, "interaction quest_from")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowQuestTo, err := sqlStaticActorUint32(questTo, "interaction quest_to")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowRewardExperience, err := sqlStaticActorUint64(rewardExperience, "interaction reward_experience")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowRewardGold, err := sqlStaticActorUint64(rewardGold, "interaction reward_gold")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowConsumeGold, err := sqlStaticActorUint64(consumeGold, "interaction consume_gold")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowConsumeExperience, err := sqlStaticActorUint64(consumeExperience, "interaction consume_experience")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowMapIndex, err := nullableSQLUint32(mapIndex, "interaction map_index")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowX, err := nullableSQLInt32(x, "interaction x")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	rowY, err := nullableSQLInt32(y, "interaction y")
	if err != nil {
		return InteractionDefinitionRow{}, err
	}
	return InteractionDefinitionRow{
		Kind:              kind,
		Ref:               ref,
		Text:              text,
		Title:             title,
		MapIndex:          rowMapIndex,
		X:                 rowX,
		Y:                 rowY,
		Size:              rowSize,
		QuestRef:          questRef,
		QuestFlag:         questFlag,
		QuestFrom:         rowQuestFrom,
		QuestTo:           rowQuestTo,
		RewardExperience:  rowRewardExperience,
		RewardGold:        rowRewardGold,
		ConsumeGold:       rowConsumeGold,
		ConsumeExperience: rowConsumeExperience,
	}, nil
}

func scanStaticActorContentStateRow(scanner staticActorSQLScanner) (StaticActorContentStateRow, error) {
	var (
		entityID, mapIndex, x, y, raceNum                       int64
		name, combatProfile, rewardQuestRef, rewardQuestFlag    string
		rewardQuestText, requireQuestRef, requireQuestFlag      string
		spawnHomeMap, spawnHomeX, spawnHomeY                    sql.NullInt64
		interactionKind, interactionRef, spawnGroupRef          sql.NullString
		rewardExperience, rewardGold, rewardQuestFrom, rewardTo int64
		requireQuestFrom                                        int64
	)
	if err := scanner.Scan(
		&entityID, &name, &mapIndex, &x, &y, &raceNum,
		&spawnHomeMap, &spawnHomeX, &spawnHomeY,
		&combatProfile, &interactionKind, &interactionRef, &spawnGroupRef,
		&rewardExperience, &rewardGold, &rewardQuestRef, &rewardQuestFlag, &rewardQuestFrom, &rewardTo, &rewardQuestText,
		&requireQuestRef, &requireQuestFlag, &requireQuestFrom,
	); err != nil {
		return StaticActorContentStateRow{}, fmt.Errorf("scan static actor: %w", err)
	}
	rowEntity, err := sqlStaticActorUint64(entityID, "static actor entity_id")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowMap, err := sqlStaticActorUint32(mapIndex, "static actor map_index")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowX, err := sqlStaticActorInt32(x, "static actor x")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowY, err := sqlStaticActorInt32(y, "static actor y")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowRace, err := sqlStaticActorUint32(raceNum, "static actor race_num")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowHomeMap, err := nullableSQLUint32(spawnHomeMap, "spawn_home_map_index")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowHomeX, err := nullableSQLInt32(spawnHomeX, "spawn_home_x")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowHomeY, err := nullableSQLInt32(spawnHomeY, "spawn_home_y")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowRewardExperience, err := sqlStaticActorUint64(rewardExperience, "static actor reward_experience")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowRewardGold, err := sqlStaticActorUint64(rewardGold, "static actor reward_gold")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowRewardFrom, err := sqlStaticActorUint32(rewardQuestFrom, "static actor reward_quest_from")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowRewardTo, err := sqlStaticActorUint32(rewardTo, "static actor reward_quest_to")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	rowRequireFrom, err := sqlStaticActorUint32(requireQuestFrom, "static actor require_quest_from")
	if err != nil {
		return StaticActorContentStateRow{}, err
	}
	return StaticActorContentStateRow{
		EntityID:          rowEntity,
		Name:              name,
		MapIndex:          rowMap,
		X:                 rowX,
		Y:                 rowY,
		RaceNum:           rowRace,
		SpawnHomeMapIndex: rowHomeMap,
		SpawnHomeX:        rowHomeX,
		SpawnHomeY:        rowHomeY,
		CombatProfile:     combatProfile,
		InteractionKind:   nullStringValue(interactionKind),
		InteractionRef:    nullStringValue(interactionRef),
		SpawnGroupRef:     nullStringValue(spawnGroupRef),
		RewardExperience:  rowRewardExperience,
		RewardGold:        rowRewardGold,
		RewardQuestRef:    rewardQuestRef,
		RewardQuestFlag:   rewardQuestFlag,
		RewardQuestFrom:   rowRewardFrom,
		RewardQuestTo:     rowRewardTo,
		RewardQuestText:   rewardQuestText,
		RequireQuestRef:   requireQuestRef,
		RequireQuestFlag:  requireQuestFlag,
		RequireQuestFrom:  rowRequireFrom,
	}, nil
}

func scanStaticActorCombatProfileRow(scanner staticActorSQLScanner) (StaticActorCombatProfileRow, error) {
	var (
		profile                                                                string
		maxHP, damage, attack, defense, level, rank                            int64
		respawn, aggro, leash, chase, returnDelay, homeward, maxStep, reaction int64
		retaliation, deathExperience, deathGold                                int64
	)
	if err := scanner.Scan(
		&profile, &maxHP, &damage, &attack, &defense, &level, &rank,
		&respawn, &aggro, &leash, &chase, &returnDelay, &homeward,
		&maxStep, &reaction, &retaliation, &deathExperience, &deathGold,
	); err != nil {
		return StaticActorCombatProfileRow{}, fmt.Errorf("scan static actor combat profile: %w", err)
	}
	rowMaxHP, err := sqlStaticActorUint8(maxHP, "combat profile max_hp")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowDamage, err := sqlStaticActorUint8(damage, "combat profile damage_per_normal_attack")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowAttack, err := sqlStaticActorUint16(attack, "combat profile attack_value")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowDefense, err := sqlStaticActorUint16(defense, "combat profile defense_value")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowLevel, err := sqlStaticActorUint16(level, "combat profile level")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowRank, err := sqlStaticActorUint8(rank, "combat profile rank")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowAggro, err := sqlStaticActorInt32(aggro, "combat profile aggro_radius")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowLeash, err := sqlStaticActorInt32(leash, "combat profile leash_radius")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowMaxStep, err := sqlStaticActorInt32(maxStep, "combat profile max_step")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowRetaliation, err := sqlStaticActorInt32(retaliation, "combat profile retaliation_point_delta")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowDeathExperience, err := sqlStaticActorUint64(deathExperience, "combat profile death_reward_experience")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	rowDeathGold, err := sqlStaticActorUint64(deathGold, "combat profile death_reward_gold")
	if err != nil {
		return StaticActorCombatProfileRow{}, err
	}
	return StaticActorCombatProfileRow{
		Profile:               profile,
		MaxHP:                 rowMaxHP,
		DamagePerNormalAttack: rowDamage,
		AttackValue:           rowAttack,
		DefenseValue:          rowDefense,
		Level:                 rowLevel,
		Rank:                  rowRank,
		RespawnDelayMs:        respawn,
		AggroRadius:           rowAggro,
		LeashRadius:           rowLeash,
		ChaseDelayMs:          chase,
		ReturnDelayMs:         returnDelay,
		HomewardDelayMs:       homeward,
		MaxStep:               rowMaxStep,
		ReactionDelayMs:       reaction,
		RetaliationPointDelta: rowRetaliation,
		DeathRewardExperience: rowDeathExperience,
		DeathRewardGold:       rowDeathGold,
	}, nil
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableSQLUint32(value sql.NullInt64, name string) (*uint32, error) {
	if !value.Valid {
		return nil, nil
	}
	decoded, err := sqlStaticActorUint32(value.Int64, name)
	if err != nil {
		return nil, err
	}
	return &decoded, nil
}

func nullableSQLInt32(value sql.NullInt64, name string) (*int32, error) {
	if !value.Valid {
		return nil, nil
	}
	decoded, err := sqlStaticActorInt32(value.Int64, name)
	if err != nil {
		return nil, err
	}
	return &decoded, nil
}

func sqlStaticActorUint64(value int64, name string) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("%w: %s %d is negative", ErrInvalidSnapshot, name, value)
	}
	return uint64(value), nil
}

func sqlStaticActorUint32(value int64, name string) (uint32, error) {
	if value < 0 || value > int64(math.MaxUint32) {
		return 0, fmt.Errorf("%w: %s %d is outside uint32", ErrInvalidSnapshot, name, value)
	}
	return uint32(value), nil
}

func sqlStaticActorUint16(value int64, name string) (uint16, error) {
	if value < 0 || value > int64(math.MaxUint16) {
		return 0, fmt.Errorf("%w: %s %d is outside uint16", ErrInvalidSnapshot, name, value)
	}
	return uint16(value), nil
}

func sqlStaticActorUint8(value int64, name string) (uint8, error) {
	if value < 0 || value > int64(math.MaxUint8) {
		return 0, fmt.Errorf("%w: %s %d is outside uint8", ErrInvalidSnapshot, name, value)
	}
	return uint8(value), nil
}

func sqlStaticActorInt32(value int64, name string) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s %d is outside int32", ErrInvalidSnapshot, name, value)
	}
	return int32(value), nil
}
