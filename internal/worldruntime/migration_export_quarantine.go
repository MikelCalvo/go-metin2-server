package worldruntime

import (
	"fmt"
)

// BootstrapGroundItemStateQuarantineSummary is the metadata-only result of
// validating or quarantining a retained bootstrap ground-item-state export. It
// never includes SQL, DSNs, or live world payloads beyond deterministic counts
// and visible ground VIDs.
type BootstrapGroundItemStateQuarantineSummary struct {
	GroundItemCount int      `json:"ground_item_count"`
	ItemShapedCount int      `json:"item_shaped_count"`
	GoldShapedCount int      `json:"gold_shaped_count"`
	VIDs            []uint32 `json:"vids"`
}

// BootstrapGroundItemStateQuarantineResult pairs the metadata-only quarantine
// summary with a canonicalized export ready for later offline review or
// backfill tools.
type BootstrapGroundItemStateQuarantineResult struct {
	Summary BootstrapGroundItemStateQuarantineSummary `json:"summary"`
	Export  BootstrapGroundItemStateExport            `json:"export"`
}

// ValidateBootstrapGroundItemStateExport fails closed when a retained export
// does not match the 0010_bootstrap_ground_item_state shape. It does not open a
// database, mutate live ground handles, or mutate the supplied export.
func ValidateBootstrapGroundItemStateExport(export BootstrapGroundItemStateExport) (BootstrapGroundItemStateQuarantineSummary, error) {
	canonical, summary, err := canonicalizeBootstrapGroundItemStateExport(export)
	if err != nil {
		return BootstrapGroundItemStateQuarantineSummary{}, err
	}
	_ = canonical
	return summary, nil
}

// QuarantineBootstrapGroundItemStateExport validates a retained export and
// returns a canonicalized copy ordered exactly like ExportBootstrapGroundItemState.
// It never opens a database or mutates live ground handles.
func QuarantineBootstrapGroundItemStateExport(export BootstrapGroundItemStateExport) (BootstrapGroundItemStateExport, BootstrapGroundItemStateQuarantineSummary, error) {
	return canonicalizeBootstrapGroundItemStateExport(export)
}

func canonicalizeBootstrapGroundItemStateExport(export BootstrapGroundItemStateExport) (BootstrapGroundItemStateExport, BootstrapGroundItemStateQuarantineSummary, error) {
	if export.MigrationVersion != BootstrapGroundItemStateMigrationVersion {
		return BootstrapGroundItemStateExport{}, BootstrapGroundItemStateQuarantineSummary{}, fmt.Errorf("%w: migration_version %d", ErrInvalidBootstrapGroundItemStateExport, export.MigrationVersion)
	}
	if export.MigrationName != BootstrapGroundItemStateMigrationName {
		return BootstrapGroundItemStateExport{}, BootstrapGroundItemStateQuarantineSummary{}, fmt.Errorf("%w: migration_name %q", ErrInvalidBootstrapGroundItemStateExport, export.MigrationName)
	}
	if export.GroundItems == nil {
		return BootstrapGroundItemStateExport{}, BootstrapGroundItemStateQuarantineSummary{}, fmt.Errorf("%w: ground_items must be present", ErrInvalidBootstrapGroundItemStateExport)
	}

	snapshots := make([]GroundItemSnapshot, 0, len(export.GroundItems))
	for _, row := range export.GroundItems {
		snapshot, err := groundItemSnapshotFromExportRow(row)
		if err != nil {
			return BootstrapGroundItemStateExport{}, BootstrapGroundItemStateQuarantineSummary{}, err
		}
		snapshots = append(snapshots, snapshot)
	}

	canonical, err := ExportBootstrapGroundItemState(snapshots)
	if err != nil {
		return BootstrapGroundItemStateExport{}, BootstrapGroundItemStateQuarantineSummary{}, err
	}

	itemShapedCount := 0
	goldShapedCount := 0
	vids := make([]uint32, 0, len(canonical.GroundItems))
	for _, row := range canonical.GroundItems {
		vids = append(vids, row.VID)
		switch {
		case row.ItemCount != nil:
			itemShapedCount++
		case row.GoldAmount != nil:
			goldShapedCount++
		}
	}

	summary := BootstrapGroundItemStateQuarantineSummary{
		GroundItemCount: len(canonical.GroundItems),
		ItemShapedCount: itemShapedCount,
		GoldShapedCount: goldShapedCount,
		VIDs:            vids,
	}
	if summary.VIDs == nil {
		summary.VIDs = []uint32{}
	}
	return canonical, summary, nil
}

func groundItemSnapshotFromExportRow(row BootstrapGroundItemStateRow) (GroundItemSnapshot, error) {
	if err := validateBootstrapGroundItemInstanceAttributes(
		row.VID,
		row.HasAttributes,
		row.Attr0Type, row.Attr0Value,
		row.Attr1Type, row.Attr1Value,
		row.Attr2Type, row.Attr2Value,
		row.Attr3Type, row.Attr3Value,
		row.Attr4Type, row.Attr4Value,
		row.Attr5Type, row.Attr5Value,
		row.Attr6Type, row.Attr6Value,
	); err != nil {
		return GroundItemSnapshot{}, err
	}
	snapshot := GroundItemSnapshot{
		VID:              row.VID,
		Vnum:             row.Vnum,
		OwnerLogin:       row.OwnerLogin,
		OwnerCharacterID: row.OwnerCharacterID,
		OwnerVID:         row.OwnerVID,
		OwnerName:        row.OwnerName,
		MapIndex:         row.MapIndex,
		X:                row.X,
		Y:                row.Y,
		Z:                row.Z,
		PickupRange:      row.PickupRange,
		HasSockets:       row.HasSockets,
		Socket0:          row.Socket0,
		Socket1:          row.Socket1,
		Socket2:          row.Socket2,
		HasAttributes:    row.HasAttributes,
		Attr0Type:        row.Attr0Type,
		Attr0Value:       row.Attr0Value,
		Attr1Type:        row.Attr1Type,
		Attr1Value:       row.Attr1Value,
		Attr2Type:        row.Attr2Type,
		Attr2Value:       row.Attr2Value,
		Attr3Type:        row.Attr3Type,
		Attr3Value:       row.Attr3Value,
		Attr4Type:        row.Attr4Type,
		Attr4Value:       row.Attr4Value,
		Attr5Type:        row.Attr5Type,
		Attr5Value:       row.Attr5Value,
		Attr6Type:        row.Attr6Type,
		Attr6Value:       row.Attr6Value,
	}
	switch {
	case row.ItemCount != nil && row.GoldAmount != nil:
		return GroundItemSnapshot{}, fmt.Errorf("%w: ground vid %d has both item count and gold amount", ErrInvalidBootstrapGroundItemStateExport, row.VID)
	case row.ItemCount != nil:
		snapshot.Count = *row.ItemCount
	case row.GoldAmount != nil:
		snapshot.GoldAmount = *row.GoldAmount
	}
	return snapshot, nil
}
