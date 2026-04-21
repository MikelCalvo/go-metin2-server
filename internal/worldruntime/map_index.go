package worldruntime

import (
	"sort"
	"sync"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

type MapOccupancy struct {
	MapIndex   uint32
	Characters []loginticket.Character
}

type MapIndex struct {
	mu                     sync.Mutex
	topology               BootstrapTopology
	byEntityID             map[uint64]PlayerEntity
	effectiveMapByEntityID map[uint64]uint32
	byMapIndex             map[uint32]map[uint64]PlayerEntity
}

func NewMapIndex(topology BootstrapTopology) *MapIndex {
	return &MapIndex{
		topology:               topology,
		byEntityID:             make(map[uint64]PlayerEntity),
		effectiveMapByEntityID: make(map[uint64]uint32),
		byMapIndex:             make(map[uint32]map[uint64]PlayerEntity),
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

	mapIndex := m.topology.EffectiveMapIndex(player.Character)
	m.byEntityID[player.Entity.ID] = player
	m.effectiveMapByEntityID[player.Entity.ID] = mapIndex
	bucket := m.byMapIndex[mapIndex]
	if bucket == nil {
		bucket = make(map[uint64]PlayerEntity)
		m.byMapIndex[mapIndex] = bucket
	}
	bucket[player.Entity.ID] = player
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
		return false
	}

	previousMapIndex := m.effectiveMapByEntityID[player.Entity.ID]
	nextMapIndex := m.topology.EffectiveMapIndex(player.Character)
	if bucket := m.byMapIndex[previousMapIndex]; bucket != nil {
		delete(bucket, previous.Entity.ID)
		if len(bucket) == 0 {
			delete(m.byMapIndex, previousMapIndex)
		}
	}

	m.byEntityID[player.Entity.ID] = player
	m.effectiveMapByEntityID[player.Entity.ID] = nextMapIndex
	bucket := m.byMapIndex[nextMapIndex]
	if bucket == nil {
		bucket = make(map[uint64]PlayerEntity)
		m.byMapIndex[nextMapIndex] = bucket
	}
	bucket[player.Entity.ID] = player
	return true
}

func (m *MapIndex) Remove(entityID uint64) (PlayerEntity, bool) {
	if m == nil || entityID == 0 {
		return PlayerEntity{}, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	player, ok := m.byEntityID[entityID]
	if !ok {
		return PlayerEntity{}, false
	}
	delete(m.byEntityID, entityID)
	mapIndex := m.effectiveMapByEntityID[entityID]
	delete(m.effectiveMapByEntityID, entityID)
	if bucket := m.byMapIndex[mapIndex]; bucket != nil {
		delete(bucket, entityID)
		if len(bucket) == 0 {
			delete(m.byMapIndex, mapIndex)
		}
	}
	return player, true
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
		characters = append(characters, player.Character)
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

	snapshots := make([]MapOccupancy, 0, len(m.byMapIndex))
	for mapIndex, bucket := range m.byMapIndex {
		characters := make([]loginticket.Character, 0, len(bucket))
		for _, player := range bucket {
			characters = append(characters, player.Character)
		}
		sortCharacters(characters)
		snapshots = append(snapshots, MapOccupancy{MapIndex: mapIndex, Characters: characters})
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].MapIndex < snapshots[j].MapIndex
	})
	return snapshots
}
