# grimcraft-go — authoritative multiplayer backend for Grimcraft RTS

Go server for the [GrimCraft-main](../GrimCraft-main) frontend
(`src/net/multiplayer.ts`). One process hosts **many lobbies, one room (game) each**.

## Protocol (matches `src/net/multiplayer.ts`)

- **WebTransport** `https://<host>:4433/grimcraft?lobby=<id>` — used by the multiplayer lobby UI.
  Client opens one bidi stream, writes 4-byte LE length-prefixed JSON frames:
  `{"Join":{"preferred_team":…}}`, then `{"Command":{…}}`.
  Server answers on a uni stream with framed
  `{"Session"…}`, `{"Snapshot"…}`, `{"Delta"…}`, `{"CmdAck"…}`.
- **WebSocket** `ws://<host>:8080/join?lobby=<id>&team=Blue&username=…&faction=…` —
  same JSON messages, unframed, one text message each. Available for development clients and protocol tests; the multiplayer UI
  uses WebTransport by default.
- **Lobby browser** `GET /api/lobbies` · `POST /api/lobbies {name, map, capacity}`
  (shape matches `src/net/lobbies.ts`), plus `GET /api/health`.

The server runs entirely in Go: rooms, WebTransport and authoritative simulation.
It does not start subprocesses and does not require Node.js, npm or the frontend
checkout. The Go simulation is the authoritative skirmish implementation: movement, melee/ranged
stances, combat, builders, construction, production queues, reinforcement with cost/time,
grenade/plasma upgrades, resources and income, point capture with listening posts,
research, demolish, transports/flyers (as movables), win detection and schema-2 snapshots.
Unknown commands are still explicitly rejected.

## Run

Requires Go only.

```sh
go run ./cmd/server                 # HTTP :8080 + WebTransport :4433
go run ./cmd/server --no-wt         # HTTP/WS only (local dev)
```

First start generates `certs/cert.pem` + `key.pem` and prints the wiring block.
The local certificate uses ECDSA P-256 and a lifetime under 14 days for browser
WebTransport hash pinning. Legacy, expired and near-expiry certificates are
replaced on startup; after replacement, update the client fingerprint.
The URL and fingerprint can also be entered in the multiplayer connection UI:

```js
// src/net/multiplayer.ts
SERVER_URL       = 'https://127.0.0.1:4433/grimcraft';
SERVER_CERT_HASH = '<printed sha-256>';
```

Lobby IDs are random eleven-character URL-safe strings (for example
`uyx9VVMaH9M`), generated with `crypto/rand`. Share the frontend link
`/server/<id>` to open that specific lobby directly.

## Two local browser sessions

1. Run this server (`go run ./cmd/server`, without `--no-wt`, which disables WebTransport).
2. In `../GrimCraft-main`, run `npm run dev` and open `http://localhost:3000`.
3. Open Multiplayer, wait for **GO SERVER · ONLINE**, then Create Game and double-click a map.
4. Open the same frontend URL in incognito, open Multiplayer and double-click the same lobby.
5. The first player is host and presses Start Game. Both browsers load the same room.

The client discovers WebTransport URL and the current certificate SHA-256 from
`GET http://127.0.0.1:8080/api/connection` (uncached), connects to QUIC/UDP `4433`,
and sends framed Join with username, faction and preferred team. Certificate
pinning remains enabled; no manual hash entry or certificate bypass is needed.
Run without `--no-wt`: a disabled WebTransport endpoint produces an explicit
error, with no automatic fallback to WebSocket.
`Lobby` messages replicate roster, host and started
state; `SetLobbyFaction` changes the sender's faction; `StartGame` is host-only
and may include computer opponents. Disconnects remove that player's squads;
late joiners receive the running room's snapshot. Simulation runs in Go at
10 ticks/s, with full snapshots and movement deltas.

Validation:

```sh
go test -race ./...
cd ../GrimCraft-main
npm test
npm run test:ts
```


## Layout

- `cmd/server` — entrypoint, flags, self-signed cert + hash print
- `internal/protocol` — wire types mirroring the JS client
- `internal/sim` — authoritative world (move/attack/reinforce, snapshot/delta)
- `internal/hub` — lobbies (one room each), join/team balance, command dispatch
- `internal/transport` — LE framing, WebSocket + WebTransport handlers
- `internal/server` — lobby HTTP API + tests
