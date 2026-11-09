// Package server wires the HTTP lobby API shared by both transports.
package server

import (
	"encoding/json"
	"net/http"

	"grimcraft-go/internal/hub"
)

// API serves GET/POST /api/lobbies in the lobbies.js shape so the
// frontend server browser can swap LOBBIES for a fetch() later.
func API(gameHub *hub.Hub) http.Handler {
	router := http.NewServeMux()
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
			if err := json.NewDecoder(request.Body).Decode(&createRequest); err != nil {
				http.Error(writer, "bad json", http.StatusBadRequest)
				return
			}
			total := createRequest.Capacity
			if total <= 0 {
				total = createRequest.Total
			}
			lobby := gameHub.CreateLobby(createRequest.Name, createRequest.Map, total)
			writer.WriteHeader(http.StatusCreated)
			writeJSON(writer, lobby)
		default:
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	router.HandleFunc("/api/health", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, map[string]string{"status": "ok"})
	})
	return router
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(writer).Encode(value)
}
