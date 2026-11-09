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
func TestRoomContractSocketFree(t *testing.T) {
	testHub := hub.NewHub()
	lobby := testHub.CreateLobby("contract", "Arena", 4)
	team := "Blue"
	client, _, err := testHub.Join(lobby.ID, &team, "bot", "Orks")
	if err != nil {
		t.Fatal(err)
	}
	room := testHub.RoomOf(lobby.ID)
	room.SendSession(client)

	mustMsg := func() map[string]any {
		select {
		case raw := <-client.Out:
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("bad json: %v", err)
			}
			return decoded
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for server message")
			return nil
		}
	}

	sessionMsg := mustMsg()
	sessionInfo, valid := sessionMsg["Session"].(map[string]any)
	if !valid {
		t.Fatalf("first message must be Session: %v", sessionMsg)
	}
	if sessionInfo["team"] != "Blue" || sessionInfo["player_id"] == nil {
		t.Fatalf("bad session: %v", sessionInfo)
	}

	snapshotMsg := mustMsg()["Snapshot"]
	if snapshotMsg == nil {
		t.Fatal("second message must be Snapshot")
	}
	snapshotBody := snapshotMsg.(map[string]any)
	pairs, valid := snapshotBody["squads"].([]any)
	if !valid || len(pairs) == 0 {
		t.Fatalf("snapshot squads missing: %v", snapshotBody)
	}
	squad := pairs[0].([]any)[1].(map[string]any)
	// Every key _onSnapshot reads in multiplayer.js.
	for _, key := range []string{
		"team", "position", "facing", "speed", "attack_range",
		"detection_range", "grenade_cd", "grenade_reload_time", "idle_time",
		"reinforce_queue", "reinforce_progress", "grenade_queue",
		"grenade_progress", "move_order", "stopped", "auto_chase",
		"projectile_type", "soldiers", "forced_target", "forced_soldier_id",
		"auto_reinforce", "owner",
	} {
		if _, found := squad[key]; !found {
			t.Fatalf("snapshot squad missing key %q", key)
		}
	}
	squadID := int(pairs[0].([]any)[0].(float64))

	var cmd map[string]any
	if err := json.Unmarshal([]byte(
		fmt.Sprintf(`{"MoveSquads":{"squad_ids":[%d],"target":{"x":5,"y":0,"z":5}}}`, squadID)), &cmd); err != nil {
		t.Fatal(err)
	}
	accepted, reason := hub.HandleCommand(room.World(), client.PlayerID, cmd)
	hub.Ack(client, accepted, reason)
	ack := mustMsg()["CmdAck"]
	if ack == nil || ack.(map[string]any)["ok"] != true {
		t.Fatalf("expected ok ack: %v", ack)
	}

	// Delta shape check.
	deltaView := room.World().Delta(true)
	raw, _ := json.Marshal(map[string]any{"Delta": deltaView})
	var decodedDelta map[string]any
	_ = json.Unmarshal(raw, &decodedDelta)
	delta := decodedDelta["Delta"].(map[string]any)
	if delta["tick"] == nil || delta["squad_pos"] == nil || delta["squad_facing"] == nil {
		t.Fatalf("bad delta shape: %v", delta)
	}
}
