package sim

import (
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestMoveArrives(testCase *testing.T) {
	world := NewWorld()
	squad := world.SpawnSquad("Blue", 1)
	target := protocol.Vec3{X: squad.Pos.X + 20, Y: 0, Z: squad.Pos.Z}
	moved, rejected := world.MoveSquads(1, []int{squad.ID}, target)
	if len(rejected) != 0 || len(moved) != 1 {
		testCase.Fatalf("move rejected: moved=%v rejected=%v", moved, rejected)
	}
	for step := 0; step < 200; step++ {
		world.Advance(0.1)
	}
	arrived := world.Squads[squad.ID]
	if arrived == nil {
		testCase.Fatal("squad vanished")
	}
	if !arrived.Stopped || arrived.MoveOrder != nil {
		testCase.Fatalf("squad did not arrive: stopped=%v order=%v pos=%v", arrived.Stopped, arrived.MoveOrder, arrived.Pos)
	}
	if arrived.Pos.X != target.X || arrived.Pos.Z != target.Z {
		testCase.Fatalf("pos=%v want=%v", arrived.Pos, target)
	}
}

func TestMoveRejectsForeignSquad(testCase *testing.T) {
	world := NewWorld()
	squad := world.SpawnSquad("Blue", 1)
	_, rejected := world.MoveSquads(2, []int{squad.ID}, protocol.Vec3{X: 1})
	if len(rejected) != 1 {
		testCase.Fatalf("expected rejection, got %v", rejected)
	}
}

func TestAttackKills(testCase *testing.T) {
	world := NewWorld()
	attacker := world.SpawnSquad("Blue", 1)
	victim := world.SpawnSquad("Red", 2)
	// Place them in range.
	world.Squads[victim.ID].Pos = protocol.Vec3{X: attacker.Pos.X + 5, Y: 0, Z: attacker.Pos.Z}
	accepted, rejected := world.Attack(1, []int{attacker.ID}, victim.ID)
	if len(accepted) != 1 || len(rejected) != 0 {
		testCase.Fatalf("attack rejected: %v %v", accepted, rejected)
	}
	for step := 0; step < 600 && world.Squads[victim.ID] != nil; step++ {
		world.Advance(0.1)
	}
	if _, alive := world.Squads[victim.ID]; alive {
		testCase.Fatal("target squad survived 60s of point-blank fire")
	}
}

func TestSnapshotShape(testCase *testing.T) {
	world := NewWorld()
	world.SpawnSquad("Blue", 1)
	snapshot := world.Snapshot()
	if len(snapshot.Squads) != 1 {
		testCase.Fatalf("want 1 squad, got %d", len(snapshot.Squads))
	}
	pair := snapshot.Squads[0]
	squadID, valid := pair[0].(int)
	if !valid || squadID == 0 {
		testCase.Fatalf("bad squad id in snapshot: %#v", pair[0])
	}
	squad, valid := pair[1].(map[string]any)
	if !valid {
		testCase.Fatalf("bad squad payload: %T", pair[1])
	}
	for _, key := range []string{"team", "position", "soldiers", "projectile_type", "owner"} {
		if _, found := squad[key]; !found {
			testCase.Fatalf("snapshot squad missing key %q", key)
		}
	}
}
