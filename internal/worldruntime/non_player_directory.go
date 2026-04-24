package worldruntime

import "sort"

type NonPlayerDirectory struct {
	byEntityID map[uint64]StaticEntity
}

func NewNonPlayerDirectory() *NonPlayerDirectory {
	return &NonPlayerDirectory{byEntityID: make(map[uint64]StaticEntity)}
}

func (d *NonPlayerDirectory) Register(actor StaticEntity) bool {
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	if _, ok := d.byEntityID[actor.Entity.ID]; ok {
		return false
	}
	d.byEntityID[actor.Entity.ID] = actor
	return true
}

func (d *NonPlayerDirectory) ByEntityID(entityID uint64) (StaticEntity, bool) {
	if d == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	actor, ok := d.byEntityID[entityID]
	return actor, ok
}

func (d *NonPlayerDirectory) Update(actor StaticEntity) bool {
	if d == nil || !validStaticEntity(actor) {
		return false
	}
	if _, ok := d.byEntityID[actor.Entity.ID]; !ok {
		return false
	}
	d.byEntityID[actor.Entity.ID] = actor
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
	return actor.Entity.ID != 0 && actor.Entity.Kind == EntityKindStaticActor && actor.Position.Valid()
}

func sortStaticEntities(actors []StaticEntity) {
	sort.Slice(actors, func(i int, j int) bool {
		if actors[i].Entity.Name == actors[j].Entity.Name {
			return actors[i].Entity.ID < actors[j].Entity.ID
		}
		return actors[i].Entity.Name < actors[j].Entity.Name
	})
}
