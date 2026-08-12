package migrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestReadJSONLedgerSnapshotReturnsEntriesForOfflinePlan(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	body := `{"format":"go-metin2-schema-migrations-ledger-v1","entries":[{"version":1,"name":"bootstrap_schema_migrations","up_sha256":"` + catalog[0].UpSHA256 + `"}]}`

	entries, err := ReadJSONLedgerSnapshot(strings.NewReader(body))
	if err != nil {
		t.Fatalf("read JSON ledger snapshot: %v", err)
	}
	if len(entries) != 1 || entries[0] != ledgerEntryFor(t, catalog, 1) {
		t.Fatalf("unexpected ledger entries: %#v", entries)
	}

	plan, err := PlanCatalogUpToLatest(catalog, entries)
	if err != nil {
		t.Fatalf("plan from JSON ledger entries: %v", err)
	}
	if plan.CurrentVersion != 1 || len(plan.Pending) != 1 || plan.Pending[0].Version != 2 {
		t.Fatalf("unexpected offline plan from JSON ledger: %#v", plan)
	}
}

func TestPlanToVersionFromJSONLedgerSnapshotReturnsRollbackPlan(t *testing.T) {
	catalog := testCatalog(t,
		bootstrapSchemaMigration(),
		testMigration{
			version:  2,
			name:     "accounts",
			upPath:   "0002_accounts.up.sql",
			downPath: "0002_accounts.down.sql",
			upSQL:    "-- go-metin2 migration: 0002 accounts up\nCREATE TABLE accounts (login TEXT PRIMARY KEY);\n",
			downSQL:  "-- go-metin2 migration: 0002 accounts down\nDROP TABLE accounts;\n",
		},
	)
	body, err := MarshalJSONLedgerSnapshot([]LedgerEntry{
		ledgerEntryFor(t, catalog, 2),
		ledgerEntryFor(t, catalog, 1),
	})
	if err != nil {
		t.Fatalf("marshal JSON ledger snapshot: %v", err)
	}

	plan, err := PlanCatalogToVersionFromJSONLedgerSnapshot(catalog, bytes.NewReader(body), 0)
	if err != nil {
		t.Fatalf("plan rollback from JSON ledger snapshot: %v", err)
	}
	if plan.CurrentVersion != 2 || plan.LatestVersion != 2 || plan.UpToDate {
		t.Fatalf("unexpected rollback plan: %#v", plan)
	}
	if len(plan.Pending) != 2 || plan.Pending[0].Direction != DirectionDown || plan.Pending[0].Version != 2 || plan.Pending[1].Version != 1 {
		t.Fatalf("unexpected rollback steps: %#v", plan.Pending)
	}
}

func TestMarshalJSONLedgerSnapshotReturnsDeterministicSnapshot(t *testing.T) {
	entries := []LedgerEntry{
		{Version: 2, Name: "accounts", UpSHA256: strings.Repeat("b", 64)},
		{Version: 1, Name: "bootstrap_schema_migrations", UpSHA256: strings.Repeat("a", 64)},
	}

	raw, err := MarshalJSONLedgerSnapshot(entries)
	if err != nil {
		t.Fatalf("marshal JSON ledger snapshot: %v", err)
	}
	want := "{\n" +
		"  \"format\": \"go-metin2-schema-migrations-ledger-v1\",\n" +
		"  \"entries\": [\n" +
		"    {\n" +
		"      \"version\": 1,\n" +
		"      \"name\": \"bootstrap_schema_migrations\",\n" +
		"      \"up_sha256\": \"" + strings.Repeat("a", 64) + "\"\n" +
		"    },\n" +
		"    {\n" +
		"      \"version\": 2,\n" +
		"      \"name\": \"accounts\",\n" +
		"      \"up_sha256\": \"" + strings.Repeat("b", 64) + "\"\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if string(raw) != want {
		t.Fatalf("unexpected deterministic JSON snapshot:\ngot:\n%s\nwant:\n%s", raw, want)
	}
	if entries[0].Version != 2 {
		t.Fatalf("MarshalJSONLedgerSnapshot mutated input order: %#v", entries)
	}
}

func TestJSONLedgerSnapshotSupportsEmptyAppliedLedger(t *testing.T) {
	raw, err := MarshalJSONLedgerSnapshot(nil)
	if err != nil {
		t.Fatalf("marshal empty JSON ledger snapshot: %v", err)
	}
	if !strings.Contains(string(raw), `"entries": []`) {
		t.Fatalf("expected empty ledger snapshot to encode entries as an empty array, got %s", raw)
	}
	entries, err := ReadJSONLedgerSnapshot(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("read empty JSON ledger snapshot: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no ledger entries, got %#v", entries)
	}
}

func TestReadJSONLedgerSnapshotRejectsInvalidSnapshots(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{name: "nil reader", raw: nil},
		{name: "empty body", raw: []byte("  \n\t")},
		{name: "invalid utf8", raw: []byte{'{', '"', 'f', 'o', 'r', 'm', 'a', 't', '"', ':', '"', 0xff, '"', '}'}},
		{name: "malformed json", raw: []byte("{not-json")},
		{name: "unknown field", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1","entries":[],"extra":true}`)},
		{name: "trailing json", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1","entries":[]} {}`)},
		{name: "unsupported format", raw: []byte(`{"format":"manual","entries":[]}`)},
		{name: "missing entries", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1"}`)},
		{name: "null entries", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1","entries":null}`)},
		{name: "invalid entry version", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1","entries":[{"version":0,"name":"bootstrap_schema_migrations","up_sha256":"` + strings.Repeat("a", 64) + `"}]}`)},
		{name: "invalid entry name", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1","entries":[{"version":1,"name":"Bootstrap","up_sha256":"` + strings.Repeat("a", 64) + `"}]}`)},
		{name: "invalid entry checksum", raw: []byte(`{"format":"go-metin2-schema-migrations-ledger-v1","entries":[{"version":1,"name":"bootstrap_schema_migrations","up_sha256":"abc"}]}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reader *strings.Reader
			if tc.raw != nil {
				reader = strings.NewReader(string(tc.raw))
			}
			_, err := ReadJSONLedgerSnapshot(reader)
			if !errors.Is(err, ErrInvalidLedgerSnapshot) {
				t.Fatalf("expected ErrInvalidLedgerSnapshot, got %v", err)
			}
		})
	}
}

func TestMarshalJSONLedgerSnapshotRejectsInvalidEntries(t *testing.T) {
	_, err := MarshalJSONLedgerSnapshot([]LedgerEntry{{Version: 1, Name: "Bootstrap", UpSHA256: strings.Repeat("a", 64)}})
	if !errors.Is(err, ErrInvalidLedgerSnapshot) {
		t.Fatalf("expected ErrInvalidLedgerSnapshot, got %v", err)
	}
}

func TestJSONLedgerSnapshotOmitsExecutableSQL(t *testing.T) {
	raw, err := MarshalJSONLedgerSnapshot([]LedgerEntry{{Version: 1, Name: "bootstrap_schema_migrations", UpSHA256: strings.Repeat("a", 64)}})
	if err != nil {
		t.Fatalf("marshal JSON ledger snapshot: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode generated ledger snapshot: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, "CREATE TABLE") || strings.Contains(body, "UpSQL") || strings.Contains(body, "DownSQL") {
		t.Fatalf("ledger snapshot must not expose executable SQL, got %s", body)
	}
}
