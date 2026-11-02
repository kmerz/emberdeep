# Emberdeep ideas

The backlog. Sizes: S = an hour or two, M = a session, L = a few sessions, XL = a milestone.

Today a run is **one floor** (reaching `>` ends it with "DEPTH CLEARED"), and the only items are gold `$` and
potions `!`. Multi-floor runs and an inventory come first because most other ideas need them.

## 1. Now / small
- [x] [S] Host can add and remove bots in the lobby (`B` / `X`)
- [x] [S] `/leave` or `/quit` chat command: leave cleanly, go back to the landing page, free the slot at once (no 60 s hold)

## 2. Foundation: depth
- [x] [L] Multi-floor runs: `>` takes the party down. 10 floors first, data-driven so it can grow to 30.
      Gold, XP and items carry over.
- [x] [M] Per-floor themes: palette, light level, tile glyphs, which monsters spawn, cave layouts for the caverns
- [ ] [M] Ambient sound per theme (needs a small audio system first)

## 3. Foundation: items
- [x] [L] Rucksack with an inventory UI (12 slots; potions, draughts, firebombs, garnets)
- [x] [M] Chests with loot, and mimics (chests that bite)
- [x] [M] Torch item and light spell (a Scroll of Light for now; could become a Sorcerer skill with the skill tree)

## 4. Progression
- [ ] [XL] Skill tree on level-up (Diablo 2 style): skill points, unlockable nodes, any learned skill can go on 1/2/3

## 5. Classes (added to Warrior, Rogue, Sorcerer, Monk; 7 in total)
- [ ] [M] Male and female variant for every class (cosmetic)
- [ ] [L] Dwarf: like the Warrior, plus a pickaxe in the inventory that digs through walls and up/down between floors
- [ ] [M] Monk rework: fights with a staff
- [ ] [L] Necromancer (kit still open)
- [ ] [XL] Druid: shapeshifts (bear to fight, mole to dig, unlocked by levelling), tames animals

## 6. Monsters
- [ ] [M] Monster roles: some slower, some faster, a few that spawn others
- [ ] [S] Blob (maybe splits when hit)
- [ ] [L] Ant hive floor: workers ignore you unless attacked, warriors, a queen
- [ ] [L] Deep floors: dragons, incubus, succubus

## 7. Special floors
- [x] [M] Pitch-black floor (floor 7, The Black Hollow)
- [ ] [XL] Gnome village further down: friendly gnomes, trading (garnets are worth gold there), a town spell to come back later

## Suggested order
bot fix → `/leave` → multi-floor + themes → inventory + chests/mimics/torch → dark floor → monster roles + blob →
skill tree → new classes one at a time → ant hive → gnome village → deep monsters

## Open questions
- What does the Necromancer do? (summons, curses, life drain?)
- Does digging down skip a floor, or lead to a hidden area?
- Do tamed animals count as party members (slots, shared XP)?
- What do the gnomes trade? (gold for potions and gear?)
- Male/female: just looks, or small stat differences?
