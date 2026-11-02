package game

import (
	"slices"
	"testing"
)

// lane finds a direction from the hero with two free cells in a row (the hero's next two steps).
func lane(t *testing.T, g *Game, h *Hero) int {
	t.Helper()
	for d, v := range dirs8 {
		if g.cellFree(h.X+v.X, h.Y+v.Y) && g.cellFree(h.X+2*v.X, h.Y+2*v.Y) {
			return d
		}
	}
	t.Fatal("no free lane next to the hero")
	return 0
}

func eventsOf(evs []Event, k string) []Event {
	var out []Event
	for _, e := range evs {
		if e.K == k {
			out = append(out, e)
		}
	}
	return out
}

func TestEveryFloorIsDressedAndConnected(t *testing.T) {
	var furn, urns, kegs, traps, lava int
	for _, seed := range []uint32{1, 7, 42, 99, 1234} {
		for d := 1; d <= MaxDepth; d++ {
			for _, n := range []int{1, 4} {
				l := Generate(seed, d, n)
				if !l.Reachable() {
					t.Fatalf("seed %d depth %d n %d: not connected", seed, d, n)
				}
				if d == 1 && len(l.Traps) > 0 {
					t.Fatalf("seed %d: no traps on floor 1", seed)
				}
				for _, tl := range l.Tiles {
					if tl == TCandle || tl == TCoffin || tl == TStalk {
						furn++
					}
				}
				for _, c := range l.Chests {
					if c.Urn {
						urns++
					}
				}
				kegs += len(l.Kegs)
				traps += len(l.Traps)
				lava += len(l.Lava)
				for _, p := range append(append([]Pt{}, l.Traps...), l.Lava...) {
					if !l.Walkable(p.X, p.Y) || p == l.Stairs {
						t.Fatalf("seed %d depth %d: hazard on a bad cell %v", seed, d, p)
					}
				}
			}
		}
	}
	if furn == 0 || urns == 0 || kegs == 0 || traps == 0 || lava == 0 {
		t.Fatalf("expected every kind of dressing somewhere: furniture %d urns %d kegs %d traps %d lava %d", furn, urns, kegs, traps, lava)
	}
}

func TestKegsChainAndHurtMonsters(t *testing.T) {
	g, hs := newTestGame(1)
	h := hs[0]
	d := dirs8[lane(t, g, h)]
	a := Pt{h.X + 2*d.X, h.Y + 2*d.Y}
	k1 := g.addMon(KegKind, a.X, a.Y, 1)
	p2, ok := g.freeNear(a.X, a.Y)
	if !ok {
		t.Fatal("no room for a second keg")
	}
	k2 := g.addMon(KegKind, p2.X, p2.Y, 1)
	k1.HP, k2.HP = 1, 1
	p3, ok := g.freeNear(p2.X, p2.Y)
	if !ok {
		t.Fatal("no room for a zombie")
	}
	z := g.addMon('z', p3.X, p3.Y, 1)
	zhp, kills := z.HP, g.Kills
	g.damageMonster(k1, 5, h.ID, false)
	var evs []Event
	for i := 0; i < 3*KegFuse; i++ {
		evs = append(evs, g.Tick()...)
	}
	if g.monByID[k1.ID] != nil || g.monByID[k2.ID] != nil {
		t.Fatal("both kegs should be gone: the second one goes off from the first")
	}
	if n := len(eventsOf(evs, "impact")); n != 2 {
		t.Fatalf("expected 2 blasts, got %d", n)
	}
	if z.HP >= zhp {
		t.Fatal("the blast should hurt the zombie")
	}
	if g.Kills > kills+1 { // only the zombie can count, if it died; kegs never do
		t.Fatalf("kegs must not count as kills: %d -> %d", kills, g.Kills)
	}
}

func TestUrnSmashesWhenBumped(t *testing.T) {
	g, hs := newTestGame(1)
	h := hs[0]
	dir := lane(t, g, h)
	d := dirs8[dir]
	c := &Chest{ID: g.newID(), X: h.X + d.X, Y: h.Y + d.Y, Urn: true}
	g.Chests = append(g.Chests, c)
	g.chestByID[c.ID] = c
	g.occ[c.Y*LW+c.X] = int32(c.ID)
	h.NextMove = 0
	if err := g.applyMove(h, Intent{Kind: IntMove, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if len(eventsOf(g.events, "urn")) != 1 || len(eventsOf(g.events, "chest")) != 0 {
		t.Fatalf("bumping an urn smashes it (and is not a chest): %+v", g.events)
	}
	if g.chestByID[c.ID] != nil || g.occ[c.Y*LW+c.X] != 0 {
		t.Fatal("the urn should be gone")
	}
}

func TestSpikeTrapShowsItselfAndSprings(t *testing.T) {
	g, hs := newTestGame(1)
	h := hs[0]
	dir := lane(t, g, h)
	d := dirs8[dir]
	tr := &Trap{X: h.X + 2*d.X, Y: h.Y + 2*d.Y}
	g.Traps = append(g.Traps, tr)
	g.Tick()
	if tr.Seen {
		t.Fatal("a trap two cells away stays hidden")
	}
	h.NextMove = 0
	if err := g.applyMove(h, Intent{Kind: IntMove, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	evs := g.Tick()
	if !tr.Seen || len(eventsOf(evs, "trapseen")) != 1 {
		t.Fatal("stepping next to a trap reveals it")
	}
	hp := h.HP
	h.NextMove = 0
	if err := g.applyMove(h, Intent{Kind: IntMove, Dir: dir}); err != nil {
		t.Fatal(err)
	}
	evs = g.Tick()
	if len(eventsOf(evs, "trap")) != 1 || h.HP >= hp {
		t.Fatalf("stepping on it hurts: hp %d -> %d", hp, h.HP)
	}
	hp = h.HP
	g.Tick()
	if h.HP != hp {
		t.Fatal("a sprung trap needs time to reset")
	}
}

func TestBossSpeaksWhenItNoticesTheParty(t *testing.T) {
	g, hs := newTestGame(1)
	h := hs[0]
	d := dirs8[lane(t, g, h)]
	k := g.addMon('K', h.X+2*d.X, h.Y+2*d.Y, 1)
	k.Awake = false // as when a floor loads (addMon makes summons, which come awake)
	var barks []Event
	for i := 0; i < 10 && len(barks) == 0; i++ {
		barks = eventsOf(g.Tick(), "bark")
	}
	if len(barks) != 1 || barks[0].Actor != k.ID || !slices.Contains(BossBarks['K'].Wake, barks[0].Text) {
		t.Fatalf("the Tallow King should greet the party once, got %+v", barks)
	}
	for i := 0; i < 20; i++ {
		if b := eventsOf(g.Tick(), "bark"); len(b) > 0 && b[0].Actor == k.ID && slices.Contains(BossBarks['K'].Wake, b[0].Text) {
			t.Fatal("a boss greets only once")
		}
	}
	g.damageMonster(k, k.HP/2+1, h.ID, false)
	if b := eventsOf(g.events, "bark"); len(b) != 1 || !slices.Contains(BossBarks['K'].Half, b[0].Text) {
		t.Fatalf("half health gets its own line, got %+v", b)
	}
}

func TestBarksStayShort(t *testing.T) {
	all := [][]string{NamedBarks.Wake, NamedBarks.Half, NamedBarks.Down, MimicBarks, CallerBarks, FollowerBarks, TraderBarks, GnomeBarks}
	for _, b := range BossBarks {
		all = append(all, b.Wake, b.Half, b.Down)
	}
	for _, l := range WakeBarks {
		all = append(all, l)
	}
	for _, lines := range all {
		for _, s := range lines {
			if len([]rune(s)) > 38 {
				t.Errorf("bark too long for a bubble: %q", s)
			}
		}
	}
}
