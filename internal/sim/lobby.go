package sim

import (
	"grimcraft-go/internal/protocol"
	"strings"
)

func canonicalFaction(faction string) string {
	switch strings.ToLower(faction) {
	case "space marines":
		return "space marines"
	case "imperial guard", "ig", "":
		return "ig"
	case "necron", "necrons":
		return "necrons"
	case "chaos", "eldar", "dark eldar", "orks", "tau":
		return strings.ToLower(faction)
	default:
		return "ig"
	}
}

func squadMovementSpeed(faction string) float64 {
	if canonicalFaction(faction) == "necrons" {
		return DefaultSquadSpeed * .5
	}
	return DefaultSquadSpeed
}

func baseFor(team, mapName string, slot int) (float64, float64) {
	layout := layoutFor(mapName)
	positions := []mapPosition{}
	for _, startPosition := range layout.Positions {
		if strings.EqualFold(startPosition.Team, team) {
			positions = append(positions, startPosition)
		}
	}
	if len(positions) > 0 {
		if strings.Contains(strings.ToLower(mapName), "alvarus") && len(positions) == 3 {
			positions = []mapPosition{positions[1], positions[0], positions[2]}
		}
		currentStartPosition := positions[slot%len(positions)]
		return currentStartPosition.X, currentStartPosition.Z
	}

	if team == "Red" {
		return RedBaseX - float64(slot%3)*12, float64((slot%5)*6 - 12)
	}
	return BlueBaseX + float64(slot%3)*12, float64((slot%5)*6 - 12)
}

// SpawnLobbySquad spawns a full skirmish seat: HQ + builder + two squads.
func (world *World) SpawnLobbySquad(team string, owner int, faction, mapName string) *Squad {
	if team != "Red" {
		team = "Blue"
	}
	faction = canonicalFaction(faction)
	world.EnsurePlayer(owner, team, faction)
	world.EnsureMapPoints(mapName)
	count := 0
	world.mutex.Lock()
	for _, building := range world.Buildings {
		if building.Team == team && building.Type == "hq" {
			count++
		}
	}
	world.mutex.Unlock()
	basePositionX, basePositionZ := baseFor(team, mapName, count)
	// HQ
	_, _ = world.PlaceBuildingFree(owner, team, faction, "hq", protocol.Vec3{X: basePositionX, Z: basePositionZ})
	// builder
	world.SpawnBuilder(team, owner, faction, basePositionX+10, basePositionZ)
	// squads
	firstSquad := world.SpawnSquad(team, owner)
	firstSquad.Faction = faction
	secondSquad := world.SpawnSquad(team, owner)
	secondSquad.Faction = faction
	world.mutex.Lock()
	firstSquad.Pos = protocol.Vec3{X: basePositionX + 16, Z: basePositionZ - 6}
	secondSquad.Pos = protocol.Vec3{X: basePositionX + 16, Z: basePositionZ + 6}
	if team == "Red" {
		firstSquad.Pos.X, secondSquad.Pos.X = basePositionX-16, basePositionX-16
		firstSquad.Facing = protocol.Vec3{X: -1}
		secondSquad.Facing = protocol.Vec3{X: -1}
	} else {
		firstSquad.Facing = protocol.Vec3{X: 1}
		secondSquad.Facing = protocol.Vec3{X: 1}
	}
	world.mutex.Unlock()
	return firstSquad
}

// PlaceBuildingFree creates a building without cost (spawn/HQ).
func (world *World) PlaceBuildingFree(owner int, team, faction, buildingType string, position protocol.Vec3) (int, bool) {
	world.mutex.Lock()
	defer world.mutex.Unlock()
	world.ensurePlayerLocked(owner, team, faction)
	entityID := world.allocID()
	healthPoints := 400.0
	if buildingType == "hq" {
		healthPoints = 2000
	}
	world.Buildings[entityID] = &Building{ID: entityID, Team: team, Faction: faction, Owner: owner,
		Type: buildingType, Pos: position, HP: healthPoints, MaxHP: healthPoints, Constructing: false,
		Rally: &protocol.Vec3{X: position.X, Z: position.Z + 10}, Tier: 1}
	return entityID, true
}

func (world *World) SetFaction(owner int, faction string) {
	faction = canonicalFaction(faction)
	world.mutex.Lock()
	defer world.mutex.Unlock()
	if player, isValid := world.Players[owner]; isValid {
		player.Faction = faction
	}
	for _, squad := range world.Squads {
		if squad.Owner == owner {
			squad.Faction = faction
			squad.Speed = squadMovementSpeed(faction)
		}
	}
	for _, builder := range world.Builders {
		if builder.Owner == owner {
			builder.Faction = faction
		}
	}
	for _, building := range world.Buildings {
		if building.Owner == owner {
			building.Faction = faction
		}
	}
	for _, structure := range world.Structures {
		if structure.Owner == owner {
			structure.Faction = faction
		}
	}
}
func (world *World) AddComputer(team string, owner int, faction, mapName string) {
	squad := world.SpawnLobbySquad(team, owner, faction, mapName)
	world.mutex.Lock()
	defer world.mutex.Unlock()
	squad.Computer = true
}
