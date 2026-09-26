//go:build sqlite_harness

package minimal

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
)

func TestGameRuntimeOptInSQLStoreRematerializesOpenCheckinPasswordAndMoney(t *testing.T) {
	root := t.TempDir()
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	db := openSQLiteSafeboxOptInRuntimeDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, safeboxstore.CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("apply safebox schema: %v", err)
	}

	owner := peerVisibilityCharacter("SafeboxSQLOptIn", 7, 0x02040931, 1100, 2100, 0, 101, 201)
	owner.Gold = 5000
	owner.Inventory = []inventory.ItemInstance{{ID: 931, Vnum: 27001, Count: 2, Slot: 5}}
	login := "safebox-sql-optin"
	const loginKey uint32 = 0x92929331
	ticketStore := loginticket.NewFileStore(filepath.Join(root, "tickets"))
	accounts := accountstore.NewFileStore(filepath.Join(root, "accounts"))
	issuePeerTicket(t, ticketStore, login, loginKey, owner)
	account := accountstore.Account{Login: login, Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}
	if err := accounts.Save(account); err != nil {
		t.Fatalf("seed opt-in safebox account: %v", err)
	}
	roster, err := accountstore.ExportAccountCharacterRoster([]accountstore.Account{account})
	if err != nil {
		t.Fatalf("export opt-in safebox roster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(context.Background(), db, roster); err != nil {
		t.Fatalf("seed opt-in safebox SQL roster: %v", err)
	}

	fileStore := safeboxstore.NewFileStore(safeboxPath)
	seeded, err := safeboxstore.ReplaceCharacterCells(safeboxstore.Snapshot{}, login, owner.ID, map[uint8]inventory.ItemInstance{
		0: {ID: 11, Vnum: 11200, Count: 1},
	})
	if err != nil {
		t.Fatalf("seed file snapshot: %v", err)
	}
	if err := fileStore.Save(seeded); err != nil {
		t.Fatalf("save sibling FileStore: %v", err)
	}

	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenSafebox,
		Ref:  "npc:warehouse",
		Size: 1,
	}})
	itemStore := newItemTemplateStore(t, []itemcatalog.Template{{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}})
	cfg := config.Service{
		PprofAddr:        "127.0.0.1:6060",
		LegacyAddr:       ":13000",
		PublicAddr:       "127.0.0.1",
		SafeboxStorePath: safeboxPath,
	}
	sqlStore := safeboxstore.NewSQLStore(db)
	runtime, err := newGameRuntimeWithOptionalSafeboxSQL(cfg, ticketStore, accounts, nil, interactionStore, itemStore, nil, nil, sqlStore)
	if err != nil {
		t.Fatalf("unexpected opt-in SQL safebox runtime error: %v", err)
	}
	if _, ok := runtime.safeboxStore.(*safeboxstore.SQLStore); !ok {
		t.Fatalf("opt-in runtime store = %T, want *safeboxstore.SQLStore", runtime.safeboxStore)
	}
	stock, err := NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("unexpected stock runtime error: %v", err)
	}
	if _, ok := stock.safeboxStore.(*safeboxstore.FileStore); !ok {
		t.Fatalf("stock runtime store = %T, want *safeboxstore.FileStore", stock.safeboxStore)
	}

	currentTime := time.Unix(1700000931, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.RegisterStaticActorWithInteraction("Warehouse", bootstrapMapIndex, 1200, 2200, 20300, interactionstore.KindOpenSafebox, "npc:warehouse")
	if !ok {
		t.Fatal("expected warehouse registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), login, loginKey)
	defer closeSessionFlow(t, flow)
	_ = flushServerFrames(t, flow)

	openOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/open_safebox",
	})))
	if err != nil {
		t.Fatalf("unexpected /open_safebox on opt-in SQL runtime: %v", err)
	}
	if len(openOut) != 2 {
		t.Fatalf("expected empty SQL warehouse SAFEBOX_SIZE + SAFEBOX_MONEY_CHANGE, got %d", len(openOut))
	}
	size, err := itemproto.DecodeSafeboxSize(decodeSingleFrame(t, openOut[0]))
	if err != nil {
		t.Fatalf("decode opt-in open SAFEBOX_SIZE: %v", err)
	}
	if size != (itemproto.SafeboxSizePacket{Size: 1}) {
		t.Fatalf("unexpected opt-in open SAFEBOX_SIZE: %+v", size)
	}
	money, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, openOut[1]))
	if err != nil {
		t.Fatalf("decode opt-in open SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if money != (itemproto.SafeboxMoneyChangePacket{Money: 0}) {
		t.Fatalf("unexpected opt-in open SAFEBOX_MONEY_CHANGE: %+v", money)
	}

	checkinOut, err := flow.HandleClientFrame(decodeSingleFrame(t, itemproto.EncodeClientSafeboxCheckin(itemproto.ClientSafeboxCheckinPacket{
		SafeSlot: 1,
		Position: itemproto.InventoryPosition(5),
	})))
	if err != nil {
		t.Fatalf("unexpected opt-in SQL safebox check-in: %v", err)
	}
	if len(checkinOut) < 2 {
		t.Fatalf("expected opt-in SQL check-in frames, got %d", len(checkinOut))
	}
	set, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, checkinOut[len(checkinOut)-1]))
	if err != nil {
		t.Fatalf("decode opt-in SQL SAFEBOX_SET: %v", err)
	}
	if set.Position != (itemproto.Position{WindowType: itemproto.WindowSafebox, Cell: 1}) || set.Vnum != 27001 || set.Count != 2 {
		t.Fatalf("unexpected opt-in SQL SAFEBOX_SET: %+v", set)
	}

	changeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_change_password 000000 vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected opt-in SQL change-password: %v", err)
	}
	if len(changeOut) != 1 {
		t.Fatalf("expected one opt-in SQL change-password chat, got %d", len(changeOut))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, changeOut[0]))
	if err != nil {
		t.Fatalf("decode opt-in SQL change-password chat: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.Message != safeboxPasswordChangedInfoMessage {
		t.Fatalf("unexpected opt-in SQL change-password chat: %+v", delivery)
	}

	saveOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_money_save 1500",
	})))
	if err != nil {
		t.Fatalf("unexpected opt-in SQL money save: %v", err)
	}
	if len(saveOut) != 2 {
		t.Fatalf("expected gold point + SAFEBOX_MONEY_CHANGE on opt-in SQL save, got %d", len(saveOut))
	}
	moneyChange, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, saveOut[1]))
	if err != nil {
		t.Fatalf("decode opt-in SQL SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if moneyChange != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected opt-in SQL SAFEBOX_MONEY_CHANGE: %+v", moneyChange)
	}

	snapshot, err := safeboxstore.LoadOrEmpty(safeboxstore.NewSQLStore(db))
	if err != nil {
		t.Fatalf("reload opt-in SQL snapshot: %v", err)
	}
	cells := safeboxstore.CharacterCells(snapshot, login, owner.ID)
	if cells[1].Vnum != 27001 || cells[1].Count != 2 || cells[1].ID != 931 {
		t.Fatalf("opt-in SQL cell = %+v, want vnum 27001 count 2 id 931", cells[1])
	}
	if got := safeboxstore.CharacterPassword(snapshot, login, owner.ID); got != "vault2" {
		t.Fatalf("opt-in SQL password = %q, want vault2", got)
	}
	if got := safeboxstore.CharacterMoney(snapshot, login, owner.ID); got != 1500 {
		t.Fatalf("opt-in SQL money = %d, want 1500", got)
	}

	fileSnapshot, err := safeboxstore.LoadOrEmpty(fileStore)
	if err != nil {
		t.Fatalf("reload sibling FileStore: %v", err)
	}
	fileCells := safeboxstore.CharacterCells(fileSnapshot, login, owner.ID)
	if fileCells[0].Vnum != 11200 || fileCells[1].Vnum != 0 {
		t.Fatalf("opt-in SQL runtime wrote the sibling FileStore: %+v", fileCells)
	}

	assertCloseSafeboxCommandChat(t, flow, "/close_safebox", "opt-in SQL close before password reopen")
	currentTime = currentTime.Add(bootstrapSafeboxReopenCooldown)
	if _, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)}))); err != nil {
		t.Fatalf("unexpected warehouse prompt after opt-in SQL close: %v", err)
	}
	reopenOut, err := flow.HandleClientFrame(decodeSingleFrame(t, chatproto.EncodeClientChat(chatproto.ClientChatPacket{
		Type:    chatproto.ChatTypeTalking,
		Message: "/safebox_password vault2",
	})))
	if err != nil {
		t.Fatalf("unexpected opt-in SQL password reopen: %v", err)
	}
	if len(reopenOut) != 3 {
		t.Fatalf("expected SAFEBOX_SIZE + remembered SAFEBOX_SET + SAFEBOX_MONEY_CHANGE, got %d", len(reopenOut))
	}
	reopenSet, err := itemproto.DecodeSafeboxSet(decodeSingleFrame(t, reopenOut[1]))
	if err != nil {
		t.Fatalf("decode opt-in SQL reopen SAFEBOX_SET: %v", err)
	}
	if reopenSet.Vnum != 27001 || reopenSet.Count != 2 || reopenSet.Position.Cell != 1 {
		t.Fatalf("unexpected opt-in SQL reopen SAFEBOX_SET: %+v", reopenSet)
	}
	reopenMoney, err := itemproto.DecodeSafeboxMoneyChange(decodeSingleFrame(t, reopenOut[2]))
	if err != nil {
		t.Fatalf("decode opt-in SQL reopen SAFEBOX_MONEY_CHANGE: %v", err)
	}
	if reopenMoney != (itemproto.SafeboxMoneyChangePacket{Money: 1500}) {
		t.Fatalf("unexpected opt-in SQL reopen SAFEBOX_MONEY_CHANGE: %+v", reopenMoney)
	}
}

func openSQLiteSafeboxOptInRuntimeDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "safebox-optin-runtime.sqlite")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("ping sqlite opt-in safebox runtime: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
