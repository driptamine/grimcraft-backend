// Package hub tracks lobbies (one room/game each) and the players inside.
// Model: many lobbies, each lobby owns exactly one sim.World (its room).
package hub

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"grimcraft-go/internal/balance"
	"grimcraft-go/internal/protocol"
	"grimcraft-go/internal/sim"
)

// PlayerSlot is the lobby-browser player row (matches lobbies.js shape).
type PlayerSlot struct {
	PlayerID   int    `json:"player_id"`
	Controller string `json:"controller,omitempty"`
	Username   string `json:"username"`
	Faction    string `json:"faction"`
	Team       int    `json:"team"` // 1 = Blue, 2 = Red
	Ping       int    `json:"ping"`
}

// Lobby is one entry in the server browser; it owns one Room.
type Lobby struct {
	BalanceMod  *balance.Mod `json:"balanceMod,omitempty"`
	BalanceHash string       `json:"balanceHash,omitempty"`
	HostID      int          `json:"host_id"`
	Started     bool         `json:"started"`
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Map         string       `json:"map"`
	Current     int          `json:"current"`
	Total       int          `json:"total"`
	Players     []PlayerSlot `json:"players"`

	room *Room
}

// Client is one connected player endpoint (transport-agnostic).
// Send must be non-blocking-safe; hub closes Done on removal.
type Client struct {
	ready    bool // guarded by room.mu; Session must precede tick updates
	PlayerID int
	Team     string      // "Blue" | "Red"
	Out      chan []byte // JSON-encoded ServerMessage
	Done     chan struct{}
	Username string
	Faction  string
}

// Room runs one game: a sim world plus its clients and tick loop.
type Room struct {
	LobbyID string
	world   *sim.World

	mutex        sync.Mutex
	clients      map[int]*Client
	nextPlayerID int
	loading      bool
	loaded       map[int]bool
	startsAt     time.Time
	running      bool

	broadcast chan []byte
	quit      chan struct{}
	tickRate  time.Duration
	snapEvery int // ticks between full snapshots
}

// Hub owns all lobbies.
type Hub struct {
	mutex   sync.Mutex
	lobbies map[string]*Lobby
}

// NewHub seeds the browser with the two classic entries so the existing
// frontend list has something to show before anyone creates a game.
func NewHub() *Hub {
	hub := &Hub{lobbies: map[string]*Lobby{}}
	hub.CreateLobby("Mechanize — Arena", "Arena", 6)
	hub.CreateLobby("Driptamine — Oasis", "Oasis of Sharr", 8)
	return hub
}

// Eight random bytes encode to eleven URL-safe characters without padding.
func newLobbyID() string {
	var randomBytes [8]byte
	if _, operationError := rand.Read(randomBytes[:]); operationError != nil {
		panic(operationError)
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes[:])
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
	lobbyID := newLobbyID()
	hub.mutex.Lock()
	for hub.lobbies[lobbyID] != nil {
		lobbyID = newLobbyID()
	}
	lobby := &Lobby{ID: lobbyID, Name: name, Map: mapName, Total: total, Players: []PlayerSlot{}}
	lobby.room = newRoom(lobbyID, mapName)
	hub.lobbies[lobbyID] = lobby
	hub.mutex.Unlock()
	return lobby
}

// List returns a snapshot of lobbies for GET /api/lobbies.
func (hub *Hub) List() []*Lobby {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	result := make([]*Lobby, 0, len(hub.lobbies))
	for _, lobby := range hub.lobbies {
		lobbyCopy := *lobby
		lobbyCopy.Players = append([]PlayerSlot(nil), lobby.Players...)
		result = append(result, &lobbyCopy)
	}
	return result
}

// Get returns a lobby by id or nil.
func (hub *Hub) Get(lobbyID string) *Lobby {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
	return hub.lobbies[lobbyID]
}

func newRoom(lobbyID, mapName string) *Room {
	room := &Room{
		LobbyID:   lobbyID,
		world:     sim.NewWorld(),
		clients:   map[int]*Client{},
		broadcast: make(chan []byte, 256),
		quit:      make(chan struct{}),
		tickRate:  100 * time.Millisecond, // 10 ticks/s
		snapEvery: 10,                     // full snapshot 1/s
	}
	room.world.EnsureMapPoints(mapName)
	go room.loop()
	return room
}

// Join adds a client to the lobby's room, assigns team/player id, spawns
// their starting squad, and returns the Session + Snapshot payloads.
func (hub *Hub) Join(lobbyID string, preferredTeam *string, username, faction string) (*Client, *Lobby, error) {
	hub.mutex.Lock()
	lobby, found := hub.lobbies[lobbyID]
	if !found {
		hub.mutex.Unlock()
		return nil, nil, fmt.Errorf("unknown lobby %q", lobbyID)
	}
	room := lobby.room
	room.mutex.Lock()
	if len(lobby.Players) >= lobby.Total {
		room.mutex.Unlock()
		hub.mutex.Unlock()
		return nil, nil, fmt.Errorf("lobby full")
	}
	team := pickTeam(room, preferredTeam)
	room.nextPlayerID++
	playerID := room.nextPlayerID
	client := &Client{PlayerID: playerID, Team: team, Out: make(chan []byte, 64), Done: make(chan struct{}), Username: username, Faction: faction}
	room.clients[playerID] = client
	if room.loading && !room.running {
		room.startsAt = time.Time{}
	}
	room.mutex.Unlock()

	room.world.SpawnLobbySquad(team, playerID, faction, lobby.Map)
	if lobby.HostID == 0 {
		lobby.HostID = playerID
	}
	lobby.Current = len(lobby.Players) + 1
	slotTeam := 1
	if team == "Red" {
		slotTeam = 2
	}
	if username == "" {
		username = fmt.Sprintf("Player%d", playerID)
	}
	lobby.Players = append(lobby.Players, PlayerSlot{PlayerID: playerID, Controller: "human", Username: username, Faction: faction, Team: slotTeam, Ping: 0})
	hub.mutex.Unlock()
	hub.BroadcastLobby(lobbyID, playerID)

	return client, lobby, nil
}

// Leave removes a client, drops their squads, and updates the lobby count.
func (hub *Hub) Leave(lobbyID string, client *Client) {
	hub.mutex.Lock()
	lobby, found := hub.lobbies[lobbyID]
	if !found {
		hub.mutex.Unlock()
		return
	}
	room := lobby.room
	room.mutex.Lock()
	if _, stillJoined := room.clients[client.PlayerID]; stillJoined {
		delete(room.clients, client.PlayerID)
		close(client.Done)
	}

	room.maybeStartLocked()
	remaining := len(room.clients)
	room.mutex.Unlock()
	room.world.RemovePlayerSquads(client.PlayerID)
	keep := lobby.Players[:0]
	for _, slot := range lobby.Players {
		if slot.PlayerID != client.PlayerID {
			keep = append(keep, slot)
		}
	}
	if remaining == 0 {
		room.mutex.Lock()
		room.loading = false
		room.running = false
		room.startsAt = time.Time{}
		room.loaded = nil
		room.mutex.Unlock()
		for _, slot := range keep {
			if slot.PlayerID < 0 {
				room.world.RemovePlayerSquads(slot.PlayerID)
			}
		}
		keep = nil
		lobby.Started = false
	}
	lobby.Players = keep
	lobby.Current = len(keep)
	if lobby.HostID == client.PlayerID {
		lobby.HostID = 0
		for _, slot := range keep {
			if slot.PlayerID > 0 {
				lobby.HostID = slot.PlayerID
				break
			}
		}
	}

	hub.mutex.Unlock()
	hub.BroadcastLobby(lobbyID)
}

// RoomOf returns the room for a lobby id (nil when unknown).
func (hub *Hub) RoomOf(lobbyID string) *Room {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()
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
	room.mutex.Lock()
	defer room.mutex.Unlock()
	result := make([]*Client, 0, len(room.clients))
	for _, client := range room.clients {
		result = append(result, client)
	}
	return result
}

// SendSession queues Session + full Snapshot to one client.
func (room *Room) SendSession(client *Client) {
	room.mutex.Lock()
	defer room.mutex.Unlock()
	sessionMsg, _ := json.Marshal(protocol.ServerMessage{Session: &protocol.SessionInfo{PlayerID: client.PlayerID, Team: client.Team}})
	snapshotJSON := room.snapshotFrame()
	select {
	case client.Out <- sessionMsg:
	default:
	}
	select {
	case client.Out <- snapshotJSON:
	default:
	}
	client.ready = true
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
	for {
		select {
		case <-room.quit:
			return
		case payload := <-room.broadcast:
			room.mutex.Lock()
			for _, client := range room.clients {
				if !client.ready {
					continue
				}
				select {
				case client.Out <- payload:
				default:
				}
			}
			room.mutex.Unlock()
		case <-ticker.C:
			room.mutex.Lock()
			if room.loading && !room.startsAt.IsZero() && !time.Now().Before(room.startsAt) {
				room.running = true
			}
			if room.running {
				room.world.Advance(room.tickRate.Seconds())
			}
			payload := room.snapshotFrame()
			for _, client := range room.clients {
				if !client.ready {
					continue
				}
				select {
				case client.Out <- payload:
				default:
				}
			}
			room.mutex.Unlock()
		}
	}
}
