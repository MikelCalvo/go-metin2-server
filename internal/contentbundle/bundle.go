package contentbundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

var ErrInvalidBundle = errors.New("invalid content bundle")

type StaticActor struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	CombatProfile   string `json:"combat_profile,omitempty"`
	InteractionKind string `json:"interaction_kind,omitempty"`
	InteractionRef  string `json:"interaction_ref,omitempty"`
}

type SpawnGroup struct {
	Ref              string   `json:"ref"`
	Name             string   `json:"name,omitempty"`
	MapIndex         uint32   `json:"map_index"`
	X                int32    `json:"x"`
	Y                int32    `json:"y"`
	RaceNum          uint32   `json:"race_num"`
	CombatProfile    string   `json:"combat_profile"`
	RewardExperience uint64   `json:"reward_experience,omitempty"`
	RewardGold       uint64   `json:"reward_gold,omitempty"`
	RewardDropVnums  []uint32 `json:"reward_drop_vnums,omitempty"`
}

type Bundle struct {
	StaticActors           []StaticActor                                   `json:"static_actors"`
	SpawnGroups            []SpawnGroup                                    `json:"spawn_groups,omitempty"`
	CombatProfiles         []worldruntime.StaticActorCombatProfileSnapshot `json:"combat_profiles,omitempty"`
	ItemTemplates          []itemcatalog.Template                          `json:"item_templates,omitempty"`
	QuestState             []queststate.Flag                               `json:"quest_state,omitempty"`
	InteractionDefinitions []interactionstore.Definition                   `json:"interaction_definitions"`
}

func (bundle *Bundle) UnmarshalJSON(raw []byte) error {
	if bundle == nil {
		return errors.New("content bundle receiver is nil")
	}
	if !utf8.Valid(raw) {
		return errors.New("content bundle must be valid utf-8")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("content bundle cannot be null")
	}
	var jsonBundle struct {
		StaticActors           json.RawMessage `json:"static_actors"`
		SpawnGroups            json.RawMessage `json:"spawn_groups"`
		CombatProfiles         json.RawMessage `json:"combat_profiles"`
		ItemTemplates          json.RawMessage `json:"item_templates"`
		QuestState             json.RawMessage `json:"quest_state"`
		InteractionDefinitions json.RawMessage `json:"interaction_definitions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&jsonBundle); err != nil {
		return err
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("content bundle has trailing json value")
		}
		return err
	}
	var decoded Bundle
	if err := decodeBundleCollection(jsonBundle.StaticActors, &decoded.StaticActors); err != nil {
		return err
	}
	if err := decodeBundleCollection(jsonBundle.SpawnGroups, &decoded.SpawnGroups); err != nil {
		return err
	}
	if err := decodeBundleCollection(jsonBundle.CombatProfiles, &decoded.CombatProfiles); err != nil {
		return err
	}
	if err := decodeBundleCollection(jsonBundle.ItemTemplates, &decoded.ItemTemplates); err != nil {
		return err
	}
	if err := decodeBundleCollection(jsonBundle.QuestState, &decoded.QuestState); err != nil {
		return err
	}
	if len(decoded.QuestState) > 0 {
		decoded.QuestState = queststate.NormalizeSnapshot(queststate.Snapshot{Flags: decoded.QuestState}).Flags
	}
	if err := decodeBundleCollection(jsonBundle.InteractionDefinitions, &decoded.InteractionDefinitions); err != nil {
		return err
	}
	*bundle = decoded
	return nil
}

func decodeBundleCollection[T any](raw json.RawMessage, dst *[]T) error {
	if raw == nil {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("content bundle collection cannot be null")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("content bundle collection has trailing json value")
		}
		return err
	}
	return nil
}

type Summary struct {
	StaticActorCount                       int                                             `json:"static_actor_count"`
	InteractableStaticActorCount           int                                             `json:"interactable_static_actor_count"`
	SpawnGroupCount                        int                                             `json:"spawn_group_count"`
	CombatProfileCount                     int                                             `json:"combat_profile_count"`
	ItemTemplateCount                      int                                             `json:"item_template_count"`
	QuestStateFlagCount                    int                                             `json:"quest_state_flag_count,omitempty"`
	QuestStateCharacterCount               int                                             `json:"quest_state_character_count,omitempty"`
	QuestStateQuestCount                   int                                             `json:"quest_state_quest_count,omitempty"`
	QuestStateQuestRefs                    []string                                        `json:"quest_state_quest_refs,omitempty"`
	QuestStateCharacters                   []QuestStateCharacterSummary                    `json:"quest_state_characters,omitempty"`
	QuestStateQuests                       []QuestStateQuestSummary                        `json:"quest_state_quests,omitempty"`
	StaticActors                           []StaticActor                                   `json:"static_actors,omitempty"`
	ShopCatalogEntryCount                  int                                             `json:"shop_catalog_entry_count"`
	ShopCatalogs                           []ShopCatalogSummary                            `json:"shop_catalogs,omitempty"`
	ShopRouteCount                         int                                             `json:"shop_route_count"`
	ShopRoutes                             []ShopRouteSummary                              `json:"shop_routes,omitempty"`
	WarpDestinationCount                   int                                             `json:"warp_destination_count"`
	WarpDestinations                       []WarpDestinationSummary                        `json:"warp_destinations,omitempty"`
	WarpRouteCount                         int                                             `json:"warp_route_count"`
	WarpRoutes                             []WarpRouteSummary                              `json:"warp_routes,omitempty"`
	RewardExperienceTotal                  uint64                                          `json:"reward_experience_total,omitempty"`
	RewardGoldTotal                        uint64                                          `json:"reward_gold_total,omitempty"`
	RewardDropItemCount                    int                                             `json:"reward_drop_item_count,omitempty"`
	RewardDrops                            []RewardDropAggregateSummary                    `json:"reward_drops,omitempty"`
	InteractionDefinitionCount             int                                             `json:"interaction_definition_count"`
	ReferencedInteractionDefinitionCount   int                                             `json:"referenced_interaction_definition_count"`
	UnreferencedInteractionDefinitionCount int                                             `json:"unreferenced_interaction_definition_count"`
	InteractionKinds                       []InteractionKindSummary                        `json:"interaction_kinds,omitempty"`
	InteractionDefinitionPreviews          []InteractionDefinitionPreviewSummary           `json:"interaction_definition_previews,omitempty"`
	ReferencedInteractionDefinitions       []InteractionDefinitionReferenceSummary         `json:"referenced_interaction_definitions,omitempty"`
	UnreferencedInteractionDefinitions     []InteractionDefinitionReferenceSummary         `json:"unreferenced_interaction_definitions,omitempty"`
	InteractableStaticActors               []InteractableStaticActorSummary                `json:"interactable_static_actors,omitempty"`
	SpawnGroups                            []SpawnGroupReferenceSummary                    `json:"spawn_groups,omitempty"`
	CombatProfiles                         []worldruntime.StaticActorCombatProfileSnapshot `json:"combat_profiles,omitempty"`
	ItemTemplates                          []ItemTemplateReferenceSummary                  `json:"item_templates,omitempty"`
	Maps                                   []MapContentSummary                             `json:"maps,omitempty"`
}

type QuestStateCharacterSummary struct {
	Character string                    `json:"character"`
	FlagCount int                       `json:"flag_count"`
	Flags     []queststate.FlagSnapshot `json:"flags,omitempty"`
}

type QuestStateQuestSummary struct {
	QuestRef   string                       `json:"quest_ref"`
	FlagCount  int                          `json:"flag_count"`
	Characters []QuestStateCharacterSummary `json:"characters,omitempty"`
}

type QuestStateOverview struct {
	FlagCount      int                          `json:"flag_count"`
	CharacterCount int                          `json:"character_count"`
	QuestCount     int                          `json:"quest_count"`
	QuestRefs      []string                     `json:"quest_refs,omitempty"`
	Characters     []QuestStateCharacterSummary `json:"characters,omitempty"`
	Quests         []QuestStateQuestSummary     `json:"quests,omitempty"`
}

type ImportPreview struct {
	Current   Summary       `json:"current"`
	Candidate Summary       `json:"candidate"`
	Deltas    SummaryDeltas `json:"deltas"`
}

type SummaryDeltas struct {
	StaticActorCount                       SummaryCountDelta              `json:"static_actor_count"`
	InteractableStaticActorCount           SummaryCountDelta              `json:"interactable_static_actor_count"`
	SpawnGroupCount                        SummaryCountDelta              `json:"spawn_group_count"`
	CombatProfileCount                     SummaryCountDelta              `json:"combat_profile_count"`
	ItemTemplateCount                      SummaryCountDelta              `json:"item_template_count"`
	QuestStateFlagCount                    SummaryCountDelta              `json:"quest_state_flag_count,omitempty"`
	QuestStateCharacterCount               SummaryCountDelta              `json:"quest_state_character_count,omitempty"`
	QuestStateQuestCount                   SummaryCountDelta              `json:"quest_state_quest_count,omitempty"`
	QuestStateFlags                        []QuestStateDelta              `json:"quest_state_flags,omitempty"`
	ShopCatalogEntryCount                  SummaryCountDelta              `json:"shop_catalog_entry_count"`
	ShopCatalogs                           []ShopCatalogDelta             `json:"shop_catalogs,omitempty"`
	ShopRouteCount                         SummaryCountDelta              `json:"shop_route_count"`
	WarpDestinationCount                   SummaryCountDelta              `json:"warp_destination_count"`
	WarpDestinations                       []WarpDestinationDelta         `json:"warp_destinations,omitempty"`
	WarpRouteCount                         SummaryCountDelta              `json:"warp_route_count"`
	RewardExperienceTotal                  SummaryAmountDelta             `json:"reward_experience_total"`
	RewardGoldTotal                        SummaryAmountDelta             `json:"reward_gold_total"`
	RewardDropItemCount                    SummaryCountDelta              `json:"reward_drop_item_count"`
	RewardDrops                            []RewardDropDelta              `json:"reward_drops,omitempty"`
	InteractionDefinitionCount             SummaryCountDelta              `json:"interaction_definition_count"`
	ReferencedInteractionDefinitionCount   SummaryCountDelta              `json:"referenced_interaction_definition_count"`
	UnreferencedInteractionDefinitionCount SummaryCountDelta              `json:"unreferenced_interaction_definition_count"`
	StaticActors                           []StaticActorDelta             `json:"static_actors,omitempty"`
	InteractableStaticActors               []InteractableStaticActorDelta `json:"interactable_static_actors,omitempty"`
	InteractionKinds                       []InteractionKindDelta         `json:"interaction_kinds,omitempty"`
	InteractionDefinitions                 []InteractionDefinitionDelta   `json:"interaction_definitions,omitempty"`
	ItemTemplates                          []ItemTemplateDelta            `json:"item_templates,omitempty"`
	CombatProfiles                         []CombatProfileDelta           `json:"combat_profiles,omitempty"`
	SpawnGroups                            []SpawnGroupDelta              `json:"spawn_groups,omitempty"`
	ShopRoutes                             []ShopRouteDelta               `json:"shop_routes,omitempty"`
	WarpRoutes                             []WarpRouteDelta               `json:"warp_routes,omitempty"`
	Maps                                   []MapContentDelta              `json:"maps,omitempty"`
}

type SummaryCountDelta struct {
	Current   int `json:"current"`
	Candidate int `json:"candidate"`
	Delta     int `json:"delta"`
}

type SummaryAmountDelta struct {
	Current   uint64 `json:"current"`
	Candidate uint64 `json:"candidate"`
	Delta     int64  `json:"delta"`
}

type InteractionKindDelta struct {
	Kind              string            `json:"kind"`
	Count             SummaryCountDelta `json:"count"`
	ReferencedCount   SummaryCountDelta `json:"referenced_count"`
	UnreferencedCount SummaryCountDelta `json:"unreferenced_count"`
}

type QuestStateDelta struct {
	Character string                   `json:"character"`
	QuestRef  string                   `json:"quest_ref"`
	Name      string                   `json:"name"`
	Change    string                   `json:"change"`
	Current   *queststate.FlagSnapshot `json:"current,omitempty"`
	Candidate *queststate.FlagSnapshot `json:"candidate,omitempty"`
}

type QuestStateFlagIdentity struct {
	Character string
	QuestRef  string
	Name      string
}

type InteractionDefinitionDelta struct {
	Kind             string `json:"kind"`
	Ref              string `json:"ref"`
	Change           string `json:"change"`
	CurrentPreview   string `json:"current_preview,omitempty"`
	CandidatePreview string `json:"candidate_preview,omitempty"`
}

type StaticActorDelta struct {
	Change    string       `json:"change"`
	Current   *StaticActor `json:"current,omitempty"`
	Candidate *StaticActor `json:"candidate,omitempty"`
}

type InteractableStaticActorDelta struct {
	Change    string                          `json:"change"`
	Current   *InteractableStaticActorSummary `json:"current,omitempty"`
	Candidate *InteractableStaticActorSummary `json:"candidate,omitempty"`
}

type ItemTemplateDelta struct {
	Vnum      uint32                `json:"vnum"`
	Change    string                `json:"change"`
	Current   *itemcatalog.Template `json:"current,omitempty"`
	Candidate *itemcatalog.Template `json:"candidate,omitempty"`
}

type CombatProfileDelta struct {
	Profile   string                                         `json:"profile"`
	Change    string                                         `json:"change"`
	Current   *worldruntime.StaticActorCombatProfileSnapshot `json:"current,omitempty"`
	Candidate *worldruntime.StaticActorCombatProfileSnapshot `json:"candidate,omitempty"`
}

type SpawnGroupDelta struct {
	Ref       string                      `json:"ref"`
	Change    string                      `json:"change"`
	Current   *SpawnGroupReferenceSummary `json:"current,omitempty"`
	Candidate *SpawnGroupReferenceSummary `json:"candidate,omitempty"`
}

type RewardDropDelta struct {
	ItemVnum  uint32                      `json:"item_vnum"`
	Change    string                      `json:"change"`
	Current   *RewardDropAggregateSummary `json:"current,omitempty"`
	Candidate *RewardDropAggregateSummary `json:"candidate,omitempty"`
}

type ShopCatalogDelta struct {
	Kind      string              `json:"kind"`
	Ref       string              `json:"ref"`
	Change    string              `json:"change"`
	Current   *ShopCatalogSummary `json:"current,omitempty"`
	Candidate *ShopCatalogSummary `json:"candidate,omitempty"`
}

type WarpDestinationDelta struct {
	Kind      string                  `json:"kind"`
	Ref       string                  `json:"ref"`
	Change    string                  `json:"change"`
	Current   *WarpDestinationSummary `json:"current,omitempty"`
	Candidate *WarpDestinationSummary `json:"candidate,omitempty"`
}

type ShopRouteDelta struct {
	ActorName      string            `json:"actor_name"`
	SourceMapIndex uint32            `json:"source_map_index"`
	SourceX        int32             `json:"source_x"`
	SourceY        int32             `json:"source_y"`
	Ref            string            `json:"ref"`
	Change         string            `json:"change"`
	Current        *ShopRouteSummary `json:"current,omitempty"`
	Candidate      *ShopRouteSummary `json:"candidate,omitempty"`
}

type WarpRouteDelta struct {
	ActorName      string            `json:"actor_name"`
	SourceMapIndex uint32            `json:"source_map_index"`
	SourceX        int32             `json:"source_x"`
	SourceY        int32             `json:"source_y"`
	Ref            string            `json:"ref"`
	Change         string            `json:"change"`
	Current        *WarpRouteSummary `json:"current,omitempty"`
	Candidate      *WarpRouteSummary `json:"candidate,omitempty"`
}

type MapContentDelta struct {
	MapIndex                     uint32             `json:"map_index"`
	StaticActorCount             SummaryCountDelta  `json:"static_actor_count"`
	InteractableStaticActorCount SummaryCountDelta  `json:"interactable_static_actor_count"`
	InfoActorCount               SummaryCountDelta  `json:"info_actor_count,omitempty"`
	TalkActorCount               SummaryCountDelta  `json:"talk_actor_count,omitempty"`
	ShopPreviewActorCount        SummaryCountDelta  `json:"shop_preview_actor_count,omitempty"`
	ShopCatalogEntryCount        SummaryCountDelta  `json:"shop_catalog_entry_count,omitempty"`
	WarpActorCount               SummaryCountDelta  `json:"warp_actor_count,omitempty"`
	SpawnGroupCount              SummaryCountDelta  `json:"spawn_group_count"`
	RewardExperienceTotal        SummaryAmountDelta `json:"reward_experience_total,omitempty"`
	RewardGoldTotal              SummaryAmountDelta `json:"reward_gold_total,omitempty"`
	RewardDropItemCount          SummaryCountDelta  `json:"reward_drop_item_count,omitempty"`
	StaticActors                 []StaticActorDelta `json:"static_actors,omitempty"`
	SpawnGroups                  []SpawnGroupDelta  `json:"spawn_groups,omitempty"`
	ShopRoutes                   []ShopRouteDelta   `json:"shop_routes,omitempty"`
	WarpRoutes                   []WarpRouteDelta   `json:"warp_routes,omitempty"`
}

type InteractionKindSummary struct {
	Kind              string `json:"kind"`
	Count             int    `json:"count"`
	ReferencedCount   int    `json:"referenced_count"`
	UnreferencedCount int    `json:"unreferenced_count"`
}

type InteractionDefinitionReferenceSummary struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type InteractionDefinitionPreviewSummary struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Preview string `json:"preview"`
}

type InteractableStaticActorSummary struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	InteractionKind string `json:"interaction_kind"`
	InteractionRef  string `json:"interaction_ref"`
	Preview         string `json:"preview,omitempty"`
}

type SpawnGroupReferenceSummary struct {
	Ref              string                  `json:"ref"`
	Name             string                  `json:"name"`
	MapIndex         uint32                  `json:"map_index"`
	X                int32                   `json:"x"`
	Y                int32                   `json:"y"`
	RaceNum          uint32                  `json:"race_num"`
	CombatProfile    string                  `json:"combat_profile"`
	RewardExperience uint64                  `json:"reward_experience,omitempty"`
	RewardGold       uint64                  `json:"reward_gold,omitempty"`
	RewardDropVnums  []uint32                `json:"reward_drop_vnums,omitempty"`
	RewardDropItems  []RewardDropItemSummary `json:"reward_drop_items,omitempty"`
}

type RewardDropItemSummary struct {
	ItemVnum             uint32                   `json:"item_vnum"`
	ItemName             string                   `json:"item_name"`
	Stackable            bool                     `json:"stackable"`
	MaxCount             uint16                   `json:"max_count"`
	ShopBuyPrice         uint64                   `json:"shop_buy_price,omitempty"`
	ShopSellPrice        uint64                   `json:"shop_sell_price,omitempty"`
	Refineable           bool                     `json:"refineable,omitempty"`
	RefineRejectMessage  string                   `json:"refine_reject_message,omitempty"`
	ConfirmWhenUse       bool                     `json:"confirm_when_use,omitempty"`
	QuestUse             bool                     `json:"quest_use,omitempty"`
	QuestUseMultiple     bool                     `json:"quest_use_multiple,omitempty"`
	Applicable           bool                     `json:"applicable,omitempty"`
	EquipSlot            string                   `json:"equip_slot,omitempty"`
	AppearanceVnum       uint32                   `json:"appearance_vnum,omitempty"`
	Irremovable          bool                     `json:"irremovable,omitempty"`
	UseEffect            *itemcatalog.UseEffect   `json:"use_effect,omitempty"`
	EquipEffect          *itemcatalog.PointEffect `json:"equip_effect,omitempty"`
	AntiGet              bool                     `json:"anti_get,omitempty"`
	AntiDrop             bool                     `json:"anti_drop,omitempty"`
	AntiGive             bool                     `json:"anti_give,omitempty"`
	AntiSell             bool                     `json:"anti_sell,omitempty"`
	AntiStack            bool                     `json:"anti_stack,omitempty"`
	AntiSafebox          bool                     `json:"anti_safebox,omitempty"`
	AntiMale             bool                     `json:"anti_male,omitempty"`
	AntiFemale           bool                     `json:"anti_female,omitempty"`
	AntiWarrior          bool                     `json:"anti_warrior,omitempty"`
	AntiAssassin         bool                     `json:"anti_assassin,omitempty"`
	AntiSura             bool                     `json:"anti_sura,omitempty"`
	AntiShaman           bool                     `json:"anti_shaman,omitempty"`
	AntiEmpireA          bool                     `json:"anti_empire_a,omitempty"`
	AntiEmpireB          bool                     `json:"anti_empire_b,omitempty"`
	AntiEmpireC          bool                     `json:"anti_empire_c,omitempty"`
	MinLevel             uint8                    `json:"min_level,omitempty"`
	UseRejectMessage     string                   `json:"use_reject_message,omitempty"`
	BuyRejectMessage     string                   `json:"buy_reject_message,omitempty"`
	DropRejectMessage    string                   `json:"drop_reject_message,omitempty"`
	GiveRejectMessage    string                   `json:"give_reject_message,omitempty"`
	PickupRejectMessage  string                   `json:"pickup_reject_message,omitempty"`
	SellRejectMessage    string                   `json:"sell_reject_message,omitempty"`
	EquipRejectMessage   string                   `json:"equip_reject_message,omitempty"`
	UnequipRejectMessage string                   `json:"unequip_reject_message,omitempty"`
	SafeboxRejectMessage string                   `json:"safebox_reject_message,omitempty"`
	PickupRange          uint16                   `json:"pickup_range,omitempty"`
}

type RewardDropAggregateSummary struct {
	ItemVnum             uint32                   `json:"item_vnum"`
	ItemName             string                   `json:"item_name"`
	SourceCount          int                      `json:"source_count"`
	Stackable            bool                     `json:"stackable"`
	MaxCount             uint16                   `json:"max_count"`
	ShopBuyPrice         uint64                   `json:"shop_buy_price,omitempty"`
	ShopSellPrice        uint64                   `json:"shop_sell_price,omitempty"`
	Refineable           bool                     `json:"refineable,omitempty"`
	RefineRejectMessage  string                   `json:"refine_reject_message,omitempty"`
	ConfirmWhenUse       bool                     `json:"confirm_when_use,omitempty"`
	QuestUse             bool                     `json:"quest_use,omitempty"`
	QuestUseMultiple     bool                     `json:"quest_use_multiple,omitempty"`
	Applicable           bool                     `json:"applicable,omitempty"`
	EquipSlot            string                   `json:"equip_slot,omitempty"`
	AppearanceVnum       uint32                   `json:"appearance_vnum,omitempty"`
	Irremovable          bool                     `json:"irremovable,omitempty"`
	UseEffect            *itemcatalog.UseEffect   `json:"use_effect,omitempty"`
	EquipEffect          *itemcatalog.PointEffect `json:"equip_effect,omitempty"`
	AntiGet              bool                     `json:"anti_get,omitempty"`
	AntiDrop             bool                     `json:"anti_drop,omitempty"`
	AntiGive             bool                     `json:"anti_give,omitempty"`
	AntiSell             bool                     `json:"anti_sell,omitempty"`
	AntiStack            bool                     `json:"anti_stack,omitempty"`
	AntiSafebox          bool                     `json:"anti_safebox,omitempty"`
	AntiMale             bool                     `json:"anti_male,omitempty"`
	AntiFemale           bool                     `json:"anti_female,omitempty"`
	AntiWarrior          bool                     `json:"anti_warrior,omitempty"`
	AntiAssassin         bool                     `json:"anti_assassin,omitempty"`
	AntiSura             bool                     `json:"anti_sura,omitempty"`
	AntiShaman           bool                     `json:"anti_shaman,omitempty"`
	AntiEmpireA          bool                     `json:"anti_empire_a,omitempty"`
	AntiEmpireB          bool                     `json:"anti_empire_b,omitempty"`
	AntiEmpireC          bool                     `json:"anti_empire_c,omitempty"`
	MinLevel             uint8                    `json:"min_level,omitempty"`
	UseRejectMessage     string                   `json:"use_reject_message,omitempty"`
	BuyRejectMessage     string                   `json:"buy_reject_message,omitempty"`
	DropRejectMessage    string                   `json:"drop_reject_message,omitempty"`
	GiveRejectMessage    string                   `json:"give_reject_message,omitempty"`
	PickupRejectMessage  string                   `json:"pickup_reject_message,omitempty"`
	SellRejectMessage    string                   `json:"sell_reject_message,omitempty"`
	EquipRejectMessage   string                   `json:"equip_reject_message,omitempty"`
	UnequipRejectMessage string                   `json:"unequip_reject_message,omitempty"`
	SafeboxRejectMessage string                   `json:"safebox_reject_message,omitempty"`
	PickupRange          uint16                   `json:"pickup_range,omitempty"`
}

type ItemTemplateReferenceSummary struct {
	Vnum                 uint32                   `json:"vnum"`
	Name                 string                   `json:"name"`
	Stackable            bool                     `json:"stackable"`
	MaxCount             uint16                   `json:"max_count"`
	ShopBuyPrice         uint64                   `json:"shop_buy_price,omitempty"`
	ShopSellPrice        uint64                   `json:"shop_sell_price,omitempty"`
	Refineable           bool                     `json:"refineable,omitempty"`
	RefineRejectMessage  string                   `json:"refine_reject_message,omitempty"`
	ConfirmWhenUse       bool                     `json:"confirm_when_use,omitempty"`
	QuestUse             bool                     `json:"quest_use,omitempty"`
	QuestUseMultiple     bool                     `json:"quest_use_multiple,omitempty"`
	Applicable           bool                     `json:"applicable,omitempty"`
	EquipSlot            string                   `json:"equip_slot,omitempty"`
	AppearanceVnum       uint32                   `json:"appearance_vnum,omitempty"`
	Irremovable          bool                     `json:"irremovable,omitempty"`
	UseEffect            *itemcatalog.UseEffect   `json:"use_effect,omitempty"`
	EquipEffect          *itemcatalog.PointEffect `json:"equip_effect,omitempty"`
	AntiGet              bool                     `json:"anti_get,omitempty"`
	AntiDrop             bool                     `json:"anti_drop,omitempty"`
	AntiGive             bool                     `json:"anti_give,omitempty"`
	AntiSell             bool                     `json:"anti_sell,omitempty"`
	AntiStack            bool                     `json:"anti_stack,omitempty"`
	AntiSafebox          bool                     `json:"anti_safebox,omitempty"`
	AntiMale             bool                     `json:"anti_male,omitempty"`
	AntiFemale           bool                     `json:"anti_female,omitempty"`
	AntiWarrior          bool                     `json:"anti_warrior,omitempty"`
	AntiAssassin         bool                     `json:"anti_assassin,omitempty"`
	AntiSura             bool                     `json:"anti_sura,omitempty"`
	AntiShaman           bool                     `json:"anti_shaman,omitempty"`
	AntiEmpireA          bool                     `json:"anti_empire_a,omitempty"`
	AntiEmpireB          bool                     `json:"anti_empire_b,omitempty"`
	AntiEmpireC          bool                     `json:"anti_empire_c,omitempty"`
	MinLevel             uint8                    `json:"min_level,omitempty"`
	UseRejectMessage     string                   `json:"use_reject_message,omitempty"`
	BuyRejectMessage     string                   `json:"buy_reject_message,omitempty"`
	DropRejectMessage    string                   `json:"drop_reject_message,omitempty"`
	GiveRejectMessage    string                   `json:"give_reject_message,omitempty"`
	PickupRejectMessage  string                   `json:"pickup_reject_message,omitempty"`
	SellRejectMessage    string                   `json:"sell_reject_message,omitempty"`
	EquipRejectMessage   string                   `json:"equip_reject_message,omitempty"`
	UnequipRejectMessage string                   `json:"unequip_reject_message,omitempty"`
	SafeboxRejectMessage string                   `json:"safebox_reject_message,omitempty"`
	PickupRange          uint16                   `json:"pickup_range,omitempty"`
}

type ShopCatalogSummary struct {
	Kind       string                    `json:"kind"`
	Ref        string                    `json:"ref"`
	Title      string                    `json:"title"`
	EntryCount int                       `json:"entry_count"`
	Entries    []ShopCatalogEntrySummary `json:"entries,omitempty"`
}

type ShopCatalogEntrySummary struct {
	Slot                 uint16                   `json:"slot"`
	ItemVnum             uint32                   `json:"item_vnum"`
	ItemName             string                   `json:"item_name"`
	Count                uint16                   `json:"count"`
	Price                uint64                   `json:"price"`
	Stackable            bool                     `json:"stackable"`
	MaxCount             uint16                   `json:"max_count"`
	ShopBuyPrice         uint64                   `json:"shop_buy_price,omitempty"`
	ShopSellPrice        uint64                   `json:"shop_sell_price,omitempty"`
	Refineable           bool                     `json:"refineable,omitempty"`
	RefineRejectMessage  string                   `json:"refine_reject_message,omitempty"`
	ConfirmWhenUse       bool                     `json:"confirm_when_use,omitempty"`
	QuestUse             bool                     `json:"quest_use,omitempty"`
	QuestUseMultiple     bool                     `json:"quest_use_multiple,omitempty"`
	Applicable           bool                     `json:"applicable,omitempty"`
	EquipSlot            string                   `json:"equip_slot,omitempty"`
	AppearanceVnum       uint32                   `json:"appearance_vnum,omitempty"`
	Irremovable          bool                     `json:"irremovable,omitempty"`
	UseEffect            *itemcatalog.UseEffect   `json:"use_effect,omitempty"`
	EquipEffect          *itemcatalog.PointEffect `json:"equip_effect,omitempty"`
	AntiGet              bool                     `json:"anti_get,omitempty"`
	AntiDrop             bool                     `json:"anti_drop,omitempty"`
	AntiGive             bool                     `json:"anti_give,omitempty"`
	AntiSell             bool                     `json:"anti_sell,omitempty"`
	AntiStack            bool                     `json:"anti_stack,omitempty"`
	AntiSafebox          bool                     `json:"anti_safebox,omitempty"`
	AntiMale             bool                     `json:"anti_male,omitempty"`
	AntiFemale           bool                     `json:"anti_female,omitempty"`
	AntiWarrior          bool                     `json:"anti_warrior,omitempty"`
	AntiAssassin         bool                     `json:"anti_assassin,omitempty"`
	AntiSura             bool                     `json:"anti_sura,omitempty"`
	AntiShaman           bool                     `json:"anti_shaman,omitempty"`
	AntiEmpireA          bool                     `json:"anti_empire_a,omitempty"`
	AntiEmpireB          bool                     `json:"anti_empire_b,omitempty"`
	AntiEmpireC          bool                     `json:"anti_empire_c,omitempty"`
	MinLevel             uint8                    `json:"min_level,omitempty"`
	UseRejectMessage     string                   `json:"use_reject_message,omitempty"`
	BuyRejectMessage     string                   `json:"buy_reject_message,omitempty"`
	DropRejectMessage    string                   `json:"drop_reject_message,omitempty"`
	GiveRejectMessage    string                   `json:"give_reject_message,omitempty"`
	PickupRejectMessage  string                   `json:"pickup_reject_message,omitempty"`
	SellRejectMessage    string                   `json:"sell_reject_message,omitempty"`
	EquipRejectMessage   string                   `json:"equip_reject_message,omitempty"`
	UnequipRejectMessage string                   `json:"unequip_reject_message,omitempty"`
	SafeboxRejectMessage string                   `json:"safebox_reject_message,omitempty"`
	PickupRange          uint16                   `json:"pickup_range,omitempty"`
}

type ShopRouteSummary struct {
	ActorName      string `json:"actor_name"`
	SourceMapIndex uint32 `json:"source_map_index"`
	SourceX        int32  `json:"source_x"`
	SourceY        int32  `json:"source_y"`
	Ref            string `json:"ref"`
	Title          string `json:"title"`
	EntryCount     int    `json:"entry_count"`
}

type WarpDestinationSummary struct {
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Text     string `json:"text,omitempty"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type WarpRouteSummary struct {
	ActorName      string `json:"actor_name"`
	SourceMapIndex uint32 `json:"source_map_index"`
	SourceX        int32  `json:"source_x"`
	SourceY        int32  `json:"source_y"`
	Ref            string `json:"ref"`
	Text           string `json:"text,omitempty"`
	TargetMapIndex uint32 `json:"target_map_index"`
	TargetX        int32  `json:"target_x"`
	TargetY        int32  `json:"target_y"`
}

type MapContentSummary struct {
	MapIndex                     uint32 `json:"map_index"`
	StaticActorCount             int    `json:"static_actor_count"`
	InteractableStaticActorCount int    `json:"interactable_static_actor_count"`
	InfoActorCount               int    `json:"info_actor_count,omitempty"`
	TalkActorCount               int    `json:"talk_actor_count,omitempty"`
	ShopPreviewActorCount        int    `json:"shop_preview_actor_count,omitempty"`
	ShopCatalogEntryCount        int    `json:"shop_catalog_entry_count,omitempty"`
	WarpActorCount               int    `json:"warp_actor_count,omitempty"`
	SpawnGroupCount              int    `json:"spawn_group_count"`
	RewardExperienceTotal        uint64 `json:"reward_experience_total,omitempty"`
	RewardGoldTotal              uint64 `json:"reward_gold_total,omitempty"`
	RewardDropItemCount          int    `json:"reward_drop_item_count,omitempty"`
}

func FromSnapshots(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot) (Bundle, error) {
	return FromSnapshotsWithItems(staticActors, interactions, itemcatalog.Snapshot{})
}

func spawnGroupAuthoredPosition(actor staticstore.StaticActor) (uint32, int32, int32) {
	if actor.SpawnHome != nil && actor.SpawnHome.MapIndex != 0 {
		return actor.SpawnHome.MapIndex, actor.SpawnHome.X, actor.SpawnHome.Y
	}
	return actor.MapIndex, actor.X, actor.Y
}

func FromSnapshotsWithItems(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot, items itemcatalog.Snapshot) (Bundle, error) {
	bundle := Bundle{
		InteractionDefinitions: cloneDefinitions(interactions.Definitions),
	}
	bundle.StaticActors = make([]StaticActor, 0, len(staticActors.StaticActors))
	bundle.SpawnGroups = make([]SpawnGroup, 0, len(staticActors.StaticActors))
	for _, actor := range staticActors.StaticActors {
		if actor.SpawnGroupRef != "" {
			mapIndex, x, y := spawnGroupAuthoredPosition(actor)
			bundle.SpawnGroups = append(bundle.SpawnGroups, SpawnGroup{
				Ref:              actor.SpawnGroupRef,
				Name:             actor.Name,
				MapIndex:         mapIndex,
				X:                x,
				Y:                y,
				RaceNum:          actor.RaceNum,
				CombatProfile:    actor.CombatProfile,
				RewardExperience: actor.RewardExperience,
				RewardGold:       actor.RewardGold,
				RewardDropVnums:  cloneUint32s(actor.RewardDropVnums),
			})
			continue
		}
		bundle.StaticActors = append(bundle.StaticActors, StaticActor{
			Name:            actor.Name,
			MapIndex:        actor.MapIndex,
			X:               actor.X,
			Y:               actor.Y,
			RaceNum:         actor.RaceNum,
			CombatProfile:   actor.CombatProfile,
			InteractionKind: actor.InteractionKind,
			InteractionRef:  actor.InteractionRef,
		})
	}
	normalizedStaticActors := normalizeStaticActors(bundle.StaticActors)
	normalizedSpawnGroups := normalizeSpawnGroups(bundle.SpawnGroups, nil)
	portableCombatProfiles := combatProfilesForAuthoredActors(normalizedStaticActors, normalizedSpawnGroups, nil)
	bundle.ItemTemplates = filterReferencedItemTemplates(items.Templates, referencedItemTemplateVnums(bundle.InteractionDefinitions, normalizedSpawnGroups, portableCombatProfiles))
	return Canonicalize(bundle)
}

func CanonicalJSON(bundle Bundle) ([]byte, error) {
	normalized, err := Canonicalize(bundle)
	if err != nil {
		return nil, err
	}
	normalized = normalizeCanonicalJSONContractCollections(normalized)
	encoded, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func normalizeCanonicalJSONContractCollections(bundle Bundle) Bundle {
	if bundle.StaticActors == nil {
		bundle.StaticActors = []StaticActor{}
	}
	if bundle.InteractionDefinitions == nil {
		bundle.InteractionDefinitions = []interactionstore.Definition{}
	}
	return bundle
}

func Canonicalize(bundle Bundle) (Bundle, error) {
	if !combatProfileSnapshotIdentitiesAreCanonical(bundle.CombatProfiles) {
		return Bundle{}, ErrInvalidBundle
	}
	normalizedStaticActors := normalizeStaticActors(bundle.StaticActors)
	normalizedCombatProfiles := normalizeCombatProfiles(bundle.CombatProfiles)
	normalizedSpawnGroups := normalizeSpawnGroups(bundle.SpawnGroups, normalizedCombatProfiles)
	normalized := Bundle{
		StaticActors:           normalizedStaticActors,
		SpawnGroups:            normalizedSpawnGroups,
		CombatProfiles:         combatProfilesForAuthoredActors(normalizedStaticActors, normalizedSpawnGroups, normalizedCombatProfiles),
		ItemTemplates:          normalizeItemTemplates(bundle.ItemTemplates),
		QuestState:             normalizeQuestStateFlags(bundle.QuestState),
		InteractionDefinitions: cloneDefinitions(bundle.InteractionDefinitions),
	}
	sort.Slice(normalized.StaticActors, func(i int, j int) bool {
		if normalized.StaticActors[i].Name == normalized.StaticActors[j].Name {
			if normalized.StaticActors[i].MapIndex == normalized.StaticActors[j].MapIndex {
				if normalized.StaticActors[i].X == normalized.StaticActors[j].X {
					if normalized.StaticActors[i].Y == normalized.StaticActors[j].Y {
						if normalized.StaticActors[i].RaceNum == normalized.StaticActors[j].RaceNum {
							if normalized.StaticActors[i].CombatProfile == normalized.StaticActors[j].CombatProfile {
								if normalized.StaticActors[i].InteractionKind == normalized.StaticActors[j].InteractionKind {
									return normalized.StaticActors[i].InteractionRef < normalized.StaticActors[j].InteractionRef
								}
								return normalized.StaticActors[i].InteractionKind < normalized.StaticActors[j].InteractionKind
							}
							return normalized.StaticActors[i].CombatProfile < normalized.StaticActors[j].CombatProfile
						}
						return normalized.StaticActors[i].RaceNum < normalized.StaticActors[j].RaceNum
					}
					return normalized.StaticActors[i].Y < normalized.StaticActors[j].Y
				}
				return normalized.StaticActors[i].X < normalized.StaticActors[j].X
			}
			return normalized.StaticActors[i].MapIndex < normalized.StaticActors[j].MapIndex
		}
		return normalized.StaticActors[i].Name < normalized.StaticActors[j].Name
	})
	sort.Slice(normalized.SpawnGroups, func(i int, j int) bool {
		if normalized.SpawnGroups[i].Ref == normalized.SpawnGroups[j].Ref {
			if normalized.SpawnGroups[i].MapIndex == normalized.SpawnGroups[j].MapIndex {
				if normalized.SpawnGroups[i].X == normalized.SpawnGroups[j].X {
					if normalized.SpawnGroups[i].Y == normalized.SpawnGroups[j].Y {
						if normalized.SpawnGroups[i].RaceNum == normalized.SpawnGroups[j].RaceNum {
							if normalized.SpawnGroups[i].CombatProfile == normalized.SpawnGroups[j].CombatProfile {
								if normalized.SpawnGroups[i].RewardExperience == normalized.SpawnGroups[j].RewardExperience {
									if normalized.SpawnGroups[i].RewardGold == normalized.SpawnGroups[j].RewardGold {
										if compareUint32s(normalized.SpawnGroups[i].RewardDropVnums, normalized.SpawnGroups[j].RewardDropVnums) == 0 {
											return normalized.SpawnGroups[i].Name < normalized.SpawnGroups[j].Name
										}
										return compareUint32s(normalized.SpawnGroups[i].RewardDropVnums, normalized.SpawnGroups[j].RewardDropVnums) < 0
									}
									return normalized.SpawnGroups[i].RewardGold < normalized.SpawnGroups[j].RewardGold
								}
								return normalized.SpawnGroups[i].RewardExperience < normalized.SpawnGroups[j].RewardExperience
							}
							return normalized.SpawnGroups[i].CombatProfile < normalized.SpawnGroups[j].CombatProfile
						}
						return normalized.SpawnGroups[i].RaceNum < normalized.SpawnGroups[j].RaceNum
					}
					return normalized.SpawnGroups[i].Y < normalized.SpawnGroups[j].Y
				}
				return normalized.SpawnGroups[i].X < normalized.SpawnGroups[j].X
			}
			return normalized.SpawnGroups[i].MapIndex < normalized.SpawnGroups[j].MapIndex
		}
		return normalized.SpawnGroups[i].Ref < normalized.SpawnGroups[j].Ref
	})
	sort.Slice(normalized.InteractionDefinitions, func(i int, j int) bool {
		if normalized.InteractionDefinitions[i].Kind == normalized.InteractionDefinitions[j].Kind {
			return normalized.InteractionDefinitions[i].Ref < normalized.InteractionDefinitions[j].Ref
		}
		return normalized.InteractionDefinitions[i].Kind < normalized.InteractionDefinitions[j].Kind
	})
	sort.Slice(normalized.ItemTemplates, func(i int, j int) bool {
		return normalized.ItemTemplates[i].Vnum < normalized.ItemTemplates[j].Vnum
	})
	if len(normalized.QuestState) > 0 {
		normalized.QuestState = queststate.NormalizeSnapshot(queststate.Snapshot{Flags: normalized.QuestState}).Flags
	}
	if err := validateBundle(normalized); err != nil {
		return Bundle{}, err
	}
	return normalized, nil
}

func BuildImportPreview(current Bundle, candidate Bundle) (ImportPreview, error) {
	currentCanonical, err := Canonicalize(current)
	if err != nil {
		return ImportPreview{}, err
	}
	candidateCanonical, err := Canonicalize(candidate)
	if err != nil {
		return ImportPreview{}, err
	}
	currentSummary, err := Summarize(currentCanonical)
	if err != nil {
		return ImportPreview{}, err
	}
	candidateSummary, err := Summarize(candidateCanonical)
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{
		Current:   currentSummary,
		Candidate: candidateSummary,
		Deltas:    buildSummaryDeltas(currentSummary, candidateSummary, currentCanonical, candidateCanonical),
	}, nil
}

func buildSummaryDeltas(current Summary, candidate Summary, currentBundle Bundle, candidateBundle Bundle) SummaryDeltas {
	return SummaryDeltas{
		StaticActorCount:                       summaryCountDelta(current.StaticActorCount, candidate.StaticActorCount),
		InteractableStaticActorCount:           summaryCountDelta(current.InteractableStaticActorCount, candidate.InteractableStaticActorCount),
		SpawnGroupCount:                        summaryCountDelta(current.SpawnGroupCount, candidate.SpawnGroupCount),
		CombatProfileCount:                     summaryCountDelta(current.CombatProfileCount, candidate.CombatProfileCount),
		ItemTemplateCount:                      summaryCountDelta(current.ItemTemplateCount, candidate.ItemTemplateCount),
		QuestStateFlagCount:                    summaryCountDelta(current.QuestStateFlagCount, candidate.QuestStateFlagCount),
		QuestStateCharacterCount:               summaryCountDelta(current.QuestStateCharacterCount, candidate.QuestStateCharacterCount),
		QuestStateQuestCount:                   summaryCountDelta(current.QuestStateQuestCount, candidate.QuestStateQuestCount),
		QuestStateFlags:                        buildQuestStateFlagDeltas(currentBundle.QuestState, candidateBundle.QuestState),
		ShopCatalogEntryCount:                  summaryCountDelta(current.ShopCatalogEntryCount, candidate.ShopCatalogEntryCount),
		ShopCatalogs:                           buildShopCatalogDeltas(current.ShopCatalogs, candidate.ShopCatalogs),
		ShopRouteCount:                         summaryCountDelta(current.ShopRouteCount, candidate.ShopRouteCount),
		WarpDestinationCount:                   summaryCountDelta(current.WarpDestinationCount, candidate.WarpDestinationCount),
		WarpDestinations:                       buildWarpDestinationDeltas(current.WarpDestinations, candidate.WarpDestinations),
		WarpRouteCount:                         summaryCountDelta(current.WarpRouteCount, candidate.WarpRouteCount),
		RewardExperienceTotal:                  summaryAmountDelta(current.RewardExperienceTotal, candidate.RewardExperienceTotal),
		RewardGoldTotal:                        summaryAmountDelta(current.RewardGoldTotal, candidate.RewardGoldTotal),
		RewardDropItemCount:                    summaryCountDelta(current.RewardDropItemCount, candidate.RewardDropItemCount),
		RewardDrops:                            buildRewardDropDeltas(current.RewardDrops, candidate.RewardDrops),
		InteractionDefinitionCount:             summaryCountDelta(current.InteractionDefinitionCount, candidate.InteractionDefinitionCount),
		ReferencedInteractionDefinitionCount:   summaryCountDelta(current.ReferencedInteractionDefinitionCount, candidate.ReferencedInteractionDefinitionCount),
		UnreferencedInteractionDefinitionCount: summaryCountDelta(current.UnreferencedInteractionDefinitionCount, candidate.UnreferencedInteractionDefinitionCount),
		StaticActors:                           buildStaticActorDeltas(currentBundle.StaticActors, candidateBundle.StaticActors),
		InteractableStaticActors:               buildInteractableStaticActorDeltas(current.InteractableStaticActors, candidate.InteractableStaticActors),
		InteractionKinds:                       buildInteractionKindDeltas(current.InteractionKinds, candidate.InteractionKinds),
		InteractionDefinitions:                 buildInteractionDefinitionDeltas(currentBundle, candidateBundle),
		ItemTemplates:                          buildItemTemplateDeltas(currentBundle.ItemTemplates, candidateBundle.ItemTemplates),
		CombatProfiles:                         buildCombatProfileDeltas(currentBundle.CombatProfiles, candidateBundle.CombatProfiles),
		SpawnGroups:                            buildSpawnGroupDeltas(current.SpawnGroups, candidate.SpawnGroups),
		ShopRoutes:                             buildShopRouteDeltas(current.ShopRoutes, candidate.ShopRoutes),
		WarpRoutes:                             buildWarpRouteDeltas(current.WarpRoutes, candidate.WarpRoutes),
		Maps:                                   buildMapContentDeltas(current, candidate, currentBundle, candidateBundle),
	}
}

func summaryCountDelta(current int, candidate int) SummaryCountDelta {
	return SummaryCountDelta{Current: current, Candidate: candidate, Delta: candidate - current}
}

func summaryAmountDelta(current uint64, candidate uint64) SummaryAmountDelta {
	delta := int64(0)
	if candidate >= current {
		delta = int64(candidate - current)
	} else {
		delta = -int64(current - candidate)
	}
	return SummaryAmountDelta{Current: current, Candidate: candidate, Delta: delta}
}

func buildInteractionKindDeltas(currentKinds []InteractionKindSummary, candidateKinds []InteractionKindSummary) []InteractionKindDelta {
	if len(currentKinds) == 0 && len(candidateKinds) == 0 {
		return nil
	}
	currentByKind := make(map[string]InteractionKindSummary, len(currentKinds))
	candidateByKind := make(map[string]InteractionKindSummary, len(candidateKinds))
	kindsSeen := make(map[string]struct{}, len(currentKinds)+len(candidateKinds))
	for _, summary := range currentKinds {
		currentByKind[summary.Kind] = summary
		kindsSeen[summary.Kind] = struct{}{}
	}
	for _, summary := range candidateKinds {
		candidateByKind[summary.Kind] = summary
		kindsSeen[summary.Kind] = struct{}{}
	}
	kinds := make([]string, 0, len(kindsSeen))
	for kind := range kindsSeen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	deltas := make([]InteractionKindDelta, 0, len(kinds))
	for _, kind := range kinds {
		current := currentByKind[kind]
		candidate := candidateByKind[kind]
		delta := InteractionKindDelta{
			Kind:              kind,
			Count:             summaryCountDelta(current.Count, candidate.Count),
			ReferencedCount:   summaryCountDelta(current.ReferencedCount, candidate.ReferencedCount),
			UnreferencedCount: summaryCountDelta(current.UnreferencedCount, candidate.UnreferencedCount),
		}
		if !interactionKindDeltaIsZero(delta) {
			deltas = append(deltas, delta)
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func InteractionKindDeltaByKind(deltas []InteractionKindDelta, kind string) (InteractionKindDelta, bool) {
	kind = strings.TrimSpace(kind)
	if kind == "" || !interactionstore.ValidKind(kind) {
		return InteractionKindDelta{}, false
	}
	for _, delta := range deltas {
		if strings.TrimSpace(delta.Kind) == kind {
			return delta, true
		}
	}
	return InteractionKindDelta{}, false
}

func interactionKindDeltaIsZero(delta InteractionKindDelta) bool {
	return delta.Count.Delta == 0 &&
		delta.ReferencedCount.Delta == 0 &&
		delta.UnreferencedCount.Delta == 0
}

func buildQuestStateFlagDeltas(currentFlags []queststate.Flag, candidateFlags []queststate.Flag) []QuestStateDelta {
	if len(currentFlags) == 0 && len(candidateFlags) == 0 {
		return nil
	}
	currentByKey := questStateFlagMapByKey(currentFlags)
	candidateByKey := questStateFlagMapByKey(candidateFlags)
	keysSeen := make(map[string]struct{}, len(currentByKey)+len(candidateByKey))
	for key := range currentByKey {
		keysSeen[key] = struct{}{}
	}
	for key := range candidateByKey {
		keysSeen[key] = struct{}{}
	}
	keys := make([]string, 0, len(keysSeen))
	for key := range keysSeen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	deltas := make([]QuestStateDelta, 0, len(keys))
	for _, key := range keys {
		current, currentOK := currentByKey[key]
		candidate, candidateOK := candidateByKey[key]
		identity := current
		if !currentOK {
			identity = candidate
		}
		delta := QuestStateDelta{Character: identity.Character, QuestRef: identity.QuestRef, Name: identity.Name}
		switch {
		case !currentOK:
			candidateSnapshot := queststate.FlagSnapshot{QuestRef: candidate.QuestRef, Name: candidate.Name, Value: candidate.Value}
			delta.Change = "added"
			delta.Candidate = &candidateSnapshot
		case !candidateOK:
			currentSnapshot := queststate.FlagSnapshot{QuestRef: current.QuestRef, Name: current.Name, Value: current.Value}
			delta.Change = "removed"
			delta.Current = &currentSnapshot
		case current.Value != candidate.Value:
			currentSnapshot := queststate.FlagSnapshot{QuestRef: current.QuestRef, Name: current.Name, Value: current.Value}
			candidateSnapshot := queststate.FlagSnapshot{QuestRef: candidate.QuestRef, Name: candidate.Name, Value: candidate.Value}
			delta.Change = "changed"
			delta.Current = &currentSnapshot
			delta.Candidate = &candidateSnapshot
		default:
			continue
		}
		deltas = append(deltas, delta)
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func QuestStateFlagDeltaByIdentity(deltas []QuestStateDelta, identity QuestStateFlagIdentity) (QuestStateDelta, bool) {
	identity.Character = strings.TrimSpace(identity.Character)
	identity.QuestRef = strings.TrimSpace(identity.QuestRef)
	identity.Name = strings.TrimSpace(identity.Name)
	if identity.Character == "" || identity.QuestRef == "" || identity.Name == "" {
		return QuestStateDelta{}, false
	}
	for _, delta := range deltas {
		if delta.Character == identity.Character && delta.QuestRef == identity.QuestRef && delta.Name == identity.Name {
			return cloneQuestStateDelta(delta), true
		}
	}
	return QuestStateDelta{}, false
}

func QuestStateFlagDeltasByCharacter(deltas []QuestStateDelta, character string) []QuestStateDelta {
	character = strings.TrimSpace(character)
	if character == "" || !queststate.ValidCharacterName(character) {
		return nil
	}
	matches := make([]QuestStateDelta, 0)
	for _, delta := range deltas {
		if delta.Character != character {
			continue
		}
		matches = append(matches, cloneQuestStateDelta(delta))
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func QuestStateFlagDeltasByQuestRef(deltas []QuestStateDelta, questRef string) []QuestStateDelta {
	questRef = strings.TrimSpace(questRef)
	if questRef == "" || !queststate.ValidQuestRef(questRef) {
		return nil
	}
	matches := make([]QuestStateDelta, 0)
	for _, delta := range deltas {
		if delta.QuestRef != questRef {
			continue
		}
		matches = append(matches, cloneQuestStateDelta(delta))
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func cloneQuestStateDelta(delta QuestStateDelta) QuestStateDelta {
	cloned := delta
	if delta.Current != nil {
		current := *delta.Current
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := *delta.Candidate
		cloned.Candidate = &candidate
	}
	return cloned
}

func questStateFlagMapByKey(flags []queststate.Flag) map[string]queststate.Flag {
	byKey := make(map[string]queststate.Flag, len(flags))
	for _, flag := range normalizeQuestStateFlags(flags) {
		byKey[questStateFlagKey(flag)] = flag
	}
	return byKey
}

func buildStaticActorDeltas(currentActors []StaticActor, candidateActors []StaticActor) []StaticActorDelta {
	if len(currentActors) == 0 && len(candidateActors) == 0 {
		return nil
	}
	currentByKey := staticActorMapByAuthoringKey(currentActors)
	candidateByKey := staticActorMapByAuthoringKey(candidateActors)
	keysSeen := make(map[string]struct{}, len(currentByKey)+len(candidateByKey))
	for key := range currentByKey {
		keysSeen[key] = struct{}{}
	}
	for key := range candidateByKey {
		keysSeen[key] = struct{}{}
	}
	keys := make([]string, 0, len(keysSeen))
	for key := range keysSeen {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	deltas := make([]StaticActorDelta, 0, len(keys))
	for _, key := range keys {
		current, currentOK := currentByKey[key]
		candidate, candidateOK := candidateByKey[key]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, StaticActorDelta{Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, StaticActorDelta{Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, StaticActorDelta{Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func StaticActorDeltasByName(deltas []StaticActorDelta, name string) []StaticActorDelta {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		return nil
	}
	matches := make([]StaticActorDelta, 0)
	for _, delta := range deltas {
		if staticActorDeltaMatchesName(delta, name) {
			matches = append(matches, cloneStaticActorDelta(delta))
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func staticActorDeltaMatchesName(delta StaticActorDelta, name string) bool {
	return (delta.Current != nil && strings.TrimSpace(delta.Current.Name) == name) ||
		(delta.Candidate != nil && strings.TrimSpace(delta.Candidate.Name) == name)
}

func cloneStaticActorDelta(delta StaticActorDelta) StaticActorDelta {
	cloned := delta
	if delta.Current != nil {
		current := *delta.Current
		current.Name = strings.TrimSpace(current.Name)
		current.CombatProfile = strings.TrimSpace(current.CombatProfile)
		current.InteractionKind = strings.TrimSpace(current.InteractionKind)
		current.InteractionRef = strings.TrimSpace(current.InteractionRef)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := *delta.Candidate
		candidate.Name = strings.TrimSpace(candidate.Name)
		candidate.CombatProfile = strings.TrimSpace(candidate.CombatProfile)
		candidate.InteractionKind = strings.TrimSpace(candidate.InteractionKind)
		candidate.InteractionRef = strings.TrimSpace(candidate.InteractionRef)
		cloned.Candidate = &candidate
	}
	return cloned
}

func buildInteractableStaticActorDeltas(currentActors []InteractableStaticActorSummary, candidateActors []InteractableStaticActorSummary) []InteractableStaticActorDelta {
	if len(currentActors) == 0 && len(candidateActors) == 0 {
		return nil
	}
	currentByKey := interactableStaticActorMapByAuthoringKey(currentActors)
	candidateByKey := interactableStaticActorMapByAuthoringKey(candidateActors)
	keysSeen := make(map[string]struct{}, len(currentByKey)+len(candidateByKey))
	for key := range currentByKey {
		keysSeen[key] = struct{}{}
	}
	for key := range candidateByKey {
		keysSeen[key] = struct{}{}
	}
	keys := make([]string, 0, len(keysSeen))
	for key := range keysSeen {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	deltas := make([]InteractableStaticActorDelta, 0, len(keys))
	for _, key := range keys {
		current, currentOK := currentByKey[key]
		candidate, candidateOK := candidateByKey[key]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, InteractableStaticActorDelta{Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, InteractableStaticActorDelta{Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, InteractableStaticActorDelta{Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func InteractableStaticActorDeltasByName(deltas []InteractableStaticActorDelta, name string) []InteractableStaticActorDelta {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		return nil
	}
	matches := make([]InteractableStaticActorDelta, 0)
	for _, delta := range deltas {
		if interactableStaticActorDeltaMatchesName(delta, name) {
			matches = append(matches, cloneInteractableStaticActorDelta(delta))
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func interactableStaticActorDeltaMatchesName(delta InteractableStaticActorDelta, name string) bool {
	return (delta.Current != nil && strings.TrimSpace(delta.Current.Name) == name) ||
		(delta.Candidate != nil && strings.TrimSpace(delta.Candidate.Name) == name)
}

func cloneInteractableStaticActorDelta(delta InteractableStaticActorDelta) InteractableStaticActorDelta {
	cloned := delta
	if delta.Current != nil {
		current := normalizeInteractableStaticActorSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := normalizeInteractableStaticActorSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func interactableStaticActorMapByAuthoringKey(actors []InteractableStaticActorSummary) map[string]InteractableStaticActorSummary {
	byKey := make(map[string]InteractableStaticActorSummary, len(actors))
	for _, actor := range actors {
		actor = normalizeInteractableStaticActorSummary(actor)
		byKey[interactableStaticActorAuthoringKey(actor)] = actor
	}
	return byKey
}

func interactableStaticActorAuthoringKey(actor InteractableStaticActorSummary) string {
	return fmt.Sprintf("%s\x00%d\x00%d\x00%d\x00%d\x00%s\x00%s", actor.Name, actor.MapIndex, actor.X, actor.Y, actor.RaceNum, actor.InteractionKind, actor.InteractionRef)
}

func normalizeInteractableStaticActorSummary(actor InteractableStaticActorSummary) InteractableStaticActorSummary {
	actor.Name = strings.TrimSpace(actor.Name)
	actor.InteractionKind = strings.TrimSpace(actor.InteractionKind)
	actor.InteractionRef = strings.TrimSpace(actor.InteractionRef)
	actor.Preview = strings.TrimSpace(actor.Preview)
	return actor
}

func staticActorMapByAuthoringKey(actors []StaticActor) map[string]StaticActor {
	byKey := make(map[string]StaticActor, len(actors))
	for _, actor := range normalizeStaticActors(actors) {
		byKey[staticActorAuthoringKey(actor)] = actor
	}
	return byKey
}

func buildInteractionDefinitionDeltas(current Bundle, candidate Bundle) []InteractionDefinitionDelta {
	if len(current.InteractionDefinitions) == 0 && len(candidate.InteractionDefinitions) == 0 {
		return nil
	}
	currentDefinitions := interactionDefinitionMapByKey(current.InteractionDefinitions)
	candidateDefinitions := interactionDefinitionMapByKey(candidate.InteractionDefinitions)
	currentItemTemplates := itemTemplateMapByVnum(current.ItemTemplates)
	candidateItemTemplates := itemTemplateMapByVnum(candidate.ItemTemplates)

	identitiesByKey := make(map[string]InteractionDefinitionReferenceSummary, len(currentDefinitions)+len(candidateDefinitions))
	for _, definition := range current.InteractionDefinitions {
		definition = interactionstore.NormalizeDefinition(definition)
		identitiesByKey[interactionDefinitionKey(definition.Kind, definition.Ref)] = InteractionDefinitionReferenceSummary{Kind: definition.Kind, Ref: definition.Ref}
	}
	for _, definition := range candidate.InteractionDefinitions {
		definition = interactionstore.NormalizeDefinition(definition)
		identitiesByKey[interactionDefinitionKey(definition.Kind, definition.Ref)] = InteractionDefinitionReferenceSummary{Kind: definition.Kind, Ref: definition.Ref}
	}
	identities := make([]InteractionDefinitionReferenceSummary, 0, len(identitiesByKey))
	for _, identity := range identitiesByKey {
		identities = append(identities, identity)
	}
	sort.Slice(identities, func(i int, j int) bool {
		if identities[i].Kind == identities[j].Kind {
			return identities[i].Ref < identities[j].Ref
		}
		return identities[i].Kind < identities[j].Kind
	})

	deltas := make([]InteractionDefinitionDelta, 0, len(identities))
	for _, identity := range identities {
		key := interactionDefinitionKey(identity.Kind, identity.Ref)
		currentDefinition, currentOK := currentDefinitions[key]
		candidateDefinition, candidateOK := candidateDefinitions[key]
		switch {
		case !currentOK:
			deltas = append(deltas, InteractionDefinitionDelta{
				Kind:             identity.Kind,
				Ref:              identity.Ref,
				Change:           "added",
				CandidatePreview: compactInteractionPreview(interactionDefinitionCatalogPreview(candidateDefinition, candidateItemTemplates)),
			})
		case !candidateOK:
			deltas = append(deltas, InteractionDefinitionDelta{
				Kind:           identity.Kind,
				Ref:            identity.Ref,
				Change:         "removed",
				CurrentPreview: compactInteractionPreview(interactionDefinitionCatalogPreview(currentDefinition, currentItemTemplates)),
			})
		case !reflect.DeepEqual(currentDefinition, candidateDefinition):
			deltas = append(deltas, InteractionDefinitionDelta{
				Kind:             identity.Kind,
				Ref:              identity.Ref,
				Change:           "changed",
				CurrentPreview:   compactInteractionPreview(interactionDefinitionCatalogPreview(currentDefinition, currentItemTemplates)),
				CandidatePreview: compactInteractionPreview(interactionDefinitionCatalogPreview(candidateDefinition, candidateItemTemplates)),
			})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func InteractionDefinitionDeltaByIdentity(deltas []InteractionDefinitionDelta, kind string, ref string) (InteractionDefinitionDelta, bool) {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" || ref == "" || !interactionstore.ValidKind(kind) || !interactionstore.ValidRef(ref) {
		return InteractionDefinitionDelta{}, false
	}
	for _, delta := range deltas {
		if strings.TrimSpace(delta.Kind) == kind && strings.TrimSpace(delta.Ref) == ref {
			return delta, true
		}
	}
	return InteractionDefinitionDelta{}, false
}

func buildItemTemplateDeltas(currentTemplates []itemcatalog.Template, candidateTemplates []itemcatalog.Template) []ItemTemplateDelta {
	if len(currentTemplates) == 0 && len(candidateTemplates) == 0 {
		return nil
	}
	currentByVnum := itemTemplateMapByVnum(currentTemplates)
	candidateByVnum := itemTemplateMapByVnum(candidateTemplates)
	vnumsSeen := make(map[uint32]struct{}, len(currentByVnum)+len(candidateByVnum))
	for vnum := range currentByVnum {
		vnumsSeen[vnum] = struct{}{}
	}
	for vnum := range candidateByVnum {
		vnumsSeen[vnum] = struct{}{}
	}
	vnums := make([]uint32, 0, len(vnumsSeen))
	for vnum := range vnumsSeen {
		vnums = append(vnums, vnum)
	}
	sort.Slice(vnums, func(i int, j int) bool { return vnums[i] < vnums[j] })

	deltas := make([]ItemTemplateDelta, 0, len(vnums))
	for _, vnum := range vnums {
		current, currentOK := currentByVnum[vnum]
		candidate, candidateOK := candidateByVnum[vnum]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, ItemTemplateDelta{Vnum: vnum, Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, ItemTemplateDelta{Vnum: vnum, Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, ItemTemplateDelta{Vnum: vnum, Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func ItemTemplateDeltaByVnum(deltas []ItemTemplateDelta, vnum uint32) (ItemTemplateDelta, bool) {
	if vnum == 0 {
		return ItemTemplateDelta{}, false
	}
	for _, delta := range deltas {
		if delta.Vnum == vnum {
			return cloneItemTemplateDelta(delta), true
		}
	}
	return ItemTemplateDelta{}, false
}

func cloneItemTemplateDelta(delta ItemTemplateDelta) ItemTemplateDelta {
	cloned := delta
	if delta.Current != nil {
		current := itemcatalog.NormalizeTemplate(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := itemcatalog.NormalizeTemplate(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func buildCombatProfileDeltas(currentProfiles []worldruntime.StaticActorCombatProfileSnapshot, candidateProfiles []worldruntime.StaticActorCombatProfileSnapshot) []CombatProfileDelta {
	if len(currentProfiles) == 0 && len(candidateProfiles) == 0 {
		return nil
	}
	currentByProfile := combatProfileSnapshotMapByProfile(currentProfiles)
	candidateByProfile := combatProfileSnapshotMapByProfile(candidateProfiles)
	profilesSeen := make(map[string]struct{}, len(currentByProfile)+len(candidateByProfile))
	for profile := range currentByProfile {
		profilesSeen[profile] = struct{}{}
	}
	for profile := range candidateByProfile {
		profilesSeen[profile] = struct{}{}
	}
	profiles := make([]string, 0, len(profilesSeen))
	for profile := range profilesSeen {
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)

	deltas := make([]CombatProfileDelta, 0, len(profiles))
	for _, profile := range profiles {
		current, currentOK := currentByProfile[profile]
		candidate, candidateOK := candidateByProfile[profile]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, CombatProfileDelta{Profile: profile, Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, CombatProfileDelta{Profile: profile, Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, CombatProfileDelta{Profile: profile, Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func CombatProfileDeltaByProfile(deltas []CombatProfileDelta, profile string) (CombatProfileDelta, bool) {
	profile = strings.TrimSpace(profile)
	if !worldruntime.ValidStaticActorCombatProfileName(profile) {
		return CombatProfileDelta{}, false
	}
	for _, delta := range deltas {
		if strings.TrimSpace(delta.Profile) == profile {
			return cloneCombatProfileDelta(delta), true
		}
	}
	return CombatProfileDelta{}, false
}

func cloneCombatProfileDelta(delta CombatProfileDelta) CombatProfileDelta {
	cloned := delta
	cloned.Profile = strings.TrimSpace(cloned.Profile)
	if delta.Current != nil {
		current := *delta.Current
		current.Profile = strings.TrimSpace(current.Profile)
		current.DeathReward = current.DeathReward.Clone()
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := *delta.Candidate
		candidate.Profile = strings.TrimSpace(candidate.Profile)
		candidate.DeathReward = candidate.DeathReward.Clone()
		cloned.Candidate = &candidate
	}
	return cloned
}

func combatProfileSnapshotMapByProfile(profiles []worldruntime.StaticActorCombatProfileSnapshot) map[string]worldruntime.StaticActorCombatProfileSnapshot {
	byProfile := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	for _, profile := range cloneCombatProfileSnapshots(profiles) {
		if profile.Profile == "" {
			continue
		}
		byProfile[profile.Profile] = profile
	}
	return byProfile
}

func buildSpawnGroupDeltas(currentSpawnGroups []SpawnGroupReferenceSummary, candidateSpawnGroups []SpawnGroupReferenceSummary) []SpawnGroupDelta {
	if len(currentSpawnGroups) == 0 && len(candidateSpawnGroups) == 0 {
		return nil
	}
	currentByRef := spawnGroupSummaryMapByRef(currentSpawnGroups)
	candidateByRef := spawnGroupSummaryMapByRef(candidateSpawnGroups)
	refsSeen := make(map[string]struct{}, len(currentByRef)+len(candidateByRef))
	for ref := range currentByRef {
		refsSeen[ref] = struct{}{}
	}
	for ref := range candidateByRef {
		refsSeen[ref] = struct{}{}
	}
	refs := make([]string, 0, len(refsSeen))
	for ref := range refsSeen {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	deltas := make([]SpawnGroupDelta, 0, len(refs))
	for _, ref := range refs {
		current, currentOK := currentByRef[ref]
		candidate, candidateOK := candidateByRef[ref]
		switch {
		case !currentOK:
			candidateCopy := candidate
			deltas = append(deltas, SpawnGroupDelta{Ref: ref, Change: "added", Candidate: &candidateCopy})
		case !candidateOK:
			currentCopy := current
			deltas = append(deltas, SpawnGroupDelta{Ref: ref, Change: "removed", Current: &currentCopy})
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			deltas = append(deltas, SpawnGroupDelta{Ref: ref, Change: "changed", Current: &currentCopy, Candidate: &candidateCopy})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func SpawnGroupDeltaByRef(deltas []SpawnGroupDelta, ref string) (SpawnGroupDelta, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || !worldruntime.ValidStaticActorSpawnGroupRef(ref) {
		return SpawnGroupDelta{}, false
	}
	for _, delta := range deltas {
		if strings.TrimSpace(delta.Ref) == ref {
			return cloneSpawnGroupDelta(delta), true
		}
	}
	return SpawnGroupDelta{}, false
}

func cloneSpawnGroupDelta(delta SpawnGroupDelta) SpawnGroupDelta {
	cloned := delta
	cloned.Ref = strings.TrimSpace(cloned.Ref)
	if delta.Current != nil {
		current := cloneSpawnGroupReferenceSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := cloneSpawnGroupReferenceSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func cloneSpawnGroupReferenceSummary(spawnGroup SpawnGroupReferenceSummary) SpawnGroupReferenceSummary {
	spawnGroup.Ref = strings.TrimSpace(spawnGroup.Ref)
	spawnGroup.Name = strings.TrimSpace(spawnGroup.Name)
	spawnGroup.CombatProfile = strings.TrimSpace(spawnGroup.CombatProfile)
	spawnGroup.RewardDropVnums = cloneUint32s(spawnGroup.RewardDropVnums)
	spawnGroup.RewardDropItems = cloneRewardDropItemSummaries(spawnGroup.RewardDropItems)
	return spawnGroup
}

func spawnGroupSummaryMapByRef(spawnGroups []SpawnGroupReferenceSummary) map[string]SpawnGroupReferenceSummary {
	byRef := make(map[string]SpawnGroupReferenceSummary, len(spawnGroups))
	for _, spawnGroup := range spawnGroups {
		spawnGroup = cloneSpawnGroupReferenceSummary(spawnGroup)
		byRef[spawnGroup.Ref] = spawnGroup
	}
	return byRef
}

func buildRewardDropDeltas(currentDrops []RewardDropAggregateSummary, candidateDrops []RewardDropAggregateSummary) []RewardDropDelta {
	if len(currentDrops) == 0 && len(candidateDrops) == 0 {
		return nil
	}
	currentByVnum := rewardDropSummaryMapByVnum(currentDrops)
	candidateByVnum := rewardDropSummaryMapByVnum(candidateDrops)
	vnumsSeen := make(map[uint32]struct{}, len(currentDrops)+len(candidateDrops))
	for vnum := range currentByVnum {
		vnumsSeen[vnum] = struct{}{}
	}
	for vnum := range candidateByVnum {
		vnumsSeen[vnum] = struct{}{}
	}
	vnums := make([]uint32, 0, len(vnumsSeen))
	for vnum := range vnumsSeen {
		vnums = append(vnums, vnum)
	}
	sort.Slice(vnums, func(i int, j int) bool { return vnums[i] < vnums[j] })
	deltas := make([]RewardDropDelta, 0, len(vnums))
	for _, vnum := range vnums {
		current, currentOK := currentByVnum[vnum]
		candidate, candidateOK := candidateByVnum[vnum]
		switch {
		case currentOK && candidateOK:
			if reflect.DeepEqual(current, candidate) {
				continue
			}
			deltas = append(deltas, RewardDropDelta{ItemVnum: vnum, Change: "changed", Current: &current, Candidate: &candidate})
		case currentOK:
			deltas = append(deltas, RewardDropDelta{ItemVnum: vnum, Change: "removed", Current: &current})
		case candidateOK:
			deltas = append(deltas, RewardDropDelta{ItemVnum: vnum, Change: "added", Candidate: &candidate})
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func RewardDropDeltaByVnum(deltas []RewardDropDelta, itemVnum uint32) (RewardDropDelta, bool) {
	if itemVnum == 0 {
		return RewardDropDelta{}, false
	}
	for _, delta := range deltas {
		if delta.ItemVnum == itemVnum {
			return cloneRewardDropDelta(delta), true
		}
	}
	return RewardDropDelta{}, false
}

func cloneRewardDropDelta(delta RewardDropDelta) RewardDropDelta {
	cloned := delta
	if delta.Current != nil {
		current := normalizeRewardDropAggregateSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := normalizeRewardDropAggregateSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func rewardDropSummaryMapByVnum(drops []RewardDropAggregateSummary) map[uint32]RewardDropAggregateSummary {
	byVnum := make(map[uint32]RewardDropAggregateSummary, len(drops))
	for _, drop := range drops {
		byVnum[drop.ItemVnum] = drop
	}
	return byVnum
}

func buildShopCatalogDeltas(currentCatalogs []ShopCatalogSummary, candidateCatalogs []ShopCatalogSummary) []ShopCatalogDelta {
	if len(currentCatalogs) == 0 && len(candidateCatalogs) == 0 {
		return nil
	}
	currentByKey := make(map[string]ShopCatalogSummary, len(currentCatalogs))
	candidateByKey := make(map[string]ShopCatalogSummary, len(candidateCatalogs))
	identitiesByKey := make(map[string]InteractionDefinitionReferenceSummary, len(currentCatalogs)+len(candidateCatalogs))
	for _, catalog := range currentCatalogs {
		catalog = normalizeShopCatalogSummary(catalog)
		key := interactionDefinitionKey(catalog.Kind, catalog.Ref)
		currentByKey[key] = catalog
		identitiesByKey[key] = InteractionDefinitionReferenceSummary{Kind: catalog.Kind, Ref: catalog.Ref}
	}
	for _, catalog := range candidateCatalogs {
		catalog = normalizeShopCatalogSummary(catalog)
		key := interactionDefinitionKey(catalog.Kind, catalog.Ref)
		candidateByKey[key] = catalog
		identitiesByKey[key] = InteractionDefinitionReferenceSummary{Kind: catalog.Kind, Ref: catalog.Ref}
	}
	identities := make([]InteractionDefinitionReferenceSummary, 0, len(identitiesByKey))
	for _, identity := range identitiesByKey {
		identities = append(identities, identity)
	}
	sort.Slice(identities, func(i int, j int) bool {
		if identities[i].Kind == identities[j].Kind {
			return identities[i].Ref < identities[j].Ref
		}
		return identities[i].Kind < identities[j].Kind
	})

	deltas := make([]ShopCatalogDelta, 0, len(identities))
	for _, identity := range identities {
		key := interactionDefinitionKey(identity.Kind, identity.Ref)
		current, currentOK := currentByKey[key]
		candidate, candidateOK := candidateByKey[key]
		delta := ShopCatalogDelta{Kind: identity.Kind, Ref: identity.Ref}
		switch {
		case !currentOK:
			candidateCopy := candidate
			delta.Change = "added"
			delta.Candidate = &candidateCopy
		case !candidateOK:
			currentCopy := current
			delta.Change = "removed"
			delta.Current = &currentCopy
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			delta.Change = "changed"
			delta.Current = &currentCopy
			delta.Candidate = &candidateCopy
		default:
			continue
		}
		deltas = append(deltas, delta)
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func ShopCatalogDeltaByIdentity(deltas []ShopCatalogDelta, kind string, ref string) (ShopCatalogDelta, bool) {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" || ref == "" || kind != interactionstore.KindShopPreview || !interactionstore.ValidRef(ref) {
		return ShopCatalogDelta{}, false
	}
	for _, delta := range deltas {
		if strings.TrimSpace(delta.Kind) == kind && strings.TrimSpace(delta.Ref) == ref {
			return cloneShopCatalogDelta(delta), true
		}
	}
	return ShopCatalogDelta{}, false
}

func cloneShopCatalogDelta(delta ShopCatalogDelta) ShopCatalogDelta {
	cloned := delta
	if delta.Current != nil {
		current := normalizeShopCatalogSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := normalizeShopCatalogSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func normalizeShopCatalogSummary(catalog ShopCatalogSummary) ShopCatalogSummary {
	catalog.Kind = strings.TrimSpace(catalog.Kind)
	catalog.Ref = strings.TrimSpace(catalog.Ref)
	catalog.Title = strings.TrimSpace(catalog.Title)
	catalog.Entries = cloneShopCatalogEntrySummaries(catalog.Entries)
	catalog.EntryCount = len(catalog.Entries)
	return catalog
}

func buildWarpDestinationDeltas(currentDestinations []WarpDestinationSummary, candidateDestinations []WarpDestinationSummary) []WarpDestinationDelta {
	if len(currentDestinations) == 0 && len(candidateDestinations) == 0 {
		return nil
	}
	currentByKey := make(map[string]WarpDestinationSummary, len(currentDestinations))
	candidateByKey := make(map[string]WarpDestinationSummary, len(candidateDestinations))
	identitiesByKey := make(map[string]InteractionDefinitionReferenceSummary, len(currentDestinations)+len(candidateDestinations))
	for _, destination := range currentDestinations {
		destination = normalizeWarpDestinationSummary(destination)
		key := interactionDefinitionKey(destination.Kind, destination.Ref)
		currentByKey[key] = destination
		identitiesByKey[key] = InteractionDefinitionReferenceSummary{Kind: destination.Kind, Ref: destination.Ref}
	}
	for _, destination := range candidateDestinations {
		destination = normalizeWarpDestinationSummary(destination)
		key := interactionDefinitionKey(destination.Kind, destination.Ref)
		candidateByKey[key] = destination
		identitiesByKey[key] = InteractionDefinitionReferenceSummary{Kind: destination.Kind, Ref: destination.Ref}
	}
	identities := make([]InteractionDefinitionReferenceSummary, 0, len(identitiesByKey))
	for _, identity := range identitiesByKey {
		identities = append(identities, identity)
	}
	sort.Slice(identities, func(i int, j int) bool {
		if identities[i].Kind == identities[j].Kind {
			return identities[i].Ref < identities[j].Ref
		}
		return identities[i].Kind < identities[j].Kind
	})

	deltas := make([]WarpDestinationDelta, 0, len(identities))
	for _, identity := range identities {
		key := interactionDefinitionKey(identity.Kind, identity.Ref)
		current, currentOK := currentByKey[key]
		candidate, candidateOK := candidateByKey[key]
		delta := WarpDestinationDelta{Kind: identity.Kind, Ref: identity.Ref}
		switch {
		case !currentOK:
			candidateCopy := candidate
			delta.Change = "added"
			delta.Candidate = &candidateCopy
		case !candidateOK:
			currentCopy := current
			delta.Change = "removed"
			delta.Current = &currentCopy
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			delta.Change = "changed"
			delta.Current = &currentCopy
			delta.Candidate = &candidateCopy
		default:
			continue
		}
		deltas = append(deltas, delta)
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func WarpDestinationDeltaByIdentity(deltas []WarpDestinationDelta, kind string, ref string) (WarpDestinationDelta, bool) {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" || ref == "" || kind != interactionstore.KindWarp || !interactionstore.ValidRef(ref) {
		return WarpDestinationDelta{}, false
	}
	for _, delta := range deltas {
		if strings.TrimSpace(delta.Kind) == kind && strings.TrimSpace(delta.Ref) == ref {
			return cloneWarpDestinationDelta(delta), true
		}
	}
	return WarpDestinationDelta{}, false
}

func cloneWarpDestinationDelta(delta WarpDestinationDelta) WarpDestinationDelta {
	cloned := delta
	if delta.Current != nil {
		current := normalizeWarpDestinationSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := normalizeWarpDestinationSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func normalizeWarpDestinationSummary(destination WarpDestinationSummary) WarpDestinationSummary {
	destination.Kind = strings.TrimSpace(destination.Kind)
	destination.Ref = strings.TrimSpace(destination.Ref)
	destination.Text = strings.TrimSpace(destination.Text)
	return destination
}

func cloneShopCatalogEntrySummaries(entries []ShopCatalogEntrySummary) []ShopCatalogEntrySummary {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]ShopCatalogEntrySummary, len(entries))
	copy(cloned, entries)
	for i := range cloned {
		cloned[i].UseEffect = cloneItemTemplateUseEffect(cloned[i].UseEffect)
		cloned[i].EquipEffect = cloneItemTemplatePointEffect(cloned[i].EquipEffect)
	}
	sort.Slice(cloned, func(i int, j int) bool {
		if cloned[i].Slot == cloned[j].Slot {
			return cloned[i].ItemVnum < cloned[j].ItemVnum
		}
		return cloned[i].Slot < cloned[j].Slot
	})
	return cloned
}

type serviceRouteIdentity struct {
	actorName      string
	sourceMapIndex uint32
	sourceX        int32
	sourceY        int32
	ref            string
}

func buildShopRouteDeltas(currentRoutes []ShopRouteSummary, candidateRoutes []ShopRouteSummary) []ShopRouteDelta {
	if len(currentRoutes) == 0 && len(candidateRoutes) == 0 {
		return nil
	}
	currentByID := make(map[serviceRouteIdentity]ShopRouteSummary, len(currentRoutes))
	candidateByID := make(map[serviceRouteIdentity]ShopRouteSummary, len(candidateRoutes))
	idsSeen := make(map[serviceRouteIdentity]struct{}, len(currentRoutes)+len(candidateRoutes))
	for _, route := range currentRoutes {
		route = normalizeShopRouteSummary(route)
		id := shopRouteIdentity(route)
		currentByID[id] = route
		idsSeen[id] = struct{}{}
	}
	for _, route := range candidateRoutes {
		route = normalizeShopRouteSummary(route)
		id := shopRouteIdentity(route)
		candidateByID[id] = route
		idsSeen[id] = struct{}{}
	}
	ids := sortedServiceRouteIdentities(idsSeen)
	deltas := make([]ShopRouteDelta, 0, len(ids))
	for _, id := range ids {
		current, currentOK := currentByID[id]
		candidate, candidateOK := candidateByID[id]
		delta := ShopRouteDelta{ActorName: id.actorName, SourceMapIndex: id.sourceMapIndex, SourceX: id.sourceX, SourceY: id.sourceY, Ref: id.ref}
		switch {
		case !currentOK:
			candidateCopy := candidate
			delta.Change = "added"
			delta.Candidate = &candidateCopy
		case !candidateOK:
			currentCopy := current
			delta.Change = "removed"
			delta.Current = &currentCopy
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			delta.Change = "changed"
			delta.Current = &currentCopy
			delta.Candidate = &candidateCopy
		default:
			continue
		}
		deltas = append(deltas, delta)
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func buildWarpRouteDeltas(currentRoutes []WarpRouteSummary, candidateRoutes []WarpRouteSummary) []WarpRouteDelta {
	if len(currentRoutes) == 0 && len(candidateRoutes) == 0 {
		return nil
	}
	currentByID := make(map[serviceRouteIdentity]WarpRouteSummary, len(currentRoutes))
	candidateByID := make(map[serviceRouteIdentity]WarpRouteSummary, len(candidateRoutes))
	idsSeen := make(map[serviceRouteIdentity]struct{}, len(currentRoutes)+len(candidateRoutes))
	for _, route := range currentRoutes {
		route = normalizeWarpRouteSummary(route)
		id := warpRouteIdentity(route)
		currentByID[id] = route
		idsSeen[id] = struct{}{}
	}
	for _, route := range candidateRoutes {
		route = normalizeWarpRouteSummary(route)
		id := warpRouteIdentity(route)
		candidateByID[id] = route
		idsSeen[id] = struct{}{}
	}
	ids := sortedServiceRouteIdentities(idsSeen)
	deltas := make([]WarpRouteDelta, 0, len(ids))
	for _, id := range ids {
		current, currentOK := currentByID[id]
		candidate, candidateOK := candidateByID[id]
		delta := WarpRouteDelta{ActorName: id.actorName, SourceMapIndex: id.sourceMapIndex, SourceX: id.sourceX, SourceY: id.sourceY, Ref: id.ref}
		switch {
		case !currentOK:
			candidateCopy := candidate
			delta.Change = "added"
			delta.Candidate = &candidateCopy
		case !candidateOK:
			currentCopy := current
			delta.Change = "removed"
			delta.Current = &currentCopy
		case !reflect.DeepEqual(current, candidate):
			currentCopy := current
			candidateCopy := candidate
			delta.Change = "changed"
			delta.Current = &currentCopy
			delta.Candidate = &candidateCopy
		default:
			continue
		}
		deltas = append(deltas, delta)
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func ShopRouteDeltasByActorName(deltas []ShopRouteDelta, actorName string) []ShopRouteDelta {
	actorName = strings.TrimSpace(actorName)
	if actorName == "" || strings.Contains(actorName, "/") {
		return nil
	}
	matches := make([]ShopRouteDelta, 0)
	for _, delta := range deltas {
		if shopRouteDeltaMatchesActorName(delta, actorName) {
			matches = append(matches, cloneShopRouteDelta(delta))
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func shopRouteDeltaMatchesActorName(delta ShopRouteDelta, actorName string) bool {
	return strings.TrimSpace(delta.ActorName) == actorName ||
		(delta.Current != nil && strings.TrimSpace(delta.Current.ActorName) == actorName) ||
		(delta.Candidate != nil && strings.TrimSpace(delta.Candidate.ActorName) == actorName)
}

func cloneShopRouteDelta(delta ShopRouteDelta) ShopRouteDelta {
	cloned := delta
	cloned.ActorName = strings.TrimSpace(cloned.ActorName)
	cloned.Ref = strings.TrimSpace(cloned.Ref)
	cloned.Change = strings.TrimSpace(cloned.Change)
	if delta.Current != nil {
		current := normalizeShopRouteSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := normalizeShopRouteSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func WarpRouteDeltasByActorName(deltas []WarpRouteDelta, actorName string) []WarpRouteDelta {
	actorName = strings.TrimSpace(actorName)
	if actorName == "" || strings.Contains(actorName, "/") {
		return nil
	}
	matches := make([]WarpRouteDelta, 0)
	for _, delta := range deltas {
		if warpRouteDeltaMatchesActorName(delta, actorName) {
			matches = append(matches, cloneWarpRouteDelta(delta))
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func warpRouteDeltaMatchesActorName(delta WarpRouteDelta, actorName string) bool {
	return strings.TrimSpace(delta.ActorName) == actorName ||
		(delta.Current != nil && strings.TrimSpace(delta.Current.ActorName) == actorName) ||
		(delta.Candidate != nil && strings.TrimSpace(delta.Candidate.ActorName) == actorName)
}

func cloneWarpRouteDelta(delta WarpRouteDelta) WarpRouteDelta {
	cloned := delta
	cloned.ActorName = strings.TrimSpace(cloned.ActorName)
	cloned.Ref = strings.TrimSpace(cloned.Ref)
	cloned.Change = strings.TrimSpace(cloned.Change)
	if delta.Current != nil {
		current := normalizeWarpRouteSummary(*delta.Current)
		cloned.Current = &current
	}
	if delta.Candidate != nil {
		candidate := normalizeWarpRouteSummary(*delta.Candidate)
		cloned.Candidate = &candidate
	}
	return cloned
}

func normalizeShopRouteSummary(route ShopRouteSummary) ShopRouteSummary {
	route.ActorName = strings.TrimSpace(route.ActorName)
	route.Ref = strings.TrimSpace(route.Ref)
	route.Title = strings.TrimSpace(route.Title)
	return route
}

func normalizeWarpRouteSummary(route WarpRouteSummary) WarpRouteSummary {
	route.ActorName = strings.TrimSpace(route.ActorName)
	route.Ref = strings.TrimSpace(route.Ref)
	route.Text = strings.TrimSpace(route.Text)
	return route
}

func shopRouteIdentity(route ShopRouteSummary) serviceRouteIdentity {
	return serviceRouteIdentity{
		actorName:      route.ActorName,
		sourceMapIndex: route.SourceMapIndex,
		sourceX:        route.SourceX,
		sourceY:        route.SourceY,
		ref:            route.Ref,
	}
}

func warpRouteIdentity(route WarpRouteSummary) serviceRouteIdentity {
	return serviceRouteIdentity{
		actorName:      route.ActorName,
		sourceMapIndex: route.SourceMapIndex,
		sourceX:        route.SourceX,
		sourceY:        route.SourceY,
		ref:            route.Ref,
	}
}

func sortedServiceRouteIdentities(idsSeen map[serviceRouteIdentity]struct{}) []serviceRouteIdentity {
	ids := make([]serviceRouteIdentity, 0, len(idsSeen))
	for id := range idsSeen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i int, j int) bool {
		if ids[i].actorName == ids[j].actorName {
			if ids[i].sourceMapIndex == ids[j].sourceMapIndex {
				if ids[i].sourceX == ids[j].sourceX {
					if ids[i].sourceY == ids[j].sourceY {
						return ids[i].ref < ids[j].ref
					}
					return ids[i].sourceY < ids[j].sourceY
				}
				return ids[i].sourceX < ids[j].sourceX
			}
			return ids[i].sourceMapIndex < ids[j].sourceMapIndex
		}
		return ids[i].actorName < ids[j].actorName
	})
	return ids
}

func MapContentDeltaByIndex(deltas []MapContentDelta, mapIndex uint32) (MapContentDelta, bool) {
	if mapIndex == 0 {
		return MapContentDelta{}, false
	}
	for _, delta := range deltas {
		if delta.MapIndex == mapIndex {
			return cloneMapContentDelta(delta), true
		}
	}
	return MapContentDelta{}, false
}

func cloneMapContentDelta(delta MapContentDelta) MapContentDelta {
	cloned := delta
	if len(delta.StaticActors) > 0 {
		cloned.StaticActors = make([]StaticActorDelta, len(delta.StaticActors))
		for i, actorDelta := range delta.StaticActors {
			cloned.StaticActors[i] = cloneStaticActorDelta(actorDelta)
		}
	}
	if len(delta.SpawnGroups) > 0 {
		cloned.SpawnGroups = make([]SpawnGroupDelta, len(delta.SpawnGroups))
		for i, spawnGroupDelta := range delta.SpawnGroups {
			cloned.SpawnGroups[i] = cloneSpawnGroupDelta(spawnGroupDelta)
		}
	}
	if len(delta.ShopRoutes) > 0 {
		cloned.ShopRoutes = make([]ShopRouteDelta, len(delta.ShopRoutes))
		for i, routeDelta := range delta.ShopRoutes {
			cloned.ShopRoutes[i] = cloneShopRouteDelta(routeDelta)
		}
	}
	if len(delta.WarpRoutes) > 0 {
		cloned.WarpRoutes = make([]WarpRouteDelta, len(delta.WarpRoutes))
		for i, routeDelta := range delta.WarpRoutes {
			cloned.WarpRoutes[i] = cloneWarpRouteDelta(routeDelta)
		}
	}
	return cloned
}

func buildMapContentDeltas(current Summary, candidate Summary, currentBundle Bundle, candidateBundle Bundle) []MapContentDelta {
	currentMaps := current.Maps
	candidateMaps := candidate.Maps
	if len(currentMaps) == 0 && len(candidateMaps) == 0 {
		return nil
	}
	currentByIndex := make(map[uint32]MapContentSummary, len(currentMaps))
	candidateByIndex := make(map[uint32]MapContentSummary, len(candidateMaps))
	indexesSeen := make(map[uint32]struct{}, len(currentMaps)+len(candidateMaps))
	for _, summary := range currentMaps {
		currentByIndex[summary.MapIndex] = summary
		indexesSeen[summary.MapIndex] = struct{}{}
	}
	for _, summary := range candidateMaps {
		candidateByIndex[summary.MapIndex] = summary
		indexesSeen[summary.MapIndex] = struct{}{}
	}
	indexes := make([]uint32, 0, len(indexesSeen))
	for index := range indexesSeen {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i int, j int) bool { return indexes[i] < indexes[j] })
	deltas := make([]MapContentDelta, 0, len(indexes))
	for _, index := range indexes {
		currentMap := currentByIndex[index]
		candidateMap := candidateByIndex[index]
		delta := MapContentDelta{
			MapIndex:                     index,
			StaticActorCount:             summaryCountDelta(currentMap.StaticActorCount, candidateMap.StaticActorCount),
			InteractableStaticActorCount: summaryCountDelta(currentMap.InteractableStaticActorCount, candidateMap.InteractableStaticActorCount),
			InfoActorCount:               summaryCountDelta(currentMap.InfoActorCount, candidateMap.InfoActorCount),
			TalkActorCount:               summaryCountDelta(currentMap.TalkActorCount, candidateMap.TalkActorCount),
			ShopPreviewActorCount:        summaryCountDelta(currentMap.ShopPreviewActorCount, candidateMap.ShopPreviewActorCount),
			ShopCatalogEntryCount:        summaryCountDelta(currentMap.ShopCatalogEntryCount, candidateMap.ShopCatalogEntryCount),
			WarpActorCount:               summaryCountDelta(currentMap.WarpActorCount, candidateMap.WarpActorCount),
			SpawnGroupCount:              summaryCountDelta(currentMap.SpawnGroupCount, candidateMap.SpawnGroupCount),
			RewardExperienceTotal:        summaryAmountDelta(currentMap.RewardExperienceTotal, candidateMap.RewardExperienceTotal),
			RewardGoldTotal:              summaryAmountDelta(currentMap.RewardGoldTotal, candidateMap.RewardGoldTotal),
			RewardDropItemCount:          summaryCountDelta(currentMap.RewardDropItemCount, candidateMap.RewardDropItemCount),
			StaticActors:                 buildStaticActorDeltas(staticActorsForMap(currentBundle.StaticActors, index), staticActorsForMap(candidateBundle.StaticActors, index)),
			SpawnGroups:                  buildSpawnGroupDeltas(spawnGroupSummariesForMap(current.SpawnGroups, index), spawnGroupSummariesForMap(candidate.SpawnGroups, index)),
			ShopRoutes:                   buildShopRouteDeltas(shopRouteSummariesForMap(current.ShopRoutes, index), shopRouteSummariesForMap(candidate.ShopRoutes, index)),
			WarpRoutes:                   buildWarpRouteDeltas(warpRouteSummariesForMap(current.WarpRoutes, index), warpRouteSummariesForMap(candidate.WarpRoutes, index)),
		}
		if !mapContentDeltaIsZero(delta) {
			deltas = append(deltas, delta)
		}
	}
	if len(deltas) == 0 {
		return nil
	}
	return deltas
}

func staticActorsForMap(actors []StaticActor, mapIndex uint32) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	filtered := make([]StaticActor, 0, len(actors))
	for _, actor := range actors {
		if actor.MapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, actor)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func spawnGroupSummariesForMap(spawnGroups []SpawnGroupReferenceSummary, mapIndex uint32) []SpawnGroupReferenceSummary {
	if len(spawnGroups) == 0 {
		return nil
	}
	filtered := make([]SpawnGroupReferenceSummary, 0, len(spawnGroups))
	for _, spawnGroup := range spawnGroups {
		if spawnGroup.MapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, spawnGroup)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func shopRouteSummariesForMap(routes []ShopRouteSummary, mapIndex uint32) []ShopRouteSummary {
	if len(routes) == 0 {
		return nil
	}
	filtered := make([]ShopRouteSummary, 0, len(routes))
	for _, route := range routes {
		route = normalizeShopRouteSummary(route)
		if route.SourceMapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, route)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func warpRouteSummariesForMap(routes []WarpRouteSummary, mapIndex uint32) []WarpRouteSummary {
	if len(routes) == 0 {
		return nil
	}
	filtered := make([]WarpRouteSummary, 0, len(routes))
	for _, route := range routes {
		route = normalizeWarpRouteSummary(route)
		if route.SourceMapIndex != mapIndex {
			continue
		}
		filtered = append(filtered, route)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func mapContentDeltaIsZero(delta MapContentDelta) bool {
	return delta.StaticActorCount.Delta == 0 &&
		delta.InteractableStaticActorCount.Delta == 0 &&
		delta.InfoActorCount.Delta == 0 &&
		delta.TalkActorCount.Delta == 0 &&
		delta.ShopPreviewActorCount.Delta == 0 &&
		delta.ShopCatalogEntryCount.Delta == 0 &&
		delta.WarpActorCount.Delta == 0 &&
		delta.SpawnGroupCount.Delta == 0 &&
		delta.RewardExperienceTotal.Delta == 0 &&
		delta.RewardGoldTotal.Delta == 0 &&
		delta.RewardDropItemCount.Delta == 0 &&
		len(delta.StaticActors) == 0 &&
		len(delta.SpawnGroups) == 0 &&
		len(delta.ShopRoutes) == 0 &&
		len(delta.WarpRoutes) == 0
}

func Summarize(bundle Bundle) (Summary, error) {
	normalized, err := Canonicalize(bundle)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		StaticActorCount:           len(normalized.StaticActors),
		SpawnGroupCount:            len(normalized.SpawnGroups),
		CombatProfileCount:         len(normalized.CombatProfiles),
		CombatProfiles:             cloneCombatProfileSnapshots(normalized.CombatProfiles),
		ItemTemplateCount:          len(normalized.ItemTemplates),
		StaticActors:               cloneStaticActors(normalized.StaticActors),
		InteractionDefinitionCount: len(normalized.InteractionDefinitions),
	}
	questStateSummary, err := queststate.SummarizeSnapshot(queststate.Snapshot{Flags: normalized.QuestState})
	if err != nil {
		return Summary{}, ErrInvalidBundle
	}
	summary.QuestStateFlagCount = questStateSummary.FlagCount
	summary.QuestStateCharacterCount = len(questStateSummary.Characters)
	summary.QuestStateQuestCount = len(questStateSummary.QuestRefs)
	if summary.QuestStateFlagCount > 0 {
		summary.QuestStateQuestRefs = questStateSummary.QuestRefs
	}
	summary.QuestStateCharacters = questStateCharacterSummaries(normalized.QuestState)
	summary.QuestStateQuests = questStateQuestSummaries(normalized.QuestState)
	itemTemplatesByVnum := itemTemplateMapByVnum(normalized.ItemTemplates)
	definitionsByKey := interactionDefinitionMapByKey(normalized.InteractionDefinitions)

	referencedDefinitions := make(map[string]struct{})
	for _, actor := range normalized.StaticActors {
		if actor.InteractionKind != "" && actor.InteractionRef != "" {
			referencedDefinitions[interactionDefinitionKey(actor.InteractionKind, actor.InteractionRef)] = struct{}{}
		}
	}
	summary.ReferencedInteractionDefinitionCount = len(referencedDefinitions)
	summary.UnreferencedInteractionDefinitionCount = summary.InteractionDefinitionCount - summary.ReferencedInteractionDefinitionCount

	interactionKindCounts := make(map[string]int)
	interactionKindReferencedCounts := make(map[string]int)
	interactionKindUnreferencedCounts := make(map[string]int)
	for _, definition := range normalized.InteractionDefinitions {
		interactionKindCounts[definition.Kind]++
		summary.InteractionDefinitionPreviews = append(summary.InteractionDefinitionPreviews, interactionDefinitionPreviewSummary(definition, itemTemplatesByVnum))
		if definition.Kind == interactionstore.KindShopPreview {
			summary.ShopCatalogEntryCount += len(definition.Catalog)
			summary.ShopCatalogs = append(summary.ShopCatalogs, shopCatalogSummary(definition, itemTemplatesByVnum))
		}
		if definition.Kind == interactionstore.KindWarp {
			summary.WarpDestinationCount++
			summary.WarpDestinations = append(summary.WarpDestinations, warpDestinationSummary(definition))
		}
		reference := InteractionDefinitionReferenceSummary{Kind: definition.Kind, Ref: definition.Ref}
		if _, ok := referencedDefinitions[interactionDefinitionKey(definition.Kind, definition.Ref)]; ok {
			interactionKindReferencedCounts[definition.Kind]++
			summary.ReferencedInteractionDefinitions = append(summary.ReferencedInteractionDefinitions, reference)
			continue
		}
		interactionKindUnreferencedCounts[definition.Kind]++
		summary.UnreferencedInteractionDefinitions = append(summary.UnreferencedInteractionDefinitions, reference)
	}
	summary.ItemTemplates = itemTemplateReferenceSummaries(normalized.ItemTemplates)
	if len(interactionKindCounts) > 0 {
		kinds := make([]string, 0, len(interactionKindCounts))
		for kind := range interactionKindCounts {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		summary.InteractionKinds = make([]InteractionKindSummary, 0, len(kinds))
		for _, kind := range kinds {
			summary.InteractionKinds = append(summary.InteractionKinds, InteractionKindSummary{
				Kind:              kind,
				Count:             interactionKindCounts[kind],
				ReferencedCount:   interactionKindReferencedCounts[kind],
				UnreferencedCount: interactionKindUnreferencedCounts[kind],
			})
		}
	}

	mapCounts := make(map[uint32]*MapContentSummary)
	for _, actor := range normalized.StaticActors {
		entry := mapContentSummaryForIndex(mapCounts, actor.MapIndex)
		entry.StaticActorCount++
		if actor.InteractionKind != "" && actor.InteractionRef != "" {
			summary.InteractableStaticActorCount++
			entry.InteractableStaticActorCount++
			definition := definitionsByKey[interactionDefinitionKey(actor.InteractionKind, actor.InteractionRef)]
			addMapServiceInteractionSummary(entry, definition)
			if definition.Kind == interactionstore.KindShopPreview {
				summary.ShopRoutes = append(summary.ShopRoutes, shopRouteSummary(actor, definition))
			}
			if definition.Kind == interactionstore.KindWarp {
				summary.WarpRoutes = append(summary.WarpRoutes, warpRouteSummary(actor, definition))
			}
			summary.InteractableStaticActors = append(summary.InteractableStaticActors, interactableStaticActorSummary(actor, definition, itemTemplatesByVnum))
		}
	}
	summary.ShopRouteCount = len(summary.ShopRoutes)
	summary.WarpRouteCount = len(summary.WarpRoutes)
	rewardDropCountsByVnum := make(map[uint32]int)
	for _, spawnGroup := range normalized.SpawnGroups {
		entry := mapContentSummaryForIndex(mapCounts, spawnGroup.MapIndex)
		entry.SpawnGroupCount++
		entry.RewardExperienceTotal += spawnGroup.RewardExperience
		entry.RewardGoldTotal += spawnGroup.RewardGold
		entry.RewardDropItemCount += len(spawnGroup.RewardDropVnums)
		summary.RewardExperienceTotal += spawnGroup.RewardExperience
		summary.RewardGoldTotal += spawnGroup.RewardGold
		summary.RewardDropItemCount += len(spawnGroup.RewardDropVnums)
		for _, vnum := range spawnGroup.RewardDropVnums {
			rewardDropCountsByVnum[vnum]++
		}
		summary.SpawnGroups = append(summary.SpawnGroups, SpawnGroupReferenceSummary{
			Ref:              spawnGroup.Ref,
			Name:             spawnGroup.Name,
			MapIndex:         spawnGroup.MapIndex,
			X:                spawnGroup.X,
			Y:                spawnGroup.Y,
			RaceNum:          spawnGroup.RaceNum,
			CombatProfile:    spawnGroup.CombatProfile,
			RewardExperience: spawnGroup.RewardExperience,
			RewardGold:       spawnGroup.RewardGold,
			RewardDropVnums:  cloneUint32s(spawnGroup.RewardDropVnums),
			RewardDropItems:  rewardDropItemSummaries(spawnGroup.RewardDropVnums, itemTemplatesByVnum),
		})
	}
	summary.RewardDrops = rewardDropAggregateSummaries(rewardDropCountsByVnum, itemTemplatesByVnum)
	if len(mapCounts) > 0 {
		mapIndexes := make([]uint32, 0, len(mapCounts))
		for mapIndex := range mapCounts {
			mapIndexes = append(mapIndexes, mapIndex)
		}
		sort.Slice(mapIndexes, func(i int, j int) bool { return mapIndexes[i] < mapIndexes[j] })
		summary.Maps = make([]MapContentSummary, 0, len(mapIndexes))
		for _, mapIndex := range mapIndexes {
			summary.Maps = append(summary.Maps, *mapCounts[mapIndex])
		}
	}

	return summary, nil
}

func itemTemplateMapByVnum(templates []itemcatalog.Template) map[uint32]itemcatalog.Template {
	byVnum := make(map[uint32]itemcatalog.Template, len(templates))
	for _, template := range templates {
		template = itemcatalog.NormalizeTemplate(template)
		byVnum[template.Vnum] = template
	}
	return byVnum
}

func interactionDefinitionMapByKey(definitions []interactionstore.Definition) map[string]interactionstore.Definition {
	byKey := make(map[string]interactionstore.Definition, len(definitions))
	for _, definition := range definitions {
		definition = interactionstore.NormalizeDefinition(definition)
		byKey[interactionDefinitionKey(definition.Kind, definition.Ref)] = definition
	}
	return byKey
}

func questStateCharacterSummaries(flags []queststate.Flag) []QuestStateCharacterSummary {
	flags = normalizeQuestStateFlags(flags)
	if len(flags) == 0 {
		return nil
	}
	byCharacter := make(map[string][]queststate.FlagSnapshot)
	for _, flag := range flags {
		byCharacter[flag.Character] = append(byCharacter[flag.Character], queststate.FlagSnapshot{QuestRef: flag.QuestRef, Name: flag.Name, Value: flag.Value})
	}
	characters := make([]string, 0, len(byCharacter))
	for character := range byCharacter {
		characters = append(characters, character)
	}
	sort.Strings(characters)
	summaries := make([]QuestStateCharacterSummary, 0, len(characters))
	for _, character := range characters {
		characterFlags := byCharacter[character]
		summaries = append(summaries, QuestStateCharacterSummary{
			Character: character,
			FlagCount: len(characterFlags),
			Flags:     characterFlags,
		})
	}
	return summaries
}

func questStateQuestSummaries(flags []queststate.Flag) []QuestStateQuestSummary {
	flags = normalizeQuestStateFlags(flags)
	if len(flags) == 0 {
		return nil
	}
	type questCharacterFlags map[string][]queststate.FlagSnapshot
	byQuest := make(map[string]questCharacterFlags)
	for _, flag := range flags {
		if _, ok := byQuest[flag.QuestRef]; !ok {
			byQuest[flag.QuestRef] = make(questCharacterFlags)
		}
		byQuest[flag.QuestRef][flag.Character] = append(byQuest[flag.QuestRef][flag.Character], queststate.FlagSnapshot{QuestRef: flag.QuestRef, Name: flag.Name, Value: flag.Value})
	}
	questRefs := make([]string, 0, len(byQuest))
	for questRef := range byQuest {
		questRefs = append(questRefs, questRef)
	}
	sort.Strings(questRefs)
	summaries := make([]QuestStateQuestSummary, 0, len(questRefs))
	for _, questRef := range questRefs {
		charactersByName := byQuest[questRef]
		characters := make([]string, 0, len(charactersByName))
		for character := range charactersByName {
			characters = append(characters, character)
		}
		sort.Strings(characters)
		questSummary := QuestStateQuestSummary{QuestRef: questRef}
		for _, character := range characters {
			characterFlags := charactersByName[character]
			questSummary.FlagCount += len(characterFlags)
			questSummary.Characters = append(questSummary.Characters, QuestStateCharacterSummary{
				Character: character,
				FlagCount: len(characterFlags),
				Flags:     characterFlags,
			})
		}
		summaries = append(summaries, questSummary)
	}
	return summaries
}

func QuestStateOverviewFromSummary(summary Summary) QuestStateOverview {
	return QuestStateOverview{
		FlagCount:      summary.QuestStateFlagCount,
		CharacterCount: summary.QuestStateCharacterCount,
		QuestCount:     summary.QuestStateQuestCount,
		QuestRefs:      cloneStrings(summary.QuestStateQuestRefs),
		Characters:     cloneQuestStateCharacterSummaries(summary.QuestStateCharacters),
		Quests:         cloneQuestStateQuestSummaries(summary.QuestStateQuests),
	}
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneQuestStateQuestSummaries(summaries []QuestStateQuestSummary) []QuestStateQuestSummary {
	if len(summaries) == 0 {
		return nil
	}
	cloned := make([]QuestStateQuestSummary, len(summaries))
	copy(cloned, summaries)
	for i := range cloned {
		cloned[i].Characters = cloneQuestStateCharacterSummaries(summaries[i].Characters)
	}
	return cloned
}

func cloneQuestStateCharacterSummaries(summaries []QuestStateCharacterSummary) []QuestStateCharacterSummary {
	if len(summaries) == 0 {
		return nil
	}
	cloned := make([]QuestStateCharacterSummary, len(summaries))
	copy(cloned, summaries)
	for i := range cloned {
		cloned[i].Flags = cloneQuestStateFlagSnapshots(summaries[i].Flags)
	}
	return cloned
}

func cloneQuestStateFlagSnapshots(flags []queststate.FlagSnapshot) []queststate.FlagSnapshot {
	if len(flags) == 0 {
		return nil
	}
	cloned := make([]queststate.FlagSnapshot, len(flags))
	copy(cloned, flags)
	return cloned
}

func interactableStaticActorSummary(actor StaticActor, definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) InteractableStaticActorSummary {
	actor = normalizeStaticActors([]StaticActor{actor})[0]
	definition = interactionstore.NormalizeDefinition(definition)
	return InteractableStaticActorSummary{
		Name:            actor.Name,
		MapIndex:        actor.MapIndex,
		X:               actor.X,
		Y:               actor.Y,
		RaceNum:         actor.RaceNum,
		InteractionKind: actor.InteractionKind,
		InteractionRef:  actor.InteractionRef,
		Preview:         compactInteractionPreview(interactionDefinitionPreview(actor.Name, definition, itemTemplatesByVnum)),
	}
}

func interactionDefinitionPreview(actorName string, definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) string {
	switch definition.Kind {
	case interactionstore.KindInfo:
		return definition.Text
	case interactionstore.KindTalk:
		return fmt.Sprintf("%s:\n%s", actorName, definition.Text)
	case interactionstore.KindShopPreview:
		return shopCatalogPreview(definition, itemTemplatesByVnum)
	case interactionstore.KindWarp:
		return warpDestinationPreview(definition)
	default:
		return ""
	}
}

func interactionDefinitionPreviewSummary(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) InteractionDefinitionPreviewSummary {
	definition = interactionstore.NormalizeDefinition(definition)
	return InteractionDefinitionPreviewSummary{
		Kind:    definition.Kind,
		Ref:     definition.Ref,
		Preview: compactInteractionPreview(interactionDefinitionCatalogPreview(definition, itemTemplatesByVnum)),
	}
}

func interactionDefinitionCatalogPreview(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) string {
	switch definition.Kind {
	case interactionstore.KindInfo, interactionstore.KindTalk:
		return definition.Text
	case interactionstore.KindShopPreview:
		return shopCatalogPreview(definition, itemTemplatesByVnum)
	case interactionstore.KindWarp:
		return warpDestinationPreview(definition)
	default:
		return ""
	}
}

func shopCatalogPreview(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) string {
	if definition.Kind != interactionstore.KindShopPreview || len(definition.Catalog) == 0 {
		return ""
	}
	entries := make([]string, 0, len(definition.Catalog))
	for _, entry := range definition.Catalog {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[entry.ItemVnum])
		if template.Name == "" {
			return ""
		}
		entries = append(entries, fmt.Sprintf("[%d] %s x%d @ %dg", entry.Slot, template.Name, entry.Count, entry.Price))
	}
	return fmt.Sprintf("%s: %s", definition.Title, strings.Join(entries, "; "))
}

func warpDestinationPreview(definition interactionstore.Definition) string {
	summary := fmt.Sprintf("warp -> map %d @ %d,%d", definition.MapIndex, definition.X, definition.Y)
	message := strings.TrimSpace(definition.Text)
	if message == "" {
		return summary
	}
	return fmt.Sprintf("%s [%s]", message, summary)
}

func compactInteractionPreview(preview string) string {
	preview = strings.TrimSpace(preview)
	const maxPreviewLength = 160
	previewRunes := []rune(preview)
	if len(previewRunes) <= maxPreviewLength {
		return preview
	}
	return string(previewRunes[:maxPreviewLength-3]) + "..."
}

func shopCatalogSummary(definition interactionstore.Definition, itemTemplatesByVnum map[uint32]itemcatalog.Template) ShopCatalogSummary {
	definition = interactionstore.NormalizeDefinition(definition)
	summary := ShopCatalogSummary{
		Kind:       definition.Kind,
		Ref:        definition.Ref,
		Title:      definition.Title,
		EntryCount: len(definition.Catalog),
	}
	if len(definition.Catalog) == 0 {
		return summary
	}
	summary.Entries = make([]ShopCatalogEntrySummary, 0, len(definition.Catalog))
	for _, entry := range definition.Catalog {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[entry.ItemVnum])
		summary.Entries = append(summary.Entries, ShopCatalogEntrySummary{
			Slot:                 entry.Slot,
			ItemVnum:             entry.ItemVnum,
			ItemName:             template.Name,
			Count:                entry.Count,
			Price:                entry.Price,
			Stackable:            template.Stackable,
			MaxCount:             template.MaxCount,
			ShopBuyPrice:         template.ShopBuyPrice,
			ShopSellPrice:        template.ShopSellPrice,
			Refineable:           template.Refineable,
			RefineRejectMessage:  template.RefineRejectText,
			ConfirmWhenUse:       template.ConfirmWhenUse,
			QuestUse:             template.QuestUse,
			QuestUseMultiple:     template.QuestUseMultiple,
			Applicable:           template.Applicable,
			EquipSlot:            template.EquipSlot,
			AppearanceVnum:       template.AppearanceVnum,
			Irremovable:          template.Irremovable,
			UseEffect:            cloneItemTemplateUseEffect(template.UseEffect),
			EquipEffect:          cloneItemTemplatePointEffect(template.EquipEffect),
			AntiGet:              template.AntiGet,
			AntiDrop:             template.AntiDrop,
			AntiGive:             template.AntiGive,
			AntiSell:             template.AntiSell,
			AntiStack:            template.AntiStack,
			AntiSafebox:          template.AntiSafebox,
			AntiMale:             template.AntiMale,
			AntiFemale:           template.AntiFemale,
			AntiWarrior:          template.AntiWarrior,
			AntiAssassin:         template.AntiAssassin,
			AntiSura:             template.AntiSura,
			AntiShaman:           template.AntiShaman,
			AntiEmpireA:          template.AntiEmpireA,
			AntiEmpireB:          template.AntiEmpireB,
			AntiEmpireC:          template.AntiEmpireC,
			MinLevel:             template.MinLevel,
			UseRejectMessage:     template.UseRejectText,
			BuyRejectMessage:     template.BuyRejectText,
			DropRejectMessage:    template.DropRejectText,
			GiveRejectMessage:    template.GiveRejectText,
			PickupRejectMessage:  template.PickupRejectText,
			SellRejectMessage:    template.SellRejectText,
			EquipRejectMessage:   template.EquipRejectText,
			UnequipRejectMessage: template.UnequipRejectText,
			SafeboxRejectMessage: template.SafeboxRejectText,
			PickupRange:          template.PickupRange,
		})
	}
	return summary
}

func warpDestinationSummary(definition interactionstore.Definition) WarpDestinationSummary {
	definition = interactionstore.NormalizeDefinition(definition)
	return WarpDestinationSummary{
		Kind:     definition.Kind,
		Ref:      definition.Ref,
		Text:     definition.Text,
		MapIndex: definition.MapIndex,
		X:        definition.X,
		Y:        definition.Y,
	}
}

func shopRouteSummary(actor StaticActor, definition interactionstore.Definition) ShopRouteSummary {
	actor = normalizeStaticActors([]StaticActor{actor})[0]
	definition = interactionstore.NormalizeDefinition(definition)
	return ShopRouteSummary{
		ActorName:      actor.Name,
		SourceMapIndex: actor.MapIndex,
		SourceX:        actor.X,
		SourceY:        actor.Y,
		Ref:            definition.Ref,
		Title:          definition.Title,
		EntryCount:     len(definition.Catalog),
	}
}

func warpRouteSummary(actor StaticActor, definition interactionstore.Definition) WarpRouteSummary {
	actor = normalizeStaticActors([]StaticActor{actor})[0]
	definition = interactionstore.NormalizeDefinition(definition)
	return WarpRouteSummary{
		ActorName:      actor.Name,
		SourceMapIndex: actor.MapIndex,
		SourceX:        actor.X,
		SourceY:        actor.Y,
		Ref:            definition.Ref,
		Text:           definition.Text,
		TargetMapIndex: definition.MapIndex,
		TargetX:        definition.X,
		TargetY:        definition.Y,
	}
}

func itemTemplateReferenceSummaries(templates []itemcatalog.Template) []ItemTemplateReferenceSummary {
	if len(templates) == 0 {
		return nil
	}
	summaries := make([]ItemTemplateReferenceSummary, 0, len(templates))
	for _, template := range templates {
		template = itemcatalog.NormalizeTemplate(template)
		summaries = append(summaries, ItemTemplateReferenceSummary{
			Vnum:                 template.Vnum,
			Name:                 template.Name,
			Stackable:            template.Stackable,
			MaxCount:             template.MaxCount,
			ShopBuyPrice:         template.ShopBuyPrice,
			ShopSellPrice:        template.ShopSellPrice,
			Refineable:           template.Refineable,
			RefineRejectMessage:  template.RefineRejectText,
			ConfirmWhenUse:       template.ConfirmWhenUse,
			QuestUse:             template.QuestUse,
			QuestUseMultiple:     template.QuestUseMultiple,
			Applicable:           template.Applicable,
			EquipSlot:            template.EquipSlot,
			AppearanceVnum:       template.AppearanceVnum,
			Irremovable:          template.Irremovable,
			UseEffect:            cloneItemTemplateUseEffect(template.UseEffect),
			EquipEffect:          cloneItemTemplatePointEffect(template.EquipEffect),
			AntiGet:              template.AntiGet,
			AntiDrop:             template.AntiDrop,
			AntiGive:             template.AntiGive,
			AntiSell:             template.AntiSell,
			AntiStack:            template.AntiStack,
			AntiSafebox:          template.AntiSafebox,
			AntiMale:             template.AntiMale,
			AntiFemale:           template.AntiFemale,
			AntiWarrior:          template.AntiWarrior,
			AntiAssassin:         template.AntiAssassin,
			AntiSura:             template.AntiSura,
			AntiShaman:           template.AntiShaman,
			AntiEmpireA:          template.AntiEmpireA,
			AntiEmpireB:          template.AntiEmpireB,
			AntiEmpireC:          template.AntiEmpireC,
			MinLevel:             template.MinLevel,
			UseRejectMessage:     template.UseRejectText,
			BuyRejectMessage:     template.BuyRejectText,
			DropRejectMessage:    template.DropRejectText,
			GiveRejectMessage:    template.GiveRejectText,
			PickupRejectMessage:  template.PickupRejectText,
			SellRejectMessage:    template.SellRejectText,
			EquipRejectMessage:   template.EquipRejectText,
			UnequipRejectMessage: template.UnequipRejectText,
			SafeboxRejectMessage: template.SafeboxRejectText,
			PickupRange:          template.PickupRange,
		})
	}
	sort.Slice(summaries, func(i int, j int) bool {
		return summaries[i].Vnum < summaries[j].Vnum
	})
	return summaries
}

func rewardDropItemSummaries(dropVnums []uint32, itemTemplatesByVnum map[uint32]itemcatalog.Template) []RewardDropItemSummary {
	if len(dropVnums) == 0 {
		return nil
	}
	sortedVnums := cloneUint32s(dropVnums)
	summaries := make([]RewardDropItemSummary, 0, len(sortedVnums))
	for _, vnum := range sortedVnums {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[vnum])
		if template.Vnum == 0 || template.Name == "" {
			continue
		}
		summaries = append(summaries, RewardDropItemSummary{
			ItemVnum:             template.Vnum,
			ItemName:             template.Name,
			Stackable:            template.Stackable,
			MaxCount:             template.MaxCount,
			ShopBuyPrice:         template.ShopBuyPrice,
			ShopSellPrice:        template.ShopSellPrice,
			Refineable:           template.Refineable,
			RefineRejectMessage:  template.RefineRejectText,
			ConfirmWhenUse:       template.ConfirmWhenUse,
			QuestUse:             template.QuestUse,
			QuestUseMultiple:     template.QuestUseMultiple,
			Applicable:           template.Applicable,
			EquipSlot:            template.EquipSlot,
			AppearanceVnum:       template.AppearanceVnum,
			Irremovable:          template.Irremovable,
			UseEffect:            cloneItemTemplateUseEffect(template.UseEffect),
			EquipEffect:          cloneItemTemplatePointEffect(template.EquipEffect),
			AntiGet:              template.AntiGet,
			AntiDrop:             template.AntiDrop,
			AntiGive:             template.AntiGive,
			AntiSell:             template.AntiSell,
			AntiStack:            template.AntiStack,
			AntiSafebox:          template.AntiSafebox,
			AntiMale:             template.AntiMale,
			AntiFemale:           template.AntiFemale,
			AntiWarrior:          template.AntiWarrior,
			AntiAssassin:         template.AntiAssassin,
			AntiSura:             template.AntiSura,
			AntiShaman:           template.AntiShaman,
			AntiEmpireA:          template.AntiEmpireA,
			AntiEmpireB:          template.AntiEmpireB,
			AntiEmpireC:          template.AntiEmpireC,
			MinLevel:             template.MinLevel,
			UseRejectMessage:     template.UseRejectText,
			BuyRejectMessage:     template.BuyRejectText,
			DropRejectMessage:    template.DropRejectText,
			GiveRejectMessage:    template.GiveRejectText,
			PickupRejectMessage:  template.PickupRejectText,
			SellRejectMessage:    template.SellRejectText,
			EquipRejectMessage:   template.EquipRejectText,
			UnequipRejectMessage: template.UnequipRejectText,
			SafeboxRejectMessage: template.SafeboxRejectText,
			PickupRange:          template.PickupRange,
		})
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

func RewardDropAggregatesForMap(summary Summary, mapIndex uint32) []RewardDropAggregateSummary {
	if mapIndex == 0 || len(summary.SpawnGroups) == 0 || len(summary.RewardDrops) == 0 {
		return nil
	}
	totalByVnum := make(map[uint32]RewardDropAggregateSummary, len(summary.RewardDrops))
	for _, drop := range summary.RewardDrops {
		drop = normalizeRewardDropAggregateSummary(drop)
		if drop.ItemVnum == 0 || drop.SourceCount == 0 {
			continue
		}
		totalByVnum[drop.ItemVnum] = drop
	}
	if len(totalByVnum) == 0 {
		return nil
	}
	countsByVnum := make(map[uint32]int)
	for _, spawnGroup := range summary.SpawnGroups {
		if spawnGroup.MapIndex != mapIndex {
			continue
		}
		for _, vnum := range spawnGroup.RewardDropVnums {
			if _, ok := totalByVnum[vnum]; ok {
				countsByVnum[vnum]++
			}
		}
	}
	if len(countsByVnum) == 0 {
		return nil
	}
	vnums := make([]uint32, 0, len(countsByVnum))
	for vnum := range countsByVnum {
		vnums = append(vnums, vnum)
	}
	sort.Slice(vnums, func(i int, j int) bool { return vnums[i] < vnums[j] })
	aggregates := make([]RewardDropAggregateSummary, 0, len(vnums))
	for _, vnum := range vnums {
		aggregate := totalByVnum[vnum]
		aggregate.SourceCount = countsByVnum[vnum]
		aggregates = append(aggregates, aggregate)
	}
	return aggregates
}

func normalizeRewardDropAggregateSummary(drop RewardDropAggregateSummary) RewardDropAggregateSummary {
	drop.ItemName = strings.TrimSpace(drop.ItemName)
	drop.RefineRejectMessage = strings.TrimSpace(drop.RefineRejectMessage)
	drop.UseEffect = cloneItemTemplateUseEffect(drop.UseEffect)
	drop.EquipEffect = cloneItemTemplatePointEffect(drop.EquipEffect)
	drop.UseRejectMessage = strings.TrimSpace(drop.UseRejectMessage)
	drop.BuyRejectMessage = strings.TrimSpace(drop.BuyRejectMessage)
	drop.DropRejectMessage = strings.TrimSpace(drop.DropRejectMessage)
	drop.GiveRejectMessage = strings.TrimSpace(drop.GiveRejectMessage)
	drop.PickupRejectMessage = strings.TrimSpace(drop.PickupRejectMessage)
	drop.SellRejectMessage = strings.TrimSpace(drop.SellRejectMessage)
	drop.EquipRejectMessage = strings.TrimSpace(drop.EquipRejectMessage)
	drop.UnequipRejectMessage = strings.TrimSpace(drop.UnequipRejectMessage)
	drop.SafeboxRejectMessage = strings.TrimSpace(drop.SafeboxRejectMessage)
	return drop
}

func cloneRewardDropItemSummaries(items []RewardDropItemSummary) []RewardDropItemSummary {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]RewardDropItemSummary, len(items))
	copy(cloned, items)
	for i := range cloned {
		cloned[i].UseEffect = cloneItemTemplateUseEffect(cloned[i].UseEffect)
		cloned[i].EquipEffect = cloneItemTemplatePointEffect(cloned[i].EquipEffect)
	}
	sort.Slice(cloned, func(i int, j int) bool {
		if cloned[i].ItemVnum == cloned[j].ItemVnum {
			return cloned[i].ItemName < cloned[j].ItemName
		}
		return cloned[i].ItemVnum < cloned[j].ItemVnum
	})
	return cloned
}

func cloneItemTemplateUseEffect(effect *itemcatalog.UseEffect) *itemcatalog.UseEffect {
	if effect == nil {
		return nil
	}
	cloned := *effect
	return &cloned
}

func cloneItemTemplatePointEffect(effect *itemcatalog.PointEffect) *itemcatalog.PointEffect {
	if effect == nil {
		return nil
	}
	cloned := *effect
	return &cloned
}

func rewardDropAggregateSummaries(countsByVnum map[uint32]int, itemTemplatesByVnum map[uint32]itemcatalog.Template) []RewardDropAggregateSummary {
	if len(countsByVnum) == 0 {
		return nil
	}
	vnums := make([]uint32, 0, len(countsByVnum))
	for vnum := range countsByVnum {
		vnums = append(vnums, vnum)
	}
	sort.Slice(vnums, func(i int, j int) bool { return vnums[i] < vnums[j] })
	summaries := make([]RewardDropAggregateSummary, 0, len(vnums))
	for _, vnum := range vnums {
		template := itemcatalog.NormalizeTemplate(itemTemplatesByVnum[vnum])
		if template.Vnum == 0 || template.Name == "" {
			continue
		}
		summaries = append(summaries, RewardDropAggregateSummary{
			ItemVnum:             template.Vnum,
			ItemName:             template.Name,
			SourceCount:          countsByVnum[vnum],
			Stackable:            template.Stackable,
			MaxCount:             template.MaxCount,
			ShopBuyPrice:         template.ShopBuyPrice,
			ShopSellPrice:        template.ShopSellPrice,
			Refineable:           template.Refineable,
			RefineRejectMessage:  template.RefineRejectText,
			ConfirmWhenUse:       template.ConfirmWhenUse,
			QuestUse:             template.QuestUse,
			QuestUseMultiple:     template.QuestUseMultiple,
			Applicable:           template.Applicable,
			EquipSlot:            template.EquipSlot,
			AppearanceVnum:       template.AppearanceVnum,
			Irremovable:          template.Irremovable,
			UseEffect:            cloneItemTemplateUseEffect(template.UseEffect),
			EquipEffect:          cloneItemTemplatePointEffect(template.EquipEffect),
			AntiGet:              template.AntiGet,
			AntiDrop:             template.AntiDrop,
			AntiGive:             template.AntiGive,
			AntiSell:             template.AntiSell,
			AntiStack:            template.AntiStack,
			AntiSafebox:          template.AntiSafebox,
			AntiMale:             template.AntiMale,
			AntiFemale:           template.AntiFemale,
			AntiWarrior:          template.AntiWarrior,
			AntiAssassin:         template.AntiAssassin,
			AntiSura:             template.AntiSura,
			AntiShaman:           template.AntiShaman,
			AntiEmpireA:          template.AntiEmpireA,
			AntiEmpireB:          template.AntiEmpireB,
			AntiEmpireC:          template.AntiEmpireC,
			MinLevel:             template.MinLevel,
			UseRejectMessage:     template.UseRejectText,
			BuyRejectMessage:     template.BuyRejectText,
			DropRejectMessage:    template.DropRejectText,
			GiveRejectMessage:    template.GiveRejectText,
			PickupRejectMessage:  template.PickupRejectText,
			SellRejectMessage:    template.SellRejectText,
			EquipRejectMessage:   template.EquipRejectText,
			UnequipRejectMessage: template.UnequipRejectText,
			SafeboxRejectMessage: template.SafeboxRejectText,
			PickupRange:          template.PickupRange,
		})
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

func mapContentSummaryForIndex(counts map[uint32]*MapContentSummary, mapIndex uint32) *MapContentSummary {
	entry, ok := counts[mapIndex]
	if ok {
		return entry
	}
	entry = &MapContentSummary{MapIndex: mapIndex}
	counts[mapIndex] = entry
	return entry
}

func addMapServiceInteractionSummary(entry *MapContentSummary, definition interactionstore.Definition) {
	definition = interactionstore.NormalizeDefinition(definition)
	switch definition.Kind {
	case interactionstore.KindInfo:
		entry.InfoActorCount++
	case interactionstore.KindTalk:
		entry.TalkActorCount++
	case interactionstore.KindShopPreview:
		entry.ShopPreviewActorCount++
		entry.ShopCatalogEntryCount += len(definition.Catalog)
	case interactionstore.KindWarp:
		entry.WarpActorCount++
	}
}

func validateBundle(bundle Bundle) error {
	if !queststate.ValidSnapshot(queststate.Snapshot{Flags: bundle.QuestState}) {
		return ErrInvalidBundle
	}
	itemTemplatesByVnum := make(map[uint32]itemcatalog.Template, len(bundle.ItemTemplates))
	for _, template := range bundle.ItemTemplates {
		normalizedTemplate := itemcatalog.NormalizeTemplate(template)
		if !validItemTemplateStrings(normalizedTemplate) {
			return ErrInvalidBundle
		}
		if !itemcatalog.ValidTemplate(normalizedTemplate) {
			return ErrInvalidBundle
		}
		if _, ok := itemTemplatesByVnum[normalizedTemplate.Vnum]; ok {
			return ErrInvalidBundle
		}
		itemTemplatesByVnum[normalizedTemplate.Vnum] = normalizedTemplate
	}
	profileSnapshots := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(bundle.CombatProfiles))
	referencedProfiles := referencedCombatProfileNames(bundle.StaticActors, bundle.SpawnGroups)
	for _, profile := range bundle.CombatProfiles {
		if !validCombatProfileSnapshot(profile) {
			return ErrInvalidBundle
		}
		if !validRewardDropRefs(profile.DeathReward.DropVnums, itemTemplatesByVnum) {
			return ErrInvalidBundle
		}
		name := strings.TrimSpace(profile.Profile)
		if existing, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(name); ok && !combatProfileSnapshotMatchesDefaults(profile, existing) {
			return ErrInvalidBundle
		}
		if _, referenced := referencedProfiles[name]; !referenced {
			return ErrInvalidBundle
		}
		if _, ok := profileSnapshots[name]; ok {
			return ErrInvalidBundle
		}
		profileSnapshots[name] = profile
	}
	definitionsByKey := make(map[string]struct{}, len(bundle.InteractionDefinitions))
	for _, definition := range bundle.InteractionDefinitions {
		definition = interactionstore.NormalizeDefinition(definition)
		if !validInteractionDefinitionStrings(definition) {
			return ErrInvalidBundle
		}
		if !validDefinition(definition) {
			return ErrInvalidBundle
		}
		if definition.Kind == interactionstore.KindShopPreview {
			if len(itemTemplatesByVnum) == 0 {
				return ErrInvalidBundle
			}
			for _, entry := range definition.Catalog {
				template, ok := itemTemplatesByVnum[entry.ItemVnum]
				if !ok {
					return ErrInvalidBundle
				}
				if !validMerchantCatalogCountForTemplate(entry, template) {
					return ErrInvalidBundle
				}
			}
		}
		key := interactionDefinitionKey(definition.Kind, definition.Ref)
		if _, ok := definitionsByKey[key]; ok {
			return ErrInvalidBundle
		}
		definitionsByKey[key] = struct{}{}
	}
	referencedItemVnums := referencedItemTemplateVnums(bundle.InteractionDefinitions, bundle.SpawnGroups, bundle.CombatProfiles)
	for vnum := range itemTemplatesByVnum {
		if _, referenced := referencedItemVnums[vnum]; !referenced {
			return ErrInvalidBundle
		}
	}
	spawnGroupsByRef := make(map[string]struct{}, len(bundle.SpawnGroups))
	for _, spawnGroup := range bundle.SpawnGroups {
		if !validSpawnGroup(spawnGroup, profileSnapshots) {
			return ErrInvalidBundle
		}
		if !validRewardDropRefs(spawnGroup.RewardDropVnums, itemTemplatesByVnum) {
			return ErrInvalidBundle
		}
		if _, ok := spawnGroupsByRef[spawnGroup.Ref]; ok {
			return ErrInvalidBundle
		}
		spawnGroupsByRef[spawnGroup.Ref] = struct{}{}
	}
	staticActorsByKey := make(map[string]struct{}, len(bundle.StaticActors))
	for _, actor := range bundle.StaticActors {
		if !validAuthoredContentName(actor.Name) || actor.MapIndex == 0 || !validBootstrapRaceNum(actor.RaceNum) {
			return ErrInvalidBundle
		}
		if !validAuthoredCombatProfile(actor.CombatProfile, profileSnapshots) {
			return ErrInvalidBundle
		}
		if !validInteractionMetadata(actor.InteractionKind, actor.InteractionRef) {
			return ErrInvalidBundle
		}
		key := staticActorAuthoringKey(actor)
		if _, ok := staticActorsByKey[key]; ok {
			return ErrInvalidBundle
		}
		staticActorsByKey[key] = struct{}{}
		if actor.InteractionKind == "" && actor.InteractionRef == "" {
			continue
		}
		if _, ok := definitionsByKey[interactionDefinitionKey(actor.InteractionKind, actor.InteractionRef)]; !ok {
			return ErrInvalidBundle
		}
	}
	return nil
}

func validInteractionDefinitionStrings(definition interactionstore.Definition) bool {
	if !validAuthoredContentString(definition.Text) || !validAuthoredContentString(definition.Title) {
		return false
	}
	return true
}

func validItemTemplateStrings(template itemcatalog.Template) bool {
	if !validAuthoredContentString(template.Name) ||
		!validAuthoredContentString(template.RefineRejectText) ||
		!validAuthoredContentString(template.UseRejectText) ||
		!validAuthoredContentString(template.BuyRejectText) ||
		!validAuthoredContentString(template.DropRejectText) ||
		!validAuthoredContentString(template.PickupRejectText) ||
		!validAuthoredContentString(template.SellRejectText) ||
		!validAuthoredContentString(template.EquipRejectText) ||
		!validAuthoredContentString(template.UnequipRejectText) ||
		!validAuthoredContentString(template.SafeboxRejectText) {
		return false
	}
	if template.UseEffect != nil && (!validAuthoredContentString(template.UseEffect.Message) || !validAuthoredContentString(template.UseEffect.InfoMessage)) {
		return false
	}
	return true
}

func validDefinition(definition interactionstore.Definition) bool {
	return interactionstore.ValidDefinition(definition)
}

func validInteractionMetadata(kind string, ref string) bool {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != "" && interactionstore.ValidKind(kind) && interactionstore.ValidRef(ref)
}

func validBootstrapRaceNum(raceNum uint32) bool {
	return worldruntime.ValidStaticActorVisibilityRaceNum(raceNum)
}

func validSpawnGroup(spawnGroup SpawnGroup, profileSnapshots map[string]worldruntime.StaticActorCombatProfileSnapshot) bool {
	if !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroup.Ref) || strings.TrimSpace(spawnGroup.Ref) == "" || !validAuthoredContentName(spawnGroup.Name) || spawnGroup.MapIndex == 0 || !validBootstrapRaceNum(spawnGroup.RaceNum) {
		return false
	}
	if strings.TrimSpace(spawnGroup.CombatProfile) == "" || !validAuthoredCombatProfile(spawnGroup.CombatProfile, profileSnapshots) {
		return false
	}
	return validRewardDescriptor(spawnGroup)
}

func validAuthoredCombatProfile(profile string, profileSnapshots map[string]worldruntime.StaticActorCombatProfileSnapshot) bool {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return true
	}
	if worldruntime.ValidStaticActorCombatProfile(profile) {
		return true
	}
	_, ok := profileSnapshots[profile]
	return ok
}

func validAuthoredContentName(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && validAuthoredContentString(name)
}

func validAuthoredContentString(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

func validCombatProfileSnapshot(profile worldruntime.StaticActorCombatProfileSnapshot) bool {
	name := strings.TrimSpace(profile.Profile)
	if name == "" || name == worldruntime.StaticActorCombatProfilePracticeMob || name == worldruntime.StaticActorCombatProfileTrainingDummy {
		return false
	}
	if profile.RetaliationPointDelta > 0 {
		return false
	}
	if profile.MaxHP == 0 || profile.AttackValue == 0 || !worldruntime.ValidStaticActorCombatProfileRespawnDelayMs(profile.RespawnDelayMs) {
		return false
	}
	if profile.AttackValue > profile.DefenseValue && profile.AttackValue-profile.DefenseValue > uint16(profile.MaxHP) {
		return false
	}
	if profile.DamagePerNormalAttack != 0 {
		expectedDamage := combatProfileSnapshotFormulaDamage(profile)
		if profile.DamagePerNormalAttack != expectedDamage || profile.DamagePerNormalAttack > profile.MaxHP {
			return false
		}
	}
	return worldruntime.ValidStaticActorDeathReward(profile.DeathReward)
}

func combatProfileSnapshotMatchesDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot, defaults worldruntime.StaticActorCombatProfileDefaults) bool {
	candidateDefaults, ok := combatProfileSnapshotDefaults(snapshot)
	return ok &&
		candidateDefaults.MaxHP == defaults.MaxHP &&
		candidateDefaults.DamagePerNormalAttack == defaults.DamagePerNormalAttack &&
		candidateDefaults.AttackValue == defaults.AttackValue &&
		candidateDefaults.DefenseValue == defaults.DefenseValue &&
		candidateDefaults.Level == defaults.Level &&
		candidateDefaults.Rank == defaults.Rank &&
		candidateDefaults.RespawnDelay == defaults.RespawnDelay &&
		candidateDefaults.RetaliationPointDelta == defaults.RetaliationPointDelta &&
		reflect.DeepEqual(candidateDefaults.DeathReward.Clone(), defaults.DeathReward.Clone())
}

func combatProfileSnapshotDefaults(snapshot worldruntime.StaticActorCombatProfileSnapshot) (worldruntime.StaticActorCombatProfileDefaults, bool) {
	respawnDelay, ok := worldruntime.StaticActorCombatProfileRespawnDelay(snapshot.RespawnDelayMs)
	if strings.TrimSpace(snapshot.Profile) == "" || snapshot.MaxHP == 0 || snapshot.AttackValue == 0 || !ok {
		return worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	defaults := worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 snapshot.MaxHP,
		DamagePerNormalAttack: snapshot.DamagePerNormalAttack,
		AttackValue:           snapshot.AttackValue,
		DefenseValue:          snapshot.DefenseValue,
		Level:                 snapshot.Level,
		Rank:                  snapshot.Rank,
		RespawnDelay:          respawnDelay,
		RetaliationPointDelta: snapshot.RetaliationPointDelta,
		DeathReward:           snapshot.DeathReward.Clone(),
	}
	if defaults.DamagePerNormalAttack == 0 {
		defaults.DamagePerNormalAttack = combatProfileSnapshotFormulaDamage(snapshot)
	}
	if defaults.Level == 0 {
		defaults.Level = worldruntime.TrainingDummyBootstrapLevel
	}
	if defaults.RetaliationPointDelta == 0 {
		defaults.RetaliationPointDelta = worldruntime.PracticeMobBootstrapRetaliationPointDelta
	}
	return defaults, true
}

func combatProfileSnapshotFormulaDamage(profile worldruntime.StaticActorCombatProfileSnapshot) uint8 {
	if profile.AttackValue <= profile.DefenseValue {
		return 1
	}
	damage := profile.AttackValue - profile.DefenseValue
	if damage == 0 {
		return 1
	}
	if damage > uint16(profile.MaxHP) {
		return profile.MaxHP
	}
	return uint8(damage)
}

func validRewardDescriptor(spawnGroup SpawnGroup) bool {
	return worldruntime.ValidStaticActorDeathReward(worldruntime.StaticActorDeathReward{Experience: spawnGroup.RewardExperience, Gold: spawnGroup.RewardGold, DropVnums: spawnGroup.RewardDropVnums})
}

func validMerchantCatalogCountForTemplate(entry interactionstore.MerchantCatalogEntry, template itemcatalog.Template) bool {
	if entry.Count == 0 || template.MaxCount == 0 {
		return false
	}
	if !template.Stackable {
		return entry.Count == 1
	}
	return entry.Count <= template.MaxCount
}

func validRewardDropRefs(dropVnums []uint32, itemTemplatesByVnum map[uint32]itemcatalog.Template) bool {
	if len(dropVnums) == 0 {
		return true
	}
	if len(itemTemplatesByVnum) == 0 {
		return false
	}
	for _, vnum := range dropVnums {
		if _, ok := itemTemplatesByVnum[vnum]; !ok {
			return false
		}
	}
	return true
}

func normalizeSpawnGroups(spawnGroups []SpawnGroup, profileSnapshots []worldruntime.StaticActorCombatProfileSnapshot) []SpawnGroup {
	if len(spawnGroups) == 0 {
		return nil
	}
	profileRewards := make(map[string]worldruntime.StaticActorDeathReward, len(profileSnapshots))
	for _, snapshot := range profileSnapshots {
		profileRewards[strings.TrimSpace(snapshot.Profile)] = snapshot.DeathReward.Clone()
	}
	normalized := make([]SpawnGroup, len(spawnGroups))
	for i, spawnGroup := range spawnGroups {
		spawnGroup.Name = strings.TrimSpace(spawnGroup.Name)
		spawnGroup.CombatProfile = strings.TrimSpace(spawnGroup.CombatProfile)
		if spawnGroup.CombatProfile == "" {
			spawnGroup.CombatProfile = worldruntime.StaticActorCombatProfilePracticeMob
		}
		if spawnGroup.RewardExperience == 0 && spawnGroup.RewardGold == 0 && len(spawnGroup.RewardDropVnums) == 0 {
			if reward, ok := worldruntime.BootstrapStaticActorDeathReward(spawnGroup.CombatProfile); ok {
				spawnGroup.RewardExperience = reward.Experience
				spawnGroup.RewardGold = reward.Gold
				spawnGroup.RewardDropVnums = reward.DropVnums
			} else if reward, ok := profileRewards[spawnGroup.CombatProfile]; ok {
				spawnGroup.RewardExperience = reward.Experience
				spawnGroup.RewardGold = reward.Gold
				spawnGroup.RewardDropVnums = reward.DropVnums
			}
		}
		spawnGroup.RewardDropVnums = cloneUint32s(spawnGroup.RewardDropVnums)
		normalized[i] = spawnGroup
	}
	return normalized
}

func combatProfilesForAuthoredActors(staticActors []StaticActor, spawnGroups []SpawnGroup, importedProfiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	seen := make(map[string]struct{})
	profiles := make([]worldruntime.StaticActorCombatProfileSnapshot, 0)
	for _, profile := range importedProfiles {
		profile.Profile = strings.TrimSpace(profile.Profile)
		profile.DeathReward = cloneDeathRewardPreservingDropMultiplicity(profile.DeathReward)
		profiles = append(profiles, profile)
		seen[profile.Profile] = struct{}{}
	}
	for _, actor := range staticActors {
		profiles = appendCombatProfileSnapshot(profiles, seen, actor.CombatProfile)
	}
	for _, spawnGroup := range spawnGroups {
		profiles = appendCombatProfileSnapshot(profiles, seen, spawnGroup.CombatProfile)
	}
	sort.Slice(profiles, func(i int, j int) bool {
		return profiles[i].Profile < profiles[j].Profile
	})
	if len(profiles) == 0 {
		return nil
	}
	return profiles
}

func normalizeCombatProfiles(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	normalized := cloneCombatProfileSnapshotsPreservingRewardDropMultiplicity(profiles)
	if len(normalized) == 0 {
		return nil
	}
	sort.Slice(normalized, func(i int, j int) bool {
		return normalized[i].Profile < normalized[j].Profile
	})
	return normalized
}

func combatProfileSnapshotIdentitiesAreCanonical(profiles []worldruntime.StaticActorCombatProfileSnapshot) bool {
	for _, profile := range profiles {
		if !worldruntime.ValidStaticActorCombatProfileName(profile.Profile) {
			return false
		}
		if !validCombatProfileSnapshot(profile) {
			return false
		}
	}
	return true
}

func cloneCombatProfileSnapshotsPreservingRewardDropMultiplicity(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make([]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	copy(cloned, profiles)
	for i := range cloned {
		cloned[i].Profile = strings.TrimSpace(cloned[i].Profile)
		if cloned[i].RetaliationPointDelta == worldruntime.PracticeMobBootstrapRetaliationPointDelta {
			cloned[i].RetaliationPointDelta = 0
		}
		cloned[i].DeathReward = cloneDeathRewardPreservingDropMultiplicity(cloned[i].DeathReward)
	}
	return cloned
}

func cloneCombatProfileSnapshots(profiles []worldruntime.StaticActorCombatProfileSnapshot) []worldruntime.StaticActorCombatProfileSnapshot {
	if len(profiles) == 0 {
		return nil
	}
	cloned := make([]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	copy(cloned, profiles)
	for i := range cloned {
		cloned[i].Profile = strings.TrimSpace(cloned[i].Profile)
		if cloned[i].RetaliationPointDelta == worldruntime.PracticeMobBootstrapRetaliationPointDelta {
			cloned[i].RetaliationPointDelta = 0
		}
		cloned[i].DeathReward = cloned[i].DeathReward.Clone()
	}
	return cloned
}

func referencedCombatProfileNames(staticActors []StaticActor, spawnGroups []SpawnGroup) map[string]struct{} {
	referenced := make(map[string]struct{}, len(staticActors)+len(spawnGroups))
	for _, actor := range staticActors {
		profile := strings.TrimSpace(actor.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	for _, spawnGroup := range spawnGroups {
		profile := strings.TrimSpace(spawnGroup.CombatProfile)
		if profile != "" {
			referenced[profile] = struct{}{}
		}
	}
	return referenced
}

func appendCombatProfileSnapshot(profiles []worldruntime.StaticActorCombatProfileSnapshot, seen map[string]struct{}, profile string) []worldruntime.StaticActorCombatProfileSnapshot {
	profile = strings.TrimSpace(profile)
	if profile == "" || profile == worldruntime.StaticActorCombatProfilePracticeMob || profile == worldruntime.StaticActorCombatProfileTrainingDummy {
		return profiles
	}
	if _, ok := seen[profile]; ok {
		return profiles
	}
	for _, snapshot := range worldruntime.StaticActorCombatProfileSnapshots() {
		if snapshot.Profile != profile {
			continue
		}
		profiles = append(profiles, snapshot)
		seen[profile] = struct{}{}
		return profiles
	}
	return profiles
}

func cloneDeathRewardPreservingDropMultiplicity(reward worldruntime.StaticActorDeathReward) worldruntime.StaticActorDeathReward {
	cloned := worldruntime.StaticActorDeathReward{Experience: reward.Experience, Gold: reward.Gold}
	if len(reward.DropVnums) > 0 {
		cloned.DropVnums = append([]uint32(nil), reward.DropVnums...)
		sort.Slice(cloned.DropVnums, func(i int, j int) bool {
			return cloned.DropVnums[i] < cloned.DropVnums[j]
		})
	}
	return cloned
}

func cloneUint32s(values []uint32) []uint32 {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]uint32, len(values))
	copy(cloned, values)
	sort.Slice(cloned, func(i int, j int) bool {
		return cloned[i] < cloned[j]
	})
	return cloned
}

func compareUint32s(left []uint32, right []uint32) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

func interactionDefinitionKey(kind string, ref string) string {
	return strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(ref)
}

func staticActorAuthoringKey(actor StaticActor) string {
	return strings.Join([]string{
		strings.TrimSpace(actor.Name),
		fmt.Sprintf("%d", actor.MapIndex),
		fmt.Sprintf("%d", actor.X),
		fmt.Sprintf("%d", actor.Y),
		fmt.Sprintf("%d", actor.RaceNum),
		strings.TrimSpace(actor.CombatProfile),
		strings.TrimSpace(actor.InteractionKind),
		strings.TrimSpace(actor.InteractionRef),
	}, "\x00")
}

func normalizeStaticActors(actors []StaticActor) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	normalized := make([]StaticActor, len(actors))
	copy(normalized, actors)
	for i := range normalized {
		normalized[i].Name = strings.TrimSpace(normalized[i].Name)
		normalized[i].CombatProfile = strings.TrimSpace(normalized[i].CombatProfile)
		normalized[i].InteractionKind = strings.TrimSpace(normalized[i].InteractionKind)
		normalized[i].InteractionRef = strings.TrimSpace(normalized[i].InteractionRef)
	}
	return normalized
}

func cloneStaticActors(actors []StaticActor) []StaticActor {
	if len(actors) == 0 {
		return nil
	}
	cloned := make([]StaticActor, len(actors))
	copy(cloned, actors)
	return cloned
}

func cloneDefinitions(definitions []interactionstore.Definition) []interactionstore.Definition {
	if len(definitions) == 0 {
		return nil
	}
	cloned := make([]interactionstore.Definition, len(definitions))
	for i, definition := range definitions {
		cloned[i] = interactionstore.NormalizeDefinition(definition)
	}
	return cloned
}

func normalizeItemTemplates(templates []itemcatalog.Template) []itemcatalog.Template {
	if len(templates) == 0 {
		return nil
	}
	normalized := make([]itemcatalog.Template, len(templates))
	for i, template := range templates {
		normalized[i] = itemcatalog.NormalizeTemplate(template)
	}
	return normalized
}

func filterReferencedItemTemplates(templates []itemcatalog.Template, referenced map[uint32]struct{}) []itemcatalog.Template {
	if len(templates) == 0 || len(referenced) == 0 {
		return nil
	}
	filtered := make([]itemcatalog.Template, 0, len(templates))
	for _, template := range normalizeItemTemplates(templates) {
		if _, ok := referenced[template.Vnum]; ok {
			filtered = append(filtered, template)
		}
	}
	return filtered
}

func normalizeQuestStateFlags(flags []queststate.Flag) []queststate.Flag {
	if len(flags) == 0 {
		return nil
	}
	return queststate.NormalizeSnapshot(queststate.Snapshot{Flags: flags}).Flags
}

func questStateFlagKey(flag queststate.Flag) string {
	return strings.TrimSpace(flag.Character) + "\x00" + strings.TrimSpace(flag.QuestRef) + "\x00" + strings.TrimSpace(flag.Name)
}

func referencedItemTemplateVnums(definitions []interactionstore.Definition, spawnGroups []SpawnGroup, combatProfiles []worldruntime.StaticActorCombatProfileSnapshot) map[uint32]struct{} {
	referenced := make(map[uint32]struct{})
	for _, definition := range definitions {
		definition = interactionstore.NormalizeDefinition(definition)
		if definition.Kind != interactionstore.KindShopPreview {
			continue
		}
		for _, entry := range definition.Catalog {
			if entry.ItemVnum != 0 {
				referenced[entry.ItemVnum] = struct{}{}
			}
		}
	}
	for _, spawnGroup := range spawnGroups {
		for _, vnum := range spawnGroup.RewardDropVnums {
			if vnum != 0 {
				referenced[vnum] = struct{}{}
			}
		}
	}
	for _, profile := range combatProfiles {
		for _, vnum := range profile.DeathReward.DropVnums {
			if vnum != 0 {
				referenced[vnum] = struct{}{}
			}
		}
	}
	return referenced
}
