package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Log sink startup env names. Service-specific values win over the shared
// name, matching other daemon overrides. An empty value is missing config
// and stays on local stdout. The value is a loopback UDP host:port only.
const (
	logSinkEnvShared = "METIN2_LOG_SINK"
	logSinkEnvAuthd  = "METIN2_AUTHD_LOG_SINK"
	logSinkEnvGamed  = "METIN2_GAMED_LOG_SINK"
)

// NewStartupServiceLogger chooses the already-owned loopback UDP log sink
// once, at authd or gamed startup.
//
// Missing sink config returns NewServiceLogger on local stdout and leaves
// the package sink disabled. A present loopback UDP host:port returns the
// same logger plus that copy. Remote hosts, syslog framing, schemes, and
// SIEM targets are refused: the error is the constant remote-log refusal
// and does not echo the endpoint. The endpoint is never written into a
// log record.
func NewStartupServiceLogger(serviceName string, w io.Writer) (*slog.Logger, error) {
	endpoint, configured, err := startupLogSinkEndpoint(serviceName)
	if err != nil {
		return nil, err
	}
	if !configured {
		return NewServiceLogger(serviceName, w), nil
	}
	return NewServiceLoggerWithRemoteSink(serviceName, w, endpoint)
}

func startupLogSinkEndpoint(serviceName string) (string, bool, error) {
	raw, ok := lookupStartupLogSink(serviceName)
	if !ok {
		return "", false, nil
	}
	endpoint, accepted := loopbackLogEndpoint(raw)
	if !accepted {
		return "", false, fmt.Errorf("%w", errRemoteLogSinkRefused)
	}
	return endpoint, true, nil
}

func lookupStartupLogSink(serviceName string) (string, bool) {
	for _, key := range startupLogSinkEnvKeys(serviceName) {
		raw, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		if strings.TrimSpace(raw) == "" {
			return "", false
		}
		return raw, true
	}
	return "", false
}

func startupLogSinkEnvKeys(serviceName string) []string {
	switch strings.ToLower(strings.TrimSpace(serviceName)) {
	case "authd":
		return []string{logSinkEnvAuthd, logSinkEnvShared}
	case "gamed":
		return []string{logSinkEnvGamed, logSinkEnvShared}
	default:
		return []string{logSinkEnvShared}
	}
}
