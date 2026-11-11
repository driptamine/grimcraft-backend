// Package server wires the HTTP lobby API shared by both transports.
package server

import (
	"encoding/json"
	"net/http"

	"grimcraft-go/internal/hub"
)

// ConnectionInfo contains only public connection metadata, never private key material.
type ConnectionInfo struct {
	Enabled         bool   `json:"enabled"`
	WebTransportURL string `json:"webtransport_url"`
	CertificateHash string `json:"certificate_hash"`
}

// API serves GET/POST /api/lobbies in the lobbies.js shape so the
// frontend server browser can swap LOBBIES for a fetch() later.
func API(gameHub *hub.Hub, connection ...*ConnectionInfo) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("/api/connection", func(responseWriter http.ResponseWriter, currentRequest *http.Request) {
		if currentRequest.Method != http.MethodGet {
			http.Error(responseWriter, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		responseWriter.Header().Set("Cache-Control", "no-store")
		info := ConnectionInfo{}
		if len(connection) > 0 && connection[0] != nil {
			info = *connection[0]
		}
		writeJSON(responseWriter, info)
	})
	router.HandleFunc("/api/lobbies", func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodGet:
			writeJSON(writer, gameHub.List())
		case http.MethodPost:
			var createRequest struct {
				Name     string `json:"name"`
				Map      string `json:"map"`
				Capacity int    `json:"capacity"`
				Total    int    `json:"total"`
			}
			if operationError := json.NewDecoder(request.Body).Decode(&createRequest); operationError != nil {
				http.Error(writer, "bad json", http.StatusBadRequest)
				return
			}
			total := createRequest.Capacity
			if total <= 0 {
				total = createRequest.Total
			}
			lobby := gameHub.CreateLobby(createRequest.Name, createRequest.Map, total)
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusCreated)
			writeJSON(writer, lobby)
		default:
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	router.HandleFunc("/api/health", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, map[string]any{"status": "ok", "skirmish": 2, "snapshot": "v2"})
	})
	return http.HandlerFunc(func(preflightResponseWriter http.ResponseWriter, preflightRequest *http.Request) {
		preflightResponseWriter.Header().Set("Access-Control-Allow-Origin", "*")
		preflightResponseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		preflightResponseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if preflightRequest.Method == http.MethodOptions {
			preflightResponseWriter.WriteHeader(http.StatusNoContent)
			return
		}
		router.ServeHTTP(preflightResponseWriter, preflightRequest)
	})
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(writer).Encode(value)
}
