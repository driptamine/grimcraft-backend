package sim

import (
	"grimcraft-go/internal/balance"
	"grimcraft-go/internal/protocol"
	"testing"
)

func TestAuthoritativeBalanceShotsAndFrozenConfig(testCase *testing.T) {
	world := NewWorld()
	attacker := world.SpawnSquad("Blue", 1)
	attacker.Faction = "ig"
	attacker.Pos = protocol.Vec3{}
	target := world.SpawnSquad("Red", 2)
	target.Faction = "chaos"
	target.Pos = protocol.Vec3{X: 15}
	target.Stopped = true
	for index := range attacker.Soldiers {
		attacker.Soldiers[index].Alive = index == 0
		attacker.Soldiers[index].HP = 10000
	}
	for index := range target.Soldiers {
		target.Soldiers[index].HP = 10000
	}
	profile := balance.Weapon{Name: "Lasgun", Mode: "ap", MinDamage: 20, MaxDamage: 24, MinimumDamage: 1, Accuracy: 1, MovingAccuracy: 1, Refire: 1, Range: 27, ArmorPiercing: map[string]*float64{}, DPS: map[string]float64{}}
	for _, armor := range balance.ArmorTypes {
		piercing := 25.0
		profile.ArmorPiercing[armor] = &piercing
		profile.DPS[armor] = 5.5
	}
	mod := &balance.Mod{SchemaVersion: 1, ID: "test", Name: "Test", Version: "1", Weapons: map[string]balance.Weapon{"lasgun": profile}}
	if operationError := world.InstallBalance(mod); operationError != nil {
		testCase.Fatal(operationError)
	}
	profile.MinDamage = 1000
	mod.Weapons["lasgun"] = profile
	world.Advance(.1)
	if target.Soldiers[0].HP != 9994.5 {
		testCase.Fatalf("wrong initial damage: %g", target.Soldiers[0].HP)
	}
	for tick := 0; tick < 9; tick++ {
		world.Advance(.1)
	}
	if target.Soldiers[0].HP != 9994.5 {
		testCase.Fatal("fired before one second")
	}
	world.Advance(.1)
	if target.Soldiers[0].HP != 9989 || attacker.Soldiers[0].ShotSerial != 2 {
		testCase.Fatal("incorrect authoritative refire")
	}
}
