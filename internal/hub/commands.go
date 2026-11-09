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
	out := make([]int, 0, len(items))
	for _, entry := range items {
		switch number := entry.(type) {
		case float64:
			out = append(out, int(number))
		case int:
			out = append(out, number)
		}
	}
	return out
}

func toInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	}
	return 0, false
}

// HandleCommand applies cmd (the inner {"MoveSquads": {...}} object) and
// returns (accepted, reason). Unknown/echo-only commands ack accepted so the
// client keeps running; simulation-relevant ones mutate the world.
func HandleCommand(world *sim.World, playerID int, cmd map[string]any) (bool, string) {
	for commandName, commandArgs := range cmd {
		args, _ := commandArgs.(map[string]any)
		switch commandName {
		case "MoveSquads":
			target, valid := toVec3(args["target"])
			if !valid {
				return false, "bad target"
			}
			_, rejected := world.MoveSquads(playerID, toIntList(args["squad_ids"]), target)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected squads %v", rejected)
			}
			return true, ""
		case "MoveBuilders", "MoveApcs", "MoveJets":
			// Movement-class units share the squad sim for now.
			target, valid := toVec3(args["target"])
			if !valid {
				return false, "bad target"
			}
			var squadIDs []int
			for _, key := range []string{"squad_ids", "builder_ids", "apc_ids", "jet_ids"} {
				if matched := toIntList(args[key]); len(matched) > 0 {
					squadIDs = matched
					break
				}
			}
			_, rejected := world.MoveSquads(playerID, squadIDs, target)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected units %v", rejected)
			}
			return true, ""
		case "StopSquads":
			world.StopSquads(playerID, toIntList(args["squad_ids"]))
			return true, ""
		case "AttackSquad":
			targetID, valid := toInt(args["target_squad"])
			if !valid {
				return false, "bad target_squad"
			}
			_, rejected := world.Attack(playerID, toIntList(args["attacker_ids"]), targetID)
			if len(rejected) > 0 {
				return false, fmt.Sprintf("rejected attackers %v", rejected)
			}
			return true, ""
		case "AttackSoldier":
			// Soldier-level targeting maps onto its squad when known.
			if targetID, valid := toInt(args["target_soldier"]); valid {
				_, rejected := world.Attack(playerID, toIntList(args["attacker_ids"]), targetID)
				if len(rejected) > 0 {
					return false, fmt.Sprintf("rejected attackers %v", rejected)
				}
				return true, ""
			}
			return true, ""
		case "SetProjectileType":
			squadID, _ := toInt(args["squad_id"])
			projectile, _ := args["proj_type"].(string)
			if !world.SetProjectileType(playerID, squadID, projectile) {
				return false, "unknown squad"
			}
			return true, ""
		case "AutoReinforce":
			squadID, _ := toInt(args["squad_id"])
			enabled, _ := args["enabled"].(bool)
			if !world.SetAutoReinforce(playerID, squadID, enabled) {
				return false, "unknown squad"
			}
			return true, ""
		case "Reinforce":
			squadID, _ := toInt(args["squad_id"])
			if !world.QueueReinforce(playerID, squadID) {
				return false, "reinforce rejected"
			}
			return true, ""
		case "PlaceBuilding", "TrainUnit", "GrenadeUpgrade",
			"DemolishBuilding", "SpendRequisition":
			// Economy/building sim lives client-side in this revision;
			// ack so the client treats the order as accepted.
			return true, ""
		default:
			return false, "unknown command " + commandName
		}
	}
	return false, "empty command"
}

// DecodeClientMessage parses one JSON client message (framing stripped).
func DecodeClientMessage(raw []byte) (protocol.ClientMessage, error) {
	var message protocol.ClientMessage
	decoder := jsonUnmarshaler{}
	if err := decoder.unmarshal(raw, &message); err != nil {
		return message, err
	}
	return message, nil
}

type jsonUnmarshaler struct{}

func (jsonUnmarshaler) unmarshal(raw []byte, value any) error {
	return json.Unmarshal(raw, value)
}
