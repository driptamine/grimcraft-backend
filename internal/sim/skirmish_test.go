package sim

import (
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestSkirmishParity(testCase *testing.T) {
	world := NewWorld()
	world.EnsurePlayer(1, "Blue", "ig")
	world.EnsurePlayer(2, "Red", "ig")
	world.EnsureMapPoints("Arena")
	builder := world.SpawnBuilder("Blue", 1, "ig", -30, 0)
	if builder == nil {
		testCase.Fatal("no builder")
	}
	// PlaceBuilding deducts cost and creates constructing building.
	startingRequisition := world.Players[1].Requisition
	entityID, isValid := world.PlaceBuilding(1, builder.ID, "barracks", protocol.Vec3{X: -25, Z: 0})
	if !isValid {
		testCase.Fatal("PlaceBuilding rejected")
	}
	if world.Players[1].Requisition >= startingRequisition {
		testCase.Fatal("PlaceBuilding must deduct requisition")
	}
	// Finish construction via ticks.
	for index := 0; index < 200 && world.Buildings[entityID].Constructing; index++ {
		world.Advance(0.1)
	}
	if world.Buildings[entityID].Constructing {
		testCase.Fatal("building never finished")
	}
	// TrainUnit queues production.
	if !world.TrainUnit(1, entityID, "squad") {
		testCase.Fatal("TrainUnit rejected")
	}
	initialSquadCount := len(world.Squads)
	for currentIndex := 0; currentIndex < 200 && len(world.Squads) == initialSquadCount; currentIndex++ {
		world.Advance(0.1)
	}
	if len(world.Squads) == initialSquadCount {
		testCase.Fatal("TrainUnit never produced squad")
	}
	// Reinforce with cost/time: queue then gradual fill.
	var squad *Squad
	for _, ownedSquadCandidate := range world.Squads {
		if ownedSquadCandidate.Owner == 1 {
			squad = ownedSquadCandidate
			break
		}
	}
	if squad == nil {
		testCase.Fatal("no owned squad")
	}
	// Kill one soldier slot to open space.
	world.MuLockTestKillSoldier(squad.ID)
	requisitionBefore := world.Players[1].Requisition
	if !world.QueueReinforceAt(1, squad.ID, 1) {
		testCase.Fatal("QueueReinforceAt rejected")
	}
	if world.Players[1].Requisition >= requisitionBefore {
		testCase.Fatal("reinforce must cost requisition")
	}
	// Research queue.
	if !world.QueueResearch(1, entityID, "test_research") {
		testCase.Fatal("QueueResearch rejected")
	}
	for researchStep := 0; researchStep < 300 && !world.Players[1].Research["test_research"]; researchStep++ {
		world.Advance(0.1)
	}
	if !world.Players[1].Research["test_research"] {
		testCase.Fatal("research never completed")
	}
	// SnapshotV2 shape for applySkirmishSnapshot.
	snap := world.SnapshotV2()
	if snap.Schema != 2 || len(snap.Entities) == 0 || len(snap.Players) == 0 {
		testCase.Fatalf("bad v2 snapshot: %+v", snap)
	}
}
