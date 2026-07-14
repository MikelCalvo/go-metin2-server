package player

import (
	"sort"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const (
	quickslotMaxNum         uint8 = 36
	quickslotSkillSlotMax   uint8 = 200
	quickslotCommandSlotMax uint8 = 60
)

type SessionLink struct {
	Login          string
	CharacterIndex uint8
}

type Runtime struct {
	persisted      loginticket.Character
	live           worldruntime.Position
	liveGold       uint64
	livePoints     [255]int32
	liveInventory  []inventory.ItemInstance
	liveEquipment  []inventory.ItemInstance
	liveQuickslots []loginticket.Quickslot
	sessionLink    SessionLink
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

const ExperiencePointIndex uint8 = 3

type DeathRewardResult struct {
	Experience       uint64
	ExperienceBefore int32
	ExperienceAfter  int32
	Gold             uint64
	GoldBefore       uint64
	GoldAfter        uint64
}

type MerchantBuyItemChange struct {
	Item    inventory.ItemInstance
	Created bool
}

type MerchantBuyResult struct {
	Items       []inventory.ItemInstance
	ItemChanges []MerchantBuyItemChange
	Gold        uint64
}

type MerchantSellResult struct {
	Slot        inventory.SlotIndex
	ItemRemoved bool
	Item        inventory.ItemInstance
	GoldBefore  uint64
	Gold        uint64
}

type GroundItemPickupResult struct {
	Item         inventory.ItemInstance
	Merged       bool
	Split        bool
	Updated      inventory.ItemInstance
	UpdatedItems []inventory.ItemInstance
	Placed       inventory.ItemInstance
}

type QuickslotSwapResult struct {
	Position       uint8
	TargetPosition uint8
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
	live.Quickslots = cloneQuickslots(r.liveQuickslots)
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

func (r *Runtime) LiveQuickslots() []loginticket.Quickslot {
	if r == nil {
		return []loginticket.Quickslot{}
	}
	return cloneQuickslots(r.liveQuickslots)
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

func (r *Runtime) AddLiveGold(amount uint64) (uint64, bool) {
	if r == nil || amount == 0 || amount > uint64(1<<31-1) {
		return 0, false
	}
	nextGold := r.liveGold + amount
	if nextGold < r.liveGold || nextGold > uint64(1<<31-1) {
		return 0, false
	}
	r.liveGold = nextGold
	return nextGold, true
}

func (r *Runtime) SetLivePoint(pointIndex uint8, value int32) bool {
	if r == nil || int(pointIndex) >= len(r.livePoints) {
		return false
	}
	r.livePoints[pointIndex] = value
	return true
}

func (r *Runtime) ApplyStaticActorDeathReward(reward worldruntime.StaticActorDeathReward) (DeathRewardResult, bool) {
	if r == nil {
		return DeathRewardResult{}, false
	}
	experienceBefore := r.livePoints[ExperiencePointIndex]
	experienceAfter := experienceBefore
	if reward.Experience != 0 {
		if reward.Experience > uint64(1<<31-1) {
			return DeathRewardResult{}, false
		}
		nextExperience := int64(experienceBefore) + int64(reward.Experience)
		if nextExperience > 1<<31-1 {
			return DeathRewardResult{}, false
		}
		experienceAfter = int32(nextExperience)
	}
	goldBefore := r.liveGold
	goldAfter := goldBefore
	if reward.Gold != 0 {
		if reward.Gold > uint64(1<<31-1) {
			return DeathRewardResult{}, false
		}
		goldAfter = goldBefore + reward.Gold
		if goldAfter < goldBefore || goldAfter > uint64(1<<31-1) {
			return DeathRewardResult{}, false
		}
	}
	r.livePoints[ExperiencePointIndex] = experienceAfter
	r.liveGold = goldAfter
	return DeathRewardResult{
		Experience:       reward.Experience,
		ExperienceBefore: experienceBefore,
		ExperienceAfter:  experienceAfter,
		Gold:             reward.Gold,
		GoldBefore:       goldBefore,
		GoldAfter:        goldAfter,
	}, true
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
	if r == nil || from == to {
		return inventory.MoveResult{}, false
	}
	result := inventory.MoveResult{From: from, To: to}
	return r.moveInventoryItemFullStack(from, to, result)
}

func (r *Runtime) SetQuickslot(position uint8, slot loginticket.Quickslot) (loginticket.Quickslot, bool) {
	if r == nil || !validQuickslotPosition(position) || !validQuickslotTuple(slot) {
		return loginticket.Quickslot{}, false
	}
	if slot.Type == quickslotproto.TypeNone {
		return r.DeleteQuickslot(position)
	}
	if slot.Type == quickslotproto.TypeItem && countInventorySlotOccupancy(r.liveInventory, inventory.SlotIndex(slot.Slot)) != 1 {
		return loginticket.Quickslot{}, false
	}
	updated := cloneQuickslots(r.liveQuickslots)
	for i := 0; i < len(updated); {
		if updated[i].Type == slot.Type && updated[i].Slot == slot.Slot {
			updated = append(updated[:i], updated[i+1:]...)
			continue
		}
		i++
	}
	result := loginticket.Quickslot{Position: position, Type: slot.Type, Slot: slot.Slot}
	if index := findQuickslotPosition(updated, position); index >= 0 {
		updated[index] = result
	} else {
		updated = append(updated, result)
	}
	sortQuickslots(updated)
	r.liveQuickslots = updated
	return result, true
}

func (r *Runtime) DeleteQuickslot(position uint8) (loginticket.Quickslot, bool) {
	if r == nil || !validQuickslotPosition(position) {
		return loginticket.Quickslot{}, false
	}
	updated := cloneQuickslots(r.liveQuickslots)
	index := findQuickslotPosition(updated, position)
	if index < 0 {
		return loginticket.Quickslot{}, false
	}
	updated = append(updated[:index], updated[index+1:]...)
	r.liveQuickslots = updated
	return loginticket.Quickslot{Position: position}, true
}

func (r *Runtime) SwapQuickslots(position uint8, targetPosition uint8) (QuickslotSwapResult, bool) {
	if r == nil || position == targetPosition || !validQuickslotPosition(position) || !validQuickslotPosition(targetPosition) {
		return QuickslotSwapResult{}, false
	}
	updated := cloneQuickslots(r.liveQuickslots)
	leftIndex := findQuickslotPosition(updated, position)
	rightIndex := findQuickslotPosition(updated, targetPosition)
	if leftIndex < 0 && rightIndex < 0 {
		return QuickslotSwapResult{}, false
	}
	if leftIndex >= 0 {
		updated[leftIndex].Position = targetPosition
	}
	if rightIndex >= 0 {
		updated[rightIndex].Position = position
	}
	sortQuickslots(updated)
	r.liveQuickslots = updated
	return QuickslotSwapResult{Position: position, TargetPosition: targetPosition}, true
}

func (r *Runtime) SyncItemQuickslotsForInventoryMove(from inventory.SlotIndex, to inventory.SlotIndex) ([]loginticket.Quickslot, []loginticket.Quickslot, bool) {
	if r == nil || from == to || from >= inventory.CarriedInventorySlotCount || to >= inventory.CarriedInventorySlotCount {
		return nil, nil, false
	}
	updated := cloneQuickslots(r.liveQuickslots)
	changed := make([]loginticket.Quickslot, 0, 1)
	deleted := make([]loginticket.Quickslot, 0, 1)
	for i := 0; i < len(updated); {
		if updated[i].Type != quickslotproto.TypeItem {
			i++
			continue
		}
		slot := inventory.SlotIndex(updated[i].Slot)
		switch slot {
		case from:
			updated[i].Slot = uint8(to)
			changed = append(changed, updated[i])
			i++
		case to:
			deleted = append(deleted, updated[i])
			updated = append(updated[:i], updated[i+1:]...)
		default:
			i++
		}
	}
	if len(changed) == 0 && len(deleted) == 0 {
		return nil, nil, true
	}
	sortQuickslots(updated)
	sortQuickslots(changed)
	sortQuickslots(deleted)
	r.liveQuickslots = updated
	return changed, deleted, true
}

func (r *Runtime) SyncItemQuickslotsForItemRemoval(slot inventory.SlotIndex) ([]loginticket.Quickslot, bool) {
	if r == nil || slot >= inventory.CarriedInventorySlotCount {
		return nil, false
	}
	updated := cloneQuickslots(r.liveQuickslots)
	deleted := make([]loginticket.Quickslot, 0, 1)
	for i := 0; i < len(updated); {
		if updated[i].Type != quickslotproto.TypeItem || inventory.SlotIndex(updated[i].Slot) != slot {
			i++
			continue
		}
		deleted = append(deleted, updated[i])
		updated = append(updated[:i], updated[i+1:]...)
	}
	if len(deleted) == 0 {
		return nil, true
	}
	sortQuickslots(updated)
	sortQuickslots(deleted)
	r.liveQuickslots = updated
	return deleted, true
}

func (r *Runtime) DropInventoryItem(slot inventory.SlotIndex, count uint16) (inventory.MoveResult, bool) {
	return r.dropInventoryItem(slot, count, itemcatalog.Template{})
}

func (r *Runtime) DropInventoryItemWithTemplate(slot inventory.SlotIndex, count uint16, template itemcatalog.Template) (inventory.MoveResult, bool) {
	if !itemcatalog.ValidTemplate(template) || !r.CanUseTemplate(template) {
		return inventory.MoveResult{}, false
	}
	return r.dropInventoryItem(slot, count, template)
}

func (r *Runtime) dropInventoryItem(slot inventory.SlotIndex, count uint16, template itemcatalog.Template) (inventory.MoveResult, bool) {
	if r == nil || count == 0 {
		return inventory.MoveResult{}, false
	}
	result := inventory.MoveResult{From: slot}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return inventory.MoveResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 || r.liveInventory[index].Locked {
		return inventory.MoveResult{}, false
	}
	if template.AntiDrop || template.AntiGive || template.AntiSell {
		return inventory.MoveResult{}, false
	}
	item := r.liveInventory[index]
	if template.Vnum != 0 && item.Vnum != template.Vnum {
		return inventory.MoveResult{}, false
	}
	if template.MaxCount != 0 && item.Count > template.MaxCount {
		return inventory.MoveResult{}, false
	}
	if err := item.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	if count > item.Count {
		return inventory.MoveResult{}, false
	}
	if count == item.Count {
		updatedInventory := cloneItemInstances(r.liveInventory)
		updatedInventory = removeInventoryIndex(updatedInventory, index)
		sortInventoryItems(updatedInventory)
		r.liveInventory = updatedInventory
		result.Changed = true
		return result, true
	}
	updatedInventory := cloneItemInstances(r.liveInventory)
	item = updatedInventory[index]
	item.Count -= count
	if err := item.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	updatedInventory[index] = item
	sortInventoryItems(updatedInventory)
	r.liveInventory = updatedInventory
	result.Changed = true
	result.FromOccupied = true
	result.FromItem = item
	result.CountOnly = true
	return result, true
}

func (r *Runtime) PickupGroundItemWithTemplate(item inventory.ItemInstance, preferred inventory.SlotIndex, template itemcatalog.Template) (GroundItemPickupResult, bool) {
	if !itemcatalog.ValidTemplate(template) || template.EquipSlot != "" || item.Vnum != template.Vnum || item.Count > template.MaxCount || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || !r.CanUseTemplate(template) {
		return GroundItemPickupResult{}, false
	}
	maxCount := uint16(0)
	if template.Stackable && !template.AntiStack {
		maxCount = template.MaxCount
	}
	return r.PickupGroundItem(item, preferred, maxCount)
}

func (r *Runtime) PickupGroundItem(item inventory.ItemInstance, preferred inventory.SlotIndex, maxCount uint16) (GroundItemPickupResult, bool) {
	if r == nil || item.ID == 0 || item.Vnum == 0 || item.Count == 0 || item.Count > 255 || preferred >= inventory.CarriedInventorySlotCount {
		return GroundItemPickupResult{}, false
	}
	if err := item.Validate(); err != nil {
		return GroundItemPickupResult{}, false
	}
	if item.Equipped || item.Locked {
		return GroundItemPickupResult{}, false
	}
	if hasDuplicateInventorySlotOccupancy(r.liveInventory) || hasItemInstanceID(r.liveInventory, item.ID) || hasItemInstanceID(r.liveEquipment, item.ID) {
		return GroundItemPickupResult{}, false
	}
	updatedInventory := cloneItemInstances(r.liveInventory)
	if maxCount > 0 {
		if mergeIndex := findMergeableInventoryIndex(updatedInventory, item.Vnum, item.Count, maxCount); mergeIndex >= 0 {
			merged := updatedInventory[mergeIndex]
			merged.Count += item.Count
			if err := merged.Validate(); err != nil {
				return GroundItemPickupResult{}, false
			}
			updatedInventory[mergeIndex] = merged
			sortInventoryItems(updatedInventory)
			r.liveInventory = updatedInventory
			return GroundItemPickupResult{Item: item, Merged: true, Updated: merged}, true
		}
		if distributedInventory, changes, remaining, ok := distributeMerchantGrantAcrossExistingStacks(updatedInventory, item.Vnum, item.Count, maxCount); ok && len(changes) > 0 {
			updatedInventory = distributedInventory
			if remaining == 0 {
				sortInventoryItems(updatedInventory)
				r.liveInventory = updatedInventory
				return GroundItemPickupResult{Item: item, Split: true, UpdatedItems: clonePickupUpdatedItems(changes)}, true
			}
			placementSlot := preferred
			if inventorySlotOccupied(updatedInventory, placementSlot) {
				var found bool
				placementSlot, found = nextFreeInventorySlot(updatedInventory)
				if !found {
					return GroundItemPickupResult{}, false
				}
			}
			remainder := item
			remainder.Count = remaining
			placed, err := remainder.WithInventorySlot(placementSlot)
			if err != nil {
				return GroundItemPickupResult{}, false
			}
			updatedInventory = append(updatedInventory, placed)
			sortInventoryItems(updatedInventory)
			r.liveInventory = updatedInventory
			return GroundItemPickupResult{Item: item, Split: true, UpdatedItems: clonePickupUpdatedItems(changes), Placed: placed}, true
		}
	}
	placementSlot := preferred
	if inventorySlotOccupied(updatedInventory, placementSlot) {
		var ok bool
		placementSlot, ok = nextFreeInventorySlot(updatedInventory)
		if !ok {
			return GroundItemPickupResult{}, false
		}
	}
	placed, err := item.WithInventorySlot(placementSlot)
	if err != nil {
		return GroundItemPickupResult{}, false
	}
	updatedInventory = append(updatedInventory, placed)
	sortInventoryItems(updatedInventory)
	r.liveInventory = updatedInventory
	return GroundItemPickupResult{Item: item, Placed: placed}, true
}

func clonePickupUpdatedItems(changes []inventory.ItemInstance) []inventory.ItemInstance {
	items := cloneItemInstances(changes)
	sortInventoryItems(items)
	return items
}

func (r *Runtime) MoveInventoryItemBounded(from inventory.SlotIndex, to inventory.SlotIndex, maxCount uint16) (inventory.MoveResult, bool) {
	if r == nil || from == to {
		return inventory.MoveResult{}, false
	}
	if maxCount == 0 {
		if !canForceSameVnumSwap(r.liveInventory, from, to) {
			return inventory.MoveResult{}, false
		}
		return r.moveInventoryItemFullStack(from, to, inventory.MoveResult{From: from, To: to, CompatibleSwap: true, ForcedSwap: true})
	}
	if countInventorySlotOccupancy(r.liveInventory, from) != 1 || countInventorySlotOccupancy(r.liveInventory, to) > 1 {
		return inventory.MoveResult{}, false
	}
	result := inventory.MoveResult{From: from, To: to}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Locked {
		return inventory.MoveResult{}, false
	}
	sourceItem := r.liveInventory[fromIndex]
	if sourceItem.Count == 0 || sourceItem.Count > maxCount {
		return inventory.MoveResult{}, false
	}
	toIndex := findInventorySlot(r.liveInventory, to)
	if toIndex < 0 {
		return r.moveInventoryItemFullStack(from, to, result)
	}
	destinationItem := r.liveInventory[toIndex]
	if destinationItem.Locked || destinationItem.ID == sourceItem.ID || destinationItem.Count == 0 {
		return inventory.MoveResult{}, false
	}
	if destinationItem.Vnum != sourceItem.Vnum {
		return r.moveInventoryItemFullStack(from, to, result)
	}
	if destinationItem.Count >= maxCount {
		return inventory.MoveResult{}, false
	}
	mergeCount := sourceItem.Count
	available := maxCount - destinationItem.Count
	if mergeCount > available {
		mergeCount = available
	}
	if mergeCount == 0 {
		return inventory.MoveResult{}, false
	}
	destinationItem.Count += mergeCount
	if err := destinationItem.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	if mergeCount == sourceItem.Count {
		r.liveInventory = removeInventoryIndex(r.liveInventory, fromIndex)
		toIndex = findInventorySlot(r.liveInventory, to)
		if toIndex < 0 {
			return inventory.MoveResult{}, false
		}
		r.liveInventory[toIndex] = destinationItem
		sortInventoryItems(r.liveInventory)
		result.Changed = true
		result.ToOccupied = true
		result.ToItem = destinationItem
		result.CountOnly = true
		return result, true
	}
	sourceItem.Count -= mergeCount
	if err := sourceItem.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	r.liveInventory[fromIndex] = sourceItem
	r.liveInventory[toIndex] = destinationItem
	sortInventoryItems(r.liveInventory)
	result.Changed = true
	result.FromOccupied = true
	result.FromItem = sourceItem
	result.ToOccupied = true
	result.ToItem = destinationItem
	result.CountOnly = true
	return result, true
}

func (r *Runtime) MoveInventoryItemCount(from inventory.SlotIndex, to inventory.SlotIndex, count uint16) (inventory.MoveResult, bool) {
	return r.MoveInventoryItemCountBounded(from, to, count, ^uint16(0))
}

func (r *Runtime) MoveInventoryItemCountBounded(from inventory.SlotIndex, to inventory.SlotIndex, count uint16, maxCount uint16) (inventory.MoveResult, bool) {
	if r == nil || count == 0 {
		return inventory.MoveResult{}, false
	}
	if from == to {
		return inventory.MoveResult{}, false
	}
	if maxCount == 0 {
		fromIndex := findInventorySlot(r.liveInventory, from)
		if fromIndex < 0 || count < r.liveInventory[fromIndex].Count || !canForceSameVnumSwap(r.liveInventory, from, to) {
			return inventory.MoveResult{}, false
		}
		return r.moveInventoryItemFullStack(from, to, inventory.MoveResult{From: from, To: to, CompatibleSwap: true, ForcedSwap: true})
	}
	if count > maxCount {
		return inventory.MoveResult{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, from) != 1 || countInventorySlotOccupancy(r.liveInventory, to) > 1 {
		return inventory.MoveResult{}, false
	}
	result := inventory.MoveResult{From: from, To: to}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Locked {
		return inventory.MoveResult{}, false
	}
	sourceItem := r.liveInventory[fromIndex]
	if count > sourceItem.Count {
		return inventory.MoveResult{}, false
	}
	if sourceItem.Count > maxCount {
		return inventory.MoveResult{}, false
	}
	toIndex := findInventorySlot(r.liveInventory, to)
	sourceRemainder := sourceItem
	if count == sourceItem.Count {
		if toIndex >= 0 {
			destinationItem := r.liveInventory[toIndex]
			if destinationItem.Locked || destinationItem.ID == sourceItem.ID {
				return inventory.MoveResult{}, false
			}
			if destinationItem.Vnum != sourceItem.Vnum {
				return r.moveInventoryItemFullStack(from, to, result)
			}
			mergedCount := uint32(destinationItem.Count) + uint32(count)
			if mergedCount > uint32(maxCount) {
				return inventory.MoveResult{}, false
			}
			destinationItem.Count = uint16(mergedCount)
			if err := destinationItem.Validate(); err != nil {
				return inventory.MoveResult{}, false
			}
			r.liveInventory = removeInventoryIndex(r.liveInventory, fromIndex)
			toIndex = findInventorySlot(r.liveInventory, to)
			if toIndex < 0 {
				return inventory.MoveResult{}, false
			}
			r.liveInventory[toIndex] = destinationItem
			sortInventoryItems(r.liveInventory)
			result.Changed = true
			result.ToOccupied = true
			result.ToItem = destinationItem
			result.CountOnly = true
			return result, true
		}
		return r.moveInventoryItemFullStack(from, to, result)
	}
	sourceRemainder.Count -= count
	if toIndex >= 0 {
		destinationItem := r.liveInventory[toIndex]
		if destinationItem.Locked || destinationItem.ID == sourceItem.ID || destinationItem.Vnum != sourceItem.Vnum || destinationItem.Count == 0 {
			return inventory.MoveResult{}, false
		}
		mergedCount := uint32(destinationItem.Count) + uint32(count)
		if mergedCount > uint32(maxCount) {
			return inventory.MoveResult{}, false
		}
		destinationItem.Count = uint16(mergedCount)
		if err := sourceRemainder.Validate(); err != nil {
			return inventory.MoveResult{}, false
		}
		if err := destinationItem.Validate(); err != nil {
			return inventory.MoveResult{}, false
		}
		r.liveInventory[fromIndex] = sourceRemainder
		r.liveInventory[toIndex] = destinationItem
		sortInventoryItems(r.liveInventory)
		result.Changed = true
		result.FromOccupied = true
		result.FromItem = sourceRemainder
		result.ToOccupied = true
		result.ToItem = destinationItem
		result.CountOnly = true
		return result, true
	}
	destinationItem := sourceItem
	destinationItem.ID = r.nextSplitItemID()
	destinationItem.Count = count
	var err error
	destinationItem, err = destinationItem.WithInventorySlot(to)
	if err != nil {
		return inventory.MoveResult{}, false
	}
	if err := sourceRemainder.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	r.liveInventory[fromIndex] = sourceRemainder
	r.liveInventory = append(r.liveInventory, destinationItem)
	sortInventoryItems(r.liveInventory)
	result.Changed = true
	result.FromOccupied = true
	result.FromItem = sourceRemainder
	result.ToOccupied = true
	result.ToItem = destinationItem
	return result, true
}

func (r *Runtime) moveInventoryItemFullStack(from inventory.SlotIndex, to inventory.SlotIndex, result inventory.MoveResult) (inventory.MoveResult, bool) {
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Locked {
		return inventory.MoveResult{}, false
	}
	movedItem, err := r.liveInventory[fromIndex].WithInventorySlot(to)
	if err != nil {
		return inventory.MoveResult{}, false
	}
	toIndex := findInventorySlot(r.liveInventory, to)
	if toIndex >= 0 {
		destinationItem := r.liveInventory[toIndex]
		if destinationItem.Locked {
			return inventory.MoveResult{}, false
		}
		sourceItem, err := destinationItem.WithInventorySlot(from)
		if err != nil {
			return inventory.MoveResult{}, false
		}
		r.liveInventory[fromIndex] = movedItem
		r.liveInventory[toIndex] = sourceItem
		sortInventoryItems(r.liveInventory)
		result.Changed = true
		result.FromOccupied = true
		result.FromItem = sourceItem
		result.ToOccupied = true
		result.ToItem = movedItem
		return result, true
	}
	r.liveInventory[fromIndex] = movedItem
	sortInventoryItems(r.liveInventory)
	result.Changed = true
	result.ToOccupied = true
	result.ToItem = movedItem
	return result, true
}

func (r *Runtime) EquipItem(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot) (inventory.ItemInstance, bool) {
	return r.equipItem(from, equipSlot)
}

func (r *Runtime) EquipItemWithTemplate(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot, template itemcatalog.Template) (inventory.ItemInstance, bool) {
	if !templateAuthoredForEquipSlot(template, equipSlot) || !r.CanUseTemplate(template) || template.AntiStack || template.AntiDrop || template.AntiGive || template.AntiSell {
		return inventory.ItemInstance{}, false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Vnum != template.Vnum {
		return inventory.ItemInstance{}, false
	}
	return r.equipItem(from, equipSlot)
}

func (r *Runtime) equipItem(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot) (inventory.ItemInstance, bool) {
	if r == nil || !equipSlot.Valid() || equipmentSlotOccupied(r.liveEquipment, equipSlot) {
		return inventory.ItemInstance{}, false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Locked {
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
	if equipIndex < 0 || r.liveEquipment[equipIndex].Locked {
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

func (r *Runtime) ApplyEquipTemplateEffect(template itemcatalog.Template, equipSlot inventory.EquipmentSlot) (PointChangeResult, bool) {
	if r == nil || !templateAuthoredForEquipSlot(template, equipSlot) || template.EquipEffect == nil {
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

func (r *Runtime) RemoveEquipTemplateEffect(template itemcatalog.Template, equipSlot inventory.EquipmentSlot) (PointChangeResult, bool) {
	if r == nil || !templateAuthoredForEquipSlot(template, equipSlot) || template.EquipEffect == nil {
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

func templateAuthoredForEquipSlot(template itemcatalog.Template, equipSlot inventory.EquipmentSlot) bool {
	if !equipSlot.Valid() || !itemcatalog.ValidTemplate(template) || template.EquipSlot == "" {
		return false
	}
	templateSlot, ok := inventory.ParseEquipmentSlot(template.EquipSlot)
	return ok && templateSlot == equipSlot
}

func (r *Runtime) CanUseTemplate(template itemcatalog.Template) bool {
	if r == nil || !itemcatalog.ValidTemplate(template) {
		return false
	}
	if r.persisted.Job == 0 && template.AntiWarrior {
		return false
	}
	if r.persisted.Job == 1 && template.AntiAssassin {
		return false
	}
	if r.persisted.Job == 2 && template.AntiSura {
		return false
	}
	if r.persisted.Job == 3 && template.AntiShaman {
		return false
	}
	if r.persisted.RaceNum%2 == 0 && template.AntiMale {
		return false
	}
	if r.persisted.RaceNum%2 == 1 && template.AntiFemale {
		return false
	}
	if r.persisted.Empire == 1 && template.AntiEmpireA {
		return false
	}
	if r.persisted.Empire == 2 && template.AntiEmpireB {
		return false
	}
	if r.persisted.Empire == 3 && template.AntiEmpireC {
		return false
	}
	if template.MinLevel != 0 && r.persisted.Level < template.MinLevel {
		return false
	}
	return true
}

func (r *Runtime) UseItem(slot inventory.SlotIndex, template itemcatalog.Template) (ItemUseResult, bool) {
	if r == nil || slot >= inventory.CarriedInventorySlotCount || !r.CanUseTemplate(template) || template.EquipSlot != "" || template.UseEffect == nil || template.AntiStack || template.AntiDrop || template.AntiGive || template.AntiSell {
		return ItemUseResult{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return ItemUseResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return ItemUseResult{}, false
	}
	effect := *template.UseEffect
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return ItemUseResult{}, false
	}
	if err := item.Validate(); err != nil {
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

func (r *Runtime) UseItemOnItem(source inventory.SlotIndex, target inventory.SlotIndex, template itemcatalog.Template) (inventory.MoveResult, bool) {
	return r.useItemOnItem(source, target, template, nil)
}

func (r *Runtime) useItemOnItem(source inventory.SlotIndex, target inventory.SlotIndex, template itemcatalog.Template, rewriteItem func(inventory.ItemInstance) inventory.ItemInstance) (inventory.MoveResult, bool) {
	if r == nil || source == target || source >= inventory.CarriedInventorySlotCount || target >= inventory.CarriedInventorySlotCount || !r.CanUseTemplate(template) || !template.Stackable || template.EquipSlot != "" || template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.MaxCount == 0 || template.MaxCount > 255 {
		return inventory.MoveResult{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, source) != 1 || countInventorySlotOccupancy(r.liveInventory, target) != 1 {
		return inventory.MoveResult{}, false
	}
	sourceIndex := findInventorySlot(r.liveInventory, source)
	if sourceIndex < 0 {
		return inventory.MoveResult{}, false
	}
	sourceItem := r.liveInventory[sourceIndex]
	if sourceItem.Equipped || sourceItem.Locked || sourceItem.Vnum != template.Vnum || sourceItem.Count == 0 || sourceItem.Count > template.MaxCount {
		return inventory.MoveResult{}, false
	}
	targetIndex := findInventorySlot(r.liveInventory, target)
	if targetIndex < 0 {
		return inventory.MoveResult{}, false
	}
	targetItem := r.liveInventory[targetIndex]
	if targetItem.Equipped || targetItem.Locked || targetItem.ID == sourceItem.ID || targetItem.Vnum != sourceItem.Vnum || targetItem.Count == 0 || targetItem.Count >= template.MaxCount {
		return inventory.MoveResult{}, false
	}
	mergeCount := sourceItem.Count
	available := template.MaxCount - targetItem.Count
	if mergeCount > available {
		mergeCount = available
	}
	if mergeCount == 0 {
		return inventory.MoveResult{}, false
	}

	updatedInventory := cloneItemInstances(r.liveInventory)
	updatedSourceIndex := findInventorySlot(updatedInventory, source)
	updatedTargetIndex := findInventorySlot(updatedInventory, target)
	if updatedSourceIndex < 0 || updatedTargetIndex < 0 {
		return inventory.MoveResult{}, false
	}
	sourceItem = updatedInventory[updatedSourceIndex]
	targetItem = updatedInventory[updatedTargetIndex]
	targetItem.Count += mergeCount
	targetItem = applyItemRewriteHook(targetItem, rewriteItem)
	if err := targetItem.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	result := inventory.MoveResult{Changed: true, From: source, To: target, ToOccupied: true, ToItem: targetItem}
	if mergeCount == sourceItem.Count {
		sourceItem = applyItemRewriteHook(sourceItem, rewriteItem)
		if err := sourceItem.Validate(); err != nil {
			return inventory.MoveResult{}, false
		}
		updatedInventory = removeInventoryIndex(updatedInventory, updatedSourceIndex)
		updatedTargetIndex = findInventorySlot(updatedInventory, target)
		if updatedTargetIndex < 0 {
			return inventory.MoveResult{}, false
		}
		updatedInventory[updatedTargetIndex] = targetItem
		sortInventoryItems(updatedInventory)
		r.liveInventory = updatedInventory
		return result, true
	}
	sourceItem.Count -= mergeCount
	sourceItem = applyItemRewriteHook(sourceItem, rewriteItem)
	if err := sourceItem.Validate(); err != nil {
		return inventory.MoveResult{}, false
	}
	updatedInventory[updatedSourceIndex] = sourceItem
	updatedInventory[updatedTargetIndex] = targetItem
	sortInventoryItems(updatedInventory)
	r.liveInventory = updatedInventory
	result.FromOccupied = true
	result.FromItem = sourceItem
	result.CountOnly = true
	return result, true
}

func applyItemRewriteHook(item inventory.ItemInstance, rewriteItem func(inventory.ItemInstance) inventory.ItemInstance) inventory.ItemInstance {
	if rewriteItem != nil {
		item = rewriteItem(item)
	}
	return item
}

func (r *Runtime) ValidateMerchantBuy(template itemcatalog.Template, count uint16, price uint64) MerchantBuyFailure {
	if r == nil || !r.CanUseTemplate(template) || count == 0 || count > template.MaxCount || price == 0 {
		return MerchantBuyFailureInvalid
	}
	if !template.Stackable && count != 1 {
		return MerchantBuyFailureInvalid
	}
	if r.liveGold < price {
		return MerchantBuyFailureInsufficientGold
	}
	if template.Stackable && !template.AntiStack {
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
	changedExistingSlots := map[inventory.SlotIndex]bool{}
	remaining := count
	if template.Stackable && !template.AntiStack {
		if mergeIndex := findMergeableInventoryIndex(inventoryItems, template.Vnum, remaining, template.MaxCount); mergeIndex >= 0 {
			item := inventoryItems[mergeIndex]
			item.Count += remaining
			if err := item.Validate(); err != nil {
				return MerchantBuyResult{}, false
			}
			inventoryItems[mergeIndex] = item
			changedItems = append(changedItems, item)
			changedExistingSlots[item.Slot] = true
			remaining = 0
		} else if distributedItems, distributedChanged, distributedRemaining, ok := distributeMerchantGrantAcrossExistingStacks(inventoryItems, template.Vnum, remaining, template.MaxCount); ok {
			inventoryItems = distributedItems
			changedItems = append(changedItems, distributedChanged...)
			for _, item := range distributedChanged {
				changedExistingSlots[item.Slot] = true
			}
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
	itemChanges := make([]MerchantBuyItemChange, 0, len(changedItems))
	for _, item := range changedItems {
		itemChanges = append(itemChanges, MerchantBuyItemChange{Item: item, Created: !changedExistingSlots[item.Slot]})
	}
	r.liveGold -= price
	r.liveInventory = inventoryItems
	return MerchantBuyResult{Items: changedItems, ItemChanges: itemChanges, Gold: r.liveGold}, true
}

func (r *Runtime) SellMerchantItem(slot inventory.SlotIndex, count uint16, unitPrice uint64) (MerchantSellResult, bool) {
	if r == nil || unitPrice == 0 {
		return MerchantSellResult{}, false
	}
	soldCount, ok := r.MerchantSellCount(slot, count)
	if !ok || unitPrice > (^uint64(0))/uint64(soldCount) {
		return MerchantSellResult{}, false
	}
	return r.SellMerchantItemForCredit(slot, count, unitPrice*uint64(soldCount))
}

func (r *Runtime) SellMerchantItemWithTemplate(slot inventory.SlotIndex, count uint16, template itemcatalog.Template) (MerchantSellResult, bool) {
	if r == nil || !r.CanUseTemplate(template) || template.AntiSell {
		return MerchantSellResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 || r.liveInventory[index].Vnum != template.Vnum {
		return MerchantSellResult{}, false
	}
	soldCount, ok := r.MerchantSellCount(slot, count)
	if !ok {
		return MerchantSellResult{}, false
	}
	credit, ok := MerchantSellCredit(template, soldCount)
	if !ok {
		return MerchantSellResult{}, false
	}
	return r.SellMerchantItemForCredit(slot, soldCount, credit)
}

func (r *Runtime) SellMerchantItemWithTemplateCounted(slot inventory.SlotIndex, count uint16, template itemcatalog.Template) (MerchantSellResult, bool) {
	if count == 0 {
		return MerchantSellResult{}, false
	}
	return r.SellMerchantItemWithTemplate(slot, count, template)
}

func (r *Runtime) MerchantSellCount(slot inventory.SlotIndex, count uint16) (uint16, bool) {
	if r == nil || countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return 0, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return 0, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Count == 0 {
		return 0, false
	}
	soldCount := count
	if soldCount == 0 {
		soldCount = item.Count
	}
	if soldCount == 0 || soldCount > item.Count {
		return 0, false
	}
	return soldCount, true
}

func (r *Runtime) SellMerchantItemForCredit(slot inventory.SlotIndex, count uint16, credit uint64) (MerchantSellResult, bool) {
	if r == nil || credit == 0 || countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return MerchantSellResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return MerchantSellResult{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Count == 0 {
		return MerchantSellResult{}, false
	}
	soldCount := count
	if soldCount == 0 {
		soldCount = item.Count
	}
	if soldCount == 0 || soldCount > item.Count {
		return MerchantSellResult{}, false
	}
	if r.liveGold > (^uint64(0))-credit {
		return MerchantSellResult{}, false
	}
	result := MerchantSellResult{Slot: slot, GoldBefore: r.liveGold, Gold: r.liveGold + credit}
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

func MerchantSellCredit(template itemcatalog.Template, count uint16) (uint64, bool) {
	if !itemcatalog.ValidTemplate(template) || template.AntiSell || count == 0 {
		return 0, false
	}
	var price uint64
	if template.SellCountPerGold {
		if template.ShopBuyPrice == 0 {
			price = uint64(count)
		} else {
			price = uint64(count) / template.ShopBuyPrice
		}
	} else {
		if template.ShopBuyPrice == 0 || template.ShopBuyPrice > (^uint64(0))/uint64(count) {
			return 0, false
		}
		price = template.ShopBuyPrice * uint64(count)
	}
	price /= 5
	tax := price * 3 / 100
	price -= tax
	if price == 0 {
		return 0, false
	}
	return price, true
}

func MerchantSellUnitPrice(template itemcatalog.Template) (uint64, bool) {
	return MerchantSellCredit(template, 1)
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
	r.liveQuickslots = cloneQuickslots(r.persisted.Quickslots)
	sortInventoryItems(r.liveInventory)
	sortEquipmentItems(r.liveEquipment)
	sortQuickslots(r.liveQuickslots)
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

func cloneQuickslots(quickslots []loginticket.Quickslot) []loginticket.Quickslot {
	if quickslots == nil {
		return []loginticket.Quickslot{}
	}
	return append([]loginticket.Quickslot(nil), quickslots...)
}

func (r *Runtime) nextSplitItemID() uint64 {
	var maxID uint64
	for _, item := range r.liveInventory {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	for _, item := range r.liveEquipment {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	if maxID == ^uint64(0) {
		return 0
	}
	return maxID + 1
}

func findInventorySlot(items []inventory.ItemInstance, slot inventory.SlotIndex) int {
	for index, item := range items {
		if !item.Equipped && item.Slot == slot {
			return index
		}
	}
	return -1
}

func countInventorySlotOccupancy(items []inventory.ItemInstance, slot inventory.SlotIndex) int {
	count := 0
	for _, item := range items {
		if !item.Equipped && item.Slot == slot {
			count++
		}
	}
	return count
}

func canForceSameVnumSwap(items []inventory.ItemInstance, from inventory.SlotIndex, to inventory.SlotIndex) bool {
	if countInventorySlotOccupancy(items, from) != 1 || countInventorySlotOccupancy(items, to) != 1 {
		return false
	}
	fromIndex := findInventorySlot(items, from)
	toIndex := findInventorySlot(items, to)
	if fromIndex < 0 || toIndex < 0 {
		return false
	}
	fromItem := items[fromIndex]
	toItem := items[toIndex]
	return !fromItem.Locked && !toItem.Locked && fromItem.ID != toItem.ID && fromItem.Vnum == toItem.Vnum && fromItem.Count > 0 && toItem.Count > 0
}

func hasDuplicateInventorySlotOccupancy(items []inventory.ItemInstance) bool {
	seen := make(map[inventory.SlotIndex]bool, len(items))
	for _, item := range items {
		if item.Equipped {
			continue
		}
		if seen[item.Slot] {
			return true
		}
		seen[item.Slot] = true
	}
	return false
}

func hasItemInstanceID(items []inventory.ItemInstance, id uint64) bool {
	if id == 0 {
		return false
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
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
		if item.Equipped || item.Locked || item.Vnum != vnum || item.Count == 0 {
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
		if item.Equipped || item.Locked || item.Vnum != vnum || item.Count == 0 || item.Count >= maxCount {
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

func sortQuickslots(quickslots []loginticket.Quickslot) {
	sort.Slice(quickslots, func(i int, j int) bool {
		return quickslots[i].Position < quickslots[j].Position
	})
}

func findQuickslotPosition(quickslots []loginticket.Quickslot, position uint8) int {
	for index, quickslot := range quickslots {
		if quickslot.Position == position {
			return index
		}
	}
	return -1
}

func validQuickslotPosition(position uint8) bool {
	return position < quickslotMaxNum
}

func validQuickslotTuple(slot loginticket.Quickslot) bool {
	switch slot.Type {
	case quickslotproto.TypeNone:
		return true
	case quickslotproto.TypeItem:
		return slot.Slot < uint8(inventory.CarriedInventorySlotCount)
	case quickslotproto.TypeSkill:
		return slot.Slot < quickslotSkillSlotMax
	case quickslotproto.TypeCommand:
		return slot.Slot < quickslotCommandSlotMax
	default:
		return false
	}
}

func equipmentSlotOrderIndex() map[inventory.EquipmentSlot]int {
	order := make(map[inventory.EquipmentSlot]int, len(inventory.AllEquipmentSlots()))
	for idx, slot := range inventory.AllEquipmentSlots() {
		order[slot] = idx
	}
	return order
}
