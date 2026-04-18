package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	stdpprof "net/http/pprof"
	"strings"
)

type localRelocationRequest struct {
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

func NewPprofMux(serviceName string) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeSnapshot(serviceName, nil, nil, nil)
}

func NewPprofMuxWithLocalNotice(serviceName string, broadcastNotice func(string) int) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeSnapshot(serviceName, broadcastNotice, nil, nil)
}

func NewPprofMuxWithLocalRelocation(serviceName string, broadcastNotice func(string) int, relocateCharacter func(string, uint32, int32, int32) bool) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeSnapshot(serviceName, broadcastNotice, relocateCharacter, nil)
}

func NewPprofMuxWithLocalRuntimeSnapshot(serviceName string, broadcastNotice func(string) int, relocateCharacter func(string, uint32, int32, int32) bool, connectedCharacters func() any) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "%s ok\n", serviceName)
	})

	mux.HandleFunc("/debug/pprof/", stdpprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", stdpprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", stdpprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", stdpprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", stdpprof.Trace)
	mux.Handle("/debug/pprof/allocs", stdpprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", stdpprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", stdpprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", stdpprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", stdpprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", stdpprof.Handler("threadcreate"))

	if broadcastNotice != nil {
		mux.HandleFunc("/local/notice", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			message := strings.TrimSpace(string(body))
			if message == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintf(w, "queued %d\n", broadcastNotice(message))
		})
	}

	if relocateCharacter != nil {
		mux.HandleFunc("/local/relocate", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			var request localRelocationRequest
			decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var trailing struct{}
			if err := decoder.Decode(&trailing); err != io.EOF {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			request.Name = strings.TrimSpace(request.Name)
			if request.Name == "" || request.MapIndex == 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !relocateCharacter(request.Name, request.MapIndex, request.X, request.Y) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "relocated 1\n")
		})
	}

	if connectedCharacters != nil {
		mux.HandleFunc("/local/players", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(connectedCharacters()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}

	return mux
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
