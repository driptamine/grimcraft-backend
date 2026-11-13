package server

import (
	"encoding/json"
	"grimcraft-go/internal/balance"
	"io"
	"net/http"
)

func balanceTest(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 512000)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var testRequest balance.TestRequest
	if operationError := decoder.Decode(&testRequest); operationError != nil {
		http.Error(writer, "invalid balance request: "+operationError.Error(), http.StatusBadRequest)
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		http.Error(writer, "expected one JSON object", http.StatusBadRequest)
		return
	}
	result, operationError := balance.TestBattle(testRequest)
	if operationError != nil {
		http.Error(writer, operationError.Error(), http.StatusBadRequest)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, result)
}
