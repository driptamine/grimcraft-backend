package sim

// Vehicle orders must remain vehicles in the authoritative simulation and in
// snapshots, so the client can instantiate the matching model.
type vehicleProductionEntry struct {
	buildingType string
	cost         float64
	time         float64
	icon         string
	requiredTech string
}

var imperialGuardVehicleProduction = map[string]vehicleProductionEntry{
	"chimera":                {"mechanized", 180, 7, "guard_squad_chimera.png", ""},
	"sentinel":               {"mechanized", 140, 5, "guard_squad_sentinel.png", ""},
	"hellhound":              {"mechanized", 220, 8, "guard_squad_hellhound.png", ""},
	"basilisk":               {"mechanized", 260, 10, "guard_squad_basilisk.png", ""},
	"marauder_bomber":        {"mechanized", 320, 12, "marauder_icon.png", ""},
	"leman_russ_battle_tank": {"mechanized", 300, 11, "guard_squad_lemanruss.png", "full_scale_war"},
	"baneblade":              {"mars_pattern", 400, 20, "guard_squad_baneblade.png", "full_scale_war"},
}
