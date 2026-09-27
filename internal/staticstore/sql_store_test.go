package staticstore

import (
	"errors"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestSQLStoreLoadRejectsNilExecutor(t *testing.T) {
	_, err := (*SQLStore)(nil).Load()
	if !errors.Is(err, ErrStaticActorContentStateImportExecutorRequired) {
		t.Fatalf("nil SQLStore.Load error = %v, want %v", err, ErrStaticActorContentStateImportExecutorRequired)
	}
	_, err = NewSQLStore(nil).Load()
	if !errors.Is(err, ErrStaticActorContentStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Load error = %v, want %v", err, ErrStaticActorContentStateImportExecutorRequired)
	}
}

func TestSQLStoreSaveRejectsNilExecutor(t *testing.T) {
	if err := (*SQLStore)(nil).Save(Snapshot{}); !errors.Is(err, ErrStaticActorContentStateImportExecutorRequired) {
		t.Fatalf("nil SQLStore.Save error = %v, want %v", err, ErrStaticActorContentStateImportExecutorRequired)
	}
	if err := NewSQLStore(nil).Save(Snapshot{}); !errors.Is(err, ErrStaticActorContentStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Save error = %v, want %v", err, ErrStaticActorContentStateImportExecutorRequired)
	}
}

func TestSQLStoreExportRejectsNilExecutor(t *testing.T) {
	_, err := NewSQLStore(nil).ExportStaticActorContentState(nil)
	if !errors.Is(err, ErrStaticActorContentStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Export error = %v, want %v", err, ErrStaticActorContentStateImportExecutorRequired)
	}
}

func TestSQLStoreSaveRejectsInvalidSnapshotBeforeOpeningTransaction(t *testing.T) {
	err := NewSQLStore(failingStaticActorContentStateImportExecutor{}).Save(Snapshot{StaticActors: []StaticActor{{
		EntityID: 0, Name: "Broken", MapIndex: 1, X: 1, Y: 1, RaceNum: 101,
	}}})
	if err == nil || !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("SQLStore.Save(invalid) error = %v, want %v", err, ErrInvalidSnapshot)
	}
}

func TestSQLStoreSaveRejectsFileStoreOnlyCombatStateBeforeOpeningTransaction(t *testing.T) {
	currentHP := uint8(3)
	readyAt := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	cases := []StaticActor{
		{
			EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
			CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob",
			CombatCurrentHP: &currentHP,
		},
		{
			EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
			CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob",
			CombatCurrentHP: uint8Ptr(0), RespawnReadyAt: &readyAt,
		},
		{
			EntityID: 7, Name: "PracticeMob", MapIndex: 42, X: 1800, Y: 2900, RaceNum: 101,
			CombatProfile: worldruntime.StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.reward_mob",
			ProximitySuppressVIDs: []uint32{0x0103019d},
		},
	}
	for _, actor := range cases {
		err := NewSQLStore(failingStaticActorContentStateImportExecutor{}).Save(Snapshot{StaticActors: []StaticActor{actor}})
		if err == nil || !errors.Is(err, ErrInvalidSnapshot) {
			t.Fatalf("SQLStore.Save(file-store-only state) error = %v, want %v", err, ErrInvalidSnapshot)
		}
	}
}

func TestSelectRematerializeStoreKeepsFileStoreUnlessSQLIsPassed(t *testing.T) {
	fileStore := NewMemoryStore()
	sqlStore := NewSQLStore(nil)
	if got := SelectRematerializeStore(fileStore, nil); got != fileStore {
		t.Fatalf("SelectRematerializeStore(file, nil) = %T, want the FileStore", got)
	}
	if got := SelectRematerializeStore(fileStore, sqlStore); got != sqlStore {
		t.Fatalf("SelectRematerializeStore(file, sql) = %T, want SQLStore", got)
	}
	if got := SelectRematerializeStore(nil, nil); got != nil {
		t.Fatalf("SelectRematerializeStore(nil, nil) = %#v, want nil", got)
	}
	if _, ok := SelectRematerializeStore(fileStore, nil).(*MemoryStore); !ok {
		t.Fatal("stock rematerialize must keep the supplied store when no SQLStore is passed")
	}
}

func uint8Ptr(value uint8) *uint8 { return &value }
