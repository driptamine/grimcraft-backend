// Package transport — framing: WebTransport streams carry 4-byte
// little-endian length-prefixed JSON frames (see _makeFrame in
// multiplayer.js). WebSocket messages carry the same JSON unframed.
package transport

import "encoding/binary"

// EncodeFrame prefixes payload with its LE uint32 length.
func EncodeFrame(payload []byte) []byte {
	frame := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)
	return frame
}

// Framer incrementally splits a byte stream into frames.
type Framer struct {
	buffer []byte
}

// Push appends bytes and returns every complete frame payload.
func (framer *Framer) Push(chunk []byte) [][]byte {
	framer.buffer = append(framer.buffer, chunk...)
	var result [][]byte
	for len(framer.buffer) >= 4 {
		frameLength := int(binary.LittleEndian.Uint32(framer.buffer[:4]))
		if len(framer.buffer) < 4+frameLength {
			break
		}
		payload := make([]byte, frameLength)
		copy(payload, framer.buffer[4:4+frameLength])
		result = append(result, payload)
		framer.buffer = append([]byte(nil), framer.buffer[4+frameLength:]...)
	}
	return result
}
