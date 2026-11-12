package hub

import (
	"encoding/json"
	"testing"
)

func mustCommand(testCase *testing.T, commandJSON string) map[string]any {
	testCase.Helper()
	var command map[string]any
	if operationError := json.Unmarshal([]byte(commandJSON), &command); operationError != nil {
		testCase.Fatal(operationError)
	}
	return command
}

func ownedSquad(testCase *testing.T, gameHub *Hub, lobbyID string, player int) int {
	testCase.Helper()
	for entityID, squad := range gameHub.RoomOf(lobbyID).World().Squads {
		if squad.Owner == player {
			return entityID
		}
	}
	testCase.Fatal("no squad")
	return 0
}

func TestSkirmishCommandParity(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("parity", "Arena", 4)
	client, _, operationError := gameHub.Join(lobby.ID, nil, "p1", "ig")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	world := gameHub.RoomOf(lobby.ID).World()
	squadID := ownedSquad(testCase, gameHub, lobby.ID, client.PlayerID)

	// MoveUnits (schema 2) with capture point
	var pointID int
	for entityID := range world.Points {
		pointID = entityID
		break
	}
	encodedData, _ := json.Marshal(map[string]any{"MoveUnits": map[string]any{
		"ids": []any{float64(squadID)}, "target": map[string]any{"x": 1.0, "y": 0.0, "z": 2.0}, "point_id": float64(pointID)}})
	var command map[string]any
	_ = json.Unmarshal(encodedData, &command)
	if isValid, reason := HandleCommand(world, client.PlayerID, command); !isValid {
		testCase.Fatalf("MoveUnits rejected: %s", reason)
	}

	// PlaceBuilding + TrainUnit + Research + upgrades
	var builderID int
	for ownedBuilderID, builder := range world.Builders {
		if builder.Owner == client.PlayerID {
			builderID = ownedBuilderID
			break
		}
	}
	if builderID == 0 {
		builder := world.SpawnBuilder(world.Players[client.PlayerID].Team, client.PlayerID, "ig", -25, 5)
		builderID = builder.ID
	}
	buildPosition := world.Builders[builderID].Pos
	placementJSON, _ := json.Marshal(map[string]any{"PlaceBuilding": map[string]any{
		"builder_id": builderID, "building_type": "barracks", "position": map[string]any{"x": buildPosition.X + 8, "y": 0, "z": buildPosition.Z}}})
	if buildingPlaced, placementFailureReason := HandleCommand(world, client.PlayerID, mustCommand(testCase, string(placementJSON))); !buildingPlaced {
		testCase.Fatalf("PlaceBuilding rejected: %s", placementFailureReason)
	}
	var barracks int
	for buildingID, building := range world.Buildings {
		if building.Owner == client.PlayerID && building.Type == "barracks" {
			barracks = buildingID
			break
		}
	}
	if barracks == 0 {
		testCase.Fatal("no barracks after PlaceBuilding")
	}
	// The assigned worker approaches and finishes construction through ticks.
	for index := 0; index < 200 && world.Buildings[barracks].Constructing; index++ {
		world.Advance(0.1)
	}
	if world.Buildings[barracks].Constructing {
		testCase.Fatal("the assigned worker did not finish construction")
	}
	if invalidProductionAccepted, invalidProductionReason := HandleCommand(world, client.PlayerID, mustCommand(testCase, `{"TrainUnit":{"building_id":0,"unit_type":"squad"}}`)); invalidProductionAccepted {
		testCase.Fatalf("TrainUnit with bad id must reject, got ok")
		_ = invalidProductionReason
	}
	trainCommandJSON, _ := json.Marshal(map[string]any{"TrainUnit": map[string]any{"building_id": float64(barracks), "unit_type": "squad"}})
	_ = json.Unmarshal(trainCommandJSON, &command)
	if productionAccepted, productionFailureReason := HandleCommand(world, client.PlayerID, command); !productionAccepted {
		testCase.Fatalf("TrainUnit rejected: %s", productionFailureReason)
	}
	if invalidStanceAccepted, invalidStanceReason := HandleCommand(world, client.PlayerID, mustCommand(testCase, `{"SetCombatStance":{"ids":[0],"mode":"melee"}}`)); invalidStanceAccepted {
		testCase.Fatalf("SetCombatStance with bad id must reject: %s", invalidStanceReason)
	}
	stanceCommandJSON, _ := json.Marshal(map[string]any{"SetCombatStance": map[string]any{"ids": []any{float64(squadID)}, "mode": "melee"}})
	_ = json.Unmarshal(stanceCommandJSON, &command)
	if stanceAccepted, stanceFailureReason := HandleCommand(world, client.PlayerID, command); !stanceAccepted {
		testCase.Fatalf("SetCombatStance rejected: %s", stanceFailureReason)
	}
	// Grenade + Plasma + Research + Demolish paths
	world.MuLockTestKillSoldier(squadID)
	grenadeCommandJSON, _ := json.Marshal(map[string]any{"GrenadeUpgrade": map[string]any{"squad_id": float64(squadID)}})
	_ = json.Unmarshal(grenadeCommandJSON, &command)
	if grenadeUpgradeAccepted, grenadeUpgradeFailureReason := HandleCommand(world, client.PlayerID, command); !grenadeUpgradeAccepted {
		testCase.Fatalf("GrenadeUpgrade rejected: %s", grenadeUpgradeFailureReason)
	}
	plasmaCommandJSON, _ := json.Marshal(map[string]any{"PlasmaUpgrade": map[string]any{"squad_id": float64(squadID)}})
	_ = json.Unmarshal(plasmaCommandJSON, &command)
	if plasmaUpgradeAccepted, plasmaUpgradeFailureReason := HandleCommand(world, client.PlayerID, command); !plasmaUpgradeAccepted {
		testCase.Fatalf("PlasmaUpgrade rejected: %s", plasmaUpgradeFailureReason)
	}
	researchCommandJSON, _ := json.Marshal(map[string]any{"Research": map[string]any{"entity_id": float64(barracks), "key": "rk"}})
	_ = json.Unmarshal(researchCommandJSON, &command)
	if researchAccepted, researchFailureReason := HandleCommand(world, client.PlayerID, command); !researchAccepted {
		testCase.Fatalf("Research rejected: %s", researchFailureReason)
	}
	// Unknown command still rejected
	if unknownCommandAccepted, _ := HandleCommand(world, client.PlayerID, mustCommand(testCase, `{"Nope":{}}`)); unknownCommandAccepted {
		testCase.Fatal("unknown command must reject")
	}
}
