package room

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"emberdeep/internal/game"
)

// The Hall of Fame keeps the best runs that reached the bottom, for the landing page. It lives in
// Config.SaveDir next to the saves (in memory only without one).

const (
	FameMax  = 8                   // runs kept (as many as the landing page shows)
	fameFile = "hall-of-fame.json" // never a room code, so loadSave never reads it
)

type FameHero struct {
	Name  string `json:"name"`
	Cls   string `json:"cls"`
	Lvl   int    `json:"lvl"`
	Title string `json:"title"`
	Bot   bool   `json:"bot,omitempty"`
}

type FameRun struct {
	Party []FameHero `json:"party"`
	Secs  int        `json:"secs"`  // time played
	Wipes int        `json:"wipes"` // floors retried
	Diff  string     `json:"diff"`  // difficulty name
	FF    bool       `json:"ff,omitempty"`
	Date  string     `json:"date"` // YYYY-MM-DD
}

// diffRank orders the difficulties for the ranking: harder first.
func diffRank(name string) int {
	switch name {
	case "NIGHTMARE":
		return 2
	case "NORMAL":
		return 1
	}
	return 0
}

// fameLess ranks runs: harder difficulty, then fewer retried floors, then faster.
func fameLess(a, b FameRun) bool {
	if ra, rb := diffRank(a.Diff), diffRank(b.Diff); ra != rb {
		return ra > rb
	}
	if a.Wipes != b.Wipes {
		return a.Wipes < b.Wipes
	}
	return a.Secs < b.Secs
}

type fame struct {
	mu   sync.Mutex
	dir  string
	runs []FameRun
}

func loadFame(dir string) *fame {
	f := &fame{dir: dir}
	if dir == "" {
		return f
	}
	b, err := os.ReadFile(filepath.Join(dir, fameFile))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("hall of fame: %v", err)
		}
		return f
	}
	if err := json.Unmarshal(b, &f.runs); err != nil {
		log.Printf("hall of fame: %v", err)
		f.runs = nil
	}
	return f
}

// add puts a run in its place (after runs that rank the same) and keeps the best FameMax. It reports the run's
// rank (1 = best), or 0 if it did not make the list.
func (f *fame) add(run FameRun) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := sort.Search(len(f.runs), func(i int) bool { return fameLess(run, f.runs[i]) })
	if i >= FameMax {
		return 0
	}
	runs := append(append(append([]FameRun(nil), f.runs[:i]...), run), f.runs[i:]...)
	if len(runs) > FameMax {
		runs = runs[:FameMax]
	}
	f.runs = runs
	if err := f.write(); err != nil {
		log.Printf("hall of fame: %v", err)
	}
	return i + 1
}

// write saves the list atomically (f.mu held).
func (f *fame) write() error {
	if f.dir == "" {
		return nil
	}
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(f.runs)
	if err != nil {
		return err
	}
	p := filepath.Join(f.dir, fameFile)
	if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

func (f *fame) list() []FameRun {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FameRun{}, f.runs...)
}

// fameRun is the Hall of Fame entry for a won run.
func fameRun(g *game.Game, players []*Player, now time.Time) FameRun {
	bots := map[string]bool{}
	for _, p := range players {
		bots[p.Name] = p.Bot
	}
	run := FameRun{Secs: g.PlayTicks() / game.TickHz, Wipes: g.Wipes, Diff: game.Difficulties[g.Settings.Difficulty].Name,
		FF: g.Settings.FriendlyFire, Date: now.UTC().Format("2006-01-02")}
	for _, s := range g.Summary() {
		run.Party = append(run.Party, FameHero{Name: s.Name, Cls: s.Cls, Lvl: s.Lvl, Title: s.Title, Bot: bots[s.Name]})
	}
	return run
}

func (h *Hub) handleFame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(h.fame.list())
}
