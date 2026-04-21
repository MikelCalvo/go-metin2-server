package worldruntime

import (
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type PlayerDirectory struct {
	mu             sync.Mutex
	byEntityID     map[uint64]PlayerEntity
	entityIDByVID  map[uint32]uint64
	entityIDByName map[string]uint64
}

func NewPlayerDirectory() *PlayerDirectory {
	return &PlayerDirectory{
		byEntityID:     make(map[uint64]PlayerEntity),
		entityIDByVID:  make(map[uint32]uint64),
		entityIDByName: make(map[string]uint64),
	}
}

func (d *PlayerDirectory) Register(player PlayerEntity) bool {
	if d == nil || !validPlayerDirectoryEntity(player) {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.byEntityID[player.Entity.ID]; ok {
		return false
	}
	if conflictingEntityID(d.entityIDByVID, player.Entity.VID, player.Entity.ID) {
		return false
	}
	if conflictingEntityID(d.entityIDByName, player.Entity.Name, player.Entity.ID) {
		return false
	}

	d.byEntityID[player.Entity.ID] = player
	d.entityIDByVID[player.Entity.VID] = player.Entity.ID
	d.entityIDByName[player.Entity.Name] = player.Entity.ID
	return true
}

func (d *PlayerDirectory) Update(player PlayerEntity) bool {
	if d == nil || !validPlayerDirectoryEntity(player) {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	previous, ok := d.byEntityID[player.Entity.ID]
	if !ok {
		return false
	}
	if conflictingEntityID(d.entityIDByVID, player.Entity.VID, player.Entity.ID) {
		return false
	}
	if conflictingEntityID(d.entityIDByName, player.Entity.Name, player.Entity.ID) {
		return false
	}

	delete(d.entityIDByVID, previous.Entity.VID)
	delete(d.entityIDByName, previous.Entity.Name)
	d.byEntityID[player.Entity.ID] = player
	d.entityIDByVID[player.Entity.VID] = player.Entity.ID
	d.entityIDByName[player.Entity.Name] = player.Entity.ID
	return true
}

func (d *PlayerDirectory) Remove(entityID uint64) (PlayerEntity, bool) {
	if d == nil || entityID == 0 {
		return PlayerEntity{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	player, ok := d.byEntityID[entityID]
	if !ok {
		return PlayerEntity{}, false
	}
	delete(d.byEntityID, entityID)
	delete(d.entityIDByVID, player.Entity.VID)
	delete(d.entityIDByName, player.Entity.Name)
	return player, true
}

func (d *PlayerDirectory) ByEntityID(entityID uint64) (PlayerEntity, bool) {
	if d == nil || entityID == 0 {
		return PlayerEntity{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	player, ok := d.byEntityID[entityID]
	return player, ok
}

func (d *PlayerDirectory) ByVID(vid uint32) (PlayerEntity, bool) {
	if d == nil || vid == 0 {
		return PlayerEntity{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	entityID, ok := d.entityIDByVID[vid]
	if !ok {
		return PlayerEntity{}, false
	}
	player, ok := d.byEntityID[entityID]
	return player, ok
}

func (d *PlayerDirectory) ByName(name string) (PlayerEntity, bool) {
	if d == nil || name == "" {
		return PlayerEntity{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	entityID, ok := d.entityIDByName[name]
	if !ok {
		return PlayerEntity{}, false
	}
	player, ok := d.byEntityID[entityID]
	return player, ok
}

func (d *PlayerDirectory) PlayerCharacters() []loginticket.Character {
	if d == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	characters := make([]loginticket.Character, 0, len(d.byEntityID))
	for _, player := range d.byEntityID {
		characters = append(characters, player.Character)
	}
	sortCharacters(characters)
	return characters
}

func validPlayerDirectoryEntity(player PlayerEntity) bool {
	return player.Entity.ID != 0 && player.Entity.Kind == EntityKindPlayer && player.Entity.VID != 0 && player.Entity.Name != ""
}

func conflictingEntityID[K comparable](index map[K]uint64, key K, entityID uint64) bool {
	indexedEntityID, ok := index[key]
	return ok && indexedEntityID != entityID
}
