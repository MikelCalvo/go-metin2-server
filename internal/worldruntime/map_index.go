package worldruntime

import (
	"sort"
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type MapOccupancy struct {
	MapIndex     uint32
	Characters   []loginticket.Character
	StaticActors []StaticEntity
}

type MapIndex struct {
	mu                     sync.Mutex
	topology               BootstrapTopology
	byEntityID             map[uint64]PlayerEntity
	effectiveMapByEntityID map[uint64]uint32
	byMapIndex             map[uint32]map[uint64]PlayerEntity
	staticByEntityID       map[uint64]StaticEntity
	staticByMapIndex       map[uint32]map[uint64]StaticEntity
}

func NewMapIndex(topology BootstrapTopology) *MapIndex {
	return &MapIndex{
		topology:               topology,
		byEntityID:             make(map[uint64]PlayerEntity),
		effectiveMapByEntityID: make(map[uint64]uint32),
		byMapIndex:             make(map[uint32]map[uint64]PlayerEntity),
		staticByEntityID:       make(map[uint64]StaticEntity),
		staticByMapIndex:       make(map[uint32]map[uint64]StaticEntity),
	}
}

func (m *MapIndex) Register(player PlayerEntity) bool {
	if m == nil || !validPlayerDirectoryEntity(player) {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byEntityID[player.Entity.ID]; ok {
		return false
	}
	if _, ok := m.staticActorMapPresenceLocked(player.Entity.ID); ok {
		return false
	}

	mapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: player.Position().MapIndex})
	m.removePlayerMapPresenceLocked(player.Entity.ID)
	m.byEntityID[player.Entity.ID] = clonePlayerEntity(player)
	m.effectiveMapByEntityID[player.Entity.ID] = mapIndex
	bucket := m.byMapIndex[mapIndex]
	if bucket == nil {
		bucket = make(map[uint64]PlayerEntity)
		m.byMapIndex[mapIndex] = bucket
	}
	bucket[player.Entity.ID] = clonePlayerEntity(player)
	return true
}

func (m *MapIndex) Update(player PlayerEntity) bool {
	if m == nil || !validPlayerDirectoryEntity(player) {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	previous, ok := m.byEntityID[player.Entity.ID]
	if !ok {
		var found bool
		previous, found = m.playerMapPresenceLocked(player.Entity.ID)
		if !found {
			return false
		}
	}

	previousMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: previous.Position().MapIndex})
	nextMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: player.Position().MapIndex})
	if bucket := m.byMapIndex[previousMapIndex]; bucket != nil {
		delete(bucket, player.Entity.ID)
		if len(bucket) == 0 {
			delete(m.byMapIndex, previousMapIndex)
		}
	}

	m.byEntityID[player.Entity.ID] = clonePlayerEntity(player)
	m.effectiveMapByEntityID[player.Entity.ID] = nextMapIndex
	bucket := m.byMapIndex[nextMapIndex]
	if bucket == nil {
		bucket = make(map[uint64]PlayerEntity)
		m.byMapIndex[nextMapIndex] = bucket
	}
	bucket[player.Entity.ID] = clonePlayerEntity(player)
	return true
}

func (m *MapIndex) playerMapPresenceLocked(entityID uint64) (PlayerEntity, bool) {
	for _, bucket := range m.byMapIndex {
		player, ok := bucket[entityID]
		if ok {
			return clonePlayerEntity(player), true
		}
	}
	return PlayerEntity{}, false
}

func (m *MapIndex) Remove(entityID uint64) (PlayerEntity, bool) {
	if m == nil || entityID == 0 {
		return PlayerEntity{}, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.byEntityID[entityID]
	if ok {
		delete(m.byEntityID, entityID)
		m.removePlayerMapPresenceLocked(entityID)
		return clonePlayerEntity(player), true
	}

	for mapIndex, bucket := range m.byMapIndex {
		player, ok := bucket[entityID]
		if !ok {
			continue
		}
		delete(bucket, entityID)
		if len(bucket) == 0 {
			delete(m.byMapIndex, mapIndex)
		}
		delete(m.byEntityID, entityID)
		delete(m.effectiveMapByEntityID, entityID)
		return clonePlayerEntity(player), true
	}
	delete(m.effectiveMapByEntityID, entityID)
	return PlayerEntity{}, false
}

func (m *MapIndex) removePlayerMapPresenceLocked(entityID uint64) {
	mapIndex, ok := m.effectiveMapByEntityID[entityID]
	if ok {
		delete(m.effectiveMapByEntityID, entityID)
		if bucket := m.byMapIndex[mapIndex]; bucket != nil {
			delete(bucket, entityID)
			if len(bucket) == 0 {
				delete(m.byMapIndex, mapIndex)
			}
		}
		return
	}
	for mapIndex, bucket := range m.byMapIndex {
		if _, ok := bucket[entityID]; !ok {
			continue
		}
		delete(bucket, entityID)
		if len(bucket) == 0 {
			delete(m.byMapIndex, mapIndex)
		}
		return
	}
}

func (m *MapIndex) PlayerByVID(vid uint32) (PlayerEntity, bool) {
	if m == nil || vid == 0 {
		return PlayerEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, player := range m.byEntityID {
		if player.Entity.VID == vid {
			return clonePlayerEntity(player), true
		}
	}
	for _, bucket := range m.byMapIndex {
		for _, player := range bucket {
			if player.Entity.VID == vid {
				return clonePlayerEntity(player), true
			}
		}
	}
	return PlayerEntity{}, false
}

func (m *MapIndex) Player(entityID uint64) (PlayerEntity, bool) {
	if m == nil || entityID == 0 {
		return PlayerEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	player, ok := m.byEntityID[entityID]
	if ok {
		return clonePlayerEntity(player), true
	}
	return m.playerMapPresenceLocked(entityID)
}

func (m *MapIndex) PlayerCharacters(mapIndex uint32) []loginticket.Character {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: mapIndex})
	bucket := m.byMapIndex[effectiveMapIndex]
	if len(bucket) == 0 {
		return nil
	}
	characters := make([]loginticket.Character, 0, len(bucket))
	for _, player := range bucket {
		characters = append(characters, cloneCharacterSnapshot(player.Character))
	}
	sortCharacters(characters)
	return characters
}

func (m *MapIndex) Snapshot() []MapOccupancy {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	mapIndices := make(map[uint32]struct{}, len(m.byMapIndex)+len(m.staticByMapIndex))
	for mapIndex := range m.byMapIndex {
		mapIndices[mapIndex] = struct{}{}
	}
	for mapIndex := range m.staticByMapIndex {
		mapIndices[mapIndex] = struct{}{}
	}
	if len(mapIndices) == 0 {
		return nil
	}

	snapshots := make([]MapOccupancy, 0, len(mapIndices))
	for mapIndex := range mapIndices {
		characters := make([]loginticket.Character, 0, len(m.byMapIndex[mapIndex]))
		for _, player := range m.byMapIndex[mapIndex] {
			characters = append(characters, cloneCharacterSnapshot(player.Character))
		}
		sortCharacters(characters)

		actors := make([]StaticEntity, 0, len(m.staticByMapIndex[mapIndex]))
		for _, actor := range m.staticByMapIndex[mapIndex] {
			actors = append(actors, cloneStaticEntity(actor))
		}
		sortStaticEntities(actors)

		snapshots = append(snapshots, MapOccupancy{MapIndex: mapIndex, Characters: characters, StaticActors: actors})
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].MapIndex < snapshots[j].MapIndex
	})
	return snapshots
}

func (m *MapIndex) RegisterStatic(actor StaticEntity) bool {
	if m == nil || !validStaticEntity(actor) {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.staticByEntityID[actor.Entity.ID]; ok {
		return false
	}
	if _, ok := m.playerMapPresenceLocked(actor.Entity.ID); ok {
		return false
	}
	actor = cloneStaticEntity(actor)
	mapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
	m.removeStaticMapPresenceLocked(actor.Entity.ID)
	m.staticByEntityID[actor.Entity.ID] = actor
	bucket := m.staticByMapIndex[mapIndex]
	if bucket == nil {
		bucket = make(map[uint64]StaticEntity)
		m.staticByMapIndex[mapIndex] = bucket
	}
	bucket[actor.Entity.ID] = actor
	return true
}

func (m *MapIndex) UpdateStatic(actor StaticEntity) bool {
	if m == nil || !validStaticEntity(actor) {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.staticByEntityID[actor.Entity.ID]; !ok {
		if _, found := m.staticActorMapPresenceLocked(actor.Entity.ID); !found {
			return false
		}
	}
	nextMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
	m.removeStaticMapPresenceLocked(actor.Entity.ID)
	actor = cloneStaticEntity(actor)
	m.staticByEntityID[actor.Entity.ID] = actor
	bucket := m.staticByMapIndex[nextMapIndex]
	if bucket == nil {
		bucket = make(map[uint64]StaticEntity)
		m.staticByMapIndex[nextMapIndex] = bucket
	}
	bucket[actor.Entity.ID] = actor
	return true
}

func (m *MapIndex) staticActorMapPresenceLocked(entityID uint64) (StaticEntity, bool) {
	for _, bucket := range m.staticByMapIndex {
		actor, ok := bucket[entityID]
		if ok {
			return actor, true
		}
	}
	return StaticEntity{}, false
}

func (m *MapIndex) StaticActor(entityID uint64) (StaticEntity, bool) {
	if m == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	actor, ok := m.staticByEntityID[entityID]
	if ok {
		m.removeStaticMapPresenceLocked(entityID)
		mapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
		bucket := m.staticByMapIndex[mapIndex]
		if bucket == nil {
			bucket = make(map[uint64]StaticEntity)
			m.staticByMapIndex[mapIndex] = bucket
		}
		bucket[entityID] = actor
		return cloneStaticEntity(actor), true
	}
	actor, ok = m.staticActorMapPresenceLocked(entityID)
	return cloneStaticEntity(actor), ok
}

func (m *MapIndex) RemoveStatic(entityID uint64) (StaticEntity, bool) {
	if m == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	actor, ok := m.staticByEntityID[entityID]
	if ok {
		delete(m.staticByEntityID, entityID)
		m.removeStaticMapPresenceLocked(entityID)
		return cloneStaticEntity(actor), true
	}
	for _, bucket := range m.staticByMapIndex {
		actor, ok := bucket[entityID]
		if !ok {
			continue
		}
		m.removeStaticMapPresenceLocked(entityID)
		delete(m.staticByEntityID, entityID)
		return cloneStaticEntity(actor), true
	}
	return StaticEntity{}, false
}

func (m *MapIndex) removeStaticMapPresenceLocked(entityID uint64) {
	for mapIndex, bucket := range m.staticByMapIndex {
		if _, ok := bucket[entityID]; !ok {
			continue
		}
		delete(bucket, entityID)
		if len(bucket) == 0 {
			delete(m.staticByMapIndex, mapIndex)
		}
	}
}

func (m *MapIndex) StaticActors(mapIndex uint32) []StaticEntity {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: mapIndex})
	bucket := m.staticByMapIndex[effectiveMapIndex]
	if len(bucket) == 0 {
		return nil
	}
	actors := make([]StaticEntity, 0, len(bucket))
	for _, actor := range bucket {
		actors = append(actors, cloneStaticEntity(actor))
	}
	sortStaticEntities(actors)
	return actors
}
