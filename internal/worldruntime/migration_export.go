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

	BootstrapGroundItemInstanceSocketsMigrationVersion = 26
	BootstrapGroundItemInstanceSocketsMigrationName    = "bootstrap_ground_item_instance_sockets"

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
// frozen by migration 0010, including optional additive 0026 instance sockets.
// HasSockets=false / omitted means nil instance sockets (template fallback);
// HasSockets=true including all-zero is authoritative. Gold-shaped rows stay
// socket-less. Export identity stays tip-0010.
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
	HasSockets       bool    `json:"has_sockets,omitempty"`
	Socket0          int32   `json:"socket0,omitempty"`
	Socket1          int32   `json:"socket1,omitempty"`
	Socket2          int32   `json:"socket2,omitempty"`
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
		if snapshot.HasSockets || snapshot.Socket0 != 0 || snapshot.Socket1 != 0 || snapshot.Socket2 != 0 {
			return BootstrapGroundItemStateRow{}, fmt.Errorf("%w: ground vid %d gold-shaped row must omit instance sockets", ErrInvalidBootstrapGroundItemStateExport, snapshot.VID)
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
	if err := validateBootstrapGroundItemInstanceSockets(snapshot.VID, snapshot.HasSockets, snapshot.Socket0, snapshot.Socket1, snapshot.Socket2); err != nil {
		return BootstrapGroundItemStateRow{}, err
	}
	itemCount := snapshot.Count
	row.ItemCount = &itemCount
	row.HasSockets = snapshot.HasSockets
	row.Socket0 = snapshot.Socket0
	row.Socket1 = snapshot.Socket1
	row.Socket2 = snapshot.Socket2
	return row, nil
}

func validateBootstrapGroundItemInstanceSockets(vid uint32, hasSockets bool, socket0, socket1, socket2 int32) error {
	if hasSockets {
		return nil
	}
	if socket0 != 0 || socket1 != 0 || socket2 != 0 {
		return fmt.Errorf("%w: ground vid %d has non-zero sockets without has_sockets", ErrInvalidBootstrapGroundItemStateExport, vid)
	}
	return nil
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
