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
	Slot              inventory.SlotIndex
	ItemRemoved       bool
	Item              inventory.ItemInstance
	Vnum              uint32
	PointType         uint8
	PointAmount       int32
	PointValue        int32
	EffectMessage     string
	SpecialEffectType uint8
}

type PointChangeResult struct {
	PointType   uint8
	PointAmount int32
	PointValue  int32
}

// EquipReplaceResult captures an atomic occupied-wear swap: the worn item lands
// on the carried source cell and the source wearable lands on the wear cell.
type EquipReplaceResult struct {
	UnequippedItem inventory.ItemInstance
	EquippedItem   inventory.ItemInstance
	RemovedEffect  *PointChangeResult
	AppliedEffect  *PointChangeResult
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

type CarriedItemGrantFailure string

const (
	CarriedItemGrantFailureInvalid          CarriedItemGrantFailure = "invalid"
	CarriedItemGrantFailureNoValidPlacement CarriedItemGrantFailure = "no_valid_placement"
)

type CarriedItemGrantResult struct {
	Items       []inventory.ItemInstance
	ItemChanges []MerchantBuyItemChange
}

type MerchantSellResult struct {
	Slot        inventory.SlotIndex
	ItemRemoved bool
	Item        inventory.ItemInstance
	GoldBefore  uint64
	Gold        uint64
}

type ExchangeItemAddDisplay struct {
	Item       inventory.ItemInstance
	Sockets    itemcatalog.SocketValues
	Attributes itemcatalog.AttributeValues
}

// SafeboxCheckinItemResult is the live inventory removal produced by an accepted
// bootstrap safebox check-in. Safebox slot placement itself stays session-local.
type SafeboxCheckinItemResult struct {
	Slot inventory.SlotIndex
	Item inventory.ItemInstance
}

// SafeboxCheckoutItemResult is the live carried placement produced by an accepted
// bootstrap safebox check-out. Safebox slot removal itself stays session-local.
type SafeboxCheckoutItemResult struct {
	Destination inventory.SlotIndex
	Merged      bool
	Item        inventory.ItemInstance
}

type RefineInformation struct {
	Type        uint8
	Position    inventory.SlotIndex
	SourceVnum  uint32
	ResultVnum  uint32
	Cost        int32
	Probability int32
	Materials   []itemcatalog.RefineMaterial
}

type RefineMaterialChange struct {
	Slot        inventory.SlotIndex
	ItemRemoved bool
	Item        inventory.ItemInstance
}

type RefineSuccessResult struct {
	SourceSlot      inventory.SlotIndex
	ResultItem      inventory.ItemInstance
	MaterialChanges []RefineMaterialChange
	GoldBefore      uint64
	Gold            uint64
	Cost            int32
}

// RefineDestroyFailureResult is the live mutation outcome for remembered
// probability = 0 confirm-after-preview dialogs: gold/materials consumed and
// the source carried item destroyed without placing a result vnum.
type RefineDestroyFailureResult struct {
	SourceSlot      inventory.SlotIndex
	MaterialChanges []RefineMaterialChange
	GoldBefore      uint64
	Gold            uint64
	Cost            int32
}

// RefineKeepFailureResult is the live mutation outcome for remembered
// probability values in 1..99 with template-authored keep_on_fail when the
// injected roll fails: gold/materials consumed, source carried item kept.
type RefineKeepFailureResult struct {
	SourceSlot      inventory.SlotIndex
	MaterialChanges []RefineMaterialChange
	GoldBefore      uint64
	Gold            uint64
	Cost            int32
}

// RefineWithRollResult is the live mutation outcome for remembered
// probability values in 1..99 when confirm supplies one injected roll in
// 1..100. Exactly one of Succeeded, Destroyed, or Kept is set on acceptance.
type RefineWithRollResult struct {
	Succeeded bool
	Destroyed bool
	Kept      bool
	Success   RefineSuccessResult
	Destroy   RefineDestroyFailureResult
	Keep      RefineKeepFailureResult
}

// CarriedItemConsumeRequirement is one by-vnum carried-inventory debit request.
type CarriedItemConsumeRequirement struct {
	ItemVnum uint32
	Count    uint16
}

type CarriedItemConsumeChange struct {
	Slot        inventory.SlotIndex
	ItemRemoved bool
	Item        inventory.ItemInstance
}

type CarriedItemConsumeResult struct {
	Changes []CarriedItemConsumeChange
}

type CarriedItemConsumeFailure string

const (
	CarriedItemConsumeFailureInvalid               CarriedItemConsumeFailure = "invalid"
	CarriedItemConsumeFailureInsufficientMaterials CarriedItemConsumeFailure = "insufficient_materials"
)

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

// EquipmentSlotOccupied reports whether the live equipment slice already has
// any occupant in the addressed wear cell. Packet / slash equip uses this to
// emit occupied-wear reject chat before the empty-slot mutation path runs.
func (r *Runtime) EquipmentSlotOccupied(slot inventory.EquipmentSlot) bool {
	if r == nil || !slot.Valid() {
		return false
	}
	return equipmentSlotOccupied(r.liveEquipment, slot)
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

// DeductLiveGold removes amount from live gold when the carrier and balance allow it.
// amount == 0 is treated as a no-op success that returns the current balance.
func (r *Runtime) DeductLiveGold(amount uint64) (uint64, bool) {
	if r == nil {
		return 0, false
	}
	if amount == 0 {
		return r.liveGold, true
	}
	if amount > uint64(1<<31-1) || amount > r.liveGold || r.liveGold > uint64(1<<31-1) {
		return 0, false
	}
	r.liveGold -= amount
	return r.liveGold, true
}

// DeductLiveExperience removes amount from the live experience point when the
// carrier and balance allow it. amount == 0 is treated as a no-op success that
// returns the current experience value.
func (r *Runtime) DeductLiveExperience(amount uint64) (int32, bool) {
	if r == nil {
		return 0, false
	}
	current := r.livePoints[ExperiencePointIndex]
	if amount == 0 {
		return current, true
	}
	if amount > uint64(1<<31-1) || current < 0 || uint64(current) < amount {
		return 0, false
	}
	updated := current - int32(amount)
	r.livePoints[ExperiencePointIndex] = updated
	return updated, true
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
	if slot.Type == quickslotproto.TypeItem {
		itemSlot := inventory.SlotIndex(slot.Slot)
		if countInventorySlotOccupancy(r.liveInventory, itemSlot) != 1 {
			return loginticket.Quickslot{}, false
		}
		itemIndex := findInventorySlot(r.liveInventory, itemSlot)
		if itemIndex < 0 || r.liveInventory[itemIndex].Locked {
			return loginticket.Quickslot{}, false
		}
		if err := r.liveInventory[itemIndex].Validate(); err != nil {
			return loginticket.Quickslot{}, false
		}
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

func (r *Runtime) SetQuickslotWithTemplate(position uint8, slot loginticket.Quickslot, template itemcatalog.Template) (loginticket.Quickslot, bool) {
	if slot.Type != quickslotproto.TypeItem {
		return r.SetQuickslot(position, slot)
	}
	if r == nil || !itemcatalog.ValidTemplate(template) || template.Vnum == 0 || !validQuickslotPosition(position) || !validQuickslotTuple(slot) {
		return loginticket.Quickslot{}, false
	}
	itemSlot := inventory.SlotIndex(slot.Slot)
	if countInventorySlotOccupancy(r.liveInventory, itemSlot) != 1 {
		return loginticket.Quickslot{}, false
	}
	itemIndex := findInventorySlot(r.liveInventory, itemSlot)
	if itemIndex < 0 {
		return loginticket.Quickslot{}, false
	}
	item := r.liveInventory[itemIndex]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return loginticket.Quickslot{}, false
	}
	if err := item.Validate(); err != nil {
		return loginticket.Quickslot{}, false
	}
	return r.SetQuickslot(position, slot)
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
	if !itemcatalog.ValidTemplate(template) || !r.CanUseTemplate(template) || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack {
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
	if !itemcatalog.ValidTemplate(template) || item.Vnum != template.Vnum || item.Count > template.MaxCount || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack || !r.CanUseTemplate(template) {
		return GroundItemPickupResult{}, false
	}
	maxCount := uint16(0)
	if template.Stackable {
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
	if !templateAuthoredForEquipSlot(template, equipSlot) || !r.CanUseTemplate(template) || template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell {
		return inventory.ItemInstance{}, false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Vnum != template.Vnum || r.liveInventory[fromIndex].Count == 0 || r.liveInventory[fromIndex].Count > template.MaxCount {
		return inventory.ItemInstance{}, false
	}
	return r.equipItem(from, equipSlot)
}

// ReplaceOccupiedEquipItem swaps a carried wearable onto an already-occupied
// wear cell when that cell has exactly one unlocked occupant and the source
// cell can accept the worn item. Point effects are left to the caller.
func (r *Runtime) ReplaceOccupiedEquipItem(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot) (EquipReplaceResult, bool) {
	return r.replaceOccupiedEquipItem(from, equipSlot)
}

// CanReplaceOccupiedEquipItem reports whether an occupied-wear swap can place
// the worn item onto the carried source cell without mutating live state.
// Packet / slash equip uses this to keep the occupied reject chat for
// non-swappable destinations while effect overflow stays silent fail-closed.
func (r *Runtime) CanReplaceOccupiedEquipItem(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot) bool {
	if r == nil || !equipSlot.Valid() || countEquipmentSlotOccupancy(r.liveEquipment, equipSlot) != 1 {
		return false
	}
	if countInventorySlotOccupancy(r.liveInventory, from) != 1 {
		return false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	equipIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if fromIndex < 0 || equipIndex < 0 {
		return false
	}
	sourceItem := r.liveInventory[fromIndex]
	wornItem := r.liveEquipment[equipIndex]
	if sourceItem.Locked || wornItem.Locked {
		return false
	}
	unequippedItem, err := wornItem.WithInventorySlot(from)
	if err != nil {
		return false
	}
	equippedItem := sourceItem
	equippedItem.Slot = 0
	equippedItem.Equipped = true
	equippedItem.EquipSlot = equipSlot
	if err := equippedItem.Validate(); err != nil {
		return false
	}
	return unequippedItem.Validate() == nil
}

// ReplaceOccupiedEquipItemWithTemplates performs the occupied-wear swap and,
// when templates author equip_effect metadata, inverts the previous effect
// then applies the new one. Effect carrier overflow/underflow rolls the whole
// mutation back fail-closed.
func (r *Runtime) ReplaceOccupiedEquipItemWithTemplates(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot, newTemplate itemcatalog.Template, previousTemplate itemcatalog.Template) (EquipReplaceResult, bool) {
	if !templateAuthoredForEquipSlot(newTemplate, equipSlot) || !r.CanUseTemplate(newTemplate) || newTemplate.AntiStack || newTemplate.AntiGet || newTemplate.AntiDrop || newTemplate.AntiGive || newTemplate.AntiSell {
		return EquipReplaceResult{}, false
	}
	if previousTemplate.Vnum == 0 || !itemcatalog.ValidTemplate(previousTemplate) || !templateAuthoredForEquipSlot(previousTemplate, equipSlot) {
		return EquipReplaceResult{}, false
	}
	if previousTemplate.Irremovable {
		return EquipReplaceResult{}, false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	if fromIndex < 0 || r.liveInventory[fromIndex].Vnum != newTemplate.Vnum || r.liveInventory[fromIndex].Count == 0 || r.liveInventory[fromIndex].Count > newTemplate.MaxCount {
		return EquipReplaceResult{}, false
	}
	equipIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if equipIndex < 0 || r.liveEquipment[equipIndex].Vnum != previousTemplate.Vnum || r.liveEquipment[equipIndex].Count == 0 || r.liveEquipment[equipIndex].Count > previousTemplate.MaxCount {
		return EquipReplaceResult{}, false
	}
	previousInventory := cloneItemInstances(r.liveInventory)
	previousEquipment := cloneItemInstances(r.liveEquipment)
	previousPoints := r.livePoints
	result, ok := r.replaceOccupiedEquipItem(from, equipSlot)
	if !ok {
		return EquipReplaceResult{}, false
	}
	if previousTemplate.EquipEffect != nil {
		removed, ok := r.RemoveEquipTemplateEffectFromItem(previousTemplate, equipSlot, result.UnequippedItem)
		if !ok {
			r.liveInventory = previousInventory
			r.liveEquipment = previousEquipment
			r.livePoints = previousPoints
			return EquipReplaceResult{}, false
		}
		result.RemovedEffect = &removed
	}
	if newTemplate.EquipEffect != nil {
		applied, ok := r.ApplyEquipTemplateEffect(newTemplate, equipSlot)
		if !ok {
			r.liveInventory = previousInventory
			r.liveEquipment = previousEquipment
			r.livePoints = previousPoints
			return EquipReplaceResult{}, false
		}
		result.AppliedEffect = &applied
	}
	return result, true
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

func (r *Runtime) replaceOccupiedEquipItem(from inventory.SlotIndex, equipSlot inventory.EquipmentSlot) (EquipReplaceResult, bool) {
	if r == nil || !equipSlot.Valid() || countEquipmentSlotOccupancy(r.liveEquipment, equipSlot) != 1 {
		return EquipReplaceResult{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, from) != 1 {
		return EquipReplaceResult{}, false
	}
	fromIndex := findInventorySlot(r.liveInventory, from)
	equipIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if fromIndex < 0 || equipIndex < 0 {
		return EquipReplaceResult{}, false
	}
	sourceItem := r.liveInventory[fromIndex]
	wornItem := r.liveEquipment[equipIndex]
	if sourceItem.Locked || wornItem.Locked {
		return EquipReplaceResult{}, false
	}
	unequippedItem, err := wornItem.WithInventorySlot(from)
	if err != nil {
		return EquipReplaceResult{}, false
	}
	equippedItem := sourceItem
	equippedItem.Slot = 0
	equippedItem.Equipped = true
	equippedItem.EquipSlot = equipSlot
	if err := equippedItem.Validate(); err != nil {
		return EquipReplaceResult{}, false
	}
	if err := unequippedItem.Validate(); err != nil {
		return EquipReplaceResult{}, false
	}
	r.liveInventory[fromIndex] = unequippedItem
	r.liveEquipment[equipIndex] = equippedItem
	sortInventoryItems(r.liveInventory)
	sortEquipmentItems(r.liveEquipment)
	return EquipReplaceResult{UnequippedItem: unequippedItem, EquippedItem: equippedItem}, true
}

func (r *Runtime) UnequipItem(equipSlot inventory.EquipmentSlot, to inventory.SlotIndex) (inventory.ItemInstance, bool) {
	if r == nil || !equipSlot.Valid() || inventorySlotOccupied(r.liveInventory, to) || countEquipmentSlotOccupancy(r.liveEquipment, equipSlot) != 1 {
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

func (r *Runtime) UnequipItemWithTemplate(equipSlot inventory.EquipmentSlot, to inventory.SlotIndex, template itemcatalog.Template) (inventory.ItemInstance, bool) {
	if !templateAuthoredForEquipSlot(template, equipSlot) || !r.CanUseTemplate(template) || template.Irremovable || countEquipmentSlotOccupancy(r.liveEquipment, equipSlot) != 1 {
		return inventory.ItemInstance{}, false
	}
	equipIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if equipIndex < 0 || r.liveEquipment[equipIndex].Vnum != template.Vnum || r.liveEquipment[equipIndex].Count == 0 || r.liveEquipment[equipIndex].Count > template.MaxCount {
		return inventory.ItemInstance{}, false
	}
	return r.UnequipItem(equipSlot, to)
}

func (r *Runtime) ApplyEquipTemplateEffect(template itemcatalog.Template, equipSlot inventory.EquipmentSlot) (PointChangeResult, bool) {
	if r == nil || !templateAuthoredForEquipSlot(template, equipSlot) || template.EquipEffect == nil {
		return PointChangeResult{}, false
	}
	equippedIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if equippedIndex < 0 {
		return PointChangeResult{}, false
	}
	equippedItem := r.liveEquipment[equippedIndex]
	if equippedItem.Vnum != template.Vnum || equippedItem.Count == 0 {
		return PointChangeResult{}, false
	}
	if err := equippedItem.Validate(); err != nil {
		return PointChangeResult{}, false
	}
	effect := *template.EquipEffect
	return r.ApplyPointDelta(effect.PointType, effect.PointIndex, effect.PointDelta)
}

func (r *Runtime) RemoveEquipTemplateEffect(template itemcatalog.Template, equipSlot inventory.EquipmentSlot) (PointChangeResult, bool) {
	if r == nil || !templateAuthoredForEquipSlot(template, equipSlot) || template.EquipEffect == nil {
		return PointChangeResult{}, false
	}
	equippedIndex := findEquipmentSlot(r.liveEquipment, equipSlot)
	if equippedIndex < 0 {
		return PointChangeResult{}, false
	}
	equippedItem := r.liveEquipment[equippedIndex]
	if equippedItem.Vnum != template.Vnum || equippedItem.Count == 0 {
		return PointChangeResult{}, false
	}
	if err := equippedItem.Validate(); err != nil {
		return PointChangeResult{}, false
	}
	return r.removeEquipTemplateEffectDelta(template)
}

func (r *Runtime) RemoveEquipTemplateEffectFromItem(template itemcatalog.Template, equipSlot inventory.EquipmentSlot, item inventory.ItemInstance) (PointChangeResult, bool) {
	if r == nil || !templateAuthoredForEquipSlot(template, equipSlot) || template.EquipEffect == nil {
		return PointChangeResult{}, false
	}
	if item.Vnum != template.Vnum || item.Count == 0 {
		return PointChangeResult{}, false
	}
	if err := item.Validate(); err != nil {
		return PointChangeResult{}, false
	}
	return r.removeEquipTemplateEffectDelta(template)
}

func (r *Runtime) removeEquipTemplateEffectDelta(template itemcatalog.Template) (PointChangeResult, bool) {
	effect := *template.EquipEffect
	if effect.PointDelta == -1<<31 {
		return PointChangeResult{}, false
	}
	return r.ApplyPointDelta(effect.PointType, effect.PointIndex, -effect.PointDelta)
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
	if r == nil || slot >= inventory.CarriedInventorySlotCount || !r.CanUseTemplate(template) || template.EquipSlot != "" || template.UseEffect == nil || template.QuestUse || template.QuestUseMultiple || template.Applicable || template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell {
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
	consumeCount := effect.ConsumeCount
	if consumeCount == 0 {
		consumeCount = 1
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount || consumeCount == 0 || consumeCount > item.Count {
		return ItemUseResult{}, false
	}
	if err := item.Validate(); err != nil {
		return ItemUseResult{}, false
	}
	currentPointValue := r.livePoints[effect.PointIndex]
	nextPointValue := int64(currentPointValue) + int64(effect.PointDelta)
	if nextPointValue < -1<<31 || nextPointValue > 1<<31-1 {
		return ItemUseResult{}, false
	}
	updatedPointValue := int32(nextPointValue)
	result := ItemUseResult{
		Slot:              slot,
		Vnum:              item.Vnum,
		PointType:         effect.PointType,
		PointAmount:       effect.PointDelta,
		PointValue:        updatedPointValue,
		EffectMessage:     useEffectInfoMessage(&effect),
		SpecialEffectType: effect.SpecialEffectType,
	}
	r.livePoints[effect.PointIndex] = updatedPointValue
	if item.Count == consumeCount {
		r.liveInventory = removeInventoryIndex(r.liveInventory, index)
		sortInventoryItems(r.liveInventory)
		result.ItemRemoved = true
		return result, true
	}
	item.Count -= consumeCount
	if err := item.Validate(); err != nil {
		r.livePoints[effect.PointIndex] = currentPointValue
		return ItemUseResult{}, false
	}
	r.liveInventory[index] = item
	sortInventoryItems(r.liveInventory)
	result.Item = item
	return result, true
}

func (r *Runtime) UseItemRejectText(slot inventory.SlotIndex, template itemcatalog.Template) (string, bool) {
	if r == nil || template.UseRejectText == "" || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) || template.EquipSlot != "" || template.UseEffect == nil {
		return "", false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return "", false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return "", false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return "", false
	}
	if err := item.Validate(); err != nil {
		return "", false
	}
	effect := *template.UseEffect
	consumeCount := effect.ConsumeCount
	if consumeCount == 0 {
		consumeCount = 1
	}
	if consumeCount == 0 || consumeCount > item.Count {
		return "", false
	}
	if r.CanUseTemplate(template) && !template.QuestUse && !template.QuestUseMultiple && !template.Applicable && !template.AntiStack && !template.AntiGet && !template.AntiDrop && !template.AntiGive && !template.AntiSell {
		return "", false
	}
	return template.UseRejectText, true
}

func (r *Runtime) GiveRejectText(slot inventory.SlotIndex, count uint16, template itemcatalog.Template) (string, bool) {
	if count == 0 {
		return "", false
	}
	item, ok := r.templateBackedAntiGiveInventoryItem(slot, template)
	if !ok || count > item.Count {
		return "", false
	}
	return template.GiveRejectText, true
}

func (r *Runtime) ExchangeItemAddRejectText(slot inventory.SlotIndex, template itemcatalog.Template) (string, bool) {
	if r == nil || template.GiveRejectText == "" || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return "", false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return "", false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return "", false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return "", false
	}
	if err := item.Validate(); err != nil {
		return "", false
	}
	if r.CanUseTemplate(template) && !template.AntiStack && !template.AntiGet && !template.AntiDrop && !template.AntiGive && !template.AntiSell {
		return "", false
	}
	return template.GiveRejectText, true
}

func (r *Runtime) SafeboxCheckinRejectText(slot inventory.SlotIndex, template itemcatalog.Template) (string, bool) {
	if r == nil || template.SafeboxRejectText == "" || !template.AntiSafebox || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return "", false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return "", false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return "", false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return "", false
	}
	if err := item.Validate(); err != nil {
		return "", false
	}
	return template.SafeboxRejectText, true
}

// SafeboxCheckinItem removes one whole carried stack for an accepted bootstrap
// safebox check-in. Templates that author anti_safebox stay fail-closed here;
// authored reject chat remains owned by SafeboxCheckinRejectText.
func (r *Runtime) SafeboxCheckinItem(slot inventory.SlotIndex, template itemcatalog.Template) (SafeboxCheckinItemResult, bool) {
	if r == nil || template.AntiSafebox || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return SafeboxCheckinItemResult{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return SafeboxCheckinItemResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return SafeboxCheckinItemResult{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return SafeboxCheckinItemResult{}, false
	}
	if err := item.Validate(); err != nil {
		return SafeboxCheckinItemResult{}, false
	}
	updatedInventory := cloneItemInstances(r.liveInventory)
	updatedInventory = removeInventoryIndex(updatedInventory, index)
	sortInventoryItems(updatedInventory)
	r.liveInventory = updatedInventory
	return SafeboxCheckinItemResult{Slot: slot, Item: item}, true
}

// SafeboxCheckoutItem places one whole same-session safebox stack into a named
// carried destination cell. Empty cells preserve item identity; compatible
// unlocked under-max stacks merge the whole count into that named cell.
func (r *Runtime) SafeboxCheckoutItem(destination inventory.SlotIndex, item inventory.ItemInstance, template itemcatalog.Template) (SafeboxCheckoutItemResult, bool) {
	if r == nil || destination >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return SafeboxCheckoutItemResult{}, false
	}
	if item.ID == 0 || item.Vnum == 0 || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return SafeboxCheckoutItemResult{}, false
	}
	if item.Equipped || item.Locked {
		return SafeboxCheckoutItemResult{}, false
	}
	if err := item.Validate(); err != nil {
		return SafeboxCheckoutItemResult{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, destination) > 1 {
		return SafeboxCheckoutItemResult{}, false
	}
	updatedInventory := cloneItemInstances(r.liveInventory)
	destinationIndex := findInventorySlot(updatedInventory, destination)
	if destinationIndex < 0 {
		if hasItemInstanceID(updatedInventory, item.ID) || hasItemInstanceID(r.liveEquipment, item.ID) {
			return SafeboxCheckoutItemResult{}, false
		}
		placed, err := item.WithInventorySlot(destination)
		if err != nil {
			return SafeboxCheckoutItemResult{}, false
		}
		updatedInventory = append(updatedInventory, placed)
		sortInventoryItems(updatedInventory)
		r.liveInventory = updatedInventory
		return SafeboxCheckoutItemResult{Destination: destination, Item: placed}, true
	}
	destinationItem := updatedInventory[destinationIndex]
	if destinationItem.Equipped || destinationItem.Locked || destinationItem.Vnum != item.Vnum || destinationItem.Count == 0 || destinationItem.Count > template.MaxCount {
		return SafeboxCheckoutItemResult{}, false
	}
	if uint32(destinationItem.Count)+uint32(item.Count) > uint32(template.MaxCount) {
		return SafeboxCheckoutItemResult{}, false
	}
	destinationItem.Count += item.Count
	if err := destinationItem.Validate(); err != nil {
		return SafeboxCheckoutItemResult{}, false
	}
	updatedInventory[destinationIndex] = destinationItem
	sortInventoryItems(updatedInventory)
	r.liveInventory = updatedInventory
	return SafeboxCheckoutItemResult{Destination: destination, Merged: true, Item: destinationItem}, true
}

func (r *Runtime) ExchangeItemAddDisplay(slot inventory.SlotIndex, template itemcatalog.Template) (ExchangeItemAddDisplay, bool) {
	item, ok := r.templateBackedExchangeDisplayInventoryItem(slot, template)
	if !ok {
		return ExchangeItemAddDisplay{}, false
	}
	sockets := itemcatalog.SocketValues(item.EffectiveSockets(inventory.SocketValues(template.Sockets)))
	return ExchangeItemAddDisplay{Item: item, Sockets: sockets, Attributes: template.Attributes}, true
}

func (r *Runtime) templateBackedAntiGiveInventoryItem(slot inventory.SlotIndex, template itemcatalog.Template) (inventory.ItemInstance, bool) {
	if r == nil || template.GiveRejectText == "" || !template.AntiGive || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return inventory.ItemInstance{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return inventory.ItemInstance{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return inventory.ItemInstance{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return inventory.ItemInstance{}, false
	}
	if err := item.Validate(); err != nil {
		return inventory.ItemInstance{}, false
	}
	return item, true
}

func (r *Runtime) templateBackedExchangeDisplayInventoryItem(slot inventory.SlotIndex, template itemcatalog.Template) (inventory.ItemInstance, bool) {
	if r == nil || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return inventory.ItemInstance{}, false
	}
	if template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || !r.CanUseTemplate(template) {
		return inventory.ItemInstance{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return inventory.ItemInstance{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return inventory.ItemInstance{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return inventory.ItemInstance{}, false
	}
	if err := item.Validate(); err != nil {
		return inventory.ItemInstance{}, false
	}
	return item, true
}

func (r *Runtime) RefineRejectText(slot inventory.SlotIndex, template itemcatalog.Template) (string, bool) {
	if r == nil || template.RefineRejectText == "" || template.Refineable || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) {
		return "", false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return "", false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return "", false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return "", false
	}
	if err := item.Validate(); err != nil {
		return "", false
	}
	return template.RefineRejectText, true
}

func (r *Runtime) RefineInformation(slot inventory.SlotIndex, refineType uint8, template itemcatalog.Template) (RefineInformation, bool) {
	if r == nil || !template.Refineable || template.RefineInfo == nil || slot >= inventory.CarriedInventorySlotCount || !itemcatalog.ValidTemplate(template) || !r.CanUseTemplate(template) || template.AntiStack || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell {
		return RefineInformation{}, false
	}
	if countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
		return RefineInformation{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return RefineInformation{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Vnum != template.Vnum || item.Count == 0 || item.Count > template.MaxCount {
		return RefineInformation{}, false
	}
	if err := item.Validate(); err != nil {
		return RefineInformation{}, false
	}
	return RefineInformation{
		Type:        refineType,
		Position:    slot,
		SourceVnum:  item.Vnum,
		ResultVnum:  template.RefineInfo.ResultVnum,
		Cost:        template.RefineInfo.Cost,
		Probability: template.RefineInfo.Probability,
		Materials:   append([]itemcatalog.RefineMaterial(nil), template.RefineInfo.Materials...),
	}, true
}

func (r *Runtime) ApplyRefineSuccess(slot inventory.SlotIndex, refineType uint8, sourceID uint64, remembered itemcatalog.RefineInfo, sourceTemplate itemcatalog.Template, resultTemplate itemcatalog.Template) (RefineSuccessResult, bool) {
	if r == nil || sourceID == 0 || remembered.Probability != 100 || remembered.Cost < 0 || remembered.ResultVnum == 0 || len(remembered.Materials) > itemcatalog.MaxRefineMaterialCount {
		return RefineSuccessResult{}, false
	}
	info, ok := r.RefineInformation(slot, refineType, sourceTemplate)
	if !ok || info.SourceVnum == 0 || sourceTemplate.RefineInfo == nil {
		return RefineSuccessResult{}, false
	}
	if !refineInfoEqual(remembered, *sourceTemplate.RefineInfo) || !refineInfoEqual(remembered, itemcatalog.RefineInfo{
		ResultVnum:  info.ResultVnum,
		Cost:        info.Cost,
		Probability: info.Probability,
		KeepOnFail:  sourceTemplate.RefineInfo.KeepOnFail,
		Materials:   info.Materials,
	}) {
		return RefineSuccessResult{}, false
	}
	if !itemcatalog.ValidTemplate(resultTemplate) || resultTemplate.Vnum != remembered.ResultVnum {
		return RefineSuccessResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return RefineSuccessResult{}, false
	}
	sourceItem := r.liveInventory[index]
	if sourceItem.ID != sourceID || sourceItem.Vnum != info.SourceVnum || sourceItem.Equipped || sourceItem.Locked || sourceItem.Count != 1 {
		return RefineSuccessResult{}, false
	}
	cost := uint64(remembered.Cost)
	const maxPointChangeCarrier = uint64(1<<31 - 1)
	if cost > maxPointChangeCarrier || r.liveGold < cost || r.liveGold > maxPointChangeCarrier {
		return RefineSuccessResult{}, false
	}
	nextGold := r.liveGold - cost
	materialPlan, ok := planRefineMaterialChanges(r.liveInventory, slot, remembered.Materials)
	if !ok {
		return RefineSuccessResult{}, false
	}

	inventoryItems := cloneItemInstances(r.liveInventory)
	materialChanges := make([]RefineMaterialChange, 0, len(materialPlan))
	for _, planned := range materialPlan {
		currentIndex := findInventorySlot(inventoryItems, planned.Slot)
		if currentIndex < 0 {
			return RefineSuccessResult{}, false
		}
		item := inventoryItems[currentIndex]
		if item.Equipped || item.Locked || item.Vnum != planned.Vnum || item.Count < planned.Consume {
			return RefineSuccessResult{}, false
		}
		change := RefineMaterialChange{Slot: planned.Slot}
		if item.Count == planned.Consume {
			inventoryItems = removeInventoryIndex(inventoryItems, currentIndex)
			change.ItemRemoved = true
		} else {
			item.Count -= planned.Consume
			if err := item.Validate(); err != nil {
				return RefineSuccessResult{}, false
			}
			inventoryItems[currentIndex] = item
			change.Item = item
		}
		materialChanges = append(materialChanges, change)
	}
	sourceIndex := findInventorySlot(inventoryItems, slot)
	if sourceIndex < 0 {
		return RefineSuccessResult{}, false
	}
	resultItem := inventoryItems[sourceIndex]
	if resultItem.ID != sourceID || resultItem.Vnum != info.SourceVnum || resultItem.Count != 1 || resultItem.Equipped || resultItem.Locked {
		return RefineSuccessResult{}, false
	}
	resultItem.Vnum = remembered.ResultVnum
	if err := resultItem.Validate(); err != nil {
		return RefineSuccessResult{}, false
	}
	inventoryItems[sourceIndex] = resultItem
	sortInventoryItems(inventoryItems)

	result := RefineSuccessResult{
		SourceSlot:      slot,
		ResultItem:      resultItem,
		MaterialChanges: materialChanges,
		GoldBefore:      r.liveGold,
		Gold:            nextGold,
		Cost:            remembered.Cost,
	}
	r.liveGold = nextGold
	r.liveInventory = inventoryItems
	return result, true
}

func (r *Runtime) ApplyRefineDestroyFailure(slot inventory.SlotIndex, refineType uint8, sourceID uint64, remembered itemcatalog.RefineInfo, sourceTemplate itemcatalog.Template, resultTemplate itemcatalog.Template) (RefineDestroyFailureResult, bool) {
	if r == nil || sourceID == 0 || remembered.Probability != 0 || remembered.Cost < 0 || remembered.ResultVnum == 0 || len(remembered.Materials) > itemcatalog.MaxRefineMaterialCount {
		return RefineDestroyFailureResult{}, false
	}
	info, ok := r.RefineInformation(slot, refineType, sourceTemplate)
	if !ok || info.SourceVnum == 0 || sourceTemplate.RefineInfo == nil {
		return RefineDestroyFailureResult{}, false
	}
	if !refineInfoEqual(remembered, *sourceTemplate.RefineInfo) || !refineInfoEqual(remembered, itemcatalog.RefineInfo{
		ResultVnum:  info.ResultVnum,
		Cost:        info.Cost,
		Probability: info.Probability,
		KeepOnFail:  sourceTemplate.RefineInfo.KeepOnFail,
		Materials:   info.Materials,
	}) {
		return RefineDestroyFailureResult{}, false
	}
	if !itemcatalog.ValidTemplate(resultTemplate) || resultTemplate.Vnum != remembered.ResultVnum {
		return RefineDestroyFailureResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return RefineDestroyFailureResult{}, false
	}
	sourceItem := r.liveInventory[index]
	if sourceItem.ID != sourceID || sourceItem.Vnum != info.SourceVnum || sourceItem.Equipped || sourceItem.Locked || sourceItem.Count != 1 {
		return RefineDestroyFailureResult{}, false
	}
	cost := uint64(remembered.Cost)
	const maxPointChangeCarrier = uint64(1<<31 - 1)
	if cost > maxPointChangeCarrier || r.liveGold < cost || r.liveGold > maxPointChangeCarrier {
		return RefineDestroyFailureResult{}, false
	}
	nextGold := r.liveGold - cost
	materialPlan, ok := planRefineMaterialChanges(r.liveInventory, slot, remembered.Materials)
	if !ok {
		return RefineDestroyFailureResult{}, false
	}

	inventoryItems := cloneItemInstances(r.liveInventory)
	materialChanges := make([]RefineMaterialChange, 0, len(materialPlan))
	for _, planned := range materialPlan {
		currentIndex := findInventorySlot(inventoryItems, planned.Slot)
		if currentIndex < 0 {
			return RefineDestroyFailureResult{}, false
		}
		item := inventoryItems[currentIndex]
		if item.Equipped || item.Locked || item.Vnum != planned.Vnum || item.Count < planned.Consume {
			return RefineDestroyFailureResult{}, false
		}
		change := RefineMaterialChange{Slot: planned.Slot}
		if item.Count == planned.Consume {
			inventoryItems = removeInventoryIndex(inventoryItems, currentIndex)
			change.ItemRemoved = true
		} else {
			item.Count -= planned.Consume
			if err := item.Validate(); err != nil {
				return RefineDestroyFailureResult{}, false
			}
			inventoryItems[currentIndex] = item
			change.Item = item
		}
		materialChanges = append(materialChanges, change)
	}
	sourceIndex := findInventorySlot(inventoryItems, slot)
	if sourceIndex < 0 {
		return RefineDestroyFailureResult{}, false
	}
	destroyed := inventoryItems[sourceIndex]
	if destroyed.ID != sourceID || destroyed.Vnum != info.SourceVnum || destroyed.Count != 1 || destroyed.Equipped || destroyed.Locked {
		return RefineDestroyFailureResult{}, false
	}
	inventoryItems = removeInventoryIndex(inventoryItems, sourceIndex)
	sortInventoryItems(inventoryItems)

	result := RefineDestroyFailureResult{
		SourceSlot:      slot,
		MaterialChanges: materialChanges,
		GoldBefore:      r.liveGold,
		Gold:            nextGold,
		Cost:            remembered.Cost,
	}
	r.liveGold = nextGold
	r.liveInventory = inventoryItems
	return result, true
}

// ApplyRefineKeepFailure owns the keep-on-fail mutation for remembered
// refine_info with KeepOnFail and probability in 1..99: gold/materials are
// consumed while the source carried item remains in place.
func (r *Runtime) ApplyRefineKeepFailure(slot inventory.SlotIndex, refineType uint8, sourceID uint64, remembered itemcatalog.RefineInfo, sourceTemplate itemcatalog.Template, resultTemplate itemcatalog.Template) (RefineKeepFailureResult, bool) {
	if r == nil || sourceID == 0 || !remembered.KeepOnFail || remembered.Probability < 1 || remembered.Probability > 99 || remembered.Cost < 0 || remembered.ResultVnum == 0 || len(remembered.Materials) > itemcatalog.MaxRefineMaterialCount {
		return RefineKeepFailureResult{}, false
	}
	info, ok := r.RefineInformation(slot, refineType, sourceTemplate)
	if !ok || info.SourceVnum == 0 || sourceTemplate.RefineInfo == nil {
		return RefineKeepFailureResult{}, false
	}
	if !refineInfoEqual(remembered, *sourceTemplate.RefineInfo) || !refineInfoEqual(remembered, itemcatalog.RefineInfo{
		ResultVnum:  info.ResultVnum,
		Cost:        info.Cost,
		Probability: info.Probability,
		KeepOnFail:  sourceTemplate.RefineInfo.KeepOnFail,
		Materials:   info.Materials,
	}) {
		return RefineKeepFailureResult{}, false
	}
	if !itemcatalog.ValidTemplate(resultTemplate) || resultTemplate.Vnum != remembered.ResultVnum {
		return RefineKeepFailureResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return RefineKeepFailureResult{}, false
	}
	sourceItem := r.liveInventory[index]
	if sourceItem.ID != sourceID || sourceItem.Vnum != info.SourceVnum || sourceItem.Equipped || sourceItem.Locked || sourceItem.Count != 1 {
		return RefineKeepFailureResult{}, false
	}
	cost := uint64(remembered.Cost)
	const maxPointChangeCarrier = uint64(1<<31 - 1)
	if cost > maxPointChangeCarrier || r.liveGold < cost || r.liveGold > maxPointChangeCarrier {
		return RefineKeepFailureResult{}, false
	}
	nextGold := r.liveGold - cost
	materialPlan, ok := planRefineMaterialChanges(r.liveInventory, slot, remembered.Materials)
	if !ok {
		return RefineKeepFailureResult{}, false
	}

	inventoryItems := cloneItemInstances(r.liveInventory)
	materialChanges := make([]RefineMaterialChange, 0, len(materialPlan))
	for _, planned := range materialPlan {
		currentIndex := findInventorySlot(inventoryItems, planned.Slot)
		if currentIndex < 0 {
			return RefineKeepFailureResult{}, false
		}
		item := inventoryItems[currentIndex]
		if item.Equipped || item.Locked || item.Vnum != planned.Vnum || item.Count < planned.Consume {
			return RefineKeepFailureResult{}, false
		}
		change := RefineMaterialChange{Slot: planned.Slot}
		if item.Count == planned.Consume {
			inventoryItems = removeInventoryIndex(inventoryItems, currentIndex)
			change.ItemRemoved = true
		} else {
			item.Count -= planned.Consume
			if err := item.Validate(); err != nil {
				return RefineKeepFailureResult{}, false
			}
			inventoryItems[currentIndex] = item
			change.Item = item
		}
		materialChanges = append(materialChanges, change)
	}
	sourceIndex := findInventorySlot(inventoryItems, slot)
	if sourceIndex < 0 {
		return RefineKeepFailureResult{}, false
	}
	kept := inventoryItems[sourceIndex]
	if kept.ID != sourceID || kept.Vnum != info.SourceVnum || kept.Count != 1 || kept.Equipped || kept.Locked {
		return RefineKeepFailureResult{}, false
	}
	sortInventoryItems(inventoryItems)

	result := RefineKeepFailureResult{
		SourceSlot:      slot,
		MaterialChanges: materialChanges,
		GoldBefore:      r.liveGold,
		Gold:            nextGold,
		Cost:            remembered.Cost,
	}
	r.liveGold = nextGold
	r.liveInventory = inventoryItems
	return result, true
}

// ApplyRefineWithRoll owns the first deterministic confirm path for remembered
// refine_info.probability values in 1..99. roll must be in 1..100:
// roll <= probability applies the owned success mutation; roll > probability
// applies keep-on-fail when authored, otherwise the owned whole-source destroy
// mutation. Rolls outside 1..100 and remembered probabilities outside 1..99
// fail closed with no mutation.
func (r *Runtime) ApplyRefineWithRoll(slot inventory.SlotIndex, refineType uint8, sourceID uint64, remembered itemcatalog.RefineInfo, sourceTemplate itemcatalog.Template, resultTemplate itemcatalog.Template, roll int) (RefineWithRollResult, bool) {
	if r == nil || roll < 1 || roll > 100 || remembered.Probability < 1 || remembered.Probability > 99 {
		return RefineWithRollResult{}, false
	}
	if sourceTemplate.RefineInfo == nil || !refineInfoEqual(remembered, *sourceTemplate.RefineInfo) {
		return RefineWithRollResult{}, false
	}
	adjustedRemembered := remembered
	adjustedSource := sourceTemplate
	adjustedInfo := *sourceTemplate.RefineInfo
	if int32(roll) <= remembered.Probability {
		adjustedRemembered.Probability = 100
		adjustedRemembered.KeepOnFail = false
		adjustedInfo.Probability = 100
		adjustedInfo.KeepOnFail = false
		adjustedSource.RefineInfo = &adjustedInfo
		success, ok := r.ApplyRefineSuccess(slot, refineType, sourceID, adjustedRemembered, adjustedSource, resultTemplate)
		if !ok {
			return RefineWithRollResult{}, false
		}
		return RefineWithRollResult{Succeeded: true, Success: success}, true
	}
	if remembered.KeepOnFail {
		keep, ok := r.ApplyRefineKeepFailure(slot, refineType, sourceID, remembered, sourceTemplate, resultTemplate)
		if !ok {
			return RefineWithRollResult{}, false
		}
		return RefineWithRollResult{Kept: true, Keep: keep}, true
	}
	adjustedRemembered.Probability = 0
	adjustedRemembered.KeepOnFail = false
	adjustedInfo.Probability = 0
	adjustedInfo.KeepOnFail = false
	adjustedSource.RefineInfo = &adjustedInfo
	destroy, ok := r.ApplyRefineDestroyFailure(slot, refineType, sourceID, adjustedRemembered, adjustedSource, resultTemplate)
	if !ok {
		return RefineWithRollResult{}, false
	}
	return RefineWithRollResult{Destroyed: true, Destroy: destroy}, true
}

type refineMaterialPlanEntry struct {
	Slot    inventory.SlotIndex
	Vnum    uint32
	Consume uint16
}

func planRefineMaterialChanges(items []inventory.ItemInstance, sourceSlot inventory.SlotIndex, materials []itemcatalog.RefineMaterial) ([]refineMaterialPlanEntry, bool) {
	remainingByVnum := make(map[uint32]uint64, len(materials))
	order := make([]uint32, 0, len(materials))
	for _, material := range materials {
		if material.Vnum == 0 || material.Count <= 0 {
			return nil, false
		}
		needed := uint64(material.Count)
		if _, seen := remainingByVnum[material.Vnum]; !seen {
			order = append(order, material.Vnum)
		}
		remainingByVnum[material.Vnum] += needed
	}

	indices := make([]int, 0, len(items))
	for i, item := range items {
		if item.Equipped || item.Locked || item.Slot == sourceSlot || item.Count == 0 {
			continue
		}
		if _, needed := remainingByVnum[item.Vnum]; !needed {
			continue
		}
		if err := item.Validate(); err != nil {
			continue
		}
		indices = append(indices, i)
	}
	sort.Slice(indices, func(i, j int) bool {
		return items[indices[i]].Slot < items[indices[j]].Slot
	})

	plan := make([]refineMaterialPlanEntry, 0)
	for _, index := range indices {
		item := items[index]
		needed := remainingByVnum[item.Vnum]
		if needed == 0 {
			continue
		}
		consume := uint64(item.Count)
		if consume > needed {
			consume = needed
		}
		if consume == 0 || consume > uint64(^uint16(0)) {
			return nil, false
		}
		plan = append(plan, refineMaterialPlanEntry{Slot: item.Slot, Vnum: item.Vnum, Consume: uint16(consume)})
		remainingByVnum[item.Vnum] -= consume
	}
	for _, vnum := range order {
		if remainingByVnum[vnum] != 0 {
			return nil, false
		}
	}
	sort.Slice(plan, func(i, j int) bool {
		return plan[i].Slot < plan[j].Slot
	})
	return plan, true
}

func refineInfoEqual(left, right itemcatalog.RefineInfo) bool {
	if left.ResultVnum != right.ResultVnum || left.Cost != right.Cost || left.Probability != right.Probability || left.KeepOnFail != right.KeepOnFail || len(left.Materials) != len(right.Materials) {
		return false
	}
	for i := range left.Materials {
		if left.Materials[i] != right.Materials[i] {
			return false
		}
	}
	return true
}

func useEffectInfoMessage(effect *itemcatalog.UseEffect) string {
	if effect == nil {
		return ""
	}
	if effect.InfoMessage != "" {
		return effect.InfoMessage
	}
	return effect.Message
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
		result.CountOnly = true
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
	if r == nil || !r.CanUseTemplate(template) || template.AntiGet || count == 0 || count > template.MaxCount || price == 0 {
		return MerchantBuyFailureInvalid
	}
	if !template.Stackable && count != 1 {
		return MerchantBuyFailureInvalid
	}
	if r.liveGold < price {
		return MerchantBuyFailureInsufficientGold
	}
	if failure := r.ValidateCarriedItemGrant(template, count); failure != "" {
		switch failure {
		case CarriedItemGrantFailureNoValidPlacement:
			return MerchantBuyFailureNoValidPlacement
		default:
			return MerchantBuyFailureInvalid
		}
	}
	return ""
}

func (r *Runtime) ValidateCarriedItemGrant(template itemcatalog.Template, count uint16) CarriedItemGrantFailure {
	if r == nil || !itemcatalog.ValidTemplate(template) || !r.CanUseTemplate(template) || template.AntiGet || count == 0 || count > template.MaxCount {
		return CarriedItemGrantFailureInvalid
	}
	if !template.Stackable && count != 1 {
		return CarriedItemGrantFailureInvalid
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
			return CarriedItemGrantFailureNoValidPlacement
		}
	}
	if _, ok := nextFreeInventorySlot(r.liveInventory); !ok {
		return CarriedItemGrantFailureNoValidPlacement
	}
	return ""
}

func (r *Runtime) BuyMerchantItem(template itemcatalog.Template, count uint16, price uint64) (MerchantBuyResult, bool) {
	if failure := r.ValidateMerchantBuy(template, count, price); failure != "" {
		return MerchantBuyResult{}, false
	}
	grant, ok := r.grantCarriedItem(template, count)
	if !ok {
		return MerchantBuyResult{}, false
	}
	r.liveGold -= price
	return MerchantBuyResult{Items: grant.Items, ItemChanges: grant.ItemChanges, Gold: r.liveGold}, true
}

func (r *Runtime) GrantCarriedItem(template itemcatalog.Template, count uint16) (CarriedItemGrantResult, bool) {
	if failure := r.ValidateCarriedItemGrant(template, count); failure != "" {
		return CarriedItemGrantResult{}, false
	}
	return r.grantCarriedItem(template, count)
}

func (r *Runtime) grantCarriedItem(template itemcatalog.Template, count uint16) (CarriedItemGrantResult, bool) {
	inventoryItems := cloneItemInstances(r.liveInventory)
	changedItems := make([]inventory.ItemInstance, 0, 2)
	changedExistingSlots := map[inventory.SlotIndex]bool{}
	remaining := count
	if template.Stackable && !template.AntiStack {
		if mergeIndex := findMergeableInventoryIndex(inventoryItems, template.Vnum, remaining, template.MaxCount); mergeIndex >= 0 {
			item := inventoryItems[mergeIndex]
			item.Count += remaining
			if err := item.Validate(); err != nil {
				return CarriedItemGrantResult{}, false
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
			return CarriedItemGrantResult{}, false
		}
		item, err := (inventory.ItemInstance{ID: nextLiveItemInstanceID(inventoryItems, r.liveEquipment), Vnum: template.Vnum, Count: remaining}).WithInventorySlot(slot)
		if err != nil {
			return CarriedItemGrantResult{}, false
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
	r.liveInventory = inventoryItems
	return CarriedItemGrantResult{Items: changedItems, ItemChanges: itemChanges}, true
}

func (r *Runtime) ValidateCarriedItemConsume(requirements []CarriedItemConsumeRequirement) CarriedItemConsumeFailure {
	if r == nil {
		return CarriedItemConsumeFailureInvalid
	}
	if _, ok := planCarriedItemConsumeChanges(r.liveInventory, requirements, nil); !ok {
		if len(requirements) == 0 {
			return ""
		}
		for _, requirement := range requirements {
			if requirement.ItemVnum == 0 || requirement.Count == 0 {
				return CarriedItemConsumeFailureInvalid
			}
		}
		return CarriedItemConsumeFailureInsufficientMaterials
	}
	return ""
}

func (r *Runtime) ConsumeCarriedItems(requirements []CarriedItemConsumeRequirement) (CarriedItemConsumeResult, bool) {
	if failure := r.ValidateCarriedItemConsume(requirements); failure != "" {
		return CarriedItemConsumeResult{}, false
	}
	return r.consumeCarriedItems(requirements, nil)
}

// HasCarriedItemExcludingSlots reports whether at least one carried unlocked
// unequipped stack of vnum exists outside exclude. Used by MYSHOP open so a
// listed stock cell or locked/equipped silk bag cannot unlock the silk path.
func (r *Runtime) HasCarriedItemExcludingSlots(vnum uint32, exclude map[inventory.SlotIndex]struct{}) bool {
	if r == nil || vnum == 0 {
		return false
	}
	_, ok := planCarriedItemConsumeChanges(r.liveInventory, []CarriedItemConsumeRequirement{{ItemVnum: vnum, Count: 1}}, exclude)
	return ok
}

// SetCarriedItemSockets replaces the authoritative per-instance sockets on a
// carried unlocked unequipped cell. Presence is always set (including all-zero).
func (r *Runtime) SetCarriedItemSockets(slot inventory.SlotIndex, sockets inventory.SocketValues) (inventory.ItemInstance, bool) {
	if r == nil {
		return inventory.ItemInstance{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 {
		return inventory.ItemInstance{}, false
	}
	item := r.liveInventory[index]
	if item.Equipped || item.Locked || item.Count == 0 {
		return inventory.ItemInstance{}, false
	}
	if err := item.Validate(); err != nil {
		return inventory.ItemInstance{}, false
	}
	copied := sockets
	item.Sockets = &copied
	r.liveInventory[index] = item
	return item, true
}

// ConsumeCarriedItemsExcludingSlots debits by-vnum carried stacks while skipping
// the given inventory cells. Used by MYSHOP open so a listed stock cell cannot
// also pay the shop-bag cost.
func (r *Runtime) ConsumeCarriedItemsExcludingSlots(requirements []CarriedItemConsumeRequirement, exclude map[inventory.SlotIndex]struct{}) (CarriedItemConsumeResult, bool) {
	if r == nil {
		return CarriedItemConsumeResult{}, false
	}
	if len(requirements) == 0 {
		return CarriedItemConsumeResult{}, true
	}
	for _, requirement := range requirements {
		if requirement.ItemVnum == 0 || requirement.Count == 0 {
			return CarriedItemConsumeResult{}, false
		}
	}
	return r.consumeCarriedItems(requirements, exclude)
}

func (r *Runtime) consumeCarriedItems(requirements []CarriedItemConsumeRequirement, exclude map[inventory.SlotIndex]struct{}) (CarriedItemConsumeResult, bool) {
	plan, ok := planCarriedItemConsumeChanges(r.liveInventory, requirements, exclude)
	if !ok {
		return CarriedItemConsumeResult{}, false
	}
	if len(plan) == 0 {
		return CarriedItemConsumeResult{}, true
	}
	inventoryItems := cloneItemInstances(r.liveInventory)
	changes := make([]CarriedItemConsumeChange, 0, len(plan))
	for _, planned := range plan {
		currentIndex := findInventorySlot(inventoryItems, planned.Slot)
		if currentIndex < 0 {
			return CarriedItemConsumeResult{}, false
		}
		item := inventoryItems[currentIndex]
		if item.Equipped || item.Locked || item.Vnum != planned.Vnum || item.Count < planned.Consume {
			return CarriedItemConsumeResult{}, false
		}
		if exclude != nil {
			if _, blocked := exclude[planned.Slot]; blocked {
				return CarriedItemConsumeResult{}, false
			}
		}
		change := CarriedItemConsumeChange{Slot: planned.Slot}
		if item.Count == planned.Consume {
			inventoryItems = removeInventoryIndex(inventoryItems, currentIndex)
			change.ItemRemoved = true
		} else {
			item.Count -= planned.Consume
			if err := item.Validate(); err != nil {
				return CarriedItemConsumeResult{}, false
			}
			inventoryItems[currentIndex] = item
			change.Item = item
		}
		changes = append(changes, change)
	}
	sortInventoryItems(inventoryItems)
	r.liveInventory = inventoryItems
	return CarriedItemConsumeResult{Changes: changes}, true
}

func planCarriedItemConsumeChanges(items []inventory.ItemInstance, requirements []CarriedItemConsumeRequirement, exclude map[inventory.SlotIndex]struct{}) ([]refineMaterialPlanEntry, bool) {
	if len(requirements) == 0 {
		return nil, true
	}
	materials := make([]itemcatalog.RefineMaterial, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.ItemVnum == 0 || requirement.Count == 0 {
			return nil, false
		}
		materials = append(materials, itemcatalog.RefineMaterial{Vnum: requirement.ItemVnum, Count: int32(requirement.Count)})
	}
	// Reuse refine planning with an impossible source slot so every carried stack remains eligible,
	// then drop any plan entries that land on excluded MYSHOP-listed cells.
	plan, ok := planRefineMaterialChanges(items, inventory.CarriedInventorySlotCount, materials)
	if !ok {
		return nil, false
	}
	if len(exclude) == 0 {
		return plan, true
	}
	filtered := make([]refineMaterialPlanEntry, 0, len(plan))
	remainingByVnum := make(map[uint32]uint64, len(materials))
	for _, material := range materials {
		remainingByVnum[material.Vnum] += uint64(material.Count)
	}
	for _, entry := range plan {
		if _, blocked := exclude[entry.Slot]; blocked {
			continue
		}
		filtered = append(filtered, entry)
		if remainingByVnum[entry.Vnum] < uint64(entry.Consume) {
			return nil, false
		}
		remainingByVnum[entry.Vnum] -= uint64(entry.Consume)
	}
	for _, material := range materials {
		if remainingByVnum[material.Vnum] != 0 {
			return nil, false
		}
	}
	return filtered, true
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
	if r == nil || !r.CanUseTemplate(template) || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack {
		return MerchantSellResult{}, false
	}
	index := findInventorySlot(r.liveInventory, slot)
	if index < 0 || r.liveInventory[index].Vnum != template.Vnum || r.liveInventory[index].Count > template.MaxCount {
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
	if err := item.Validate(); err != nil {
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
	const maxPointChangeCarrier = uint64(1<<31 - 1)
	if r == nil || credit == 0 || credit > maxPointChangeCarrier || countInventorySlotOccupancy(r.liveInventory, slot) != 1 {
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
	if err := item.Validate(); err != nil {
		return MerchantSellResult{}, false
	}
	soldCount := count
	if soldCount == 0 {
		soldCount = item.Count
	}
	if soldCount == 0 || soldCount > item.Count {
		return MerchantSellResult{}, false
	}
	if r.liveGold > maxPointChangeCarrier || r.liveGold > maxPointChangeCarrier-credit {
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
	if !itemcatalog.ValidTemplate(template) || template.AntiGet || template.AntiDrop || template.AntiGive || template.AntiSell || template.AntiStack || count == 0 {
		return 0, false
	}
	const maxPointChangeCarrier = uint64(1<<31 - 1)
	if template.ShopSellPrice != 0 {
		if template.ShopSellPrice > maxPointChangeCarrier || template.ShopSellPrice > (^uint64(0))/uint64(count) {
			return 0, false
		}
		price := template.ShopSellPrice * uint64(count)
		if price == 0 || price > maxPointChangeCarrier {
			return 0, false
		}
		return price, true
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
	if price == 0 || price > maxPointChangeCarrier {
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
	cloned := append([]inventory.ItemInstance(nil), items...)
	for i := range cloned {
		cloned[i].Sockets = items[i].CloneSockets()
	}
	return cloned
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

func countEquipmentSlotOccupancy(items []inventory.ItemInstance, slot inventory.EquipmentSlot) int {
	count := 0
	for _, item := range items {
		if item.Equipped && item.EquipSlot == slot {
			count++
		}
	}
	return count
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
		return slot.Slot == 0
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
