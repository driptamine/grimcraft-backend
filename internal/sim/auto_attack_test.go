package sim

import (
	"grimcraft-go/internal/protocol"
	"testing"
)

func TestAutomaticRangedAndAssault(testCase *testing.T) {
	world := NewWorld()
	attackingSquad := world.SpawnSquad("Blue", 1)
	defendingSquad := world.SpawnSquad("Red", 2)
	attackingSquad.Pos = protocol.Vec3{}
	defendingSquad.Pos = protocol.Vec3{X: 16}
	for index := range attackingSquad.Soldiers {
		attackingSquad.Soldiers[index].HP = 1000
	}
	for currentIndex := range defendingSquad.Soldiers {
		defendingSquad.Soldiers[currentIndex].HP = 1000
	}
	world.Advance(.1)
	if attackingSquad.ForcedTarget == nil || *attackingSquad.ForcedTarget != defendingSquad.ID || defendingSquad.ForcedTarget == nil {
		testCase.Fatal("enemies do not auto-acquire each other")
	}
	if attackingSquad.Soldiers[0].ShotSerial == 0 || defendingSquad.Soldiers[0].HP >= 1000 {
		testCase.Fatal("automatic ranged fire missing")
	}
	if attackingSquad.Pos.X != 0 {
		testCase.Fatal("ranged must fire in range without charging")
	}
	world.SetCombatStance(1, []int{attackingSquad.ID}, "melee")
	for simulationStep := 0; simulationStep < 40; simulationStep++ {
		world.Advance(.1)
	}
	if attackingSquad.Soldiers[0].MeleeSerial == 0 {
		testCase.Fatal("Assault failed to close to melee")
	}
	if attackingSquad.AttackMode != "melee" {
		testCase.Fatal("stance lost")
	}
}
func TestAttackMoveKeepsAssaultAndAcquiresBeyondGunRange(testCase *testing.T) {
	world := NewWorld()
	attackingSquad := world.SpawnSquad("Blue", 1)
	defendingSquad := world.SpawnSquad("Red", 2)
	attackingSquad.Pos = protocol.Vec3{}
	defendingSquad.Pos = protocol.Vec3{X: 25}
	world.SetCombatStance(1, []int{attackingSquad.ID}, "melee")
	world.UnitCommand(1, []int{attackingSquad.ID}, "attackmove", protocol.Vec3{X: 80}, nil)
	world.Advance(.1)
	if attackingSquad.AttackMode != "melee" || !attackingSquad.AttackMove || attackingSquad.ForcedTarget == nil {
		testCase.Fatal("attack move lost stance or did not acquire enemy")
	}
	if attackingSquad.Pos.X > attackingSquad.Speed*.1+.001 {
		testCase.Fatal("attack move moved twice in one tick")
	}
	world.SetCombatStance(1, []int{attackingSquad.ID}, "ranged")
	world.Advance(.1)
	if attackingSquad.AttackMode != "ranged" {
		testCase.Fatal("ranged switch lost")
	}
}
