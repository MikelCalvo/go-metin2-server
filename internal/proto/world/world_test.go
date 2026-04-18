package world

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
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

func TestEncodeCharacterCreateBuildsAClientFrame(t *testing.T) {
	packet := CharacterCreatePacket{
		Index:   2,
		Name:    "FreshSura",
		RaceNum: 2,
		Shape:   1,
		Con:     0,
		Int:     0,
		Str:     0,
		Dex:     0,
	}
	want := expectedCharacterCreateFrame(packet)
	got, err := EncodeCharacterCreate(packet)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected character create frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeCharacterCreateReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeCharacterCreate(decodeSingleFrame(t, expectedCharacterCreateFrame(CharacterCreatePacket{
		Index:   2,
		Name:    "FreshSura",
		RaceNum: 2,
		Shape:   1,
		Con:     0,
		Int:     0,
		Str:     0,
		Dex:     0,
	})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Index != 2 || packet.Name != "FreshSura" || packet.RaceNum != 2 || packet.Shape != 1 {
		t.Fatalf("unexpected create packet: %+v", packet)
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

func TestEncodeCharacterAddBuildsAServerFrame(t *testing.T) {
	packet := sampleCharacterAddPacket()
	want := expectedCharacterAddFrame(packet)
	got := EncodeCharacterAdd(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected character add frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeCharacterAddReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeCharacterAdd(decodeSingleFrame(t, expectedCharacterAddFrame(sampleCharacterAddPacket())))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleCharacterAddPacket() {
		t.Fatalf("unexpected character add packet: %+v", packet)
	}
}

func TestEncodeCharacterAdditionalInfoBuildsAServerFrame(t *testing.T) {
	packet := sampleCharacterAdditionalInfoPacket()
	want := expectedCharacterAdditionalInfoFrame(packet)
	got, err := EncodeCharacterAdditionalInfo(packet)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected character additional info frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeCharacterAdditionalInfoReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeCharacterAdditionalInfo(decodeSingleFrame(t, expectedCharacterAdditionalInfoFrame(sampleCharacterAdditionalInfoPacket())))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleCharacterAdditionalInfoPacket() {
		t.Fatalf("unexpected character additional info packet: %+v", packet)
	}
}

func TestEncodeCharacterUpdateBuildsAServerFrame(t *testing.T) {
	packet := sampleCharacterUpdatePacket()
	want := expectedCharacterUpdateFrame(packet)
	got := EncodeCharacterUpdate(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected character update frame bytes: got %x want %x", got, want)
	}
}

func TestDecodeCharacterUpdateReturnsExpectedFields(t *testing.T) {
	packet, err := DecodeCharacterUpdate(decodeSingleFrame(t, expectedCharacterUpdateFrame(sampleCharacterUpdatePacket())))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != sampleCharacterUpdatePacket() {
		t.Fatalf("unexpected character update packet: %+v", packet)
	}
}

func TestEncodePlayerCreateSuccessBuildsAServerFrame(t *testing.T) {
	packet := PlayerCreateSuccessPacket{Index: 2, Player: sampleCreatedPlayer()}
	want := expectedPlayerCreateSuccessFrame(packet)
	got, err := EncodePlayerCreateSuccess(packet)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected player create success frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePlayerCreateSuccessReturnsExpectedFields(t *testing.T) {
	packet, err := DecodePlayerCreateSuccess(decodeSingleFrame(t, expectedPlayerCreateSuccessFrame(PlayerCreateSuccessPacket{
		Index:  2,
		Player: sampleCreatedPlayer(),
	})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Index != 2 {
		t.Fatalf("unexpected slot index: got %d want %d", packet.Index, 2)
	}
	if packet.Player.Name != "FreshSura" || packet.Player.Job != 2 || packet.Player.Level != 1 {
		t.Fatalf("unexpected created player summary: %+v", packet.Player)
	}
}

func TestEncodePlayerCreateFailureBuildsAServerFrame(t *testing.T) {
	want := frame.Encode(HeaderPlayerCreateFailure, []byte{1})
	got := EncodePlayerCreateFailure(PlayerCreateFailurePacket{Type: 1})
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected player create failure frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePlayerCreateFailureReturnsExpectedType(t *testing.T) {
	packet, err := DecodePlayerCreateFailure(decodeSingleFrame(t, frame.Encode(HeaderPlayerCreateFailure, []byte{1})))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet.Type != 1 {
		t.Fatalf("unexpected failure type: got %d want %d", packet.Type, 1)
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

func TestEncodePlayerPointChangeBuildsAServerFrame(t *testing.T) {
	packet := samplePlayerPointChangePacket()
	want := expectedPlayerPointChangeFrame(packet)
	got := EncodePlayerPointChange(packet)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected player point change frame bytes: got %x want %x", got, want)
	}
}

func TestDecodePlayerPointChangeReturnsExpectedFields(t *testing.T) {
	packet, err := DecodePlayerPointChange(decodeSingleFrame(t, expectedPlayerPointChangeFrame(samplePlayerPointChangePacket())))
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if packet != samplePlayerPointChangePacket() {
		t.Fatalf("unexpected player point change packet: %+v", packet)
	}
}

func TestDecodeCharacterSelectRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeCharacterSelect(frame.Frame{Header: HeaderMainCharacter, Length: 118, Payload: make([]byte, mainCharacterPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeCharacterAddRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeCharacterAdd(frame.Frame{Header: HeaderCharacterAdditionalInfo, Length: 97, Payload: make([]byte, characterAdditionalInfoPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeCharacterAdditionalInfoRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeCharacterAdditionalInfo(frame.Frame{Header: HeaderCharacterAdditionalInfo, Length: 96, Payload: make([]byte, characterAdditionalInfoPayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodeCharacterUpdateRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodeCharacterUpdate(frame.Frame{Header: HeaderCharacterAdd, Length: 38, Payload: make([]byte, characterAddPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodeCharacterUpdateRejectsInvalidPayload(t *testing.T) {
	_, err := DecodeCharacterUpdate(frame.Frame{Header: HeaderCharacterUpdate, Length: 37, Payload: make([]byte, characterUpdatePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestDecodePlayerPointChangeRejectsUnexpectedHeader(t *testing.T) {
	_, err := DecodePlayerPointChange(frame.Frame{Header: HeaderPlayerPoints, Length: 1024, Payload: make([]byte, playerPointsPayloadSize)})
	if !errors.Is(err, ErrUnexpectedHeader) {
		t.Fatalf("expected ErrUnexpectedHeader, got %v", err)
	}
}

func TestDecodePlayerPointChangeRejectsInvalidPayload(t *testing.T) {
	_, err := DecodePlayerPointChange(frame.Frame{Header: HeaderPlayerPointChange, Length: 16, Payload: make([]byte, playerPointChangePayloadSize-1)})
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
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

func expectedCharacterCreateFrame(packet CharacterCreatePacket) []byte {
	payload := make([]byte, 1+CharacterNameFieldSize+2+1+4)
	payload[0] = packet.Index
	copy(payload[1:1+CharacterNameFieldSize], packet.Name)
	binary.LittleEndian.PutUint16(payload[1+CharacterNameFieldSize:], packet.RaceNum)
	offset := 1 + CharacterNameFieldSize + 2
	payload[offset] = packet.Shape
	offset++
	payload[offset] = packet.Con
	offset++
	payload[offset] = packet.Int
	offset++
	payload[offset] = packet.Str
	offset++
	payload[offset] = packet.Dex
	return frame.Encode(HeaderCharacterCreate, payload)
}

func expectedCharacterAddFrame(packet CharacterAddPacket) []byte {
	payload := make([]byte, characterAddPayloadSize)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], packet.VID)
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], math.Float32bits(packet.Angle))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.X))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.Y))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.Z))
	offset += 4
	payload[offset] = packet.Type
	offset++
	binary.LittleEndian.PutUint16(payload[offset:], packet.RaceNum)
	offset += 2
	payload[offset] = packet.MovingSpeed
	offset++
	payload[offset] = packet.AttackSpeed
	offset++
	payload[offset] = packet.StateFlag
	offset++
	for _, affect := range packet.AffectFlags {
		binary.LittleEndian.PutUint32(payload[offset:], affect)
		offset += 4
	}
	return frame.Encode(HeaderCharacterAdd, payload)
}

func expectedCharacterAdditionalInfoFrame(packet CharacterAdditionalInfoPacket) []byte {
	payload := make([]byte, characterAdditionalInfoPayloadSize)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], packet.VID)
	offset += 4
	copy(payload[offset:offset+CharacterNameFieldSize], packet.Name)
	offset += CharacterNameFieldSize
	for _, part := range packet.Parts {
		binary.LittleEndian.PutUint16(payload[offset:], part)
		offset += 2
	}
	payload[offset] = packet.Empire
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], packet.GuildID)
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], packet.Level)
	offset += 4
	binary.LittleEndian.PutUint16(payload[offset:], uint16(packet.Alignment))
	offset += 2
	payload[offset] = packet.PKMode
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], packet.MountVnum)
	return frame.Encode(HeaderCharacterAdditionalInfo, payload)
}

func expectedCharacterUpdateFrame(packet CharacterUpdatePacket) []byte {
	payload := make([]byte, characterUpdatePayloadSize)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], packet.VID)
	offset += 4
	for _, part := range packet.Parts {
		binary.LittleEndian.PutUint16(payload[offset:], part)
		offset += 2
	}
	payload[offset] = packet.MovingSpeed
	offset++
	payload[offset] = packet.AttackSpeed
	offset++
	payload[offset] = packet.StateFlag
	offset++
	for _, affect := range packet.AffectFlags {
		binary.LittleEndian.PutUint32(payload[offset:], affect)
		offset += 4
	}
	binary.LittleEndian.PutUint32(payload[offset:], packet.GuildID)
	offset += 4
	binary.LittleEndian.PutUint16(payload[offset:], uint16(packet.Alignment))
	offset += 2
	payload[offset] = packet.PKMode
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], packet.MountVnum)
	return frame.Encode(HeaderCharacterUpdate, payload)
}

func expectedPlayerPointChangeFrame(packet PlayerPointChangePacket) []byte {
	payload := make([]byte, playerPointChangePayloadSize)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], packet.VID)
	offset += 4
	payload[offset] = packet.Type
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.Amount))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(packet.Value))
	return frame.Encode(HeaderPlayerPointChange, payload)
}

func expectedPlayerCreateSuccessFrame(packet PlayerCreateSuccessPacket) []byte {
	payload := make([]byte, 1+103)
	payload[0] = packet.Index
	copy(payload[1:], expectedSimplePlayerPayload(packet.Player))
	return frame.Encode(HeaderPlayerCreateSuccess, payload)
}

func expectedSimplePlayerPayload(player loginproto.SimplePlayer) []byte {
	payload := make([]byte, 103)
	offset := 0
	binary.LittleEndian.PutUint32(payload[offset:], player.ID)
	offset += 4
	copy(payload[offset:offset+loginproto.CharacterNameFieldSize], player.Name)
	offset += loginproto.CharacterNameFieldSize
	payload[offset] = player.Job
	offset++
	payload[offset] = player.Level
	offset++
	binary.LittleEndian.PutUint32(payload[offset:], player.PlayMinutes)
	offset += 4
	payload[offset] = player.ST
	offset++
	payload[offset] = player.HT
	offset++
	payload[offset] = player.DX
	offset++
	payload[offset] = player.IQ
	offset++
	binary.LittleEndian.PutUint16(payload[offset:], player.MainPart)
	offset += 2
	payload[offset] = player.ChangeName
	offset++
	binary.LittleEndian.PutUint16(payload[offset:], player.HairPart)
	offset += 2
	copy(payload[offset:offset+4], player.Dummy[:])
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(player.X))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], uint32(player.Y))
	offset += 4
	binary.LittleEndian.PutUint32(payload[offset:], player.Addr)
	offset += 4
	binary.LittleEndian.PutUint16(payload[offset:], player.Port)
	offset += 2
	payload[offset] = player.SkillGroup
	return payload
}

func sampleCreatedPlayer() loginproto.SimplePlayer {
	return loginproto.SimplePlayer{
		ID:          3,
		Name:        "FreshSura",
		Job:         2,
		Level:       1,
		PlayMinutes: 0,
		ST:          5,
		HT:          3,
		DX:          3,
		IQ:          5,
		MainPart:    1,
		ChangeName:  0,
		HairPart:    0,
		Dummy:       [4]byte{},
		X:           1500,
		Y:           2500,
		Addr:        0x0100007f,
		Port:        13000,
		SkillGroup:  0,
	}
}

func sampleCharacterAddPacket() CharacterAddPacket {
	return CharacterAddPacket{
		VID:         0x01020304,
		Angle:       90.5,
		X:           1000,
		Y:           2000,
		Z:           0,
		Type:        6,
		RaceNum:     2,
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   2,
		AffectFlags: [AffectFlagCount]uint32{0x11111111, 0x22222222},
	}
}

func sampleCharacterAdditionalInfoPacket() CharacterAdditionalInfoPacket {
	return CharacterAdditionalInfoPacket{
		VID:       0x01020304,
		Name:      "Mkmk",
		Parts:     [CharacterEquipmentPartCount]uint16{101, 0, 0, 201},
		Empire:    2,
		GuildID:   10,
		Level:     15,
		Alignment: 0,
		PKMode:    0,
		MountVnum: 0,
	}
}

func sampleCharacterUpdatePacket() CharacterUpdatePacket {
	return CharacterUpdatePacket{
		VID:         0x01020304,
		Parts:       [CharacterEquipmentPartCount]uint16{101, 0, 0, 201},
		MovingSpeed: 150,
		AttackSpeed: 100,
		StateFlag:   2,
		AffectFlags: [AffectFlagCount]uint32{0x11111111, 0x22222222},
		GuildID:     10,
		Alignment:   0,
		PKMode:      0,
		MountVnum:   0,
	}
}

func samplePlayerPointChangePacket() PlayerPointChangePacket {
	return PlayerPointChangePacket{VID: 0x01020304, Type: 1, Amount: 1234, Value: 1234}
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
