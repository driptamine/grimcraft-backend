// Package protocol defines the JSON wire protocol spoken with the
// grimcraft-threejs frontend (src/multiplayer.js).
//
// Client -> server (each as one length-prefixed frame over WebTransport,
// one text message over WebSocket):
//
//	{"Join": {"preferred_team": "Blue"|"Red"|null}}
//	{"Command": {"MoveSquads": {"squad_ids": [...], "target": {"x":..,"y":..,"z":..}}}}
//
// Server -> client:
//
//	{"Session": {"player_id": 7, "team": "Blue"}}
//	{"Snapshot": {"tick": N, "squads": [[id, squadData], ...]}}
//	{"Delta": {"tick": N, "squad_pos": [[id,x,z]...], "squad_facing": [[id,fx,fz]...]}}
//	{"CmdAck": {"ok": true}}
package protocol

// Vec3 mirrors THREE.Vector3 JSON {x,y,z}.
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// JoinRequest is sent by the client right after connecting.
type JoinRequest struct {
	PreferredTeam *string `json:"preferred_team"`
}

// ClientMessage is any single client->server message.
type ClientMessage struct {
	Join    *JoinRequest `json:"Join,omitempty"`
	Command map[string]any `json:"Command,omitempty"`
}

// SessionInfo is the first server->client message.
type SessionInfo struct {
	PlayerID int    `json:"player_id"`
	Team     string `json:"team"` // "Blue" | "Red"
}

// SquadData mirrors what _onSnapshot reads in multiplayer.js.
type SquadData struct {
	Team              string    `json:"team"` // "Blue" | "Red"
	Position          Vec3      `json:"position"`
	Facing            Vec3      `json:"facing"`
	Speed             float64   `json:"speed"`
	AttackRange       float64   `json:"attack_range"`
	DetectionRange    float64   `json:"detection_range"`
	GrenadeCd         float64   `json:"grenade_cd"`
	GrenadeReloadTime float64   `json:"grenade_reload_time"`
	IdleTime          float64   `json:"idle_time"`
	ReinforceQueue    []any     `json:"reinforce_queue"`
	ReinforceProgress float64   `json:"reinforce_progress"`
	GrenadeQueue      []any     `json:"grenade_queue"`
	GrenadeProgress   float64   `json:"grenade_progress"`
	MoveOrder         *Vec3     `json:"move_order"`
	Stopped           bool      `json:"stopped"`
	AutoChase         bool      `json:"auto_chase"`
	ProjectileType    string    `json:"projectile_type"`
	Soldiers          []any     `json:"soldiers"`
	ForcedTarget      *int      `json:"forced_target"`
	ForcedSoldierID   *int      `json:"forced_soldier_id"`
	AutoReinforce     bool      `json:"auto_reinforce"`
	Owner             int       `json:"owner"`
}

// SnapshotEntry is one [id, squadData] pair; squads arrive as a JSON
// array of 2-element arrays, so Marshal/Unmarshal via raw pairs.
type SnapshotEntry struct {
	ID    int
	Squad SquadData
}

// Snapshot is the full-state broadcast.
type Snapshot struct {
	Tick   uint64          `json:"tick"`
	Squads [][]any         `json:"squads"`
}

// Delta is the per-tick position broadcast.
type Delta struct {
	Tick        uint64      `json:"tick"`
	SquadPos    [][]float64 `json:"squad_pos"`
	SquadFacing [][]float64 `json:"squad_facing"`
}

// CmdAck answers every Command.
type CmdAck struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// ServerMessage is any single server->client message.
type ServerMessage struct {
	Session  *SessionInfo `json:"Session,omitempty"`
	Snapshot *Snapshot    `json:"Snapshot,omitempty"`
	Delta    *Delta       `json:"Delta,omitempty"`
	CmdAck   *CmdAck      `json:"CmdAck,omitempty"`
}
