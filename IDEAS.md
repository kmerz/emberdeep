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
- [x] [M] Ambient sound per theme, plus synthesized sound effects for every event

## 3. Foundation: items
- [x] [L] Rucksack with an inventory UI (12 slots; potions, draughts, firebombs, garnets)
- [x] [M] Chests with loot, and mimics (chests that bite)
- [x] [M] Torch item and light spell (Scroll of Light for everyone; Light is a Sorcerer skill too)

## 3b. Gear and loot
- [x] [L] Gear: a weapon and a trinket slot, common/magic/rare with 0-2 bonuses, class weapons, loot beams for rare drops
- [x] [M] Grimble sells a few rolled pieces per visit (answers "what do the gnomes trade?")
- [x] [S] Unique items: one hand-made trinket per boss (regen, thorns + endless lantern, slayer)
- [ ] [S] More uniques: rare world drops on deep floors, and one weapon per class
- [x] [M] Shrines: haste, fury, blood, fortune, or a gamble (Shrine of Chance)

## 3c. Run feel
- [ ] [M] Floor events: 1 in 4 floors gets something special (a treasure goblin, "the dead stir", a cursed floor with double gold)
- [ ] [M] Run summary: per-hero stats at the end (damage, kills, revives, times downed) and a funny title each
- [ ] [S] Hall of Fame: the best runs on the landing page
- [ ] [S] Death recap: "Downed by an Imp's firebolt, 34 dmg"
- [ ] [S] Pause menu for gamepads (so a pad can `/save`)
- [ ] [M] Pressure plates and secret walls

## 4. Progression
- [x] [XL] Skill tree (Diablo 2 style): 6 skills per class in 3 tiers, ranks 1-5, any 3 on keys 1-3, free reset at the stairs

## 5. Classes (added to Warrior, Rogue, Sorcerer, Monk; 7 in total)
- [ ] [M] Male and female variant for every class (cosmetic)
- [ ] [L] Dwarf: like the Warrior, plus a pickaxe in the inventory that digs through walls and up/down between floors
- [ ] [M] Monk rework: fights with a staff
- [ ] [L] Necromancer (kit still open)
- [ ] [XL] Druid: shapeshifts (bear to fight, mole to dig, unlocked by levelling), tames animals

## 6. Monsters
- [x] [M] Monster roles: bone rats (fast), bone hulks (slow, heavy), Bone Callers (summon rats)
- [x] [S] Blob (splits into two bloblings when it dies)
- [x] [L] Ant hive floor (floor 4): neutral workers, alarm within 12 cells, soldiers, a queen that lays soldiers
- [x] [L] Deep floors: Ember Dragon (final boss), Drakes, Imps, Magma Golems, Shades, Succubus, Incubus

## 7. Special floors
- [x] [M] Pitch-black floor (floor 7, The Black Hollow)
- [x] [XL] Gnome village after floor 5: gnomes, Grimble the trader, Scroll of Return portal
- [x] [S] Gnome Satchel in the village (+4 rucksack slots)
- [ ] [S] More traders and wares in the village

## 8. Quests
- [x] [L] Sidequests: notice board on floor 1 and Grimble; hunt, rescue, fetch, collect; shared rewards
- [ ] [M] More quests (the pool is in `internal/game/quests.go`)

## Suggested order
bot fix → `/leave` → multi-floor + themes → inventory + chests/mimics/torch → dark floor → monster roles + blob →
skill tree → new classes one at a time → ant hive → gnome village → deep monsters

## Open questions
- What does the Necromancer do? (summons, curses, life drain?)
- Does digging down skip a floor, or lead to a hidden area?
- Do tamed animals count as party members (slots, shared XP)?
- What do the gnomes trade? (gold for potions and gear?)
- Male/female: just looks, or small stat differences?
