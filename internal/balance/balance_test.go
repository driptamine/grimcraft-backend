package balance

import (
	"reflect"
	"testing"
)

func testWeapon() Weapon {
	profile := Weapon{Name: "Lasgun", Mode: "ap", MinDamage: 20, MaxDamage: 24, MinimumDamage: 1, Accuracy: 1, MovingAccuracy: .15, Refire: 1, Range: 27, ArmorPiercing: map[string]*float64{}, DPS: map[string]float64{}}
	for _, armor := range ArmorTypes {
		piercing := 25.0
		profile.ArmorPiercing[armor] = &piercing
		profile.DPS[armor] = 5.5
	}
	return profile
}
func TestDamageRules(testCase *testing.T) {
	profile := testWeapon()
	if profile.ExpectedHit("infantry_med") != 5.5 || profile.ExpectedDPS("infantry_med", false) != 5.5 {
		testCase.Fatal("incorrect AP formula")
	}
	profile.Refire = 2
	if profile.ExpectedHit("infantry_med") != 5.5 || profile.ExpectedDPS("infantry_med", false) != 2.75 {
		testCase.Fatal("refire changed hit damage")
	}
	zero := 0.0
	profile.ArmorPiercing["vehicle_high"] = &zero
	if profile.HitDamage("vehicle_high", false, 0) != 1 || profile.HitDamage("infantry_med", true, .9) != 0 {
		testCase.Fatal("minimum damage or moving accuracy failed")
	}
}
func TestRepeatableArenaAndBounds(testCase *testing.T) {
	request := TestRequest{Weapon: testWeapon(), Armor: "infantry_med", TargetHP: 100000, Count: 5, Duration: 10, Seed: 42}
	first, operationError := TestBattle(request)
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	second, _ := TestBattle(request)
	if !reflect.DeepEqual(first, second) || first.Shots != 50 || first.Hits != 50 || first.Damage != 275 || first.MeasuredDPS != 27.5 {
		testCase.Fatalf("unexpected result: %+v", first)
	}
	request.TargetHP = 11
	request.Count = 1
	killed, _ := TestBattle(request)
	if !killed.Killed || killed.Elapsed != 2 || killed.Damage != 11 {
		testCase.Fatal("incorrect kill time")
	}
	request.Weapon.Refire = 0
	if _, operationError = TestBattle(request); operationError == nil {
		testCase.Fatal("accepted zero refire")
	}
	request.Weapon = testWeapon()
	request.Weapon.Refire = .05
	request.Duration = 300
	request.Count = 100
	if _, operationError = TestBattle(request); operationError == nil {
		testCase.Fatal("unbounded test")
	}
}
func TestUnknownPiercingIsRejected(testCase *testing.T) {
	profile := testWeapon()
	profile.ArmorPiercing["vehicle_high"] = nil
	if profile.Validate() == nil {
		testCase.Fatal("unknown AP became zero")
	}
	profile.Mode = "dps"
	if profile.Validate() != nil {
		testCase.Fatal("legacy DPS must preserve unknown AP")
	}
}
