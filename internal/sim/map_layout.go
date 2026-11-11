package sim

import (
	"embed"
	"encoding/json"
	"grimcraft-go/internal/protocol"
	"path"
	"strings"
	"unicode"
)

// Map layouts are data shared with the client, embedded in the Go binary.
//
//go:embed maps/*.json
var mapFiles embed.FS

type mapPosition struct {
	X    float64 `json:"x"`
	Z    float64 `json:"z"`
	Team string  `json:"team"`
}
type mapPoint struct {
	X    float64 `json:"x"`
	Z    float64 `json:"z"`
	Kind string  `json:"kind"`
}
type mapLayout struct {
	Name          string        `json:"name"`
	Positions     []mapPosition `json:"positions"`
	Points        []mapPoint    `json:"strategicPoints"`
	CenterPoints  [][2]float64  `json:"centerPoints"`
	Ring          bool          `json:"ringPerPosition"`
	Offsets       [][2]float64  `json:"ringOffsets"`
	RelicDistance float64       `json:"relicDistance"`
}

func mapNameKey(name string) string {
	name = strings.TrimSuffix(path.Base(name), ".sgb")
	if len(name) > 2 && name[0] >= '0' && name[0] <= '9' && name[1] == 'p' {
		name = name[2:]
	}
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return unicode.ToLower(character)
		}
		return -1
	}, name)
}
func layoutFor(name string) mapLayout {
	key := mapNameKey(name)
	if key == "arena" {
		key = "defaultarena"
	}
	entries, _ := mapFiles.ReadDir("maps")
	var fallback mapLayout
	for _, mapFile := range entries {
		data, _ := mapFiles.ReadFile("maps/" + mapFile.Name())
		var layout mapLayout
		if json.Unmarshal(data, &layout) != nil {
			continue
		}
		if mapNameKey(layout.Name) == "defaultarena" {
			fallback = layout
		}
		if mapNameKey(layout.Name) == key {
			return layout
		}
	}
	return fallback
}
func (world *World) seedMapPointsLocked(layout mapLayout) {
	if world.pointsSeeded {
		return
	}
	world.pointsSeeded = true
	points := layout.Points
	if len(points) == 0 {
		for _, pointCoordinates := range layout.CenterPoints {
			points = append(points, mapPoint{X: pointCoordinates[0], Z: pointCoordinates[1], Kind: "point"})
		}
		if layout.Ring {
			for _, base := range layout.Positions {
				for _, offset := range layout.Offsets {
					points = append(points, mapPoint{X: base.X + offset[0], Z: base.Z + offset[1], Kind: "point"})
				}
				if layout.RelicDistance > 0 {
					direction := 1.
					if base.Z >= 0 {
						direction = -1
					}
					points = append(points, mapPoint{X: base.X, Z: base.Z + direction*layout.RelicDistance, Kind: "relic"})
				}
			}
		}
	}
	for _, point := range points {
		kind := point.Kind
		if kind != "relic" && kind != "objective" {
			kind = "point"
		}
		entityID := world.allocID()
		world.Points[entityID] = &Point{ID: entityID, Pos: protocol.Vec3{X: point.X, Z: point.Z}, Kind: kind}
	}
}
