package config

import (
	"os"
	"strings"
)

type Service struct {
	Name       string
	PprofAddr  string
	LegacyAddr string
	PublicAddr string
}

func LoadService(name string, defaultPprofAddr string, defaultLegacyAddr string, defaultPublicAddr string) Service {
	upperName := strings.ToUpper(name)

	return Service{
		Name:       name,
		PprofAddr:  loadOverride(upperName, "PPROF_ADDR", defaultPprofAddr),
		LegacyAddr: loadOverride(upperName, "LEGACY_ADDR", defaultLegacyAddr),
		PublicAddr: loadOverride(upperName, "PUBLIC_ADDR", defaultPublicAddr),
	}
}

func loadOverride(upperName string, suffix string, fallback string) string {
	if serviceValue := os.Getenv("METIN2_" + upperName + "_" + suffix); serviceValue != "" {
		return serviceValue
	}

	if globalValue := os.Getenv("METIN2_" + suffix); globalValue != "" {
		return globalValue
	}

	return fallback
}
