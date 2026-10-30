package game

import "testing"

func TestIntentValidation(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Sorcerer, 30, 25)
	g.Now = 100
	cases := []struct {
		name string
		in   Intent
		want error
	}{
		{"bad dir", Intent{Kind: IntMove, Dir: 9}, ErrBadIntent},
		{"bad slot", Intent{Kind: IntCast, Slot: 5, X: 31, Y: 25}, ErrBadIntent},
		{"target off map", Intent{Kind: IntCast, Slot: 0, X: -3, Y: 5}, ErrBadTarget},
		{"out of range", Intent{Kind: IntCast, Slot: 0, X: 55, Y: 25}, ErrRange},
		{"target in a wall", Intent{Kind: IntCast, Slot: 0, X: 19, Y: 25}, ErrBlocked},
		{"unknown hero", Intent{Kind: IntMove, Dir: 1}, ErrNoHero},
	}
	for _, c := range cases {
		pid := 1
		if c.name == "unknown hero" {
			pid = 77
		}
		if err := g.Apply(pid, c.in); err != c.want {
			t.Errorf("%s: got %v want %v", c.name, err, c.want)
		}
	}
	// cooldown
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 0, X: 36, Y: 25}); err != nil {
		t.Fatal(err)
	}
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 0, X: 36, Y: 25}); err != ErrCooldown {
		t.Fatalf("expected cooldown, got %v", err)
	}
	// no mana
	h.Res = 3
	g.Now += 500
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 1, X: 36, Y: 25}); err != ErrResource {
		t.Fatalf("expected ErrResource, got %v", err)
	}
	// blocked line of sight
	h.Res = 100
	for y := 20; y <= 30; y++ {
		g.Level.Tiles[y*LW+33] = TWall
	}
	if err := g.Apply(1, Intent{Kind: IntCast, Slot: 0, X: 38, Y: 25}); err != ErrLOS {
		t.Fatalf("expected ErrLOS, got %v", err)
	}
	// potions: cooldown and count
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != nil {
		t.Fatal(err)
	}
	if err := g.Apply(1, Intent{Kind: IntPotion}); err != ErrCooldown {
		t.Fatalf("expected potion cooldown, got %v", err)
	}
	// walking into a wall / another body
	g.place(h, 20, 25)
	h.NextMove = 0
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: 6}); err != ErrBlocked {
		t.Fatalf("expected blocked, got %v", err)
	}
}

func TestMovementSpeedIsEnforcedOverTime(t *testing.T) {
	g := arena(1)
	hero(g, 1, Warrior, 25, 25)
	ok := 0
	// a hostile client spams a move every tick for 2 seconds: it may only get ~1 step per 3 ticks
	for i := 0; i < 2*TickHz; i++ {
		g.Tick()
		if g.Apply(1, Intent{Kind: IntMove, Dir: 2}) == nil {
			ok++
		}
	}
	if max := 2*TickHz/MoveTicks + 2; ok > max {
		t.Fatalf("hero moved %d times in 2 s, limit %d", ok, max)
	}
	if ok < 10 {
		t.Fatalf("legit movement throttled too hard: %d", ok)
	}
}
