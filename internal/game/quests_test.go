package game

import "testing"

// forceQuest makes a quest part of this run's offers (the offers are random per seed) and activates it.
func forceQuest(g *Game, id string) *Quest {
	q := g.quest(id)
	if q == nil {
		for i := range QuestPool {
			if QuestPool[i].ID == id {
				q = &Quest{Def: &QuestPool[i]}
				g.Quests = append(g.Quests, q)
			}
		}
	}
	q.State = QuestActive
	return q
}

func board(g *Game) *Npc {
	for _, n := range g.Npcs {
		if n.Board {
			return n
		}
	}
	return nil
}

func TestOffersPerRun(t *testing.T) {
	a, b := New(42, Settings{}, 1), New(42, Settings{}, 1)
	count := map[string]int{}
	for i, q := range a.Quests {
		count[q.Def.Giver]++
		if q.State != QuestOffered || b.Quests[i].Def.ID != q.Def.ID {
			t.Fatal("offers are fixed by the seed and start as offers")
		}
	}
	if count["board"] != QuestOffers || count["grimble"] != QuestOffers {
		t.Fatalf("each giver offers %d quests: %v", QuestOffers, count)
	}
}

func TestAcceptAtTheBoard(t *testing.T) {
	g := New(42, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Warrior)
	b := board(g)
	if b == nil {
		t.Fatal("floor 1 has the notice board")
	}
	var offer *Quest
	for _, q := range g.Quests {
		if q.Def.Giver == "board" {
			offer = q
			break
		}
	}
	g.place(h, b.X+5, b.Y)
	if err := g.Apply(1, Intent{Kind: IntAccept, Skill: offer.Def.ID}); err != ErrNotHere {
		t.Fatalf("accepting needs the board next to you: %v", err)
	}
	p, _ := g.Level.nearestFree(Pt{b.X, b.Y + 1}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	g.place(h, p.X, p.Y)
	if err := g.Apply(1, Intent{Kind: IntAccept, Skill: offer.Def.ID}); err != nil {
		t.Fatal(err)
	}
	if offer.State != QuestActive || !g.TakeQuestsChanged() {
		t.Fatal("the quest is active and the change is reported")
	}
}

func TestHuntQuest(t *testing.T) {
	g := New(42, Settings{}, 2)
	a := g.AddHero(1, "kmerz", Warrior)
	b := g.AddHero(2, "nyx", Rogue)
	q := forceQuest(g, "snapjaw")
	takeStairs(g) // floor 2
	var snap *Monster
	for _, m := range g.Mons {
		if m.Named == "Old Snapjaw" {
			snap = m
		}
	}
	if snap == nil || snap.Kind != 'r' || snap.HPMax <= MonDefs['r'].HP*2 || snap.Elite {
		t.Fatalf("Old Snapjaw waits on floor 2, a big rat but not the floor boss: %+v", snap)
	}
	ga, gb, xp := a.Gold, b.Gold, a.XP+a.Lvl*1000
	g.killMonster(snap, 1)
	if q.State != QuestDone || a.Gold != ga+q.Def.Gold || b.Gold != gb+q.Def.Gold || a.XP+a.Lvl*1000 <= xp {
		t.Fatalf("killing it pays every hero the full reward: %v gold %d/%d", q.State, a.Gold-ga, b.Gold-gb)
	}
}

func TestRescueQuest(t *testing.T) {
	g := NewAt(42, Settings{}, 1, 2)
	h := g.AddHero(1, "kmerz", Monk)
	q := forceQuest(g, "tibble")
	takeStairs(g) // floor 3
	var tib *Npc
	for _, n := range g.Npcs {
		if n.Quest == "tibble" {
			tib = n
		}
	}
	if tib == nil {
		t.Fatal("Tibble waits on floor 3")
	}
	// walk into him: he follows
	p, _ := g.Level.nearestFree(Pt{tib.X - 1, tib.Y}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	g.place(h, p.X, p.Y)
	h.NextMove = 0
	_ = g.Apply(1, Intent{Kind: IntMove, Dir: dirIndex(tib.X-h.X, tib.Y-h.Y, 0)})
	if tib.Following != h.ID {
		t.Fatal("bumping the lost gnome makes him follow")
	}
	clearMons(g)
	// a few steps away he catches up with the hero
	g.place(h, tib.X+4, tib.Y)
	for i := 0; i < 30; i++ {
		g.Tick()
	}
	if cheb(tib.X, tib.Y, h.X, h.Y) > 1 {
		t.Fatalf("a follower catches up: tibble %d,%d hero %d,%d", tib.X, tib.Y, h.X, h.Y)
	}
	// the stairs wait for him: the hero alone at the stairs isn't enough
	st := g.Level.Stairs
	g.place(h, st.X-1, st.Y)
	g.Tick()
	if g.StairsNeed != 2 || g.StairsAt != 1 {
		t.Fatalf("the rescued gnome counts at the stairs: %d/%d", g.StairsAt, g.StairsNeed)
	}
	p2, _ := g.Level.nearestFree(Pt{st.X, st.Y - 1}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	g.moveEntity(int32(tib.ID), &tib.X, &tib.Y, p2.X, p2.Y)
	takeStairs(g)
	if q.State != QuestDone || h.Points < 2 {
		t.Fatalf("bringing him to the stairs completes it (with a skill point): %v points %d", q.State, h.Points)
	}
}

func TestRescueFailsIfLeftBehindOrKilled(t *testing.T) {
	g := NewAt(42, Settings{}, 1, 2)
	g.AddHero(1, "kmerz", Monk)
	q := forceQuest(g, "tibble")
	takeStairs(g) // floor 3, never talk to Tibble
	takeStairs(g)
	if q.State != QuestFailed {
		t.Fatal("leaving the floor without him fails the quest")
	}

	g2 := NewAt(42, Settings{}, 1, 3)
	g2.AddHero(1, "kmerz", Monk)
	q2 := forceQuest(g2, "tibble")
	g2.questsOnFloor()
	var tib *Npc
	for _, n := range g2.Npcs {
		if n.Quest == "tibble" {
			tib = n
		}
	}
	g2.hurtFollower(tib, FollowerHP, 0)
	if q2.State != QuestFailed || g2.npcByID[tib.ID] != nil {
		t.Fatal("a killed gnome fails the quest")
	}
}

func TestFetchAndTurnIn(t *testing.T) {
	g := NewAt(7, Settings{}, 1, VillageAfter)
	h := g.AddHero(1, "kmerz", Rogue)
	q := forceQuest(g, "ledger")
	g.questsOnFloor()
	var ledger *Item
	for _, it := range g.Items {
		if it.Def == "ledger" {
			ledger = it
		}
	}
	if ledger == nil {
		t.Fatal("the ledger lies on floor 5")
	}
	clearMons(g)
	g.place(h, ledger.X, ledger.Y)
	g.pickup(h)
	if h.Count("ledger") != 1 || q.Progress != 1 {
		t.Fatal("picking it up counts as found")
	}
	if err := g.Apply(1, Intent{Kind: IntDrop, Slot: h.firstSlot("ledger")}); err != ErrBadIntent {
		t.Fatalf("quest items can't be dropped: %v", err)
	}
	takeStairs(g) // the village
	g2, _ := g, h
	var tr *Npc
	for _, n := range g2.Npcs {
		if n.Trader {
			tr = n
		}
	}
	p, _ := g.Level.nearestFree(Pt{tr.X, tr.Y + 1}, func(x, y int) bool { return g.occ[y*LW+x] == 0 })
	g.place(h, p.X, p.Y)
	if err := g.Apply(1, Intent{Kind: IntSell, Slot: h.firstSlot("ledger")}); err != ErrBadIntent {
		t.Fatalf("Grimble won't buy quest items: %v", err)
	}
	gold := h.Gold
	h.NextMove = 0
	_ = g.Apply(1, Intent{Kind: IntMove, Dir: dirIndex(tr.X-h.X, tr.Y-h.Y, 0)})
	if q.State != QuestDone || h.Count("ledger") != 0 || h.Gold != gold+q.Def.Gold {
		t.Fatalf("walking into Grimble hands it in: %v ledger %d", q.State, h.Count("ledger"))
	}
}

func TestCollectDropsOnlyWhileActive(t *testing.T) {
	g := arena(1)
	h := hero(g, 1, Warrior, 30, 25)
	for i := 0; i < 50; i++ {
		g.killMonster(g.addMon('a', 45, 30, 1), 1)
	}
	for _, it := range g.Items {
		if it.Def == "antegg" {
			t.Fatal("no eggs without the quest")
		}
	}
	q := forceQuest(g, "eggs")
	for i := 0; i < 50; i++ {
		g.killMonster(g.addMon('a', 45, 30, 1), 1)
	}
	eggs := 0
	for _, it := range g.Items {
		if it.Def == "antegg" {
			eggs++
		}
	}
	if eggs == 0 {
		t.Fatal("ants drop eggs while the quest is active")
	}
	h.AddItem("antegg", q.Def.Count)
	g.questPickup("antegg")
	if q.Progress != q.Def.Count {
		t.Fatalf("progress counts the eggs: %d", q.Progress)
	}
}

func TestQuestsSurviveFloorsAndRetry(t *testing.T) {
	g := New(42, Settings{}, 1)
	h := g.AddHero(1, "kmerz", Warrior)
	q := forceQuest(g, "wick")
	takeStairs(g)
	downHeroNow(g, h)
	for i := 0; i < 3; i++ {
		g.Tick()
	}
	g.RetryFloor()
	if q.State != QuestActive {
		t.Fatal("quests survive the stairs and a retry")
	}
}
