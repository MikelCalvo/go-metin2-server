package control

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func loadHexFixture(t *testing.T, name string) []byte {
	t.Helper()

	path := filepath.Join("testdata", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	decoded, err := hex.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}

	return decoded
}

func TestEncodePhaseBuildsAControlFrame(t *testing.T) {
	want := loadHexFixture(t, "phase-login-frame.hex")

	got, err := EncodePhase(session.PhaseLogin)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected phase frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePhaseReturnsTheExpectedSessionPhase(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "phase-login-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodePhase(frames[0])
	if err != nil {
		t.Fatalf("unexpected phase decode error: %v", err)
	}

	if packet.Phase != session.PhaseLogin {
		t.Fatalf("unexpected phase: got %q want %q", packet.Phase, session.PhaseLogin)
	}
}

func TestDecodePhaseRejectsUnknownPhaseValues(t *testing.T) {
	badFrame := frame.Frame{Header: HeaderPhase, Length: 5, Payload: []byte{0xff}}

	_, err := DecodePhase(badFrame)
	if !errors.Is(err, ErrUnknownPhaseValue) {
		t.Fatalf("expected ErrUnknownPhaseValue, got %v", err)
	}
}

func TestDecodePingReturnsServerTime(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "ping-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodePing(frames[0])
	if err != nil {
		t.Fatalf("unexpected ping decode error: %v", err)
	}

	if packet.ServerTime != 0x01020304 {
		t.Fatalf("unexpected server time: got %#08x want %#08x", packet.ServerTime, uint32(0x01020304))
	}
}

func TestEncodePingBuildsAControlFrame(t *testing.T) {
	want := loadHexFixture(t, "ping-frame.hex")
	got := EncodePing(PingPacket{ServerTime: 0x01020304})

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected ping frame bytes: got %x want %x", got, want)
	}
}

func TestEncodePhaseSupportsDeadPhaseValue(t *testing.T) {
	want := loadHexFixture(t, "phase-dead-frame.hex")

	got, err := EncodePhase(session.PhaseDead)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected dead phase frame bytes: got %x want %x", got, want)
	}
}

func TestEncodePhaseSupportsAuthPhaseValue(t *testing.T) {
	want := loadHexFixture(t, "phase-auth-frame.hex")

	got, err := EncodePhase(session.PhaseAuth)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected auth phase frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePhaseReturnsTheExpectedAuthSessionPhase(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "phase-auth-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodePhase(frames[0])
	if err != nil {
		t.Fatalf("unexpected phase decode error: %v", err)
	}

	if packet.Phase != session.PhaseAuth {
		t.Fatalf("unexpected phase: got %q want %q", packet.Phase, session.PhaseAuth)
	}
}

func TestEncodePongBuildsAHeaderOnlyControlFrame(t *testing.T) {
	want := loadHexFixture(t, "pong-frame.hex")
	got := EncodePong()

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected pong frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePongAcceptsAHeaderOnlyControlFrame(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "pong-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	if _, err := DecodePong(frames[0]); err != nil {
		t.Fatalf("unexpected pong decode error: %v", err)
	}
}

func TestDecodePongRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodePong(frame.Frame{Header: HeaderPing, Length: 8, Payload: make([]byte, 4)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodePongRejectsUnexpectedPayload(t *testing.T) {
	_, err := DecodePong(frame.Frame{Header: HeaderPong, Length: 5, Payload: []byte{0x01}})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeStateCheckerBuildsAHeaderOnlyControlFrame(t *testing.T) {
	want := frame.Encode(HeaderStateChecker, nil)
	got := EncodeStateChecker()

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected state checker frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeStateCheckerAcceptsHeaderOnlyControlFrame(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(EncodeStateChecker())
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	if _, err := DecodeStateChecker(frames[0]); err != nil {
		t.Fatalf("unexpected state checker decode error: %v", err)
	}
}

func TestEncodeRespondChannelStatusRoundTripsPackedStatusEntries(t *testing.T) {
	want := []ChannelStatus{
		{Port: 13000, Status: ChannelStatusNormal},
		{Port: 13001, Status: 0},
	}

	raw := EncodeRespondChannelStatus(RespondChannelStatusPacket{Channels: want})

	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodeRespondChannelStatus(frames[0])
	if err != nil {
		t.Fatalf("unexpected respond channel status decode error: %v", err)
	}

	if len(packet.Channels) != len(want) {
		t.Fatalf("unexpected channel count: got %d want %d", len(packet.Channels), len(want))
	}

	for i := range want {
		if packet.Channels[i] != want[i] {
			t.Fatalf("unexpected channel status %d: got %+v want %+v", i, packet.Channels[i], want[i])
		}
	}
}

func TestEncodeKeyChallengeBuildsAControlFrame(t *testing.T) {
	want := loadHexFixture(t, "key-challenge-frame.hex")

	got := EncodeKeyChallenge(KeyChallengePacket{
		ServerPublicKey: sequentialBytes32(0x00),
		Challenge:       sequentialBytes32(0x20),
		ServerTime:      0x01020304,
	})

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected key challenge frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeKeyChallengeReturnsExpectedFields(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "key-challenge-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodeKeyChallenge(frames[0])
	if err != nil {
		t.Fatalf("unexpected key challenge decode error: %v", err)
	}

	if packet.ServerPublicKey != sequentialBytes32(0x00) {
		t.Fatalf("unexpected server public key: got %x want %x", packet.ServerPublicKey, sequentialBytes32(0x00))
	}

	if packet.Challenge != sequentialBytes32(0x20) {
		t.Fatalf("unexpected challenge bytes: got %x want %x", packet.Challenge, sequentialBytes32(0x20))
	}

	if packet.ServerTime != 0x01020304 {
		t.Fatalf("unexpected server time: got %#08x want %#08x", packet.ServerTime, uint32(0x01020304))
	}
}

func TestDecodeKeyChallengeRejectsInvalidPayloadLength(t *testing.T) {
	badFrame := frame.Frame{Header: HeaderKeyChallenge, Length: 71, Payload: make([]byte, 67)}

	_, err := DecodeKeyChallenge(badFrame)
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestEncodeKeyResponseBuildsAControlFrame(t *testing.T) {
	want := loadHexFixture(t, "key-response-frame.hex")

	got := EncodeKeyResponse(KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected key response frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeKeyResponseReturnsExpectedFields(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "key-response-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodeKeyResponse(frames[0])
	if err != nil {
		t.Fatalf("unexpected key response decode error: %v", err)
	}

	if packet.ClientPublicKey != sequentialBytes32(0x40) {
		t.Fatalf("unexpected client public key: got %x want %x", packet.ClientPublicKey, sequentialBytes32(0x40))
	}

	if packet.ChallengeResponse != sequentialBytes32(0x60) {
		t.Fatalf("unexpected challenge response bytes: got %x want %x", packet.ChallengeResponse, sequentialBytes32(0x60))
	}
}

func TestEncodeKeyCompleteBuildsAControlFrame(t *testing.T) {
	want := loadHexFixture(t, "key-complete-frame.hex")

	got := EncodeKeyComplete(KeyCompletePacket{
		EncryptedToken: sequentialBytes48(0x80),
		Nonce:          sequentialBytes24(0xb0),
	})

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected key complete frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeKeyCompleteReturnsExpectedFields(t *testing.T) {
	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(loadHexFixture(t, "key-complete-frame.hex"))
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	packet, err := DecodeKeyComplete(frames[0])
	if err != nil {
		t.Fatalf("unexpected key complete decode error: %v", err)
	}

	if packet.EncryptedToken != sequentialBytes48(0x80) {
		t.Fatalf("unexpected encrypted token bytes: got %x want %x", packet.EncryptedToken, sequentialBytes48(0x80))
	}

	if packet.Nonce != sequentialBytes24(0xb0) {
		t.Fatalf("unexpected nonce bytes: got %x want %x", packet.Nonce, sequentialBytes24(0xb0))
	}
}

func TestEncodeClientVersionRoundTripsFilenameAndTimestamp(t *testing.T) {
	raw, err := EncodeClientVersion(ClientVersionPacket{ExecutableName: "metin2client.bin", Timestamp: "1215955205"})
	if err != nil {
		t.Fatalf("unexpected client version encode error: %v", err)
	}

	decoder := frame.NewDecoder(1024)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}

	packet, err := DecodeClientVersion(frames[0])
	if err != nil {
		t.Fatalf("unexpected client version decode error: %v", err)
	}
	if packet.ExecutableName != "metin2client.bin" {
		t.Fatalf("unexpected executable name: got %q want %q", packet.ExecutableName, "metin2client.bin")
	}
	if packet.Timestamp != "1215955205" {
		t.Fatalf("unexpected timestamp: got %q want %q", packet.Timestamp, "1215955205")
	}
}

func TestDecodeClientVersionRejectsInvalidPayloadLength(t *testing.T) {
	badFrame := frame.Frame{Header: HeaderClientVersion, Length: 69, Payload: make([]byte, 65)}

	_, err := DecodeClientVersion(badFrame)
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func sequentialBytes32(start byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = start + byte(i)
	}

	return out
}

func sequentialBytes48(start byte) [48]byte {
	var out [48]byte
	for i := range out {
		out[i] = start + byte(i)
	}

	return out
}

func sequentialBytes24(start byte) [24]byte {
	var out [24]byte
	for i := range out {
		out[i] = start + byte(i)
	}

	return out
}
