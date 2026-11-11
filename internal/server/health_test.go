package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"grimcraft-go/internal/hub"
)

func TestHealthReportsSkirmishV2(testCase *testing.T) {
	testServer := httptest.NewServer(API(hub.NewHub()))
	defer testServer.Close()
	response, operationError := testServer.Client().Get(testServer.URL + "/api/health")
	if operationError != nil {
		testCase.Fatal(operationError)
	}
	defer response.Body.Close()
	var body map[string]any
	if currentOperationError := json.NewDecoder(response.Body).Decode(&body); currentOperationError != nil {
		testCase.Fatal(currentOperationError)
	}
	if body["status"] != "ok" || body["snapshot"] != "v2" {
		testCase.Fatalf("old server build: %v", body)
	}
}
