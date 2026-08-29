//go:build sqlite_harness

package itemstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

func TestSQLiteHarnessItemTemplateStateImportInsertsTemplatesAndChildren(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}

	snapshot := Snapshot{Templates: []Template{
		{
			Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200,
			ShopBuyPrice: 50, ShopSellPrice: 13, Highlight: true, Unique: true,
			AntiSell: true, AntiGet: true, AntiSafebox: true, PickupRange: 750,
			Sockets: SocketValues{1, 2, 3}, Attributes: AttributeValues{{Type: 1, Value: 10}},
			UseEffect: &UseEffect{
				PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 2,
				Message: "Recovered HP", InfoMessage: "You feel better.", SpecialEffectType: 3,
			},
			UseRejectText: "You cannot use this yet.", BuyRejectText: "The merchant will not sell this.",
			DropRejectText: "You cannot drop this.", PickupRejectText: "You cannot pick this up.",
			SellRejectText: "The merchant refuses this.", SafeboxRejectText: "You cannot store this.",
		},
		{
			Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1,
			Refineable: true, Save: true, Irremovable: true, AntiMale: true,
			AppearanceVnum: 11201, EquipSlot: "weapon",
			RefineInfo: &RefineInfo{
				ResultVnum: 11201, Cost: 2500, Probability: 75, KeepOnFail: true,
				Materials: []RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}},
			},
			EquipEffect:       &PointEffect{PointType: 1, PointIndex: 0, PointDelta: 4},
			EquipRejectText:   "You cannot wield this.",
			UnequipRejectText: "You cannot remove this.",
		},
		{
			Vnum: 11199, Name: "Downgrade Blade", Stackable: false, MaxCount: 1,
		},
		{
			Vnum: 11300, Name: "Downgrade Source Blade", Stackable: false, MaxCount: 1,
			Refineable: true,
			RefineInfo: &RefineInfo{
				ResultVnum: 11301, Cost: 1800, Probability: 60, FailResultVnum: 11199,
				Materials: []RefineMaterial{{Vnum: 27001, Count: 1}},
			},
		},
	}}

	export, err := ExportItemTemplateState(snapshot)
	if err != nil {
		t.Fatalf("ExportItemTemplateState: %v", err)
	}

	result, err := ImportItemTemplateState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportItemTemplateState: %v", err)
	}
	if result.MigrationVersion != ItemTemplateStateMigrationVersion || result.MigrationName != ItemTemplateStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.TemplateCount != 4 || result.SocketCount != 3 || result.AttributeCount != 1 ||
		result.UseEffectCount != 1 || result.EquipEffectCount != 1 ||
		result.RefineInfoCount != 2 || result.RefineMaterialCount != 3 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.Vnums) != 4 || result.Vnums[0] != 11199 || result.Vnums[1] != 11200 || result.Vnums[2] != 11300 || result.Vnums[3] != 27001 {
		t.Fatalf("unexpected vnums: %+v", result.Vnums)
	}

	assertItemTemplateRow(t, db, 11200, "Wooden Sword", 0, 1, 0, 0, 1, 1, 1, 11201, 1, "", "weapon",
		"", "", "", "", "", "", "You cannot wield this.", "You cannot remove this.", 0)
	assertItemTemplateRow(t, db, 27001, "Small Red Potion", 1, 200, 50, 13, 0, 0, 0, 0, 0, "You cannot store this.", "",
		"You cannot use this yet.", "The merchant will not sell this.", "You cannot drop this.", "",
		"You cannot pick this up.", "The merchant refuses this.", "", "", 750)

	assertItemTemplateSocket(t, db, 27001, 0, 1)
	assertItemTemplateSocket(t, db, 27001, 1, 2)
	assertItemTemplateSocket(t, db, 27001, 2, 3)
	assertItemTemplateAttribute(t, db, 27001, 0, 1, 10)
	assertItemTemplateUseEffect(t, db, 27001, 7, 1, 25, 2, "Recovered HP", "You feel better.", 3)
	assertItemTemplateEquipEffect(t, db, 11200, 1, 0, 4)
	assertItemTemplateRefineInfo(t, db, 11200, 11201, 2500, 75, true, 0)
	assertItemTemplateRefineInfo(t, db, 11300, 11301, 1800, 60, false, 11199)
	assertItemTemplateRefineMaterial(t, db, 11200, 0, 27001, 2)
	assertItemTemplateRefineMaterial(t, db, 11200, 1, 27002, 3)
	assertItemTemplateRefineMaterial(t, db, 11300, 0, 27001, 1)
}

func TestSQLiteHarnessItemTemplateStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}

	export, err := ExportItemTemplateState(Snapshot{Templates: []Template{{
		Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200,
	}}})
	if err != nil {
		t.Fatalf("ExportItemTemplateState: %v", err)
	}
	if _, err := ImportItemTemplateState(ctx, db, export); err != nil {
		t.Fatalf("first ImportItemTemplateState: %v", err)
	}

	_, err = ImportItemTemplateState(ctx, db, export)
	if err == nil {
		t.Fatal("second ImportItemTemplateState succeeded, want unique conflict")
	}

	var rowCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_templates`).Scan(&rowCount); err != nil {
		t.Fatalf("count templates after failed reimport: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("template rows after failed reimport = %d, want 1 (no partial second import)", rowCount)
	}
}

func TestSQLiteHarnessItemTemplateStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
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

	_, err := ImportItemTemplateState(ctx, db, export)
	if !errors.Is(err, ErrItemTemplateStateImportSchemaRequired) {
		t.Fatalf("ImportItemTemplateState on empty DB error = %v, want %v", err, ErrItemTemplateStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessItemTemplateStateImportRejectsTip0009WithoutKeepOnFailSchema(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateStateMigrationVersion, err)
	}

	export, err := ExportItemTemplateState(Snapshot{Templates: []Template{{
		Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, Refineable: true,
		RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, KeepOnFail: true},
	}}})
	if err != nil {
		t.Fatalf("ExportItemTemplateState: %v", err)
	}

	_, err = ImportItemTemplateState(ctx, db, export)
	if !errors.Is(err, ErrItemTemplateStateImportSchemaRequired) {
		t.Fatalf("ImportItemTemplateState tip-0009-only error = %v, want %v", err, ErrItemTemplateStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "21") || !strings.Contains(err.Error(), ItemTemplateRefineKeepOnFailMigrationName) {
		t.Fatalf("expected tip-0009-only reject to name keep_on_fail schema 21, got %v", err)
	}
}

func TestSQLiteHarnessItemTemplateStateImportRejectsTip0021WithoutFailResultSchema(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineKeepOnFailMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineKeepOnFailMigrationVersion, err)
	}

	export, err := ExportItemTemplateState(Snapshot{Templates: []Template{{
		Vnum: 11200, Name: "Wooden Sword", Stackable: false, MaxCount: 1, Refineable: true,
		RefineInfo: &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, FailResultVnum: 11199},
	}}})
	if err != nil {
		t.Fatalf("ExportItemTemplateState: %v", err)
	}

	_, err = ImportItemTemplateState(ctx, db, export)
	if !errors.Is(err, ErrItemTemplateStateImportSchemaRequired) {
		t.Fatalf("ImportItemTemplateState tip-0021-only error = %v, want %v", err, ErrItemTemplateStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "22") || !strings.Contains(err.Error(), ItemTemplateRefineFailResultVnumMigrationName) {
		t.Fatalf("expected tip-0021-only reject to name fail_result_vnum schema 22, got %v", err)
	}
}

func TestSQLiteHarnessItemTemplateStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteItemTemplateStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, ItemTemplateRefineFailResultVnumMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", ItemTemplateRefineFailResultVnumMigrationVersion, err)
	}

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
	result, err := ImportItemTemplateState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportItemTemplateState(empty): %v", err)
	}
	if result.TemplateCount != 0 || result.SocketCount != 0 || result.AttributeCount != 0 ||
		result.UseEffectCount != 0 || result.EquipEffectCount != 0 ||
		result.RefineInfoCount != 0 || result.RefineMaterialCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.Vnums) != 0 {
		t.Fatalf("empty import vnums = %+v, want empty", result.Vnums)
	}
}

func assertItemTemplateRow(
	t *testing.T,
	db *sql.DB,
	vnum uint32,
	name string,
	stackable, maxCount int,
	shopBuy, shopSell int64,
	refineable, save, irremovable int,
	appearanceVnum int64,
	antiMale int,
	safeboxReject, equipSlot string,
	useReject, buyReject, dropReject, giveReject, pickupReject, sellReject, equipReject, unequipReject string,
	pickupRange int,
) {
	t.Helper()

	var (
		gotVnum                                                                                                                             int64
		gotName                                                                                                                             string
		gotStackable, gotMaxCount, gotRefineable, gotSave, gotHighlight, gotUnique, gotIrremovable, gotAntiSell, gotAntiGet, gotAntiSafebox int
		gotAntiMale                                                                                                                         int
		gotShopBuy, gotShopSell, gotAppearance                                                                                              int64
		gotSafeboxReject, gotEquipSlot                                                                                                      string
		gotUseReject, gotBuyReject, gotDropReject, gotGiveReject, gotPickupReject, gotSellReject, gotEquipReject, gotUnequipReject          string
		gotPickupRange                                                                                                                      int
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, name, stackable, max_count, shop_buy_price, shop_sell_price, refineable, save, highlight, unique_item,
       irremovable, appearance_vnum, anti_sell, anti_get, anti_male, anti_safebox, safebox_reject_message, equip_slot,
       use_reject_message, buy_reject_message, drop_reject_message, give_reject_message, pickup_reject_message,
       sell_reject_message, equip_reject_message, unequip_reject_message, pickup_range
FROM item_templates WHERE vnum = ?`, vnum).Scan(
		&gotVnum, &gotName, &gotStackable, &gotMaxCount, &gotShopBuy, &gotShopSell, &gotRefineable, &gotSave,
		&gotHighlight, &gotUnique, &gotIrremovable, &gotAppearance, &gotAntiSell, &gotAntiGet, &gotAntiMale,
		&gotAntiSafebox, &gotSafeboxReject, &gotEquipSlot, &gotUseReject, &gotBuyReject, &gotDropReject,
		&gotGiveReject, &gotPickupReject, &gotSellReject, &gotEquipReject, &gotUnequipReject, &gotPickupRange,
	); err != nil {
		t.Fatalf("select item template vnum %d: %v", vnum, err)
	}
	if gotVnum != int64(vnum) || gotName != name || gotStackable != stackable || gotMaxCount != maxCount ||
		gotShopBuy != shopBuy || gotShopSell != shopSell || gotRefineable != refineable || gotSave != save ||
		gotIrremovable != irremovable || gotAppearance != appearanceVnum || gotAntiMale != antiMale ||
		gotSafeboxReject != safeboxReject || gotEquipSlot != equipSlot || gotUseReject != useReject ||
		gotBuyReject != buyReject || gotDropReject != dropReject || gotGiveReject != giveReject ||
		gotPickupReject != pickupReject || gotSellReject != sellReject || gotEquipReject != equipReject ||
		gotUnequipReject != unequipReject || gotPickupRange != pickupRange {
		t.Fatalf("item template row mismatch for vnum %d: got name=%q stackable=%d max=%d buy=%d sell=%d refineable=%d save=%d irremovable=%d appearance=%d anti_male=%d safebox=%q equip_slot=%q use=%q buy=%q drop=%q give=%q pickup=%q sell=%q equip=%q unequip=%q pickup_range=%d",
			vnum, gotName, gotStackable, gotMaxCount, gotShopBuy, gotShopSell, gotRefineable, gotSave, gotIrremovable,
			gotAppearance, gotAntiMale, gotSafeboxReject, gotEquipSlot, gotUseReject, gotBuyReject, gotDropReject,
			gotGiveReject, gotPickupReject, gotSellReject, gotEquipReject, gotUnequipReject, gotPickupRange)
	}
	if vnum == 27001 {
		if gotHighlight != 1 || gotUnique != 1 || gotAntiSell != 1 || gotAntiGet != 1 || gotAntiSafebox != 1 {
			t.Fatalf("potion flags mismatch: highlight=%d unique=%d anti_sell=%d anti_get=%d anti_safebox=%d",
				gotHighlight, gotUnique, gotAntiSell, gotAntiGet, gotAntiSafebox)
		}
	}
}

func assertItemTemplateSocket(t *testing.T, db *sql.DB, vnum uint32, position uint8, value int32) {
	t.Helper()
	var gotVnum int64
	var gotPosition, gotValue int
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, position, value FROM item_template_sockets WHERE vnum = ? AND position = ?`,
		vnum, position).Scan(&gotVnum, &gotPosition, &gotValue); err != nil {
		t.Fatalf("select socket vnum %d position %d: %v", vnum, position, err)
	}
	if gotVnum != int64(vnum) || gotPosition != int(position) || gotValue != int(value) {
		t.Fatalf("socket mismatch: got vnum=%d position=%d value=%d", gotVnum, gotPosition, gotValue)
	}
}

func assertItemTemplateAttribute(t *testing.T, db *sql.DB, vnum uint32, position, typ uint8, value int16) {
	t.Helper()
	var gotVnum int64
	var gotPosition, gotType, gotValue int
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, position, type, value FROM item_template_attributes WHERE vnum = ? AND position = ?`,
		vnum, position).Scan(&gotVnum, &gotPosition, &gotType, &gotValue); err != nil {
		t.Fatalf("select attribute vnum %d position %d: %v", vnum, position, err)
	}
	if gotVnum != int64(vnum) || gotPosition != int(position) || gotType != int(typ) || gotValue != int(value) {
		t.Fatalf("attribute mismatch: got vnum=%d position=%d type=%d value=%d", gotVnum, gotPosition, gotType, gotValue)
	}
}

func assertItemTemplateUseEffect(t *testing.T, db *sql.DB, vnum uint32, pointType, pointIndex uint8, pointDelta int32, consumeCount uint16, message, infoMessage string, specialEffectType uint8) {
	t.Helper()
	var (
		gotVnum                                                            int64
		gotPointType, gotPointIndex, gotPointDelta, gotConsume, gotSpecial int
		gotMessage, gotInfo                                                string
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, point_type, point_index, point_delta, consume_count, message, info_message, special_effect_type
FROM item_template_use_effects WHERE vnum = ?`, vnum).Scan(
		&gotVnum, &gotPointType, &gotPointIndex, &gotPointDelta, &gotConsume, &gotMessage, &gotInfo, &gotSpecial,
	); err != nil {
		t.Fatalf("select use effect vnum %d: %v", vnum, err)
	}
	if gotVnum != int64(vnum) || gotPointType != int(pointType) || gotPointIndex != int(pointIndex) ||
		gotPointDelta != int(pointDelta) || gotConsume != int(consumeCount) || gotMessage != message ||
		gotInfo != infoMessage || gotSpecial != int(specialEffectType) {
		t.Fatalf("use effect mismatch for vnum %d: %+v/%+v/%+v/%+v/%q/%q/%+v",
			vnum, gotPointType, gotPointIndex, gotPointDelta, gotConsume, gotMessage, gotInfo, gotSpecial)
	}
}

func assertItemTemplateEquipEffect(t *testing.T, db *sql.DB, vnum uint32, pointType, pointIndex uint8, pointDelta int32) {
	t.Helper()
	var gotVnum int64
	var gotPointType, gotPointIndex, gotPointDelta int
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, point_type, point_index, point_delta FROM item_template_equip_effects WHERE vnum = ?`,
		vnum).Scan(&gotVnum, &gotPointType, &gotPointIndex, &gotPointDelta); err != nil {
		t.Fatalf("select equip effect vnum %d: %v", vnum, err)
	}
	if gotVnum != int64(vnum) || gotPointType != int(pointType) || gotPointIndex != int(pointIndex) || gotPointDelta != int(pointDelta) {
		t.Fatalf("equip effect mismatch for vnum %d", vnum)
	}
}

func assertItemTemplateRefineInfo(t *testing.T, db *sql.DB, vnum, resultVnum uint32, cost, probability int32, keepOnFail bool, failResultVnum uint32) {
	t.Helper()
	var gotVnum, gotResult, gotFailResult int64
	var gotCost, gotProbability, gotKeepOnFail int
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, result_vnum, cost, probability, keep_on_fail, fail_result_vnum FROM item_template_refine_infos WHERE vnum = ?`,
		vnum).Scan(&gotVnum, &gotResult, &gotCost, &gotProbability, &gotKeepOnFail, &gotFailResult); err != nil {
		t.Fatalf("select refine info vnum %d: %v", vnum, err)
	}
	wantKeep := 0
	if keepOnFail {
		wantKeep = 1
	}
	if gotVnum != int64(vnum) || gotResult != int64(resultVnum) || gotCost != int(cost) || gotProbability != int(probability) || gotKeepOnFail != wantKeep || gotFailResult != int64(failResultVnum) {
		t.Fatalf("refine info mismatch for vnum %d: got keep_on_fail=%d fail_result_vnum=%d want keep=%d fail=%d", vnum, gotKeepOnFail, gotFailResult, wantKeep, failResultVnum)
	}
}

func assertItemTemplateRefineMaterial(t *testing.T, db *sql.DB, vnum uint32, position uint8, itemVnum uint32, count int32) {
	t.Helper()
	var gotVnum, gotItemVnum int64
	var gotPosition, gotCount int
	if err := db.QueryRowContext(context.Background(), `
SELECT vnum, position, item_vnum, count FROM item_template_refine_materials WHERE vnum = ? AND position = ?`,
		vnum, position).Scan(&gotVnum, &gotPosition, &gotItemVnum, &gotCount); err != nil {
		t.Fatalf("select refine material vnum %d position %d: %v", vnum, position, err)
	}
	if gotVnum != int64(vnum) || gotPosition != int(position) || gotItemVnum != int64(itemVnum) || gotCount != int(count) {
		t.Fatalf("refine material mismatch for vnum %d position %d", vnum, position)
	}
}

func openSQLiteItemTemplateStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "item-template-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite item-template-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
