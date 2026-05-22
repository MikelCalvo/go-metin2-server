package quickslot

import (
	"bytes"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func TestEncodeClientAddBuildsItemQuickslotFrame(t *testing.T) {
	got := EncodeClientAdd(ClientAddPacket{Position: 3, Slot: Slot{Type: TypeItem, Position: 17}})
	want := frame.Encode(HeaderClientAdd, []byte{3, TypeItem, 17})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client quickslot add frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientAddReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientAdd(frame.Frame{Header: HeaderClientAdd, Length: 7, Payload: []byte{3, TypeItem, 17}})
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientAddPacket{Position: 3, Slot: Slot{Type: TypeItem, Position: 17}}) {
		t.Fatalf("unexpected client quickslot add packet: %+v", packet)
	}
}

func TestEncodeClientDelBuildsFrame(t *testing.T) {
	got := EncodeClientDel(ClientDelPacket{Position: 3})
	want := frame.Encode(HeaderClientDel, []byte{3})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client quickslot del frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientDelReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientDel(frame.Frame{Header: HeaderClientDel, Length: 5, Payload: []byte{3}})
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientDelPacket{Position: 3}) {
		t.Fatalf("unexpected client quickslot del packet: %+v", packet)
	}
}

func TestEncodeClientSwapBuildsFrame(t *testing.T) {
	got := EncodeClientSwap(ClientSwapPacket{Position: 3, TargetPosition: 4})
	want := frame.Encode(HeaderClientSwap, []byte{3, 4})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected client quickslot swap frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeClientSwapReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeClientSwap(frame.Frame{Header: HeaderClientSwap, Length: 6, Payload: []byte{3, 4}})
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (ClientSwapPacket{Position: 3, TargetPosition: 4}) {
		t.Fatalf("unexpected client quickslot swap packet: %+v", packet)
	}
}

func TestDecodeClientAddRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeClientAdd(frame.Frame{Header: HeaderClientAdd + 1, Length: 7, Payload: []byte{3, TypeItem, 17}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeClientAddRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientAdd(frame.Frame{Header: HeaderClientAdd, Length: 6, Payload: []byte{3, TypeItem}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientDelRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientDel(frame.Frame{Header: HeaderClientDel, Length: 4, Payload: nil})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeClientSwapRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeClientSwap(frame.Frame{Header: HeaderClientSwap, Length: 5, Payload: []byte{3}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeAddBuildsItemQuickslotFrame(t *testing.T) {
	got := EncodeAdd(AddPacket{Position: 3, Slot: Slot{Type: TypeItem, Position: 17}})
	want := frame.Encode(HeaderAdd, []byte{3, TypeItem, 17})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected quickslot add frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeAddReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeAdd(frame.Frame{Header: HeaderAdd, Length: 7, Payload: []byte{3, TypeItem, 17}})
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (AddPacket{Position: 3, Slot: Slot{Type: TypeItem, Position: 17}}) {
		t.Fatalf("unexpected quickslot add packet: %+v", packet)
	}
}

func TestEncodeDelBuildsFrame(t *testing.T) {
	got := EncodeDel(DelPacket{Position: 3})
	want := frame.Encode(HeaderDel, []byte{3})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected quickslot del frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeDelReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeDel(frame.Frame{Header: HeaderDel, Length: 5, Payload: []byte{3}})
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (DelPacket{Position: 3}) {
		t.Fatalf("unexpected quickslot del packet: %+v", packet)
	}
}

func TestEncodeSwapBuildsFrame(t *testing.T) {
	got := EncodeSwap(SwapPacket{Position: 3, TargetPosition: 4})
	want := frame.Encode(HeaderSwap, []byte{3, 4})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected quickslot swap frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeSwapReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeSwap(frame.Frame{Header: HeaderSwap, Length: 6, Payload: []byte{3, 4}})
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != (SwapPacket{Position: 3, TargetPosition: 4}) {
		t.Fatalf("unexpected quickslot swap packet: %+v", packet)
	}
}

func TestDecodeAddRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeAdd(frame.Frame{Header: HeaderAdd + 1, Length: 7, Payload: []byte{3, TypeItem, 17}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeAddRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeAdd(frame.Frame{Header: HeaderAdd, Length: 6, Payload: []byte{3, TypeItem}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeDelRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeDel(frame.Frame{Header: HeaderDel, Length: 4, Payload: nil})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeSwapRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeSwap(frame.Frame{Header: HeaderSwap, Length: 5, Payload: []byte{3}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}
