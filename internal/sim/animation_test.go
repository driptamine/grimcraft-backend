package sim

import (
	"grimcraft-go/internal/protocol"
	"testing"
)

func actorFrom(world *World, entityID int) map[string]any {
	for _, entity := range world.SnapshotV2().Entities {
		entityRecord := entity.(map[string]any)
		if entityRecord["id"] == entityID {
			for key, component := range entityRecord["components"].(map[string]any) {
				if key != "networkOwner" {
					return component.(map[string]any)
				}
			}
		}
	}
	return nil
}
func TestActionAnimations(testCase *testing.T) {
	world := NewWorld()
	attackingSquad := world.SpawnSquad("Blue", 1)
	defendingSquad := world.SpawnSquad("Red", 2)
	world.MoveSquads(1, []int{attackingSquad.ID}, protocol.Vec3{X: attackingSquad.Pos.X + 10, Z: attackingSquad.Pos.Z})
	world.Advance(.1)
	if actorFrom(world, attackingSquad.Soldiers[0].EntityID)["animation"] != "run" {
		testCase.Fatal("moving soldier stays idle")
	}
	world.StopSquads(1, []int{attackingSquad.ID})
	world.Advance(.1)
	if actorFrom(world, attackingSquad.Soldiers[0].EntityID)["animation"] != "idle" {
		testCase.Fatal("stop did not idle")
	}
	attackingSquad.AttackMode = "melee"
	defendingSquad.Pos = protocol.Vec3{X: attackingSquad.Pos.X + 12, Z: attackingSquad.Pos.Z}
	world.Attack(1, []int{attackingSquad.ID}, defendingSquad.ID)
	world.Advance(.1)
	if actorFrom(world, attackingSquad.Soldiers[0].EntityID)["animation"] != "shootMove" || attackingSquad.Soldiers[0].ShotSerial == 0 {
		testCase.Fatal("assault must fire while approaching")
	}
	defendingSquad.Pos = protocol.Vec3{X: attackingSquad.Pos.X + 1, Z: attackingSquad.Pos.Z}
	attackingSquad.visualCooldown = 0
	world.Advance(.1)
	if attackingSquad.Soldiers[0].MeleeTimer <= 0 || attackingSquad.Soldiers[0].MeleeSerial != 1 {
		testCase.Fatal("missing melee event")
	}
	if defendingSquad.Soldiers[0].HitReactionAnim != "hitL" {
		testCase.Fatal("missing melee damage reaction")
	}
	attackingSquad.visualCooldown = 0
	world.Advance(.1)
	if attackingSquad.Soldiers[0].MeleeSerial != 2 || defendingSquad.Soldiers[0].HitReactionAnim != "hitR" {
		testCase.Fatal("melee/reaction sequence not advancing")
	}
	defendingSquad.Soldiers[0].Alive = false
	if actorFrom(world, defendingSquad.Soldiers[0].EntityID) != nil {
		testCase.Fatal("dead entity prevents client death animation")
	}
	worker := world.SpawnBuilder("Blue", 1, "ig", 0, 0)
	worker.MoveTarget = &protocol.Vec3{X: 10}
	world.Advance(.1)
	if actorFrom(world, worker.ID)["animation"] != "run" {
		testCase.Fatal("worker stays idle")
	}
	worker.MoveTarget = nil
	worker.IsBuilding = true
	world.Advance(.1)
	if actorFrom(world, worker.ID)["animation"] != "build" {
		testCase.Fatal("missing build animation")
	}
}

func TestCaptureAnimationSequence(testCase *testing.T) {
	world := NewWorld()
	squad := world.SpawnSquad("Blue", 1)
	point := 42
	squad.CapturingSP = &point
	for _, currentTestCase := range []struct {
		elapsed float64
		want    string
	}{{1, "flagPlant1"}, {3, "flagPlant2"}, {5, "capture1"}, {7, "capture2"}} {
		squad.CaptureElapsed = currentTestCase.elapsed
		if actualResult := soldierAnimation(squad, &squad.Soldiers[0], 0); actualResult != currentTestCase.want {
			testCase.Fatalf("capture at %v: %s", currentTestCase.elapsed, actualResult)
		}
	}
}
