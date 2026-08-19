package buildinfo

import "testing"

func TestDefaultBuildInfoIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}

	if Commit == "" {
		t.Fatal("Commit should not be empty")
	}

	if BuildDate == "" {
		t.Fatal("BuildDate should not be empty")
	}
}

func TestCurrentReturnsPackageIdentity(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuildDate := BuildDate
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuildDate = originalBuildDate
	})

	Version = "v0.1.0-test"
	Commit = "abc1234"
	BuildDate = "2026-08-19T12:00:00Z"

	got := Current()
	if got.Version != Version || got.Commit != Commit || got.BuildDate != BuildDate {
		t.Fatalf("unexpected snapshot %#v", got)
	}
}
