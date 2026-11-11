package hub

import "testing"

func TestOnlineLobbyStartFactionAndHost(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("incognito", "6p alvarus", 4)
	host, _, operationError := gameHub.Join(lobby.ID, nil, "Host", "Imperial Guard")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	guest, _, operationError := gameHub.Join(lobby.ID, nil, "Guest", "Tau")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	if host.Team != "Blue" || guest.Team != "Red" {
		testCase.Fatal("players must join opposite sides")
	}
	handled, isValid, _ := gameHub.HandleLobbyCommand(lobby.ID, guest, map[string]any{"StartGame": map[string]any{}})
	if !handled || isValid {
		testCase.Fatal("guest must not start host's game")
	}
	_, isValid, _ = gameHub.HandleLobbyCommand(lobby.ID, host, map[string]any{"SetLobbyFaction": map[string]any{"faction": "Space Marines"}})
	if !isValid {
		testCase.Fatal("cannot choose faction")
	}
	_, isValid, reason := gameHub.HandleLobbyCommand(lobby.ID, host, map[string]any{"StartGame": map[string]any{"computers": []any{map[string]any{"faction": "Necrons", "team": 2}}}})
	if !isValid {
		testCase.Fatal(reason)
	}
	if !lobby.Started || lobby.Current != 3 {
		testCase.Fatalf("wrong lobby state: %+v", lobby)
	}
	snapshot := lobby.room.World().Snapshot()
	if len(snapshot.Squads) != 6 {
		testCase.Fatal("missing computer")
	}
	found := false
	for _, pair := range snapshot.Squads {
		data := pair[1].(map[string]any)
		if data["owner"] == host.PlayerID {
			found = data["faction"] == "space marines"
			position := data["position"]
			if position == nil {
				testCase.Fatal("no position")
			}
		}
	}
	if !found {
		testCase.Fatal("chosen faction not replicated")
	}
	_, isValid, _ = gameHub.HandleLobbyCommand(lobby.ID, host, map[string]any{"StartGame": map[string]any{}})
	if !isValid || len(lobby.room.World().Snapshot().Squads) != 6 {
		testCase.Fatal("repeated start duplicated units")
	}
	gameHub.Leave(lobby.ID, guest)
	if lobby.Current != 2 {
		testCase.Fatal("disconnect must remove only the departing human")
	}
	gameHub.Leave(lobby.ID, host)
	if lobby.Current != 0 || lobby.Started || len(lobby.room.World().Snapshot().Squads) != 0 {
		testCase.Fatal("empty rooms must reset and remove computer squads")
	}
}

func TestSameUsernameDisconnectUsesPlayerID(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("names", "Arena", 2)
	hostClient, _, _ := gameHub.Join(lobby.ID, nil, "Player", "Orks")
	guestClient, _, _ := gameHub.Join(lobby.ID, nil, "Player", "Tau")
	gameHub.Leave(lobby.ID, guestClient)
	if lobby.Current != 1 || len(lobby.Players) != 1 || lobby.Players[0].PlayerID != hostClient.PlayerID {
		testCase.Fatal("disconnect removed the wrong player")
	}
}
