package sim

import "grimcraft-go/internal/protocol"

const constructionDeliveryTime = 1.0

func hasAnimatedConstruction(faction, buildingType string) bool {
	if faction != "ig" {
		return false
	}
	switch buildingType {
	case "hq", "hq-anim", "infantry_command", "barracks", "listening_post", "mechanized", "mars_pattern", "plasma_generator", "tactica_control":
		return true
	}
	return false
}

func clearBuilderBuildOrder(builder *Builder) {
	builder.IsBuilding = false
	builder.BuildProg = 0
	builder.BuildTarget = nil
	builder.BuildType = ""
	builder.BuildPos = nil
	builder.BuildWorkPos = nil
	builder.BuildSP = nil
}

func assignConstructionBuilder(builder *Builder, position protocol.Vec3, faction, buildingType string) {
	destination := constructionWorkPosition(builder.Pos, position, faction, buildingType)
	builder.MoveTarget = &destination
	builder.BuildWorkPos = &destination
	builder.RepairTarget = nil
	builder.BuildSP = nil
	builder.IsBuilding = false
	builder.BuildProg = 0
}

func advanceBuilderAtSite(builder *Builder, position protocol.Vec3, faction, buildingType string, constructing bool, pendingWorker *bool, delivered bool, deltaTime float64) bool {
	if !constructing {
		clearBuilderBuildOrder(builder)
		return true
	}
	if builder.BuildWorkPos == nil {
		workPosition := constructionWorkPosition(builder.Pos, position, faction, buildingType)
		builder.BuildWorkPos = &workPosition
	}
	if !canWorkAtStructure(builder.Pos, structureFootprint(faction, buildingType, true, position)) || (!builder.IsBuilding && dist(builder.Pos, *builder.BuildWorkPos) >= .4) {
		builder.IsBuilding = false
		builder.MoveTarget = builder.BuildWorkPos
		return false
	}
	builder.MoveTarget = nil
	*pendingWorker = false
	builder.IsBuilding = delivered
	if delivered {
		builder.BuildProg += deltaTime
	}
	return true
}

// Delivery advances at placement; assembly starts when the worker reaches the
// site. Keep the build order alive until the building itself is complete.
func (world *World) advanceConstructionBuilderLocked(builder *Builder, deltaTime float64) bool {
	if builder.BuildTarget == nil {
		return false
	}
	building := world.Buildings[*builder.BuildTarget]
	if building != nil && (building.RequiresWorker || hasAnimatedConstruction(building.Faction, building.Type)) {
		return advanceBuilderAtSite(builder, building.Pos, building.Faction, building.Type, building.Constructing, &building.PendingWorker, building.DeliveryProgress >= building.DeliveryTime, deltaTime)
	}
	structure := world.Structures[*builder.BuildTarget]
	if structure != nil && (structure.RequiresWorker || hasAnimatedConstruction(structure.Faction, structure.Type)) {
		return advanceBuilderAtSite(builder, structure.Pos, structure.Faction, structure.Type, structure.Constructing, &structure.PendingWorker, structure.DeliveryProgress >= structure.DeliveryTime, deltaTime)
	}
	return false
}

func (world *World) finishConstructionBuildersLocked(entityID int) {
	for _, builder := range world.Builders {
		if builder.BuildTarget != nil && *builder.BuildTarget == entityID {
			clearBuilderBuildOrder(builder)
		}
	}
}

func (world *World) hasActiveConstructionWorkerLocked(entityID, owner int, position protocol.Vec3, faction, buildingType string) bool {
	for _, worker := range world.Builders {
		if worker.Owner == owner && worker.HP > 0 && worker.IsBuilding && worker.MoveTarget == nil && worker.RepairTarget == nil &&
			worker.BuildTarget != nil && *worker.BuildTarget == entityID && canWorkAtStructure(worker.Pos, structureFootprint(faction, buildingType, true, position)) {
			return true
		}
	}
	return false
}

func (world *World) removeListeningPostLinkLocked(building *Building) {
	if building.StrategicPointID == nil {
		return
	}
	point := world.Points[*building.StrategicPointID]
	if point != nil && point.ListeningPostEntityID != nil && *point.ListeningPostEntityID == building.ID {
		point.ListeningPostEntityID = nil
		point.HasListeningPost = false
		point.LPTier = 0
	}
	world.finishConstructionBuildersLocked(building.ID)
}
