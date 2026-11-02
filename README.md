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
| `Q` | drink a healing potion |
| `I` | open the rucksack (arrows pick a slot, `U` or `ENTER` use, `X` drop, `I`/`Esc` close) |
| hold `E` | revive an adjacent downed ally (3 s) |
| `ENTER` | chat (`ENTER` sends, `Esc` cancels; `/p text` party chat, `/leave` or `/quit` leave the game, `/help`) |
| Alt+click or middle click | ping the map (4 s) |
| lobby: `ENTER` `C` `I` | ready, change class, copy invite link (host: `F` friendly fire, `D` difficulty, `B` add bot, `X` remove bot) |

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
- Gold is split between connected heroes. Item drops (healing potions, resource draughts, firebombs, garnets) are
  personal: only their owner sees and picks them up. Each hero has a 12-slot rucksack (items stack); an item
  dropped from the rucksack can be picked up by anyone, which is how you hand things to a teammate.
- Floor 7, The Black Hollow, is pitch black: you see only what a light reaches. Everyone starts with 2 torches
  (`I`, then use one: it burns for 2 minutes and doesn't survive the stairs), and a Scroll of Light lights the
  spot at the cursor for a minute, for the whole party. Torches and scrolls also drop from monsters and chests.
- About a third of the rooms hold a chest `■`: walk into it to open it. It spills gold for the party and two
  item rolls for every hero. From floor 2 on, some chests are mimics (8% rising to 38% at floor 10): they bite
  when bumped, and hitting a chest gives a mimic away. A dead mimic drops the treasure it was guarding.
- At 0 HP a hero is downed for 20 s; if nobody revives them they die and spectate. The party wipes when no
  connected hero is standing (heroes held for a reconnect are idle and invulnerable, so they don't count).
- Descend by having every living, connected hero on or next to `>` for 3 s. A run is 10 floors deep (`MaxDepth`);
  level, XP, gold and potions carry over, and dead or downed heroes stand up on the next floor at 50% HP. Enemies get
  tougher with depth, and a boss guards every 5th floor. The stairs on floor 10 end the run ("DEPTH CLEARED") and
  the room returns to the lobby with a new seed.
- Floors come in four themes, each with its own look, light and monster mix: Cathedral of Ash (1-2), Flooded
  Crypt (3-5), Fungal Caverns (6-8, cave-shaped rooms, glowing fungus) and Ember Depths (9-10). The bands live in
  `ThemeBands` in `balance.go`; the client palettes in `web/src/render/themes.ts`.
- After a wipe the host's `ENTER` retries the same floor, with every hero as they arrived on it.
- A disconnected hero stays for 60 s; reconnecting with the same browser (token in `localStorage`) resumes it.
  Rooms that stay empty for 5 minutes are closed.

## Server flags and environment

| Flag / env | Meaning |
|---|---|
| `-addr` / `ADDR` | listen address (default `:8080`) |
| `MAX_ROOMS` | maximum concurrent rooms (default 200) |
| `PUBLIC_URL` | public origin, used for invite links and the websocket Origin check |
| `-depth N` | dev: start every run on floor N (to look at deeper floors and themes) |
| `-bots N` | fill N slots of each new room with AI teammates (1-3); joining players replace them |
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
