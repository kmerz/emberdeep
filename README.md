# Emberdeep: co-op for 1-4 players

A server-authoritative terminal-style dungeon crawler. The Go server runs the game; the browser client
(TypeScript, one `<canvas>`, a 160x48 cell grid) draws it exactly like the boards in `design/`.

## Run it

```sh
cd web && npm install && npm run build && cd ..   # builds the client and copies it to internal/web/dist
go run ./cmd/emberdeep                            # http://localhost:8080
```

Open the page, **create a game** (or join with a code such as `ASH-7Q2`, or open `/r/ASH-7Q2`), pick a name and class, press
`ENTER` to ready up. The run starts when everyone is ready, or when the host presses `ENTER` again.

### Controls

| Key | Action |
|---|---|
| WASD / arrows | move (hold two for diagonals) |
| `1` `2` `3` | class skills, aimed at the mouse cursor (or the nearest enemy / facing) |
| left click | skill 1 |
| `Q` | potion |
| hold `E` | revive an adjacent downed ally (3 s) |
| `ENTER` | chat (`ENTER` sends, `Esc` cancels; `/p text` party chat, `/help`) |
| Alt+click or middle click | ping the map (4 s) |
| lobby: `ENTER` `C` `I` | ready, change class, copy invite link (host: `F` friendly fire, `D` difficulty) |

### Classes

| Class | Resource | Skills |
|---|---|---|
| Warrior | Rage | Cleave, Iron Wall (absorb + threat), Leap (shock ring, shake) |
| Rogue | Energy | Stab (x2 from behind), Caltrops (slow + bleed), Shadow Bomb (Void Rift) |
| Sorcerer | Mana | Spark, Ember Nova, Blink |
| Monk | Ki | Palm Strike (knockback), Dash, Bell of Warding (-30% damage, 12 s) |

All numbers live in `internal/game/balance.go`.

### Rules worth knowing

- Scaling uses the party size **at run start**: enemy HP x `1+0.6(N-1)`, elite HP x `1+0.8(N-1)`, packs `+N-1`.
  Heroes joining mid-run do **not** rescale enemies that already exist.
- Gold is split between connected heroes; potion drops are personal (only their owner sees and picks them up).
- At 0 HP a hero is downed for 20 s; if nobody revives them they die and spectate. The party wipes when no
  connected hero is standing (heroes held for a reconnect are idle and invulnerable, so they don't count).
- Descend by having every living, connected hero on or next to `>` for 3 s. The run then ends ("DEPTH CLEARED")
  and the room returns to the lobby with a new seed. After a wipe the host's `ENTER` restarts with the same seed.
- A disconnected hero stays for 60 s; reconnecting with the same browser (token in `localStorage`) resumes it.
  Rooms that stay empty for 5 minutes are closed.

## Server flags and environment

| Flag / env | Meaning |
|---|---|
| `-addr` / `ADDR` | listen address (default `:8080`) |
| `MAX_ROOMS` | maximum concurrent rooms (default 200) |
| `PUBLIC_URL` | public origin, used for invite links and the websocket Origin check |
| `-bots N` | fill N slots of each new room with AI teammates (1-3) |
| `-lag ms`, `-jitter ms` | artificial one-way latency in both directions (never reorders messages) |
| `-dev` | also accept the Vite dev origin `http://localhost:5173` |

## Development

```sh
go run ./cmd/emberdeep -dev -bots 3     # terminal 1
cd web && npm run dev                   # terminal 2: http://localhost:5173 (proxies /ws and /api)
cd web && npm test                      # client logic and FX tests
go build ./... && go vet ./... && go test -race -short ./...
go test -race ./...                     # also plays 4 headless bots for 60 s
go run ./cmd/loadtest                   # 50 rooms x 4 headless clients, prints tick p50/p99
```

Architecture in one paragraph: a `Hub` maps room codes to rooms; each `Room` runs in its own goroutine and exclusively
owns its deterministic `internal/game` simulation (no goroutines, clocks or I/O in that package). Websocket readers push
intents into the room's channel (token bucket: 30/s); the room pushes marshalled messages into each connection's
outbox: events are queued, snapshots occupy a single slot so a slow client only ever sees the latest one.
Anything a client must not miss (explored tiles, chat, lobby, level data) travels as events, never in snapshots.

## Deploy

One static binary, or the multi-stage `Dockerfile` (node build, go build, distroless):

```sh
docker build -t emberdeep .
docker run -p 8080:8080 -e PUBLIC_URL=https://play.example.com emberdeep
```

The server speaks plain HTTP. Put it behind a TLS reverse proxy so the websocket runs over `wss`; see
`Caddyfile.example` (`play.example.com { reverse_proxy emberdeep:8080 }`: Caddy gets the certificate and
forwards the upgrade). Set `PUBLIC_URL` to the public origin so invite links and the Origin check match.
