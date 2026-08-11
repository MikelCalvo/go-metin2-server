package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestExportAccountCharacterRosterBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	alpha := Account{
		Login:  "Alpha",
		Empire: 1,
		Characters: []loginticket.Character{
			{
				ID:          11,
				VID:         0x02040111,
				Name:        "AlphaWar",
				Job:         0,
				RaceNum:     4,
				Level:       5,
				PlayMinutes: 42,
				ST:          6,
				HT:          7,
				DX:          8,
				IQ:          9,
				MainPart:    11200,
				ChangeName:  1,
				HairPart:    100,
				X:           111,
				Y:           222,
				Z:           333,
				MapIndex:    1,
				Empire:      1,
				SkillGroup:  2,
				GuildID:     99,
				GuildName:   "GuildA",
				Gold:        1234,
				Inventory:   []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 8}},
			},
			{},
			{},
			rosterExportCharacter(12, "AlphaSura"),
		},
	}
	bravo := Account{
		Login:  "Bravo",
		Empire: 2,
		Characters: []loginticket.Character{
			{},
			{
				ID:       22,
				Name:     "BravoNinja",
				Job:      1,
				RaceNum:  1,
				Level:    7,
				X:        1700,
				Y:        2800,
				MapIndex: 42,
				Empire:   2,
				Gold:     500,
			},
		},
	}

	export, err := ExportAccountCharacterRoster([]Account{bravo, alpha})
	if err != nil {
		t.Fatalf("export account/character roster: %v", err)
	}

	if export.MigrationVersion != 2 || export.MigrationName != "account_character_roster" {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	if len(export.Accounts) != 2 {
		t.Fatalf("expected two account rows, got %#v", export.Accounts)
	}
	if export.Accounts[0].Login != "Alpha" || export.Accounts[0].LoginNormalized != "alpha" || export.Accounts[0].Empire != 1 {
		t.Fatalf("unexpected first account row: %#v", export.Accounts[0])
	}
	if export.Accounts[1].Login != "Bravo" || export.Accounts[1].LoginNormalized != "bravo" || export.Accounts[1].Empire != 2 {
		t.Fatalf("unexpected second account row: %#v", export.Accounts[1])
	}
	if export.Accounts[0].ID <= 0 || export.Accounts[1].ID <= 0 || export.Accounts[0].ID == export.Accounts[1].ID {
		t.Fatalf("expected stable positive distinct account ids, got %#v", export.Accounts)
	}

	if len(export.Characters) != 3 {
		t.Fatalf("expected three non-empty character rows, got %#v", export.Characters)
	}
	wantFirstCharacter := AccountCharacterRosterCharacterRow{
		ID:             11,
		AccountID:      export.Accounts[0].ID,
		Slot:           0,
		Name:           "AlphaWar",
		NameNormalized: "alphawar",
		Job:            0,
		RaceNum:        4,
		Level:          5,
		PlayMinutes:    42,
		ST:             6,
		HT:             7,
		DX:             8,
		IQ:             9,
		MainPart:       11200,
		ChangeName:     1,
		HairPart:       100,
		X:              111,
		Y:              222,
		Z:              333,
		MapIndex:       1,
		Empire:         1,
		SkillGroup:     2,
		GuildID:        99,
		GuildName:      "GuildA",
		Gold:           1234,
	}
	if !reflect.DeepEqual(export.Characters[0], wantFirstCharacter) {
		t.Fatalf("unexpected first character row:\n got: %#v\nwant: %#v", export.Characters[0], wantFirstCharacter)
	}
	if export.Characters[1].ID != 12 || export.Characters[1].AccountID != export.Accounts[0].ID || export.Characters[1].Slot != 3 || export.Characters[1].Name != "AlphaSura" {
		t.Fatalf("expected alpha slot 3 character before bravo rows, got %#v", export.Characters[1])
	}
	if export.Characters[2].ID != 22 || export.Characters[2].AccountID != export.Accounts[1].ID || export.Characters[2].Slot != 1 || export.Characters[2].NameNormalized != "bravoninja" {
		t.Fatalf("unexpected bravo character row: %#v", export.Characters[2])
	}

	exportAgain, err := ExportAccountCharacterRoster([]Account{bravo, alpha})
	if err != nil {
		t.Fatalf("export account/character roster again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic export rows:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportAccountCharacterRosterRejectsStateThatWouldViolateMigrationSchema(t *testing.T) {
	const aboveSignedBigInt = uint64(1 << 63)

	tooMuchGold := rosterExportCharacter(1, "RichWar")
	tooMuchGold.Gold = aboveSignedBigInt

	zeroMapIndex := rosterExportCharacter(1, "LostWar")
	zeroMapIndex.MapIndex = 0

	zeroLevel := rosterExportCharacter(1, "LevelZero")
	zeroLevel.Level = 0

	nonEmptyZeroID := rosterExportCharacter(0, "GhostSlot")

	cases := []struct {
		name     string
		accounts []Account
	}{
		{
			name: "duplicate normalized account login",
			accounts: []Account{
				{Login: "Alpha", Characters: []loginticket.Character{rosterExportCharacter(1, "AlphaWar")}},
				{Login: "alpha", Characters: []loginticket.Character{rosterExportCharacter(2, "AlphaNinja")}},
			},
		},
		{
			name: "too many select screen slots",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{
				rosterExportCharacter(1, "One"),
				{},
				{},
				{},
				{},
			}}},
		},
		{
			name:     "zero map index",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{zeroMapIndex}}},
		},
		{
			name:     "zero level",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{zeroLevel}}},
		},
		{
			name:     "gold outside signed bigint",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{tooMuchGold}}},
		},
		{
			name:     "non-empty zero id slot",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{nonEmptyZeroID}}},
		},
		{
			name: "duplicate character ids across accounts",
			accounts: []Account{
				{Login: "Alpha", Characters: []loginticket.Character{rosterExportCharacter(1, "AlphaWar")}},
				{Login: "Bravo", Characters: []loginticket.Character{rosterExportCharacter(1, "BravoWar")}},
			},
		},
		{
			name: "duplicate normalized character names across accounts",
			accounts: []Account{
				{Login: "Alpha", Characters: []loginticket.Character{rosterExportCharacter(1, "Hero")}},
				{Login: "Bravo", Characters: []loginticket.Character{rosterExportCharacter(2, "hero")}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExportAccountCharacterRoster(tc.accounts)
			if !errors.Is(err, ErrInvalidAccount) {
				t.Fatalf("expected ErrInvalidAccount, got %v", err)
			}
		})
	}
}

func TestFileStoreExportAccountCharacterRosterReadsCommittedSnapshots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.Save(Account{Login: "Beta", Empire: 2, Characters: []loginticket.Character{rosterExportCharacter(200, "BetaWar")}}); err != nil {
		t.Fatalf("save beta account: %v", err)
	}
	if err := store.Save(Account{Login: "Alpha", Empire: 1}); err != nil {
		t.Fatalf("save alpha account: %v", err)
	}

	export, err := store.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatalf("file-store roster export: %v", err)
	}
	if len(export.Accounts) != 2 || export.Accounts[0].Login != "Alpha" || export.Accounts[1].Login != "Beta" {
		t.Fatalf("expected file-store export to use deterministic committed account order, got %#v", export.Accounts)
	}
	if len(export.Characters) != 1 || export.Characters[0].Name != "BetaWar" || export.Characters[0].AccountID != export.Accounts[1].ID {
		t.Fatalf("unexpected file-store character export rows: %#v", export.Characters)
	}
}

func rosterExportCharacter(id uint32, name string) loginticket.Character {
	return loginticket.Character{
		ID:       id,
		VID:      0x02040000 + id,
		Name:     name,
		Level:    1,
		MapIndex: 1,
	}
}
