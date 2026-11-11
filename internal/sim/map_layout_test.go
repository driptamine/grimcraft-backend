package sim

import "testing"

func TestAlvarusLayout(testCase *testing.T) {
	world := NewWorld()
	world.SpawnLobbySquad("Blue", 1, "ig", "Alvarus")
	world.SpawnLobbySquad("Red", 2, "ig", "Alvarus")
	layout := layoutFor("/maps/6p_alvarus/6p_alvarus.sgb")
	if len(world.Points) != len(layout.Points) || len(layout.Points) != 24 {
		testCase.Fatalf("wrong points: %d", len(world.Points))
	}
	for _, point := range layout.Points {
		found := false
		for _, actual := range world.Points {
			if actual.Pos.X == point.X && actual.Pos.Z == point.Z && actual.Kind == point.Kind {
				found = true
			}
		}
		if !found {
			testCase.Fatalf("missing point: %+v", point)
		}
	}
	for _, building := range world.Buildings {
		if building.Owner == 1 && (building.Pos.X != 14.76 || building.Pos.Z != -102.98) {
			testCase.Fatalf("host spawn %+v", building.Pos)
		}
		if building.Owner == 2 && (building.Pos.X != -5.38 || building.Pos.Z != 114.06) {
			testCase.Fatalf("opponent spawn %+v", building.Pos)
		}
	}
}
