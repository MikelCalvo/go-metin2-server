package ops

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubGroundItemStoreBacker struct {
	summary any
	err     error
	calls   int
	lastDir string
}

func (s *stubGroundItemStoreBacker) Backup(dstDir string) (any, error) {
	s.calls++
	s.lastDir = dstDir
	return s.summary, s.err
}

type stubGroundItemStoreBackupValidator struct {
	summary any
	err     error
	calls   int
	lastDir string
}

func (s *stubGroundItemStoreBackupValidator) ValidateBackup(srcDir string) (any, error) {
	s.calls++
	s.lastDir = srcDir
	return s.summary, s.err
}

type stubGroundItemStoreRestorer struct {
	summary any
	err     error
	calls   int
	lastDir string
}

func (s *stubGroundItemStoreRestorer) Restore(srcDir string) (any, error) {
	s.calls++
	s.lastDir = srcDir
	return s.summary, s.err
}

type stubGroundItemStoreValidator struct {
	summary any
	err     error
	calls   int
}

func (s *stubGroundItemStoreValidator) Validate() (any, error) {
	s.calls++
	return s.summary, s.err
}

func TestLocalGroundItemStoreValidateEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	validator := &stubGroundItemStoreValidator{summary: map[string]any{"ground_item_count": 2, "vids": []uint32{1, 2}}}
	mux := RegisterLocalGroundItemStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/validate", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if validator.calls != 1 {
		t.Fatalf("expected validate called once, got %d", validator.calls)
	}
	if !strings.Contains(rec.Body.String(), `"ground_item_count":2`) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestLocalGroundItemStoreValidateEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	validator := &stubGroundItemStoreValidator{}
	mux := RegisterLocalGroundItemStoreValidateEndpoint(NewPprofMux("gamed"), validator.Validate)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/validate", nil)
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

func TestLocalGroundItemStoreBackupEndpointBacksUpToLoopbackRequestedDirectory(t *testing.T) {
	backer := &stubGroundItemStoreBacker{summary: map[string]any{"ground_item_count": 1, "vids": []uint32{7}}}
	mux := RegisterLocalGroundItemStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/backup", strings.NewReader(`{"dst_dir":"/tmp/ground-item-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if backer.calls != 1 || backer.lastDir != "/tmp/ground-item-backup" {
		t.Fatalf("unexpected backer state: calls=%d dir=%q", backer.calls, backer.lastDir)
	}
}

func TestLocalGroundItemStoreBackupEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	backer := &stubGroundItemStoreBacker{summary: map[string]any{"ground_item_count": 1}}
	mux := RegisterLocalGroundItemStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/backup", strings.NewReader(`{"dst_dir":"/tmp/ground-item-backup"}`))
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

func TestLocalGroundItemStoreBackupValidateEndpointDryRunsLoopbackRequestedSource(t *testing.T) {
	validator := &stubGroundItemStoreBackupValidator{summary: map[string]any{"ground_item_count": 1, "vids": []uint32{9}}}
	mux := RegisterLocalGroundItemStoreBackupValidateEndpoint(NewPprofMux("gamed"), validator.ValidateBackup)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/backup/validate", strings.NewReader(`{"src_dir":"/tmp/ground-item-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if validator.calls != 1 || validator.lastDir != "/tmp/ground-item-backup" {
		t.Fatalf("unexpected validator state: calls=%d dir=%q", validator.calls, validator.lastDir)
	}
}

func TestLocalGroundItemStoreRestoreEndpointRestoresFromLoopbackRequestedSource(t *testing.T) {
	restorer := &stubGroundItemStoreRestorer{summary: map[string]any{"ground_item_count": 1, "vids": []uint32{11}}}
	mux := RegisterLocalGroundItemStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/restore", strings.NewReader(`{"src_dir":"/tmp/ground-item-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if restorer.calls != 1 || restorer.lastDir != "/tmp/ground-item-backup" {
		t.Fatalf("unexpected restorer state: calls=%d dir=%q", restorer.calls, restorer.lastDir)
	}
}

func TestLocalGroundItemStoreRestoreEndpointRejectsNonLoopbackRemoteAddr(t *testing.T) {
	restorer := &stubGroundItemStoreRestorer{summary: map[string]any{"ground_item_count": 1}}
	mux := RegisterLocalGroundItemStoreRestoreEndpoint(NewPprofMux("gamed"), restorer.Restore)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/restore", strings.NewReader(`{"src_dir":"/tmp/ground-item-backup"}`))
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

func TestLocalGroundItemStoreCrashTempCleanupEndpointReturnsSummaryForLoopbackPost(t *testing.T) {
	cleaner := &stubGroundItemStoreValidator{summary: map[string]any{"ground_item_count": 0, "vids": []uint32{}}}
	mux := RegisterLocalGroundItemStoreCrashTempCleanupEndpoint(NewPprofMux("gamed"), cleaner.Validate)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/crash-temps/cleanup", nil)
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

func TestLocalGroundItemStoreBackupEndpointReturnsConflictOnBackupError(t *testing.T) {
	backer := &stubGroundItemStoreBacker{err: errors.New("backup failed")}
	mux := RegisterLocalGroundItemStoreBackupEndpoint(NewPprofMux("gamed"), backer.Backup)
	req := httptest.NewRequest(http.MethodPost, "/local/ground-item-store/backup", strings.NewReader(`{"dst_dir":"/tmp/ground-item-backup"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}
