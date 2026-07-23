package worldruntime

import (
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
)

type NonPlayerDirectory struct {
	byEntityID    map[uint64]StaticEntity
	entityIDByVID map[uint32]uint64
}

func NewNonPlayerDirectory() *NonPlayerDirectory {
	return &NonPlayerDirectory{byEntityID: make(map[uint64]StaticEntity), entityIDByVID: make(map[uint32]uint64)}
}

func (d *NonPlayerDirectory) Register(actor StaticEntity) bool {
	actor = normalizeStaticEntityCombat(actor)
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	if _, ok := d.byEntityID[actor.Entity.ID]; ok {
		return false
	}
	if vid, ok := StaticActorVisibilityVID(actor); ok && d.conflictingVisibilityVID(vid, actor.Entity.ID) {
		return false
	}
	d.removeVisibilityVIDsForEntityID(actor.Entity.ID)
	d.byEntityID[actor.Entity.ID] = cloneStaticEntity(actor)
	if vid, ok := StaticActorVisibilityVID(actor); ok {
		d.entityIDByVID[vid] = actor.Entity.ID
	}
	return true
}

func (d *NonPlayerDirectory) ByEntityID(entityID uint64) (StaticEntity, bool) {
	if d == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	actor, ok := d.byEntityID[entityID]
	return cloneStaticEntity(actor), ok
}

func (d *NonPlayerDirectory) ByVID(vid uint32) (StaticEntity, bool) {
	if d == nil || vid == 0 {
		return StaticEntity{}, false
	}
	entityID, ok := d.entityIDByVID[vid]
	if !ok {
		return StaticEntity{}, false
	}
	actor, ok := d.byEntityID[entityID]
	if !ok {
		delete(d.entityIDByVID, vid)
		return StaticEntity{}, false
	}
	canonicalVID, ok := StaticActorVisibilityVID(actor)
	if !ok || canonicalVID != vid {
		delete(d.entityIDByVID, vid)
		return StaticEntity{}, false
	}
	return cloneStaticEntity(actor), true
}

func (d *NonPlayerDirectory) Update(actor StaticEntity) bool {
	actor = normalizeStaticEntityCombat(actor)
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	if _, ok := d.byEntityID[actor.Entity.ID]; !ok {
		return false
	}
	if vid, ok := StaticActorVisibilityVID(actor); ok && d.conflictingVisibilityVID(vid, actor.Entity.ID) {
		return false
	}
	d.removeVisibilityVIDsForEntityID(actor.Entity.ID)
	d.byEntityID[actor.Entity.ID] = cloneStaticEntity(actor)
	if vid, ok := StaticActorVisibilityVID(actor); ok {
		d.entityIDByVID[vid] = actor.Entity.ID
	}
	return true
}

func (d *NonPlayerDirectory) Remove(entityID uint64) (StaticEntity, bool) {
	if d == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	actor, ok := d.byEntityID[entityID]
	if !ok {
		d.removeVisibilityVIDsForEntityID(entityID)
		return StaticEntity{}, false
	}
	delete(d.byEntityID, entityID)
	d.removeVisibilityVIDsForEntityID(entityID)
	return cloneStaticEntity(actor), true
}

func (d *NonPlayerDirectory) removeVisibilityVIDsForEntityID(entityID uint64) {
	if d == nil || entityID == 0 {
		return
	}
	for vid, indexedEntityID := range d.entityIDByVID {
		if indexedEntityID == entityID {
			delete(d.entityIDByVID, vid)
		}
	}
}

func (d *NonPlayerDirectory) conflictingVisibilityVID(vid uint32, entityID uint64) bool {
	indexedEntityID, ok := d.entityIDByVID[vid]
	if !ok || indexedEntityID == entityID {
		return false
	}
	indexedActor, exists := d.byEntityID[indexedEntityID]
	if !exists {
		delete(d.entityIDByVID, vid)
		return false
	}
	canonicalVID, ok := StaticActorVisibilityVID(indexedActor)
	if !ok || canonicalVID != vid {
		delete(d.entityIDByVID, vid)
		return false
	}
	return true
}

func (d *NonPlayerDirectory) StaticActors() []StaticEntity {
	if d == nil || len(d.byEntityID) == 0 {
		return nil
	}
	actors := make([]StaticEntity, 0, len(d.byEntityID))
	for _, actor := range d.byEntityID {
		actors = append(actors, cloneStaticEntity(actor))
	}
	sortStaticEntities(actors)
	return actors
}

func cloneStaticEntity(actor StaticEntity) StaticEntity {
	actor.DeathReward = actor.DeathReward.Clone()
	return actor
}

func validStaticEntity(actor StaticEntity) bool {
	actor = normalizeStaticEntityCombat(actor)
	if actor.Entity.ID == 0 || actor.Entity.Kind != EntityKindStaticActor || !actor.Position.Valid() || !ValidStaticActorVisibilityRaceNum(actor.RaceNum) || !ValidStaticActorInteractionMetadata(actor.InteractionKind, actor.InteractionRef) || !ValidStaticActorCombatProfile(actor.CombatProfile) || !ValidStaticActorSpawnGroupRef(actor.SpawnGroupRef) || !ValidStaticActorDeathReward(actor.DeathReward) {
		return false
	}
	if actor.SpawnGroupRef != "" {
		return actor.CombatProfile != "" && actor.InteractionKind == "" && actor.InteractionRef == ""
	}
	return actor.DeathReward.Empty()
}

func ValidStaticActorVisibilityRaceNum(raceNum uint32) bool {
	return raceNum != 0 && raceNum <= uint32(^uint16(0))
}

func ValidStaticActorInteractionMetadata(kind string, ref string) bool {
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != "" && interactionstore.ValidKind(kind) && interactionstore.ValidRef(ref)
}

func ValidStaticActorCombatKind(kind string) bool {
	return ValidStaticActorCombatProfile(kind)
}

func ValidStaticActorCombatProfile(profile string) bool {
	switch profile {
	case "", StaticActorCombatKindTrainingDummy, StaticActorCombatProfilePracticeMob:
		return true
	default:
		_, ok := BootstrapStaticActorCombatProfileDefaults(profile)
		return ok
	}
}

func ValidStaticActorSpawnGroupRef(ref string) bool {
	if ref == "" {
		return true
	}
	if ref != strings.TrimSpace(ref) {
		return false
	}
	segments := strings.Split(ref, ".")
	if len(segments) < 2 {
		return false
	}
	for _, segment := range segments {
		if !validSpawnGroupRefSegment(segment) {
			return false
		}
	}
	return true
}

func validSpawnGroupRefSegment(segment string) bool {
	if segment == "" {
		return false
	}
	first := segment[0]
	if first < 'a' || first > 'z' {
		return false
	}
	for i := 1; i < len(segment); i++ {
		c := segment[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func sortStaticEntities(actors []StaticEntity) {
	sort.Slice(actors, func(i int, j int) bool {
		if actors[i].Entity.Name == actors[j].Entity.Name {
			return actors[i].Entity.ID < actors[j].Entity.ID
		}
		return actors[i].Entity.Name < actors[j].Entity.Name
	})
}
