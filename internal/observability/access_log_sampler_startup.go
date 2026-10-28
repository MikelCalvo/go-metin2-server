package observability

import (
	"errors"
	"os"
	"strings"
	"time"
)

const (
	accessLogSampleEnvShared = "METIN2_ACCESS_LOG_SAMPLE_INTERVAL"
	accessLogSampleEnvAuthd  = "METIN2_AUTHD_ACCESS_LOG_SAMPLE_INTERVAL"
	accessLogSampleEnvGamed  = "METIN2_GAMED_ACCESS_LOG_SAMPLE_INTERVAL"
)

var errAccessLogSampleIntervalRefused = errors.New("access log sampler interval refused")

// ConfigureStartupAccessLogSampler selects the existing access-line sampler
// once, before authd or gamed serves requests. A blank service override
// disables sampling even when the shared value is set. Missing config keeps
// the existing every-request access line. Invalid durations refuse startup
// without echoing their value or mutating the sampler.
func ConfigureStartupAccessLogSampler(serviceName string) error {
	raw := startupAccessLogSampleInterval(serviceName)
	if strings.TrimSpace(raw) == "" {
		SetAccessLogSampler(nil)
		return nil
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		return errAccessLogSampleIntervalRefused
	}
	SetAccessLogSampler(&AccessLogSampler{Every: interval})
	return nil
}

func startupAccessLogSampleInterval(serviceName string) string {
	var serviceKey string
	switch strings.ToLower(strings.TrimSpace(serviceName)) {
	case "authd":
		serviceKey = accessLogSampleEnvAuthd
	case "gamed":
		serviceKey = accessLogSampleEnvGamed
	}
	if serviceKey != "" {
		if raw, ok := os.LookupEnv(serviceKey); ok {
			return raw
		}
	}
	return os.Getenv(accessLogSampleEnvShared)
}
