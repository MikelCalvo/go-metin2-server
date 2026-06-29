package worldruntime

import (
	"sort"
	"strings"
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
	if vid, ok := StaticActorVisibilityVID(actor); ok && conflictingEntityID(d.entityIDByVID, vid, actor.Entity.ID) {
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
	return cloneStaticEntity(actor), ok
}

func (d *NonPlayerDirectory) Update(actor StaticEntity) bool {
	actor = normalizeStaticEntityCombat(actor)
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	if _, ok := d.byEntityID[actor.Entity.ID]; !ok {
		return false
	}
	if vid, ok := StaticActorVisibilityVID(actor); ok && conflictingEntityID(d.entityIDByVID, vid, actor.Entity.ID) {
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
		return StaticEntity{}, false
	}
	delete(d.byEntityID, entityID)
	if vid, ok := StaticActorVisibilityVID(actor); ok {
		delete(d.entityIDByVID, vid)
	}
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
	if actor.Entity.ID == 0 || actor.Entity.Kind != EntityKindStaticActor || !actor.Position.Valid() || !ValidStaticActorInteractionMetadata(actor.InteractionKind, actor.InteractionRef) || !ValidStaticActorCombatProfile(actor.CombatProfile) || !ValidStaticActorSpawnGroupRef(actor.SpawnGroupRef) || !ValidStaticActorDeathReward(actor.DeathReward) {
		return false
	}
	if actor.SpawnGroupRef != "" {
		return actor.CombatProfile != "" && actor.InteractionKind == "" && actor.InteractionRef == ""
	}
	return actor.DeathReward.Empty()
}

func ValidStaticActorInteractionMetadata(kind string, ref string) bool {
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != ""
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
	return ref == strings.TrimSpace(ref)
}

func sortStaticEntities(actors []StaticEntity) {
	sort.Slice(actors, func(i int, j int) bool {
		if actors[i].Entity.Name == actors[j].Entity.Name {
			return actors[i].Entity.ID < actors[j].Entity.ID
		}
		return actors[i].Entity.Name < actors[j].Entity.Name
	})
}
