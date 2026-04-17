package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	store := NewFileStore(t.TempDir())
	want := Account{
		Login: "mkmk",
		Characters: []loginticket.Character{{
			ID:         1,
			VID:        0x01020304,
			Name:       "MkmkWar",
			Job:        0,
			RaceNum:    0,
			Level:      15,
			ST:         6,
			HT:         5,
			DX:         4,
			IQ:         3,
			MainPart:   101,
			HairPart:   201,
			X:          1000,
			Y:          2000,
			Empire:     2,
			SkillGroup: 1,
		}},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("save account: %v", err)
	}
	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected account:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreLoadReturnsNotFoundForUnknownLogin(t *testing.T) {
	store := NewFileStore(t.TempDir())
	_, err := store.Load("ghost")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}
