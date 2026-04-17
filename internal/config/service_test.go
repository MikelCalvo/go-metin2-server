package config

import "testing"

func TestLoadServiceUsesDefaultWhenEnvIsMissing(t *testing.T) {
    t.Setenv("METIN2_PPROF_ADDR", "")
    t.Setenv("METIN2_GAMED_PPROF_ADDR", "")

    cfg := LoadService("gamed", ":6060")
    if cfg.PprofAddr != ":6060" {
        t.Fatalf("expected default pprof addr, got %q", cfg.PprofAddr)
    }
}

func TestLoadServiceUsesGlobalOverride(t *testing.T) {
    t.Setenv("METIN2_PPROF_ADDR", ":9999")
    t.Setenv("METIN2_GAMED_PPROF_ADDR", "")

    cfg := LoadService("gamed", ":6060")
    if cfg.PprofAddr != ":9999" {
        t.Fatalf("expected global pprof addr, got %q", cfg.PprofAddr)
    }
}

func TestLoadServicePrefersServiceSpecificOverride(t *testing.T) {
    t.Setenv("METIN2_PPROF_ADDR", ":9999")
    t.Setenv("METIN2_GAMED_PPROF_ADDR", ":6067")

    cfg := LoadService("gamed", ":6060")
    if cfg.PprofAddr != ":6067" {
        t.Fatalf("expected service-specific pprof addr, got %q", cfg.PprofAddr)
    }
}
