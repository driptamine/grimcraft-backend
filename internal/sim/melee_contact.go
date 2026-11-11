package sim

const meleeReleaseRange = MeleeRange + .5

// Contact temporarily overrides the combat target, never the selected stance
// or movement order. A ranged defender answers an assault with melee.
func (world *World) enterMeleeLocked(squad, enemy *Squad) {
	if squad.meleeOpponent != nil && *squad.meleeOpponent == enemy.ID {
		return
	}
	opponentID := enemy.ID
	squad.meleeOpponent = &opponentID
	squad.visualCooldown = 0
	squad.CaptureOrderSP = nil
	squad.CapturingSP = nil
	for index := range squad.Soldiers {
		squad.Soldiers[index].ShootTimer = 0
		squad.Soldiers[index].MeleeTimer = 0
	}
}
func (world *World) updateMeleeContactsLocked() {
	previousMeleeOpponents := map[int]int{}
	for squadID, squad := range world.Squads {
		if squad.meleeOpponent != nil {
			previousMeleeOpponents[squadID] = *squad.meleeOpponent
		}
		squad.meleeOpponent = nil
	}
	for _, currentSquad := range world.Squads {
		if aliveCount(currentSquad) == 0 {
			continue
		}
		var opponent *Squad
		closestDistance := meleeReleaseRange
		for _, otherSquad := range world.Squads {
			if currentSquad.Team == otherSquad.Team || aliveCount(otherSquad) == 0 {
				continue
			}
			distance := dist(currentSquad.Pos, otherSquad.Pos)
			wasInMeleeContact := previousMeleeOpponents[currentSquad.ID] == otherSquad.ID || previousMeleeOpponents[otherSquad.ID] == currentSquad.ID
			startsMeleeContact := distance <= MeleeRange && (currentSquad.AttackMode == "melee" || otherSquad.AttackMode == "melee")
			if !(startsMeleeContact || wasInMeleeContact && distance <= meleeReleaseRange) {
				continue
			}
			if distance < closestDistance || distance == closestDistance && (opponent == nil || otherSquad.ID < opponent.ID) {
				opponent = otherSquad
				closestDistance = distance
			}
		}
		if opponent != nil {
			if previousMeleeOpponents[currentSquad.ID] == opponent.ID {
				opponentID := opponent.ID
				currentSquad.meleeOpponent = &opponentID
				currentSquad.CaptureOrderSP = nil
				currentSquad.CapturingSP = nil
			} else {
				world.enterMeleeLocked(currentSquad, opponent)
			}
		} else if _, wasEngaged := previousMeleeOpponents[currentSquad.ID]; wasEngaged {
			currentSquad.visualCooldown = 0
			for index := range currentSquad.Soldiers {
				currentSquad.Soldiers[index].MeleeTimer = 0
				currentSquad.Soldiers[index].ShootTimer = 0
			}
		}
	}
}
