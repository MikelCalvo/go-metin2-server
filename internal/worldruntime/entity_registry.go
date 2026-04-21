package worldruntime

import (
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type EntityRegistry struct {
	mu      sync.Mutex
	nextID  uint64
	players *PlayerDirectory
}

func NewEntityRegistry() *EntityRegistry {
	return &EntityRegistry{players: NewPlayerDirectory()}
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
	return registered
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

func (r *EntityRegistry) UpdatePlayer(id uint64, character loginticket.Character) bool {
	if r == nil || id == 0 || r.players == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.Update(newPlayerEntity(id, character))
}

func (r *EntityRegistry) Remove(id uint64) (PlayerEntity, bool) {
	if r == nil || id == 0 || r.players == nil {
		return PlayerEntity{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.Remove(id)
}

func (r *EntityRegistry) PlayerCharacters() []loginticket.Character {
	if r == nil || r.players == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.players.PlayerCharacters()
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
