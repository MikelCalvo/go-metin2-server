package world

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

func TestEncodeCharacterSelectBuildsAClientFrame(t *testing.T) {
	want := loadHexFixture(t, "character-select-frame.hex")
	got := EncodeCharacterSelect(CharacterSelectPacket{Index: 1})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected character select frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeCharacterSelectReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeCharacterSelect(decodeSingleFrame(t, loadHexFixture(t, "character-select-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Index != 1 {
		t.Fatalf("unexpected index: got %d want %d", packet.Index, 1)
	}
}

func TestEncodeEnterGameBuildsAHeaderOnlyClientFrame(t *testing.T) {
	want := loadHexFixture(t, "entergame-frame.hex")
	got := EncodeEnterGame()
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected entergame frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeEnterGameAcceptsAHeaderOnlyClientFrame(t *testing.T) {
	if err := DecodeEnterGame(decodeSingleFrame(t, loadHexFixture(t, "entergame-frame.hex"))); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
}

func TestEncodeMainCharacterBuildsAServerFrame(t *testing.T) {
	want := loadHexFixture(t, "main-character-frame.hex")
	got, err := EncodeMainCharacter(MainCharacterPacket{
		VID:        0x01020304,
		RaceNum:    2,
		Name:       "Mkmk",
		BGMName:    "",
		BGMVolume:  0,
		X:          1000,
		Y:          2000,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	})
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected main character frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeMainCharacterReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeMainCharacter(decodeSingleFrame(t, loadHexFixture(t, "main-character-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.VID != 0x01020304 || packet.RaceNum != 2 {
		t.Fatalf("unexpected main character ids: got vid=%#08x race=%d", packet.VID, packet.RaceNum)
	}
	if packet.Name != "Mkmk" {
		t.Fatalf("unexpected name: got %q", packet.Name)
	}
	if packet.X != 1000 || packet.Y != 2000 || packet.Z != 0 {
		t.Fatalf("unexpected position: got %d,%d,%d", packet.X, packet.Y, packet.Z)
	}
	if packet.Empire != 2 || packet.SkillGroup != 1 {
		t.Fatalf("unexpected empire/group: got %d/%d", packet.Empire, packet.SkillGroup)
	}
}

func TestEncodePlayerPointsBuildsAServerFrame(t *testing.T) {
	want := loadHexFixture(t, "player-points-frame.hex")
	points := samplePoints()
	got := EncodePlayerPoints(PlayerPointsPacket{Points: points})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected player points frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePlayerPointsReturnsExpectedFields(t *testing.T) {
	packet, err := DecodePlayerPoints(decodeSingleFrame(t, loadHexFixture(t, "player-points-frame.hex")))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Points[0] != 15 || packet.Points[1] != 1234 || packet.Points[2] != 5678 {
		t.Fatalf("unexpected leading points: got %d %d %d", packet.Points[0], packet.Points[1], packet.Points[2])
	}
	if packet.Points[8] != 50 {
		t.Fatalf("unexpected point[8]: got %d want %d", packet.Points[8], 50)
	}
}

func TestDecodeCharacterSelectRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeCharacterSelect(frame.Frame{Header: HeaderMainCharacter, Length: 118, Payload: make([]byte, mainCharacterPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
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

func samplePoints() [PointCount]int32 {
	var points [PointCount]int32
	points[0] = 15
	points[1] = 1234
	points[2] = 5678
	points[3] = 900
	points[4] = 1000
	points[5] = 200
	points[6] = 300
	points[7] = 999999
	points[8] = 50
	return points
}
