package server

import (
	"bytes"
	"encoding/json"
	"grimcraft-go/internal/balance"
	"grimcraft-go/internal/hub"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBalanceArenaAPI(testCase *testing.T) {
	profile := balance.Weapon{Name: "Test", Mode: "ap", MinDamage: 20, MaxDamage: 24, Accuracy: 1, Refire: 1, Range: 20, ArmorPiercing: map[string]*float64{}, DPS: map[string]float64{}}
	for _, armor := range balance.ArmorTypes {
		piercing := 25.0
		profile.ArmorPiercing[armor] = &piercing
		profile.DPS[armor] = 0
	}
	payload, _ := json.Marshal(balance.TestRequest{Weapon: profile, Armor: "infantry_med", TargetHP: 1000, Count: 1, Duration: 10})
	handler := API(hub.NewHub())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/balance/test", bytes.NewReader(payload)))
	if response.Code != http.StatusOK {
		testCase.Fatal(response.Body.String())
	}
	var result balance.TestResult
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if result.Damage != 55 || result.MeasuredDPS != 5.5 || response.Header().Get("Access-Control-Allow-Origin") != "*" {
		testCase.Fatalf("bad response: %+v", result)
	}
	for _, body := range []string{`{}`, string(payload) + `{}`, `{"weapon":{"refire":0}}`} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/balance/test", bytes.NewBufferString(body)))
		if response.Code != http.StatusBadRequest {
			testCase.Fatal("accepted invalid request")
		}
	}
}
