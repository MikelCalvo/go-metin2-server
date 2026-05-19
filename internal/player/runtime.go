package player

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
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

type ItemUseResult struct {
	Slot          inventory.SlotIndex
	ItemRemoved   bool
	Item          inventory.ItemInstance
	PointType     uint8
	PointAmount   int32
	PointValue    int32
	EffectMessage string
}

type PointChangeResult struct {
	PointType   uint8
	PointAmount int32
	PointValue  int32
}

type MerchantBuyResult struct {
	Items []inventory.ItemInstance
	Gold  uint64
}

type MerchantSellResult struct {
	Slot        inventory.SlotIndex
	ItemRemoved bool
	Item        inventory.ItemInstance
	Gold        uint64
}

type MerchantBuyFailure string

const (
	MerchantBuyFailureInvalid          MerchantBuyFailure = "invalid"
	MerchantBuyFailureInsufficientGold MerchantBuyFailure = "insufficient_gold"
	MerchantBuyFailureNoValidPlacement MerchantBuyFailure = "no_valid_placement"
)

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

func (r *Runtime) ApplyPointDelta(pointType uint8, pointIndex uint8, pointDelta int32) (PointChangeResult, bool) {
	if r == nil || int(pointIndex) >= len(r.livePoints) {
		return PointChangeResult{}, false
	}
	currentPointValue := r.livePoints[pointIndex]
	nextPointValue := int64(currentPointValue) + int64(pointDelta)
	if nextPointValue < -1<<31 || nextPointValue > 1<<31-1 {
		return PointChangeResult{}, false
	}
	updatedPointValue := int32(nextPointValue)
	r.livePoints[pointIndex] = updatedPointValue
	return PointChangeResult{PointType: pointType, PointAmount: pointDelta, PointValue: updatedPointValue}, true
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

func (r *Runtime) ApplyEquipTemplateEffect(template itemcatalog.Template) (PointChangeResult, bool) {
	if r == nil || !itemcatalog.ValidTemplate(template) || template.EquipEffect == nil {
		return PointChangeResult{}, false
	}
	effect := *template.EquipEffect
	currentPointValue := r.livePoints[effect.PointIndex]
	if currentPointValue > (1<<31-1)-effect.PointDelta {
		return PointChangeResult{}, false
	}
	updatedPointValue := currentPointValue + effect.PointDelta
	r.livePoints[effect.PointIndex] = updatedPointValue
	return PointChangeResult{PointType: effect.PointType, PointAmount: effect.PointDelta, PointValue: updatedPointValue}, true
}

func (r *Runtime) RemoveEquipTemplateEffect(template itemcatalog.Template) (PointChangeResult, bool) {
	if r == nil || !itemcatalog.ValidTemplate(template) || template.EquipEffect == nil {
		return PointChangeResult{}, false
	}
	effect := *template.EquipEffect
	currentPointValue := r.livePoints[effect.PointIndex]
	if currentPointValue < (-1<<31)+effect.PointDelta {
		return PointChangeResult{}, false
	}
	updatedPointValue := currentPointValue - effect.PointDelta
	r.livePoints[effect.PointIndex] = updatedPointValue
	return PointChangeResult{PointType: effect.PointType, PointAmount: -effect.PointDelta, PointValue: updatedPointValue}, true
}

func (r *Runtime) UseItem(slot inventory.SlotIndex, template itemcatalog.Template) (ItemUseResult, bool) {
	if r == nil || !itemcatalog.ValidTemplate(template) || template.UseEffect == nil {
		return ItemUseResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return ItemUseResult{}, false
	}
	effect := *template.UseEffect
	item := r.liveInventory[index]
	if item.Equipped || item.Vnum != template.Vnum || item.Count == 0 {
		return ItemUseResult{}, false
	}
	currentPointValue := r.livePoints[effect.PointIndex]
	if currentPointValue > (1<<31-1)-effect.PointDelta {
		return ItemUseResult{}, false
	}
	updatedPointValue := currentPointValue + effect.PointDelta
	result := ItemUseResult{
		Slot:          slot,
		PointType:     effect.PointType,
		PointAmount:   effect.PointDelta,
		PointValue:    updatedPointValue,
		EffectMessage: effect.Message,
	}
	r.livePoints[effect.PointIndex] = updatedPointValue
	if item.Count == 1 {
		r.liveInventory = removeInventoryIndex(r.liveInventory, index)
		sortInventoryItems(r.liveInventory)
		result.ItemRemoved = true
		return result, true
	}
	item.Count--
	if err := item.Validate(); err != nil {
		r.livePoints[effect.PointIndex] = currentPointValue
		return ItemUseResult{}, false
	}
	r.liveInventory[index] = item
	sortInventoryItems(r.liveInventory)
	result.Item = item
	return result, true
}

func (r *Runtime) ValidateMerchantBuy(template itemcatalog.Template, count uint16, price uint64) MerchantBuyFailure {
	if r == nil || !itemcatalog.ValidTemplate(template) || count == 0 || count > template.MaxCount || price == 0 {
		return MerchantBuyFailureInvalid
	}
	if !template.Stackable && count != 1 {
		return MerchantBuyFailureInvalid
	}
	if r.liveGold < price {
		return MerchantBuyFailureInsufficientGold
	}
	if template.Stackable {
		if findMergeableInventoryIndex(r.liveInventory, template.Vnum, count, template.MaxCount) >= 0 {
			return ""
		}
		if _, _, remaining, ok := distributeMerchantGrantAcrossExistingStacks(r.liveInventory, template.Vnum, count, template.MaxCount); ok {
			if remaining == 0 {
				return ""
			}
			if _, ok := nextFreeInventorySlot(r.liveInventory); ok {
				return ""
			}
			return MerchantBuyFailureNoValidPlacement
		}
	}
	if _, ok := nextFreeInventorySlot(r.liveInventory); !ok {
		return MerchantBuyFailureNoValidPlacement
	}
	return ""
}

func (r *Runtime) BuyMerchantItem(template itemcatalog.Template, count uint16, price uint64) (MerchantBuyResult, bool) {
	if failure := r.ValidateMerchantBuy(template, count, price); failure != "" {
		return MerchantBuyResult{}, false
	}
	inventoryItems := cloneItemInstances(r.liveInventory)
	changedItems := make([]inventory.ItemInstance, 0, 2)
	remaining := count
	if template.Stackable {
		if mergeIndex := findMergeableInventoryIndex(inventoryItems, template.Vnum, remaining, template.MaxCount); mergeIndex >= 0 {
			item := inventoryItems[mergeIndex]
			item.Count += remaining
			if err := item.Validate(); err != nil {
				return MerchantBuyResult{}, false
			}
			inventoryItems[mergeIndex] = item
			changedItems = append(changedItems, item)
			remaining = 0
		} else if distributedItems, distributedChanged, distributedRemaining, ok := distributeMerchantGrantAcrossExistingStacks(inventoryItems, template.Vnum, remaining, template.MaxCount); ok {
			inventoryItems = distributedItems
			changedItems = append(changedItems, distributedChanged...)
			remaining = distributedRemaining
		}
	}
	if remaining > 0 {
		slot, ok := nextFreeInventorySlot(inventoryItems)
		if !ok {
			return MerchantBuyResult{}, false
		}
		item, err := (inventory.ItemInstance{ID: nextLiveItemInstanceID(inventoryItems, r.liveEquipment), Vnum: template.Vnum, Count: remaining}).WithInventorySlot(slot)
		if err != nil {
			return MerchantBuyResult{}, false
		}
		inventoryItems = append(inventoryItems, item)
		changedItems = append(changedItems, item)
	}
	sortInventoryItems(inventoryItems)
	sortInventoryItems(changedItems)
	r.liveGold -= price
	r.liveInventory = inventoryItems
	return MerchantBuyResult{Items: changedItems, Gold: r.liveGold}, true
}

func (r *Runtime) SellMerchantItem(slot inventory.SlotIndex, count uint16, unitPrice uint64) (MerchantSellResult, bool) {
	if r == nil || unitPrice == 0 {
		return MerchantSellResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return MerchantSellResult{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Count == 0 {
		return MerchantSellResult{}, false
	}
	soldCount := count
	if soldCount == 0 || soldCount > item.Count {
		soldCount = item.Count
	}
	if soldCount == 0 || unitPrice > (^uint64(0))/uint64(soldCount) {
		return MerchantSellResult{}, false
	}
	credit := unitPrice * uint64(soldCount)
	if r.liveGold > (^uint64(0))-credit {
		return MerchantSellResult{}, false
	}
	result := MerchantSellResult{Slot: slot, Gold: r.liveGold + credit}
	inventoryItems := cloneItemInstances(r.liveInventory)
	if soldCount == item.Count {
		inventoryItems = removeInventoryIndex(inventoryItems, index)
		result.ItemRemoved = true
	} else {
		item.Count -= soldCount
		if err := item.Validate(); err != nil {
			return MerchantSellResult{}, false
		}
		inventoryItems[index] = item
		result.Item = item
	}
	sortInventoryItems(inventoryItems)
	r.liveGold = result.Gold
	r.liveInventory = inventoryItems
	return result, true
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

func (r *Runtime) SetPersistedSnapshot(persisted loginticket.Character) {
	if r == nil {
		return
	}
	r.persisted = normalizeCharacter(persisted)
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

func findMergeableInventoryIndex(items []inventory.ItemInstance, vnum uint32, count uint16, maxCount uint16) int {
	if vnum == 0 || count == 0 || maxCount == 0 || count > maxCount {
		return -1
	}
	mergeIndex := -1
	for i, item := range items {
		if item.Equipped || item.Vnum != vnum || item.Count == 0 {
			continue
		}
		if uint32(item.Count)+uint32(count) > uint32(maxCount) {
			continue
		}
		if mergeIndex < 0 || item.Slot < items[mergeIndex].Slot {
			mergeIndex = i
		}
	}
	return mergeIndex
}

func findPartiallyMergeableInventoryIndices(items []inventory.ItemInstance, vnum uint32, maxCount uint16) []int {
	if vnum == 0 || maxCount == 0 {
		return nil
	}
	indices := make([]int, 0)
	for i, item := range items {
		if item.Equipped || item.Vnum != vnum || item.Count == 0 || item.Count >= maxCount {
			continue
		}
		indices = append(indices, i)
	}
	sort.Slice(indices, func(i, j int) bool {
		return items[indices[i]].Slot < items[indices[j]].Slot
	})
	return indices
}

func distributeMerchantGrantAcrossExistingStacks(items []inventory.ItemInstance, vnum uint32, count uint16, maxCount uint16) ([]inventory.ItemInstance, []inventory.ItemInstance, uint16, bool) {
	if count == 0 {
		return cloneItemInstances(items), nil, 0, false
	}
	indices := findPartiallyMergeableInventoryIndices(items, vnum, maxCount)
	if len(indices) == 0 {
		return cloneItemInstances(items), nil, count, false
	}
	cloned := cloneItemInstances(items)
	changed := make([]inventory.ItemInstance, 0, len(indices))
	remaining := count
	for _, index := range indices {
		if remaining == 0 {
			break
		}
		item := cloned[index]
		room := maxCount - item.Count
		if room == 0 {
			continue
		}
		add := room
		if add > remaining {
			add = remaining
		}
		item.Count += add
		if err := item.Validate(); err != nil {
			return cloneItemInstances(items), nil, count, false
		}
		cloned[index] = item
		changed = append(changed, item)
		remaining -= add
	}
	sortInventoryItems(changed)
	return cloned, changed, remaining, len(changed) > 0
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
