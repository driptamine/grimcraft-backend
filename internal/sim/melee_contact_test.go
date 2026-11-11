package sim

import (
	"grimcraft-go/internal/protocol"
	"testing"
)

func contactWorld() (*World, *Squad, *Squad) {
	world := NewWorld()
	attackingSquad := world.SpawnSquad("Blue", 1)
	defendingSquad := world.SpawnSquad("Red", 2)
	attackingSquad.Pos = protocol.Vec3{}
	defendingSquad.Pos = protocol.Vec3{X: 2.9}
	attackingSquad.AttackMode = "melee"
	defendingSquad.AttackMode = "ranged"
	for _, squad := range []*Squad{attackingSquad, defendingSquad} {
		for index := range squad.Soldiers {
			squad.Soldiers[index].HP = 1000
		}
	}
	return world, attackingSquad, defendingSquad
}
func TestMeleeIsMutualAndDoesNotChangeStance(testCase *testing.T) {
	for attempt := 0; attempt < 25; attempt++ {
		world, attackingSquad, defendingSquad := contactWorld()
		world.Advance(.1)
		for _, pair := range [][2]*Squad{{attackingSquad, defendingSquad}, {defendingSquad, attackingSquad}} {
			squad, enemy := pair[0], pair[1]
			if squad.meleeOpponent == nil || *squad.meleeOpponent != enemy.ID {
				testCase.Fatal("contact must be reciprocal")
			}
			if squad.Soldiers[0].MeleeSerial == 0 || squad.Soldiers[0].ShootTimer > 0 || squad.Soldiers[0].ShotSerial > 0 {
				testCase.Fatal("both units must strike instead of shooting")
			}
			if squad.Soldiers[0].HP >= 1000 {
				testCase.Fatal("both units must receive melee damage")
			}
			actor := actorFrom(world, squad.Soldiers[0].EntityID)
			if actor["meleeEngaged"] != true {
				testCase.Fatal("contact missing from client snapshot")
			}
		}
		if defendingSquad.AttackMode != "ranged" {
			testCase.Fatal("contact overwrote ranged stance")
		}
	}
}
func TestContactOverridesDistantTargetAndReleases(testCase *testing.T) {
	world, attackingSquad, defendingSquad := contactWorld()
	far := world.SpawnSquad("Blue", 3)
	far.Pos = protocol.Vec3{X: 16}
	world.Attack(2, []int{defendingSquad.ID}, far.ID)
	world.Advance(.1)
	if defendingSquad.Soldiers[0].TargetID != attackingSquad.Soldiers[0].EntityID {
		testCase.Fatal("ranged defender kept shooting distant enemy")
	}
	previous := defendingSquad.Soldiers[0].MeleeSerial
	attackingSquad.Pos = protocol.Vec3{X: -10}
	world.StopUnits(1, []int{attackingSquad.ID})
	world.Advance(.1)
	if defendingSquad.meleeOpponent != nil || defendingSquad.Soldiers[0].MeleeTimer > 0 {
		testCase.Fatal("contact did not release")
	}
	if defendingSquad.AttackMode != "ranged" || defendingSquad.Soldiers[0].ShotSerial == 0 || defendingSquad.Soldiers[0].MeleeSerial != previous {
		testCase.Fatal("ranged defender failed to resume ranged fire")
	}
}
func TestRangedUnitsDoNotStartMeleeWithoutAnAssault(testCase *testing.T) {
	world, attackingSquad, defendingSquad := contactWorld()
	attackingSquad.AttackMode = "ranged"
	world.Advance(.1)
	if attackingSquad.meleeOpponent != nil || defendingSquad.meleeOpponent != nil {
		testCase.Fatal("two ranged units were forced into melee")
	}
}
func TestMeleeReleaseHysteresisAndFriendlyUnits(testCase *testing.T) {
	world, attackingSquad, defendingSquad := contactWorld()
	world.Advance(.1)
	attackingSquad.AttackMode = "ranged"
	defendingSquad.Pos.X = 3.2
	world.Advance(.1)
	if defendingSquad.meleeOpponent == nil {
		testCase.Fatal("contact flickered near range boundary")
	}
	defendingSquad.Team = attackingSquad.Team
	world.Advance(.1)
	if attackingSquad.meleeOpponent != nil || defendingSquad.meleeOpponent != nil {
		testCase.Fatal("friendly squads entered melee")
	}
}
