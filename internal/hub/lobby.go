package hub

import (
	"encoding/json"
	"fmt"
	"time"
)

func (gameHub *Hub) BroadcastLobby(lobbyID string, exclude ...int) {
	gameHub.mutex.Lock()
	lobby := gameHub.lobbies[lobbyID]
	if lobby == nil {
		gameHub.mutex.Unlock()
		return
	}
	data, _ := json.Marshal(map[string]any{"Lobby": lobby})
	room := lobby.room
	gameHub.mutex.Unlock()
	for _, client := range room.Clients() {
		if len(exclude) > 0 && client.PlayerID == exclude[0] {
			continue
		}
		select {
		case client.Out <- data:
		default:
		}
	}
}

// HandleLobbyCommand is shared by WebSocket and WebTransport clients.
func (gameHub *Hub) HandleLobbyCommand(lobbyID string, client *Client, command map[string]any) (handled, isValid bool, reason string) {
	_, start := command["StartGame"]
	_, loaded := command["PlayerLoaded"]
	factionArgs, factionChange := command["SetLobbyFaction"]
	if !start && !factionChange && !loaded {
		return false, false, ""
	}
	gameHub.mutex.Lock()
	lobby := gameHub.lobbies[lobbyID]
	if lobby == nil {
		gameHub.mutex.Unlock()
		return true, false, "unknown lobby"
	}
	if loaded {
		if !lobby.Started {
			gameHub.mutex.Unlock()
			return true, false, "match has not started loading"
		}
		room := lobby.room
		room.mutex.Lock()
		if room.clients[client.PlayerID] != client {
			room.mutex.Unlock()
			gameHub.mutex.Unlock()
			return true, false, "not a member"
		}
		room.loaded[client.PlayerID] = true
		room.maybeStartLocked()
		room.mutex.Unlock()
		gameHub.mutex.Unlock()
		return true, true, ""
	}
	if factionChange {
		if lobby.Started {
			gameHub.mutex.Unlock()
			return true, false, "game already started"
		}
		arguments, _ := factionArgs.(map[string]any)
		faction, _ := arguments["faction"].(string)
		client.Faction = faction
		for index := range lobby.Players {
			if lobby.Players[index].PlayerID == client.PlayerID {
				lobby.Players[index].Faction = faction
			}
		}
		lobby.room.world.SetFaction(client.PlayerID, faction)
		gameHub.mutex.Unlock()
		gameHub.BroadcastLobby(lobbyID)
		return true, true, ""
	}
	if client.PlayerID != lobby.HostID {
		gameHub.mutex.Unlock()
		return true, false, "only the host can start"
	}
	if lobby.Started {
		gameHub.mutex.Unlock()
		return true, true, ""
	}
	var currentArguments struct {
		Computers []struct {
			Faction string `json:"faction"`
			Team    int    `json:"team"`
		} `json:"computers"`
	}
	encodedData, _ := json.Marshal(command["StartGame"])
	if operationError := json.Unmarshal(encodedData, &currentArguments); operationError != nil {
		gameHub.mutex.Unlock()
		return true, false, "invalid computer slots"
	}
	if len(lobby.Players)+len(currentArguments.Computers) > lobby.Total {
		gameHub.mutex.Unlock()
		return true, false, "lobby full"
	}
	teams := map[int]bool{}
	for _, playerSlot := range lobby.Players {
		teams[playerSlot.Team] = true
	}
	for _, bot := range currentArguments.Computers {
		if bot.Team != 1 && bot.Team != 2 {
			gameHub.mutex.Unlock()
			return true, false, "invalid computer team"
		}
		teams[bot.Team] = true
	}
	if len(teams) < 2 {
		gameHub.mutex.Unlock()
		return true, false, "add an opponent or wait for another player"
	}
	for currentIndex, bot := range currentArguments.Computers {
		team := "Blue"
		if bot.Team == 2 {
			team = "Red"
		}
		owner := -(currentIndex + 1)
		lobby.room.world.AddComputer(team, owner, bot.Faction, lobby.Map)
		lobby.Players = append(lobby.Players, PlayerSlot{PlayerID: owner, Controller: "computer", Username: fmt.Sprintf("Computer %d", currentIndex+1), Faction: bot.Faction, Team: bot.Team})
	}
	lobby.room.mutex.Lock()
	lobby.room.world.ResetMatchClock()
	lobby.room.loading = true
	lobby.room.running = false
	lobby.room.loaded = map[int]bool{}
	lobby.room.startsAt = time.Time{}
	lobby.room.mutex.Unlock()
	lobby.Started = true
	lobby.Current = len(lobby.Players)
	gameHub.mutex.Unlock()
	gameHub.BroadcastLobby(lobbyID)
	return true, true, ""
}
