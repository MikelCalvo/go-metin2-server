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
