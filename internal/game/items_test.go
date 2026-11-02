package game

import "testing"

func floorItem(g *Game, id string, x, y, n, owner int) *Item {
	it := &Item{ID: g.newID(), Kind: Items[id].Glyph, Def: id, X: x, Y: y, Amount: n, Owner: owner}
	g.Items = append(g.Items, it)
	return it
}

func step(t *testing.T, g *Game, h *Hero, dir int) {
	t.Helper()
	h.NextMove = 0
	if err := g.Apply(h.ID, Intent{Kind: IntMove, Dir: dir}); err != nil {
		t.Fatal(err)
	}
}

func TestStartingKitAndStacking(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	if h.Count("potion") != StartPotions || h.PackUsed() != 1 || h.Oil != StartOil {
		t.Fatalf("expected %d potions in one slot and a lantern with oil, got %d in %d slots, oil %d", StartPotions, h.Count("potion"), h.PackUsed(), h.Oil)
	}
	if n := h.AddItem("potion", 10); n != 10 || h.PackUsed() != 2 || h.Pack[0].N != 9 {
		t.Fatalf("12 potions should fill one stack of 9 and start another: added %d, %+v", n, h.Pack[:2])
	}
	if h.AddItem("nothing", 1) != 0 {
		t.Fatal("unknown items must not fit")
	}
}

func TestFullRucksackLeavesTheRestOnTheFloor(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 40, 25)
	for i := 1; i < PackSlots; i++ {
		h.Pack[i] = Stack{Item: "gem", N: Items["gem"].MaxStack}
	}
	h.Pack[0] = Stack{Item: "potion", N: 7}
	it := floorItem(g, "potion", 41, 25, 5, 0)
	step(t, g, h, 2)
	if h.Pack[0].N != 9 || it.Amount != 3 || len(g.Items) != 1 {
		t.Fatalf("2 should fit and 3 stay on the floor: slot %+v floor %d", h.Pack[0], it.Amount)
	}
	evs := g.Tick()
	full := false
	for _, e := range evs {
		full = full || (e.K == "packfull" && e.Actor == 1)
	}
	if !full {
		t.Fatal("expected a rucksack-full event")
	}
}

func TestPotionAndDraughtShareTheCooldown(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 40, 25)
	h.HP, h.Res = 10, 0
	h.AddItem("draught", 1)
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != nil {
		t.Fatal(err)
	}
	if h.HP <= 10 || h.Count("potion") != StartPotions-1 {
		t.Fatalf("Q should drink a potion from the rucksack: HP %d potions %d", h.HP, h.Count("potion"))
	}
	ds := h.firstSlot("draught")
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: ds}); err != ErrCooldown {
		t.Fatalf("the draught shares the potion cooldown, got %v", err)
	}
	g.Now += PotionCD
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: ds}); err != nil {
		t.Fatal(err)
	}
	if h.Res != h.ResMax/2 || h.Count("draught") != 0 {
		t.Fatalf("the draught refills half the resource: %d/%d", h.Res, h.ResMax)
	}
}

func TestFirebomb(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Rogue, 30, 25)
	h.AddItem("firebomb", 2)
	slot := h.firstSlot("firebomb")
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot, X: 58, Y: 25}); err != ErrRange {
		t.Fatalf("too far: got %v", err)
	}
	m := g.spawnMon('z', 38, 25)
	near := g.spawnMon('z', 39, 26)
	far := g.spawnMon('z', 50, 25)
	for _, mm := range []*Monster{m, near, far} {
		mm.StunUntil = 1 << 30 // hold still while the bomb flies
	}
	hp, hpFar := m.HP, far.HP
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot, X: 38, Y: 25}); err != nil {
		t.Fatal(err)
	}
	if h.Count("firebomb") != 1 || m.HP != hp {
		t.Fatal("one bomb used, no damage before it lands")
	}
	for i := 0; i < ThrowDelay+1; i++ {
		g.Tick()
	}
	if m.HP >= hp || near.HP >= near.HPMax || far.HP != hpFar {
		t.Fatalf("blast should hit monsters in the radius only: m %d/%d near %d far %d", m.HP, hp, near.HP, far.HP)
	}
}

func TestGemsCannotBeUsed(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Monk, 40, 25)
	h.AddItem("gem", 1)
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: h.firstSlot("gem")}); err != ErrBadIntent {
		t.Fatalf("got %v", err)
	}
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: 11}); err != ErrNoItem {
		t.Fatalf("empty slot: got %v", err)
	}
}

func TestDropHandsItemsToTeammates(t *testing.T) {
	g := arena(2)
	a := hero(g, 1, Warrior, 40, 25)
	b := hero(g, 2, Rogue, 38, 25)
	a.AddItem("gem", 3)
	if err := g.Apply(1, Intent{Kind: IntDrop, Slot: a.firstSlot("gem")}); err != nil {
		t.Fatal(err)
	}
	if a.Count("gem") != 0 || len(g.Items) != 1 || g.Items[0].Owner != 0 || g.Items[0].Amount != 3 {
		t.Fatalf("drop should leave a shared stack of 3: %+v", g.Items)
	}
	step(t, g, a, 2) // a walks away
	step(t, g, b, 2) // b walks onto 39,25
	step(t, g, b, 2) // ... and 40,25
	if b.Count("gem") != 3 || len(g.Items) != 0 {
		t.Fatalf("the teammate should pick it up: gems %d floor %d", b.Count("gem"), len(g.Items))
	}
}

func TestKillsDropRucksackItems(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 40, 25)
	seen := map[string]bool{}
	for i := 0; i < 600; i++ {
		g.killMonster(g.spawnMon('f', 45, 30), 1)
	}
	for _, it := range g.Items {
		seen[it.Def] = true
	}
	for _, id := range ItemOrder {
		if !seen[id] {
			t.Errorf("no %s dropped in 600 kills", id)
		}
	}
}

func TestPotionKeyDrinksADraughtWhenThatHelps(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	h.AddItem("draught", 2)
	pots := h.Count("potion")
	h.HP, h.Res = h.HPMax, 10
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != nil {
		t.Fatal(err)
	}
	if h.Count("draught") != 1 || h.Count("potion") != pots || h.Res <= 10 {
		t.Fatal("healthy with low mana: Q drinks a draught")
	}
	g.Now = h.PotionReady
	h.HP, h.Res = h.HPMax/3, 10
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != nil {
		t.Fatal(err)
	}
	if h.Count("potion") != pots-1 || h.Count("draught") != 1 {
		t.Fatal("hurt: Q drinks a healing potion first")
	}
	for h.Count("potion") > 0 {
		h.removeAt(h.firstSlot("potion"), 1)
	}
	g.Now = h.PotionReady
	h.HP = h.HPMax / 3
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != nil || h.Count("draught") != 0 {
		t.Fatalf("no potions left: Q drinks the draught (%v)", err)
	}
}
