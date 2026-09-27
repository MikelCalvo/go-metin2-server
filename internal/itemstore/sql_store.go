package itemstore

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

// SQLStore is the first live DB-backed item-template repository. It implements
// Store Load/Save and ItemTemplateStateExporter against the already-owned
// tip-0009 item_templates tables through a caller-supplied database/sql
// executor. Export identity stays tip-0009. Schema preflight requires the
// ledger entries for version 9 / item_template_refine_info plus additive 0021
// keep_on_fail and 0022 fail_result_vnum.
//
// Save replaces the entire authored snapshot: it deletes every tip-0009 parent
// and child row, then inserts the canonicalized snapshot. An empty snapshot
// clears those tables. That is the FileStore whole-snapshot posture, not
// insert-only import, not the opt-in per-vnum scoped replace, and not an upsert.
//
// The package does not select a driver, load a DSN, embed secrets, or register
// a production engine. Stock gamed rematerialize stays on FileStore unless a
// caller passes SQLStore into the explicit opt-in selector. Tip-0009 has no
// myshop_reject_message column. A snapshot that carries that FileStore-only
// text fails closed before any SQL mutation so the live repository cannot
// silently drop it.
type SQLStore struct {
	executor dbmigrations.SQLMigrationExecutor
}

var _ Store = (*SQLStore)(nil)
var _ ItemTemplateStateExporter = (*SQLStore)(nil)

// NewSQLStore returns an opt-in SQL item-template repository. A nil executor
// fails closed on Load/Save/Export rather than at construction.
func NewSQLStore(executor dbmigrations.SQLMigrationExecutor) *SQLStore {
	return &SQLStore{executor: executor}
}

// Load projects item_templates plus socket, attribute, use-effect, equip-effect,
// refine-info, and refine-material rows onto an authored Snapshot. An empty
// table is an empty snapshot, not a missing FileStore. Schema preflight
// requires tip-0009 plus additive 0021 and 0022. Rows that cannot reconstruct
// a valid authored template fail closed.
func (s *SQLStore) Load() (Snapshot, error) {
	if s == nil || itemTemplateStateImportExecutorIsNil(s.executor) {
		return Snapshot{}, ErrItemTemplateStateImportExecutorRequired
	}
	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin item-template SQL load transaction: %w", err)
	}
	snapshot, err := loadItemTemplateSnapshot(ctx, tx)
	if err != nil {
		return Snapshot{}, rollbackAfterItemTemplateStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit item-template SQL load transaction: %w", err)
	}
	return snapshot, nil
}

// Save replaces every tip-0009 item-template row with the supplied snapshot
// inside one transaction. A nil or empty template slice clears the tables.
// myshop_reject_message is rejected before the transaction opens.
func (s *SQLStore) Save(snapshot Snapshot) error {
	if s == nil || itemTemplateStateImportExecutorIsNil(s.executor) {
		return ErrItemTemplateStateImportExecutorRequired
	}
	normalized := normalizeSnapshot(snapshot)
	if err := validateSnapshot(normalized); err != nil {
		return fmt.Errorf("%w: validate item template snapshot", err)
	}
	if err := rejectMyShopRejectTextOutsideSQL(normalized); err != nil {
		return err
	}
	export, err := ExportItemTemplateState(normalized)
	if err != nil {
		return err
	}

	ctx := context.Background()
	tx, err := s.executor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin item-template SQL save transaction: %w", err)
	}
	if err := requireItemTemplateStateSchema(ctx, tx); err != nil {
		return rollbackAfterItemTemplateStateImportFailure(tx, err)
	}
	if err := replaceItemTemplateSnapshot(ctx, tx, export); err != nil {
		return rollbackAfterItemTemplateStateImportFailure(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit item-template SQL save transaction: %w", err)
	}
	return nil
}

// ExportItemTemplateState projects the committed SQL rows onto the tip-0009
// migration export. An empty table yields an empty export.
func (s *SQLStore) ExportItemTemplateState() (ItemTemplateStateExport, error) {
	snapshot, err := s.Load()
	if err != nil {
		return ItemTemplateStateExport{}, err
	}
	return ExportItemTemplateState(snapshot)
}

// SelectRematerializeStore chooses the item-template Load/Save seam. A nil SQL
// store keeps the FileStore. A non-nil SQL store is the explicit opt-in for
// that one caller. Nil file store with a nil opt-in stays nil. This does not
// select a driver, load a DSN, upsert, auto-run, or replace stock gamed
// rematerialize.
func SelectRematerializeStore(fileStore Store, sqlStore *SQLStore) Store {
	if sqlStore != nil {
		return sqlStore
	}
	return fileStore
}

func rejectMyShopRejectTextOutsideSQL(snapshot Snapshot) error {
	for _, template := range snapshot.Templates {
		if template.MyShopRejectText != "" {
			return fmt.Errorf("%w: template vnum %d myshop_reject_message is not a tip-0009 column", ErrInvalidSnapshot, template.Vnum)
		}
	}
	return nil
}

func replaceItemTemplateSnapshot(ctx context.Context, tx *sql.Tx, export ItemTemplateStateExport) error {
	// Child tables declare FOREIGN KEY (... ) REFERENCES item_templates(vnum)
	// without ON DELETE CASCADE, and refine materials reference refine infos.
	// Delete children before parents. This is the whole snapshot, not a vnum scope.
	statements := []struct {
		table string
		query string
	}{
		{"item_template_refine_materials", `DELETE FROM item_template_refine_materials`},
		{"item_template_refine_infos", `DELETE FROM item_template_refine_infos`},
		{"item_template_sockets", `DELETE FROM item_template_sockets`},
		{"item_template_attributes", `DELETE FROM item_template_attributes`},
		{"item_template_use_effects", `DELETE FROM item_template_use_effects`},
		{"item_template_equip_effects", `DELETE FROM item_template_equip_effects`},
		{"item_templates", `DELETE FROM item_templates`},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query); err != nil {
			return fmt.Errorf("delete %s: %w", statement.table, err)
		}
	}
	for _, row := range export.Templates {
		if err := insertItemTemplate(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.Sockets {
		if err := insertItemTemplateSocket(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.Attributes {
		if err := insertItemTemplateAttribute(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.UseEffects {
		if err := insertItemTemplateUseEffect(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.EquipEffects {
		if err := insertItemTemplateEquipEffect(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.RefineInfos {
		if err := insertItemTemplateRefineInfo(ctx, tx, row); err != nil {
			return err
		}
	}
	for _, row := range export.RefineMaterials {
		if err := insertItemTemplateRefineMaterial(ctx, tx, row); err != nil {
			return err
		}
	}
	return nil
}

func loadItemTemplateSnapshot(ctx context.Context, tx *sql.Tx) (Snapshot, error) {
	if err := requireItemTemplateStateSchema(ctx, tx); err != nil {
		return Snapshot{}, err
	}
	export, err := queryItemTemplateStateExport(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	canonical, _, err := QuarantineItemTemplateStateExport(export)
	if err != nil {
		return Snapshot{}, err
	}
	templates := make([]Template, 0, len(canonical.Templates))
	byVnum := make(map[uint32]Template, len(canonical.Templates))
	for _, row := range canonical.Templates {
		template, err := templateFromExportRow(row)
		if err != nil {
			return Snapshot{}, err
		}
		byVnum[template.Vnum] = template
	}
	for _, row := range canonical.Sockets {
		template := byVnum[row.Vnum]
		if int(row.Position) >= len(template.Sockets) {
			return Snapshot{}, fmt.Errorf("%w: socket position %d for vnum %d", ErrInvalidSnapshot, row.Position, row.Vnum)
		}
		template.Sockets[row.Position] = row.Value
		byVnum[row.Vnum] = template
	}
	for _, row := range canonical.Attributes {
		template := byVnum[row.Vnum]
		if int(row.Position) >= len(template.Attributes) {
			return Snapshot{}, fmt.Errorf("%w: attribute position %d for vnum %d", ErrInvalidSnapshot, row.Position, row.Vnum)
		}
		template.Attributes[row.Position] = Attribute{Type: row.Type, Value: row.Value}
		byVnum[row.Vnum] = template
	}
	for _, row := range canonical.UseEffects {
		template := byVnum[row.Vnum]
		template.UseEffect = &UseEffect{
			PointType:         row.PointType,
			PointIndex:        row.PointIndex,
			PointDelta:        row.PointDelta,
			ConsumeCount:      row.ConsumeCount,
			Message:           row.Message,
			InfoMessage:       row.InfoMessage,
			SpecialEffectType: row.SpecialEffectType,
		}
		byVnum[row.Vnum] = template
	}
	for _, row := range canonical.EquipEffects {
		template := byVnum[row.Vnum]
		template.EquipEffect = &PointEffect{
			PointType:  row.PointType,
			PointIndex: row.PointIndex,
			PointDelta: row.PointDelta,
		}
		byVnum[row.Vnum] = template
	}
	materialsByVnum := make(map[uint32][]RefineMaterial, len(canonical.RefineInfos))
	for _, row := range canonical.RefineMaterials {
		materials := materialsByVnum[row.Vnum]
		if int(row.Position) != len(materials) {
			return Snapshot{}, fmt.Errorf("%w: refine material position %d for vnum %d", ErrInvalidSnapshot, row.Position, row.Vnum)
		}
		materialsByVnum[row.Vnum] = append(materials, RefineMaterial{Vnum: row.ItemVnum, Count: row.Count})
	}
	for _, row := range canonical.RefineInfos {
		template := byVnum[row.Vnum]
		template.RefineInfo = &RefineInfo{
			ResultVnum:     row.ResultVnum,
			Cost:           row.Cost,
			Probability:    row.Probability,
			KeepOnFail:     row.KeepOnFail,
			FailResultVnum: row.FailResultVnum,
			Materials:      materialsByVnum[row.Vnum],
		}
		byVnum[row.Vnum] = template
	}
	for _, row := range canonical.Templates {
		templates = append(templates, byVnum[row.Vnum])
	}
	normalized := normalizeSnapshot(Snapshot{Templates: templates})
	if err := validateSnapshot(normalized); err != nil {
		return Snapshot{}, fmt.Errorf("%w: validate loaded item template snapshot", err)
	}
	return normalized, nil
}

func queryItemTemplateStateExport(ctx context.Context, tx *sql.Tx) (ItemTemplateStateExport, error) {
	export := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}

	templateRows, err := tx.QueryContext(ctx, `
SELECT vnum, name, stackable, max_count, shop_buy_price, shop_sell_price, refineable, refine_reject_message,
       save, sell_count_per_gold, slow_query, highlight, rare, unique_item, make_count, irremovable,
       confirm_when_use, quest_use, quest_use_multiple, log, applicable, appearance_vnum,
       anti_sell, anti_drop, anti_give, anti_stack, anti_get, anti_male, anti_female,
       anti_warrior, anti_assassin, anti_sura, anti_shaman, anti_empire_a, anti_empire_b, anti_empire_c,
       anti_save, anti_pk_drop, anti_myshop, anti_safebox, safebox_reject_message, min_level, equip_slot,
       use_reject_message, buy_reject_message, drop_reject_message, give_reject_message,
       pickup_reject_message, sell_reject_message, equip_reject_message, unequip_reject_message, pickup_range
FROM item_templates
ORDER BY vnum ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item templates: %w", err)
	}
	defer templateRows.Close()
	for templateRows.Next() {
		row, err := scanItemTemplateRow(templateRows)
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.Templates = append(export.Templates, row)
	}
	if err := templateRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item templates: %w", err)
	}

	socketRows, err := tx.QueryContext(ctx, `
SELECT vnum, position, value FROM item_template_sockets ORDER BY vnum ASC, position ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item template sockets: %w", err)
	}
	defer socketRows.Close()
	for socketRows.Next() {
		var vnum int64
		var position, value int64
		if err := socketRows.Scan(&vnum, &position, &value); err != nil {
			return ItemTemplateStateExport{}, fmt.Errorf("scan item template socket: %w", err)
		}
		rowVnum, err := sqlItemTemplateUint32(vnum, "socket vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPosition, err := sqlItemTemplateUint8(position, "socket position")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowValue, err := sqlItemTemplateInt32(value, "socket value")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.Sockets = append(export.Sockets, ItemTemplateSocketRow{Vnum: rowVnum, Position: rowPosition, Value: rowValue})
	}
	if err := socketRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item template sockets: %w", err)
	}

	attributeRows, err := tx.QueryContext(ctx, `
SELECT vnum, position, type, value FROM item_template_attributes ORDER BY vnum ASC, position ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item template attributes: %w", err)
	}
	defer attributeRows.Close()
	for attributeRows.Next() {
		var vnum, position, typ, value int64
		if err := attributeRows.Scan(&vnum, &position, &typ, &value); err != nil {
			return ItemTemplateStateExport{}, fmt.Errorf("scan item template attribute: %w", err)
		}
		rowVnum, err := sqlItemTemplateUint32(vnum, "attribute vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPosition, err := sqlItemTemplateUint8(position, "attribute position")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowType, err := sqlItemTemplateUint8(typ, "attribute type")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowValue, err := sqlItemTemplateInt16(value, "attribute value")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.Attributes = append(export.Attributes, ItemTemplateAttributeRow{Vnum: rowVnum, Position: rowPosition, Type: rowType, Value: rowValue})
	}
	if err := attributeRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item template attributes: %w", err)
	}

	useRows, err := tx.QueryContext(ctx, `
SELECT vnum, point_type, point_index, point_delta, consume_count, message, info_message, special_effect_type
FROM item_template_use_effects
ORDER BY vnum ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item template use effects: %w", err)
	}
	defer useRows.Close()
	for useRows.Next() {
		var (
			vnum, pointType, pointIndex, pointDelta, consumeCount, special int64
			message, infoMessage                                           string
		)
		if err := useRows.Scan(&vnum, &pointType, &pointIndex, &pointDelta, &consumeCount, &message, &infoMessage, &special); err != nil {
			return ItemTemplateStateExport{}, fmt.Errorf("scan item template use effect: %w", err)
		}
		rowVnum, err := sqlItemTemplateUint32(vnum, "use effect vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPointType, err := sqlItemTemplateUint8(pointType, "use effect point type")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPointIndex, err := sqlItemTemplateUint8(pointIndex, "use effect point index")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPointDelta, err := sqlItemTemplateInt32(pointDelta, "use effect point delta")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowConsume, err := sqlItemTemplateUint16(consumeCount, "use effect consume count")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowSpecial, err := sqlItemTemplateUint8(special, "use effect special type")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.UseEffects = append(export.UseEffects, ItemTemplateUseEffectRow{
			Vnum: rowVnum, PointType: rowPointType, PointIndex: rowPointIndex, PointDelta: rowPointDelta,
			ConsumeCount: rowConsume, Message: message, InfoMessage: infoMessage, SpecialEffectType: rowSpecial,
		})
	}
	if err := useRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item template use effects: %w", err)
	}

	equipRows, err := tx.QueryContext(ctx, `
SELECT vnum, point_type, point_index, point_delta FROM item_template_equip_effects ORDER BY vnum ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item template equip effects: %w", err)
	}
	defer equipRows.Close()
	for equipRows.Next() {
		var vnum, pointType, pointIndex, pointDelta int64
		if err := equipRows.Scan(&vnum, &pointType, &pointIndex, &pointDelta); err != nil {
			return ItemTemplateStateExport{}, fmt.Errorf("scan item template equip effect: %w", err)
		}
		rowVnum, err := sqlItemTemplateUint32(vnum, "equip effect vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPointType, err := sqlItemTemplateUint8(pointType, "equip effect point type")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPointIndex, err := sqlItemTemplateUint8(pointIndex, "equip effect point index")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPointDelta, err := sqlItemTemplateInt32(pointDelta, "equip effect point delta")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.EquipEffects = append(export.EquipEffects, ItemTemplateEquipEffectRow{
			Vnum: rowVnum, PointType: rowPointType, PointIndex: rowPointIndex, PointDelta: rowPointDelta,
		})
	}
	if err := equipRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item template equip effects: %w", err)
	}

	refineRows, err := tx.QueryContext(ctx, `
SELECT vnum, result_vnum, cost, probability, keep_on_fail, fail_result_vnum
FROM item_template_refine_infos
ORDER BY vnum ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item template refine infos: %w", err)
	}
	defer refineRows.Close()
	for refineRows.Next() {
		var vnum, resultVnum, cost, probability, keepOnFail, failResultVnum int64
		if err := refineRows.Scan(&vnum, &resultVnum, &cost, &probability, &keepOnFail, &failResultVnum); err != nil {
			return ItemTemplateStateExport{}, fmt.Errorf("scan item template refine info: %w", err)
		}
		rowVnum, err := sqlItemTemplateUint32(vnum, "refine info vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowResult, err := sqlItemTemplateUint32(resultVnum, "refine result vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowCost, err := sqlItemTemplateInt32(cost, "refine cost")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowProbability, err := sqlItemTemplateInt32(probability, "refine probability")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowKeep, err := sqlItemTemplateBool(keepOnFail, "keep_on_fail")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowFailResult, err := sqlItemTemplateUint32(failResultVnum, "fail result vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.RefineInfos = append(export.RefineInfos, ItemTemplateRefineInfoRow{
			Vnum: rowVnum, ResultVnum: rowResult, Cost: rowCost, Probability: rowProbability,
			KeepOnFail: rowKeep, FailResultVnum: rowFailResult,
		})
	}
	if err := refineRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item template refine infos: %w", err)
	}

	materialRows, err := tx.QueryContext(ctx, `
SELECT vnum, position, item_vnum, count FROM item_template_refine_materials ORDER BY vnum ASC, position ASC`)
	if err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("query item template refine materials: %w", err)
	}
	defer materialRows.Close()
	for materialRows.Next() {
		var vnum, position, itemVnum, count int64
		if err := materialRows.Scan(&vnum, &position, &itemVnum, &count); err != nil {
			return ItemTemplateStateExport{}, fmt.Errorf("scan item template refine material: %w", err)
		}
		rowVnum, err := sqlItemTemplateUint32(vnum, "refine material vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowPosition, err := sqlItemTemplateUint8(position, "refine material position")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowItemVnum, err := sqlItemTemplateUint32(itemVnum, "refine material item vnum")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		rowCount, err := sqlItemTemplateInt32(count, "refine material count")
		if err != nil {
			return ItemTemplateStateExport{}, err
		}
		export.RefineMaterials = append(export.RefineMaterials, ItemTemplateRefineMaterialRow{
			Vnum: rowVnum, Position: rowPosition, ItemVnum: rowItemVnum, Count: rowCount,
		})
	}
	if err := materialRows.Err(); err != nil {
		return ItemTemplateStateExport{}, fmt.Errorf("iterate item template refine materials: %w", err)
	}
	return export, nil
}

type itemTemplateRowScanner interface {
	Scan(dest ...any) error
}

func scanItemTemplateRow(scanner itemTemplateRowScanner) (ItemTemplateRow, error) {
	var (
		vnum, maxCount, shopBuy, shopSell, appearance, minLevel, pickup int64
		stackable, refineable, save, sellCount, slowQuery               int64
		highlight, rare, uniqueItem, makeCount, irremovable             int64
		confirmWhenUse, questUse, questUseMultiple, log, applicable     int64
		antiSell, antiDrop, antiGive, antiStack, antiGet                int64
		antiMale, antiFemale, antiWarrior, antiAssassin                 int64
		antiSura, antiShaman, antiEmpireA, antiEmpireB, antiEmpireC     int64
		antiSave, antiPKDrop, antiMyShop, antiSafebox                   int64
		name, refineReject, safeboxReject, equipSlot                    string
		useReject, buyReject, dropReject, giveReject                    string
		pickupReject, sellReject, equipReject, unequipReject            string
	)
	if err := scanner.Scan(
		&vnum, &name, &stackable, &maxCount, &shopBuy, &shopSell, &refineable, &refineReject,
		&save, &sellCount, &slowQuery, &highlight, &rare, &uniqueItem, &makeCount, &irremovable,
		&confirmWhenUse, &questUse, &questUseMultiple, &log, &applicable, &appearance,
		&antiSell, &antiDrop, &antiGive, &antiStack, &antiGet, &antiMale, &antiFemale,
		&antiWarrior, &antiAssassin, &antiSura, &antiShaman, &antiEmpireA, &antiEmpireB, &antiEmpireC,
		&antiSave, &antiPKDrop, &antiMyShop, &antiSafebox, &safeboxReject, &minLevel, &equipSlot,
		&useReject, &buyReject, &dropReject, &giveReject,
		&pickupReject, &sellReject, &equipReject, &unequipReject, &pickup,
	); err != nil {
		return ItemTemplateRow{}, fmt.Errorf("scan item template: %w", err)
	}
	rowVnum, err := sqlItemTemplateUint32(vnum, "vnum")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowMaxCount, err := sqlItemTemplateUint16(maxCount, "max_count")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowShopBuy, err := sqlItemTemplateUint64(shopBuy, "shop_buy_price")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowShopSell, err := sqlItemTemplateUint64(shopSell, "shop_sell_price")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAppearance, err := sqlItemTemplateUint32(appearance, "appearance_vnum")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowMinLevel, err := sqlItemTemplateUint8(minLevel, "min_level")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowPickup, err := sqlItemTemplateUint16(pickup, "pickup_range")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowStackable, err := sqlItemTemplateBool(stackable, "stackable")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowRefineable, err := sqlItemTemplateBool(refineable, "refineable")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowSave, err := sqlItemTemplateBool(save, "save")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowSellCount, err := sqlItemTemplateBool(sellCount, "sell_count_per_gold")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowSlowQuery, err := sqlItemTemplateBool(slowQuery, "slow_query")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowHighlight, err := sqlItemTemplateBool(highlight, "highlight")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowRare, err := sqlItemTemplateBool(rare, "rare")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowUnique, err := sqlItemTemplateBool(uniqueItem, "unique_item")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowMakeCount, err := sqlItemTemplateBool(makeCount, "make_count")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowIrremovable, err := sqlItemTemplateBool(irremovable, "irremovable")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowConfirm, err := sqlItemTemplateBool(confirmWhenUse, "confirm_when_use")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowQuestUse, err := sqlItemTemplateBool(questUse, "quest_use")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowQuestMultiple, err := sqlItemTemplateBool(questUseMultiple, "quest_use_multiple")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowLog, err := sqlItemTemplateBool(log, "log")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowApplicable, err := sqlItemTemplateBool(applicable, "applicable")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiSell, err := sqlItemTemplateBool(antiSell, "anti_sell")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiDrop, err := sqlItemTemplateBool(antiDrop, "anti_drop")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiGive, err := sqlItemTemplateBool(antiGive, "anti_give")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiStack, err := sqlItemTemplateBool(antiStack, "anti_stack")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiGet, err := sqlItemTemplateBool(antiGet, "anti_get")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiMale, err := sqlItemTemplateBool(antiMale, "anti_male")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiFemale, err := sqlItemTemplateBool(antiFemale, "anti_female")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiWarrior, err := sqlItemTemplateBool(antiWarrior, "anti_warrior")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiAssassin, err := sqlItemTemplateBool(antiAssassin, "anti_assassin")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiSura, err := sqlItemTemplateBool(antiSura, "anti_sura")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiShaman, err := sqlItemTemplateBool(antiShaman, "anti_shaman")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiEmpireA, err := sqlItemTemplateBool(antiEmpireA, "anti_empire_a")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiEmpireB, err := sqlItemTemplateBool(antiEmpireB, "anti_empire_b")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiEmpireC, err := sqlItemTemplateBool(antiEmpireC, "anti_empire_c")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiSave, err := sqlItemTemplateBool(antiSave, "anti_save")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiPKDrop, err := sqlItemTemplateBool(antiPKDrop, "anti_pk_drop")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiMyShop, err := sqlItemTemplateBool(antiMyShop, "anti_myshop")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	rowAntiSafebox, err := sqlItemTemplateBool(antiSafebox, "anti_safebox")
	if err != nil {
		return ItemTemplateRow{}, err
	}
	return ItemTemplateRow{
		Vnum:              rowVnum,
		Name:              name,
		Stackable:         rowStackable,
		MaxCount:          rowMaxCount,
		ShopBuyPrice:      rowShopBuy,
		ShopSellPrice:     rowShopSell,
		Refineable:        rowRefineable,
		RefineRejectText:  refineReject,
		Save:              rowSave,
		SellCountPerGold:  rowSellCount,
		SlowQuery:         rowSlowQuery,
		Highlight:         rowHighlight,
		Rare:              rowRare,
		Unique:            rowUnique,
		MakeCount:         rowMakeCount,
		Irremovable:       rowIrremovable,
		ConfirmWhenUse:    rowConfirm,
		QuestUse:          rowQuestUse,
		QuestUseMultiple:  rowQuestMultiple,
		Log:               rowLog,
		Applicable:        rowApplicable,
		AppearanceVnum:    rowAppearance,
		AntiSell:          rowAntiSell,
		AntiDrop:          rowAntiDrop,
		AntiGive:          rowAntiGive,
		AntiStack:         rowAntiStack,
		AntiGet:           rowAntiGet,
		AntiMale:          rowAntiMale,
		AntiFemale:        rowAntiFemale,
		AntiWarrior:       rowAntiWarrior,
		AntiAssassin:      rowAntiAssassin,
		AntiSura:          rowAntiSura,
		AntiShaman:        rowAntiShaman,
		AntiEmpireA:       rowAntiEmpireA,
		AntiEmpireB:       rowAntiEmpireB,
		AntiEmpireC:       rowAntiEmpireC,
		AntiSave:          rowAntiSave,
		AntiPKDrop:        rowAntiPKDrop,
		AntiMyShop:        rowAntiMyShop,
		AntiSafebox:       rowAntiSafebox,
		SafeboxRejectText: safeboxReject,
		MinLevel:          rowMinLevel,
		EquipSlot:         equipSlot,
		UseRejectText:     useReject,
		BuyRejectText:     buyReject,
		DropRejectText:    dropReject,
		GiveRejectText:    giveReject,
		PickupRejectText:  pickupReject,
		SellRejectText:    sellReject,
		EquipRejectText:   equipReject,
		UnequipRejectText: unequipReject,
		PickupRange:       rowPickup,
	}, nil
}

func sqlItemTemplateUint32(value int64, name string) (uint32, error) {
	if value < 0 || value > int64(math.MaxUint32) {
		return 0, fmt.Errorf("%w: %s %d is outside uint32", ErrInvalidSnapshot, name, value)
	}
	return uint32(value), nil
}

func sqlItemTemplateUint64(value int64, name string) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("%w: %s %d is negative", ErrInvalidSnapshot, name, value)
	}
	return uint64(value), nil
}

func sqlItemTemplateUint16(value int64, name string) (uint16, error) {
	if value < 0 || value > int64(math.MaxUint16) {
		return 0, fmt.Errorf("%w: %s %d is outside uint16", ErrInvalidSnapshot, name, value)
	}
	return uint16(value), nil
}

func sqlItemTemplateUint8(value int64, name string) (uint8, error) {
	if value < 0 || value > int64(math.MaxUint8) {
		return 0, fmt.Errorf("%w: %s %d is outside uint8", ErrInvalidSnapshot, name, value)
	}
	return uint8(value), nil
}

func sqlItemTemplateInt32(value int64, name string) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%w: %s %d is outside int32", ErrInvalidSnapshot, name, value)
	}
	return int32(value), nil
}

func sqlItemTemplateInt16(value int64, name string) (int16, error) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, fmt.Errorf("%w: %s %d is outside int16", ErrInvalidSnapshot, name, value)
	}
	return int16(value), nil
}

func sqlItemTemplateBool(value int64, name string) (bool, error) {
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("%w: %s %d is not 0 or 1", ErrInvalidSnapshot, name, value)
	}
}
