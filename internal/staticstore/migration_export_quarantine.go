package staticstore

import (
	"errors"
	"fmt"
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

// ErrInvalidStaticActorContentStateExport reports that a retained static-actor
// content-state export failed the 0013 migration-shaped quarantine contract.
var ErrInvalidStaticActorContentStateExport = errors.New("invalid static actor content-state export")

// StaticActorContentStateQuarantineSummary is the metadata-only result of
// validating or quarantining a retained static-actor content-state export. It
// never includes SQL, DSNs, or raw JSON store bytes.
type StaticActorContentStateQuarantineSummary struct {
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

// StaticActorContentStateQuarantineResult pairs the metadata-only quarantine
// summary with a canonicalized export ready for later offline review or
// backfill tools.
type StaticActorContentStateQuarantineResult struct {
	Summary StaticActorContentStateQuarantineSummary `json:"summary"`
	Export  StaticActorContentStateExport            `json:"export"`
}

// ValidateStaticActorContentStateExport fails closed when a retained export
// does not match the 0013_static_actor_combat_profile_state shape. It does not
// open a database, write static-actor/interaction snapshots, or mutate the
// supplied export.
func ValidateStaticActorContentStateExport(export StaticActorContentStateExport) (StaticActorContentStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeStaticActorContentStateExport(export)
	if err != nil {
		return StaticActorContentStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineStaticActorContentStateExport validates a retained export and
// returns a canonicalized copy ordered exactly like ExportStaticActorContentState.
// It never opens a database or mutates static-actor/interaction snapshots.
func QuarantineStaticActorContentStateExport(export StaticActorContentStateExport) (StaticActorContentStateExport, StaticActorContentStateQuarantineSummary, error) {
	return canonicalizeStaticActorContentStateExport(export)
}

func canonicalizeStaticActorContentStateExport(export StaticActorContentStateExport) (StaticActorContentStateExport, StaticActorContentStateQuarantineSummary, error) {
	if export.MigrationVersion != StaticActorContentStateMigrationVersion {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidStaticActorContentStateExport, export.MigrationVersion)
	}
	if export.MigrationName != StaticActorContentStateMigrationName {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidStaticActorContentStateExport, export.MigrationName)
	}
	if export.InteractionDefinitions == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: interaction_definitions must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.MerchantCatalogEntries == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: merchant_catalog_entries must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.QuestFlagRewardItems == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: quest_flag_reward_items must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.QuestFlagConsumeItems == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: quest_flag_consume_items must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.StaticActors == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: static_actors must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.RewardDrops == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: reward_drops must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.CombatProfiles == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: combat_profiles must be present", ErrInvalidStaticActorContentStateExport)
	}
	if export.CombatProfileDeathRewardDrops == nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: combat_profile_death_reward_drops must be present", ErrInvalidStaticActorContentStateExport)
	}

	staticSnapshot, interactionSnapshot, err := snapshotsFromStaticActorContentStateExport(export)
	if err != nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, err
	}
	canonical, err := ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
	if err != nil {
		return StaticActorContentStateExport{}, StaticActorContentStateQuarantineSummary{}, fmt.Errorf("%w: %v", ErrInvalidStaticActorContentStateExport, err)
	}

	entityIDs := make([]uint64, 0, len(canonical.StaticActors))
	for _, actor := range canonical.StaticActors {
		entityIDs = append(entityIDs, actor.EntityID)
	}
	kindSet := make(map[string]struct{}, len(canonical.InteractionDefinitions))
	for _, definition := range canonical.InteractionDefinitions {
		kindSet[definition.Kind] = struct{}{}
	}
	kinds := make([]string, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	profileNames := make([]string, 0, len(canonical.CombatProfiles))
	for _, profile := range canonical.CombatProfiles {
		profileNames = append(profileNames, profile.Profile)
	}

	summary := StaticActorContentStateQuarantineSummary{
		InteractionDefinitionCount:        len(canonical.InteractionDefinitions),
		MerchantCatalogEntryCount:         len(canonical.MerchantCatalogEntries),
		QuestFlagRewardItemCount:          len(canonical.QuestFlagRewardItems),
		QuestFlagConsumeItemCount:         len(canonical.QuestFlagConsumeItems),
		StaticActorCount:                  len(canonical.StaticActors),
		RewardDropCount:                   len(canonical.RewardDrops),
		CombatProfileCount:                len(canonical.CombatProfiles),
		CombatProfileDeathRewardDropCount: len(canonical.CombatProfileDeathRewardDrops),
		EntityIDs:                         entityIDs,
		InteractionKinds:                  kinds,
		CombatProfiles:                    profileNames,
	}
	if summary.EntityIDs == nil {
		summary.EntityIDs = []uint64{}
	}
	if summary.InteractionKinds == nil {
		summary.InteractionKinds = []string{}
	}
	if summary.CombatProfiles == nil {
		summary.CombatProfiles = []string{}
	}
	return canonical, summary, nil
}

func snapshotsFromStaticActorContentStateExport(export StaticActorContentStateExport) (Snapshot, interactionstore.Snapshot, error) {
	definitionsByKey := make(map[string]interactionstore.Definition, len(export.InteractionDefinitions))
	definitions := make([]interactionstore.Definition, 0, len(export.InteractionDefinitions))
	for _, row := range export.InteractionDefinitions {
		definition, err := interactionDefinitionFromContentStateRow(row)
		if err != nil {
			return Snapshot{}, interactionstore.Snapshot{}, err
		}
		key := interactionDefinitionExportKey(definition.Kind, definition.Ref)
		if _, exists := definitionsByKey[key]; exists {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate interaction definition %s:%s", ErrInvalidStaticActorContentStateExport, definition.Kind, definition.Ref)
		}
		definitionsByKey[key] = definition
		definitions = append(definitions, definition)
	}

	for _, entry := range export.MerchantCatalogEntries {
		key := interactionDefinitionExportKey(entry.DefinitionKind, entry.DefinitionRef)
		definition, ok := definitionsByKey[key]
		if !ok {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: merchant catalog entry references missing interaction definition %s:%s", ErrInvalidStaticActorContentStateExport, entry.DefinitionKind, entry.DefinitionRef)
		}
		if entry.DefinitionKind != interactionstore.KindShopPreview {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: merchant catalog entry requires shop_preview definition", ErrInvalidStaticActorContentStateExport)
		}
		if entry.Slot >= 40 || entry.ItemVnum == 0 || entry.Price == 0 || entry.Price > 4294967295 || entry.Count == 0 || entry.Count > 255 {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: invalid merchant catalog entry slot=%d item_vnum=%d", ErrInvalidStaticActorContentStateExport, entry.Slot, entry.ItemVnum)
		}
		for _, existing := range definition.Catalog {
			if existing.Slot == entry.Slot {
				return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate merchant catalog slot %d for %s:%s", ErrInvalidStaticActorContentStateExport, entry.Slot, entry.DefinitionKind, entry.DefinitionRef)
			}
		}
		definition.Catalog = append(definition.Catalog, interactionstore.MerchantCatalogEntry{
			Slot:     entry.Slot,
			ItemVnum: entry.ItemVnum,
			Price:    entry.Price,
			Count:    entry.Count,
		})
		definitionsByKey[key] = definition
	}

	if err := attachQuestFlagItems(definitionsByKey, export.QuestFlagRewardItems, true); err != nil {
		return Snapshot{}, interactionstore.Snapshot{}, err
	}
	if err := attachQuestFlagItems(definitionsByKey, export.QuestFlagConsumeItems, false); err != nil {
		return Snapshot{}, interactionstore.Snapshot{}, err
	}

	for i := range definitions {
		key := interactionDefinitionExportKey(definitions[i].Kind, definitions[i].Ref)
		definitions[i] = definitionsByKey[key]
	}

	actorsByID := make(map[uint64]StaticActor, len(export.StaticActors))
	actors := make([]StaticActor, 0, len(export.StaticActors))
	for _, row := range export.StaticActors {
		actor, err := staticActorFromContentStateRow(row)
		if err != nil {
			return Snapshot{}, interactionstore.Snapshot{}, err
		}
		if _, exists := actorsByID[actor.EntityID]; exists {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate static actor entity_id %d", ErrInvalidStaticActorContentStateExport, actor.EntityID)
		}
		actorsByID[actor.EntityID] = actor
		actors = append(actors, actor)
	}

	dropsByEntity := make(map[uint64][]StaticActorRewardDropRow, len(export.RewardDrops))
	for _, drop := range export.RewardDrops {
		if _, ok := actorsByID[drop.EntityID]; !ok {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: reward drop references missing entity_id %d", ErrInvalidStaticActorContentStateExport, drop.EntityID)
		}
		if drop.ItemVnum == 0 {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: reward drop for entity_id %d requires item_vnum > 0", ErrInvalidStaticActorContentStateExport, drop.EntityID)
		}
		dropsByEntity[drop.EntityID] = append(dropsByEntity[drop.EntityID], drop)
	}
	for entityID, drops := range dropsByEntity {
		sort.Slice(drops, func(i, j int) bool { return drops[i].Position < drops[j].Position })
		seenPositions := make(map[uint8]struct{}, len(drops))
		vnums := make([]uint32, 0, len(drops))
		for i, drop := range drops {
			if _, exists := seenPositions[drop.Position]; exists {
				return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate reward drop position %d for entity_id %d", ErrInvalidStaticActorContentStateExport, drop.Position, entityID)
			}
			if int(drop.Position) != i {
				return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: reward drop positions for entity_id %d must be contiguous from 0", ErrInvalidStaticActorContentStateExport, entityID)
			}
			seenPositions[drop.Position] = struct{}{}
			vnums = append(vnums, drop.ItemVnum)
		}
		actor := actorsByID[entityID]
		actor.RewardDropVnums = vnums
		actorsByID[entityID] = actor
	}

	for i := range actors {
		actors[i] = actorsByID[actors[i].EntityID]
	}

	profilesByName := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(export.CombatProfiles))
	profiles := make([]worldruntime.StaticActorCombatProfileSnapshot, 0, len(export.CombatProfiles))
	for _, row := range export.CombatProfiles {
		profile, err := combatProfileFromContentStateRow(row)
		if err != nil {
			return Snapshot{}, interactionstore.Snapshot{}, err
		}
		if _, exists := profilesByName[profile.Profile]; exists {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate combat profile %q", ErrInvalidStaticActorContentStateExport, profile.Profile)
		}
		profilesByName[profile.Profile] = profile
		profiles = append(profiles, profile)
	}

	dropsByProfile := make(map[string][]StaticActorCombatProfileDropRow, len(export.CombatProfileDeathRewardDrops))
	for _, drop := range export.CombatProfileDeathRewardDrops {
		if _, ok := profilesByName[drop.Profile]; !ok {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: combat profile death-reward drop references missing profile %q", ErrInvalidStaticActorContentStateExport, drop.Profile)
		}
		if drop.ItemVnum == 0 {
			return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: combat profile death-reward drop for %q requires item_vnum > 0", ErrInvalidStaticActorContentStateExport, drop.Profile)
		}
		dropsByProfile[drop.Profile] = append(dropsByProfile[drop.Profile], drop)
	}
	for profileName, drops := range dropsByProfile {
		sort.Slice(drops, func(i, j int) bool { return drops[i].Position < drops[j].Position })
		seenPositions := make(map[uint8]struct{}, len(drops))
		seenVnums := make(map[uint32]struct{}, len(drops))
		vnums := make([]uint32, 0, len(drops))
		for i, drop := range drops {
			if _, exists := seenPositions[drop.Position]; exists {
				return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate combat profile death-reward drop position %d for %q", ErrInvalidStaticActorContentStateExport, drop.Position, profileName)
			}
			if int(drop.Position) != i {
				return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: combat profile death-reward drop positions for %q must be contiguous from 0", ErrInvalidStaticActorContentStateExport, profileName)
			}
			if _, exists := seenVnums[drop.ItemVnum]; exists {
				return Snapshot{}, interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate combat profile death-reward drop item_vnum %d for %q", ErrInvalidStaticActorContentStateExport, drop.ItemVnum, profileName)
			}
			seenPositions[drop.Position] = struct{}{}
			seenVnums[drop.ItemVnum] = struct{}{}
			vnums = append(vnums, drop.ItemVnum)
		}
		profile := profilesByName[profileName]
		profile.DeathReward.DropVnums = vnums
		profilesByName[profileName] = profile
	}
	for i := range profiles {
		profiles[i] = profilesByName[profiles[i].Profile]
	}

	return Snapshot{StaticActors: actors, CombatProfiles: profiles}, interactionstore.Snapshot{Definitions: definitions}, nil
}

func attachQuestFlagItems(definitionsByKey map[string]interactionstore.Definition, rows []InteractionQuestFlagItemRow, reward bool) error {
	grouped := make(map[string][]InteractionQuestFlagItemRow)
	for _, row := range rows {
		key := interactionDefinitionExportKey(row.DefinitionKind, row.DefinitionRef)
		definition, ok := definitionsByKey[key]
		if !ok {
			return fmt.Errorf("%w: quest_flag item references missing interaction definition %s:%s", ErrInvalidStaticActorContentStateExport, row.DefinitionKind, row.DefinitionRef)
		}
		if row.DefinitionKind != interactionstore.KindQuestFlag || definition.Kind != interactionstore.KindQuestFlag {
			return fmt.Errorf("%w: quest_flag item requires quest_flag definition", ErrInvalidStaticActorContentStateExport)
		}
		if row.Position >= 8 || row.ItemVnum == 0 || row.Count == 0 || row.Count > 255 {
			return fmt.Errorf("%w: invalid quest_flag item position=%d item_vnum=%d", ErrInvalidStaticActorContentStateExport, row.Position, row.ItemVnum)
		}
		grouped[key] = append(grouped[key], row)
	}
	for key, items := range grouped {
		sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
		entries := make([]interactionstore.RewardItemEntry, 0, len(items))
		seen := make(map[uint8]struct{}, len(items))
		for i, item := range items {
			if _, exists := seen[item.Position]; exists {
				return fmt.Errorf("%w: duplicate quest_flag item position %d for %s", ErrInvalidStaticActorContentStateExport, item.Position, key)
			}
			if int(item.Position) != i {
				return fmt.Errorf("%w: quest_flag item positions for %s must be contiguous from 0", ErrInvalidStaticActorContentStateExport, key)
			}
			seen[item.Position] = struct{}{}
			entries = append(entries, interactionstore.RewardItemEntry{ItemVnum: item.ItemVnum, Count: item.Count})
		}
		definition := definitionsByKey[key]
		if reward {
			definition.RewardItems = entries
		} else {
			definition.ConsumeItems = entries
		}
		definitionsByKey[key] = definition
	}
	return nil
}

func interactionDefinitionFromContentStateRow(row InteractionDefinitionRow) (interactionstore.Definition, error) {
	definition := interactionstore.Definition{
		Kind:              row.Kind,
		Ref:               row.Ref,
		Text:              row.Text,
		Title:             row.Title,
		Size:              row.Size,
		QuestRef:          row.QuestRef,
		QuestFlag:         row.QuestFlag,
		QuestFrom:         row.QuestFrom,
		QuestTo:           row.QuestTo,
		RewardExperience:  row.RewardExperience,
		RewardGold:        row.RewardGold,
		ConsumeGold:       row.ConsumeGold,
		ConsumeExperience: row.ConsumeExperience,
	}
	switch row.Kind {
	case interactionstore.KindInfo, interactionstore.KindTalk:
		if row.MapIndex != nil || row.X != nil || row.Y != nil || row.Title != "" || row.Size != 0 || row.RewardExperience != 0 || row.RewardGold != 0 || row.ConsumeGold != 0 || row.ConsumeExperience != 0 {
			return interactionstore.Definition{}, fmt.Errorf("%w: %s definition %q carries unsupported fields", ErrInvalidStaticActorContentStateExport, row.Kind, row.Ref)
		}
	case interactionstore.KindShopPreview:
		if row.MapIndex != nil || row.X != nil || row.Y != nil || row.Text != "" || row.Size != 0 || row.RewardExperience != 0 || row.RewardGold != 0 || row.ConsumeGold != 0 || row.ConsumeExperience != 0 {
			return interactionstore.Definition{}, fmt.Errorf("%w: shop_preview definition %q carries unsupported fields", ErrInvalidStaticActorContentStateExport, row.Ref)
		}
	case interactionstore.KindWarp:
		if row.Title != "" || row.Size != 0 || row.RewardExperience != 0 || row.RewardGold != 0 || row.ConsumeGold != 0 || row.ConsumeExperience != 0 {
			return interactionstore.Definition{}, fmt.Errorf("%w: warp definition %q carries unsupported fields", ErrInvalidStaticActorContentStateExport, row.Ref)
		}
		if row.MapIndex == nil || row.X == nil || row.Y == nil {
			return interactionstore.Definition{}, fmt.Errorf("%w: warp definition %q requires map_index/x/y", ErrInvalidStaticActorContentStateExport, row.Ref)
		}
		definition.MapIndex = *row.MapIndex
		definition.X = *row.X
		definition.Y = *row.Y
	case interactionstore.KindOpenSafebox:
		if row.MapIndex != nil || row.X != nil || row.Y != nil || row.Title != "" || row.RewardExperience != 0 || row.RewardGold != 0 || row.ConsumeGold != 0 || row.ConsumeExperience != 0 {
			return interactionstore.Definition{}, fmt.Errorf("%w: open_safebox definition %q carries unsupported fields", ErrInvalidStaticActorContentStateExport, row.Ref)
		}
	case interactionstore.KindQuestFlag:
		if row.MapIndex != nil || row.X != nil || row.Y != nil || row.Title != "" || row.Size != 0 {
			return interactionstore.Definition{}, fmt.Errorf("%w: quest_flag definition %q carries unsupported fields", ErrInvalidStaticActorContentStateExport, row.Ref)
		}
	default:
		return interactionstore.Definition{}, fmt.Errorf("%w: unsupported interaction kind %q", ErrInvalidStaticActorContentStateExport, row.Kind)
	}
	return definition, nil
}

func staticActorFromContentStateRow(row StaticActorContentStateRow) (StaticActor, error) {
	actor := StaticActor{
		EntityID:         row.EntityID,
		Name:             row.Name,
		MapIndex:         row.MapIndex,
		X:                row.X,
		Y:                row.Y,
		RaceNum:          row.RaceNum,
		CombatProfile:    row.CombatProfile,
		InteractionKind:  row.InteractionKind,
		InteractionRef:   row.InteractionRef,
		SpawnGroupRef:    row.SpawnGroupRef,
		RewardExperience: row.RewardExperience,
		RewardGold:       row.RewardGold,
		RewardQuestRef:   row.RewardQuestRef,
		RewardQuestFlag:  row.RewardQuestFlag,
		RewardQuestFrom:  row.RewardQuestFrom,
		RewardQuestTo:    row.RewardQuestTo,
		RewardQuestText:  row.RewardQuestText,
		RequireQuestRef:  row.RequireQuestRef,
		RequireQuestFlag: row.RequireQuestFlag,
		RequireQuestFrom: row.RequireQuestFrom,
	}
	homePresent := row.SpawnHomeMapIndex != nil || row.SpawnHomeX != nil || row.SpawnHomeY != nil
	if homePresent {
		if row.SpawnHomeMapIndex == nil || row.SpawnHomeX == nil || row.SpawnHomeY == nil {
			return StaticActor{}, fmt.Errorf("%w: static actor %d has partial spawn home fields", ErrInvalidStaticActorContentStateExport, row.EntityID)
		}
		actor.SpawnHome = &worldruntime.PositionSnapshot{
			MapIndex: *row.SpawnHomeMapIndex,
			X:        *row.SpawnHomeX,
			Y:        *row.SpawnHomeY,
		}
	}
	return actor, nil
}

func combatProfileFromContentStateRow(row StaticActorCombatProfileRow) (worldruntime.StaticActorCombatProfileSnapshot, error) {
	profile := worldruntime.StaticActorCombatProfileSnapshot{
		Profile:               row.Profile,
		MaxHP:                 row.MaxHP,
		DamagePerNormalAttack: row.DamagePerNormalAttack,
		AttackValue:           row.AttackValue,
		DefenseValue:          row.DefenseValue,
		Level:                 row.Level,
		Rank:                  row.Rank,
		RespawnDelayMs:        row.RespawnDelayMs,
		AggroRadius:           row.AggroRadius,
		LeashRadius:           row.LeashRadius,
		RetaliationPointDelta: row.RetaliationPointDelta,
		DeathReward: worldruntime.StaticActorDeathReward{
			Experience: row.DeathRewardExperience,
			Gold:       row.DeathRewardGold,
		},
	}
	if !validCombatProfileSnapshot(profile) {
		return worldruntime.StaticActorCombatProfileSnapshot{}, fmt.Errorf("%w: invalid combat profile %q", ErrInvalidStaticActorContentStateExport, row.Profile)
	}
	return profile, nil
}
