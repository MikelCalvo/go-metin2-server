package ops

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubSafeboxStoreBacker struct {
	summary any
	err     error
	calls   int
	lastDir string
}

func (s *stubSafeboxStoreBacker) Backup(dstDir string) (any, error) {
	s.calls++
	s.lastDir = dstDir
	return s.summary, s.err
}

type stubSafeboxStoreBackupValidator struct {
	summary any
	err     error
	calls   int
	lastDir string
}

func (s *stubSafeboxStoreBackupValidator) ValidateBackup(srcDir string) (any, error) {
	s.calls++
	s.lastDir = srcDir
	return s.summary, s.err
}

type stubSafeboxStoreRestorer struct {
	summary any
	err     error
	calls   int
	lastDir string
}

func (s *stubSafeboxStoreRestorer) Restore(srcDir string) (any, error) {
	s.calls++
	s.lastDir = srcDir
	return s.summary, s.err
}

type stubSafeboxStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubSafeboxStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

func TestLocalSafeboxStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubSafeboxStoreValidator{summary: map[string]any{"character_count": 2, "cell_count": 3, "logins": []string{"alpha", "beta"}}}
	mux := RegisterLocalSafeboxStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/validate", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if validator.calls != 1 {
		t.Fatalf("expected validate called once, got %d", validator.calls)
	}
	if !strings.Contains(rec.Body.String(), `"character_count":2`) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestLocalSafeboxStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubSafeboxStoreValidator{}
	mux := RegisterLocalSafeboxStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/validate", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if validator.calls != 0 {
		t.Fatalf("expected validate not called, got %d", validator.calls)
	}
}

func TestLocalSafeboxStoreBackupEndpointBacksUpToLoopbackRequestedDirectory(t *testing.T) {
	backer := &stubSafeboxStoreBacker{summary: map[string]any{"character_count": 1, "cell_count": 1, "logins": []string{"owner"}}}
	mux := RegisterLocalSafeboxStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/backup", strings.NewReader(`{"dst_dir":"/tmp/safebox-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if backer.calls != 1 || backer.lastDir != "/tmp/safebox-backup" {
		t.Fatalf("unexpected backer state: calls=%d dir=%q", backer.calls, backer.lastDir)
	}
}

func TestLocalSafeboxStoreBackupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	backer := &stubSafeboxStoreBacker{summary: map[string]any{"character_count": 1}}
	mux := RegisterLocalSafeboxStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/backup", strings.NewReader(`{"dst_dir":"/tmp/safebox-backup"}`))
	req.RemoteAddr = "198.51.100.9:55"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if backer.calls != 0 {
		t.Fatalf("expected backup not called, got %d", backer.calls)
	}
}

func TestLocalSafeboxStoreBackupValidateEndpointDryRunsLoopbackRequestedSource(t *testing.T) {
	validator := &stubSafeboxStoreBackupValidator{summary: map[string]any{"character_count": 1, "cell_count": 2, "character_keys": []string{"owner:7"}}}
	mux := RegisterLocalSafeboxStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/backup/validate", strings.NewReader(`{"src_dir":"/tmp/safebox-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if validator.calls != 1 || validator.lastDir != "/tmp/safebox-backup" {
		t.Fatalf("unexpected validator state: calls=%d dir=%q", validator.calls, validator.lastDir)
	}
}

func TestLocalSafeboxStoreRestoreEndpointRestoresFromLoopbackRequestedSource(t *testing.T) {
	restorer := &stubSafeboxStoreRestorer{summary: map[string]any{"character_count": 1, "cell_count": 1, "logins": []string{"owner"}}}
	mux := RegisterLocalSafeboxStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/restore", strings.NewReader(`{"src_dir":"/tmp/safebox-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if restorer.calls != 1 || restorer.lastDir != "/tmp/safebox-backup" {
		t.Fatalf("unexpected restorer state: calls=%d dir=%q", restorer.calls, restorer.lastDir)
	}
}

func TestLocalSafeboxStoreRestoreEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	restorer := &stubSafeboxStoreRestorer{summary: map[string]any{"character_count": 1}}
	mux := RegisterLocalSafeboxStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/restore", strings.NewReader(`{"src_dir":"/tmp/safebox-backup"}`))
	req.RemoteAddr = "203.0.113.44:9"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if restorer.calls != 0 {
		t.Fatalf("expected restore not called, got %d", restorer.calls)
	}
}

func TestLocalSafeboxStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubSafeboxStoreValidator{summary: map[string]any{"character_count": 0, "cell_count": 0, "logins": []string{}, "character_keys": []string{}}}
	mux := RegisterLocalSafeboxStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Validate)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/crash-temps/cleanup", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if cleaner.calls != 1 {
		t.Fatalf("expected cleanup called once, got %d", cleaner.calls)
	}
}

func TestLocalSafeboxStoreBackupEndpointReturnsConflictOnBackupError(t *testing.T) {
	backer := &stubSafeboxStoreBacker{err: errors.New("backup failed")}
	mux := RegisterLocalSafeboxStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	req := httptest.NewRequest(http.MethodPost, "/local/safebox-store/backup", strings.NewReader(`{"dst_dir":"/tmp/safebox-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}
