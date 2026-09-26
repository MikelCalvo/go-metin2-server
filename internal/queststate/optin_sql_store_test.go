package queststate

import "testing"

func TestSelectRematerializeStoreKeepsFileStoreUntilSQLIsOptedIn(t *testing.T) {
	fileStore := NewFileStore(t.TempDir() + "/quest-state.json")
	sqlStore := NewSQLStore(nil)

	if got := SelectRematerializeStore(fileStore, nil); got != fileStore {
		t.Fatalf("stock rematerialize = %T, want the FileStore", got)
	}
	if got := SelectRematerializeStore(nil, nil); got != nil {
		t.Fatalf("nil stock rematerialize = %v, want nil", got)
	}
	if got := SelectRematerializeStore(fileStore, sqlStore); got != sqlStore {
		t.Fatalf("opt-in rematerialize = %T, want the supplied SQLStore", got)
	}
	if _, ok := SelectRematerializeStore(fileStore, nil).(*FileStore); !ok {
		t.Fatal("stock gamed must keep *FileStore when no SQLStore is supplied")
	}
}

func TestStockRematerializeTransitionStaysOnFileStore(t *testing.T) {
	fileStore := NewMemoryStore()
	selected := SelectRematerializeStore(fileStore, nil)
	if selected != fileStore {
		t.Fatal("stock selector did not keep the FileStore")
	}
	if err := rematerializeQuestFlag(selected, Transition{
		Character: "AlphaWar",
		QuestRef:  "quest:first_steps",
		Flag:      "met_guide",
		From:      0,
		To:        1,
	}); err != nil {
		t.Fatalf("stock transition: %v", err)
	}
	got, err := rematerializeQuestFlagValue(selected, "AlphaWar", "quest:first_steps", "met_guide")
	if err != nil {
		t.Fatalf("stock reload: %v", err)
	}
	if got != 1 {
		t.Fatalf("stock reloaded flag = %d, want 1", got)
	}
}

func rematerializeQuestFlag(store Store, transition Transition) error {
	applier, ok := store.(interface {
		ApplyTransition(Transition) (TransitionApplyResult, error)
	})
	if !ok {
		return ErrInvalidSnapshot
	}
	result, err := applier.ApplyTransition(transition)
	if err != nil {
		return err
	}
	if !result.Result.Applied {
		return ErrInvalidSnapshot
	}
	return nil
}

func rematerializeQuestFlagValue(store Store, character string, questRef string, flag string) (uint32, error) {
	snapshot, err := store.Load()
	if err != nil {
		return 0, err
	}
	for _, row := range snapshot.Flags {
		if row.Character == character && row.QuestRef == questRef && row.Name == flag {
			return row.Value, nil
		}
	}
	return 0, nil
}
