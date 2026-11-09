// Package transport — WebTransport (QUIC) endpoint. Matches the threejs
// WTNet client: the client opens one bidirectional stream and writes
// length-prefixed JSON frames ({Join:{...}} then {Command:{...}}).
// The server answers on one unidirectional stream with framed
// {Session}/{Snapshot}/{Delta}/{CmdAck} messages.
package transport

import (
	"io"
	"log"
	"net/http"

	"github.com/quic-go/webtransport-go"

	"grimcraft-go/internal/hub"
)

// WTHandler upgrades /grimcraft to WebTransport and serves one session.
// lobbyID picks the default lobby when the URL carries no ?lobby=.
func WTHandler(gameHub *hub.Hub, wtServer *webtransport.Server) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		session, err := wtServer.Upgrade(writer, request)
		if err != nil {
			log.Printf("[wt] upgrade failed: %v", err)
			return
		}
		if requestedLobby := request.URL.Query().Get("lobby"); requestedLobby != "" {
			serveWTSession(gameHub, session, requestedLobby)
		} else {
			lobbies := gameHub.List()
			if len(lobbies) == 0 {
				created := gameHub.CreateLobby("Arena", "Arena", 8)
				serveWTSession(gameHub, session, created.ID)
			} else {
				serveWTSession(gameHub, session, lobbies[0].ID)
			}
		}
	}
}

func serveWTSession(gameHub *hub.Hub, session *webtransport.Session, lobbyID string) {
	// First client stream must be the command bidi stream.
	stream, err := session.AcceptStream(session.Context())
	if err != nil {
		session.CloseWithError(0, "no cmd stream")
		return
	}
	defer stream.Close()

	// Server->client uni stream (framed).
	serverStream, err := session.OpenUniStream()
	if err != nil {
		return
	}
	defer serverStream.Close()

	var client *hub.Client
	room := gameHub.RoomOf(lobbyID)
	if room == nil {
		return
	}
	joined := false
	writeFramed := func(raw []byte) error {
		_, err := serverStream.Write(EncodeFrame(raw))
		return err
	}
	// Forward room broadcasts to the uni stream.
	forwardDone := make(chan struct{})
	defer close(forwardDone)

	streamFramer := &Framer{}
	readBuffer := make([]byte, 65536)
	for {
		bytesRead, err := stream.Read(readBuffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("[wt] cmd stream read: %v", err)
			}
			break
		}
		for _, payload := range streamFramer.Push(readBuffer[:bytesRead]) {
			incoming, err := hub.DecodeClientMessage(payload)
			if err != nil {
				continue
			}
			if incoming.Join != nil && !joined {
				newClient, _, err := gameHub.Join(lobbyID, incoming.Join.PreferredTeam, "", "")
				if err != nil {
					session.CloseWithError(1, err.Error())
					return
				}
				client = newClient
				room = gameHub.RoomOf(lobbyID)
				joined = true
				// Drain the queued Session+Snapshot into the uni stream.
				go func() {
					for {
						select {
						case <-session.Context().Done():
							return
						case <-forwardDone:
							return
						case <-client.Done:
							return
						case outgoing, active := <-client.Out:
							if !active {
								return
							}
							if err := writeFramed(outgoing); err != nil {
								return
							}
						}
					}
				}()
				room.SendSession(client)
				continue
			}
			if incoming.Command != nil && joined {
				accepted, reason := hub.HandleCommand(room.World(), client.PlayerID, incoming.Command)
				hub.Ack(client, accepted, reason)
			}
		}
	}
	if client != nil {
		gameHub.Leave(lobbyID, client)
	}
}
