package sim

import "math"

const captureRodPlacementTime = 2.333333

// Action state is authoritative. Clients select the authored GLB clip for a slot.
func (world *World) tickActionTimersLocked(deltaTime float64) {
	for _, squad := range world.Squads {
		if weapon, found := world.modWeapon(squad); found {
			squad.AttackRange = weapon.Range
		}
		squad.visualCooldown = math.Max(0, squad.visualCooldown-deltaTime)
		if squad.CaptureOrderSP == nil {
			squad.CaptureElapsed = 0
			squad.capturePoint = 0
			squad.CapturingSP = nil
		}
		for index := range squad.Soldiers {
			soldier := &squad.Soldiers[index]
			soldier.balanceCooldown = math.Max(0, soldier.balanceCooldown-deltaTime)
			soldier.ShootTimer = math.Max(0, soldier.ShootTimer-deltaTime)
			soldier.MeleeTimer = math.Max(0, soldier.MeleeTimer-deltaTime)
			soldier.HitReactionTimer = math.Max(0, soldier.HitReactionTimer-deltaTime)
		}
	}
}
func (world *World) recordAttackLocked(squad, target *Squad, victim *Soldier, melee bool) {
	if squad.visualCooldown > 0 {
		return
	}
	squad.visualCooldown = .5
	if melee {
		squad.visualCooldown = 2.5
	}
	for index := range squad.Soldiers {
		soldier := &squad.Soldiers[index]
		if !soldier.Alive {
			continue
		}
		soldier.TargetID = victim.EntityID
		if melee {
			soldier.MeleeSerial++
			soldier.MeleeTimer = squad.visualCooldown
			soldier.ShootTimer = 0
		} else {
			soldier.ShotSerial++
			soldier.ShootTimer = .5
			soldier.MeleeTimer = 0
		}
	}
	if melee {
		reactions := []string{"hitL", "hitR"}
		if (squad.Faction == "space marines" || squad.Faction == "chaos") && (target.Faction == "space marines" || target.Faction == "chaos") {
			reactions = append(reactions, "hitBlock")
		}
		victim.HitReactionAnim = reactions[victim.ReactionSerial%uint64(len(reactions))]
		victim.ReactionSerial++
		victim.HitReactionTimer = .45
	}
}
func soldierAnimation(squad *Squad, soldier *Soldier, index int) string {
	if soldier.HitReactionTimer > 0 {
		return soldier.HitReactionAnim
	}
	if soldier.MeleeTimer > 0 {
		return "melee"
	}
	if soldier.ShootTimer > 0 {
		if squad.moved {
			return "shootMove"
		}
		return "shoot"
	}
	if squad.moved {
		return "run"
	}
	if squad.CapturingSP != nil {
		first := 0
		for first < len(squad.Soldiers) && !squad.Soldiers[first].Alive {
			first++
		}
		if index == first {
			if squad.CaptureElapsed < captureRodPlacementTime {
				return "flagPlant1"
			}
			if squad.CaptureElapsed < 3.708333 {
				return "flagPlant2"
			}
			if squad.CaptureElapsed < 6.625 {
				return "capture1"
			}
		}
		if squad.CaptureElapsed < 2.916667 {
			return "capture1"
		}
		return "capture2"
	}
	return "idle"
}
func builderAnimation(builder *Builder) string {
	if builder.moved {
		return "run"
	}
	if builder.IsBuilding {
		return "build"
	}
	if builder.RepairTarget != nil {
		return "repair"
	}
	return "idle"
}

// Find targets like Skirmish: fire at enemies in range, pursue on attack-move.
func (world *World) acquireTargetLocked(squad *Squad) {
	radius := squad.AttackRange
	if squad.AttackMove || squad.AutoChase {
		radius = squad.DetectRange
	}
	if squad.ForcedTarget != nil {
		target := world.Squads[*squad.ForcedTarget]
		if target != nil && aliveCount(target) > 0 && (!squad.autoTarget || dist(squad.Pos, target.Pos) <= radius) {
			return
		}
		squad.ForcedTarget = nil
		squad.autoTarget = false
	}
	var nearest *Squad
	closestDistance := radius
	for _, other := range world.Squads {
		if other.Team == squad.Team || aliveCount(other) == 0 {
			continue
		}
		distance := dist(squad.Pos, other.Pos)
		if distance <= closestDistance {
			nearest = other
			closestDistance = distance
		}
	}
	if nearest != nil {
		entityID := nearest.ID
		squad.ForcedTarget = &entityID
		squad.autoTarget = true
	}
}
