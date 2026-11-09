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
	buf []byte
}

// Push appends bytes and returns every complete frame payload.
func (framer *Framer) Push(chunk []byte) [][]byte {
	framer.buf = append(framer.buf, chunk...)
	var out [][]byte
	for len(framer.buf) >= 4 {
		frameLen := int(binary.LittleEndian.Uint32(framer.buf[:4]))
		if len(framer.buf) < 4+frameLen {
			break
		}
		payload := make([]byte, frameLen)
		copy(payload, framer.buf[4:4+frameLen])
		out = append(out, payload)
		framer.buf = append([]byte(nil), framer.buf[4+frameLen:]...)
	}
	return out
}
