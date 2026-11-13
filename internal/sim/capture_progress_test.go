package sim

import (
	"grimcraft-go/internal/protocol"
	"testing"
)

func TestCaptureProgressWaitsForRodAndStartsDuringRecovery(testCase *testing.T) {
	world := NewWorld()
	squad := world.SpawnSquad("Blue", 1)
	world.EnsureMapPoints("Arena")
	var point *Point
	for _, candidate := range world.Points {
		point = candidate
		break
	}
	if point == nil {
		testCase.Fatal("missing point")
	}
	squad.Pos = point.Pos
	squad.MoveOrder = nil
	squad.CaptureOrderSP = &point.ID
	for step := 0; step < 23; step++ {
		world.Advance(.1)
		if point.CaptureProgress != 0 || point.RaiseFrom != nil {
			testCase.Fatal("capture started before flagPlant1 placed the rod")
		}
	}
	world.Advance(.1)
	if point.CaptureProgress <= 0 || point.RaiseFrom == nil {
		testCase.Fatal("capture did not start after rod placement")
	}
	if animation := soldierAnimation(squad, &squad.Soldiers[0], 0); animation != "flagPlant2" {
		testCase.Fatalf("expected recovery, got %s", animation)
	}
	// A stopped capture must never keep applying progress at the point.
	world.MoveUnits(1, []int{squad.ID}, protocol.Vec3{X: point.Pos.X + 30, Z: point.Pos.Z}, nil)
	progress := point.CaptureProgress
	world.Advance(.1)
	if point.CaptureProgress > progress {
		testCase.Fatal("cancelled capture kept progressing")
	}
}

func TestFinalCaptureAnimationContinuesUntilCaptureEnds(testCase *testing.T) {
	world := NewWorld()
	squad := world.SpawnSquad("Blue", 1)
	pointID := 42
	squad.CapturingSP = &pointID
	for _, elapsed := range []float64{7, 10, 20, 40} {
		squad.CaptureElapsed = elapsed
		if soldierAnimation(squad, &squad.Soldiers[0], 0) != "capture2" {
			testCase.Fatal("planter stopped final capture animation early")
		}
	}
	squad.CapturingSP = nil
	if soldierAnimation(squad, &squad.Soldiers[0], 0) != "idle" {
		testCase.Fatal("planter did not stop after capture")
	}
}
