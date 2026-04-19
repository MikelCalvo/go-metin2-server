package frame

import (
	"encoding/binary"
	"errors"
)

const EnvelopeSize = 4

var (
	ErrInvalidLength = errors.New("frame length smaller than envelope size")
	ErrFrameTooLarge = errors.New("frame length exceeds decoder maximum")
)

type Frame struct {
	Header  uint16
	Length  uint16
	Payload []byte
}

type Decoder struct {
	buffer       []byte
	maxFrameSize uint16
}

func NewDecoder(maxFrameSize uint16) *Decoder {
	return &Decoder{maxFrameSize: maxFrameSize}
}

func (d *Decoder) BufferedLen() int {
	if d == nil {
		return 0
	}
	return len(d.buffer)
}

func Encode(header uint16, payload []byte) []byte {
	frame := make([]byte, EnvelopeSize+len(payload))

	binary.LittleEndian.PutUint16(frame[0:2], header)
	binary.LittleEndian.PutUint16(frame[2:4], uint16(len(frame)))
	copy(frame[EnvelopeSize:], payload)

	return frame
}

func (d *Decoder) Feed(chunk []byte) ([]Frame, error) {
	if len(chunk) > 0 {
		d.buffer = append(d.buffer, chunk...)
	}

	frames := make([]Frame, 0)

	for {
		if len(d.buffer) < EnvelopeSize {
			return frames, nil
		}

		length := binary.LittleEndian.Uint16(d.buffer[2:4])
		if length < EnvelopeSize {
			d.buffer = nil
			return nil, ErrInvalidLength
		}

		if d.maxFrameSize > 0 && length > d.maxFrameSize {
			d.buffer = nil
			return nil, ErrFrameTooLarge
		}

		frameLen := int(length)
		if len(d.buffer) < frameLen {
			return frames, nil
		}

		payload := make([]byte, frameLen-EnvelopeSize)
		copy(payload, d.buffer[EnvelopeSize:frameLen])

		frames = append(frames, Frame{
			Header:  binary.LittleEndian.Uint16(d.buffer[0:2]),
			Length:  length,
			Payload: payload,
		})

		d.buffer = d.buffer[frameLen:]
	}
}
