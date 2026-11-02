package game

import (
	"strings"
	"testing"
)

func TestGodsTrialHoldsTheLevelUntilYouPray(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	h.Lvl = 4
	g.gainXP(XPNeed(4) + XPNeed(5) + 5)
	if h.Lvl != 4 || !h.TrialOn || h.XP < XPNeed(4) {
		t.Fatalf("level 4 waits for Korrath's trial, XP is kept: lvl %d trial %v xp %d", h.Lvl, h.TrialOn, h.XP)
	}
	var demand bool
	for _, e := range g.events {
		demand = demand || (e.K == "trial" && strings.Contains(e.Text, "10") && e.FX == Gods[Warrior].Name)
	}
	if !demand {
		t.Fatal("the god says what the trial is")
	}
	altar := &Npc{ID: g.newID(), X: 31, Y: 25, Name: AltarName, Altar: true}
	g.Npcs = append(g.Npcs, altar)
	g.npcByID[altar.ID] = altar
	g.occ[25*LW+31] = int32(altar.ID)
	g.pray(h, altar)
	if h.Lvl != 4 {
		t.Fatal("praying before the foes are slain does nothing")
	}
	for i := 0; i < TrialLevels[4]; i++ {
		m := g.spawnMon('z', 40, 10+i%5)
		g.killMonster(m, h.ID)
	}
	if !h.TrialDone() {
		t.Fatalf("10 kills finish the hunt: %d", h.TrialKills)
	}
	g.events = nil
	if err := g.Apply(1, Intent{Kind: IntMove, Dir: dirIndex(1, 0, 0)}); err != nil { // walk into the altar
		t.Fatal(err)
	}
	if h.Lvl < 6 || h.TrialOn || h.Blessed != 4 || h.HP != h.HPMax {
		t.Fatalf("praying grants the held-back levels and heals: lvl %d blessed %d hp %d/%d", h.Lvl, h.Blessed, h.HP, h.HPMax)
	}
}

func TestNewcomersSkipTheTrialsBelowTheirLevel(t *testing.T) {
	g := New(1, Settings{}, 2)
	a := g.AddHero(1, "a", Warrior)
	a.Lvl = 8
	b := g.AddHero(2, "b", Monk)
	g.CatchUp(b)
	if b.Lvl != 8 || b.Blessed != 7 {
		t.Fatalf("caught up to 8 with the trial of 7 passed: lvl %d blessed %d", b.Lvl, b.Blessed)
	}
}

func TestEveryFloorHasAnInscriptionAndAltarsBelowTheFirst(t *testing.T) {
	for d := 1; d <= MaxDepth; d++ {
		g := NewAt(5, Settings{}, 2, d)
		lore, altar := false, false
		for _, n := range g.Npcs {
			lore = lore || n.Lore == d+1
			altar = altar || n.Altar
		}
		if !lore {
			t.Errorf("depth %d has no inscription", d)
		}
		if altar != (d >= 2) {
			t.Errorf("depth %d altar %v", d, altar)
		}
	}
}

func TestReadingAnInscription(t *testing.T) {
	g := New(5, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Rogue)
	var n *Npc
	for _, x := range g.Npcs {
		if x.Lore > 0 {
			n = x
		}
	}
	g.bumpNpc(h, n.X, n.Y)
	ev := g.events[len(g.events)-1]
	if ev.K != "lore" || ev.Text != Lore[1] {
		t.Fatalf("walking into an inscription shows its page: %+v", ev)
	}
}

// vaultFloor finds a floor with a vault.
func vaultFloor(t *testing.T) *Game {
	for seed := uint32(1); seed < 60; seed++ {
		for d := 2; d <= MaxDepth; d++ {
			g := NewAt(seed, Settings{}, 1, d)
			if g.Vault != nil {
				return g
			}
		}
	}
	t.Fatal("no floor with a vault in 60 seeds")
	return nil
}

func TestVaultDoorNeedsTheKey(t *testing.T) {
	g := vaultFloor(t)
	h := g.AddHero(1, "kmerz", Warrior)
	door := g.VaultDoor
	if g.Level.At(door.X, door.Y) != TLocked {
		t.Fatal("the vault door starts locked")
	}
	keys := 0
	for _, m := range g.Mons {
		if m.HasKey {
			keys++
			g.killMonster(m, h.ID)
			break
		}
	}
	for _, it := range g.Items {
		if it.Def == KeyItem {
			keys++
		}
	}
	if keys == 0 {
		t.Fatal("the key is somewhere on the floor")
	}
	if !g.bumpLocked(h, door.X, door.Y) || g.Level.At(door.X, door.Y) != TLocked {
		t.Fatal("no key: the door stays shut")
	}
	h.AddItem(KeyItem, 1)
	g.bumpLocked(h, door.X, door.Y)
	if g.Level.At(door.X, door.Y) != TDoor || h.Count(KeyItem) != 0 || !g.memo().Vault {
		t.Fatal("the key opens the door and is used up")
	}
	var vault *Chest
	for _, c := range g.Chests {
		if c.Vault {
			vault = c
		}
	}
	if vault == nil {
		t.Fatal("a vault chest waits inside")
	}
	items := len(g.Items)
	g.openChest(h, vault)
	if len(g.Items) <= items+2 || !g.memo().VaultLooted {
		t.Fatal("the vault chest spills a lot")
	}
	// the floor remembers: come back and the door is open, the chest gone
	depth := g.Depth
	g.rememberFloor()
	g.loadFloor(depth)
	if g.Level.At(door.X, door.Y) == TLocked {
		t.Fatal("an opened vault stays open")
	}
	for _, c := range g.Chests {
		if c.Vault {
			t.Fatal("a looted vault stays empty")
		}
	}
}

func TestKeysStayOnTheirFloor(t *testing.T) {
	g := New(3, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Monk)
	h.AddItem(KeyItem, 1)
	takeStairs(g)
	if h.Count(KeyItem) != 0 {
		t.Fatal("a vault key only fits its own floor")
	}
}

func TestAltarAndInscriptionAreNeverLockedAway(t *testing.T) {
	for seed := uint32(1); seed < 30; seed++ {
		for d := 2; d <= MaxDepth; d++ {
			g := NewAt(seed, Settings{}, 2, d)
			dist := g.Level.bfsDist(g.Level.Start)
			for _, n := range g.Npcs {
				if (n.Altar || n.Lore > 0) && g.inVault(n.X, n.Y) {
					t.Fatalf("seed %d depth %d: %s is inside the vault", seed, d, n.Name)
				}
				reach := false
				for _, dd := range dirs8 {
					x, y := n.X+dd.X, n.Y+dd.Y
					reach = reach || (x >= 0 && y >= 0 && x < LW && y < LH && dist[y*LW+x] >= 0)
				}
				if (n.Altar || n.Lore > 0) && !reach {
					t.Fatalf("seed %d depth %d: %s can't be reached", seed, d, n.Name)
				}
			}
		}
	}
}
