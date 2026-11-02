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
| `T` | open the skill tree (arrows pick, `ENTER` learn or rank up, `1` `2` `3` put the skill on that key, `R` reset at the stairs) |
| `I` | open the rucksack (arrows pick a slot, `U` or `ENTER` use, `X` drop, `I`/`Esc` close) |
| hold `E` | revive an adjacent downed ally (3 s) |
| `ENTER` | chat (`ENTER` sends, `Esc` cancels; `/p text` party chat, `/leave` or `/quit` leave the game, `/help`) |
| Alt+click or middle click | ping the map (4 s) |
| lobby: `ENTER` `C` `I` | ready, change class, copy invite link (host: `F` friendly fire, `D` difficulty, `B` add bot, `X` remove bot) |

### Classes

Each class has a skill tree of 6 skills in 3 tiers. Tier 1 is open from level 1, tier 2 from level 3, tier 3
from level 8, and a skill of a lower tier needs a skill of the tier above it first. Each level gives 1 point to
learn a skill or raise its rank (up to 5: +15% damage, +10% duration, -5% cooldown per rank). Any 3 learned
skills go on keys 1-3. You start with the first skill and 1 point; the tree resets every run, and standing at
the stairs lets you reset it for free.

| Class | Resource | Tier 1 | Tier 2 | Tier 3 |
|---|---|---|---|---|
| Warrior | Rage | Cleave · Shield Bash (stun) | Iron Wall (absorb + threat) · War Cry (taunt, party +20% damage) | Leap (shock ring) · Whirlwind |
| Rogue | Energy | Stab (x2 from behind) · Poison Blade | Caltrops (slow + bleed) · Smoke Step (vanish) | Shadow Bomb · Fan of Knives |
| Sorcerer | Mana | Spark · Frost Ring (slow) | Ember Nova · Light (orb at the cursor) | Blink · Chain Spark |
| Monk | Ki | Palm Strike (knockback) · Staff Sweep (reach 2) | Dash · Healing Breath | Bell of Warding · Hundred Fists |

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
- Monsters differ: bone rats are faster than you but fragile, bone hulks are slow and hit hard, a Bone Caller
  hangs back and raises bone rats (up to 3 at a time, no loot from them), and a blob splits into two
  bloblings when it dies. Each theme has its own mix.
- Floor 4 is The Hive. Worker ants go about their business and leave you alone, but hitting any ant alarms
  every ant within 12 cells. Soldier ants are always hostile, and the Ant Queen in the deepest chamber lays a
  soldier every 6 s (up to 4 at a time) while she sees you.
- After floor 5 the stairs lead to the Gnome Village, a safe extra floor between 5 and 6. Walk into Grimble to
  trade: he sells potions, draughts, torches, Scrolls of Light, firebombs, Scrolls of Return and a Gnome Satchel
  (+4 rucksack slots, 200 gold, one per hero), and buys
  anything (garnets for 40 gold, everything else for half its price). Everyone shops with their own gold.
- A Scroll of Return, read anywhere below the village, opens a portal for 30 s. When the whole party stands in it
  for 3 s you go back to the village; its stairs then lead back down to a fresh copy of the floor you left.
- Deep monsters: Shades (floors 6-8) stay invisible until they're within 3 cells. Floor 8, the Abyss Gate, adds
  the Succubus (charms a hero for 2 s: no actions, drifting toward her) and the Incubus (drains resource to heal
  itself, blinks away when hit). The Ember Depths (9-10) add Imps (fast, shoot firebolts, keep away), Drakes
  (spit fire, bite up close) and Magma Golems (slow, tough, leave burning ground). Floor 10's boss is the Ember
  Dragon: it winds up for a second before breathing a cone of fire that keeps burning, so step out of it.
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
