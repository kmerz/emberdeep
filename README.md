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

While you wait in the lobby, the legend of Emberdeep is told page by page under the lobby box (`←` `→` turn the
pages). The text lives in `web/src/story.ts`.

### Controls

| Key | Action |
|---|---|
| WASD / arrows | move (hold two for diagonals) |
| `1` `2` `3` | class skills, aimed at the mouse cursor (or the nearest enemy / facing) |
| left click | skill 1 |
| walk into an enemy | a free weapon strike (also what `1` `2` `3` do when you lack the rage, energy, mana or ki: you can always fight back) |
| `Q` | drink a healing potion, or a Resource Draught when your HP is fine (70%+) and your resource is low (or you have no potions left) |
| `4` `5` | use the item on that key: firebombs and Scrolls of Light are thrown at the mouse cursor (or the nearest enemy). In the rucksack, `4` or `5` on an item puts it on that key; right-click puts it on `4` (again: `5`) |
| `T` | open the skill tree (arrows pick, `ENTER` learn or rank up, `1` `2` `3` put the selected skill on that key, `R` reset at the stairs). Learned skills fill an empty key by themselves |
| `J` | the quest journal |
| `I` | open the rucksack (arrows pick a slot, `U` or `ENTER` use or wear, `X` drop, `4`/`5` put on that key, `I`/`Esc` close). The two rows on top are your worn weapon and trinket: `ENTER` there takes it off |
| hold `E` | revive an adjacent downed ally (3 s) |
| `ENTER` | chat (`ENTER` sends, `Esc` cancels; `/p text` party chat, `/save` save and go back to the lobby (host), `/leave` or `/quit` leave the game, `/help`) |
| `G` | ping where you aim (the enemy in front of you, or ahead) |
| `TAB` | in the rucksack, skill tree or journal: flip to the next one (`SHIFT+TAB` back). In the world: the map in the zoomed view |
| `Esc` | the game menu: resume, journal, sound, controls (rebind keys and buttons), save and go back to the lobby (host), leave (press twice). The game keeps running behind it |
| Alt+click or middle click | ping the map (4 s) |
| `M` | music and sound → sound only → off (remembered in this browser). Every floor has its own generated music: the theme sets the mood, deeper floors play faster, and drums come in when enemies are near |
| lobby: `ENTER` `C` `I` | ready, change class, copy invite link (host: `F` friendly fire, `D` difficulty, `B` add bot, `X` remove bot, `N` twice: drop the save for a new run) |

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

With a gamepad (Diablo-style: fighting stays on the face buttons, and the right stick aims):

| Button | In the world | In panels |
|---|---|---|
| left stick / d-pad | move | choose |
| right stick | aim: skills and throws go at the enemy in that direction, or that way | |
| `A` | skill 1; hold it next to a downed ally to revive them | confirm |
| `X` `Y` | skills 2 and 3 | `X` drop / reset the tree, `Y` put on a key |
| `B` | ping where you aim | back |
| `LB` | potion | previous tab |
| `RB` `RT` | items on keys 4 and 5 | `RB` next tab |
| `LT` | map (zoomed view) | |
| `SELECT` | rucksack; `LB`/`RB` flip to the skill tree and journal | close |
| `START` | menu | close |

Without the right stick (and without a mouse cursor), skills aim at the closest enemy in the direction you face,
and keep to the one you last hit while it is still in front of you. A highlighted cell shows what they will hit.
On-screen hints show pad buttons as soon as you press one.

**Rebinding:** `Esc` (or `START`) → Controls lists every action with two keyboard keys and a gamepad button.
Pick a cell, press `ENTER` (or `A`), then the new key or button; a key that belonged to another action swaps
with it. `DEL` or `X` clears a cell, and two rows reset the keyboard or the gamepad to the defaults. The menu
itself always stays on `Esc` and `START`. Bindings are kept in this browser (`web/src/bindings.ts`).

### Phones and tablets

Touch the screen and on-screen controls appear (a key press or gamepad hides them again):

- **left thumb**: a stick that appears where you touch the left part of the screen (8 directions; in panels it moves the selection)
- **right thumb**: skills 1 2 3 (with cooldowns), `!` potion, items 4 and 5, `✚` revive (hold)
- **tap the map** to aim skills and thrown items there for 4 s (otherwise they aim at the nearest enemy); **hold** a finger on the map to ping
- **top row**: BAG (rucksack), TREE, QST (journal), MAP, SAY (chat, opens the phone keyboard), `☰` the game menu (sound, save, leave), `⛶` fullscreen
- panels get OK, BACK and their own keys (DROP, USE, → 4, RESET, ...). Tap a row or slot to pick it, tap it again to
  use, learn, buy, sell or accept it (this works with the mouse too); tap a field on the start screen to type your name or a code

On phones the game shows a zoomed view: the map around your hero with a two-line HUD on top, and each panel on its own
(MAP shows the minimap and objectives). Tablets keep the full screen. Play in landscape; on an iPhone use
"Add to Home Screen" to play without the browser bars. To try the phone layout on a desktop, open `/?layout=compact`
(`?layout=full` forces the full grid, `?layout=auto` goes back to detecting it).

### Saving

The game saves by itself on every new floor, when a player leaves, and when the server stops. The host can
also type `/save` to save and take everyone back to the lobby. A save keeps the floor, the quests and every
hero by player name; continuing builds the floor fresh, and heroes keep their level, gold, rucksack and
skills. Join with the same name and class to get your hero back. Someone new starts at the party's level, and
heroes whose player isn't there wait in the save. The invite link keeps working after a server restart.
Saves untouched for 30 days are deleted.

### Rules worth knowing

- Difficulty (host: `D` in the lobby): NORMAL; NIGHTMARE (monsters hit 50% harder); NOVICE (monsters hit for
  60% and have 70% HP, for learning the game). Bots count toward party scaling like players.
- Scaling uses the party size **at run start**: enemy HP x `1+0.6(N-1)`, elite HP x `1+0.8(N-1)`, packs `+N-1`.
  Heroes joining mid-run do **not** rescale enemies that already exist.
- Gold is split between connected heroes. Item drops (healing potions, resource draughts, firebombs, trade goods) are
  personal: only their owner sees and picks them up. Each hero has a 12-slot rucksack (items stack); an item
  dropped from the rucksack can be picked up by anyone, which is how you hand things to a teammate.
- Gear: each hero wears a weapon and a trinket (amulet, charm or talisman). Gear is a personal drop, rolled for
  your class (axes and swords, daggers, wands and rods, staves and knuckles): about 1 kill in 40, a third of
  chests, a rare for everyone from every boss and every vault. Rarity sets the bonuses: common (none), magic
  (one, blue), rare (two, gold, with a pillar of light and a chime when it drops). A weapon adds damage; the
  bonuses are damage, crit chance, life steal, shorter cooldowns, resource gain, armor and max HP (numbers in
  `internal/game/gear.go`). Gear goes straight on when the slot is empty, else into the rucksack, where the panel
  compares it with what you wear. Grimble buys gear; bots wear whatever is better.
- Uniques (orange): each boss also drops its own trinket for every hero, with an effect random gear never has.
  The Ant Queen's Royal Jelly Locket regenerates HP; the Tallow King's Wick deals 40% of damage taken back and is
  a lantern that never runs out; the Heart of the Ember Dragon adds damage against bosses and hunt targets.
  From floor 6 down, any gear drop has a 4% chance to be a world unique instead: one weapon per class
  (Grudgebearer, The Last Whisper, The Ninth Ember, Pilgrim's Knuckles) or a trinket (Gravedigger's Coin: more
  gold; The Glass Eye: more damage to badly hurt monsters). A hero never finds a second copy of one they have.
- Floor 7, The Black Hollow, is pitch black: you see only what a light reaches. Every hero carries a lantern
  that lights itself there and burns oil only on dark floors (5 minutes to start, 10 when full; the hotbar shows
  what is left). Lamp oil drops from monsters and chests and goes straight into the lantern, never into the
  rucksack; Grimble sells it too. A Scroll of Light lights the spot at the cursor for a minute, for everyone.
- Every floor below the first has stairs up (`<`, a few steps from where you arrive). Gather the party there
  like at the stairs down. Floors remember what happened: the dead stay dead, opened chests stay open and the
  map stays explored. Up from floor 6 is the gnome village, and floor 5's stairs always lead to it.
- Story: every floor has an inscription (`¶`). Walk into it to read a page of what happened to Ashfall; the
  pages you've read are kept in the journal (`J`).
- Gods: each class has a patron among the Four (Korrath the Anvil, Vess Who Walks Unseen, Ilun of the
  Burning Eye, Sabe the Still Water). At levels 4, 7 and 10 your god sets a trial before you can level up:
  slay 10 / 18 / 26 foes, then pray at an altar (`Ω`, one on every floor below the first and in the village).
  XP earned meanwhile is kept, so the blessing grants every level it held back, and heals you. Bots pray from
  wherever they are. Players who join a run late skip the trials below their level.
- Shrines (`¥`): from floor 2, most floors have one. Walk into it once for the whole party: Haste (faster steps),
  Fury (+30% damage) or Blood (+12% life steal) for 45 s; Fortune (gold on this floor counts double); or Chance
  (a rare for everyone, a full heal, or an ambush). Then it goes dark.
- Floor events: about one floor in four (never floor 1, a boss floor or the last one) has something special, told
  when you arrive and shown under the objectives. A **Treasure Goblin** (`T`) hides far from the stairs; once it sees
  the party it runs and never fights, and escapes after 30 s. Caught, it spills a big pile of gold and a rare for
  everyone. On a floor where **the dead stir**, zombies and skeletons claw their way up near the party every 25 s
  (no loot, a little XP). A **cursed** floor doubles the gold but monsters hit 25% harder. The event comes from the
  run seed, so a retry or a return trip finds the same one (`internal/game/events.go`).
- Vaults: from floor 2, one dead-end room may be locked (`▓`). An elite on the floor carries the key (or it lies
  in a far room); walk into the door with it. The vault chest holds extra gold, items, a Silver Ring and a rare piece of gear each.
  Keys only fit their own floor.
- Trade goods are only for selling: Old Coin (15 gold), Garnet (40), Silver Ring (80) and the rare Gold Idol
  (200). Picking one up says what Grimble pays for it.
- Monsters differ: bone rats are faster than you but fragile, bone hulks are slow and hit hard, a Bone Caller
  hangs back and raises bone rats (up to 3 at a time, no loot from them), and a blob splits into two
  bloblings when it dies. Each theme has its own mix.
- Floor 4 is The Hive. Worker ants go about their business and leave you alone, but hitting any ant alarms
  every ant within 12 cells. Soldier ants are always hostile, and the Ant Queen in the deepest chamber lays a
  soldier every 6 s (up to 4 at a time) while she sees you.
- After floor 5 the stairs lead to the Gnome Village, a safe extra floor between 5 and 6. Walk into Grimble to
  trade: he sells potions, draughts, lamp oil, Scrolls of Light, firebombs, Scrolls of Return and a Gnome Satchel
  (+4 rucksack slots, 200 gold, one per hero), and buys
  anything (trade goods for their value, everything else for half its price). Everyone shops with their own gold.
  Each visit he also has three pieces of gear for each hero (magic or rare, for your class and the next floor),
  at twice what he would pay for them.
- Bramwick the smith has a forge on the plaza, west of Grimble. Walk into him to work on the gear you wear:
  temper it (one rarity up, common → magic → rare: it keeps its bonuses and gains one; costs what the tempered
  piece is worth) or reforge it (its bonuses are rolled again; half its value). He won't touch uniques
  (`internal/game/forge.go`).
- A Scroll of Return, read anywhere below the village, opens a portal for 30 s. When the whole party stands in it
  for 3 s you go back to the village; its stairs then lead back down to a fresh copy of the floor you left.
- Deep monsters: Shades (floors 6-8) stay invisible until they're within 3 cells. Floor 8, the Abyss Gate, adds
  the Succubus (charms a hero for 2 s: no actions, drifting toward her) and the Incubus (drains resource to heal
  itself, blinks away when hit). The Ember Depths (9-10) add Imps (fast, shoot firebolts, keep away), Drakes
  (spit fire, bite up close) and Magma Golems (slow, tough, leave burning ground). Floor 10's boss is the Ember
  Dragon: it winds up for a second before breathing a cone of fire that keeps burning, so step out of it.
- Sidequests: the mayor's notice board `≡` in floor 1's first room and Grimble in the village (TAB to his
  errands) each offer 3 quests per run, picked from 8 each: hunt a named monster, rescue a lost gnome (walk into them, then lead them
  to the stairs; the stairs wait for them), fetch an item, or collect a few things. Quests are shared: anyone
  accepts, everyone gets the gold and XP, and some give each hero a skill point. Fetch and collect quests are
  handed in by walking into Grimble. `J` opens the journal. The quests live in `internal/game/quests.go`.
- The rooms are dressed per theme (numbers in `Dressings` in `balance.go`, logic in `internal/game/world.go`):
  candle stands `†` (they give light), coffins `Π` and giant mushroom stalks `♠` stand in the way like pillars.
  Urns `∪` break when you walk into them (sometimes a little gold, rarely an item). A powder keg `¤` explodes
  0.4 s after any hit: it hurts monsters (credited to whoever lit it), heroes in the blast, and sets off other
  kegs nearby. From floor 2 on, hidden spike traps show `×` when a hero steps next to one; they hurt whatever
  steps on them (monsters too) and reset after 3 s. The Abyss Gate and the Ember Depths have lava pools that
  never stop burning. Bots walk around seen traps and burning ground.
- Monsters talk (`internal/game/barks.go`): bosses and hunt targets always greet the party, speak at half health
  and when they down a hero (also in the log); other monsters sometimes, mimics when they bite, Bone Callers when
  they raise rats. Gnomes chat with heroes walking by, and a rescued gnome frets on the way to the stairs.
- About a third of the rooms hold a chest `■`: walk into it to open it. It spills gold for the party and two
  item rolls for every hero. From floor 2 on, some chests are mimics (8% rising to 38% at floor 10): they bite
  when bumped, and hitting a chest gives a mimic away. A dead mimic drops the treasure it was guarding.
- At 0 HP a hero is downed for 20 s; if nobody revives them they die and spectate. The party wipes when no
  connected hero is standing (heroes held for a reconnect are idle and invulnerable, so they don't count).
- Descend by having every living, connected hero on or next to `>` for 3 s. A run is 10 floors deep (`MaxDepth`);
  level, XP, gold and potions carry over, and dead or downed heroes stand up on the next floor at 50% HP. Enemies get
  tougher with depth, and a boss guards every 5th floor. The stairs on floor 10 end the run ("DEPTH CLEARED") and
  the room returns to the lobby with a new seed, after 20 s or when the host presses `ENTER`.
- The wipe and cleared screens show the run so far for every hero: kills, damage dealt and taken, times downed,
  revives, potions, gold, best hit and what downed them last, with a title each ("the Butcher", "the Lifeline",
  "the Floor Inspector", ...). The stats survive a retry and a save (`internal/game/stats.go`). The log also
  says what downed a hero: "vela is DOWN! · an Imp's firebolt · 34 dmg".
- Hall of Fame: the landing page lists the best 8 runs that reached the bottom on this server, ranked by
  difficulty, then fewest wipes, then play time (over every session of a saved run). It is kept in
  `hall-of-fame.json` in the save directory (`GET /api/fame`); runs of bots alone do not count.
- Floors come in four themes, each with its own look, light and monster mix: Cathedral of Ash (1-2), Flooded
  Crypt (3-5), Fungal Caverns (6-8, cave-shaped rooms, glowing fungus) and Ember Depths (9-10). The bands live in
  `ThemeBands` in `balance.go`; the client palettes in `web/src/render/themes.ts`.
- Sound is synthesized in the browser (`web/src/audio.ts`, no sample files): every hit, skill, pickup and death
  has a short effect, each theme has an occasional soft chord swell and a few ambient noises, and a heartbeat plays below 25% HP.
  Dead monsters burst into shards and leave remains (bones, slime, ash) on the floor, heavy hits splatter, and
  kills less than 1.5 s apart pop a streak ("DOUBLE KILL", "MASSACRE ×5"). All of this is client-only.
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
| `-saves dir` / `SAVE_DIR` | folder for saved games (default `saves`; empty = saves last only while the room is open) |
| `-version` | print the version and exit |

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
docker run -p 8080:8080 -v emberdeep-data:/data -e PUBLIC_URL=https://play.example.com emberdeep
```

Saved games go to `/data/saves` in the container; the volume keeps them across restarts and upgrades.
Stop the container normally (`docker stop` sends SIGTERM) so running games are saved before it exits.

The server speaks plain HTTP. Put it behind a TLS reverse proxy so the websocket runs over `wss`; see
`Caddyfile.example` (`play.example.com { reverse_proxy emberdeep:8080 }`: Caddy gets the certificate and
forwards the upgrade). Set `PUBLIC_URL` to the public origin so invite links and the Origin check match.
