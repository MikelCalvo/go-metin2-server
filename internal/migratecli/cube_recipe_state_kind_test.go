package migratecli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExportQuarantineKindsRemainClosedWithoutCubeRecipeState(t *testing.T) {
	if len(exportQuarantineKinds) != 10 {
		t.Fatalf("exportQuarantineKinds len = %d, want 10 (cube-recipe-state stays a cubestore primitive this slice)", len(exportQuarantineKinds))
	}
	for _, kind := range exportQuarantineKinds {
		if kind == "cube-recipe-state" {
			t.Fatal("did not expect cube-recipe-state CLI kind; SQL companion stays beside FileStore without a new export kind")
		}
	}
}

func TestRunQuarantineExportRejectsCubeRecipeStateKind(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"quarantine-export", "--kind", "cube-recipe-state", "--export", "-"}, strings.NewReader(`{}`), &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on usage error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unsupported quarantine-export kind "cube-recipe-state"`) {
		t.Fatalf("stderr = %q, want unsupported cube-recipe-state kind", stderr.String())
	}
}

func TestRunImportExportRejectsCubeRecipeStateKind(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"import-export",
		"--kind", "cube-recipe-state",
		"--export", "-",
		"--driver", "sqlite",
		"--dsn", "memory://cube-recipe-state",
		"--i-confirm-sql-import",
	}, strings.NewReader(`{}`), &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on usage error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unsupported import-export kind "cube-recipe-state"`) {
		t.Fatalf("stderr = %q, want unsupported cube-recipe-state kind", stderr.String())
	}
}

func TestRunImportExportStatusRejectsCubeRecipeStateKind(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"import-export-status", "--kind", "cube-recipe-state", "--import-result", "/tmp/missing-cube-recipe-state.json"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("expected usage exit %d, got %d stderr=%q", exitUsage, code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout on usage error, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unsupported import-export-status kind "cube-recipe-state"`) {
		t.Fatalf("stderr = %q, want unsupported cube-recipe-state kind", stderr.String())
	}
}
