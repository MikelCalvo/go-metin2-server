package player

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

type SessionLink struct {
	Login          string
	CharacterIndex uint8
}

type Runtime struct {
	persisted     loginticket.Character
	live          worldruntime.Position
	liveGold      uint64
	livePoints    [255]int32
	liveInventory []inventory.ItemInstance
	liveEquipment []inventory.ItemInstance
	sessionLink   SessionLink
}

const (
	bootstrapConsumableVnum          uint32 = 27001
	bootstrapConsumablePointIndex    uint8  = 1
	bootstrapConsumablePointType     uint8  = 1
	bootstrapConsumablePointDelta    int32  = 50
	bootstrapConsumableEffectMessage        = "consume:27001:+50"
)

type ItemUseResult struct {
	Slot          inventory.SlotIndex
	ItemRemoved   bool
	Item          inventory.ItemInstance
	PointType     uint8
	PointAmount   int32
	PointValue    int32
	EffectMessage string
}

type MerchantBuyResult struct {
	Item inventory.ItemInstance
	Gold uint64
}

func NewRuntime(persisted loginticket.Character, sessionLink SessionLink) *Runtime {
	runtime := &Runtime{sessionLink: sessionLink}
	runtime.ApplyPersistedSnapshot(persisted)
	return runtime
}

func (r *Runtime) PersistedSnapshot() loginticket.Character {
	if r == nil {
		return loginticket.Character{}
	}
	return cloneCharacter(r.persisted)
}

func (r *Runtime) LiveCharacter() loginticket.Character {
	if r == nil {
		return loginticket.Character{}
	}
	live := r.PersistedSnapshot()
	live.MapIndex = r.live.MapIndex
	live.X = r.live.X
	live.Y = r.live.Y
	live.Gold = r.liveGold
	live.Points = r.livePoints
	live.Inventory = cloneItemInstances(r.liveInventory)
	live.Equipment = cloneItemInstances(r.liveEquipment)
	return live
}

func (r *Runtime) LivePosition() worldruntime.Position {
	if r == nil {
		return worldruntime.Position{}
	}
	return r.live
}

func (r *Runtime) LiveGold() uint64 {
	if r == nil {
		return 0
	}
	return r.liveGold
}

func (r *Runtime) LiveInventory() []inventory.ItemInstance {
	if r == nil {
		return []inventory.ItemInstance{}
	}
	return cloneItemInstances(r.liveInventory)
}

func (r *Runtime) LiveEquipment() []inventory.ItemInstance {
	if r == nil {
		return []inventory.ItemInstance{}
	}
	return cloneItemInstances(r.liveEquipment)
}

func (r *Runtime) SetLivePosition(mapIndex uint32, x int32, y int32) {
	if r == nil {
		return
	}
	r.live = worldruntime.NewPosition(mapIndex, x, y)
}

func (r *Runtime) SetLiveGold(gold uint64) {
	if r == nil {
		return
	}
	r.liveGold = gold
}

func (r *Runtime) MoveInventoryItem(from inventory.SlotIndex, to inventory.SlotIndex) (inventory.MoveResult, bool) {
	if r == nil {
		return inventory.MoveResult{}, false
	}
	result := inventory.MoveResult{From: from, To: to}
	if from == to {
		return result, true
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 {
		return inventory.MoveResult{}, false
	}
	movedItem, err := r.liveInventory[fromIndex].WithInventorySlot(to)
	if err != nil {
		return inventory.MoveResult{}, false
	}
	toIndex := findInventorySlot(r.liveInventory, to)
	if toIndex < 0 {
		r.liveInventory[fromIndex] = movedItem
		sortInventoryItems(r.liveInventory)
		result.Changed = true
		result.ToOccupied = true
		result.ToItem = movedItem
		return result, true
	}
	swappedItem, err := r.liveInventory[toIndex].WithInventorySlot(from)
	if err != nil {
		return inventory.MoveResult{}, false
	}
	r.liveInventory[fromIndex] = swappedItem
	r.liveInventory[toIndex] = movedItem
	sortInventoryItems(r.liveInventory)
	result.Changed = true
	result.FromOccupied = true
	result.FromItem = swappedItem
	result.ToOccupied = true
	result.ToItem = movedItem
	return result, true
}

func (r *Runtime) EquipItem(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot) (inventory.ItemInstance, bool) {
	if r == nil || !equipSlot.Valid() || equipmentSlotOccupied(r.liveEquipment, equipSlot) {
		return inventory.ItemInstance{}, false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 {
		return inventory.ItemInstance{}, false
	}
	item := r.liveInventory[fromIndex]
	item.Slot = 0
	item.Equipped = true
	item.EquipSlot = equipSlot
	if err := item.Validate(); err != nil {
		return inventory.ItemInstance{}, false
	}
	r.liveInventory = removeInventoryIndex(r.liveInventory, fromIndex)
	r.liveEquipment = append(r.liveEquipment, item)
	sortInventoryItems(r.liveInventory)
	sortEquipmentItems(r.liveEquipment)
	return item, true
}

func (r *Runtime) UnequipItem(equipSlot inventory.EquipmentSlot, to inventory.SlotIndex) (inventory.ItemInstance, bool) {
	if r == nil || !equipSlot.Valid() || inventorySlotOccupied(r.liveInventory, to) {
		return inventory.ItemInstance{}, false
	}
	equipIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if equipIndex < 0 {
		return inventory.ItemInstance{}, false
	}
	item, err := r.liveEquipment[equipIndex].WithInventorySlot(to)
	if err != nil {
		return inventory.ItemInstance{}, false
	}
	r.liveEquipment = removeInventoryIndex(r.liveEquipment, equipIndex)
	r.liveInventory = append(r.liveInventory, item)
	sortEquipmentItems(r.liveEquipment)
	sortInventoryItems(r.liveInventory)
	return item, true
}

func (r *Runtime) UseItem(slot inventory.SlotIndex) (ItemUseResult, bool) {
	if r == nil {
		return ItemUseResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return ItemUseResult{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Vnum != bootstrapConsumableVnum || item.Count == 0 {
		return ItemUseResult{}, false
	}
	currentPointValue := r.livePoints[bootstrapConsumablePointIndex]
	if currentPointValue > (1<<31-1)-bootstrapConsumablePointDelta {
		return ItemUseResult{}, false
	}
	updatedPointValue := currentPointValue + bootstrapConsumablePointDelta
	result := ItemUseResult{
		Slot:          slot,
		PointType:     bootstrapConsumablePointType,
		PointAmount:   bootstrapConsumablePointDelta,
		PointValue:    updatedPointValue,
		EffectMessage: bootstrapConsumableEffectMessage,
	}
	r.livePoints[bootstrapConsumablePointIndex] = updatedPointValue
	if item.Count == 1 {
		r.liveInventory = removeInventoryIndex(r.liveInventory, index)
		sortInventoryItems(r.liveInventory)
		result.ItemRemoved = true
		return result, true
	}
	item.Count--
	if err := item.Validate(); err != nil {
		r.livePoints[bootstrapConsumablePointIndex] = currentPointValue
		return ItemUseResult{}, false
	}
	r.liveInventory[index] = item
	sortInventoryItems(r.liveInventory)
	result.Item = item
	return result, true
}

func (r *Runtime) BuyMerchantItem(vnum uint32, count uint16, price uint64) (MerchantBuyResult, bool) {
	if r == nil || vnum == 0 || count == 0 || price == 0 || r.liveGold < price {
		return MerchantBuyResult{}, false
	}
	slot, ok := nextFreeInventorySlot(r.liveInventory)
	if !ok {
		return MerchantBuyResult{}, false
	}
	item, err := (inventory.ItemInstance{ID: nextLiveItemInstanceID(r.liveInventory, r.liveEquipment), Vnum: vnum, Count: count}).WithInventorySlot(slot)
	if err != nil {
		return MerchantBuyResult{}, false
	}
	r.liveGold -= price
	r.liveInventory = append(r.liveInventory, item)
	sortInventoryItems(r.liveInventory)
	return MerchantBuyResult{Item: item, Gold: r.liveGold}, true
}

func (r *Runtime) ApplyPersistedSnapshot(persisted loginticket.Character) {
	if r == nil {
		return
	}
	r.persisted = normalizeCharacter(persisted)
	r.live = worldruntime.PositionFromCharacter(r.persisted)
	r.liveGold = r.persisted.Gold
	r.livePoints = r.persisted.Points
	r.liveInventory = cloneItemInstances(r.persisted.Inventory)
	r.liveEquipment = cloneItemInstances(r.persisted.Equipment)
	sortInventoryItems(r.liveInventory)
	sortEquipmentItems(r.liveEquipment)
}

func (r *Runtime) SessionLink() SessionLink {
	if r == nil {
		return SessionLink{}
	}
	return r.sessionLink
}

func cloneCharacter(character loginticket.Character) loginticket.Character {
	cloned := loginticket.CloneCharacters([]loginticket.Character{character})
	if len(cloned) == 0 {
		return loginticket.Character{}
	}
	return cloned[0]
}

func normalizeCharacter(character loginticket.Character) loginticket.Character {
	cloned := cloneCharacter(character)
	cloned.NormalizeItemState()
	return cloned
}

func cloneItemInstances(items []inventory.ItemInstance) []inventory.ItemInstance {
	if items == nil {
		return []inventory.ItemInstance{}
	}
	return append([]inventory.ItemInstance(nil), items...)
}

func findInventorySlot(items []inventory.ItemInstance, slot inventory.SlotIndex) int {
	for i, item := range items {
		if item.Slot == slot {
			return i
		}
	}
	return -1
}

func findEquipmentSlot(items []inventory.ItemInstance, slot inventory.EquipmentSlot) int {
	for i, item := range items {
		if item.Equipped && item.EquipSlot == slot {
			return i
		}
	}
	return -1
}

func inventorySlotOccupied(items []inventory.ItemInstance, slot inventory.SlotIndex) bool {
	return findInventorySlot(items, slot) >= 0
}

func equipmentSlotOccupied(items []inventory.ItemInstance, slot inventory.EquipmentSlot) bool {
	return findEquipmentSlot(items, slot) >= 0
}

func removeInventoryIndex(items []inventory.ItemInstance, index int) []inventory.ItemInstance {
	if index < 0 || index >= len(items) {
		return items
	}
	return append(items[:index], items[index+1:]...)
}

func nextFreeInventorySlot(items []inventory.ItemInstance) (inventory.SlotIndex, bool) {
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		if !inventorySlotOccupied(items, slot) {
			return slot, true
		}
	}
	return 0, false
}

func nextLiveItemInstanceID(inventoryItems []inventory.ItemInstance, equipmentItems []inventory.ItemInstance) uint64 {
	var maxID uint64
	for _, item := range inventoryItems {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	for _, item := range equipmentItems {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	if maxID == 0 {
		return 1
	}
	return maxID + 1
}

func sortInventoryItems(items []inventory.ItemInstance) {
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Slot != items[j].Slot {
			return items[i].Slot < items[j].Slot
		}
		return items[i].ID < items[j].ID
	})
}

func sortEquipmentItems(items []inventory.ItemInstance) {
	order := equipmentSlotOrderIndex()
	sort.Slice(items, func(i int, j int) bool {
		left := order[items[i].EquipSlot]
		right := order[items[j].EquipSlot]
		if left != right {
			return left < right
		}
		return items[i].ID < items[j].ID
	})
}

func equipmentSlotOrderIndex() map[inventory.EquipmentSlot]int {
	order := make(map[inventory.EquipmentSlot]int, len(inventory.AllEquipmentSlots()))
	for idx, slot := range inventory.AllEquipmentSlots() {
		order[slot] = idx
	}
	return order
}
