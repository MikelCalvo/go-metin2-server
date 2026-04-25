package worldruntime

import "sort"

type NonPlayerDirectory struct {
	byEntityID    map[uint64]StaticEntity
	entityIDByVID map[uint32]uint64
}

func NewNonPlayerDirectory() *NonPlayerDirectory {
	return &NonPlayerDirectory{byEntityID: make(map[uint64]StaticEntity), entityIDByVID: make(map[uint32]uint64)}
}

func (d *NonPlayerDirectory) Register(actor StaticEntity) bool {
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	if _, ok := d.byEntityID[actor.Entity.ID]; ok {
		return false
	}
	if vid, ok := StaticActorVisibilityVID(actor); ok && conflictingEntityID(d.entityIDByVID, vid, actor.Entity.ID) {
		return false
	}
	d.byEntityID[actor.Entity.ID] = actor
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
	return actor, ok
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
	return actor, ok
}

func (d *NonPlayerDirectory) Update(actor StaticEntity) bool {
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	previous, ok := d.byEntityID[actor.Entity.ID]
	if !ok {
		return false
	}
	if vid, ok := StaticActorVisibilityVID(actor); ok && conflictingEntityID(d.entityIDByVID, vid, actor.Entity.ID) {
		return false
	}
	if previousVID, ok := StaticActorVisibilityVID(previous); ok {
		delete(d.entityIDByVID, previousVID)
	}
	d.byEntityID[actor.Entity.ID] = actor
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
	return actor, true
}

func (d *NonPlayerDirectory) StaticActors() []StaticEntity {
	if d == nil || len(d.byEntityID) == 0 {
		return nil
	}
	actors := make([]StaticEntity, 0, len(d.byEntityID))
	for _, actor := range d.byEntityID {
		actors = append(actors, actor)
	}
	sortStaticEntities(actors)
	return actors
}

func validStaticEntity(actor StaticEntity) bool {
	return actor.Entity.ID != 0 && actor.Entity.Kind == EntityKindStaticActor && actor.Position.Valid() && ValidStaticActorInteractionMetadata(actor.InteractionKind, actor.InteractionRef)
}

func ValidStaticActorInteractionMetadata(kind string, ref string) bool {
	if kind == "" && ref == "" {
		return true
	}
	return kind != "" && ref != ""
}

func sortStaticEntities(actors []StaticEntity) {
	sort.Slice(actors, func(i int, j int) bool {
		if actors[i].Entity.Name == actors[j].Entity.Name {
			return actors[i].Entity.ID < actors[j].Entity.ID
		}
		return actors[i].Entity.Name < actors[j].Entity.Name
	})
}
