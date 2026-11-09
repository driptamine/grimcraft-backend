package transport

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	payloads := [][]byte{[]byte(`{"Join":{}}`), []byte(`{"Command":{"StopSquads":{"squad_ids":[1]}}}`)}
	var stream []byte
	for _, payload := range payloads {
		stream = append(stream, EncodeFrame(payload)...)
	}
	framer := &Framer{}
	// Feed in odd chunks to prove incremental framing.
	var got [][]byte
	for index := 0; index < len(stream); index += 3 {
		end := index + 3
		if end > len(stream) {
			end = len(stream)
		}
		got = append(got, framer.Push(stream[index:end])...)
	}
	if len(got) != len(payloads) {
		t.Fatalf("got %d frames, want %d", len(got), len(payloads))
	}
	for index, payload := range payloads {
		if !bytes.Equal(got[index], payload) {
			t.Fatalf("frame %d mismatch: %q", index, got[index])
		}
	}
}
