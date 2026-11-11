package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"grimcraft-go/internal/hub"
	"grimcraft-go/internal/transport"
)

// TestWSJoinSessionSnapshot is the end-to-end contract check: connect,
// receive Session + Snapshot, send a command, receive CmdAck.
func TestWSJoinSessionSnapshot(testCase *testing.T) {
	testHub := hub.NewHub()
	lobbies := testHub.List()
	lobbyID := lobbies[0].ID

	router := http.NewServeMux()
	router.Handle("/api/", API(testHub))
	router.HandleFunc("/join", transport.WSHandler(testHub))
	if probe, operationError := net.Listen("tcp", "127.0.0.1:0"); operationError != nil {
		testCase.Skipf("sandbox blocks TCP listen, skipping live-socket check: %v", operationError)
	} else {
		_ = probe.Close()
	}
	testServer := httptest.NewServer(router)
	defer testServer.Close()

	wsURL := "ws" + testServer.URL[len("http"):] + "/join?lobby=" + lobbyID + "&username=bot&faction=Orks"
	requestContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, _, currentOperationError := websocket.Dial(requestContext, wsURL, nil)
	if currentOperationError != nil {
		testCase.Fatal(currentOperationError)
	}
	defer connection.Close(websocket.StatusNormalClosure, "test done")

	readMessage := func() map[string]any {
		_, encodedData, readError := connection.Read(requestContext)
		if readError != nil {
			testCase.Fatalf("read: %v", readError)
		}
		var envelope map[string]any
		if decodeMessageError := json.Unmarshal(encodedData, &envelope); decodeMessageError != nil {
			testCase.Fatalf("bad json %q: %v", encodedData, decodeMessageError)
		}
		return envelope
	}

	sessionMsg := readMessage()
	sessionInfo, valid := sessionMsg["Session"].(map[string]any)
	if !valid {
		testCase.Fatalf("first message must be Session, got %v", sessionMsg)
	}
	if sessionInfo["player_id"] == nil || (sessionInfo["team"] != "Blue" && sessionInfo["team"] != "Red") {
		testCase.Fatalf("bad session: %v", sessionInfo)
	}
	snapshotMsg := readMessage()
	if snapshotMsg["Snapshot"] == nil {
		testCase.Fatalf("second message must be Snapshot, got %v", snapshotMsg)
	}

	// Send a MoveSquads for our own squad id.
	entities := snapshotMsg["Snapshot"].(map[string]any)["entities"].([]any)
	if len(entities) == 0 {
		testCase.Fatal("snapshot has no entities")
	}
	ownSquadID := 0.0
	for _, entity := range entities {
		entityRecord := entity.(map[string]any)
		components := entityRecord["components"].(map[string]any)
		if _, isValid := components["squad"]; isValid {
			ownSquadID = entityRecord["id"].(float64)
			break
		}
	}
	commandDoc := map[string]any{"Command": map[string]any{"MoveSquads": map[string]any{
		"squad_ids": []any{ownSquadID}, "target": map[string]any{"x": 5, "y": 0, "z": 5},
	}}}
	commandJSON, _ := json.Marshal(commandDoc)
	if writeCommandError := connection.Write(requestContext, websocket.MessageText, commandJSON); writeCommandError != nil {
		testCase.Fatal(writeCommandError)
	}
	// Next messages may be Delta snapshots; wait for the CmdAck.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			testCase.Fatal("no CmdAck received")
		}
		envelope := readMessage()
		if acknowledgement, valid := envelope["CmdAck"].(map[string]any); valid {
			if acknowledgement["ok"] != false || acknowledgement["reason"] != "waiting for all players to finish loading" {
				testCase.Fatalf("gameplay must be rejected before all clients load: %v", acknowledgement)
			}
			break
		}
	}
}
