// Package sim is the authoritative RTS simulation: one World per room.
// It owns squads, advances movement/combat on Tick, and renders the
// Snapshot/Delta views the threejs client consumes.
package sim

import (
	"math"
	"sync"

	"grimcraft-go/internal/protocol"
)

// Tuning.
const (
	DefaultSquadSpeed   = 6.0
	DefaultAttackRange  = 18.0
	DefaultDetectRange  = 30.0
	BlueBaseX           = -40.0
	RedBaseX            = 40.0
	ArriveEpsilon       = 0.5
	MeleeRange          = 3.0
	RangedDamagePerTick = 2.0 // at 10 ticks/s
)

// Soldier is intentionally minimal: the net client only needs the slot
// array length/shape to render. Null slots mean casualties.
type Soldier struct {
	HP    float64
	Alive  bool
}

// Squad is the simulated unit.
type Squad struct {
	ID             int
	Team           string // "Blue" | "Red"
	Owner          int
	Pos            protocol.Vec3
	Facing         protocol.Vec3
	Speed          float64
	AttackRange    float64
	DetectRange    float64
	MoveOrder      *protocol.Vec3
	Stopped        bool
	AutoChase      bool
	ProjectileType string
	Soldiers       []Soldier
	ForcedTarget   *int
	IdleTime       float64
	GrenadeCd      float64
	AutoReinforce  bool
	moved          bool // set when position changed this tick
	facingChanged  bool
}

// World is a single room's game state.
type World struct {
	mu     sync.Mutex
	Tick   uint64
	Squads map[int]*Squad
	nextID int
}

// NewWorld returns an empty world.
func NewWorld() *World {
	return &World{Squads: map[int]*Squad{}, nextID: 1}
}

// SpawnSquad creates a 5-soldier squad for owner at the team's base.
func (world *World) SpawnSquad(team string, owner int) *Squad {
	world.mu.Lock()
	defer world.mu.Unlock()
	var baseX float64
	if team == "Red" {
		baseX = RedBaseX
	} else {
		team = "Blue"
		baseX = BlueBaseX
	}
	squadID := world.nextID
	world.nextID++
	squad := &Squad{
		ID: squadID, Team: team, Owner: owner,
		Pos:            protocol.Vec3{X: baseX, Y: 0, Z: float64((squadID % 5) * 6 - 12)},
		Facing:         protocol.Vec3{X: 1, Y: 0, Z: 0},
		Speed:          DefaultSquadSpeed,
		AttackRange:    DefaultAttackRange,
		DetectRange:    DefaultDetectRange,
		Stopped:        true,
		ProjectileType: "Bullet",
		Soldiers:       []Soldier{{HP: 100, Alive: true}, {HP: 100, Alive: true}, {HP: 100, Alive: true}, {HP: 100, Alive: true}, {HP: 100, Alive: true}},
	}
	if team == "Red" {
		squad.Facing.X = -1
	}
	world.Squads[squadID] = squad
	return squad
}

// RemovePlayerSquads drops every squad owned by playerID (on disconnect).
func (world *World) RemovePlayerSquads(playerID int) {
	world.mu.Lock()
	defer world.mu.Unlock()
	for squadID, squad := range world.Squads {
		if squad.Owner == playerID {
			delete(world.Squads, squadID)
		}
	}
}

func dist(from, to protocol.Vec3) float64 {
	deltaX := from.X - to.X
	deltaZ := from.Z - to.Z
	return math.Sqrt(deltaX*deltaX + deltaZ*deltaZ)
}

// Advance moves the simulation forward by dt seconds.
func (world *World) Advance(dt float64) {
	world.mu.Lock()
	defer world.mu.Unlock()
	world.Tick++
	for _, squad := range world.Squads {
		squad.moved = false
		squad.facingChanged = false
		if squad.GrenadeCd > 0 {
			squad.GrenadeCd = math.Max(0, squad.GrenadeCd-dt)
		}
		// Movement toward move order.
		if squad.MoveOrder != nil && !squad.Stopped {
			target := *squad.MoveOrder
			distance := dist(squad.Pos, target)
			step := squad.Speed * dt
			if distance <= math.Max(step, ArriveEpsilon) {
				squad.Pos.X, squad.Pos.Z = target.X, target.Z
				squad.MoveOrder = nil
				squad.Stopped = true
				squad.IdleTime = 0
				squad.moved = true
			} else {
				dirX := (target.X - squad.Pos.X) / distance
				dirZ := (target.Z - squad.Pos.Z) / distance
				squad.Pos.X += dirX * step
				squad.Pos.Z += dirZ * step
				if math.Abs(squad.Facing.X-dirX) > 1e-6 || math.Abs(squad.Facing.Z-dirZ) > 1e-6 {
					squad.Facing.X, squad.Facing.Z = dirX, dirZ
					squad.facingChanged = true
				}
				squad.moved = true
				squad.IdleTime = 0
			}
		} else {
			squad.IdleTime += dt
		}
		// Forced-target attack: damage one alive soldier per tick when in range.
		if squad.ForcedTarget != nil {
			if targetSquad, found := world.Squads[*squad.ForcedTarget]; found && targetSquad.Team != squad.Team {
				if dist(squad.Pos, targetSquad.Pos) > squad.AttackRange {
					// chase: walk toward target
					distance := dist(squad.Pos, targetSquad.Pos)
					if distance > 0 {
						step := squad.Speed * dt
						dirX := (targetSquad.Pos.X - squad.Pos.X) / distance
						dirZ := (targetSquad.Pos.Z - squad.Pos.Z) / distance
						squad.Pos.X += dirX * math.Min(step, distance-MeleeRange*0.5)
						squad.Pos.Z += dirZ * math.Min(step, distance-MeleeRange*0.5)
						squad.Facing.X, squad.Facing.Z = dirX, dirZ
						squad.facingChanged = true
						squad.moved = true
					}
				} else {
					faceX := targetSquad.Pos.X - squad.Pos.X
					faceZ := targetSquad.Pos.Z - squad.Pos.Z
					facingLen := math.Hypot(faceX, faceZ)
					if facingLen > 0 {
						squad.Facing.X, squad.Facing.Z = faceX/facingLen, faceZ/facingLen
						squad.facingChanged = true
					}
					for index := range targetSquad.Soldiers {
						if targetSquad.Soldiers[index].Alive {
							targetSquad.Soldiers[index].HP -= RangedDamagePerTick
							if targetSquad.Soldiers[index].HP <= 0 {
								targetSquad.Soldiers[index].Alive = false
							}
							break
						}
					}
				}
			} else {
				squad.ForcedTarget = nil // target gone
			}
		}
	}
	// Remove wiped squads.
	for squadID, squad := range world.Squads {
		alive := false
		for _, soldier := range squad.Soldiers {
			if soldier.Alive {
				alive = true
				break
			}
		}
		if !alive {
			delete(world.Squads, squadID)
		}
	}
}

// --- Commands (all callable with the world lock held via methods) ---

// MoveSquads orders squads owned by playerID to target. Returns rejected IDs.
func (world *World) MoveSquads(playerID int, squadIDs []int, target protocol.Vec3) (moved, rejected []int) {
	world.mu.Lock()
	defer world.mu.Unlock()
	for _, squadID := range squadIDs {
		squad, found := world.Squads[squadID]
		if !found || squad.Owner != playerID {
			rejected = append(rejected, squadID)
			continue
		}
		order := target
		squad.MoveOrder = &order
		squad.Stopped = false
		squad.ForcedTarget = nil
		moved = append(moved, squadID)
	}
	return moved, rejected
}

// StopSquads halts squads owned by playerID.
func (world *World) StopSquads(playerID int, squadIDs []int) (stopped, rejected []int) {
	world.mu.Lock()
	defer world.mu.Unlock()
	for _, squadID := range squadIDs {
		squad, found := world.Squads[squadID]
		if !found || squad.Owner != playerID {
			rejected = append(rejected, squadID)
			continue
		}
		squad.MoveOrder = nil
		squad.Stopped = true
		squad.ForcedTarget = nil
		stopped = append(stopped, squadID)
	}
	return stopped, rejected
}

// Attack orders attacker squads (owned by playerID) onto a target squad.
func (world *World) Attack(playerID int, attackerIDs []int, targetSquad int) (acceptedIDs, rejected []int) {
	world.mu.Lock()
	defer world.mu.Unlock()
	target, found := world.Squads[targetSquad]
	if !found {
		return nil, attackerIDs
	}
	for _, attackerID := range attackerIDs {
		squad, found := world.Squads[attackerID]
		if !found || squad.Owner != playerID || target.Team == squad.Team {
			rejected = append(rejected, attackerID)
			continue
		}
		targetID := targetSquad
		squad.ForcedTarget = &targetID
		squad.MoveOrder = nil
		squad.Stopped = false
		acceptedIDs = append(acceptedIDs, attackerID)
	}
	return acceptedIDs, rejected
}

// SetProjectileType changes a squad's projectile label.
func (world *World) SetProjectileType(playerID, squadID int, projectile string) bool {
	world.mu.Lock()
	defer world.mu.Unlock()
	squad, found := world.Squads[squadID]
	if !found || squad.Owner != playerID {
		return false
	}
	if projectile == "" {
		projectile = "Bullet"
	}
	squad.ProjectileType = projectile
	return true
}

// SetAutoReinforce toggles auto reinforce.
func (world *World) SetAutoReinforce(playerID, squadID int, enabled bool) bool {
	world.mu.Lock()
	defer world.mu.Unlock()
	squad, found := world.Squads[squadID]
	if !found || squad.Owner != playerID {
		return false
	}
	squad.AutoReinforce = enabled
	return true
}

// QueueReinforce adds one soldier slot (up to 9) as a simple model.
func (world *World) QueueReinforce(playerID, squadID int) bool {
	world.mu.Lock()
	defer world.mu.Unlock()
	squad, found := world.Squads[squadID]
	if !found || squad.Owner != playerID || len(squad.Soldiers) >= 9 {
		return false
	}
	squad.Soldiers = append(squad.Soldiers, Soldier{HP: 100, Alive: true})
	return true
}

// --- Views ---

func soldierSlots(squad *Squad) []any {
	out := make([]any, len(squad.Soldiers))
	for index, soldier := range squad.Soldiers {
		if soldier.Alive {
			out[index] = map[string]any{"hp": soldier.HP}
		} else {
			out[index] = nil
		}
	}
	return out
}

func (squad *Squad) toData() protocol.SquadData {
	var orderCopy *protocol.Vec3
	if squad.MoveOrder != nil {
		copied := *squad.MoveOrder
		orderCopy = &copied
	}
	return protocol.SquadData{
		Team: squad.Team, Position: squad.Pos, Facing: squad.Facing,
		Speed: squad.Speed, AttackRange: squad.AttackRange, DetectionRange: squad.DetectRange,
		GrenadeCd: 0, GrenadeReloadTime: 30, IdleTime: squad.IdleTime,
		ReinforceQueue: []any{}, ReinforceProgress: 0,
		GrenadeQueue: []any{}, GrenadeProgress: 0,
		MoveOrder: orderCopy, Stopped: squad.Stopped, AutoChase: squad.AutoChase,
		ProjectileType: squad.ProjectileType, Soldiers: soldierSlots(squad),
		ForcedTarget: squad.ForcedTarget, ForcedSoldierID: nil,
		AutoReinforce: squad.AutoReinforce, Owner: squad.Owner,
	}
}

// Snapshot renders the full state for _onSnapshot.
func (world *World) Snapshot() protocol.Snapshot {
	world.mu.Lock()
	defer world.mu.Unlock()
	pairs := make([][]any, 0, len(world.Squads))
	for squadID, squad := range world.Squads {
		pairs = append(pairs, []any{squadID, squadToMap(squad)})
	}
	return protocol.Snapshot{Tick: world.Tick, Squads: pairs}
}

// Delta renders positions/facings for squads that moved.
func (world *World) Delta(full bool) protocol.Delta {
	world.mu.Lock()
	defer world.mu.Unlock()
	delta := protocol.Delta{Tick: world.Tick, SquadPos: [][]float64{}, SquadFacing: [][]float64{}}
	for squadID, squad := range world.Squads {
		if full || squad.moved {
			delta.SquadPos = append(delta.SquadPos, []float64{float64(squadID), squad.Pos.X, squad.Pos.Z})
		}
		if full || squad.facingChanged {
			delta.SquadFacing = append(delta.SquadFacing, []float64{float64(squadID), squad.Facing.X, squad.Facing.Z})
		}
	}
	return delta
}

// squadToMap converts to the map shape the client parses (kept as a map so
// encoding/json emits the snake_case keys from SquadData tags).
func squadToMap(squad *Squad) map[string]any {
	data := squad.toData()
	return map[string]any{
		"team": data.Team, "position": data.Position, "facing": data.Facing,
		"speed": data.Speed, "attack_range": data.AttackRange, "detection_range": data.DetectionRange,
		"grenade_cd": data.GrenadeCd, "grenade_reload_time": data.GrenadeReloadTime,
		"idle_time": data.IdleTime, "reinforce_queue": data.ReinforceQueue,
		"reinforce_progress": data.ReinforceProgress, "grenade_queue": data.GrenadeQueue,
		"grenade_progress": data.GrenadeProgress, "move_order": data.MoveOrder,
		"stopped": data.Stopped, "auto_chase": data.AutoChase,
		"projectile_type": data.ProjectileType, "soldiers": data.Soldiers,
		"forced_target": data.ForcedTarget, "forced_soldier_id": data.ForcedSoldierID,
		"auto_reinforce": data.AutoReinforce, "owner": data.Owner,
	}
}
