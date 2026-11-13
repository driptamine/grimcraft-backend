package hub

import (
	"grimcraft-go/internal/balance"
	"testing"
)

func TestHostBalanceIsValidatedAndFrozenAtStart(testCase *testing.T) {
	gameHub := NewHub()
	lobby := gameHub.CreateLobby("Balance", "Arena", 2)
	host, _, operationError := gameHub.Join(lobby.ID, nil, "Host", "Imperial Guard")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	guest, _, operationError := gameHub.Join(lobby.ID, nil, "Guest", "Chaos")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	profile := balance.Weapon{Name: "Lasgun", Mode: "ap", MinDamage: 20, MaxDamage: 24, Accuracy: 1, Refire: 1, Range: 27, ArmorPiercing: map[string]*float64{}, DPS: map[string]float64{}}
	for _, armor := range balance.ArmorTypes {
		piercing := 25.0
		profile.ArmorPiercing[armor] = &piercing
		profile.DPS[armor] = 0
	}
	mod := &balance.Mod{SchemaVersion: 1, ID: "test", Name: "Test", Version: "1", Weapons: map[string]balance.Weapon{"lasgun": profile}}
	command := map[string]any{"StartGame": map[string]any{"balanceMod": mod}}
	if _, accepted, _ := gameHub.HandleLobbyCommand(lobby.ID, guest, command); accepted {
		testCase.Fatal("guest changed balance")
	}
	profile.Refire = 0
	mod.Weapons["lasgun"] = profile
	if _, accepted, _ := gameHub.HandleLobbyCommand(lobby.ID, host, command); accepted || lobby.Started {
		testCase.Fatal("invalid mod started match")
	}
	profile.Refire = 1
	mod.Weapons["lasgun"] = profile
	if _, accepted, reason := gameHub.HandleLobbyCommand(lobby.ID, host, command); !accepted {
		testCase.Fatal(reason)
	}
	if lobby.BalanceMod == nil || lobby.BalanceMod.Name != "Test" || len(lobby.BalanceHash) != 64 {
		testCase.Fatal("missing replicated mod identity")
	}
	hash := lobby.BalanceHash
	if _, accepted, _ := gameHub.HandleLobbyCommand(lobby.ID, host, map[string]any{"StartGame": map[string]any{}}); !accepted || lobby.BalanceHash != hash {
		testCase.Fatal("balance changed during started match")
	}
}
