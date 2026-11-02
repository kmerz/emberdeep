package room

import (
	"os"
	"testing"
	"time"

	"emberdeep/internal/game"
	"emberdeep/internal/proto"
)

func startRun(t *testing.T, r *Room, host *Conn) {
	t.Helper()
	r.Submit(host, proto.ClientMsg{T: "ready"})
	r.Submit(host, proto.ClientMsg{T: "start"})
	waitFor(t, "game start", func() bool {
		started := false
		r.Inspect(func(g *game.Game, _ int) { started = g != nil })
		return started
	})
}

// The host saves, everyone goes back to the lobby, a new player joins, and the run continues where it was:
// the old hero keeps their level and gold, the newcomer starts at the party's level.
func TestSaveReturnToLobbyAndContinue(t *testing.T) {
	dir := t.TempDir()
	h := fastHub(t, func(c *Config) { c.SaveDir = dir })
	r, _ := h.Create()
	a, ra := join(t, r, "kmerz", "warrior")
	startRun(t, r, a)
	r.Inspect(func(g *game.Game, _ int) {
		hero := g.Hero(ra.PlayerID)
		hero.Lvl, hero.Gold = 6, 321
		g.Depth = 3 // as if the party went down twice
	})
	r.Submit(a, proto.ClientMsg{T: "save"})
	var l *proto.Lobby
	waitFor(t, "back in the lobby with a save", func() bool {
		if x := lastLobby(a); x != nil {
			l = x
		}
		return l != nil && !l.InGame && l.Save != nil
	})
	if l.Save.Depth != 3 || len(l.Save.Heroes) != 1 || l.Save.Heroes[0].Lvl != 6 || !l.Players[0].Saved || l.Players[0].Lvl != 6 {
		t.Fatalf("lobby should show the save: %+v players %+v", l.Save, l.Players)
	}
	if !saveExists(dir, r.Code()) {
		t.Fatal("the save should be on disk")
	}

	_, rb := join(t, r, "nyx", "rogue")
	startRun(t, r, a)
	r.Inspect(func(g *game.Game, _ int) {
		if g.Depth != 3 {
			t.Errorf("continue on depth 3, got %d", g.Depth)
		}
		if k := g.Hero(ra.PlayerID); k.Lvl != 6 || k.Gold != 321 {
			t.Errorf("saved hero should come back: lvl %d gold %d", k.Lvl, k.Gold)
		}
		if n := g.Hero(rb.PlayerID); n.Lvl != 6 {
			t.Errorf("a newcomer starts at the party's level, got %d", n.Lvl)
		}
	})
}

// A server restart: the hub closes (saving the running game), and a new hub brings the room back from disk
// when someone opens the invite link. A returning player gets their class back for their saved hero.
func TestSaveSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	h := fastHub(t, func(c *Config) { c.SaveDir = dir })
	r, _ := h.Create()
	code := r.Code()
	a, ra := join(t, r, "kmerz", "sorcerer")
	startRun(t, r, a)
	r.Inspect(func(g *game.Game, _ int) { g.Hero(ra.PlayerID).Lvl = 8 })
	h.Close()

	h2 := fastHub(t, func(c *Config) { c.SaveDir = dir })
	r2 := h2.Get(code)
	if r2 == nil {
		t.Fatal("the room should come back from its save")
	}
	b, _ := join(t, r2, "kmerz", "warrior")
	var l *proto.Lobby
	waitFor(t, "lobby", func() bool { l = lastLobby(b); return l != nil })
	if l.Save == nil || l.Players[0].Cls != "sorcerer" || l.Players[0].Lvl != 8 {
		t.Fatalf("the saved sorcerer should be waiting: save %+v players %+v", l.Save, l.Players)
	}
	if h2.Get("ZZZ-ZZZ") != nil {
		t.Fatal("a code without a room or save is not a room")
	}
}

func TestNewRunForgetsTheSave(t *testing.T) {
	dir := t.TempDir()
	h := fastHub(t, func(c *Config) { c.SaveDir = dir })
	r, _ := h.Create()
	a, _ := join(t, r, "kmerz", "monk")
	startRun(t, r, a)
	r.Submit(a, proto.ClientMsg{T: "save"})
	waitFor(t, "saved", func() bool { return saveExists(dir, r.Code()) })
	waitFor(t, "lobby", func() bool {
		inGame := true
		r.Inspect(func(g *game.Game, _ int) { inGame = g != nil })
		return !inGame
	})
	r.Submit(a, proto.ClientMsg{T: "setting", Key: "newRun"})
	waitFor(t, "save deleted", func() bool { return !saveExists(dir, r.Code()) })
	if _, err := os.Stat(dir + "/" + r.Code() + ".json.tmp"); err == nil {
		t.Fatal("no temp file should be left behind")
	}
}

func TestOldSavesArePruned(t *testing.T) {
	dir := t.TempDir()
	if err := writeSave(dir, "ABC-DEF", game.RunSave{Heroes: map[string]game.SavedHero{}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	pruneSaves(dir, SaveMaxAge, time.Now())
	if !saveExists(dir, "ABC-DEF") {
		t.Fatal("a fresh save stays")
	}
	pruneSaves(dir, SaveMaxAge, time.Now().Add(SaveMaxAge+time.Hour))
	if saveExists(dir, "ABC-DEF") {
		t.Fatal("an old save is deleted")
	}
}

func TestSaveFileNamesAreOnlyCodes(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadSave(dir, "../etc/passwd"); err == nil || saveExists(dir, "../x") {
		t.Fatal("only room codes may name a save")
	}
}
