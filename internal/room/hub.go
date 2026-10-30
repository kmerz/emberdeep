package room

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"emberdeep/internal/proto"
)

type Config struct {
	MaxRooms  int
	PublicURL string
	Origin    OriginPolicy
	Lag       time.Duration
	Jitter    time.Duration
	Bots      int

	TickEvery       time.Duration // default 50 ms (20 Hz)
	HbEvery         time.Duration // default 2 s
	ReconnectWindow time.Duration // default 60 s
	EmptyTimeout    time.Duration // default 5 min
	Clock           func() time.Time
	Stats           *TickStats
	Seed            int64 // 0 = random room codes / seeds
}

func (c *Config) defaults() {
	if c.MaxRooms == 0 {
		c.MaxRooms = 200
	}
	if c.TickEvery == 0 {
		c.TickEvery = 50 * time.Millisecond
	}
	if c.HbEvery == 0 {
		c.HbEvery = 2 * time.Second
	}
	if c.ReconnectWindow == 0 {
		c.ReconnectWindow = 60 * time.Second
	}
	if c.EmptyTimeout == 0 {
		c.EmptyTimeout = 5 * time.Minute
	}
	if c.Clock == nil {
		c.Clock = time.Now
	}
}

var ErrTooManyRooms = errors.New("server is full")

// Hub maps room codes to rooms. Its mutex protects only the map, never game state.
type Hub struct {
	cfg    Config
	mu     sync.Mutex
	rooms  map[string]*Room
	rng    *rand.Rand
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewHub(cfg Config) *Hub {
	cfg.defaults()
	seed := cfg.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{cfg: cfg, rooms: map[string]*Room{}, rng: rand.New(rand.NewSource(seed)), ctx: ctx, cancel: cancel}
}

// Create makes a new room with a unique code and starts its goroutine.
func (h *Hub) Create() (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rooms) >= h.cfg.MaxRooms {
		return nil, ErrTooManyRooms
	}
	var code string
	for {
		code = newCode(h.rng)
		if _, ok := h.rooms[code]; !ok {
			break
		}
	}
	r := newRoom(h, code, h.rng.Uint32()&0xFFFFFF)
	h.rooms[code] = r
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		r.run(h.ctx)
	}()
	return r, nil
}

func (h *Hub) Get(code string) *Room {
	code = NormalizeCode(code)
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rooms[code]
}

func (h *Hub) remove(code string) {
	h.mu.Lock()
	delete(h.rooms, code)
	h.mu.Unlock()
}

func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

// Close stops every room and waits for them.
func (h *Hub) Close() {
	h.cancel()
	h.wg.Wait()
}

// Handler returns the HTTP mux: POST /api/rooms, GET /ws?room=CODE and the SPA.
func (h *Hub) Handler(web http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/rooms", h.handleCreate)
	mux.HandleFunc("/ws", h.serveWS)
	if web != nil {
		mux.Handle("/", web)
	}
	return mux
}

func (h *Hub) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if !h.cfg.Origin.Check(r) {
		http.Error(w, "bad origin", http.StatusForbidden)
		return
	}
	rm, err := h.Create()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"code": rm.code})
}

func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request) {
	ws, err := acceptWS(w, r, h.cfg.Origin)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	c := newConn(ws, cancel)
	defer c.Close()
	fail := func(msg string) {
		c.Kick(msg)
		wctx, wc := context.WithTimeout(context.Background(), 2*time.Second)
		defer wc()
		_ = writePump(wctx, ws, c.Out, nil)
		ws.Close(websocket.StatusPolicyViolation, msg)
	}
	rm := h.Get(r.URL.Query().Get("room"))
	if rm == nil {
		fail("no such room")
		return
	}
	rctx, rc := context.WithTimeout(ctx, 5*time.Second)
	_, data, err := ws.Read(rctx)
	rc()
	if err != nil {
		return
	}
	var hello proto.ClientMsg
	if json.Unmarshal(data, &hello) != nil || hello.T != proto.CHello {
		fail("expected hello")
		return
	}
	if _, err := rm.Join(c, hello); err != nil {
		fail(strings.TrimPrefix(err.Error(), "room: "))
		return
	}
	lagOut := NewLagger(h.cfg.Lag, h.cfg.Jitter)
	go func() {
		_ = writePump(ctx, ws, c.Out, lagOut)
		c.Close()
	}()
	defer rm.Leave(c)

	submit := func(m proto.ClientMsg) { rm.Submit(c, m) }
	lagIn := NewLagger(h.cfg.Lag, h.cfg.Jitter)
	var queue chan timedMsg
	if lagIn != nil {
		queue = make(chan timedMsg, 512)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case tm := <-queue:
					SleepUntil(ctx, tm.at)
					submit(tm.m)
				}
			}
		}()
	}
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		now := time.Now()
		ok, kick := c.allow(now)
		if kick {
			c.Kick("rate limit exceeded")
			return
		}
		if !ok {
			continue
		}
		var m proto.ClientMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if queue != nil {
			select {
			case queue <- timedMsg{lagIn.DeliverAt(now), m}:
			default:
			}
		} else {
			submit(m)
		}
	}
}

type timedMsg struct {
	at time.Time
	m  proto.ClientMsg
}
