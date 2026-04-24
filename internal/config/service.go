package config

import (
	"os"
	"strconv"
	"strings"
)

type Service struct {
	Name                 string
	PprofAddr            string
	LegacyAddr           string
	PublicAddr           string
	VisibilityMode       string
	VisibilityRadius     int32
	VisibilitySectorSize int32
}

func LoadService(name string, defaultPprofAddr string, defaultLegacyAddr string, defaultPublicAddr string) Service {
	upperName := strings.ToUpper(name)

	return Service{
		Name:                 name,
		PprofAddr:            loadOverride(upperName, "PPROF_ADDR", defaultPprofAddr),
		LegacyAddr:           loadOverride(upperName, "LEGACY_ADDR", defaultLegacyAddr),
		PublicAddr:           loadOverride(upperName, "PUBLIC_ADDR", defaultPublicAddr),
		VisibilityMode:       loadVisibilityModeOverride(upperName, "whole_map"),
		VisibilityRadius:     loadInt32Override(upperName, "VISIBILITY_RADIUS", 0),
		VisibilitySectorSize: loadInt32Override(upperName, "VISIBILITY_SECTOR_SIZE", 0),
	}
}

func loadOverride(upperName string, suffix string, fallback string) string {
	if value, ok := loadRawOverride(upperName, suffix); ok {
		return value
	}
	return fallback
}

func loadVisibilityModeOverride(upperName string, fallback string) string {
	value := loadOverride(upperName, "VISIBILITY_MODE", fallback)
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	if value == "" {
		return fallback
	}
	return value
}

func loadInt32Override(upperName string, suffix string, fallback int32) int32 {
	value, ok := loadRawOverride(upperName, suffix)
	if !ok {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return -1
	}
	return int32(parsed)
}

func loadRawOverride(upperName string, suffix string) (string, bool) {
	if serviceValue := os.Getenv("METIN2_" + upperName + "_" + suffix); serviceValue != "" {
		return serviceValue, true
	}

	if globalValue := os.Getenv("METIN2_" + suffix); globalValue != "" {
		return globalValue, true
	}

	return "", false
}
