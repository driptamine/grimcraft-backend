# grimcraft-go — authoritative multiplayer backend for Grimcraft RTS

Go server for the [GrimCraft-main](../GrimCraft-main) frontend
(`src/net/multiplayer.ts`). One process hosts **many lobbies, one room (game) each**.

## Protocol (matches `src/net/multiplayer.ts`)

- **WebTransport** `https://<host>:4433/grimcraft?lobby=<id>` — what the game uses.
  Client opens one bidi stream, writes 4-byte LE length-prefixed JSON frames:
  `{"Join":{"preferred_team":…}}`, then `{"Command":{…}}`.
  Server answers on a uni stream with framed
  `{"Session"…}`, `{"Snapshot"…}`, `{"Delta"…}`, `{"CmdAck"…}`.
- **WebSocket** `ws://<host>:8080/join?lobby=<id>&team=Blue&username=…&faction=…` —
  same JSON messages, unframed, one text message each. For local dev, bots, tests.
- **Lobby browser** `GET /api/lobbies` · `POST /api/lobbies {name, map, capacity}`
  (shape matches `src/net/lobbies.ts`), plus `GET /api/health`.

Simulation: 10 ticks/s authoritative movement + forced-target combat,
full `Snapshot` 1/s, `Delta` (moved `squad_pos`/`squad_facing`) between.
Movement/combat/reinforce commands mutate the minimal simulation.
Economy/building commands currently acknowledge without applying an action;
not every command in the current frontend is implemented. This is a transport
and simulation prototype, not a complete multiplayer version of the game.
See [the integration review](INTEGRATION_REVIEW.md) for tested behavior and priorities.

## Run

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

## Layout

- `cmd/server` — entrypoint, flags, self-signed cert + hash print
- `internal/protocol` — wire types mirroring the JS client
- `internal/sim` — authoritative world (move/attack/reinforce, snapshot/delta)
- `internal/hub` — lobbies (one room each), join/team balance, command dispatch
- `internal/transport` — LE framing, WebSocket + WebTransport handlers
- `internal/server` — lobby HTTP API + tests
