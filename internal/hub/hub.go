// Package hub tracks lobbies (one room/game each) and the players inside.
// Model: many lobbies, each lobby owns exactly one sim.World (its room).
package hub

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"grimcraft-go/internal/protocol"
	"grimcraft-go/internal/sim"
)

// PlayerSlot is the lobby-browser player row (matches lobbies.js shape).
type PlayerSlot struct {
	Username string `json:"username"`
	Faction  string `json:"faction"`
	Team     int    `json:"team"` // 1 = Blue, 2 = Red
	Ping     int    `json:"ping"`
}

// Lobby is one entry in the server browser; it owns one Room.
type Lobby struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Map     string       `json:"map"`
	Current int          `json:"current"`
	Total   int          `json:"total"`
	Players []PlayerSlot `json:"players"`

	room *Room
}

// Client is one connected player endpoint (transport-agnostic).
// Send must be non-blocking-safe; hub closes Done on removal.
type Client struct {
	PlayerID int
	Team     string // "Blue" | "Red"
	Out      chan []byte // JSON-encoded ServerMessage
	Done     chan struct{}
	Username string
	Faction  string
}

// Room runs one game: a sim world plus its clients and tick loop.
type Room struct {
	LobbyID string
	world   *sim.World

	mu           sync.Mutex
	clients      map[int]*Client
	nextPlayerID int

	broadcast chan []byte
	quit      chan struct{}
	tickRate  time.Duration
	snapEvery int // ticks between full snapshots
}

// Hub owns all lobbies.
type Hub struct {
	mu      sync.Mutex
	lobbies map[string]*Lobby
	seq     atomic.Int64
}

// NewHub seeds the browser with the two classic entries so the existing
// frontend list has something to show before anyone creates a game.
func NewHub() *Hub {
	hub := &Hub{lobbies: map[string]*Lobby{}}
	hub.CreateLobby("Mechanize — Arena", "Arena", 6)
	hub.CreateLobby("Driptamine — Oasis", "Oasis of Sharr", 8)
	return hub
}

// CreateLobby makes a lobby with its own room and starts the room loop.
func (hub *Hub) CreateLobby(name, mapName string, total int) *Lobby {
	if name == "" {
		name = "My Game"
	}
	if mapName == "" {
		mapName = "Arena"
	}
	if total <= 0 {
		total = 6
	}
	lobbyID := fmt.Sprintf("lobby-%d", hub.seq.Add(1))
	lobby := &Lobby{ID: lobbyID, Name: name, Map: mapName, Total: total, Players: []PlayerSlot{}}
	lobby.room = newRoom(lobbyID)
	hub.mu.Lock()
	hub.lobbies[lobbyID] = lobby
	hub.mu.Unlock()
	return lobby
}

// List returns a snapshot of lobbies for GET /api/lobbies.
func (hub *Hub) List() []*Lobby {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	out := make([]*Lobby, 0, len(hub.lobbies))
	for _, lobby := range hub.lobbies {
		lobbyCopy := *lobby
		lobbyCopy.Players = append([]PlayerSlot(nil), lobby.Players...)
		out = append(out, &lobbyCopy)
	}
	return out
}

// Get returns a lobby by id or nil.
func (hub *Hub) Get(lobbyID string) *Lobby {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return hub.lobbies[lobbyID]
}

func newRoom(lobbyID string) *Room {
	room := &Room{
		LobbyID:   lobbyID,
		world:     sim.NewWorld(),
		clients:   map[int]*Client{},
		broadcast: make(chan []byte, 256),
		quit:      make(chan struct{}),
		tickRate:  100 * time.Millisecond, // 10 ticks/s
		snapEvery: 10,                     // full snapshot 1/s
	}
	go room.loop()
	return room
}

// Join adds a client to the lobby's room, assigns team/player id, spawns
// their starting squad, and returns the Session + Snapshot payloads.
func (hub *Hub) Join(lobbyID string, preferredTeam *string, username, faction string) (*Client, *Lobby, error) {
	hub.mu.Lock()
	lobby, found := hub.lobbies[lobbyID]
	hub.mu.Unlock()
	if !found {
		return nil, nil, fmt.Errorf("unknown lobby %q", lobbyID)
	}
	room := lobby.room
	room.mu.Lock()
	if len(room.clients) >= lobby.Total {
		room.mu.Unlock()
		return nil, nil, fmt.Errorf("lobby full")
	}
	team := pickTeam(room, preferredTeam)
	room.nextPlayerID++
	playerID := room.nextPlayerID
	client := &Client{PlayerID: playerID, Team: team, Out: make(chan []byte, 64), Done: make(chan struct{}), Username: username, Faction: faction}
	room.clients[playerID] = client
	room.mu.Unlock()

	room.world.SpawnSquad(team, playerID)

	hub.mu.Lock()
	lobby.Current = len(room.clients)
	slotTeam := 1
	if team == "Red" {
		slotTeam = 2
	}
	if username == "" {
		username = fmt.Sprintf("Player%d", playerID)
	}
	lobby.Players = append(lobby.Players, PlayerSlot{Username: username, Faction: faction, Team: slotTeam, Ping: 0})
	hub.mu.Unlock()

	return client, lobby, nil
}

// Leave removes a client, drops their squads, and updates the lobby count.
func (hub *Hub) Leave(lobbyID string, client *Client) {
	hub.mu.Lock()
	lobby, found := hub.lobbies[lobbyID]
	hub.mu.Unlock()
	if !found {
		return
	}
	room := lobby.room
	room.mu.Lock()
	if _, stillJoined := room.clients[client.PlayerID]; stillJoined {
		delete(room.clients, client.PlayerID)
		close(client.Done)
	}
	remaining := len(room.clients)
	room.mu.Unlock()
	room.world.RemovePlayerSquads(client.PlayerID)
	hub.mu.Lock()
	lobby.Current = remaining
	keep := lobby.Players[:0]
	for _, slot := range lobby.Players {
		if slot.Username != client.Username || client.Username == "" {
			keep = append(keep, slot)
		}
	}
	// Usernames may collide; simplest correct filter is rebuild by count.
	if len(keep) == len(lobby.Players) && len(lobby.Players) > 0 {
		lobby.Players = lobby.Players[:len(lobby.Players)-1]
	} else {
		lobby.Players = keep
	}
	hub.mu.Unlock()
}

// RoomOf returns the room for a lobby id (nil when unknown).
func (hub *Hub) RoomOf(lobbyID string) *Room {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if lobby, found := hub.lobbies[lobbyID]; found {
		return lobby.room
	}
	return nil
}

func pickTeam(room *Room, preferred *string) string {
	if preferred != nil && (*preferred == "Blue" || *preferred == "Red") {
		return *preferred
	}
	blue, red := 0, 0
	for _, client := range room.clients {
		if client.Team == "Red" {
			red++
		} else {
			blue++
		}
	}
	if red < blue {
		return "Red"
	}
	return "Blue"
}

// World exposes the room simulation (for command handling).
func (room *Room) World() *sim.World { return room.world }

// Clients returns a copy of current clients.
func (room *Room) Clients() []*Client {
	room.mu.Lock()
	defer room.mu.Unlock()
	out := make([]*Client, 0, len(room.clients))
	for _, client := range room.clients {
		out = append(out, client)
	}
	return out
}

// SendSession queues Session + full Snapshot to one client.
func (room *Room) SendSession(client *Client) {
	sessionMsg, _ := json.Marshal(protocol.ServerMessage{Session: &protocol.SessionInfo{PlayerID: client.PlayerID, Team: client.Team}})
	snapshot := room.world.Snapshot()
	snapshotJSON, _ := json.Marshal(protocol.ServerMessage{Snapshot: &snapshot})
	select {
	case client.Out <- sessionMsg:
	default:
	}
	select {
	case client.Out <- snapshotJSON:
	default:
	}
}

// Ack queues a CmdAck to one client.
func Ack(client *Client, accepted bool, reason string) {
	payload, _ := json.Marshal(protocol.ServerMessage{CmdAck: &protocol.CmdAck{OK: accepted, Reason: reason}})
	select {
	case client.Out <- payload:
	default:
	}
}

func (room *Room) loop() {
	ticker := time.NewTicker(room.tickRate)
	defer ticker.Stop()
	counter := 0
	for {
		select {
		case <-room.quit:
			return
		case payload := <-room.broadcast:
			room.mu.Lock()
			for _, client := range room.clients {
				select {
				case client.Out <- payload:
				default:
				}
			}
			room.mu.Unlock()
		case <-ticker.C:
			room.world.Advance(room.tickRate.Seconds())
			counter++
			var update protocol.ServerMessage
			if counter%room.snapEvery == 0 {
				snapshot := room.world.Snapshot()
				update.Snapshot = &snapshot
			} else {
				delta := room.world.Delta(false)
				update.Delta = &delta
			}
			payload, _ := json.Marshal(update)
			room.mu.Lock()
			for _, client := range room.clients {
				select {
				case client.Out <- payload:
				default:
				}
			}
			room.mu.Unlock()
		}
	}
}
