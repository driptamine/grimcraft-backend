package hub

import (
	"regexp"
	"testing"
)

func TestLobbyIDsAreShortRandomURLSafeStrings(testCase *testing.T) {
	pattern := regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	seen := map[string]bool{}
	for index := 0; index < 10000; index++ {
		lobbyID := newLobbyID()
		if !pattern.MatchString(lobbyID) {
			testCase.Fatalf("invalid ID: %q", lobbyID)
		}
		if seen[lobbyID] {
			testCase.Fatalf("duplicate ID: %q", lobbyID)
		}
		seen[lobbyID] = true
	}
	gameHub := NewHub()
	for _, lobby := range append(gameHub.List(), gameHub.CreateLobby("new", "Arena", 2)) {
		if !pattern.MatchString(lobby.ID) || gameHub.Get(lobby.ID) == nil || gameHub.Get(lobby.ID).ID != lobby.ID || gameHub.RoomOf(lobby.ID).LobbyID != lobby.ID {
			testCase.Fatalf("lobby lookup or room ID mismatch: %s", lobby.ID)
		}
	}
}
