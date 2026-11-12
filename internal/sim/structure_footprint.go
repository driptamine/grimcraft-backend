package sim

import (
	_ "embed"
	"encoding/json"
	"math"

	"grimcraft-go/internal/protocol"
)

//go:embed data/structure-footprints.json
var structureFootprintJSON []byte

type footprintBounds struct {
	MinX float64 `json:"minX"`
	MaxX float64 `json:"maxX"`
	MinZ float64 `json:"minZ"`
	MaxZ float64 `json:"maxZ"`
}

type footprintProfile struct {
	Construction footprintBounds `json:"construction"`
	Completed    footprintBounds `json:"completed"`
}

var structureFootprints = func() map[string]footprintProfile {
	profiles := map[string]footprintProfile{}
	if err := json.Unmarshal(structureFootprintJSON, &profiles); err != nil {
		panic(err)
	}
	return profiles
}()

const constructionClearance = 1.35
const constructionReach = 2.2
const workerCollisionRadius = .5

func structureFootprint(faction, buildingType string, constructing bool, position protocol.Vec3) footprintBounds {
	if buildingType == "hq-anim" {
		buildingType = "hq"
	} else if buildingType == "barracks" {
		buildingType = "infantry_command"
	}
	var bounds footprintBounds
	if profile, found := structureFootprints[buildingType]; found && faction == "ig" {
		bounds = profile.Completed
		if constructing {
			bounds = profile.Construction
		}
	} else {
		halfSize := 3.5
		switch buildingType {
		case "hq":
			halfSize = 5
		case "mechanized", "tower", "turret", "gunTurret", "mines":
			halfSize = 2.5
		case "towerBase":
			halfSize = 2.25
		case "airfield":
			halfSize = 8
		}
		bounds = footprintBounds{-halfSize, halfSize, -halfSize, halfSize}
	}
	bounds.MinX += position.X
	bounds.MaxX += position.X
	bounds.MinZ += position.Z
	bounds.MaxZ += position.Z
	return bounds
}

func insideFootprint(position protocol.Vec3, bounds footprintBounds, padding float64) bool {
	return position.X > bounds.MinX-padding && position.X < bounds.MaxX+padding && position.Z > bounds.MinZ-padding && position.Z < bounds.MaxZ+padding
}

func canWorkAtStructure(position protocol.Vec3, bounds footprintBounds) bool {
	if insideFootprint(position, bounds, workerCollisionRadius+.15) {
		return false
	}
	distanceX := math.Max(math.Max(bounds.MinX-position.X, position.X-bounds.MaxX), 0)
	distanceZ := math.Max(math.Max(bounds.MinZ-position.Z, position.Z-bounds.MaxZ), 0)
	return math.Hypot(distanceX, distanceZ) <= constructionReach
}

func constructionWorkPosition(from, position protocol.Vec3, faction, buildingType string) protocol.Vec3 {
	bounds := structureFootprint(faction, buildingType, true, position)
	front := protocol.Vec3{X: math.Max(bounds.MinX+.6, math.Min(bounds.MaxX-.6, from.X)), Y: position.Y, Z: bounds.MinZ - constructionClearance}
	right := protocol.Vec3{X: bounds.MaxX + constructionClearance, Y: position.Y, Z: math.Max(bounds.MinZ+.6, math.Min(bounds.MaxZ-.6, from.Z))}
	if dist(from, front) <= dist(from, right) {
		return front
	}
	return right
}

func outsideFootprint(position protocol.Vec3, bounds footprintBounds, padding float64) protocol.Vec3 {
	if !insideFootprint(position, bounds, padding) {
		return position
	}
	candidates := []protocol.Vec3{
		{X: position.X, Y: position.Y, Z: bounds.MinZ - padding}, {X: bounds.MaxX + padding, Y: position.Y, Z: position.Z},
		{X: bounds.MinX - padding, Y: position.Y, Z: position.Z}, {X: position.X, Y: position.Y, Z: bounds.MaxZ + padding},
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if dist(position, candidate) < dist(position, best) {
			best = candidate
		}
	}
	return best
}

func segmentEntersFootprint(from, to protocol.Vec3, bounds footprintBounds) bool {
	entry, exit := 0.0, 1.0
	for _, axis := range [][4]float64{{from.X, to.X, bounds.MinX + 1e-5, bounds.MaxX - 1e-5}, {from.Z, to.Z, bounds.MinZ + 1e-5, bounds.MaxZ - 1e-5}} {
		start, end, minimum, maximum := axis[0], axis[1], axis[2], axis[3]
		delta := end - start
		if math.Abs(delta) < 1e-9 {
			if start < minimum || start > maximum {
				return false
			}
		} else {
			first, second := (minimum-start)/delta, (maximum-start)/delta
			entry = math.Max(entry, math.Min(first, second))
			exit = math.Min(exit, math.Max(first, second))
			if entry > exit {
				return false
			}
		}
	}
	return entry <= exit
}

func constructionApproachWaypoint(from, goal protocol.Vec3, footprint footprintBounds) protocol.Vec3 {
	padding := workerCollisionRadius + .35
	bounds := footprintBounds{footprint.MinX - padding, footprint.MaxX + padding, footprint.MinZ - padding, footprint.MaxZ + padding}
	if insideFootprint(from, bounds, 0) {
		return outsideFootprint(from, bounds, .01)
	}
	if !segmentEntersFootprint(from, goal, bounds) {
		return goal
	}
	points := []protocol.Vec3{from, goal, {X: bounds.MinX, Z: bounds.MinZ}, {X: bounds.MaxX, Z: bounds.MinZ}, {X: bounds.MinX, Z: bounds.MaxZ}, {X: bounds.MaxX, Z: bounds.MaxZ}}
	distances, previous, visited := make([]float64, len(points)), make([]int, len(points)), make([]bool, len(points))
	for index := range points {
		distances[index], previous[index] = math.Inf(1), -1
	}
	distances[0] = 0
	for range points {
		current := -1
		for index := range points {
			if !visited[index] && (current < 0 || distances[index] < distances[current]) {
				current = index
			}
		}
		if current < 0 || math.IsInf(distances[current], 1) || current == 1 {
			break
		}
		visited[current] = true
		for next := range points {
			if next == current || visited[next] || segmentEntersFootprint(points[current], points[next], bounds) {
				continue
			}
			distance := distances[current] + dist(points[current], points[next])
			if distance < distances[next] {
				distances[next], previous[next] = distance, current
			}
		}
	}
	next := 1
	if previous[next] < 0 {
		return goal
	}
	for previous[next] > 0 {
		next = previous[next]
	}
	return points[next]
}

func (world *World) resolveBuilderFootprintsLocked(builder *Builder) {
	for _, building := range world.Buildings {
		if building.HP > 0 {
			builder.Pos = outsideFootprint(builder.Pos, structureFootprint(building.Faction, building.Type, building.Constructing, building.Pos), workerCollisionRadius+.02)
		}
	}
	for _, structure := range world.Structures {
		if structure.HP > 0 {
			builder.Pos = outsideFootprint(builder.Pos, structureFootprint(structure.Faction, structure.Type, structure.Constructing, structure.Pos), workerCollisionRadius+.02)
		}
	}
}
