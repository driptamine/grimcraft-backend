package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"grimcraft-go/internal/hub"
)

// TestRoomContractSocketFree exercises the exact hub calls the WS/WT
// transports make (Join -> SendSession -> HandleCommand -> Ack) without
// opening a socket, and asserts the JSON keys src/multiplayer.js parses.
func TestRoomContractSocketFree(testCase *testing.T) {
	testHub := hub.NewHub()
	lobby := testHub.CreateLobby("contract", "Arena", 4)
	team := "Blue"
	client, _, operationError := testHub.Join(lobby.ID, &team, "bot", "Orks")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	room := testHub.RoomOf(lobby.ID)
	room.SendSession(client)

	readMessage := func() map[string]any {
		select {
		case encodedData := <-client.Out:
			var decoded map[string]any
			if currentOperationError := json.Unmarshal(encodedData, &decoded); currentOperationError != nil {
				testCase.Fatalf("bad json: %v", currentOperationError)
			}
			return decoded
		case <-time.After(2 * time.Second):
			testCase.Fatal("timed out waiting for server message")
			return nil
		}
	}

	sessionMsg := readMessage()
	sessionInfo, valid := sessionMsg["Session"].(map[string]any)
	if !valid {
		testCase.Fatalf("first message must be Session: %v", sessionMsg)
	}
	if sessionInfo["team"] != "Blue" || sessionInfo["player_id"] == nil {
		testCase.Fatalf("bad session: %v", sessionInfo)
	}

	snapshotMsg := readMessage()["Snapshot"]
	if snapshotMsg == nil {
		testCase.Fatal("second message must be Snapshot")
	}
	snapshotBody := snapshotMsg.(map[string]any)
	if snapshotBody["schema"] != float64(2) {
		testCase.Fatalf("snapshot must be schema 2: %v", snapshotBody)
	}
	entities, valid := snapshotBody["entities"].([]any)
	if !valid || len(entities) == 0 {
		testCase.Fatalf("snapshot entities missing: %v", snapshotBody)
	}
	players, valid := snapshotBody["players"].([]any)
	if !valid || len(players) == 0 {
		testCase.Fatalf("snapshot players missing: %v", snapshotBody)
	}
	// Every skirmish kind the replica creates must be present.
	kinds := map[string]bool{}
	for _, entity := range entities {
		entityRecord := entity.(map[string]any)
		for fieldName := range entityRecord["components"].(map[string]any) {
			if fieldName != "networkOwner" {
				kinds[fieldName] = true
			}
		}
	}
	for _, expectedValue := range []string{"squad", "soldier", "builder", "building", "strategicPoint"} {
		if !kinds[expectedValue] {
			testCase.Fatalf("snapshot missing kind %q: %v", expectedValue, kinds)
		}
	}
	playerAccount := players[0].(map[string]any)
	for _, key := range []string{"id", "requisition", "energy", "kills", "research", "team", "faction"} {
		if _, found := playerAccount[key]; !found {
			testCase.Fatalf("snapshot player missing key %q", key)
		}
	}
	// Pick an owned squad id for the move command.
	squadID := 0
	for _, currentEntity := range entities {
		currentEntityRecord := currentEntity.(map[string]any)
		components := currentEntityRecord["components"].(map[string]any)
		if _, isValid := components["squad"]; !isValid {
			continue
		}
		if owner, _ := components["networkOwner"].(map[string]any)["id"].(float64); int(owner) == client.PlayerID {
			squadID = int(currentEntityRecord["id"].(float64))
			break
		}
	}
	if squadID == 0 {
		testCase.Fatal("no owned squad in snapshot")
	}

	var command map[string]any
	if decodeCommandError := json.Unmarshal([]byte(
		fmt.Sprintf(`{"MoveSquads":{"squad_ids":[%d],"target":{"x":5,"y":0,"z":5}}}`, squadID)), &command); decodeCommandError != nil {
		testCase.Fatal(decodeCommandError)
	}
	accepted, reason := hub.HandleCommand(room.World(), client.PlayerID, command)
	hub.Ack(client, accepted, reason)
	var acknowledgement any
	for acknowledgement == nil {
		acknowledgement = readMessage()["CmdAck"]
	}
	if acknowledgement == nil || acknowledgement.(map[string]any)["ok"] != true {
		testCase.Fatalf("expected ok ack: %v", acknowledgement)
	}

	// Delta shape check.
	deltaView := room.World().Delta(true)
	currentEncodedData, _ := json.Marshal(map[string]any{"Delta": deltaView})
	var decodedDelta map[string]any
	_ = json.Unmarshal(currentEncodedData, &decodedDelta)
	delta := decodedDelta["Delta"].(map[string]any)
	if delta["tick"] == nil || delta["squad_pos"] == nil || delta["squad_facing"] == nil {
		testCase.Fatalf("bad delta shape: %v", delta)
	}
}
