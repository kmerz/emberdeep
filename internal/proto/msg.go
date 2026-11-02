// Package proto holds the wire types. Field names are mirrored in web/src/proto.ts.
package proto

import "emberdeep/internal/game"

// ClientMsg is the single envelope for every client -> server message; unused fields stay zero.
type ClientMsg struct {
	T      string `json:"t"`
	Name   string `json:"name,omitempty"`
	Cls    string `json:"cls,omitempty"`
	Token  string `json:"token,omitempty"`
	Dir    int    `json:"dir,omitempty"`
	Seq    int    `json:"seq,omitempty"`
	Slot   int    `json:"slot,omitempty"`
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Target int    `json:"target,omitempty"`
	Hold   bool   `json:"hold,omitempty"`
	Text   string `json:"text,omitempty"`
	N      int    `json:"n,omitempty"`
	Cell   [2]int `json:"cell,omitempty"`
	Key    string `json:"key,omitempty"`
}

// Client message types.
const (
	CHello    = "hello"
	CReady    = "ready"
	CSetClass = "setClass"
	CStart    = "start"
	CSetting  = "setting"
	CMove     = "move"
	CCast     = "cast"
	CPotion   = "potion"
	CUse      = "use"  // use the rucksack item in slot (x, y: where to throw it)
	CDrop     = "drop" // drop the stack in slot
	CRevive   = "revive"
	CChat     = "chat"
	CPing     = "ping"
	CHbAck    = "hbAck"
	CLeave    = "leave" // leave the room for good: no 60 s reconnect hold
)

type Welcome struct {
	T         string `json:"t"`
	You       int    `json:"you"`
	Token     string `json:"token"`
	Code      string `json:"code"`
	InviteURL string `json:"inviteUrl"`
	IsHost    bool   `json:"isHost"`
}

type LobbyPlayer struct {
	Slot      int    `json:"slot"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Cls       string `json:"cls"`
	Lvl       int    `json:"lvl"`
	Ready     bool   `json:"ready"`
	PingMs    int    `json:"pingMs"`
	Host      bool   `json:"host"`
	Connected bool   `json:"connected"`
	Bot       bool   `json:"bot,omitempty"`
}

type LobbySettings struct {
	Difficulty   string `json:"difficulty"`
	Seed         string `json:"seed"`
	Loot         string `json:"loot"`
	FriendlyFire bool   `json:"friendlyFire"`
}

type Lobby struct {
	T        string        `json:"t"`
	Code     string        `json:"code"`
	Players  []LobbyPlayer `json:"players"`
	Settings LobbySettings `json:"settings"`
	InGame   bool          `json:"inGame"`
}

type Phase struct {
	T     string `json:"t"`
	Phase string `json:"phase"` // "lobby" | "game"
}

type Level struct {
	T        string   `json:"t"`
	W        int      `json:"w"`
	H        int      `json:"h"`
	Seed     string   `json:"seed"`
	Torches  [][2]int `json:"torches"`
	Stairs   [2]int   `json:"stairs"`
	Start    [2]int   `json:"start"`
	Depth    int      `json:"depth"`
	MaxDepth int      `json:"maxDepth"`
	Theme    string   `json:"theme"` // palette id on the client
	Glows    [][2]int `json:"glows"` // decor tiles that give off light
}

// Explore carries newly explored tiles. It is delivered as an event (never dropped).
type Explore struct {
	T     string   `json:"t"`
	Tiles [][3]int `json:"tiles"`
}

type Status struct {
	K string `json:"k"`
	T int    `json:"t"` // seconds remaining
}

type Ent struct {
	ID     int      `json:"id"`
	K      string   `json:"k"` // h hero, m monster, i item
	G      string   `json:"g"`
	X      int      `json:"x"`
	Y      int      `json:"y"`
	HP     int      `json:"hp,omitempty"`
	HPMax  int      `json:"hpMax,omitempty"`
	Cls    string   `json:"cls,omitempty"`
	Name   string   `json:"name,omitempty"`
	Downed bool     `json:"downed,omitempty"`
	Dead   bool     `json:"dead,omitempty"`
	DownT  int      `json:"downT,omitempty"` // seconds until death
	Rev    float64  `json:"rev,omitempty"`   // revive progress 0..1
	Elite  bool     `json:"el,omitempty"`
	Face   int      `json:"face"`
	St     []Status `json:"st,omitempty"`
	It     string   `json:"it,omitempty"` // item id for rucksack items on the floor
}

type PartyEntry struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Cls       string   `json:"cls"`
	Lvl       int      `json:"lvl"`
	HP        int      `json:"hp"`
	HPMax     int      `json:"hpMax"`
	Res       int      `json:"res"`
	ResMax    int      `json:"resMax"`
	St        []Status `json:"st,omitempty"`
	Downed    bool     `json:"downed,omitempty"`
	Dead      bool     `json:"dead,omitempty"`
	DownT     int      `json:"downT,omitempty"`
	Connected bool     `json:"connected"`
}

type Vis struct {
	X0 int    `json:"x0"`
	Y0 int    `json:"y0"`
	W  int    `json:"w"`
	H  int    `json:"h"`
	B  string `json:"b"` // base64 bitset, row-major within the box
}

type Objectives struct {
	Boss        bool `json:"boss"` // this floor has a boss
	KingAlive   bool `json:"kingAlive"`
	StairsAt    int  `json:"stairsAt"`
	StairsNeed  int  `json:"stairsNeed"`
	StairsLeft  int  `json:"stairsLeft"` // ticks until descent, 0 when not running
	StairsFound bool `json:"stairsFound"`
	Gold        int  `json:"gold"`
	Kills       int  `json:"kills"`
}

type PackSlot struct {
	Item string `json:"item"`
	N    int    `json:"n"`
}

type Me struct {
	Res     int        `json:"res"`
	ResMax  int        `json:"resMax"`
	CD      []int      `json:"cd"`    // remaining ticks
	CDMax   []int      `json:"cdMax"` // total ticks
	Cost    []int      `json:"cost"`
	Potions int        `json:"potions"` // healing potions in the rucksack (the Q key)
	PotCD   int        `json:"potCd"`   // shared cooldown of potions and draughts
	Pack    []PackSlot `json:"pack"`    // always PackSlots entries; empty ones are {"", 0}
	Gold    int        `json:"gold"`
	XP      int        `json:"xp"`
	XPNeed  int        `json:"xpNeed"`
	Lvl     int        `json:"lvl"`
}

// SnapShared is the part of a snapshot identical for every client of a room.
type SnapShared struct {
	T       string       `json:"t"`
	Tick    int          `json:"tick"`
	Vis     Vis          `json:"vis"`
	Ents    []Ent        `json:"ents"`
	Party   []PartyEntry `json:"party"`
	Obj     Objectives   `json:"obj"`
	Patches [][2]int     `json:"patches,omitempty"`
}

type Events struct {
	T    string       `json:"t"`
	Tick int          `json:"tick"`
	List []game.Event `json:"list"`
}

type Chat struct {
	T    string `json:"t"`
	From int    `json:"from"`
	Name string `json:"name"`
	Cls  string `json:"cls"`
	Text string `json:"text"`
}

type Ping struct {
	T    string `json:"t"`
	From int    `json:"from"`
	Name string `json:"name"`
	Cls  string `json:"cls"`
	Cell [2]int `json:"cell"`
}

type Hb struct {
	T string `json:"t"`
	N int    `json:"n"`
}

type Overlay struct {
	T    string `json:"t"`
	Kind string `json:"kind"` // wipe | cleared
}

type Error struct {
	T   string `json:"t"`
	Msg string `json:"msg"`
}
