package transport

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(testCase *testing.T) {
	payloads := [][]byte{[]byte(`{"Join":{}}`), []byte(`{"Command":{"StopSquads":{"squad_ids":[1]}}}`)}
	var stream []byte
	for _, payload := range payloads {
		stream = append(stream, EncodeFrame(payload)...)
	}
	framer := &Framer{}
	// Feed in odd chunks to prove incremental framing.
	var actualResult [][]byte
	for index := 0; index < len(stream); index += 3 {
		end := index + 3
		if end > len(stream) {
			end = len(stream)
		}
		actualResult = append(actualResult, framer.Push(stream[index:end])...)
	}
	if len(actualResult) != len(payloads) {
		testCase.Fatalf("got %d frames, want %d", len(actualResult), len(payloads))
	}
	for index, payload := range payloads {
		if !bytes.Equal(actualResult[index], payload) {
			testCase.Fatalf("frame %d mismatch: %q", index, actualResult[index])
		}
	}
}
