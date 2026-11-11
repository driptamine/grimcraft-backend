package transport

import (
	"encoding/json"
	"grimcraft-go/internal/protocol"
	"testing"
)

func TestFramedWebTransportJoinMetadata(testCase *testing.T) {
	message := protocol.ClientMessage{Join: &protocol.JoinRequest{Username: "Хост", Faction: "Space Marines"}}
	encodedData, _ := json.Marshal(message)
	var framer Framer
	var frames [][]byte
	for _, payloadByte := range EncodeFrame(encodedData) {
		frames = append(frames, framer.Push([]byte{payloadByte})...)
	}
	if len(frames) != 1 {
		testCase.Fatal("framing lost Join")
	}
	var actualResult protocol.ClientMessage
	if operationError := json.Unmarshal(frames[0], &actualResult); operationError != nil {
		testCase.Fatal(operationError)
	}
	if actualResult.Join == nil || actualResult.Join.Username != "Хост" || actualResult.Join.Faction != "Space Marines" {
		testCase.Fatalf("lost metadata: %+v", actualResult.Join)
	}
}
