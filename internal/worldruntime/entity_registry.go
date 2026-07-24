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
	registered := newPlayerEntity(r.nextID+1, character)
	if r.entityIDOwnedByStaticActorLocked(registered.Entity.ID) || r.playerIdentityConflictsLocked(registered) {
		return PlayerEntity{}
	}
	if r.staticActors != nil {
		if _, exists := r.staticActors.ByVID(registered.Entity.VID); exists {
			return PlayerEntity{}
		}
	}
	if r.maps != nil {
		if actor, exists := r.maps.StaticActor(uint64(registered.Entity.VID)); exists {
			if vid, ok := StaticActorVisibilityVID(actor); ok && vid == registered.Entity.VID {
				return PlayerEntity{}
			}
		}
	}
	if !r.players.Register(registered) {
		return PlayerEntity{}
	}
	if !r.maps.Register(registered) {
		_, _ = r.players.Remove(registered.Entity.ID)
		return PlayerEntity{}
	}
	r.nextID = registered.Entity.ID
	return registered
}

func (r *EntityRegistry) RegisterStaticActor(actor StaticEntity) (StaticEntity, bool) {
	if r == nil || r.staticActors == nil || r.maps == nil {
		return StaticEntity{}, false
	}
	return r.registerStaticActor(actor)
}

func (r *EntityRegistry) RegisterStaticActorWithID(actor StaticEntity) (StaticEntity, bool) {
	if r == nil || actor.Entity.ID == 0 || r.staticActors == nil || r.maps == nil {
		return StaticEntity{}, false
	}
	return r.registerStaticActor(actor)
}

func (r *EntityRegistry) registerStaticActor(actor StaticEntity) (StaticEntity, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := actor.Entity.ID
	if id == 0 {
		id = r.nextID + 1
	}
	registered := newStaticEntity(id, actor)
	if r.entityIDOwnedByPlayerLocked(registered.Entity.ID) || r.staticActorVisibilityVIDConflictsWithPlayerLocked(registered) {
		return StaticEntity{}, false
	}
	if !r.staticActors.Register(registered) {
		return StaticEntity{}, false
	}
	if !r.maps.RegisterStatic(registered) {
		_, _ = r.staticActors.Remove(registered.Entity.ID)
		return StaticEntity{}, false
	}
	if registered.Entity.ID > r.nextID {
		r.nextID = registered.Entity.ID
	}
	return registered, true
}

func (r *EntityRegistry) RemoveStaticActor(id uint64) (StaticEntity, bool) {
	if r == nil || id == 0 || r.staticActors == nil || r.maps == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	removed, ok := r.staticActors.ByEntityID(id)
	if ok {
		_, _ = r.staticActors.Remove(id)
		_, _ = r.maps.RemoveStatic(id)
		return removed, true
	}

	removed, ok = r.maps.RemoveStatic(id)
	if !ok {
		return StaticEntity{}, false
	}
	r.staticActors.removeVisibilityVIDsForEntityID(id)
	_, _ = r.staticActors.Remove(id)
	return removed, true
}

func (r *EntityRegistry) Player(id uint64) (PlayerEntity, bool) {
	if r == nil || id == 0 || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	player, ok := r.players.ByEntityID(id)
	if ok {
		return player, true
	}
	if r.maps == nil {
		return PlayerEntity{}, false
	}
	player, ok = r.maps.Player(id)
	if !ok {
		return PlayerEntity{}, false
	}
	if !r.repairPlayerDirectoryFromMapLocked(player) {
		return PlayerEntity{}, false
	}
	return player, true
}

func (r *EntityRegistry) PlayerByVID(vid uint32) (PlayerEntity, bool) {
	if r == nil || vid == 0 || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	player, ok := r.players.ByVID(vid)
	if ok {
		return player, true
	}
	if r.maps == nil {
		return PlayerEntity{}, false
	}
	player, ok = r.maps.PlayerByVID(vid)
	if !ok {
		return PlayerEntity{}, false
	}
	if !r.repairPlayerDirectoryFromMapLocked(player) {
		return PlayerEntity{}, false
	}
	return player, true
}

func (r *EntityRegistry) PlayerByName(name string) (PlayerEntity, bool) {
	if r == nil || name == "" || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	player, ok := r.players.ByName(name)
	if ok {
		return player, true
	}
	if r.maps == nil {
		return PlayerEntity{}, false
	}
	player, ok = r.maps.PlayerByName(name)
	if !ok {
		return PlayerEntity{}, false
	}
	if !r.repairPlayerDirectoryFromMapLocked(player) {
		return PlayerEntity{}, false
	}
	return player, true
}

func (r *EntityRegistry) StaticActor(id uint64) (StaticEntity, bool) {
	if r == nil || id == 0 || r.staticActors == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.staticActors.ByEntityID(id)
	if ok {
		return actor, true
	}
	if r.maps == nil {
		return StaticEntity{}, false
	}
	actor, ok = r.maps.StaticActor(id)
	if !ok {
		return StaticEntity{}, false
	}
	if !r.repairStaticActorDirectoryFromMapLocked(actor) {
		return StaticEntity{}, false
	}
	return actor, true
}

func (r *EntityRegistry) StaticActorByVID(vid uint32) (StaticEntity, bool) {
	if r == nil || vid == 0 || r.staticActors == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	actor, ok := r.staticActors.ByVID(vid)
	if ok {
		return actor, true
	}
	if r.maps == nil {
		return StaticEntity{}, false
	}
	actor, ok = r.maps.StaticActorByVID(vid)
	if !ok {
		return StaticEntity{}, false
	}
	if !r.repairStaticActorDirectoryFromMapLocked(actor) {
		return StaticEntity{}, false
	}
	return actor, true
}

func (r *EntityRegistry) UpdateStaticActor(actor StaticEntity) (StaticEntity, bool) {
	if r == nil || actor.Entity.ID == 0 || r.staticActors == nil || r.maps == nil {
		return StaticEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, hadDirectoryEntry := r.staticActors.ByEntityID(actor.Entity.ID)
	if !hadDirectoryEntry {
		var ok bool
		previous, ok = r.maps.StaticActor(actor.Entity.ID)
		if !ok {
			return StaticEntity{}, false
		}
	}
	if actor.DeathReward.Empty() && !previous.DeathReward.Empty() {
		actor.DeathReward = previous.DeathReward.Clone()
	}
	updated := newStaticEntity(actor.Entity.ID, actor)
	if r.entityIDOwnedByPlayerLocked(updated.Entity.ID) || r.staticActorVisibilityVIDConflictsWithPlayerLocked(updated) {
		return StaticEntity{}, false
	}
	if hadDirectoryEntry {
		if !r.staticActors.Update(updated) {
			return StaticEntity{}, false
		}
	} else if !r.staticActors.Register(updated) {
		return StaticEntity{}, false
	}
	if !r.maps.UpdateStatic(updated) {
		if !r.maps.RegisterStatic(updated) {
			if hadDirectoryEntry {
				_ = r.staticActors.Update(previous)
			} else {
				_, _ = r.staticActors.Remove(updated.Entity.ID)
			}
			return StaticEntity{}, false
		}
	}
	return updated, true
}

func (r *EntityRegistry) UpdatePlayer(id uint64, character loginticket.Character) bool {
	if r == nil || id == 0 || r.players == nil || r.maps == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, hadDirectoryEntry := r.players.ByEntityID(id)
	if !hadDirectoryEntry {
		var ok bool
		previous, ok = r.maps.Player(id)
		if !ok {
			return false
		}
	}
	updated := newPlayerEntity(id, character)
	if r.entityIDOwnedByStaticActorLocked(updated.Entity.ID) || r.playerIdentityConflictsLocked(updated) {
		return false
	}
	if r.staticActors != nil {
		if actor, exists := r.staticActors.ByVID(updated.Entity.VID); exists && actor.Entity.ID != updated.Entity.ID {
			return false
		}
	}
	if r.maps != nil {
		if actor, exists := r.maps.StaticActor(uint64(updated.Entity.VID)); exists {
			if vid, ok := StaticActorVisibilityVID(actor); ok && vid == updated.Entity.VID && actor.Entity.ID != updated.Entity.ID {
				return false
			}
		}
	}
	if hadDirectoryEntry {
		if !r.players.Update(updated) {
			return false
		}
	} else if !r.players.Register(updated) {
		return false
	}
	if !r.maps.Update(updated) {
		if !r.maps.Register(updated) {
			if hadDirectoryEntry {
				_ = r.players.Update(previous)
			} else {
				_, _ = r.players.Remove(updated.Entity.ID)
			}
			return false
		}
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
	actors := r.staticActors.StaticActors()
	if r.maps == nil {
		return actors
	}
	known := make(map[uint64]struct{}, len(actors))
	for _, actor := range actors {
		known[actor.Entity.ID] = struct{}{}
	}
	for _, actor := range r.maps.AllStaticActors() {
		if _, exists := known[actor.Entity.ID]; exists {
			continue
		}
		if !r.repairStaticActorDirectoryFromMapLocked(actor) {
			continue
		}
		actors = append(actors, actor)
		known[actor.Entity.ID] = struct{}{}
	}
	sortStaticEntities(actors)
	return actors
}

func (r *EntityRegistry) StaticActors(mapIndex uint32) []StaticEntity {
	if r == nil || r.maps == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maps.StaticActors(mapIndex)
}

func (r *EntityRegistry) NextEntityID() uint64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextID + 1
}

func (r *EntityRegistry) repairPlayerDirectoryFromMapLocked(player PlayerEntity) bool {
	if r == nil || r.players == nil {
		return false
	}
	if r.entityIDOwnedByStaticActorLocked(player.Entity.ID) || r.playerVisibilityVIDConflictsWithStaticActorLocked(player) || r.playerIdentityConflictsLocked(player) {
		return false
	}
	return r.players.Register(player)
}

func (r *EntityRegistry) repairStaticActorDirectoryFromMapLocked(actor StaticEntity) bool {
	if r == nil || r.staticActors == nil {
		return false
	}
	if r.entityIDOwnedByPlayerLocked(actor.Entity.ID) || r.staticActorVisibilityVIDConflictsWithPlayerLocked(actor) {
		return false
	}
	return r.staticActors.Register(actor)
}

func (r *EntityRegistry) playerIdentityConflictsLocked(player PlayerEntity) bool {
	if r == nil || player.Entity.ID == 0 {
		return true
	}
	if r.players != nil {
		if current, exists := r.players.ByVID(player.Entity.VID); exists && current.Entity.ID != player.Entity.ID {
			return true
		}
		if current, exists := r.players.ByName(player.Entity.Name); exists && current.Entity.ID != player.Entity.ID {
			return true
		}
	}
	if r.maps != nil {
		if current, exists := r.maps.PlayerByVID(player.Entity.VID); exists && current.Entity.ID != player.Entity.ID {
			return true
		}
		if current, exists := r.maps.PlayerByName(player.Entity.Name); exists && current.Entity.ID != player.Entity.ID {
			return true
		}
	}
	return false
}

func (r *EntityRegistry) entityIDOwnedByStaticActorLocked(entityID uint64) bool {
	if r == nil || entityID == 0 {
		return true
	}
	if r.staticActors != nil {
		if _, exists := r.staticActors.ByEntityID(entityID); exists {
			return true
		}
	}
	if r.maps != nil {
		if _, exists := r.maps.StaticActor(entityID); exists {
			return true
		}
	}
	return false
}

func (r *EntityRegistry) entityIDOwnedByPlayerLocked(entityID uint64) bool {
	if r == nil || entityID == 0 {
		return true
	}
	if r.players != nil {
		if _, exists := r.players.ByEntityID(entityID); exists {
			return true
		}
	}
	if r.maps != nil {
		if _, exists := r.maps.Player(entityID); exists {
			return true
		}
	}
	return false
}

func (r *EntityRegistry) playerVisibilityVIDConflictsWithStaticActorLocked(player PlayerEntity) bool {
	if r == nil || player.Entity.VID == 0 {
		return false
	}
	if r.staticActors != nil {
		if actor, exists := r.staticActors.ByVID(player.Entity.VID); exists && actor.Entity.ID != player.Entity.ID {
			return true
		}
	}
	if r.maps != nil {
		if actor, exists := r.maps.StaticActorByVID(player.Entity.VID); exists && actor.Entity.ID != player.Entity.ID {
			return true
		}
	}
	return false
}

func (r *EntityRegistry) staticActorVisibilityVIDConflictsWithPlayerLocked(actor StaticEntity) bool {
	vid, ok := StaticActorVisibilityVID(actor)
	if !ok || r == nil {
		return false
	}
	if r.players != nil {
		if player, exists := r.players.ByVID(vid); exists && player.Entity.ID != actor.Entity.ID {
			return true
		}
	}
	if r.maps != nil {
		if player, exists := r.maps.PlayerByVID(vid); exists && player.Entity.ID != actor.Entity.ID {
			return true
		}
	}
	return false
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
	actor = normalizeStaticEntityCombat(actor)
	return StaticEntity{
		Entity: Entity{
			ID:   id,
			Kind: EntityKindStaticActor,
			Name: actor.Entity.Name,
		},
		Position:        actor.Position,
		RaceNum:         actor.RaceNum,
		CombatProfile:   actor.CombatProfile,
		InteractionKind: actor.InteractionKind,
		InteractionRef:  actor.InteractionRef,
		SpawnGroupRef:   actor.SpawnGroupRef,
		CombatKind:      actor.CombatKind,
		DeathReward:     actor.DeathReward.Clone(),
	}
}
