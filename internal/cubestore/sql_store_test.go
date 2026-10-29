package cubestore

import (
	"errors"
	"testing"
)

func TestSQLStoreRequiresExecutor(t *testing.T) {
	var nilStore *SQLStore
	for _, store := range []*SQLStore{nilStore, NewSQLStore(nil)} {
		if _, err := store.Load(); !errors.Is(err, ErrCubeRecipeStateImportExecutorRequired) {
			t.Fatalf("Load without executor: %v", err)
		}
		if err := store.Save(BootstrapSnapshot()); !errors.Is(err, ErrCubeRecipeStateImportExecutorRequired) {
			t.Fatalf("Save without executor: %v", err)
		}
	}
	file := NewMemoryStore()
	if got := SelectRematerializeStore(file, nil); got != file {
		t.Fatalf("default selection = %T, want original store", got)
	}
	if got := SelectRematerializeStore(file, NewSQLStore(nil)); got == file {
		t.Fatal("explicit SQL opt-in ignored")
	}
}
