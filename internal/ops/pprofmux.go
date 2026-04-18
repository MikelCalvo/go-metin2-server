package ops

import (
	"fmt"
	"io"
	"net"
	"net/http"
	stdpprof "net/http/pprof"
	"strings"
)

func NewPprofMux(serviceName string) *http.ServeMux {
	return NewPprofMuxWithLocalNotice(serviceName, nil)
}

func NewPprofMuxWithLocalNotice(serviceName string, broadcastNotice func(string) int) *http.ServeMux {
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
