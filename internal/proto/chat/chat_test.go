package chat

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

func TestEncodeDecodeClientChat(t *testing.T) {
	raw := EncodeClientChat(ClientChatPacket{Type: ChatTypeTalking, Message: "hola"})
	decoded, err := DecodeClientChat(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode client chat: %v", err)
	}
	if decoded.Type != ChatTypeTalking || decoded.Message != "hola" {
		t.Fatalf("unexpected decoded client chat: %+v", decoded)
	}
}

func TestEncodeDecodeChatDelivery(t *testing.T) {
	raw := EncodeChatDelivery(ChatDeliveryPacket{Type: ChatTypeTalking, VID: 0x02040102, Empire: 0, Message: "PeerTwo : hola"})
	decoded, err := DecodeChatDelivery(decodeSingleFrame(t, raw))
	if err != nil {
		t.Fatalf("decode chat delivery: %v", err)
	}
	if decoded.Type != ChatTypeTalking || decoded.VID != 0x02040102 || decoded.Empire != 0 || decoded.Message != "PeerTwo : hola" {
		t.Fatalf("unexpected decoded chat delivery: %+v", decoded)
	}
}

func decodeSingleFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()
	decoder := frame.NewDecoder(4096)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	return frames[0]
}
