// Package transport — WebSocket fallback. The threejs client speaks
// WebTransport natively; this endpoint speaks the same JSON messages
// unframed (one text message each) for local dev, bots, and tests:
//
//	WS /join?lobby=<id>&team=Blue|Red&username=<n>&faction=<f>
package transport

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"grimcraft-go/internal/hub"
)

// WSHandler bridges one WebSocket connection to a room client.
func WSHandler(gameHub *hub.Hub) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		lobbyID := query.Get("lobby")
		if lobbyID == "" {
			if lobbies := gameHub.List(); len(lobbies) > 0 {
				lobbyID = lobbies[0].ID
			}
		}
		var preferred *string
		if teamName := query.Get("team"); teamName == "Blue" || teamName == "Red" {
			preferred = &teamName
		}
		client, _, operationError := gameHub.Join(lobbyID, preferred, query.Get("username"), query.Get("faction"))
		if operationError != nil {
			http.Error(writer, operationError.Error(), http.StatusBadRequest)
			return
		}
		connection, operationError := websocket.Accept(writer, request, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if operationError != nil {
			gameHub.Leave(lobbyID, client)
			return
		}
		defer connection.Close(websocket.StatusNormalClosure, "bye")
		defer gameHub.Leave(lobbyID, client)

		room := gameHub.RoomOf(lobbyID)
		requestContext := request.Context()
		room.SendSession(client)
		gameHub.BroadcastLobby(lobbyID)

		go func() {
			for {
				select {
				case <-requestContext.Done():
					return
				case <-client.Done:
					return
				case outgoing, active := <-client.Out:
					if !active {
						return
					}
					writeContext, cancel := context.WithTimeout(requestContext, 5*time.Second)
					currentOperationError := connection.Write(writeContext, websocket.MessageText, outgoing)
					cancel()
					if currentOperationError != nil {
						return
					}
				}
			}
		}()

		for {
			_, encodedData, readMessageError := connection.Read(requestContext)
			if readMessageError != nil {
				return
			}
			incoming, readMessageError := hub.DecodeClientMessage(encodedData)
			if readMessageError != nil {
				hub.Ack(client, false, "bad json")
				continue
			}
			if incoming.Join != nil && incoming.Join.PreferredTeam != nil {
				// Already joined; team is fixed at join time.
				hub.Ack(client, true, "")
				continue
			}
			if incoming.Command != nil {
				handled, accepted, reason := gameHub.HandleLobbyCommand(lobbyID, client, incoming.Command)
				if !handled {
					accepted, reason = room.ApplyCommand(client.PlayerID, incoming.Command)
				}
				hub.Ack(client, accepted, reason)
				if !accepted {
					log.Printf("[ws] lobby=%s player=%d rejected: %s", lobbyID, client.PlayerID, reason)
				}
			}
		}
	}
}
