package handshake

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

type handshakeTestServer struct {
	listener net.Listener
	done     chan error
	phase    atomic.Value
}

type handshakeTestClient struct {
	conn    net.Conn
	decoder *frame.Decoder
	queued  []wireFrame
}

type wireFrame struct {
	Frame frame.Frame
	Raw   []byte
}

func startHandshakeTestServer(t *testing.T, cfg Config) *handshakeTestServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}

	server := &handshakeTestServer{
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

		server.done <- runHandshakeConn(conn, cfg, &server.phase)
	}()

	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case err := <-server.done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("handshake test server error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for handshake test server to stop")
		}
	})

	return server
}

func (s *handshakeTestServer) address() string {
	return s.listener.Addr().String()
}

func (s *handshakeTestServer) currentPhase() session.Phase {
	value := s.phase.Load()
	if value == nil {
		return ""
	}

	return value.(session.Phase)
}

func newHandshakeTestClient(t *testing.T, address string) *handshakeTestClient {
	t.Helper()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}

	client := &handshakeTestClient{
		conn:    conn,
		decoder: frame.NewDecoder(4096),
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return client
}

func (c *handshakeTestClient) readFrame(t *testing.T) wireFrame {
	t.Helper()

	if len(c.queued) > 0 {
		frame := c.queued[0]
		c.queued = c.queued[1:]
		return frame
	}

	buffer := make([]byte, 4096)

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
			c.queued = append(c.queued, wireFrame{
				Frame: decoded,
				Raw:   frame.Encode(decoded.Header, decoded.Payload),
			})
		}

		frame := c.queued[0]
		c.queued = c.queued[1:]
		return frame
	}
}

func (c *handshakeTestClient) writeFrame(t *testing.T, raw []byte) {
	t.Helper()

	if err := writeAll(c.conn, raw); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}

func (c *handshakeTestClient) expectNoFrameWithin(t *testing.T, timeout time.Duration) {
	t.Helper()

	if len(c.queued) > 0 {
		t.Fatalf("expected no frame, but %d frame(s) were already queued", len(c.queued))
	}

	buffer := make([]byte, 4096)

	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	n, err := c.conn.Read(buffer)
	if err != nil {
		if isTimeout(err) {
			return
		}
		t.Fatalf("expect no frame read error: %v", err)
	}

	frames, decodeErr := c.decoder.Feed(buffer[:n])
	if decodeErr != nil {
		t.Fatalf("decode unexpected frame: %v", decodeErr)
	}

	if len(frames) > 0 {
		t.Fatalf("expected no frame within %s, got %d frame(s)", timeout, len(frames))
	}

	t.Fatalf("expected no frame within %s, got %d raw bytes", timeout, n)
}

func (c *handshakeTestClient) expectConnectionClose(t *testing.T) {
	t.Helper()

	buffer := make([]byte, 1)

	if err := c.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	_, err := c.conn.Read(buffer)
	if err == nil {
		t.Fatalf("expected connection close, but read succeeded")
	}

	if isTimeout(err) {
		t.Fatalf("expected connection close, got timeout instead")
	}
}

func runHandshakeConn(conn net.Conn, cfg Config, phaseStore *atomic.Value) error {
	machine := session.NewStateMachine()
	phaseStore.Store(machine.Current())

	flow := NewFlow(machine, cfg)
	out, err := flow.Start()
	if err != nil {
		return err
	}

	for _, raw := range out {
		if err := writeAll(conn, raw); err != nil {
			return err
		}
	}

	decoder := frame.NewDecoder(4096)
	buffer := make([]byte, 4096)

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
			phaseStore.Store(machine.Current())
			if err != nil {
				if isProtocolClose(err) {
					return nil
				}
				return err
			}

			for _, raw := range out {
				if err := writeAll(conn, raw); err != nil {
					return err
				}
			}

			phaseStore.Store(machine.Current())
			if machine.Current() != session.PhaseHandshake {
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

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isProtocolClose(err error) bool {
	return errors.Is(err, ErrKeyResponseRejected) ||
		errors.Is(err, ErrUnexpectedClientPacket) ||
		errors.Is(err, ErrInvalidPhase) ||
		errors.Is(err, ErrHandshakeNotStarted) ||
		errors.Is(err, control.ErrInvalidPayload) ||
		errors.Is(err, control.ErrUnexpectedHeader)
}

func (f wireFrame) String() string {
	return fmt.Sprintf("header=0x%04x length=%d raw=%x", f.Frame.Header, f.Frame.Length, f.Raw)
}
