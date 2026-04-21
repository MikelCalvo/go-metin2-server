package worldruntime

import "testing"

func TestSessionDirectoryRegistersAndLooksUpTransportHooks(t *testing.T) {
	directory := NewSessionDirectory()
	sink := &stubSessionFrameSink{}
	binding := SessionEntry{
		FrameSink: sink,
		Relocator: func(mapIndex uint32, x int32, y int32) (any, bool) {
			return relocateCall{MapIndex: mapIndex, X: x, Y: y, Tag: "alpha"}, true
		},
	}

	if !directory.Register(1, binding) {
		t.Fatal("expected session directory registration to succeed")
	}
	lookup, ok := directory.Lookup(1)
	if !ok || lookup.FrameSink == nil || lookup.Relocator == nil {
		t.Fatalf("expected session lookup to return registered transport hooks, got entry=%+v ok=%v", lookup, ok)
	}

	lookup.FrameSink.Enqueue([][]byte{{0x01, 0x02}})
	if sink.enqueueCalls != 1 || len(sink.frames) != 1 || len(sink.frames[0]) != 1 {
		t.Fatalf("expected looked-up frame sink to forward enqueued frames, got calls=%d frames=%+v", sink.enqueueCalls, sink.frames)
	}
	result, ok := lookup.Relocator(42, 1700, 2800)
	if !ok {
		t.Fatal("expected looked-up relocator to report success")
	}
	call, ok := result.(relocateCall)
	if !ok || call.MapIndex != 42 || call.X != 1700 || call.Y != 2800 || call.Tag != "alpha" {
		t.Fatalf("expected looked-up relocator to preserve transport callback behavior, got %#v", result)
	}
}

func TestSessionDirectoryReplaceUpdatesTransportHooks(t *testing.T) {
	directory := NewSessionDirectory()
	originalSink := &stubSessionFrameSink{}
	replacementSink := &stubSessionFrameSink{}
	if !directory.Register(1, SessionEntry{FrameSink: originalSink, Relocator: func(mapIndex uint32, x int32, y int32) (any, bool) {
		return "original", true
	}}) {
		t.Fatal("expected initial session registration to succeed")
	}

	replacement := SessionEntry{
		FrameSink: replacementSink,
		Relocator: func(mapIndex uint32, x int32, y int32) (any, bool) {
			return relocateCall{MapIndex: mapIndex, X: x, Y: y, Tag: "replacement"}, true
		},
	}
	if !directory.Replace(1, replacement) {
		t.Fatal("expected session replace to succeed")
	}

	lookup, ok := directory.Lookup(1)
	if !ok {
		t.Fatal("expected lookup to succeed after replace")
	}
	lookup.FrameSink.Enqueue([][]byte{{0x03}})
	if originalSink.enqueueCalls != 0 {
		t.Fatalf("expected replace to stop using original sink, got %d calls", originalSink.enqueueCalls)
	}
	if replacementSink.enqueueCalls != 1 {
		t.Fatalf("expected replace to use replacement sink, got %d calls", replacementSink.enqueueCalls)
	}
	result, ok := lookup.Relocator(7, 1800, 2900)
	if !ok {
		t.Fatal("expected replacement relocator to report success")
	}
	call, ok := result.(relocateCall)
	if !ok || call.Tag != "replacement" || call.MapIndex != 7 {
		t.Fatalf("expected replacement relocator to be returned from lookup, got %#v", result)
	}
}

func TestSessionDirectoryRemoveClearsTransportHooks(t *testing.T) {
	directory := NewSessionDirectory()
	sink := &stubSessionFrameSink{}
	binding := SessionEntry{FrameSink: sink}
	if !directory.Register(1, binding) {
		t.Fatal("expected session directory registration to succeed")
	}

	removed, ok := directory.Remove(1)
	if !ok || removed.FrameSink == nil {
		t.Fatalf("expected remove to return registered session entry, got entry=%+v ok=%v", removed, ok)
	}
	if _, ok := directory.Lookup(1); ok {
		t.Fatal("expected lookup to fail after remove")
	}
}

func TestSessionDirectoryRejectsInvalidRegisterAndReplaceOperations(t *testing.T) {
	directory := NewSessionDirectory()
	if directory.Register(0, SessionEntry{FrameSink: &stubSessionFrameSink{}}) {
		t.Fatal("expected zero entity id registration to fail")
	}
	if directory.Register(1, SessionEntry{}) {
		t.Fatal("expected empty session entry registration to fail")
	}
	if !directory.Register(1, SessionEntry{FrameSink: &stubSessionFrameSink{}}) {
		t.Fatal("expected valid session entry registration to succeed")
	}
	if directory.Register(1, SessionEntry{FrameSink: &stubSessionFrameSink{}}) {
		t.Fatal("expected duplicate session registration to fail")
	}
	if directory.Replace(2, SessionEntry{FrameSink: &stubSessionFrameSink{}}) {
		t.Fatal("expected replace on missing entity id to fail")
	}
	if directory.Replace(1, SessionEntry{}) {
		t.Fatal("expected replace with empty session entry to fail")
	}
}

type stubSessionFrameSink struct {
	enqueueCalls int
	frames       [][][]byte
}

func (s *stubSessionFrameSink) Enqueue(frames [][]byte) {
	s.enqueueCalls++
	cloned := make([][]byte, 0, len(frames))
	for _, raw := range frames {
		cloned = append(cloned, append([]byte(nil), raw...))
	}
	s.frames = append(s.frames, cloned)
}

type relocateCall struct {
	MapIndex uint32
	X        int32
	Y        int32
	Tag      string
}
