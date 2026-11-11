package hub

import (
	"encoding/json"
	"strconv"
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestJoinAssignsOpposingTeams(testCase *testing.T) {
	testHub := NewHub()
	lobby := testHub.CreateLobby("test", "Arena", 4)
	blue := "Blue"
	firstClient, _, operationError := testHub.Join(lobby.ID, &blue, "p1", "Orks")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	secondClient, _, operationError := testHub.Join(lobby.ID, nil, "p2", "Eldar")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	if firstClient.Team != "Blue" {
		testCase.Fatalf("preferred team ignored: %q", firstClient.Team)
	}
	if secondClient.Team != "Red" {
		testCase.Fatalf("auto-balance failed: %q", secondClient.Team)
	}
	if len(testHub.RoomOf(lobby.ID).World().Squads) != 4 {
		testCase.Fatal("each join should spawn a full skirmish seat")
	}
}

func TestLobbyFull(testCase *testing.T) {
	testHub := NewHub()
	lobby := testHub.CreateLobby("tiny", "Arena", 1)
	if _, _, operationError := testHub.Join(lobby.ID, nil, "p1", ""); operationError != nil {
		testCase.Fatal(operationError)
	}
	if _, _, currentOperationError := testHub.Join(lobby.ID, nil, "p2", ""); currentOperationError == nil {
		testCase.Fatal("expected lobby-full error")
	}
}

func TestHandleCommandMove(testCase *testing.T) {
	testHub := NewHub()
	lobby := testHub.CreateLobby("test", "Arena", 4)
	client, _, _ := testHub.Join(lobby.ID, nil, "p1", "")
	world := testHub.RoomOf(lobby.ID).World()
	var squadID int
	for entityID, squad := range world.Squads {
		if squad.Owner == client.PlayerID {
			squadID = entityID
		}
	}
	commandJSON := `{"MoveSquads":{"squad_ids":[` + strconv.Itoa(squadID) + `],"target":{"x":10,"y":0,"z":5}}}`
	var command map[string]any
	if operationError := json.Unmarshal([]byte(commandJSON), &command); operationError != nil {
		testCase.Fatal(operationError)
	}
	accepted, reason := HandleCommand(world, client.PlayerID, command)
	if !accepted {
		testCase.Fatalf("move rejected: %s", reason)
	}
	squad := world.Squads[squadID]
	if squad.MoveOrder == nil || squad.MoveOrder.X != 10 || squad.MoveOrder.Z != 5 {
		testCase.Fatalf("move order not applied: %+v", squad.MoveOrder)
	}
}

func TestClientMessageDecode(testCase *testing.T) {
	decoded, operationError := DecodeClientMessage([]byte(`{"Command":{"StopSquads":{"squad_ids":[3]}}}`))
	if operationError != nil || decoded.Command == nil {
		testCase.Fatalf("decode failed: %v %+v", operationError, decoded)
	}
	var _ protocol.SessionInfo
}
