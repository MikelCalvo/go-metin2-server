package worldruntime

import (
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type EntityRegistry struct {
	mu           sync.Mutex
	nextID       uint64
	topology     BootstrapTopology
	players      *PlayerDirectory
	staticActors *NonPlayerDirectory
	maps         *MapIndex
}

func NewEntityRegistry() *EntityRegistry {
	return NewEntityRegistryWithTopology(NewBootstrapTopology(0))
}

func NewEntityRegistryWithTopology(topology BootstrapTopology) *EntityRegistry {
	return &EntityRegistry{topology: topology, players: NewPlayerDirectory(), staticActors: NewNonPlayerDirectory(), maps: NewMapIndex(topology)}
}

func (r *EntityRegistry) RegisterPlayer(character loginticket.Character) PlayerEntity {
	if r == nil {
		return PlayerEntity{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	registered := newPlayerEntity(r.nextID, character)
	if !r.players.Register(registered) {
		return PlayerEntity{}
	}
	if !r.maps.Register(registered) {
		_, _ = r.players.Remove(registered.Entity.ID)
		return PlayerEntity{}
	}
	return registered
}

func (r *EntityRegistry) RegisterStaticActor(actor StaticEntity) (StaticEntity, bool) {
	if r == nil || r.staticActors == nil || r.maps == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	registered := newStaticEntity(r.nextID, actor)
	if !r.staticActors.Register(registered) {
		return StaticEntity{}, false
	}
	if !r.maps.RegisterStatic(registered) {
		_, _ = r.staticActors.Remove(registered.Entity.ID)
		return StaticEntity{}, false
	}
	return registered, true
}

func (r *EntityRegistry) RemoveStaticActor(id uint64) (StaticEntity, bool) {
	if r == nil || id == 0 || r.staticActors == nil || r.maps == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	removed, ok := r.staticActors.Remove(id)
	if !ok {
		return StaticEntity{}, false
	}
	_, _ = r.maps.RemoveStatic(id)
	return removed, true
}

func (r *EntityRegistry) Player(id uint64) (PlayerEntity, bool) {
	if r == nil || id == 0 || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.ByEntityID(id)
}

func (r *EntityRegistry) PlayerByVID(vid uint32) (PlayerEntity, bool) {
	if r == nil || vid == 0 || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.ByVID(vid)
}

func (r *EntityRegistry) PlayerByName(name string) (PlayerEntity, bool) {
	if r == nil || name == "" || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.ByName(name)
}

func (r *EntityRegistry) StaticActor(id uint64) (StaticEntity, bool) {
	if r == nil || id == 0 || r.staticActors == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.staticActors.ByEntityID(id)
}

func (r *EntityRegistry) UpdatePlayer(id uint64, character loginticket.Character) bool {
	if r == nil || id == 0 || r.players == nil || r.maps == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.players.ByEntityID(id)
	if !ok {
		return false
	}
	updated := newPlayerEntity(id, character)
	if !r.players.Update(updated) {
		return false
	}
	if !r.maps.Update(updated) {
		_ = r.players.Update(previous)
		return false
	}
	return true
}

func (r *EntityRegistry) Remove(id uint64) (PlayerEntity, bool) {
	if r == nil || id == 0 || r.players == nil || r.maps == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	removed, ok := r.players.ByEntityID(id)
	if ok {
		_, _ = r.players.Remove(id)
		_, _ = r.maps.Remove(id)
		return removed, true
	}

	removed, ok = r.maps.Remove(id)
	if !ok {
		return PlayerEntity{}, false
	}
	_, _ = r.players.Remove(id)
	return removed, true
}

func (r *EntityRegistry) PlayerCharacters() []loginticket.Character {
	if r == nil || r.players == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.PlayerCharacters()
}

func (r *EntityRegistry) MapCharacters(mapIndex uint32) []loginticket.Character {
	if r == nil || r.maps == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maps.PlayerCharacters(mapIndex)
}

func (r *EntityRegistry) MapOccupancy() []MapOccupancy {
	if r == nil || r.maps == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maps.Snapshot()
}

func (r *EntityRegistry) AllStaticActors() []StaticEntity {
	if r == nil || r.staticActors == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.staticActors.StaticActors()
}

func (r *EntityRegistry) StaticActors(mapIndex uint32) []StaticEntity {
	if r == nil || r.maps == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maps.StaticActors(mapIndex)
}

func newPlayerEntity(id uint64, character loginticket.Character) PlayerEntity {
	return PlayerEntity{
		Entity: Entity{
			ID:   id,
			Kind: EntityKindPlayer,
			VID:  character.VID,
			Name: character.Name,
		},
		Character: character,
	}
}

func newStaticEntity(id uint64, actor StaticEntity) StaticEntity {
	return StaticEntity{
		Entity: Entity{
			ID:   id,
			Kind: EntityKindStaticActor,
			Name: actor.Entity.Name,
		},
		Position: actor.Position,
		RaceNum:  actor.RaceNum,
	}
}
