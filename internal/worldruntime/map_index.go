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
	mu                           sync.Mutex
	topology                     BootstrapTopology
	byEntityID                   map[uint64]PlayerEntity
	effectiveMapByEntityID       map[uint64]uint32
	byMapIndex                   map[uint32]map[uint64]PlayerEntity
	staticByEntityID             map[uint64]StaticEntity
	effectiveStaticMapByEntityID map[uint64]uint32
	staticByMapIndex             map[uint32]map[uint64]StaticEntity
}

func NewMapIndex(topology BootstrapTopology) *MapIndex {
	return &MapIndex{
		topology:                     topology,
		byEntityID:                   make(map[uint64]PlayerEntity),
		effectiveMapByEntityID:       make(map[uint64]uint32),
		byMapIndex:                   make(map[uint32]map[uint64]PlayerEntity),
		staticByEntityID:             make(map[uint64]StaticEntity),
		effectiveStaticMapByEntityID: make(map[uint64]uint32),
		staticByMapIndex:             make(map[uint32]map[uint64]StaticEntity),
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
	if _, ok := m.staticByEntityID[player.Entity.ID]; ok {
		return false
	}
	if _, ok := m.staticActorMapPresenceLocked(player.Entity.ID); ok {
		return false
	}
	if m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID) {
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

	if _, ok := m.byEntityID[player.Entity.ID]; !ok {
		if _, found := m.playerMapPresenceLocked(player.Entity.ID); !found {
			return false
		}
	}
	if _, ok := m.staticByEntityID[player.Entity.ID]; ok {
		return false
	}
	if _, ok := m.staticActorMapPresenceLocked(player.Entity.ID); ok {
		return false
	}
	if m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID) {
		return false
	}

	nextMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: player.Position().MapIndex})
	m.removePlayerMapPresenceLocked(player.Entity.ID)

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
	if !ok {
		player, ok = m.canonicalPlayerMapPresenceLocked(entityID)
	}
	if !ok {
		player, ok = m.playerMapPresenceLocked(entityID)
	}
	if ok {
		delete(m.byEntityID, entityID)
		m.removePlayerMapPresenceLocked(entityID)
		return clonePlayerEntity(player), true
	}
	delete(m.effectiveMapByEntityID, entityID)
	return PlayerEntity{}, false
}

func (m *MapIndex) removePlayerMapPresenceLocked(entityID uint64) {
	delete(m.effectiveMapByEntityID, entityID)
	for mapIndex, bucket := range m.byMapIndex {
		if _, ok := bucket[entityID]; !ok {
			continue
		}
		delete(bucket, entityID)
		if len(bucket) == 0 {
			delete(m.byMapIndex, mapIndex)
		}
	}
}

func (m *MapIndex) PlayerByVID(vid uint32) (PlayerEntity, bool) {
	if m == nil || vid == 0 {
		return PlayerEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repairMisplacedPlayerMapPresenceLocked()
	m.repairPlayerMapPresenceFromEffectiveIndexLocked()
	for _, player := range m.byEntityID {
		if player.Entity.VID == vid {
			if m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID) {
				return PlayerEntity{}, false
			}
			m.repairPlayerMapPresenceIfUnblockedLocked(player)
			return clonePlayerEntity(player), true
		}
	}
	for _, bucket := range m.byMapIndex {
		for _, player := range bucket {
			if player.Entity.VID != vid {
				continue
			}
			if current, ok := m.byEntityID[player.Entity.ID]; ok {
				m.repairPlayerMapPresenceIfUnblockedLocked(current)
				if current.Entity.VID == vid {
					return clonePlayerEntity(current), true
				}
				continue
			}
			if m.playerRepairBlockedByStaticPresenceLocked(player.Entity.ID) || m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID) {
				return PlayerEntity{}, false
			}
			return clonePlayerEntity(player), true
		}
	}
	return PlayerEntity{}, false
}

func (m *MapIndex) PlayerByName(name string) (PlayerEntity, bool) {
	if m == nil || name == "" {
		return PlayerEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repairMisplacedPlayerMapPresenceLocked()
	m.repairPlayerMapPresenceFromEffectiveIndexLocked()
	for _, player := range m.byEntityID {
		if player.Entity.Name == name {
			m.repairPlayerMapPresenceIfUnblockedLocked(player)
			return clonePlayerEntity(player), true
		}
	}
	for _, bucket := range m.byMapIndex {
		for _, player := range bucket {
			if player.Entity.Name != name {
				continue
			}
			if current, ok := m.byEntityID[player.Entity.ID]; ok {
				m.repairPlayerMapPresenceIfUnblockedLocked(current)
				if current.Entity.Name == name {
					return clonePlayerEntity(current), true
				}
				continue
			}
			if m.playerRepairBlockedByStaticPresenceLocked(player.Entity.ID) || m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID) {
				return PlayerEntity{}, false
			}
			return clonePlayerEntity(player), true
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
	m.repairMisplacedPlayerMapPresenceLocked()
	m.repairPlayerMapPresenceFromEffectiveIndexLocked()
	player, ok := m.byEntityID[entityID]
	if ok {
		m.repairPlayerMapPresenceIfUnblockedLocked(player)
		return clonePlayerEntity(player), true
	}
	player, ok = m.playerMapPresenceLocked(entityID)
	if !ok {
		return PlayerEntity{}, false
	}
	if m.playerRepairBlockedByStaticPresenceLocked(entityID) || m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, entityID) {
		return PlayerEntity{}, false
	}
	return player, true
}

func (m *MapIndex) playerRepairBlockedByStaticPresenceLocked(entityID uint64) bool {
	if entityID == 0 {
		return true
	}
	if _, ok := m.staticByEntityID[entityID]; ok {
		return true
	}
	if _, ok := m.staticActorMapPresenceLocked(entityID); ok {
		return true
	}
	return false
}

func (m *MapIndex) staticActorVisibilityVIDPresenceLocked(vid uint32, entityID uint64) bool {
	if vid == 0 {
		return false
	}
	for _, actor := range m.staticByEntityID {
		actorVID, ok := StaticActorVisibilityVID(actor)
		if ok && actorVID == vid && actor.Entity.ID != entityID {
			return true
		}
	}
	for _, bucket := range m.staticByMapIndex {
		for _, actor := range bucket {
			actorVID, ok := StaticActorVisibilityVID(actor)
			if ok && actorVID == vid && actor.Entity.ID != entityID {
				return true
			}
		}
	}
	return false
}

func (m *MapIndex) repairPlayerMapPresenceIfUnblockedLocked(player PlayerEntity) bool {
	if m.playerRepairBlockedByStaticPresenceLocked(player.Entity.ID) || m.staticActorVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID) {
		return false
	}
	m.repairPlayerMapPresenceLocked(player)
	return true
}

func (m *MapIndex) repairPlayerMapPresenceLocked(player PlayerEntity) {
	mapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: player.Position().MapIndex})
	m.removePlayerMapPresenceLocked(player.Entity.ID)
	m.effectiveMapByEntityID[player.Entity.ID] = mapIndex
	bucket := m.byMapIndex[mapIndex]
	if bucket == nil {
		bucket = make(map[uint64]PlayerEntity)
		m.byMapIndex[mapIndex] = bucket
	}
	bucket[player.Entity.ID] = clonePlayerEntity(player)
}

func (m *MapIndex) repairMisplacedPlayerMapPresenceLocked() {
	type repairCandidate struct {
		sourceMap uint32
		player    PlayerEntity
	}

	repairs := make(map[uint64]repairCandidate)
	for mapIndex, bucket := range m.byMapIndex {
		for entityID, player := range bucket {
			effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: player.Position().MapIndex})
			if effectiveMapIndex == mapIndex {
				continue
			}
			if current, ok := repairs[entityID]; ok && current.sourceMap <= mapIndex {
				continue
			}
			repairs[entityID] = repairCandidate{sourceMap: mapIndex, player: clonePlayerEntity(player)}
		}
	}
	if len(repairs) == 0 {
		return
	}

	entityIDs := make([]uint64, 0, len(repairs))
	for entityID := range repairs {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i int, j int) bool {
		return entityIDs[i] < entityIDs[j]
	})
	for _, entityID := range entityIDs {
		player := repairs[entityID].player
		if current, ok := m.byEntityID[entityID]; ok {
			player = current
		} else if canonical, ok := m.canonicalPlayerMapPresenceLocked(entityID); ok {
			player = canonical
		}
		if _, ok := m.staticByEntityID[entityID]; ok {
			continue
		}
		if _, ok := m.staticActorMapPresenceLocked(entityID); ok {
			continue
		}
		m.repairPlayerMapPresenceLocked(player)
	}
}

func (m *MapIndex) canonicalPlayerMapPresenceLocked(entityID uint64) (PlayerEntity, bool) {
	if m == nil || entityID == 0 {
		return PlayerEntity{}, false
	}
	if effectiveMapIndex, ok := m.effectiveMapByEntityID[entityID]; ok {
		if bucket := m.byMapIndex[effectiveMapIndex]; bucket != nil {
			if player, ok := bucket[entityID]; ok {
				return clonePlayerEntity(player), true
			}
		}
	}
	var selected PlayerEntity
	var selectedMap uint32
	found := false
	for mapIndex, bucket := range m.byMapIndex {
		player, ok := bucket[entityID]
		if !ok {
			continue
		}
		effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: player.Position().MapIndex})
		if effectiveMapIndex != mapIndex {
			continue
		}
		if found && selectedMap <= mapIndex {
			continue
		}
		selected = clonePlayerEntity(player)
		selectedMap = mapIndex
		found = true
	}
	return selected, found
}

func (m *MapIndex) repairPlayerMapPresenceFromEffectiveIndexLocked() {
	if m == nil || len(m.effectiveMapByEntityID) == 0 {
		return
	}
	entityIDs := make([]uint64, 0, len(m.effectiveMapByEntityID))
	for entityID := range m.effectiveMapByEntityID {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i int, j int) bool {
		return entityIDs[i] < entityIDs[j]
	})
	for _, entityID := range entityIDs {
		if _, ok := m.byEntityID[entityID]; ok {
			continue
		}
		if _, ok := m.staticByEntityID[entityID]; ok {
			continue
		}
		if _, ok := m.staticActorMapPresenceLocked(entityID); ok {
			continue
		}
		effectiveMapIndex := m.effectiveMapByEntityID[entityID]
		bucket := m.byMapIndex[effectiveMapIndex]
		player, ok := bucket[entityID]
		if !ok {
			if _, found := m.playerMapPresenceLocked(entityID); !found {
				delete(m.effectiveMapByEntityID, entityID)
			}
			continue
		}
		m.repairPlayerMapPresenceLocked(player)
	}
}

func (m *MapIndex) PlayerCharacters(mapIndex uint32) []loginticket.Character {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, player := range m.byEntityID {
		m.repairPlayerMapPresenceIfUnblockedLocked(player)
	}
	m.repairMisplacedPlayerMapPresenceLocked()
	m.repairPlayerMapPresenceFromEffectiveIndexLocked()

	effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: mapIndex})
	bucket := m.byMapIndex[effectiveMapIndex]
	if len(bucket) == 0 {
		return nil
	}
	characters := make([]loginticket.Character, 0, len(bucket))
	for _, player := range bucket {
		if m.playerMapPresenceBlockedByStaticMapLocked(player) {
			continue
		}
		characters = append(characters, cloneCharacterSnapshot(player.Character))
	}
	sortCharacters(characters)
	return characters
}

func (m *MapIndex) AllPlayers() []PlayerEntity {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.byEntityID) == 0 && len(m.byMapIndex) == 0 {
		return nil
	}

	for _, player := range m.byEntityID {
		m.repairPlayerMapPresenceIfUnblockedLocked(player)
	}
	m.repairMisplacedPlayerMapPresenceLocked()
	m.repairPlayerMapPresenceFromEffectiveIndexLocked()

	playersByID := make(map[uint64]PlayerEntity, len(m.byEntityID))
	for _, player := range m.byEntityID {
		if m.playerMapPresenceBlockedByStaticMapLocked(player) {
			continue
		}
		playersByID[player.Entity.ID] = clonePlayerEntity(player)
	}

	mapIndices := make([]uint32, 0, len(m.byMapIndex))
	for mapIndex := range m.byMapIndex {
		mapIndices = append(mapIndices, mapIndex)
	}
	sort.Slice(mapIndices, func(i int, j int) bool {
		return mapIndices[i] < mapIndices[j]
	})
	for _, mapIndex := range mapIndices {
		bucket := m.byMapIndex[mapIndex]
		entityIDs := make([]uint64, 0, len(bucket))
		for entityID := range bucket {
			entityIDs = append(entityIDs, entityID)
		}
		sort.Slice(entityIDs, func(i int, j int) bool {
			return entityIDs[i] < entityIDs[j]
		})
		for _, entityID := range entityIDs {
			if _, exists := playersByID[entityID]; exists {
				continue
			}
			if m.playerMapPresenceBlockedByStaticMapLocked(bucket[entityID]) {
				continue
			}
			playersByID[entityID] = clonePlayerEntity(bucket[entityID])
		}
	}

	players := make([]PlayerEntity, 0, len(playersByID))
	for _, player := range playersByID {
		players = append(players, player)
	}
	sortPlayerEntities(players)
	return players
}

func (m *MapIndex) Snapshot() []MapOccupancy {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, player := range m.byEntityID {
		m.repairPlayerMapPresenceIfUnblockedLocked(player)
	}
	m.repairMisplacedPlayerMapPresenceLocked()
	m.repairPlayerMapPresenceFromEffectiveIndexLocked()
	for _, actor := range m.staticByEntityID {
		m.repairStaticMapPresenceIfUnblockedLocked(actor)
	}
	m.repairMisplacedStaticMapPresenceLocked()
	m.repairStaticMapPresenceFromEffectiveIndexLocked()

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
			if m.playerMapPresenceBlockedByStaticMapLocked(player) {
				continue
			}
			characters = append(characters, cloneCharacterSnapshot(player.Character))
		}
		sortCharacters(characters)

		actors := make([]StaticEntity, 0, len(m.staticByMapIndex[mapIndex]))
		for _, actor := range m.staticByMapIndex[mapIndex] {
			if m.staticMapPresenceBlockedByPlayerMapLocked(actor) {
				continue
			}
			actors = append(actors, cloneStaticEntity(actor))
		}
		sortStaticEntities(actors)
		if len(characters) == 0 && len(actors) == 0 {
			continue
		}

		snapshots = append(snapshots, MapOccupancy{MapIndex: mapIndex, Characters: characters, StaticActors: actors})
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].MapIndex < snapshots[j].MapIndex
	})
	return snapshots
}

func (m *MapIndex) RegisterStatic(actor StaticEntity) bool {
	actor = normalizeStaticEntityCombat(actor)
	if m == nil || !validStaticEntity(actor) {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.staticByEntityID[actor.Entity.ID]; ok {
		return false
	}
	if _, ok := m.byEntityID[actor.Entity.ID]; ok {
		return false
	}
	if _, ok := m.playerMapPresenceLocked(actor.Entity.ID); ok {
		return false
	}
	if m.playerVisibilityVIDPresenceLocked(actor, actor.Entity.ID) {
		return false
	}
	actor = cloneStaticEntity(actor)
	mapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
	m.removeStaticMapPresenceLocked(actor.Entity.ID)
	m.staticByEntityID[actor.Entity.ID] = actor
	m.effectiveStaticMapByEntityID[actor.Entity.ID] = mapIndex
	bucket := m.staticByMapIndex[mapIndex]
	if bucket == nil {
		bucket = make(map[uint64]StaticEntity)
		m.staticByMapIndex[mapIndex] = bucket
	}
	bucket[actor.Entity.ID] = actor
	return true
}

func (m *MapIndex) UpdateStatic(actor StaticEntity) bool {
	actor = normalizeStaticEntityCombat(actor)
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
	if _, ok := m.byEntityID[actor.Entity.ID]; ok {
		return false
	}
	if _, ok := m.playerMapPresenceLocked(actor.Entity.ID); ok {
		return false
	}
	if m.playerVisibilityVIDPresenceLocked(actor, actor.Entity.ID) {
		return false
	}
	nextMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
	m.removeStaticMapPresenceLocked(actor.Entity.ID)
	actor = cloneStaticEntity(actor)
	m.staticByEntityID[actor.Entity.ID] = actor
	m.effectiveStaticMapByEntityID[actor.Entity.ID] = nextMapIndex
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
	m.repairMisplacedStaticMapPresenceLocked()
	m.repairStaticMapPresenceFromEffectiveIndexLocked()
	actor, ok := m.staticByEntityID[entityID]
	if ok {
		m.repairStaticMapPresenceIfUnblockedLocked(actor)
		return cloneStaticEntity(actor), true
	}
	if m.staticRepairBlockedByPlayerPresenceLocked(entityID) {
		return StaticEntity{}, false
	}
	actor, ok = m.staticActorMapPresenceLocked(entityID)
	if !ok {
		return StaticEntity{}, false
	}
	if m.staticRepairBlockedByPlayerPresenceLocked(entityID) || m.playerVisibilityVIDPresenceLocked(actor, entityID) {
		return StaticEntity{}, false
	}
	return cloneStaticEntity(actor), true
}

func (m *MapIndex) StaticActorByVID(vid uint32) (StaticEntity, bool) {
	if m == nil || vid == 0 {
		return StaticEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.repairMisplacedStaticMapPresenceLocked()
	m.repairStaticMapPresenceFromEffectiveIndexLocked()
	for _, actor := range m.staticByEntityID {
		canonicalVID, ok := StaticActorVisibilityVID(actor)
		if !ok || canonicalVID != vid {
			continue
		}
		if m.playerVisibilityVIDPresenceLocked(actor, actor.Entity.ID) {
			return StaticEntity{}, false
		}
		m.repairStaticMapPresenceIfUnblockedLocked(actor)
		return cloneStaticEntity(actor), true
	}
	for _, bucket := range m.staticByMapIndex {
		for _, actor := range bucket {
			canonicalVID, ok := StaticActorVisibilityVID(actor)
			if !ok || canonicalVID != vid {
				continue
			}
			if current, exists := m.staticByEntityID[actor.Entity.ID]; exists {
				m.repairStaticMapPresenceIfUnblockedLocked(current)
				if currentVID, ok := StaticActorVisibilityVID(current); ok && currentVID == vid {
					return cloneStaticEntity(current), true
				}
				continue
			}
			if m.staticRepairBlockedByPlayerPresenceLocked(actor.Entity.ID) || m.playerVisibilityVIDPresenceLocked(actor, actor.Entity.ID) {
				return StaticEntity{}, false
			}
			return cloneStaticEntity(actor), true
		}
	}
	return StaticEntity{}, false
}

func (m *MapIndex) AllStaticActors() []StaticEntity {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.staticByEntityID) == 0 && len(m.staticByMapIndex) == 0 {
		return nil
	}
	m.repairMisplacedStaticMapPresenceLocked()
	m.repairStaticMapPresenceFromEffectiveIndexLocked()

	actorsByID := make(map[uint64]StaticEntity, len(m.staticByEntityID))
	for _, actor := range m.staticByEntityID {
		m.repairStaticMapPresenceIfUnblockedLocked(actor)
		if m.staticMapPresenceBlockedByPlayerMapLocked(actor) {
			continue
		}
		actorsByID[actor.Entity.ID] = cloneStaticEntity(actor)
	}

	mapIndices := make([]uint32, 0, len(m.staticByMapIndex))
	for mapIndex := range m.staticByMapIndex {
		mapIndices = append(mapIndices, mapIndex)
	}
	sort.Slice(mapIndices, func(i int, j int) bool {
		return mapIndices[i] < mapIndices[j]
	})
	for _, mapIndex := range mapIndices {
		bucket := m.staticByMapIndex[mapIndex]
		entityIDs := make([]uint64, 0, len(bucket))
		for entityID := range bucket {
			entityIDs = append(entityIDs, entityID)
		}
		sort.Slice(entityIDs, func(i int, j int) bool {
			return entityIDs[i] < entityIDs[j]
		})
		for _, entityID := range entityIDs {
			if _, exists := actorsByID[entityID]; exists {
				continue
			}
			if m.staticMapPresenceBlockedByPlayerMapLocked(bucket[entityID]) {
				continue
			}
			actorsByID[entityID] = cloneStaticEntity(bucket[entityID])
		}
	}

	actors := make([]StaticEntity, 0, len(actorsByID))
	for _, actor := range actorsByID {
		actors = append(actors, actor)
	}
	sortStaticEntities(actors)
	return actors
}

func (m *MapIndex) repairStaticMapPresenceLocked(actor StaticEntity) {
	m.removeStaticMapPresenceLocked(actor.Entity.ID)
	mapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
	m.effectiveStaticMapByEntityID[actor.Entity.ID] = mapIndex
	bucket := m.staticByMapIndex[mapIndex]
	if bucket == nil {
		bucket = make(map[uint64]StaticEntity)
		m.staticByMapIndex[mapIndex] = bucket
	}
	bucket[actor.Entity.ID] = cloneStaticEntity(actor)
}

func (m *MapIndex) staticRepairBlockedByPlayerPresenceLocked(entityID uint64) bool {
	if entityID == 0 {
		return true
	}
	if _, ok := m.byEntityID[entityID]; ok {
		return true
	}
	if _, ok := m.playerMapPresenceLocked(entityID); ok {
		return true
	}
	return false
}

func (m *MapIndex) playerVisibilityVIDPresenceLocked(actor StaticEntity, entityID uint64) bool {
	vid, ok := StaticActorVisibilityVID(actor)
	if !ok {
		return false
	}
	for _, player := range m.byEntityID {
		if player.Entity.VID == vid && player.Entity.ID != entityID {
			return true
		}
	}
	for _, bucket := range m.byMapIndex {
		for _, player := range bucket {
			if player.Entity.VID == vid && player.Entity.ID != entityID {
				return true
			}
		}
	}
	return false
}

func (m *MapIndex) repairStaticMapPresenceIfUnblockedLocked(actor StaticEntity) bool {
	if m.staticRepairBlockedByPlayerPresenceLocked(actor.Entity.ID) || m.playerVisibilityVIDPresenceLocked(actor, actor.Entity.ID) {
		return false
	}
	m.repairStaticMapPresenceLocked(actor)
	return true
}

func (m *MapIndex) repairMisplacedStaticMapPresenceLocked() {
	type repairCandidate struct {
		sourceMap uint32
		actor     StaticEntity
	}

	repairs := make(map[uint64]repairCandidate)
	for mapIndex, bucket := range m.staticByMapIndex {
		for entityID, actor := range bucket {
			effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
			if effectiveMapIndex == mapIndex {
				continue
			}
			if current, ok := repairs[entityID]; ok && current.sourceMap <= mapIndex {
				continue
			}
			repairs[entityID] = repairCandidate{sourceMap: mapIndex, actor: cloneStaticEntity(actor)}
		}
	}
	if len(repairs) == 0 {
		return
	}

	entityIDs := make([]uint64, 0, len(repairs))
	for entityID := range repairs {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i int, j int) bool {
		return entityIDs[i] < entityIDs[j]
	})
	for _, entityID := range entityIDs {
		actor := repairs[entityID].actor
		if current, ok := m.staticByEntityID[entityID]; ok {
			actor = current
		} else if canonical, ok := m.canonicalStaticMapPresenceLocked(entityID); ok {
			actor = canonical
		}
		if _, ok := m.byEntityID[entityID]; ok {
			continue
		}
		if _, ok := m.playerMapPresenceLocked(entityID); ok {
			continue
		}
		m.repairStaticMapPresenceLocked(actor)
	}
}

func (m *MapIndex) canonicalStaticMapPresenceLocked(entityID uint64) (StaticEntity, bool) {
	if m == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	if effectiveMapIndex, ok := m.effectiveStaticMapByEntityID[entityID]; ok {
		if bucket := m.staticByMapIndex[effectiveMapIndex]; bucket != nil {
			if actor, ok := bucket[entityID]; ok {
				return cloneStaticEntity(actor), true
			}
		}
	}
	var selected StaticEntity
	var selectedMap uint32
	found := false
	for mapIndex, bucket := range m.staticByMapIndex {
		actor, ok := bucket[entityID]
		if !ok {
			continue
		}
		effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: actor.Position.MapIndex})
		if effectiveMapIndex != mapIndex {
			continue
		}
		if found && selectedMap <= mapIndex {
			continue
		}
		selected = cloneStaticEntity(actor)
		selectedMap = mapIndex
		found = true
	}
	return selected, found
}

func (m *MapIndex) repairStaticMapPresenceFromEffectiveIndexLocked() {
	if m == nil || len(m.effectiveStaticMapByEntityID) == 0 {
		return
	}
	entityIDs := make([]uint64, 0, len(m.effectiveStaticMapByEntityID))
	for entityID := range m.effectiveStaticMapByEntityID {
		entityIDs = append(entityIDs, entityID)
	}
	sort.Slice(entityIDs, func(i int, j int) bool {
		return entityIDs[i] < entityIDs[j]
	})
	for _, entityID := range entityIDs {
		if _, ok := m.staticByEntityID[entityID]; ok {
			continue
		}
		if _, ok := m.byEntityID[entityID]; ok {
			continue
		}
		if _, ok := m.playerMapPresenceLocked(entityID); ok {
			continue
		}
		effectiveMapIndex := m.effectiveStaticMapByEntityID[entityID]
		bucket := m.staticByMapIndex[effectiveMapIndex]
		actor, ok := bucket[entityID]
		if !ok {
			if _, found := m.staticActorMapPresenceLocked(entityID); !found {
				delete(m.effectiveStaticMapByEntityID, entityID)
			}
			continue
		}
		m.repairStaticMapPresenceLocked(actor)
	}
}

func (m *MapIndex) RemoveStatic(entityID uint64) (StaticEntity, bool) {
	if m == nil || entityID == 0 {
		return StaticEntity{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	actor, ok := m.staticByEntityID[entityID]
	if !ok {
		actor, ok = m.canonicalStaticMapPresenceLocked(entityID)
	}
	if !ok {
		actor, ok = m.staticActorMapPresenceLocked(entityID)
	}
	if ok {
		delete(m.staticByEntityID, entityID)
		m.removeStaticMapPresenceLocked(entityID)
		return cloneStaticEntity(actor), true
	}
	delete(m.effectiveStaticMapByEntityID, entityID)
	return StaticEntity{}, false
}

func (m *MapIndex) removeStaticMapPresenceLocked(entityID uint64) {
	delete(m.effectiveStaticMapByEntityID, entityID)
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
	for _, actor := range m.staticByEntityID {
		m.repairStaticMapPresenceIfUnblockedLocked(actor)
	}
	m.repairMisplacedStaticMapPresenceLocked()
	m.repairStaticMapPresenceFromEffectiveIndexLocked()
	effectiveMapIndex := m.topology.EffectiveMapIndex(loginticket.Character{MapIndex: mapIndex})
	bucket := m.staticByMapIndex[effectiveMapIndex]
	if len(bucket) == 0 {
		return nil
	}
	actors := make([]StaticEntity, 0, len(bucket))
	for _, actor := range bucket {
		if m.staticMapPresenceBlockedByPlayerMapLocked(actor) {
			continue
		}
		actors = append(actors, cloneStaticEntity(actor))
	}
	sortStaticEntities(actors)
	return actors
}

func (m *MapIndex) playerMapPresenceBlockedByStaticMapLocked(player PlayerEntity) bool {
	if player.Entity.ID == 0 {
		return true
	}
	if _, primary := m.byEntityID[player.Entity.ID]; primary {
		return m.staticActorMapVisibilityVIDPresenceLocked(player.Entity.VID, player.Entity.ID)
	}
	return m.mapOnlyStaticMapPresenceCollidesWithPlayerLocked(player)
}

func (m *MapIndex) staticMapPresenceBlockedByPlayerMapLocked(actor StaticEntity) bool {
	if actor.Entity.ID == 0 {
		return true
	}
	if _, primary := m.staticByEntityID[actor.Entity.ID]; primary {
		if _, ok := m.playerMapPresenceLocked(actor.Entity.ID); ok {
			return true
		}
		return m.playerMapVisibilityVIDPresenceLocked(actor, actor.Entity.ID)
	}
	return m.mapOnlyPlayerMapPresenceCollidesWithStaticLocked(actor)
}

func (m *MapIndex) mapOnlyStaticMapPresenceCollidesWithPlayerLocked(player PlayerEntity) bool {
	if player.Entity.ID == 0 {
		return true
	}
	for _, bucket := range m.staticByMapIndex {
		for _, actor := range bucket {
			if _, primary := m.staticByEntityID[actor.Entity.ID]; primary {
				continue
			}
			if actor.Entity.ID == player.Entity.ID {
				return true
			}
			if player.Entity.VID == 0 {
				continue
			}
			actorVID, ok := StaticActorVisibilityVID(actor)
			if ok && actorVID == player.Entity.VID {
				return true
			}
		}
	}
	return false
}

func (m *MapIndex) mapOnlyPlayerMapPresenceCollidesWithStaticLocked(actor StaticEntity) bool {
	if actor.Entity.ID == 0 {
		return true
	}
	actorVID, actorHasVID := StaticActorVisibilityVID(actor)
	for _, bucket := range m.byMapIndex {
		for _, player := range bucket {
			if _, primary := m.byEntityID[player.Entity.ID]; primary {
				continue
			}
			if player.Entity.ID == actor.Entity.ID {
				return true
			}
			if actorHasVID && player.Entity.VID == actorVID {
				return true
			}
		}
	}
	return false
}

func (m *MapIndex) staticActorMapVisibilityVIDPresenceLocked(vid uint32, entityID uint64) bool {
	if vid == 0 {
		return false
	}
	for _, bucket := range m.staticByMapIndex {
		for _, actor := range bucket {
			actorVID, ok := StaticActorVisibilityVID(actor)
			if ok && actorVID == vid && actor.Entity.ID != entityID {
				return true
			}
		}
	}
	return false
}

func (m *MapIndex) playerMapVisibilityVIDPresenceLocked(actor StaticEntity, entityID uint64) bool {
	vid, ok := StaticActorVisibilityVID(actor)
	if !ok {
		return false
	}
	for _, bucket := range m.byMapIndex {
		for _, player := range bucket {
			if player.Entity.VID == vid && player.Entity.ID != entityID {
				return true
			}
		}
	}
	return false
}
