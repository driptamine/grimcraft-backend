package hub

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFullSkirmishSnapshotEveryTick(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("snapshot test", "Alvarus", 2)
	// Let the empty room tick before joining: its map must already be correct.
	time.Sleep(150 * time.Millisecond)
	client, _, operationError := gameHub.Join(lobby.ID, nil, "host", "ig")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	defer gameHub.Leave(lobby.ID, client)
	room := gameHub.RoomOf(lobby.ID)
	room.SendSession(client)
	deadline := time.After(2 * time.Second)
	count := 0
	for count < 4 {
		select {
		case encodedData := <-client.Out:
			var frame struct {
				Snapshot *struct {
					Schema   int `json:"schema"`
					Entities []struct {
						Components map[string]json.RawMessage `json:"components"`
					} `json:"entities"`
				} `json:"Snapshot"`
				Delta json.RawMessage `json:"Delta"`
			}
			if currentOperationError := json.Unmarshal(encodedData, &frame); currentOperationError != nil {
				testCase.Fatal(currentOperationError)
			}
			if len(frame.Delta) > 0 {
				testCase.Fatal("legacy delta in schema-2 stream")
			}
			if frame.Snapshot == nil {
				continue
			}
			if frame.Snapshot.Schema != 2 {
				testCase.Fatal("legacy snapshot")
			}
			counts := map[string]int{}
			for _, entity := range frame.Snapshot.Entities {
				for kind := range entity.Components {
					counts[kind]++
				}
			}
			if counts["building"] != 1 || counts["builder"] != 1 || counts["strategicPoint"] != 24 {
				testCase.Fatalf("missing entities: %v", counts)
			}
			count++
		case <-deadline:
			testCase.Fatal("snapshot stream stopped")
		}
	}
}
