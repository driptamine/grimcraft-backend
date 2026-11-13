// Package balance defines the versioned mod format and damage rules shared by
// the editor's test arena and authoritative matches. No Node.js process is used.
package balance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
)

var ArmorTypes = []string{"infantry_low", "infantry_med", "infantry_high", "infantry_hmed", "infantry_hhi", "infantry_cmdr", "vehicle_low", "vehicle_med", "vehicle_high", "vehicle_air", "building_low", "building_med", "building_high", "daemon_med", "daemon_high"}

type Weapon struct {
	Name           string              `json:"name"`
	Mode           string              `json:"mode"`
	MinDamage      float64             `json:"minDamage"`
	MaxDamage      float64             `json:"maxDamage"`
	MinimumDamage  float64             `json:"minimumDamage"`
	Accuracy       float64             `json:"accuracy"`
	MovingAccuracy float64             `json:"movingAccuracy"`
	Refire         float64             `json:"refire"`
	Range          float64             `json:"range"`
	ArmorPiercing  map[string]*float64 `json:"armorPiercing"`
	DPS            map[string]float64  `json:"dps"`
}

// Reject missing/null numeric fields rather than silently turning them into zero.
func (weapon *Weapon) UnmarshalJSON(data []byte) error {
	type plainWeapon Weapon
	var decoded plainWeapon
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if operationError := decoder.Decode(&decoded); operationError != nil {
		return operationError
	}
	var fields map[string]json.RawMessage
	if operationError := json.Unmarshal(data, &fields); operationError != nil {
		return operationError
	}
	for _, field := range []string{"minDamage", "maxDamage", "minimumDamage", "accuracy", "movingAccuracy", "refire", "range"} {
		value, present := fields[field]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s must be a number", field)
		}
	}
	var damageFields map[string]json.RawMessage
	if operationError := json.Unmarshal(fields["dps"], &damageFields); operationError != nil {
		return errors.New("dps must be an object")
	}
	for _, armor := range ArmorTypes {
		value, present := damageFields[armor]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s DPS must be a number", armor)
		}
	}
	*weapon = Weapon(decoded)
	return nil
}

type Mod struct {
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Weapons       map[string]Weapon `json:"weapons"`
}

func validNumber(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}
func ValidArmor(armor string) bool {
	for _, name := range ArmorTypes {
		if armor == name {
			return true
		}
	}
	return false
}
func (weapon Weapon) Validate() error {
	if strings.TrimSpace(weapon.Name) == "" || len(weapon.Name) > 480 {
		return errors.New("weapon name is required")
	}
	if weapon.Mode != "ap" && weapon.Mode != "dps" {
		return errors.New("weapon mode must be ap or dps")
	}
	for _, value := range []float64{weapon.MinDamage, weapon.MaxDamage, weapon.MinimumDamage} {
		if !validNumber(value, 0, 1e6) {
			return errors.New("damage must be 0–1000000")
		}
	}
	if weapon.MinDamage > weapon.MaxDamage {
		return errors.New("Min exceeds Max")
	}
	if !validNumber(weapon.Accuracy, 0, 1) || !validNumber(weapon.MovingAccuracy, 0, 1) {
		return errors.New("accuracy must be 0–1")
	}
	if !validNumber(weapon.Refire, .05, 300) || !validNumber(weapon.Range, 0, 1000) {
		return errors.New("invalid refire or range")
	}
	for _, armor := range ArmorTypes {
		piercing, present := weapon.ArmorPiercing[armor]
		if !present || (weapon.Mode == "ap" && piercing == nil) {
			return fmt.Errorf("%s: missing armour piercing", armor)
		}
		if piercing != nil && !validNumber(*piercing, 0, 100) {
			return fmt.Errorf("%s: AP must be 0–100", armor)
		}
		dps, present := weapon.DPS[armor]
		if !present || !validNumber(dps, 0, 1e7) || (weapon.Mode == "dps" && weapon.Accuracy == 0 && dps > 0) {
			return fmt.Errorf("%s: invalid DPS", armor)
		}
	}
	return nil
}

var modID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)
var weaponID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,79}$`)

func (mod Mod) Validate() error {
	if mod.SchemaVersion != 1 || !modID.MatchString(mod.ID) || strings.TrimSpace(mod.Name) == "" || len(mod.Name) > 480 || strings.TrimSpace(mod.Version) == "" || len(mod.Version) > 480 {
		return errors.New("invalid mod metadata or schemaVersion")
	}
	if mod.Weapons == nil || len(mod.Weapons) > 128 {
		return errors.New("invalid weapons table")
	}
	for key, weapon := range mod.Weapons {
		if !weaponID.MatchString(key) || key == "constructor" || key == "prototype" {
			return errors.New("invalid weapon key")
		}
		if operationError := weapon.Validate(); operationError != nil {
			return fmt.Errorf("%s: %w", key, operationError)
		}
	}
	return nil
}
func (weapon Weapon) ExpectedHit(armor string) float64 {
	if weapon.Mode == "dps" {
		if weapon.Accuracy == 0 {
			return 0
		}
		return weapon.DPS[armor] * weapon.Refire / weapon.Accuracy
	}
	piercing := weapon.ArmorPiercing[armor]
	if piercing == nil {
		return 0
	}
	return math.Max((weapon.MinDamage+weapon.MaxDamage)/2**piercing/100, weapon.MinimumDamage)
}
func (weapon Weapon) HitDamage(armor string, moving bool, roll float64) float64 {
	accuracy := weapon.Accuracy
	if moving {
		accuracy = weapon.MovingAccuracy
	}
	if roll >= accuracy {
		return 0
	}
	return weapon.ExpectedHit(armor)
}
func (weapon Weapon) ExpectedDPS(armor string, moving bool) float64 {
	accuracy := weapon.Accuracy
	if moving {
		accuracy = weapon.MovingAccuracy
	}
	return weapon.ExpectedHit(armor) * accuracy / weapon.Refire
}

type TestRequest struct {
	Weapon   Weapon  `json:"weapon"`
	Armor    string  `json:"armor"`
	TargetHP float64 `json:"targetHP"`
	Count    int     `json:"count"`
	Duration float64 `json:"duration"`
	Seed     int64   `json:"seed"`
	Moving   bool    `json:"moving"`
}
type Sample struct {
	Time float64 `json:"time"`
	HP   float64 `json:"hp"`
}
type TestResult struct {
	ExpectedDPS float64  `json:"expectedDps"`
	Damage      float64  `json:"damage"`
	MeasuredDPS float64  `json:"measuredDps"`
	Shots       int      `json:"shots"`
	Hits        int      `json:"hits"`
	Killed      bool     `json:"killed"`
	Elapsed     float64  `json:"elapsed"`
	Timeline    []Sample `json:"timeline"`
}

func TestBattle(request TestRequest) (TestResult, error) {
	if operationError := request.Weapon.Validate(); operationError != nil {
		return TestResult{}, operationError
	}
	if !ValidArmor(request.Armor) || !validNumber(request.TargetHP, 1, 1e7) || request.Count < 1 || request.Count > 100 || !validNumber(request.Duration, 1, 300) || request.Seed < 0 || request.Seed > 2147483647 {
		return TestResult{}, errors.New("invalid target, count, duration or seed")
	}
	volleys := int(math.Floor((request.Duration + 1e-9) / request.Weapon.Refire))
	if volleys*request.Count > 200000 {
		return TestResult{}, errors.New("test exceeds 200000 shots; reduce duration or shooter count")
	}
	result := TestResult{ExpectedDPS: request.Weapon.ExpectedDPS(request.Armor, request.Moving) * float64(request.Count), Elapsed: request.Duration, Timeline: []Sample{{Time: 0, HP: request.TargetHP}}}
	random := rand.New(rand.NewSource(request.Seed))
	health := request.TargetHP
	for volley := 1; volley <= volleys; volley++ {
		moment := float64(volley) * request.Weapon.Refire
		for shooter := 0; shooter < request.Count; shooter++ {
			result.Shots++
			damage := request.Weapon.HitDamage(request.Armor, request.Moving, random.Float64())
			if damage > 0 {
				result.Hits++
				health = math.Max(0, health-damage)
			}
			if health == 0 {
				result.Killed = true
				result.Elapsed = moment
				break
			}
		}
		if result.Killed || moment-result.Timeline[len(result.Timeline)-1].Time >= request.Duration/300 {
			result.Timeline = append(result.Timeline, Sample{Time: moment, HP: health})
		}
		if result.Killed {
			break
		}
	}
	if result.Timeline[len(result.Timeline)-1].Time != result.Elapsed {
		result.Timeline = append(result.Timeline, Sample{Time: result.Elapsed, HP: health})
	}
	result.Damage = request.TargetHP - health
	result.MeasuredDPS = result.Damage / result.Elapsed
	return result, nil
}
