package contentbundle

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
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
	InteractionDefinitions []interactionstore.Definition                   `json:"interaction_definitions"`
}

func FromSnapshots(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot) (Bundle, error) {
	return FromSnapshotsWithItems(staticActors, interactions, itemcatalog.Snapshot{})
}

func FromSnapshotsWithItems(staticActors staticstore.Snapshot, interactions interactionstore.Snapshot, items itemcatalog.Snapshot) (Bundle, error) {
	bundle := Bundle{
		ItemTemplates:          normalizeItemTemplates(items.Templates),
		InteractionDefinitions: cloneDefinitions(interactions.Definitions),
	}
	bundle.StaticActors = make([]StaticActor, 0, len(staticActors.StaticActors))
	bundle.SpawnGroups = make([]SpawnGroup, 0, len(staticActors.StaticActors))
	for _, actor := range staticActors.StaticActors {
		if actor.SpawnGroupRef != "" {
			bundle.SpawnGroups = append(bundle.SpawnGroups, SpawnGroup{
				Ref:              actor.SpawnGroupRef,
				Name:             actor.Name,
				MapIndex:         actor.MapIndex,
				X:                actor.X,
				Y:                actor.Y,
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
	return Canonicalize(bundle)
}

func Canonicalize(bundle Bundle) (Bundle, error) {
	normalizedStaticActors := normalizeStaticActors(bundle.StaticActors)
	normalizedCombatProfiles := normalizeCombatProfiles(bundle.CombatProfiles)
	normalizedSpawnGroups := normalizeSpawnGroups(bundle.SpawnGroups, normalizedCombatProfiles)
	normalized := Bundle{
		StaticActors:           normalizedStaticActors,
		SpawnGroups:            normalizedSpawnGroups,
		CombatProfiles:         combatProfilesForAuthoredActors(normalizedStaticActors, normalizedSpawnGroups, normalizedCombatProfiles),
		ItemTemplates:          normalizeItemTemplates(bundle.ItemTemplates),
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
	if err := validateBundle(normalized); err != nil {
		return Bundle{}, err
	}
	return normalized, nil
}

func validateBundle(bundle Bundle) error {
	itemTemplatesByVnum := make(map[uint32]struct{}, len(bundle.ItemTemplates))
	for _, template := range bundle.ItemTemplates {
		normalizedTemplate := itemcatalog.NormalizeTemplate(template)
		if !itemcatalog.ValidTemplate(normalizedTemplate) {
			return ErrInvalidBundle
		}
		if _, ok := itemTemplatesByVnum[normalizedTemplate.Vnum]; ok {
			return ErrInvalidBundle
		}
		itemTemplatesByVnum[normalizedTemplate.Vnum] = struct{}{}
	}
	profileSnapshots := make(map[string]worldruntime.StaticActorCombatProfileSnapshot, len(bundle.CombatProfiles))
	referencedProfiles := referencedCombatProfileNames(bundle.StaticActors, bundle.SpawnGroups)
	for _, profile := range bundle.CombatProfiles {
		if !validCombatProfileSnapshot(profile) {
			return ErrInvalidBundle
		}
		name := strings.TrimSpace(profile.Profile)
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
		if !validDefinition(definition) {
			return ErrInvalidBundle
		}
		if len(itemTemplatesByVnum) > 0 && definition.Kind == interactionstore.KindShopPreview {
			for _, entry := range definition.Catalog {
				if _, ok := itemTemplatesByVnum[entry.ItemVnum]; !ok {
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
	spawnGroupsByRef := make(map[string]struct{}, len(bundle.SpawnGroups))
	for _, spawnGroup := range bundle.SpawnGroups {
		if !validSpawnGroup(spawnGroup, profileSnapshots) {
			return ErrInvalidBundle
		}
		if _, ok := spawnGroupsByRef[spawnGroup.Ref]; ok {
			return ErrInvalidBundle
		}
		spawnGroupsByRef[spawnGroup.Ref] = struct{}{}
	}
	staticActorsByKey := make(map[string]struct{}, len(bundle.StaticActors))
	for _, actor := range bundle.StaticActors {
		if strings.TrimSpace(actor.Name) == "" || actor.MapIndex == 0 || actor.RaceNum == 0 {
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

func validDefinition(definition interactionstore.Definition) bool {
	return interactionstore.ValidDefinition(definition)
}

func validInteractionMetadata(kind string, ref string) bool {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != ""
}

func validSpawnGroup(spawnGroup SpawnGroup, profileSnapshots map[string]worldruntime.StaticActorCombatProfileSnapshot) bool {
	if !worldruntime.ValidStaticActorSpawnGroupRef(spawnGroup.Ref) || strings.TrimSpace(spawnGroup.Ref) == "" || strings.TrimSpace(spawnGroup.Name) == "" || spawnGroup.MapIndex == 0 || spawnGroup.RaceNum == 0 {
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

func validCombatProfileSnapshot(profile worldruntime.StaticActorCombatProfileSnapshot) bool {
	name := strings.TrimSpace(profile.Profile)
	if name == "" || name == worldruntime.StaticActorCombatProfilePracticeMob || name == worldruntime.StaticActorCombatProfileTrainingDummy {
		return false
	}
	if profile.MaxHP == 0 || profile.AttackValue == 0 || profile.RespawnDelayMs <= 0 {
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
		profile.DeathReward = profile.DeathReward.Clone()
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
	if len(profiles) == 0 {
		return nil
	}
	normalized := make([]worldruntime.StaticActorCombatProfileSnapshot, len(profiles))
	copy(normalized, profiles)
	for i := range normalized {
		normalized[i].Profile = strings.TrimSpace(normalized[i].Profile)
		normalized[i].DeathReward = normalized[i].DeathReward.Clone()
	}
	sort.Slice(normalized, func(i int, j int) bool {
		return normalized[i].Profile < normalized[j].Profile
	})
	return normalized
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
