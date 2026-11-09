package hub

import (
	"encoding/json"
	"strconv"
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestJoinAssignsOpposingTeams(t *testing.T) {
	testHub := NewHub()
	lobby := testHub.CreateLobby("test", "Arena", 4)
	blue := "Blue"
	firstClient, _, err := testHub.Join(lobby.ID, &blue, "p1", "Orks")
	if err != nil {
		t.Fatal(err)
	}
	secondClient, _, err := testHub.Join(lobby.ID, nil, "p2", "Eldar")
	if err != nil {
		t.Fatal(err)
	}
	if firstClient.Team != "Blue" {
		t.Fatalf("preferred team ignored: %q", firstClient.Team)
	}
	if secondClient.Team != "Red" {
		t.Fatalf("auto-balance failed: %q", secondClient.Team)
	}
	if len(testHub.RoomOf(lobby.ID).World().Squads) != 2 {
		t.Fatal("each join should spawn one squad")
	}
}

func TestLobbyFull(t *testing.T) {
	testHub := NewHub()
	lobby := testHub.CreateLobby("tiny", "Arena", 1)
	if _, _, err := testHub.Join(lobby.ID, nil, "p1", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := testHub.Join(lobby.ID, nil, "p2", ""); err == nil {
		t.Fatal("expected lobby-full error")
	}
}

func TestHandleCommandMove(t *testing.T) {
	testHub := NewHub()
	lobby := testHub.CreateLobby("test", "Arena", 4)
	client, _, _ := testHub.Join(lobby.ID, nil, "p1", "")
	world := testHub.RoomOf(lobby.ID).World()
	var squadID int
	for id, squad := range world.Squads {
		if squad.Owner == client.PlayerID {
			squadID = id
		}
	}
	raw := `{"MoveSquads":{"squad_ids":[` + strconv.Itoa(squadID) + `],"target":{"x":10,"y":0,"z":5}}}`
	var cmd map[string]any
	if err := json.Unmarshal([]byte(raw), &cmd); err != nil {
		t.Fatal(err)
	}
	accepted, reason := HandleCommand(world, client.PlayerID, cmd)
	if !accepted {
		t.Fatalf("move rejected: %s", reason)
	}
	squad := world.Squads[squadID]
	if squad.MoveOrder == nil || squad.MoveOrder.X != 10 || squad.MoveOrder.Z != 5 {
		t.Fatalf("move order not applied: %+v", squad.MoveOrder)
	}
}

func TestClientMessageDecode(t *testing.T) {
	decoded, err := DecodeClientMessage([]byte(`{"Command":{"StopSquads":{"squad_ids":[3]}}}`))
	if err != nil || decoded.Command == nil {
		t.Fatalf("decode failed: %v %+v", err, decoded)
	}
	var _ protocol.SessionInfo
}
