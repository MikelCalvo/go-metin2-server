package login

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
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

func TestEncodeLogin2BuildsAClientFrame(t *testing.T) {
	want := loadHexFixture(t, "login2-frame.hex")

	got, err := EncodeLogin2(Login2Packet{
		Login:    "mkmk",
		LoginKey: 0x01020304,
	})
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected login2 frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeLogin2ReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeLogin2(decodeSingleFrame(t, loadHexFixture(t, "login2-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if packet.Login != "mkmk" {
		t.Fatalf("unexpected login: got %q want %q", packet.Login, "mkmk")
	}

	if packet.LoginKey != 0x01020304 {
		t.Fatalf("unexpected login key: got %#08x want %#08x", packet.LoginKey, uint32(0x01020304))
	}
}

func TestEncodeLoginFailureBuildsAServerFrame(t *testing.T) {
	want := loadHexFixture(t, "login-failure-frame.hex")

	got, err := EncodeLoginFailure(LoginFailurePacket{Status: "FULL"})
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected login failure frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeLoginFailureReturnsExpectedStatus(t *testing.T) {
	packet, err := DecodeLoginFailure(decodeSingleFrame(t, loadHexFixture(t, "login-failure-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if packet.Status != "FULL" {
		t.Fatalf("unexpected status: got %q want %q", packet.Status, "FULL")
	}
}

func TestEncodeEmpireBuildsAServerFrame(t *testing.T) {
	want := loadHexFixture(t, "empire-frame.hex")

	got := EncodeEmpire(EmpirePacket{Empire: 2})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected empire frame bytes: got %x want %x", got, want)
	}
}

func TestEncodeEmpireSelectBuildsAClientFrame(t *testing.T) {
	want := frame.Encode(HeaderEmpireSelect, []byte{3})
	got := EncodeEmpireSelect(EmpireSelectPacket{Empire: 3})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected empire select frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeEmpireSelectReturnsExpectedValue(t *testing.T) {
	packet, err := DecodeEmpireSelect(decodeSingleFrame(t, frame.Encode(HeaderEmpireSelect, []byte{3})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Empire != 3 {
		t.Fatalf("unexpected empire select value: got %d want %d", packet.Empire, 3)
	}
}

func TestDecodeEmpireReturnsExpectedValue(t *testing.T) {
	packet, err := DecodeEmpire(decodeSingleFrame(t, loadHexFixture(t, "empire-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if packet.Empire != 2 {
		t.Fatalf("unexpected empire value: got %d want %d", packet.Empire, 2)
	}
}

func TestEncodeLoginSuccess4BuildsAServerFrame(t *testing.T) {
	want := loadHexFixture(t, "login-success4-frame.hex")

	got, err := EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected login success4 frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeLoginSuccess4ReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeLoginSuccess4(decodeSingleFrame(t, loadHexFixture(t, "login-success4-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if packet.Players[0].ID != 1 {
		t.Fatalf("unexpected player 0 id: got %d want %d", packet.Players[0].ID, 1)
	}

	if packet.Players[0].Name != "Chris" {
		t.Fatalf("unexpected player 0 name: got %q want %q", packet.Players[0].Name, "Chris")
	}

	if packet.Players[1].Name != "Mkmk" {
		t.Fatalf("unexpected player 1 name: got %q want %q", packet.Players[1].Name, "Mkmk")
	}

	if packet.GuildIDs[0] != 10 || packet.GuildIDs[1] != 20 {
		t.Fatalf("unexpected guild ids: got %v", packet.GuildIDs)
	}

	if packet.GuildNames[0] != "Alpha" || packet.GuildNames[1] != "Beta" {
		t.Fatalf("unexpected guild names: got %v", packet.GuildNames)
	}

	if packet.Handle != 0x11223344 {
		t.Fatalf("unexpected handle: got %#08x want %#08x", packet.Handle, uint32(0x11223344))
	}

	if packet.RandomKey != 0x55667788 {
		t.Fatalf("unexpected random key: got %#08x want %#08x", packet.RandomKey, uint32(0x55667788))
	}
}

func TestDecodeLogin2RejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeLogin2(frame.Frame{Header: HeaderEmpire, Length: 5, Payload: []byte{2}})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestEncodeLoginFailureRejectsStatusesThatDoNotFitTheWireField(t *testing.T) {
	_, err := EncodeLoginFailure(LoginFailurePacket{Status: "TOO-LONG-STATUS"})
	if !errors.Is(err, ErrStringTooLong) {
		t.Fatalf("expected ErrStringTooLong, got %v", err)
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

func sampleLoginSuccessPacket() LoginSuccess4Packet {
	packet := LoginSuccess4Packet{
		GuildIDs: [PlayerCount]uint32{10, 20, 0, 0},
		GuildNames: [PlayerCount]string{
			"Alpha",
			"Beta",
			"",
			"",
		},
		Handle:    0x11223344,
		RandomKey: 0x55667788,
	}

	packet.Players[0] = SimplePlayer{
		ID:          1,
		Name:        "Chris",
		Job:         2,
		Level:       30,
		PlayMinutes: 1234,
		ST:          3,
		HT:          4,
		DX:          5,
		IQ:          6,
		MainPart:    100,
		ChangeName:  0,
		HairPart:    200,
		Dummy:       [4]byte{9, 8, 7, 6},
		X:           1000,
		Y:           2000,
		Addr:        0x0100007f,
		Port:        13000,
		SkillGroup:  1,
	}

	packet.Players[1] = SimplePlayer{
		ID:          2,
		Name:        "Mkmk",
		Job:         1,
		Level:       15,
		PlayMinutes: 4321,
		ST:          6,
		HT:          5,
		DX:          4,
		IQ:          3,
		MainPart:    101,
		ChangeName:  1,
		HairPart:    201,
		Dummy:       [4]byte{1, 2, 3, 4},
		X:           3000,
		Y:           4000,
		Addr:        0x0200007f,
		Port:        13001,
		SkillGroup:  2,
	}

	return packet
}
