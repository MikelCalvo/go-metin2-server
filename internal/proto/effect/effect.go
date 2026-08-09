package effect

import (
	"encoding/binary"
	"errors"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
)

const (
	HeaderSpecialEffect  uint16 = 0x0A30
	HeaderSpecificEffect uint16 = 0x0A31

	SpecialEffectNone                  uint8 = 0
	SpecialEffectHPUpRed               uint8 = 1
	SpecialEffectSPUpBlue              uint8 = 2
	SpecialEffectSpeedUpGreen          uint8 = 3
	SpecialEffectDXUpPurple            uint8 = 4
	SpecialEffectCritical              uint8 = 5
	SpecialEffectPenetrate             uint8 = 6
	SpecialEffectBlock                 uint8 = 7
	SpecialEffectDodge                 uint8 = 8
	SpecialEffectChinaFirework         uint8 = 9
	SpecialEffectSpinTop               uint8 = 10
	SpecialEffectSuccess               uint8 = 11
	SpecialEffectFail                  uint8 = 12
	SpecialEffectFRSuccess             uint8 = 13
	SpecialEffectLevelUpOn14Germany    uint8 = 14
	SpecialEffectLevelUpUnder15Germany uint8 = 15
	SpecialEffectPercentDamage1        uint8 = 16
	SpecialEffectPercentDamage2        uint8 = 17
	SpecialEffectPercentDamage3        uint8 = 18
	SpecialEffectAutoHPUp              uint8 = 19
	SpecialEffectAutoSPUp              uint8 = 20
	SpecialEffectEquipRamadanRing      uint8 = 21
	SpecialEffectEquipHalloweenCandy   uint8 = 22
	SpecialEffectEquipHappinessRing    uint8 = 23
	SpecialEffectEquipLovePendant      uint8 = 24
	SpecialEffectAggregateMonster      uint8 = 25

	specialPayloadSize = 1 + 4
)

var (
	ErrUnexpectedHeader = errors.New("unexpected effect packet header")
	ErrInvalidPayload   = errors.New("invalid effect packet payload")
)

type SpecialPacket struct {
	Type uint8
	VID  uint32
}

func EncodeSpecial(packet SpecialPacket) []byte {
	payload := make([]byte, specialPayloadSize)
	payload[0] = packet.Type
	binary.LittleEndian.PutUint32(payload[1:5], packet.VID)
	return frame.Encode(HeaderSpecialEffect, payload)
}

func DecodeSpecial(f frame.Frame) (SpecialPacket, error) {
	if f.Header != HeaderSpecialEffect {
		return SpecialPacket{}, ErrUnexpectedHeader
	}
	if len(f.Payload) != specialPayloadSize {
		return SpecialPacket{}, ErrInvalidPayload
	}
	return SpecialPacket{Type: f.Payload[0], VID: binary.LittleEndian.Uint32(f.Payload[1:5])}, nil
}
