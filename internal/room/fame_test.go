package room

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFameRanking(t *testing.T) {
	dir := t.TempDir()
	f := loadFame(dir)
	run := func(diff string, wipes, secs int) FameRun {
		return FameRun{Party: []FameHero{{Name: "kmerz", Cls: "warrior", Lvl: 10}}, Diff: diff, Wipes: wipes, Secs: secs}
	}
	if r := f.add(run("NORMAL", 0, 3000)); r != 1 {
		t.Fatalf("first run is #1, got %d", r)
	}
	if r := f.add(run("NORMAL", 0, 2000)); r != 1 {
		t.Fatalf("a faster run goes first, got %d", r)
	}
	if r := f.add(run("NORMAL", 2, 1000)); r != 3 {
		t.Fatalf("retried floors count before time, got %d", r)
	}
	if r := f.add(run("NIGHTMARE", 5, 9000)); r != 1 {
		t.Fatalf("nightmare beats normal, got %d", r)
	}
	if r := f.add(run("NOVICE", 0, 100)); r != 5 {
		t.Fatalf("novice comes last, got %d", r)
	}
	for i := 0; i < FameMax; i++ {
		f.add(run("NORMAL", 1, 500))
	}
	if n := len(f.list()); n != FameMax {
		t.Fatalf("only %d runs are kept, got %d", FameMax, n)
	}
	if r := f.add(run("NOVICE", 9, 9999)); r != 0 {
		t.Fatalf("a run below the list does not make it, got %d", r)
	}
	// it survives a restart, and old-save pruning leaves it alone
	old := time.Now().Add(-2 * SaveMaxAge)
	_ = os.Chtimes(filepath.Join(dir, fameFile), old, old)
	pruneSaves(dir, SaveMaxAge, time.Now())
	g := loadFame(dir).list()
	if len(g) != FameMax || g[0].Diff != "NIGHTMARE" {
		t.Fatalf("the list should load back: %+v", g)
	}
}

func TestFameEndpoint(t *testing.T) {
	h := fastHub(t, nil)
	h.fame.add(FameRun{Party: []FameHero{{Name: "nyx", Cls: "rogue"}}, Diff: "NORMAL", Secs: 1234})
	srv := httptest.NewServer(h.Handler(nil))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/fame")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var runs []FameRun
	if err := json.NewDecoder(res.Body).Decode(&runs); err != nil || len(runs) != 1 || runs[0].Party[0].Name != "nyx" || runs[0].Secs != 1234 {
		t.Fatalf("GET /api/fame: %v %+v", err, runs)
	}
}
