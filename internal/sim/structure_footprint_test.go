package sim

import (
	"fmt"
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestConstructionWorkersApproachOutsideEachDrop(testCase *testing.T) {
	for _, buildingType := range []string{"hq", "infantry_command", "mechanized", "mars_pattern", "listening_post", "plasma_generator", "tactica_control", "turret"} {
		for _, start := range []protocol.Vec3{{X: -25, Z: -25}, {X: -25, Z: 25}, {X: 25, Z: -25}, {X: 25, Z: 25}} {
			testCase.Run(fmt.Sprintf("%s/%g/%g", buildingType, start.X, start.Z), func(testCase *testing.T) {
				world := NewWorld()
				world.EnsurePlayer(1, "Blue", "ig")
				pointID := world.allocID()
				world.Points[pointID] = &Point{ID: pointID, Owner: "Blue", Kind: "point"}
				worker := world.SpawnBuilder("Blue", 1, "ig", start.X, start.Z)
				entityID, accepted := world.PlaceBuilding(1, worker.ID, buildingType, protocol.Vec3{})
				if !accepted {
					testCase.Fatal("placement rejected")
				}
				isConstructing := func() bool {
					if building := world.Buildings[entityID]; building != nil {
						return building.Constructing
					}
					return world.Structures[entityID].Constructing
				}
				bounds := structureFootprint("ig", buildingType, true, protocol.Vec3{})
				if worker.BuildWorkPos == nil || !canWorkAtStructure(*worker.BuildWorkPos, bounds) {
					testCase.Fatal("work position must be outside the model")
				}
				builtOutside := false
				for tick := 0; tick < 1000 && isConstructing(); tick++ {
					previous := worker.Pos
					world.Advance(.05)
					if insideFootprint(worker.Pos, bounds, .49) || segmentEntersFootprint(previous, worker.Pos, bounds) {
						testCase.Fatalf("worker entered its construction drop: %+v", worker.Pos)
					}
					if worker.IsBuilding {
						builtOutside = true
						if !canWorkAtStructure(worker.Pos, bounds) {
							testCase.Fatal("worker built from inside the model")
						}
					}
				}
				if isConstructing() || !builtOutside {
					testCase.Fatalf("worker failed to approach and finish: %+v", worker)
				}
			})
		}
	}
}
