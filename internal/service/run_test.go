package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	testHeaderStart uint16 = 0x9001
	testHeaderPing  uint16 = 0x9002
	testHeaderPong  uint16 = 0x9003
	testHeaderAsync uint16 = 0x9004
)

func TestServeLegacyServesSessionFlowOverTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), newTestSessionFlow)
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	start := readFrame(t, conn)
	if start.Header != testHeaderStart || string(start.Payload) != "hello" {
		t.Fatalf("unexpected start frame: header=0x%04x payload=%q", start.Header, string(start.Payload))
	}

	writeFrame(t, conn, frame.Encode(testHeaderPing, []byte("ping")))

	pong := readFrame(t, conn)
	if pong.Header != testHeaderPong || string(pong.Payload) != "pong" {
		t.Fatalf("unexpected pong frame: header=0x%04x payload=%q", pong.Header, string(pong.Payload))
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve legacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyClosesActiveConnectionsBeforeReturning(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), newTestSessionFlow)
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	_ = readFrame(t, conn)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve legacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}

	expectConnectionClose(t, conn)
}

func TestRunStartsOpsAndLegacyServers(t *testing.T) {
	pprofAddr := reserveLocalAddr(t)
	legacyAddr := reserveLocalAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, config.Service{
			Name:       "authd",
			PprofAddr:  pprofAddr,
			LegacyAddr: legacyAddr,
			PublicAddr: "127.0.0.1",
		}, testLogger(), newTestSessionFlow)
	}()

	waitForHealthz(t, pprofAddr, "authd ok\n")

	conn, err := net.Dial("tcp", legacyAddr)
	if err != nil {
		t.Fatalf("dial legacy tcp: %v", err)
	}

	start := readFrame(t, conn)
	if start.Header != testHeaderStart || string(start.Payload) != "hello" {
		_ = conn.Close()
		t.Fatalf("unexpected start frame: header=0x%04x payload=%q", start.Header, string(start.Payload))
	}

	writeFrame(t, conn, frame.Encode(testHeaderPing, []byte("ping")))
	pong := readFrame(t, conn)
	if pong.Header != testHeaderPong || string(pong.Payload) != "pong" {
		_ = conn.Close()
		t.Fatalf("unexpected pong frame: header=0x%04x payload=%q", pong.Header, string(pong.Payload))
	}
	_ = conn.Close()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for Run to stop")
	}
}

func TestServeLegacyFlushesServerInitiatedFramesWithoutIncomingTraffic(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), newAsyncTestSessionFlow)
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	frames := readFrames(t, conn, 2)
	start := frames[0]
	if start.Header != testHeaderStart || string(start.Payload) != "hello" {
		t.Fatalf("unexpected start frame: header=0x%04x payload=%q", start.Header, string(start.Payload))
	}

	async := frames[1]
	if async.Header != testHeaderAsync || string(async.Payload) != "async" {
		t.Fatalf("unexpected async frame: header=0x%04x payload=%q", async.Header, string(async.Payload))
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve legacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

func TestServeLegacyCallsFlowCloserWhenConnectionEnds(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flow := newClosableTestSessionFlow()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeLegacy(ctx, listener, testLogger(), func() SessionFlow { return flow })
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}

	_ = readFrame(t, conn)
	_ = conn.Close()

	select {
	case <-flow.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for flow close hook")
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve legacy returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ServeLegacy to stop")
	}
}

type testSessionFlow struct{}

type asyncTestSessionFlow struct {
	pending [][]byte
}

type closableTestSessionFlow struct {
	closed chan struct{}
	once   sync.Once
}

func newTestSessionFlow() SessionFlow {
	return &testSessionFlow{}
}

func newAsyncTestSessionFlow() SessionFlow {
	return &asyncTestSessionFlow{pending: [][]byte{frame.Encode(testHeaderAsync, []byte("async"))}}
}

func newClosableTestSessionFlow() *closableTestSessionFlow {
	return &closableTestSessionFlow{closed: make(chan struct{})}
}

func (f *testSessionFlow) Start() ([][]byte, error) {
	return [][]byte{frame.Encode(testHeaderStart, []byte("hello"))}, nil
}

func (f *testSessionFlow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if in.Header != testHeaderPing {
		return nil, fmt.Errorf("unexpected header: 0x%04x", in.Header)
	}
	if string(in.Payload) != "ping" {
		return nil, fmt.Errorf("unexpected payload: %q", string(in.Payload))
	}
	return [][]byte{frame.Encode(testHeaderPong, []byte("pong"))}, nil
}

func (f *asyncTestSessionFlow) Start() ([][]byte, error) {
	return [][]byte{frame.Encode(testHeaderStart, []byte("hello"))}, nil
}

func (f *asyncTestSessionFlow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	return nil, fmt.Errorf("unexpected header: 0x%04x", in.Header)
}

func (f *asyncTestSessionFlow) FlushServerFrames() ([][]byte, error) {
	out := f.pending
	f.pending = nil
	return out, nil
}

func (f *closableTestSessionFlow) Start() ([][]byte, error) {
	return [][]byte{frame.Encode(testHeaderStart, []byte("hello"))}, nil
}

func (f *closableTestSessionFlow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	return nil, fmt.Errorf("unexpected header: 0x%04x", in.Header)
}

func (f *closableTestSessionFlow) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func reserveLocalAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local addr: %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

func waitForHealthz(t *testing.T, addr string, want string) {
	t.Helper()

	client := &http.Client{Timeout: 250 * time.Millisecond}
	url := "http://" + addr + "/healthz"
	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			time.Sleep(25 * time.Millisecond)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr == nil && resp.StatusCode == http.StatusOK && string(body) == want {
			return
		}

		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("healthz %s did not become ready", url)
}

func readFrame(t *testing.T, conn net.Conn) frame.Frame {
	t.Helper()
	return readFrames(t, conn, 1)[0]
}

func readFrames(t *testing.T, conn net.Conn, count int) []frame.Frame {
	t.Helper()

	decoder := frame.NewDecoder(1024)
	buffer := make([]byte, 1024)
	frames := make([]frame.Frame, 0, count)

	for len(frames) < count {
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}

		n, err := conn.Read(buffer)
		if err != nil {
			t.Fatalf("read frame: %v", err)
		}

		decoded, err := decoder.Feed(buffer[:n])
		if err != nil {
			t.Fatalf("decode frame: %v", err)
		}

		frames = append(frames, decoded...)
	}

	return frames[:count]
}

func writeFrame(t *testing.T, conn net.Conn, raw []byte) {
	t.Helper()

	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set write deadline: %v", err)
	}

	for len(raw) > 0 {
		n, err := conn.Write(raw)
		if err != nil {
			t.Fatalf("write frame: %v", err)
		}
		raw = raw[n:]
	}
}

func expectConnectionClose(t *testing.T, conn net.Conn) {
	t.Helper()

	buffer := make([]byte, 1)
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	_, err := conn.Read(buffer)
	if err == nil {
		t.Fatal("expected connection close, but read succeeded")
	}

	if isTimeout(err) {
		t.Fatalf("expected connection close, got timeout instead: %v", err)
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
