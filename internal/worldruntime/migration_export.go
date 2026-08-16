package worldruntime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	BootstrapGroundItemStateMigrationVersion = 10
	BootstrapGroundItemStateMigrationName    = "bootstrap_ground_item_state"

	bootstrapGroundItemOwnerNameMaxBytes = 25
	bootstrapGroundItemMaxCount          = uint16(^uint8(0))
	bootstrapGroundGoldVnum              = 1
	bootstrapGroundGoldMaxAmount         = uint32(1<<31 - 1)
)

var ErrInvalidBootstrapGroundItemStateExport = errors.New("invalid bootstrap ground-item state export")

// BootstrapGroundItemStateExport is a deterministic, schema-shaped projection of
// currently pending bootstrap ground handles onto the
// 0010_bootstrap_ground_item_state migration boundary. It is intentionally a
// live read-only export contract only: it does not open a database, emit SQL,
// apply migrations, mutate runtime ground state, or make ground handles durable
// across process restart.
type BootstrapGroundItemStateExport struct {
	MigrationVersion int                           `json:"migration_version"`
	MigrationName    string                        `json:"migration_name"`
	GroundItems      []BootstrapGroundItemStateRow `json:"ground_items"`
}

// BootstrapGroundItemStateRow mirrors the bootstrap_ground_items table columns
// frozen by migration 0010, excluding timestamps that are database-owned at
// insert/update time.
type BootstrapGroundItemStateRow struct {
	VID              uint32  `json:"vid"`
	Vnum             uint32  `json:"vnum"`
	ItemCount        *uint16 `json:"item_count,omitempty"`
	GoldAmount       *uint32 `json:"gold_amount,omitempty"`
	OwnerLogin       string  `json:"owner_login"`
	OwnerCharacterID uint32  `json:"owner_character_id"`
	OwnerVID         uint32  `json:"owner_vid"`
	OwnerName        string  `json:"owner_name"`
	MapIndex         uint32  `json:"map_index"`
	X                int32   `json:"x"`
	Y                int32   `json:"y"`
	Z                int32   `json:"z"`
	PickupRange      int64   `json:"pickup_range"`
}

// ExportBootstrapGroundItemState validates pending bootstrap ground snapshots
// and returns rows ordered exactly as a future backfill/import tool should
// process them: by visible ground VID. Validation fails closed against the 0010
// migration constraints so malformed live/debug state cannot be silently coerced
// into a future database import.
func ExportBootstrapGroundItemState(snapshots []GroundItemSnapshot) (BootstrapGroundItemStateExport, error) {
	ordered := append([]GroundItemSnapshot(nil), snapshots...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].VID < ordered[j].VID
	})

	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}
	seenVIDs := make(map[uint32]struct{}, len(ordered))
	for _, snapshot := range ordered {
		row, err := bootstrapGroundItemStateRowForExport(snapshot)
		if err != nil {
			return BootstrapGroundItemStateExport{}, err
		}
		if _, ok := seenVIDs[row.VID]; ok {
			return BootstrapGroundItemStateExport{}, fmt.Errorf("%w: duplicate ground vid %d", ErrInvalidBootstrapGroundItemStateExport, row.VID)
		}
		seenVIDs[row.VID] = struct{}{}
		export.GroundItems = append(export.GroundItems, row)
	}
	return export, nil
}

func bootstrapGroundItemStateRowForExport(snapshot GroundItemSnapshot) (BootstrapGroundItemStateRow, error) {
	if snapshot.VID == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid must be positive", ErrInvalidBootstrapGroundItemStateExport)
	}
	if snapshot.Vnum == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has zero vnum", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if !validBootstrapGroundOwnerMetadata(snapshot.OwnerLogin) {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has invalid owner login", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if !validBootstrapGroundOwnerMetadata(snapshot.OwnerName) || len(snapshot.OwnerName) > bootstrapGroundItemOwnerNameMaxBytes {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has invalid owner name", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if snapshot.OwnerCharacterID == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has zero owner character id", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if snapshot.OwnerVID == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has zero owner vid", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if snapshot.MapIndex == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has zero map index", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if snapshot.PickupRange <= 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has non-positive pickup range", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}

	row := BootstrapGroundItemStateRow{
		VID:              snapshot.VID,
		Vnum:             snapshot.Vnum,
		OwnerLogin:       snapshot.OwnerLogin,
		OwnerCharacterID: snapshot.OwnerCharacterID,
		OwnerVID:         snapshot.OwnerVID,
		OwnerName:        snapshot.OwnerName,
		MapIndex:         snapshot.MapIndex,
		X:                snapshot.X,
		Y:                snapshot.Y,
		Z:                snapshot.Z,
		PickupRange:      snapshot.PickupRange,
	}
	if snapshot.GoldAmount != 0 {
		if snapshot.Count != 0 {
			return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has both item count and gold amount", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
		}
		if snapshot.Vnum != bootstrapGroundGoldVnum {
			return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has gold amount with non-gold vnum %d", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID, snapshot.Vnum)
		}
		if snapshot.GoldAmount > bootstrapGroundGoldMaxAmount {
			return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d gold amount %d exceeds migration bound", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID, snapshot.GoldAmount)
		}
		goldAmount := snapshot.GoldAmount
		row.GoldAmount = &goldAmount
		return row, nil
	}

	if snapshot.Count == 0 {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d has neither item count nor gold amount", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
	}
	if snapshot.Count > bootstrapGroundItemMaxCount {
		return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d item count %d exceeds migration bound", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID, snapshot.Count)
	}
	itemCount := snapshot.Count
	row.ItemCount = &itemCount
	return row, nil
}

func validBootstrapGroundOwnerMetadata(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r == 0 || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
