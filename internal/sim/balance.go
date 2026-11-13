package sim

import (
	"encoding/json"
	"grimcraft-go/internal/balance"
	"math/rand"
	"strings"
)

// InstallBalance copies a validated mod once before the room starts.
func (world *World) InstallBalance(mod *balance.Mod) error {
	if mod == nil {
		return nil
	}
	if operationError := mod.Validate(); operationError != nil {
		return operationError
	}
	encoded, _ := json.Marshal(mod)
	var frozen balance.Mod
	if operationError := json.Unmarshal(encoded, &frozen); operationError != nil {
		return operationError
	}
	world.mutex.Lock()
	defer world.mutex.Unlock()
	world.balanceMod = &frozen
	return nil
}
func squadWeaponKey(squad *Squad) string {
	if squad.HasPlasma || strings.Contains(strings.ToLower(squad.ProjectileType), "plasma") {
		if squad.UnitType == "kasrkin" {
			return "kasrkin_plasma_gun"
		}
		if squad.Faction == "chaos" {
			return "chaos_plasma_gun"
		}
		return "plasma_gun"
	}
	switch squad.UnitType {
	case "kasrkin":
		return "hellgun"
	case "dark_reaper":
		return "reaper_launcher"
	case "boyz_shoota":
		return "shoota"
	case "grey_knight", "grey_knights":
		return "storm_bolter"
	case "sergeant", "commissar":
		return "laspistol"
	}
	switch canonicalFaction(squad.Faction) {
	case "chaos":
		return "bolter"
	case "space marines":
		return "space_marine_bolter"
	case "eldar":
		return "shuriken_catapult"
	case "necrons":
		return "gauss_flayer"
	case "tau":
		return "pulse_rifle"
	case "orks":
		return "slugga"
	default:
		return "lasgun"
	}
}
func squadArmor(squad *Squad) string {
	switch squad.UnitType {
	case "kasrkin", "grey_knight", "grey_knights", "sergeant":
		return "infantry_hmed"
	case "dark_reaper":
		return "infantry_high"
	}
	switch canonicalFaction(squad.Faction) {
	case "chaos", "space marines", "necrons":
		return "infantry_hmed"
	case "tau":
		return "infantry_hmed"
	case "orks":
		return "infantry_high"
	default:
		return "infantry_med"
	}
}
func (world *World) modWeapon(squad *Squad) (balance.Weapon, bool) {
	if world.balanceMod == nil {
		return balance.Weapon{}, false
	}
	weapon, found := world.balanceMod.Weapons[squadWeaponKey(squad)]
	return weapon, found
}

// Each living soldier fires on its own timer. The same HitDamage rule drives
// the editor arena; no per-tick flat damage or visual-cadence compensation applies.
func (world *World) balanceRangedAttack(squad, target *Squad) bool {
	weapon, found := world.modWeapon(squad)
	if !found {
		return false
	}
	for index := range squad.Soldiers {
		shooter := &squad.Soldiers[index]
		if !shooter.Alive || shooter.balanceCooldown > 1e-9 {
			continue
		}
		var victim *Soldier
		for victimIndex := range target.Soldiers {
			if target.Soldiers[victimIndex].Alive {
				victim = &target.Soldiers[victimIndex]
				break
			}
		}
		if victim == nil {
			break
		}
		shooter.balanceCooldown = weapon.Refire
		shooter.ShotSerial++
		shooter.ShootTimer = .4
		shooter.TargetID = victim.EntityID
		victim.HP -= weapon.HitDamage(squadArmor(target), squad.moved, rand.Float64())
		if victim.HP <= 0 {
			victim.HP = 0
			victim.Alive = false
			if killer := world.playerOf(squad.Owner); killer != nil {
				killer.Kills++
			}
		}
	}
	return true
}
