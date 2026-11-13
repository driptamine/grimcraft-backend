package sim

import (
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestInfantryCommandDeliveryAndWorkerLifetime(testCase *testing.T) {
	world := NewWorld()
	world.EnsurePlayer(1, "Blue", "ig")
	worker := world.SpawnBuilder("Blue", 1, "ig", -40, 0)
	buildingID, accepted := world.PlaceBuilding(1, worker.ID, "infantry_command", protocol.Vec3{})
	if !accepted {
		testCase.Fatal("placement rejected")
	}
	building := world.Buildings[buildingID]
	world.Advance(.5)
	if building.DeliveryProgress != .5 || building.Progress != 0 || !building.PendingWorker {
		testCase.Fatalf("delivery must advance before worker arrival: %+v", building)
	}
	if actorFrom(world, worker.ID)["animation"] != "run" {
		testCase.Fatal("worker must approach the site instead of building remotely")
	}
	snapshot := actorFrom(world, buildingID)
	if snapshot["deliveryProgress"] != .5 || snapshot["pendingWorker"] != true {
		testCase.Fatalf("snapshot must include independent delivery progress: %+v", snapshot)
	}
	world.Advance(.5)
	if building.DeliveryProgress != building.DeliveryTime || building.Progress != 0 {
		testCase.Fatal("delivered site must wait for its worker")
	}
	for step := 0; step < 100 && building.PendingWorker; step++ {
		world.Advance(.1)
	}
	if building.PendingWorker || !worker.IsBuilding || worker.MoveTarget != nil {
		testCase.Fatal("worker did not start assembly after arriving")
	}
	for step := 0; step < 40; step++ {
		world.Advance(.1)
	}
	if !building.Constructing || !worker.IsBuilding || worker.BuildTarget == nil || worker.BuildProg <= 3 {
		testCase.Fatal("worker stopped building after the old three-second preparation")
	}
	if actorFrom(world, worker.ID)["animation"] != "build" {
		testCase.Fatal("worker must keep its construction animation while assembly continues")
	}
	for step := 0; step < 100 && building.Constructing; step++ {
		world.Advance(.1)
	}
	if building.Constructing || worker.IsBuilding || worker.BuildTarget != nil || actorFrom(world, worker.ID)["animation"] != "idle" {
		testCase.Fatal("completed building did not release its worker")
	}
}

func TestInfantryCommandDeliveryCancelAndResume(testCase *testing.T) {
	world := NewWorld()
	world.EnsurePlayer(1, "Blue", "ig")
	worker := world.SpawnBuilder("Blue", 1, "ig", -40, 0)
	buildingID, accepted := world.PlaceBuilding(1, worker.ID, "barracks", protocol.Vec3{})
	if !accepted {
		testCase.Fatal("placement rejected")
	}
	building := world.Buildings[buildingID]
	world.StopUnits(1, []int{worker.ID})
	world.Advance(2)
	if worker.IsBuilding || worker.BuildTarget != nil || building.DeliveryProgress != building.DeliveryTime || building.Progress != 0 {
		testCase.Fatal("delivery should finish independently of a cancelled worker order")
	}
	if !world.ResumeBuild(1, worker.ID, buildingID) {
		testCase.Fatal("resume rejected")
	}
	world.Advance(.1)
	if !building.PendingWorker || worker.IsBuilding || !worker.moved || building.DeliveryProgress != building.DeliveryTime {
		testCase.Fatal("resuming must approach the site without replaying delivery")
	}
	worker.Pos = *worker.BuildWorkPos
	world.Advance(.1)
	if building.PendingWorker || !worker.IsBuilding || building.Progress <= 0 {
		testCase.Fatal("resumed worker did not start assembly")
	}
}

func TestAdditionalIGConstructionSites(testCase *testing.T) {
	for _, buildingType := range []string{"listening_post", "mechanized", "mars_pattern", "plasma_generator", "tactica_control", "turret", "hq", "hq-anim"} {
		testCase.Run(buildingType, func(testCase *testing.T) {
			world := NewWorld()
			world.EnsurePlayer(1, "Blue", "ig")
			worker := world.SpawnBuilder("Blue", 1, "ig", -40, 0)
			pointID := world.allocID()
			world.Points[pointID] = &Point{ID: pointID, Owner: "Blue", Pos: protocol.Vec3{}, Kind: "point"}
			entityID, accepted := world.PlaceBuilding(1, worker.ID, buildingType, protocol.Vec3{})
			if !accepted {
				testCase.Fatal("placement rejected")
			}
			world.Advance(.5)
			record := actorFrom(world, entityID)
			if record["deliveryProgress"] != .5 || record["constructProgress"] != 0.0 || record["pendingWorker"] != true {
				testCase.Fatalf("delivery must precede worker arrival: %+v", record)
			}
			if buildingType == "listening_post" {
				point := world.Points[pointID]
				if point.HasListeningPost || point.ListeningPostEntityID == nil || *point.ListeningPostEntityID != entityID {
					testCase.Fatal("point must reference its unfinished listening post")
				}
				startingRequisition := world.Players[1].Requisition
				if _, duplicateAccepted := world.PlaceBuilding(1, worker.ID, buildingType, protocol.Vec3{}); duplicateAccepted || world.Players[1].Requisition != startingRequisition {
					testCase.Fatal("duplicate placement must not create or charge for another post")
				}
			}
			world.Advance(.5)
			worker.Pos = *worker.BuildWorkPos
			for step := 0; step < 40; step++ {
				world.Advance(.1)
			}
			if !worker.IsBuilding || worker.BuildTarget == nil || actorFrom(world, worker.ID)["animation"] != "build" {
				testCase.Fatal("worker stopped assembly after the old preparation timer")
			}
			pausedProgress := actorFrom(world, entityID)["constructProgress"]
			world.MoveUnits(1, []int{worker.ID}, protocol.Vec3{X: -40}, nil)
			world.Advance(1)
			if record := actorFrom(world, entityID); record["constructProgress"] != pausedProgress || record["pendingWorker"] != true {
				testCase.Fatalf("construction continued after the worker received a move order: %+v", record)
			}
			worker.Pos = protocol.Vec3{X: 4}
			if !world.ResumeBuild(1, worker.ID, entityID) {
				testCase.Fatal("resume rejected")
			}
			worker.Pos = *worker.BuildWorkPos
			world.Advance(.1)
			if actorFrom(world, entityID)["constructProgress"].(float64) <= pausedProgress.(float64) {
				testCase.Fatal("resuming did not preserve and advance the previous progress")
			}
			pausedProgress = actorFrom(world, entityID)["constructProgress"]
			worker.Pos = protocol.Vec3{X: 30}
			world.Advance(.1)
			if actorFrom(world, entityID)["constructProgress"] != pausedProgress {
				testCase.Fatal("a distant worker continued construction")
			}
			worker.Pos = protocol.Vec3{X: 4}
			worker.HP = 0
			world.Advance(.1)
			if actorFrom(world, entityID)["constructProgress"] != pausedProgress {
				testCase.Fatal("a dead worker continued construction")
			}
			replacement := world.SpawnBuilder("Blue", 1, "ig", 4, 0)
			if !world.ResumeBuild(1, replacement.ID, entityID) {
				testCase.Fatal("replacement worker could not resume the site")
			}
			for step := 0; step < 200 && actorFrom(world, entityID)["constructing"] == true; step++ {
				world.Advance(.1)
			}
			if actorFrom(world, entityID)["constructing"] != false || worker.IsBuilding || worker.BuildTarget != nil {
				testCase.Fatal("finished site did not release its worker")
			}
			if buildingType == "listening_post" {
				point := world.Points[pointID]
				if !point.HasListeningPost || point.LPTier != 1 || point.Faction != "ig" {
					testCase.Fatal("completed post did not enable the point's income and upgrades")
				}
				if !world.Demolish(1, entityID) || point.HasListeningPost || point.ListeningPostEntityID != nil {
					testCase.Fatal("removing a post must release its strategic point")
				}
			}
		})
	}
}

func TestStartingHeadquartersRemainReady(testCase *testing.T) {
	world := NewWorld()
	world.SpawnLobbySquad("Blue", 1, "ig", "Arena")
	for _, building := range world.Buildings {
		if building.Type == "hq" {
			if building.Constructing || building.PendingWorker || building.DeliveryTime != 0 || building.HP != 2000 {
				testCase.Fatalf("starting HQ must be ready when the match begins: %+v", building)
			}
			if !world.TrainUnit(1, building.ID, "builder") {
				testCase.Fatal("starting HQ must allow production immediately")
			}
			return
		}
	}
	testCase.Fatal("missing starting HQ")
}

func TestUnanimatedSiteRequiresWorker(testCase *testing.T) {
	world := NewWorld()
	world.EnsurePlayer(1, "Blue", "chaos")
	worker := world.SpawnBuilder("Blue", 1, "chaos", 4, 0)
	entityID, accepted := world.PlaceBuilding(1, worker.ID, "plasma_generator", protocol.Vec3{})
	if !accepted {
		testCase.Fatal("placement rejected")
	}
	worker.Pos = *worker.BuildWorkPos
	world.Advance(.5)
	site := world.Structures[entityID]
	if !site.RequiresWorker || site.Progress != .5 {
		testCase.Fatal("unanimated site did not accept work")
	}
	world.StopUnits(1, []int{worker.ID})
	world.Advance(2)
	if site.Progress != .5 || !site.PendingWorker {
		testCase.Fatal("unanimated site continued constructing without a worker")
	}
}
