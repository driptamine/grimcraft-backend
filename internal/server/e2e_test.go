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
func TestWSJoinSessionSnapshot(t *testing.T) {
	testHub := hub.NewHub()
	lobbies := testHub.List()
	lobbyID := lobbies[0].ID

	router := http.NewServeMux()
	router.Handle("/api/", API(testHub))
	router.HandleFunc("/join", transport.WSHandler(testHub))
	if probe, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		t.Skipf("sandbox blocks TCP listen, skipping live-socket check: %v", err)
	} else {
		_ = probe.Close()
	}
	srv := httptest.NewServer(router)
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "/join?lobby=" + lobbyID + "&username=bot&faction=Orks"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	readMsg := func() map[string]any {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatalf("bad json %q: %v", raw, err)
		}
		return envelope
	}

	sessionMsg := readMsg()
	sessionInfo, valid := sessionMsg["Session"].(map[string]any)
	if !valid {
		t.Fatalf("first message must be Session, got %v", sessionMsg)
	}
	if sessionInfo["player_id"] == nil || (sessionInfo["team"] != "Blue" && sessionInfo["team"] != "Red") {
		t.Fatalf("bad session: %v", sessionInfo)
	}
	snapshotMsg := readMsg()
	if snapshotMsg["Snapshot"] == nil {
		t.Fatalf("second message must be Snapshot, got %v", snapshotMsg)
	}

	// Send a MoveSquads for our own squad id.
	squads := snapshotMsg["Snapshot"].(map[string]any)["squads"].([]any)
	if len(squads) == 0 {
		t.Fatal("snapshot has no squads")
	}
	ownSquadID := squads[0].([]any)[0].(float64)
	commandDoc := map[string]any{"Command": map[string]any{"MoveSquads": map[string]any{
		"squad_ids": []any{ownSquadID}, "target": map[string]any{"x": 5, "y": 0, "z": 5},
	}}}
	commandJSON, _ := json.Marshal(commandDoc)
	if err := conn.Write(ctx, websocket.MessageText, commandJSON); err != nil {
		t.Fatal(err)
	}
	// Next messages may be Delta snapshots; wait for the CmdAck.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("no CmdAck received")
		}
		envelope := readMsg()
		if ack, valid := envelope["CmdAck"].(map[string]any); valid {
			if ack["ok"] != true {
				t.Fatalf("command rejected: %v", ack)
			}
			break
		}
	}
}
