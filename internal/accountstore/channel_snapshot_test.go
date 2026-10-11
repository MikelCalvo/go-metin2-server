package accountstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestCharacterChannelSnapshotDefaultsPersistsAndSurvivesLocationSave(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{Login: "Alpha", Characters: []loginticket.Character{
		{ID: 11, Name: "First", MapIndex: 21},
		{},
		{ID: 12, Name: "Second", MapIndex: 42},
	}}
	if err := store.Save(account); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ChannelForCharacter(11) != 1 || loaded.ChannelForCharacter(12) != 1 {
		t.Fatalf("new characters should default to local channel 1: %#v", loaded.CharacterChannels)
	}
	loaded.CharacterChannels[12] = 2
	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	// A runtime location save rebuilds Account from the ticket (which has no
	// channel field). It must not silently rewrite an assigned channel.
	loaded.Characters[2].MapIndex = 51
	if err := store.Save(Account{Login: "Alpha", Characters: loaded.Characters}); err != nil {
		t.Fatal(err)
	}
	loaded, err = NewFileStore(store.Dir()).Load("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Characters[2].MapIndex != 51 || loaded.ChannelForCharacter(11) != 1 || loaded.ChannelForCharacter(12) != 2 {
		t.Fatalf("location save lost per-character channel: %+v", loaded)
	}
	raw, err := os.ReadFile(store.accountPath("Alpha"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Characters []map[string]json.RawMessage `json:"characters"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if string(snapshot.Characters[0]["channel_id"]) != "1" || string(snapshot.Characters[2]["channel_id"]) != "2" {
		t.Fatalf("channel is not persisted beside map index: %s", raw)
	}
	if _, ok := snapshot.Characters[1]["channel_id"]; ok {
		t.Fatalf("empty slot must not acquire a channel: %s", raw)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")
	if err := store.BackupTo(backupDir); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ValidateBackupFrom(backupDir); err != nil {
		t.Fatal(err)
	}
	restored := NewFileStore(filepath.Join(t.TempDir(), "restored"))
	if err := restored.RestoreFrom(backupDir); err != nil {
		t.Fatal(err)
	}
	got, err := restored.Load("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got.ChannelForCharacter(12) != 2 || got.Characters[2].MapIndex != 51 {
		t.Fatalf("restore lost location/channel: %+v", got)
	}
}

func TestCharacterChannelLegacyDefaultAndInvalidSnapshots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.accountPath("Alpha")
	for _, tt := range []struct {
		name      string
		raw       string
		wantError bool
	}{
		{"omitted", `{"login":"Alpha","characters":[{"id":11,"name":"First","map_index":7},{}]}`, false},
		{"other channel", `{"login":"Alpha","characters":[{"id":11,"name":"First","map_index":7,"channel_id":2}]}`, false},
		{"zero", `{"login":"Alpha","characters":[{"id":11,"name":"First","map_index":7,"channel_id":0}]}`, true},
		{"empty slot channel", `{"login":"Alpha","characters":[{"channel_id":1}]}`, true},
		{"null", `{"login":"Alpha","characters":[{"id":11,"name":"First","channel_id":null}]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := store.Load("Alpha")
			if tt.wantError {
				if !errors.Is(err, ErrInvalidAccount) {
					t.Fatalf("want invalid account, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := uint8(1)
			if tt.name == "other channel" {
				want = 2
			}
			if got.ChannelForCharacter(11) != want {
				t.Fatalf("channel = %d, want %d", got.ChannelForCharacter(11), want)
			}
			if tt.name == "omitted" {
				if err := store.Save(got); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(raw, []byte(`"channel_id":1`)) {
					t.Fatalf("legacy default not persisted: %s", raw)
				}
			}
		})
	}
}

func TestMemoryStoreRetainsCharacterChannelAcrossTicketStyleSave(t *testing.T) {
	store := NewMemoryStore()
	account := Account{Login: "Alpha", Characters: []loginticket.Character{{ID: 11, Name: "First", MapIndex: 3}}, CharacterChannels: map[uint32]uint8{11: 2}}
	if err := store.Save(account); err != nil {
		t.Fatal(err)
	}
	account.Characters[0].MapIndex = 4
	if err := store.Save(Account{Login: "Alpha", Characters: account.Characters}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got.ChannelForCharacter(11) != 2 || got.Characters[0].MapIndex != 4 {
		t.Fatalf("lost channel/map: %+v", got)
	}
	if err := store.Save(Account{Login: "Alpha", Characters: account.Characters, CharacterChannels: map[uint32]uint8{11: 0}}); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("zero channel accepted: %v", err)
	}
}

func TestPersistedCharacterChannelMismatchCannotJoinLocalChannel(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{Login: "Alpha", Characters: []loginticket.Character{
		{ID: 11, Name: "First", MapIndex: 21},
		{ID: 12, Name: "Second", MapIndex: 21},
	}, CharacterChannels: map[uint32]uint8{11: 1, 12: 2}}
	if err := store.Save(account); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.MatchesLocalChannel(1) || loaded.MatchesLocalChannel(2) || loaded.MatchesLocalChannel(0) {
		t.Fatalf("mixed local/remote roster must not join any local process: %+v", loaded.CharacterChannels)
	}
	loaded.CharacterChannels[12] = 1
	if !loaded.MatchesLocalChannel(1) || loaded.MatchesLocalChannel(2) {
		t.Fatalf("all-local roster should join only channel 1: %+v", loaded.CharacterChannels)
	}
}
