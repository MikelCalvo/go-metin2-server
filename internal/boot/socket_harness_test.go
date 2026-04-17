package boot

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

type bootTestServer struct {
	listener net.Listener
	done     chan error
	phase    atomic.Value
}

type bootTestClient struct {
	conn    net.Conn
	decoder *frame.Decoder
	queued  []bootWireFrame
}

type bootWireFrame struct {
	Frame frame.Frame
	Raw   []byte
}

func startBootTestServer(t *testing.T, cfg Config) *bootTestServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}

	server := &bootTestServer{
		listener: listener,
		done:     make(chan error, 1),
	}
	server.phase.Store(session.PhaseHandshake)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			server.done <- err
			return
		}
		defer conn.Close()

		server.done <- runBootConn(conn, cfg, &server.phase)
	}()

	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case err := <-server.done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("boot test server error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for boot test server to stop")
		}
	})

	return server
}

func (s *bootTestServer) address() string {
	return s.listener.Addr().String()
}

func (s *bootTestServer) currentPhase() session.Phase {
	value := s.phase.Load()
	if value == nil {
		return ""
	}

	return value.(session.Phase)
}

func newBootTestClient(t *testing.T, address string) *bootTestClient {
	t.Helper()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}

	client := &bootTestClient{
		conn:    conn,
		decoder: frame.NewDecoder(8192),
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return client
}

func (c *bootTestClient) readFrame(t *testing.T) bootWireFrame {
	t.Helper()

	if len(c.queued) > 0 {
		frame := c.queued[0]
		c.queued = c.queued[1:]
		return frame
	}

	buffer := make([]byte, 8192)

	for {
		if err := c.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}

		n, err := c.conn.Read(buffer)
		if err != nil {
			t.Fatalf("read frame: %v", err)
		}

		frames, err := c.decoder.Feed(buffer[:n])
		if err != nil {
			t.Fatalf("decode frame: %v", err)
		}

		if len(frames) == 0 {
			continue
		}

		for _, decoded := range frames {
			c.queued = append(c.queued, bootWireFrame{
				Frame: decoded,
				Raw:   frame.Encode(decoded.Header, decoded.Payload),
			})
		}

		frame := c.queued[0]
		c.queued = c.queued[1:]
		return frame
	}
}

func (c *bootTestClient) writeFrame(t *testing.T, raw []byte) {
	t.Helper()

	if err := writeAll(c.conn, raw); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func runBootConn(conn net.Conn, cfg Config, phaseStore *atomic.Value) error {
	flow := NewFlow(cfg)
	phaseStore.Store(flow.CurrentPhase())

	out, err := flow.Start()
	if err != nil {
		return err
	}

	for _, raw := range out {
		if err := writeAll(conn, raw); err != nil {
			return err
		}
	}

	decoder := frame.NewDecoder(8192)
	buffer := make([]byte, 8192)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		frames, err := decoder.Feed(buffer[:n])
		if err != nil {
			return err
		}

		for _, incoming := range frames {
			out, err := flow.HandleClientFrame(incoming)
			phaseStore.Store(flow.CurrentPhase())
			if err != nil {
				return err
			}

			for _, raw := range out {
				if err := writeAll(conn, raw); err != nil {
					return err
				}
			}

			phaseStore.Store(flow.CurrentPhase())
			if flow.CurrentPhase() == session.PhaseGame {
				return nil
			}
		}
	}
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}

	return nil
}
