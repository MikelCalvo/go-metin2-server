package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

type SessionFlow interface {
	Start() ([][]byte, error)
	HandleClientFrame(frame.Frame) ([][]byte, error)
}

type SessionFactory func() SessionFlow

func ListenAndServeLegacy(ctx context.Context, addr string, logger *slog.Logger, newSession SessionFactory) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen legacy tcp: %w", err)
	}

	if logger != nil {
		logger.Info("legacy server listening", "addr", listener.Addr().String())
	}

	return ServeLegacy(ctx, listener, logger, newSession)
}

func ServeLegacy(ctx context.Context, listener net.Listener, logger *slog.Logger, newSession SessionFactory) error {
	if newSession == nil {
		defer listener.Close()
		<-ctx.Done()
		return nil
	}

	defer listener.Close()
	tracker := newConnTracker()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		tracker.closeAll()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				tracker.wait()
				return nil
			}
			tracker.closeAll()
			tracker.wait()
			return fmt.Errorf("accept legacy tcp: %w", err)
		}

		tracker.add(conn)
		go func(conn net.Conn) {
			defer conn.Close()
			defer tracker.done(conn)

			if err := serveLegacyConn(ctx, conn, newSession()); err != nil && logger != nil {
				logger.Warn("legacy session closed with error", "remote_addr", conn.RemoteAddr().String(), "err", err)
			}
		}(conn)
	}
}

func serveLegacyConn(ctx context.Context, conn net.Conn, flow SessionFlow) error {
	out, err := flow.Start()
	if err != nil {
		return fmt.Errorf("start session flow: %w", err)
	}

	for _, raw := range out {
		if err := writeLegacyFrame(conn, raw); err != nil {
			return fmt.Errorf("write session start frame: %w", err)
		}
	}

	decoder := frame.NewDecoder(8192)
	buffer := make([]byte, 8192)

	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		n, err := conn.Read(buffer)
		if n > 0 {
			frames, decodeErr := decoder.Feed(buffer[:n])
			if decodeErr != nil {
				return fmt.Errorf("decode legacy frame: %w", decodeErr)
			}

			for _, incoming := range frames {
				out, handleErr := flow.HandleClientFrame(incoming)
				if handleErr != nil {
					return handleErr
				}

				for _, raw := range out {
					if writeErr := writeLegacyFrame(conn, raw); writeErr != nil {
						return fmt.Errorf("write legacy frame: %w", writeErr)
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return nil
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}

			return fmt.Errorf("read legacy frame: %w", err)
		}
	}
}

func writeLegacyFrame(conn net.Conn, raw []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	for len(raw) > 0 {
		n, err := conn.Write(raw)
		if err != nil {
			return err
		}
		raw = raw[n:]
	}

	return nil
}

type connTracker struct {
	mu    sync.Mutex
	conns map[net.Conn]struct{}
	wg    sync.WaitGroup
}

func newConnTracker() *connTracker {
	return &connTracker{conns: make(map[net.Conn]struct{})}
}

func (t *connTracker) add(conn net.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conns[conn] = struct{}{}
	t.wg.Add(1)
}

func (t *connTracker) done(conn net.Conn) {
	t.mu.Lock()
	delete(t.conns, conn)
	t.mu.Unlock()
	t.wg.Done()
}

func (t *connTracker) closeAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for conn := range t.conns {
		_ = conn.Close()
	}
}

func (t *connTracker) wait() {
	t.wg.Wait()
}
