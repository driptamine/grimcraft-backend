package hub

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMatchWaitsForEveryLoadedPlayer(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("loading", "Alvarus", 2)
	hostClient, _, _ := gameHub.Join(lobby.ID, nil, "host", "ig")
	defer gameHub.Leave(lobby.ID, hostClient)
	room := gameHub.RoomOf(lobby.ID)
	time.Sleep(200 * time.Millisecond)
	if snapshot := room.World().SnapshotV2(); snapshot.Tick != 0 || snapshot.Winner != nil {
		testCase.Fatal("lobby advanced or won before start")
	}
	guestClient, _, _ := gameHub.Join(lobby.ID, nil, "slow", "ig")
	defer gameHub.Leave(lobby.ID, guestClient)
	command := func(client *Client, currentCommand map[string]any) {
		testCase.Helper()
		handled, acceptedDuringLoading, reason := gameHub.HandleLobbyCommand(lobby.ID, client, currentCommand)
		if !handled || !acceptedDuringLoading {
			testCase.Fatal(reason)
		}
	}
	command(hostClient, map[string]any{"StartGame": map[string]any{}})
	command(hostClient, map[string]any{"PlayerLoaded": map[string]any{}})
	time.Sleep(250 * time.Millisecond)
	if currentSnapshot := room.World().SnapshotV2(); currentSnapshot.Tick != 0 || currentSnapshot.Winner != nil {
		testCase.Fatal("host advanced while peer was loading")
	}
	if commandAccepted, _ := room.ApplyCommand(hostClient.PlayerID, map[string]any{"StopSquads": map[string]any{}}); commandAccepted {
		testCase.Fatal("orders accepted during loading")
	}
	command(guestClient, map[string]any{"PlayerLoaded": map[string]any{}})
	room.mutex.Lock()
	matchStartTime := room.startsAt
	room.mutex.Unlock()
	if matchStartTime.IsZero() || time.Until(matchStartTime) < 500*time.Millisecond {
		testCase.Fatal("missing common start deadline")
	}
	if room.World().SnapshotV2().Tick != 0 {
		testCase.Fatal("advanced before common deadline")
	}
	deadline := time.Now().Add(3 * time.Second)
	for room.World().SnapshotV2().Tick == 0 {
		if time.Now().After(deadline) {
			testCase.Fatal("match never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	room.mutex.Lock()
	encodedData := room.snapshotFrame()
	room.mutex.Unlock()
	var frame struct {
		Snapshot struct {
			Phase   string  `json:"phase"`
			StartAt int64   `json:"startAt"`
			Winner  *string `json:"winner"`
		} `json:"Snapshot"`
	}
	json.Unmarshal(encodedData, &frame)
	if frame.Snapshot.Phase != "running" || frame.Snapshot.StartAt != matchStartTime.UnixMilli() || frame.Snapshot.Winner != nil {
		testCase.Fatalf("bad running snapshot: %s", encodedData)
	}
	if acceptedAfterLoading, reason := room.ApplyCommand(hostClient.PlayerID, map[string]any{"StopSquads": map[string]any{}}); !acceptedAfterLoading {
		testCase.Fatal(reason)
	}
}
func TestDisconnectUnblocksLoading(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("disconnect", "Arena", 2)
	hostClient, _, _ := gameHub.Join(lobby.ID, nil, "host", "ig")
	defer gameHub.Leave(lobby.ID, hostClient)
	guestClient, _, _ := gameHub.Join(lobby.ID, nil, "slow", "ig")
	gameHub.HandleLobbyCommand(lobby.ID, hostClient, map[string]any{"StartGame": map[string]any{}})
	gameHub.HandleLobbyCommand(lobby.ID, hostClient, map[string]any{"PlayerLoaded": map[string]any{}})
	gameHub.Leave(lobby.ID, guestClient)
	room := gameHub.RoomOf(lobby.ID)
	room.mutex.Lock()
	defer room.mutex.Unlock()
	if room.startsAt.IsZero() {
		testCase.Fatal("disconnected peer blocked start forever")
	}
}
