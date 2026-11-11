package server

import (
	"encoding/json"
	"grimcraft-go/internal/hub"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebTransportDiscovery(testCase *testing.T) {
	info := &ConnectionInfo{Enabled: true, WebTransportURL: "https://127.0.0.1:4433/grimcraft", CertificateHash: strings.Repeat("a", 64)}
	handler := API(hub.NewHub(), info)
	for _, hash := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		info.CertificateHash = hash
		responseRecorder := httptest.NewRecorder()
		handler.ServeHTTP(responseRecorder, httptest.NewRequest(http.MethodGet, "/api/connection", nil))
		if responseRecorder.Code != http.StatusOK || responseRecorder.Header().Get("Cache-Control") != "no-store" {
			testCase.Fatal("discovery must be successful and uncached")
		}
		var actualResult ConnectionInfo
		if operationError := json.Unmarshal(responseRecorder.Body.Bytes(), &actualResult); operationError != nil {
			testCase.Fatal(operationError)
		}
		if actualResult != *info {
			testCase.Fatalf("wrong or stale metadata: %+v", actualResult)
		}
		if responseRecorder.Header().Get("Access-Control-Allow-Origin") != "*" {
			testCase.Fatal("local frontend cannot read discovery")
		}
	}
}
func TestDiscoveryWithoutWebTransport(testCase *testing.T) {
	responseRecorder := httptest.NewRecorder()
	API(hub.NewHub()).ServeHTTP(responseRecorder, httptest.NewRequest(http.MethodGet, "/api/connection", nil))
	var info ConnectionInfo
	if operationError := json.Unmarshal(responseRecorder.Body.Bytes(), &info); operationError != nil {
		testCase.Fatal(operationError)
	}
	if info.Enabled || info.CertificateHash != "" || info.WebTransportURL != "" {
		testCase.Fatal("disabled transport must not advertise a certificate")
	}
}
