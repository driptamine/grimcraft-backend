// Package hub — command dispatch: applies one decoded Command object to
// the room world and answers with a CmdAck.
package hub

import (
	"encoding/json"
	"fmt"

	"grimcraft-go/internal/protocol"
	"grimcraft-go/internal/sim"
)

func toVec3(value any) (protocol.Vec3, bool) {
	fields, valid := value.(map[string]any)
	if !valid {
		return protocol.Vec3{}, false
	}
	coordOf := func(axis string) float64 {
		if number, valid := fields[axis].(float64); valid {
			return number
		}
		return 0
	}
	return protocol.Vec3{X: coordOf("x"), Y: coordOf("y"), Z: coordOf("z")}, true
}

func toIntList(value any) []int {
	items, valid := value.([]any)
	if !valid {
		return nil
	}
	result := make([]int, 0, len(items))
	for _, entry := range items {
		switch number := entry.(type) {
		case float64:
			result = append(result, int(number))
		case int:
			result = append(result, number)
		case int64:
			result = append(result, int(number))
		}
	}
	return result
}

func toInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	case int64:
		return int(number), true
	}
	return 0, false
}

func toFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	}
	return 0, false
}

func toString(value any) string {
	if text, isValid := value.(string); isValid {
		return text
	}
	return ""
}

// HandleCommand applies cmd (the inner {"MoveSquads": {...}} object) and
// returns (accepted, reason). Commands must mutate Go state or be rejected.
func HandleCommand(world *sim.World, playerID int, currentCommand map[string]any) (bool, string) {
	for commandName, commandArgs := range currentCommand {
		arguments, _ := commandArgs.(map[string]any)
		switch commandName {
		case "MoveSquads":
			target, valid := toVec3(arguments["target"])
			if !valid {
				return false, "bad target"
			}
			_, rejected := world.MoveSquads(playerID, toIntList(arguments["squad_ids"]), target)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected squads %v", rejected)
			}
			return true, ""
		case "MoveBuilders", "MoveApcs", "MoveJets":
			target, valid := toVec3(arguments["target"])
			if !valid {
				return false, "bad target"
			}
			var squadIDs []int
			for _, key := range []string{"squad_ids", "builder_ids", "apc_ids", "jet_ids", "ids"} {
				if matched := toIntList(arguments[key]); len(matched) > 0 {
					squadIDs = matched
					break
				}
			}
			_, rejected := world.MoveSquads(playerID, squadIDs, target)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected units %v", rejected)
			}
			return true, ""
		case "MoveUnits":
			target, valid := toVec3(arguments["target"])
			if !valid {
				return false, "bad target"
			}
			var pointID *int
			if componentData := arguments["point_id"]; componentData != nil {
				if capturePointID, validPointID := toInt(componentData); validPointID {
					pointID = &capturePointID
				}
			}
			_, rejected := world.MoveUnits(playerID, toIntList(arguments["ids"]), target, pointID)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected units %v", rejected)
			}
			return true, ""
		case "StopSquads":
			world.StopSquads(playerID, toIntList(arguments["squad_ids"]))
			return true, ""
		case "StopUnits":
			world.StopUnits(playerID, toIntList(arguments["ids"]))
			return true, ""
		case "AttackSquad":
			targetID, valid := toInt(arguments["target_squad"])
			if !valid {
				return false, "bad target_squad"
			}
			_, rejected := world.Attack(playerID, toIntList(arguments["attacker_ids"]), targetID)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected attackers %v", rejected)
			}
			return true, ""
		case "AttackSoldier":
			if targetID, valid := toInt(arguments["target_soldier"]); valid {
				_, rejected := world.Attack(playerID, toIntList(arguments["attacker_ids"]), targetID)
				if len(rejected) > 0 {
					return false, fmt.Sprintf("rejected attackers %v", rejected)
				}
				return true, ""
			}
			return true, ""
		case "UnitCommand":
			command := toString(arguments["command"])
			if command == "" {
				command = "move"
			}
			target, _ := toVec3(arguments["target"])
			var enemy *int
			if enemyMetadata, currentIsValid := arguments["enemy"].(map[string]any); currentIsValid {
				for _, fieldName := range []string{"squadId", "squad_id", "squad"} {
					if enemySquadID, validEnemyID := toInt(enemyMetadata[fieldName]); validEnemyID {
						enemy = &enemySquadID
						break
					}
				}
			}
			_, rejected := world.UnitCommand(playerID, toIntList(arguments["ids"]), command, target, enemy)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected units %v", rejected)
			}
			return true, ""
		case "SetStance":
			stance := toString(arguments["stance"])
			if stance == "" {
				stance = "ranged"
			}
			if !world.SetStance(playerID, toIntList(arguments["ids"]), stance) {
				return false, "no owned units"
			}
			return true, ""
		case "SetCombatStance":
			mode := toString(arguments["mode"])
			if mode == "" {
				mode = "ranged"
			}
			if !world.SetCombatStance(playerID, toIntList(arguments["ids"]), mode) {
				return false, "no owned units"
			}
			return true, ""
		case "AutoChase":
			enabled, _ := arguments["enabled"].(bool)
			if !world.SetAutoChase(playerID, toIntList(arguments["ids"]), enabled) {
				return false, "no owned squads"
			}
			return true, ""
		case "SetRally":
			buildingID, _ := toInt(arguments["building_id"])
			target, valid := toVec3(arguments["target"])
			if !valid {
				return false, "bad target"
			}
			if !world.SetRally(playerID, buildingID, target) {
				return false, "unknown building"
			}
			return true, ""
		case "SetProjectileType":
			squadID, _ := toInt(arguments["squad_id"])
			projectile := toString(arguments["proj_type"])
			if projectile == "" {
				projectile = toString(arguments["projectile"])
			}
			if !world.SetProjectileType(playerID, squadID, projectile) {
				return false, "unknown squad"
			}
			return true, ""
		case "AutoReinforce":
			squadID, _ := toInt(arguments["squad_id"])
			enabled, _ := arguments["enabled"].(bool)
			if !world.SetAutoReinforce(playerID, squadID, enabled) {
				return false, "unknown squad"
			}
			return true, ""
		case "Reinforce":
			squadID, _ := toInt(arguments["squad_id"])
			formationIndex, _ := toInt(arguments["form_idx"])
			if formationIndex == 0 {
				formationIndex, _ = toInt(arguments["formIdx"])
			}
			if !world.QueueReinforceAt(playerID, squadID, formationIndex) {
				return false, "reinforce rejected"
			}
			return true, ""
		case "GrenadeUpgrade":
			squadID, _ := toInt(arguments["squad_id"])
			if !world.GrenadeUpgrade(playerID, squadID) {
				return false, "grenade rejected"
			}
			return true, ""
		case "PlasmaUpgrade":
			squadID, _ := toInt(arguments["squad_id"])
			if !world.PlasmaUpgrade(playerID, squadID) {
				return false, "plasma rejected"
			}
			return true, ""
		case "CancelUpgrade":
			squadID, _ := toInt(arguments["squad_id"])
			if !world.CancelUpgrade(playerID, squadID) {
				return false, "nothing to cancel"
			}
			return true, ""
		case "PlaceBuilding":
			buildingType := toString(arguments["building_type"])
			if buildingType == "" {
				buildingType = toString(arguments["type"])
			}
			target, valid := toVec3(arguments["position"])
			if !valid || buildingType == "" {
				return false, "bad building"
			}
			builderID, _ := toInt(arguments["builder_id"])
			if _, buildingPlaced := world.PlaceBuilding(playerID, builderID, buildingType, target); !buildingPlaced {
				return false, "place rejected"
			}
			return true, ""
		case "TrainUnit":
			buildingID, _ := toInt(arguments["building_id"])
			unitType := toString(arguments["unit_type"])
			if unitType == "" {
				unitType = toString(arguments["type"])
			}
			if !world.TrainUnit(playerID, buildingID, unitType) {
				return false, "train rejected"
			}
			return true, ""
		case "Research":
			capturePointID, _ := toInt(arguments["entity_id"])
			key := toString(arguments["key"])
			if !world.QueueResearch(playerID, capturePointID, key) {
				return false, "research rejected"
			}
			return true, ""
		case "UpgradePoint":
			pointID, _ := toInt(arguments["point_id"])
			key := toString(arguments["key"])
			if key == "" {
				key = toString(arguments["name"])
			}
			if !world.UpgradePoint(playerID, pointID, key) {
				return false, "upgrade rejected"
			}
			return true, ""
		case "DemolishBuilding":
			buildingID, _ := toInt(arguments["building_id"])
			if buildingID == 0 {
				buildingID, _ = toInt(arguments["id"])
			}
			if !world.Demolish(playerID, buildingID) {
				return false, "demolish rejected"
			}
			return true, ""
		case "SpendRequisition":
			amount, _ := toFloat(arguments["amount"])
			if !world.SpendRequisition(playerID, amount) {
				return false, "spend rejected"
			}
			return true, ""
		case "BuildAddon":
			buildingID, _ := toInt(arguments["building_id"])
			addon := toString(arguments["addon"])
			if !world.BuildAddon(playerID, buildingID, addon) {
				return false, "addon rejected"
			}
			return true, ""
		case "ResumeBuild":
			builderID, _ := toInt(arguments["builder_id"])
			targetID, _ := toInt(arguments["target_id"])
			if !world.ResumeBuild(playerID, builderID, targetID) {
				return false, "resume rejected"
			}
			return true, ""
		case "ResumeUnits":
			world.StopUnits(playerID, toIntList(arguments["ids"]))
			return true, ""
		case "BuilderStance":
			builderID, _ := toInt(arguments["builder_id"])
			stance := toString(arguments["stance"])
			if !world.BuilderStance(playerID, builderID, stance) {
				return false, "unknown builder"
			}
			return true, ""
		case "BuilderCommand":
			builderID, _ := toInt(arguments["builder_id"])
			target, _ := toVec3(arguments["target"])
			var repair *int
			if hit, hasHitMetadata := arguments["hit"].(map[string]any); hasHitMetadata {
				for _, entityIDField := range []string{"buildingEntityId", "structureEntityId", "airfieldEntityId"} {
					if repairTargetID, validRepairTargetID := toInt(hit[entityIDField]); validRepairTargetID {
						repair = &repairTargetID
						break
					}
				}
			}
			if !world.BuilderCommand(playerID, builderID, target, repair) {
				return false, "unknown builder"
			}
			return true, ""
		case "BoardTransport":
			apcID, _ := toInt(arguments["apc_id"])
			if !world.BoardTransport(playerID, apcID, toIntList(arguments["ids"])) {
				return false, "board rejected"
			}
			return true, ""
		case "FlySquads":
			target, valid := toVec3(arguments["target"])
			if !valid {
				return false, "bad target"
			}
			_, rejected := world.FlySquads(playerID, toIntList(arguments["ids"]), target)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected units %v", rejected)
			}
			return true, ""
		default:
			return false, "unknown command " + commandName
		}
	}
	return false, "empty command"
}

// DecodeClientMessage parses one JSON client message (framing stripped).
func DecodeClientMessage(encodedData []byte) (protocol.ClientMessage, error) {
	var message protocol.ClientMessage
	decoder := jsonUnmarshaler{}
	if operationError := decoder.unmarshal(encodedData, &message); operationError != nil {
		return message, operationError
	}
	return message, nil
}

type jsonUnmarshaler struct{}

func (jsonUnmarshaler) unmarshal(encodedData []byte, value any) error {
	return json.Unmarshal(encodedData, value)
}
