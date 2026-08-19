package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestValidateAccountCharacterRosterExportAcceptsCanonicalExport(t *testing.T) {
	export, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
				{},
				{},
				rosterExportCharacter(12, "AlphaSura"),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				rosterExportCharacter(22, "BravoNinja"),
			},
		},
	})
	if err != nil {
		t.Fatalf("seed export: %v", err)
	}

	summary, err := ValidateAccountCharacterRosterExport(export)
	if err != nil {
		t.Fatalf("validate account character roster export: %v", err)
	}
	want := AccountCharacterRosterQuarantineSummary{
		AccountCount:   2,
		CharacterCount: 3,
		AccountIDs:     []int64{export.Accounts[0].ID, export.Accounts[1].ID},
		CharacterIDs:   []uint32{11, 12, 22},
	}
	if want.AccountIDs[0] > want.AccountIDs[1] {
		want.AccountIDs[0], want.AccountIDs[1] = want.AccountIDs[1], want.AccountIDs[0]
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateAccountCharacterRosterExportRejectsWrongMigrationBoundary(t *testing.T) {
	cases := []AccountCharacterRosterExport{
		{
			MigrationVersion: 3,
			MigrationName:    AccountCharacterRosterMigrationName,
			Accounts:         []AccountCharacterRosterAccountRow{},
			Characters:       []AccountCharacterRosterCharacterRow{},
		},
		{
			MigrationVersion: AccountCharacterRosterMigrationVersion,
			MigrationName:    "character_item_state",
			Accounts:         []AccountCharacterRosterAccountRow{},
			Characters:       []AccountCharacterRosterCharacterRow{},
		},
	}
	for _, export := range cases {
		if _, err := ValidateAccountCharacterRosterExport(export); !errors.Is(err, ErrInvalidAccountCharacterRosterExport) {
			t.Fatalf("expected ErrInvalidAccountCharacterRosterExport for version=%d name=%q, got %v", export.MigrationVersion, export.MigrationName, err)
		}
	}
}

func TestValidateAccountCharacterRosterExportRejectsInvalidRows(t *testing.T) {
	cases := []struct {
		name   string
		export AccountCharacterRosterExport
	}{
		{
			name: "nil accounts",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Characters:       []AccountCharacterRosterCharacterRow{},
			},
		},
		{
			name: "nil characters",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts:         []AccountCharacterRosterAccountRow{},
			},
		},
		{
			name: "duplicate login_normalized",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
					{ID: 2, Login: "alpha", LoginNormalized: "alpha", Empire: 2},
				},
				Characters: []AccountCharacterRosterCharacterRow{},
			},
		},
		{
			name: "duplicate account id",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
					{ID: 1, Login: "Bravo", LoginNormalized: "bravo", Empire: 2},
				},
				Characters: []AccountCharacterRosterCharacterRow{},
			},
		},
		{
			name: "unknown account_id",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
				},
				Characters: []AccountCharacterRosterCharacterRow{
					{ID: 11, AccountID: 99, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 1, MapIndex: 1},
				},
			},
		},
		{
			name: "duplicate account slot",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
				},
				Characters: []AccountCharacterRosterCharacterRow{
					{ID: 11, AccountID: 1, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 1, MapIndex: 1},
					{ID: 12, AccountID: 1, Slot: 0, Name: "AlphaSura", NameNormalized: "alphasura", Level: 1, MapIndex: 1},
				},
			},
		},
		{
			name: "duplicate character id",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
					{ID: 2, Login: "Bravo", LoginNormalized: "bravo", Empire: 2},
				},
				Characters: []AccountCharacterRosterCharacterRow{
					{ID: 11, AccountID: 1, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 1, MapIndex: 1},
					{ID: 11, AccountID: 2, Slot: 0, Name: "BravoWar", NameNormalized: "bravowar", Level: 1, MapIndex: 1},
				},
			},
		},
		{
			name: "duplicate name_normalized",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
					{ID: 2, Login: "Bravo", LoginNormalized: "bravo", Empire: 2},
				},
				Characters: []AccountCharacterRosterCharacterRow{
					{ID: 11, AccountID: 1, Slot: 0, Name: "Hero", NameNormalized: "hero", Level: 1, MapIndex: 1},
					{ID: 12, AccountID: 2, Slot: 0, Name: "hero", NameNormalized: "hero", Level: 1, MapIndex: 1},
				},
			},
		},
		{
			name: "mismatched name_normalized",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
				},
				Characters: []AccountCharacterRosterCharacterRow{
					{ID: 11, AccountID: 1, Slot: 0, Name: "AlphaWar", NameNormalized: "other", Level: 1, MapIndex: 1},
				},
			},
		},
		{
			name: "zero level",
			export: AccountCharacterRosterExport{
				MigrationVersion: AccountCharacterRosterMigrationVersion,
				MigrationName:    AccountCharacterRosterMigrationName,
				Accounts: []AccountCharacterRosterAccountRow{
					{ID: 1, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
				},
				Characters: []AccountCharacterRosterCharacterRow{
					{ID: 11, AccountID: 1, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 0, MapIndex: 1},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateAccountCharacterRosterExport(tc.export); !errors.Is(err, ErrInvalidAccountCharacterRosterExport) {
				t.Fatalf("expected ErrInvalidAccountCharacterRosterExport, got %v", err)
			}
		})
	}
}

func TestQuarantineAccountCharacterRosterExportCanonicalizesRowOrder(t *testing.T) {
	export := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		Accounts: []AccountCharacterRosterAccountRow{
			{ID: 200, Login: "Bravo", LoginNormalized: "bravo", Empire: 2},
			{ID: 100, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
		},
		Characters: []AccountCharacterRosterCharacterRow{
			{ID: 22, AccountID: 200, Slot: 1, Name: "BravoNinja", NameNormalized: "bravoninja", Level: 7, MapIndex: 42, Gold: 500},
			{ID: 12, AccountID: 100, Slot: 3, Name: "AlphaSura", NameNormalized: "alphasura", Level: 4, MapIndex: 1},
			{ID: 11, AccountID: 100, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 5, MapIndex: 1, Gold: 1234},
		},
	}

	quarantined, summary, err := QuarantineAccountCharacterRosterExport(export)
	if err != nil {
		t.Fatalf("quarantine account character roster export: %v", err)
	}
	wantSummary := AccountCharacterRosterQuarantineSummary{
		AccountCount:   2,
		CharacterCount: 3,
		AccountIDs:     []int64{100, 200},
		CharacterIDs:   []uint32{11, 12, 22},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}

	wantAccounts := []AccountCharacterRosterAccountRow{
		{ID: 100, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
		{ID: 200, Login: "Bravo", LoginNormalized: "bravo", Empire: 2},
	}
	if !reflect.DeepEqual(quarantined.Accounts, wantAccounts) {
		t.Fatalf("unexpected canonical account rows:\n got: %#v\nwant: %#v", quarantined.Accounts, wantAccounts)
	}
	wantCharacters := []AccountCharacterRosterCharacterRow{
		{ID: 11, AccountID: 100, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 5, MapIndex: 1, Gold: 1234},
		{ID: 12, AccountID: 100, Slot: 3, Name: "AlphaSura", NameNormalized: "alphasura", Level: 4, MapIndex: 1},
		{ID: 22, AccountID: 200, Slot: 1, Name: "BravoNinja", NameNormalized: "bravoninja", Level: 7, MapIndex: 42, Gold: 500},
	}
	if !reflect.DeepEqual(quarantined.Characters, wantCharacters) {
		t.Fatalf("unexpected canonical character rows:\n got: %#v\nwant: %#v", quarantined.Characters, wantCharacters)
	}
}
