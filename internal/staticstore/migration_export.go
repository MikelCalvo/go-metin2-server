package staticstore

import (
	"errors"
	"fmt"
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	StaticActorContentStateMigrationVersion = 13
	StaticActorContentStateMigrationName    = "static_actor_combat_profile_state"

	// StaticActorCombatProfileChaseDelayMigrationVersion / Name pin the additive
	// 0016 column that ImportStaticActorContentState writes beside tip-0013 rows.
	// Export identity stays tip-0013; SQL import preflight requires tip-0013 plus
	// additive chase-delay, return-delay, homeward-delay, and max-step columns.
	StaticActorCombatProfileChaseDelayMigrationVersion = 16
	StaticActorCombatProfileChaseDelayMigrationName    = "static_actor_combat_profile_chase_delay"

	// StaticActorCombatProfileReturnDelayMigrationVersion / Name pin additive
	// 0017 return_delay_ms that ImportStaticActorContentState also inserts.
	StaticActorCombatProfileReturnDelayMigrationVersion = 17
	StaticActorCombatProfileReturnDelayMigrationName    = "static_actor_combat_profile_return_delay"

	// StaticActorCombatProfileHomewardDelayMigrationVersion / Name pin additive
	// 0018 homeward_delay_ms that ImportStaticActorContentState also inserts.
	StaticActorCombatProfileHomewardDelayMigrationVersion = 18
	StaticActorCombatProfileHomewardDelayMigrationName    = "static_actor_combat_profile_homeward_delay"

	// StaticActorCombatProfileMaxStepMigrationVersion / Name pin additive
	// 0019 max_step that ImportStaticActorContentState also inserts.
	StaticActorCombatProfileMaxStepMigrationVersion = 19
	StaticActorCombatProfileMaxStepMigrationName    = "static_actor_combat_profile_max_step"
)

// StaticActorContentStateExport is a deterministic, schema-shaped projection of
// committed bootstrap static-actor and interaction-definition snapshots onto the
// 0013_static_actor_combat_profile_state migration boundary (after the
// historical 0008 / 0012 static-actor content tips). It is intentionally a
// data-model/export contract only: it does not open a database, emit SQL, apply
// migrations, or mutate the file stores.
type StaticActorContentStateExport struct {
	MigrationVersion              int                                  `json:"migration_version"`
	MigrationName                 string                               `json:"migration_name"`
	InteractionDefinitions        []InteractionDefinitionRow           `json:"interaction_definitions"`
	MerchantCatalogEntries        []InteractionMerchantCatalogEntryRow `json:"merchant_catalog_entries"`
	QuestFlagRewardItems          []InteractionQuestFlagItemRow        `json:"quest_flag_reward_items"`
	QuestFlagConsumeItems         []InteractionQuestFlagItemRow        `json:"quest_flag_consume_items"`
	StaticActors                  []StaticActorContentStateRow         `json:"static_actors"`
	RewardDrops                   []StaticActorRewardDropRow           `json:"reward_drops"`
	CombatProfiles                []StaticActorCombatProfileRow        `json:"combat_profiles"`
	CombatProfileDeathRewardDrops []StaticActorCombatProfileDropRow    `json:"combat_profile_death_reward_drops"`
}

// InteractionDefinitionRow mirrors the interaction_definitions table columns
// frozen by the 0012_static_actor_pve_interaction_state migration, excluding
// timestamps that are database-owned at insert/update time.
type InteractionDefinitionRow struct {
	Kind              string  `json:"kind"`
	Ref               string  `json:"ref"`
	Text              string  `json:"text,omitempty"`
	Title             string  `json:"title,omitempty"`
	MapIndex          *uint32 `json:"map_index,omitempty"`
	X                 *int32  `json:"x,omitempty"`
	Y                 *int32  `json:"y,omitempty"`
	Size              uint8   `json:"size,omitempty"`
	QuestRef          string  `json:"quest_ref,omitempty"`
	QuestFlag         string  `json:"quest_flag,omitempty"`
	QuestFrom         uint32  `json:"quest_from,omitempty"`
	QuestTo           uint32  `json:"quest_to,omitempty"`
	RewardExperience  uint64  `json:"reward_experience,omitempty"`
	RewardGold        uint64  `json:"reward_gold,omitempty"`
	ConsumeGold       uint64  `json:"consume_gold,omitempty"`
	ConsumeExperience uint64  `json:"consume_experience,omitempty"`
}

// InteractionMerchantCatalogEntryRow mirrors shop-preview child rows for the
// interaction_merchant_catalog_entries table frozen by migration 0008 / 0012.
type InteractionMerchantCatalogEntryRow struct {
	DefinitionKind string `json:"definition_kind"`
	DefinitionRef  string `json:"definition_ref"`
	Slot           uint16 `json:"slot"`
	ItemVnum       uint32 `json:"item_vnum"`
	Price          uint64 `json:"price"`
	Count          uint16 `json:"count"`
}

// InteractionQuestFlagItemRow mirrors ordered quest_flag reward/consume item
// child rows frozen by migration 0012.
type InteractionQuestFlagItemRow struct {
	DefinitionKind string `json:"definition_kind"`
	DefinitionRef  string `json:"definition_ref"`
	Position       uint8  `json:"position"`
	ItemVnum       uint32 `json:"item_vnum"`
	Count          uint16 `json:"count"`
}

// StaticActorContentStateRow mirrors the static_actors table columns frozen by
// migration 0012, excluding timestamps that are database-owned at insert/update
// time.
type StaticActorContentStateRow struct {
	EntityID          uint64  `json:"entity_id"`
	Name              string  `json:"name"`
	MapIndex          uint32  `json:"map_index"`
	X                 int32   `json:"x"`
	Y                 int32   `json:"y"`
	RaceNum           uint32  `json:"race_num"`
	SpawnHomeMapIndex *uint32 `json:"spawn_home_map_index,omitempty"`
	SpawnHomeX        *int32  `json:"spawn_home_x,omitempty"`
	SpawnHomeY        *int32  `json:"spawn_home_y,omitempty"`
	CombatProfile     string  `json:"combat_profile,omitempty"`
	InteractionKind   string  `json:"interaction_kind,omitempty"`
	InteractionRef    string  `json:"interaction_ref,omitempty"`
	SpawnGroupRef     string  `json:"spawn_group_ref,omitempty"`
	RewardExperience  uint64  `json:"reward_experience,omitempty"`
	RewardGold        uint64  `json:"reward_gold,omitempty"`
	RewardQuestRef    string  `json:"reward_quest_ref,omitempty"`
	RewardQuestFlag   string  `json:"reward_quest_flag,omitempty"`
	RewardQuestFrom   uint32  `json:"reward_quest_from,omitempty"`
	RewardQuestTo     uint32  `json:"reward_quest_to,omitempty"`
	RewardQuestText   string  `json:"reward_quest_text,omitempty"`
	RequireQuestRef   string  `json:"require_quest_ref,omitempty"`
	RequireQuestFlag  string  `json:"require_quest_flag,omitempty"`
	RequireQuestFrom  uint32  `json:"require_quest_from,omitempty"`
}

// StaticActorRewardDropRow mirrors ordered static_actor_reward_drops rows frozen
// by migration 0008 / 0012 / 0013.
type StaticActorRewardDropRow struct {
	EntityID uint64 `json:"entity_id"`
	Position uint8  `json:"position"`
	ItemVnum uint32 `json:"item_vnum"`
}

// StaticActorCombatProfileRow mirrors portable combat-profile parent rows frozen
// by migration 0013_static_actor_combat_profile_state, excluding timestamps that
// are database-owned at insert/update time.
type StaticActorCombatProfileRow struct {
	Profile               string `json:"profile"`
	MaxHP                 uint8  `json:"max_hp"`
	DamagePerNormalAttack uint8  `json:"damage_per_normal_attack"`
	AttackValue           uint16 `json:"attack_value"`
	DefenseValue          uint16 `json:"defense_value"`
	Level                 uint16 `json:"level"`
	Rank                  uint8  `json:"rank"`
	RespawnDelayMs        int64  `json:"respawn_delay_ms"`
	AggroRadius           int32  `json:"aggro_radius"`
	LeashRadius           int32  `json:"leash_radius"`
	ChaseDelayMs          int64  `json:"chase_delay_ms"`
	ReturnDelayMs         int64  `json:"return_delay_ms"`
	HomewardDelayMs       int64  `json:"homeward_delay_ms"`
	MaxStep               int32  `json:"max_step"`
	RetaliationPointDelta int32  `json:"retaliation_point_delta"`
	DeathRewardExperience uint64 `json:"death_reward_experience"`
	DeathRewardGold       uint64 `json:"death_reward_gold"`
}

// StaticActorCombatProfileDropRow mirrors ordered portable combat-profile
// death-reward drop child rows frozen by migration 0013.
type StaticActorCombatProfileDropRow struct {
	Profile  string `json:"profile"`
	Position uint8  `json:"position"`
	ItemVnum uint32 `json:"item_vnum"`
}

// ExportStaticActorContentState validates bootstrap static actor and interaction
// snapshots and returns rows ordered exactly as a future backfill/import tool
// should process them: interaction definitions by kind/ref, merchant catalog
// entries by definition/slot, quest-flag item tables by definition/position,
// static actors by name/entity id, reward drops by actor/position, and portable
// combat profiles / death-reward drops by profile name / position. All
// validation fails closed against the 0013 migration constraints so malformed
// bootstrap JSON cannot be silently coerced into a future database import.
func ExportStaticActorContentState(staticSnapshot Snapshot, interactionSnapshot interactionstore.Snapshot) (StaticActorContentStateExport, error) {
	normalizedInteractions, err := normalizedInteractionDefinitionsForExport(interactionSnapshot)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}

	normalizedStaticActors := normalizeSnapshot(staticSnapshot)
	if err := validateSnapshot(normalizedStaticActors); err != nil {
		return StaticActorContentStateExport{}, fmt.Errorf("%w: validate static actor content-state export", err)
	}

	definitionKeys := make(map[string]struct{}, len(normalizedInteractions.Definitions))
	for _, definition := range normalizedInteractions.Definitions {
		definitionKeys[interactionDefinitionExportKey(definition.Kind, definition.Ref)] = struct{}{}
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

	for _, definition := range normalizedInteractions.Definitions {
		export.InteractionDefinitions = append(export.InteractionDefinitions, interactionDefinitionRowForExport(definition))
		for _, entry := range definition.Catalog {
			export.MerchantCatalogEntries = append(export.MerchantCatalogEntries, InteractionMerchantCatalogEntryRow{
				DefinitionKind: definition.Kind,
				DefinitionRef:  definition.Ref,
				Slot:           entry.Slot,
				ItemVnum:       entry.ItemVnum,
				Price:          entry.Price,
				Count:          entry.Count,
			})
		}
		for i, entry := range interactionstore.EffectiveRewardItems(definition) {
			export.QuestFlagRewardItems = append(export.QuestFlagRewardItems, InteractionQuestFlagItemRow{
				DefinitionKind: definition.Kind,
				DefinitionRef:  definition.Ref,
				Position:       uint8(i),
				ItemVnum:       entry.ItemVnum,
				Count:          entry.Count,
			})
		}
		for i, entry := range interactionstore.EffectiveConsumeItems(definition) {
			export.QuestFlagConsumeItems = append(export.QuestFlagConsumeItems, InteractionQuestFlagItemRow{
				DefinitionKind: definition.Kind,
				DefinitionRef:  definition.Ref,
				Position:       uint8(i),
				ItemVnum:       entry.ItemVnum,
				Count:          entry.Count,
			})
		}
	}

	for _, actor := range normalizedStaticActors.StaticActors {
		if actor.InteractionKind != "" || actor.InteractionRef != "" {
			if _, ok := definitionKeys[interactionDefinitionExportKey(actor.InteractionKind, actor.InteractionRef)]; !ok {
				return StaticActorContentStateExport{}, fmt.Errorf("%w: static actor %d references missing interaction definition %s:%s", ErrInvalidSnapshot, actor.EntityID, actor.InteractionKind, actor.InteractionRef)
			}
		}
		if len(actor.RewardDropVnums) > 255 {
			return StaticActorContentStateExport{}, fmt.Errorf("%w: static actor %d has %d reward drops; migration supports 255", ErrInvalidSnapshot, actor.EntityID, len(actor.RewardDropVnums))
		}
		export.StaticActors = append(export.StaticActors, staticActorContentStateRowForExport(actor))
		for i, vnum := range actor.RewardDropVnums {
			export.RewardDrops = append(export.RewardDrops, StaticActorRewardDropRow{EntityID: actor.EntityID, Position: uint8(i), ItemVnum: vnum})
		}
	}

	for _, profile := range normalizedStaticActors.CombatProfiles {
		if len(profile.DeathReward.DropVnums) > 255 {
			return StaticActorContentStateExport{}, fmt.Errorf("%w: combat profile %q has %d death-reward drops; migration supports 255", ErrInvalidSnapshot, profile.Profile, len(profile.DeathReward.DropVnums))
		}
		export.CombatProfiles = append(export.CombatProfiles, staticActorCombatProfileRowForExport(profile))
		for i, vnum := range profile.DeathReward.DropVnums {
			export.CombatProfileDeathRewardDrops = append(export.CombatProfileDeathRewardDrops, StaticActorCombatProfileDropRow{
				Profile:  profile.Profile,
				Position: uint8(i),
				ItemVnum: vnum,
			})
		}
	}

	return export, nil
}

// ExportStaticActorContentStateFromStores validates and projects the committed
// file-store snapshots onto the 0013 static-actor combat-profile content-state
// migration shape. It reads the same committed snapshot sets as Load and
// applies no mutations.
func ExportStaticActorContentStateFromStores(staticActors Store, interactions interactionstore.Store) (StaticActorContentStateExport, error) {
	staticSnapshot, err := loadStaticActorSnapshotForExport(staticActors)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	interactionSnapshot, err := loadInteractionSnapshotForExport(interactions)
	if err != nil {
		return StaticActorContentStateExport{}, err
	}
	return ExportStaticActorContentState(staticSnapshot, interactionSnapshot)
}

// ExportStaticActorContentState projects this FileStore's committed snapshot
// onto the 0013 static-actor combat-profile content-state migration shape,
// reading the paired interaction store through the shared FromStores helper.
func (s *FileStore) ExportStaticActorContentState(interactions interactionstore.Store) (StaticActorContentStateExport, error) {
	return ExportStaticActorContentStateFromStores(s, interactions)
}

func loadStaticActorSnapshotForExport(store Store) (Snapshot, error) {
	if store == nil {
		return Snapshot{}, nil
	}
	snapshot, err := store.Load()
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}
	return snapshot, nil
}

func loadInteractionSnapshotForExport(store interactionstore.Store) (interactionstore.Snapshot, error) {
	if store == nil {
		return interactionstore.Snapshot{}, nil
	}
	snapshot, err := store.Load()
	if err != nil {
		if errors.Is(err, interactionstore.ErrSnapshotNotFound) {
			return interactionstore.Snapshot{}, nil
		}
		return interactionstore.Snapshot{}, err
	}
	return snapshot, nil
}

func normalizedInteractionDefinitionsForExport(snapshot interactionstore.Snapshot) (interactionstore.Snapshot, error) {
	definitions := append([]interactionstore.Definition(nil), snapshot.Definitions...)
	if definitions == nil {
		definitions = []interactionstore.Definition{}
	}
	for i := range definitions {
		definitions[i] = interactionstore.NormalizeDefinition(definitions[i])
	}
	sort.Slice(definitions, func(i int, j int) bool {
		if definitions[i].Kind == definitions[j].Kind {
			return definitions[i].Ref < definitions[j].Ref
		}
		return definitions[i].Kind < definitions[j].Kind
	})

	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if !interactionstore.ValidDefinition(definition) {
			return interactionstore.Snapshot{}, fmt.Errorf("%w: validate interaction definition content-state export", interactionstore.ErrInvalidSnapshot)
		}
		if !validStaticActorContentStateInteractionDefinition(definition) {
			return interactionstore.Snapshot{}, fmt.Errorf("%w: interaction definition %s:%s cannot target static actor content-state migration", interactionstore.ErrInvalidSnapshot, definition.Kind, definition.Ref)
		}
		key := interactionDefinitionExportKey(definition.Kind, definition.Ref)
		if _, ok := seen[key]; ok {
			return interactionstore.Snapshot{}, fmt.Errorf("%w: duplicate interaction definition %s:%s", interactionstore.ErrInvalidSnapshot, definition.Kind, definition.Ref)
		}
		seen[key] = struct{}{}
	}
	return interactionstore.Snapshot{Definitions: definitions}, nil
}

func validStaticActorContentStateInteractionDefinition(definition interactionstore.Definition) bool {
	switch definition.Kind {
	case interactionstore.KindInfo, interactionstore.KindTalk, interactionstore.KindWarp, interactionstore.KindShopPreview, interactionstore.KindOpenSafebox, interactionstore.KindQuestFlag:
		return true
	default:
		return false
	}
}

func interactionDefinitionRowForExport(definition interactionstore.Definition) InteractionDefinitionRow {
	row := InteractionDefinitionRow{
		Kind:              definition.Kind,
		Ref:               definition.Ref,
		Text:              definition.Text,
		Title:             definition.Title,
		Size:              definition.Size,
		QuestRef:          definition.QuestRef,
		QuestFlag:         definition.QuestFlag,
		QuestFrom:         definition.QuestFrom,
		QuestTo:           definition.QuestTo,
		RewardExperience:  definition.RewardExperience,
		RewardGold:        definition.RewardGold,
		ConsumeGold:       definition.ConsumeGold,
		ConsumeExperience: definition.ConsumeExperience,
	}
	if definition.Kind == interactionstore.KindWarp {
		row.MapIndex = uint32ExportPtr(definition.MapIndex)
		row.X = int32ExportPtr(definition.X)
		row.Y = int32ExportPtr(definition.Y)
	}
	return row
}

func staticActorContentStateRowForExport(actor StaticActor) StaticActorContentStateRow {
	row := StaticActorContentStateRow{
		EntityID:         actor.EntityID,
		Name:             actor.Name,
		MapIndex:         actor.MapIndex,
		X:                actor.X,
		Y:                actor.Y,
		RaceNum:          actor.RaceNum,
		CombatProfile:    actor.CombatProfile,
		InteractionKind:  actor.InteractionKind,
		InteractionRef:   actor.InteractionRef,
		SpawnGroupRef:    actor.SpawnGroupRef,
		RewardExperience: actor.RewardExperience,
		RewardGold:       actor.RewardGold,
		RewardQuestRef:   actor.RewardQuestRef,
		RewardQuestFlag:  actor.RewardQuestFlag,
		RewardQuestFrom:  actor.RewardQuestFrom,
		RewardQuestTo:    actor.RewardQuestTo,
		RewardQuestText:  actor.RewardQuestText,
		RequireQuestRef:  actor.RequireQuestRef,
		RequireQuestFlag: actor.RequireQuestFlag,
		RequireQuestFrom: actor.RequireQuestFrom,
	}
	if actor.SpawnHome != nil {
		row.SpawnHomeMapIndex = uint32ExportPtr(actor.SpawnHome.MapIndex)
		row.SpawnHomeX = int32ExportPtr(actor.SpawnHome.X)
		row.SpawnHomeY = int32ExportPtr(actor.SpawnHome.Y)
	}
	return row
}

func staticActorCombatProfileRowForExport(profile worldruntime.StaticActorCombatProfileSnapshot) StaticActorCombatProfileRow {
	return StaticActorCombatProfileRow{
		Profile:               profile.Profile,
		MaxHP:                 profile.MaxHP,
		DamagePerNormalAttack: profile.DamagePerNormalAttack,
		AttackValue:           profile.AttackValue,
		DefenseValue:          profile.DefenseValue,
		Level:                 profile.Level,
		Rank:                  profile.Rank,
		RespawnDelayMs:        profile.RespawnDelayMs,
		AggroRadius:           profile.AggroRadius,
		LeashRadius:           profile.LeashRadius,
		ChaseDelayMs:          profile.ChaseDelayMs,
		ReturnDelayMs:         profile.ReturnDelayMs,
		HomewardDelayMs:       profile.HomewardDelayMs,
		MaxStep:               profile.MaxStep,
		RetaliationPointDelta: profile.RetaliationPointDelta,
		DeathRewardExperience: profile.DeathReward.Experience,
		DeathRewardGold:       profile.DeathReward.Gold,
	}
}

func interactionDefinitionExportKey(kind string, ref string) string {
	return kind + "\x00" + ref
}

func uint32ExportPtr(value uint32) *uint32 { return &value }
func int32ExportPtr(value int32) *int32    { return &value }
