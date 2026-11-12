package sim

import (
	"encoding/json"
	"testing"

	"grimcraft-go/internal/protocol"
)

func TestIGVehicleProduction(testCase *testing.T) {
	for _, buildingType := range []string{"mechanized", "mars_pattern"} {
		testCase.Run(buildingType, func(testCase *testing.T) {
			world := NewWorld()
			world.EnsurePlayer(1, "Blue", "ig")
			world.Players[1].Requisition = 1000
			world.Players[1].Research["full_scale_war"] = true
			worker := world.SpawnBuilder("Blue", 1, "ig", -40, 0)
			buildingID, accepted := world.PlaceBuilding(1, worker.ID, buildingType, protocol.Vec3{})
			if !accepted || world.Buildings[buildingID] == nil {
				testCase.Fatal("vehicle factory must be a building with a production queue")
			}
			building := world.Buildings[buildingID]
			building.Constructing = false
			building.PendingWorker = false
			unitType := "chimera"
			if buildingType == "mars_pattern" {
				unitType = "baneblade"
			}
			if !world.TrainUnit(1, buildingID, unitType) {
				testCase.Fatal("vehicle order rejected")
			}
			encoded, err := json.Marshal(building.Queue)
			if err != nil {
				testCase.Fatal(err)
			}
			var orders []map[string]any
			if err := json.Unmarshal(encoded, &orders); err != nil {
				testCase.Fatal(err)
			}
			if orders[0]["type"] != unitType || orders[0]["progress"] != 0.0 || orders[0]["total"] == nil {
				testCase.Fatalf("client needs lowercase production progress: %s", encoded)
			}
			world.Advance(building.Queue[0].Total)
			if len(building.Queue) != 0 || len(world.Vehicles) != 1 {
				testCase.Fatal("vehicle order failed to complete")
			}
			for _, vehicle := range world.Vehicles {
				if vehicle.Name != unitType || vehicle.Kind != "vehicle" || vehicle.Model != "models/ig/vehicles/"+unitType+".glb" {
					testCase.Fatalf("wrong produced vehicle: %+v", vehicle)
				}
				record := actorFrom(world, vehicle.ID)
				if record["model"] != vehicle.Model || record["name"] != unitType {
					testCase.Fatalf("snapshot lost the vehicle model: %+v", record)
				}
			}
		})
	}
}

func TestMarsPatternRequiresVehicleResearch(testCase *testing.T) {
	world := NewWorld()
	world.EnsurePlayer(1, "Blue", "ig")
	world.Players[1].Requisition = 1000
	world.Buildings[100] = &Building{ID: 100, Owner: 1, Team: "Blue", Faction: "ig", Type: "mars_pattern"}
	if world.TrainUnit(1, 100, "baneblade") || world.TrainUnit(1, 100, "chimera") || world.Players[1].Requisition != 1000 {
		testCase.Fatal("invalid or locked orders must not charge resources")
	}
}
