package loginticket

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestValidateAuthLoginTicketHandoffExportAcceptsCanonicalExport(t *testing.T) {
	issuedAlpha := time.Date(2026, 8, 19, 9, 30, 0, 0, time.UTC)
	issuedZeta := time.Date(2026, 8, 19, 10, 45, 0, 0, time.UTC)
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets: []AuthLoginTicketHandoffRow{
			{
				LoginKey:               0x01020304,
				IssuedAt:               issuedAlpha,
				Login:                  "Alpha",
				LoginNormalized:        "alpha",
				Empire:                 1,
				CharactersSnapshotJSON: `[{"id":7,"name":"AlphaWar","level":1,"map_index":1,"inventory":[],"equipment":[],"quickslots":[]}]`,
			},
			{
				LoginKey:               0x02000000,
				IssuedAt:               issuedZeta,
				Login:                  "zeta",
				LoginNormalized:        "zeta",
				Empire:                 3,
				CharactersSnapshotJSON: `[]`,
			},
		},
	}

	summary, err := ValidateAuthLoginTicketHandoffExport(export)
	if err != nil {
		t.Fatalf("validate auth login-ticket handoff export: %v", err)
	}
	want := AuthLoginTicketHandoffQuarantineSummary{
		TicketCount:       2,
		ActiveTicketCount: 2,
		LoginKeys:         []uint32{0x01020304, 0x02000000},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateAuthLoginTicketHandoffExportAcceptsEmptyTickets(t *testing.T) {
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}

	summary, err := ValidateAuthLoginTicketHandoffExport(export)
	if err != nil {
		t.Fatalf("validate empty auth login-ticket handoff export: %v", err)
	}
	want := AuthLoginTicketHandoffQuarantineSummary{
		TicketCount:       0,
		ActiveTicketCount: 0,
		LoginKeys:         []uint32{},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected empty quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateAuthLoginTicketHandoffExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: 4,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets:          []AuthLoginTicketHandoffRow{},
	}
	if _, err := ValidateAuthLoginTicketHandoffExport(export); !errors.Is(err, ErrInvalidAuthLoginTicketHandoffExport) {
		t.Fatalf("expected ErrInvalidAuthLoginTicketHandoffExport, got %v", err)
	}
}

func TestValidateAuthLoginTicketHandoffExportAcceptsConsumedHistoryWithOneActiveKey(t *testing.T) {
	issuedAt := time.Date(2026, 8, 19, 9, 30, 0, 0, time.UTC)
	consumedAt := issuedAt.Add(time.Minute)
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets: []AuthLoginTicketHandoffRow{
			{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", ConsumedAt: &consumedAt, CharactersSnapshotJSON: `[]`},
			{LoginKey: 0x01020304, IssuedAt: issuedAt.Add(2 * time.Minute), Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: `[]`},
		},
	}

	summary, err := ValidateAuthLoginTicketHandoffExport(export)
	if err != nil {
		t.Fatalf("validate consumed+active auth login-ticket handoff export: %v", err)
	}
	want := AuthLoginTicketHandoffQuarantineSummary{
		TicketCount:       2,
		ActiveTicketCount: 1,
		LoginKeys:         []uint32{0x01020304},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateAuthLoginTicketHandoffExportRejectsMalformedRows(t *testing.T) {
	issuedAt := time.Date(2026, 8, 19, 9, 30, 0, 0, time.UTC)
	consumedEarly := issuedAt.Add(-time.Minute)
	validSnapshot := `[]`
	cases := []struct {
		name   string
		export AuthLoginTicketHandoffExport
	}{
		{
			name: "nil tickets",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
			},
		},
		{
			name: "zero login key",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
		{
			name: "blank login",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: " ", LoginNormalized: " ", CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
		{
			name: "login normalized mismatch",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "bravo", CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
		{
			name: "zero issued_at",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
		{
			name: "consumed_at before issued_at",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", ConsumedAt: &consumedEarly, CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
		{
			name: "empty characters snapshot",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: ""},
				},
			},
		},
		{
			name: "null characters snapshot",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: "null"},
				},
			},
		},
		{
			name: "invalid character payload",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: `[{"name":"Ghost"}]`},
				},
			},
		},
		{
			name: "duplicate primary key",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: validSnapshot},
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
		{
			name: "duplicate active login key",
			export: AuthLoginTicketHandoffExport{
				MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
				MigrationName:    AuthLoginTicketHandoffMigrationName,
				Tickets: []AuthLoginTicketHandoffRow{
					{LoginKey: 0x01020304, IssuedAt: issuedAt, Login: "Alpha", LoginNormalized: "alpha", CharactersSnapshotJSON: validSnapshot},
					{LoginKey: 0x01020304, IssuedAt: issuedAt.Add(time.Minute), Login: "Bravo", LoginNormalized: "bravo", CharactersSnapshotJSON: validSnapshot},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateAuthLoginTicketHandoffExport(tc.export); !errors.Is(err, ErrInvalidAuthLoginTicketHandoffExport) {
				t.Fatalf("expected ErrInvalidAuthLoginTicketHandoffExport, got %v", err)
			}
		})
	}
}

func TestQuarantineAuthLoginTicketHandoffExportCanonicalizesRowOrder(t *testing.T) {
	issuedAlpha := time.Date(2026, 8, 19, 9, 30, 0, 0, time.UTC)
	issuedZeta := time.Date(2026, 8, 19, 10, 45, 0, 0, time.UTC)
	issuedBravo := time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC)
	consumedAt := issuedAlpha.Add(time.Hour)
	export := AuthLoginTicketHandoffExport{
		MigrationVersion: AuthLoginTicketHandoffMigrationVersion,
		MigrationName:    AuthLoginTicketHandoffMigrationName,
		Tickets: []AuthLoginTicketHandoffRow{
			{
				LoginKey:               0x02000000,
				IssuedAt:               issuedZeta,
				Login:                  "zeta",
				LoginNormalized:        "zeta",
				Empire:                 3,
				CharactersSnapshotJSON: `[]`,
			},
			{
				LoginKey:               0x01020304,
				IssuedAt:               issuedAlpha,
				Login:                  "Alpha",
				LoginNormalized:        "alpha",
				Empire:                 1,
				ConsumedAt:             &consumedAt,
				CharactersSnapshotJSON: `[{"id":7,"name":"AlphaWar","level":1,"map_index":1,"inventory":[],"equipment":[],"quickslots":[]}]`,
			},
			{
				LoginKey:               0x00abcdef,
				IssuedAt:               issuedBravo,
				Login:                  "Bravo",
				LoginNormalized:        "bravo",
				Empire:                 2,
				CharactersSnapshotJSON: `[]`,
			},
		},
	}

	quarantined, summary, err := QuarantineAuthLoginTicketHandoffExport(export)
	if err != nil {
		t.Fatalf("quarantine auth login-ticket handoff export: %v", err)
	}
	wantSummary := AuthLoginTicketHandoffQuarantineSummary{
		TicketCount:       3,
		ActiveTicketCount: 2,
		LoginKeys:         []uint32{0x00abcdef, 0x01020304, 0x02000000},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
	wantLogins := []string{"Alpha", "Bravo", "zeta"}
	if len(quarantined.Tickets) != 3 {
		t.Fatalf("unexpected quarantine ticket count: %#v", quarantined.Tickets)
	}
	for i, wantLogin := range wantLogins {
		if quarantined.Tickets[i].Login != wantLogin {
			t.Fatalf("ticket[%d] login = %q, want %q; full=%#v", i, quarantined.Tickets[i].Login, wantLogin, quarantined.Tickets)
		}
	}
	if quarantined.Tickets[0].ConsumedAt == nil || !quarantined.Tickets[0].ConsumedAt.Equal(consumedAt.UTC()) {
		t.Fatalf("expected consumed ticket to retain UTC consumed_at, got %#v", quarantined.Tickets[0])
	}
}
