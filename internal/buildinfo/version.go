package buildinfo

// Package-level defaults are intended for `go run` / unstamped local builds.
// Release and CI builds should override them via -ldflags, for example:
//
//	-X github.com/MikelCalvo/go-metin2-server/internal/buildinfo.Version=v0.1.0
//	-X github.com/MikelCalvo/go-metin2-server/internal/buildinfo.Commit=<sha>
//	-X github.com/MikelCalvo/go-metin2-server/internal/buildinfo.BuildDate=<RFC3339 UTC>
//	-X github.com/MikelCalvo/go-metin2-server/internal/buildinfo.WorkflowRunID=<workflow run id>
var (
	Version       = "dev"
	Commit        = "none"
	BuildDate     = "unknown"
	WorkflowRunID = ""
)

// Snapshot is the metadata-only release identity exposed to operators and CLI
// version commands. It never includes configuration secrets or DSNs.
type Snapshot struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuildDate     string `json:"build_date"`
	WorkflowRunID string `json:"workflow_run_id"`
}

// Current returns the process build identity currently stamped into the binary.
func Current() Snapshot {
	return Snapshot{
		Version:       Version,
		Commit:        Commit,
		BuildDate:     BuildDate,
		WorkflowRunID: WorkflowRunID,
	}
}
