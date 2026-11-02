package game

import "testing"

// darkArena is the test arena with the pitch-black theme.
func darkArena(n int) *Game {
	g := arena(n)
	g.Level.Theme = Themes["dark"]
	return g
}

func TestDarkFloorOnlyShowsLitCells(t *testing.T) {
	g := darkArena(1)
	h := hero(g, 1, Warrior, 30, 25)
	m := g.spawnMon('z', 40, 25) // 10 cells away: in line of sight, but unlit
	m.StunUntil = 1 << 30
	g.Tick()
	if !g.IsVisible(40, 25) || g.IsVisible(50, 25) {
		t.Fatal("the lantern lights itself in the dark and shows about 12 cells")
	}
	oil := h.Oil
	g.Tick()
	if h.Oil != oil-1 {
		t.Fatal("the lantern burns oil on a dark floor")
	}
	h.Oil = 1
	g.Tick()
	g.Tick()
	if g.IsVisible(40, 25) || g.IsVisible(36, 25) || !g.IsVisible(31, 25) || h.Oil != 0 {
		t.Fatal("without oil a hero sees only the cells next to them")
	}
}

func TestLanternOnlyBurnsInTheDark(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	g.Tick()
	if h.Oil != StartOil {
		t.Fatal("no oil burns on a lit floor")
	}
}

func TestLampOilGoesIntoTheLantern(t *testing.T) {
	g := darkArena(1)
	h := hero(g, 1, Warrior, 30, 25)
	h.Oil = 0
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: 't', Def: "oil", X: 30, Y: 25, Amount: 1})
	g.pickup(h)
	if h.Oil != OilFlask || len(g.Items) != 0 || h.PackUsed() != 1 {
		t.Fatalf("a flask fills the lantern, not the rucksack: oil %d items %d pack %d", h.Oil, len(g.Items), h.PackUsed())
	}
	h.Oil = OilMax
	g.Items = append(g.Items, &Item{ID: g.newID(), Kind: 't', Def: "oil", X: 30, Y: 25, Amount: 1})
	g.pickup(h)
	if len(g.Items) != 1 {
		t.Fatal("a full lantern leaves the flask on the floor")
	}
}

func TestOldSavesTurnTorchesIntoOil(t *testing.T) {
	h := &Hero{}
	s := heroSave{Lvl: 1, HPMax: 10, HP: 10}
	s.Pack[0] = Stack{Item: "torch", N: 2}
	h.restore(s)
	if h.Oil != 2*OilFlask || h.Pack[0].N != 0 {
		t.Fatalf("torches become oil: oil %d pack %+v", h.Oil, h.Pack[0])
	}
}

func TestLightOrbLightsAnAreaForEveryone(t *testing.T) {
	g := darkArena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	h.Oil = 0 // only the orb's light
	h.AddItem("lightorb", 1)
	slot := h.firstSlot("lightorb")
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot, X: 58, Y: 25}); err != ErrRange {
		t.Fatalf("orbs have a range, got %v", err)
	}
	if err := g.Apply(1, Intent{Kind: IntUse, Slot: slot, X: 42, Y: 25}); err != nil {
		t.Fatal(err)
	}
	g.Tick()
	if !g.IsVisible(42, 25) || !g.IsVisible(46, 25) || g.IsVisible(54, 25) {
		t.Fatal("the orb lights the area around it (and only that)")
	}
	g.Now += OrbTicks
	g.Tick()
	if g.IsVisible(42, 25) || len(g.Orbs) != 0 {
		t.Fatal("the orb goes out after a minute")
	}
}

func TestLitFloorsIgnoreLight(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 30, 25)
	g.Tick()
	if !g.IsVisible(45, 25) {
		t.Fatal("normal floors see as far as before")
	}
}

func TestFloorSevenIsDark(t *testing.T) {
	for d := 1; d <= MaxDepth; d++ {
		if got := Floor(d).Theme.Dark; got != (d == 7) {
			t.Errorf("depth %d dark=%v", d, got)
		}
	}
	l := Generate(42, 7, 2)
	if len(l.Torches) != 0 || len(l.Glows) != 0 {
		t.Fatal("the dark floor has no wall torches or glowing decor")
	}
}
