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
		client, _, err := gameHub.Join(lobbyID, preferred, query.Get("username"), query.Get("faction"))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		conn, err := websocket.Accept(writer, request, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			gameHub.Leave(lobbyID, client)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "bye")
		defer gameHub.Leave(lobbyID, client)

		room := gameHub.RoomOf(lobbyID)
		ctx := request.Context()
		room.SendSession(client)

		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-client.Done:
					return
				case outgoing, active := <-client.Out:
					if !active {
						return
					}
					writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					err := conn.Write(writeCtx, websocket.MessageText, outgoing)
					cancel()
					if err != nil {
						return
					}
				}
			}
		}()

		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			incoming, err := hub.DecodeClientMessage(raw)
			if err != nil {
				hub.Ack(client, false, "bad json")
				continue
			}
			if incoming.Join != nil && incoming.Join.PreferredTeam != nil {
				// Already joined; team is fixed at join time.
				hub.Ack(client, true, "")
				continue
			}
			if incoming.Command != nil {
				accepted, reason := hub.HandleCommand(room.World(), client.PlayerID, incoming.Command)
				hub.Ack(client, accepted, reason)
				if !accepted {
					log.Printf("[ws] lobby=%s player=%d rejected: %s", lobbyID, client.PlayerID, reason)
				}
			}
		}
	}
}
