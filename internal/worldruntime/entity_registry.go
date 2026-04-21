package worldruntime

import (
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type EntityRegistry struct {
	mu      sync.Mutex
	nextID  uint64
	players map[uint64]PlayerEntity
}

func NewEntityRegistry() *EntityRegistry {
	return &EntityRegistry{players: make(map[uint64]PlayerEntity)}
}

func (r *EntityRegistry) RegisterPlayer(character loginticket.Character) PlayerEntity {
	if r == nil {
		return PlayerEntity{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	registered := newPlayerEntity(r.nextID, character)
	r.players[registered.Entity.ID] = registered
	return registered
}

func (r *EntityRegistry) Player(id uint64) (PlayerEntity, bool) {
	if r == nil || id == 0 {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	playerEntity, ok := r.players[id]
	return playerEntity, ok
}

func (r *EntityRegistry) UpdatePlayer(id uint64, character loginticket.Character) bool {
	if r == nil || id == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.players[id]; !ok {
		return false
	}
	r.players[id] = newPlayerEntity(id, character)
	return true
}

func (r *EntityRegistry) Remove(id uint64) (PlayerEntity, bool) {
	if r == nil || id == 0 {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	playerEntity, ok := r.players[id]
	if !ok {
		return PlayerEntity{}, false
	}
	delete(r.players, id)
	return playerEntity, true
}

func (r *EntityRegistry) PlayerCharacters() []loginticket.Character {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	characters := make([]loginticket.Character, 0, len(r.players))
	for _, playerEntity := range r.players {
		characters = append(characters, playerEntity.Character)
	}
	sortCharacters(characters)
	return characters
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
