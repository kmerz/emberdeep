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
	Skill  string `json:"skill,omitempty"` // learn / bind: skill id
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
	CUse      = "use"     // use the rucksack item in slot (x, y: where to throw it)
	CDrop     = "drop"    // drop the stack in slot
	CUnequip  = "unequip" // take off the weapon (slot 0) or trinket (slot 1)
	CLearn    = "learn"   // spend a point on skill
	CBind     = "bind"    // put skill on key slot (0-2)
	CRespec   = "respec"  // reset the skill tree (at the stairs)
	CBuy      = "buy"     // buy one item (skill: item id) from the trader
	CSell     = "sell"    // sell one item from rucksack slot
	CAccept   = "accept"  // accept a quest (skill: quest id) from the giver next to you
	CRevive   = "revive"
	CChat     = "chat"
	CPing     = "ping"
	CHbAck    = "hbAck"
	CLeave    = "leave" // leave the room for good: no 60 s reconnect hold
	CSave     = "save"  // host: save the game and return everyone to the lobby
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
	Saved     bool   `json:"saved,omitempty"` // continues their saved hero
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
	Save     *SaveInfo     `json:"save,omitempty"` // a saved game the next start continues
}

// SaveInfo describes a saved game in the lobby.
type SaveInfo struct {
	Depth   int        `json:"depth"`
	Village bool       `json:"village,omitempty"`
	Heroes  []SaveHero `json:"heroes"`
}

type SaveHero struct {
	Name string `json:"name"`
	Cls  string `json:"cls"`
	Lvl  int    `json:"lvl"`
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
	Up       *[2]int  `json:"up,omitempty"` // the stairs up (none on floor 1)
	Start    [2]int   `json:"start"`
	Depth    int      `json:"depth"`
	MaxDepth int      `json:"maxDepth"`
	Theme    string   `json:"theme"`             // palette id on the client
	Glows    [][2]int `json:"glows"`             // decor tiles that give off light
	Dark     bool     `json:"dark,omitempty"`    // pitch black: only lit cells are visible
	Village  bool     `json:"village,omitempty"` // the gnome village
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
	It     string   `json:"it,omitempty"`     // item id for rucksack items on the floor
	Trader bool     `json:"trader,omitempty"` // gnomes (k "n"): this one trades
	Board  bool     `json:"board,omitempty"`  // k "n": the notice board
	Follow bool     `json:"follow,omitempty"` // k "n": a gnome being rescued
	Rar    int      `json:"rar,omitempty"`    // items: rarity (2 = rare: a loot beam)
	Nm     string   `json:"nm,omitempty"`     // items: a gear piece's name
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
	Boss        bool   `json:"boss"` // this floor has a boss
	BossName    string `json:"bossName,omitempty"`
	Village     bool   `json:"village,omitempty"`
	PortalAt    int    `json:"portalAt,omitempty"`
	PortalNeed  int    `json:"portalNeed,omitempty"`
	KingAlive   bool   `json:"kingAlive"`
	StairsAt    int    `json:"stairsAt"`
	StairsNeed  int    `json:"stairsNeed"`
	StairsLeft  int    `json:"stairsLeft"`         // ticks until descent, 0 when not running
	StairsUp    bool   `json:"stairsUp,omitempty"` // the party is gathering at the stairs up
	StairsFound bool   `json:"stairsFound"`
	Gold        int    `json:"gold"`
	Kills       int    `json:"kills"`
}

// Trial: the hero's god wants Need foes slain (Kills so far), then a prayer at an altar.
type Trial struct {
	God   string `json:"god"`
	Need  int    `json:"need"`
	Kills int    `json:"kills"`
}

type TreeSkill struct {
	ID   string `json:"id"`
	Rank int    `json:"rank"`
}

type PackSlot struct {
	Item string    `json:"item"`
	N    int       `json:"n"`
	G    *GearView `json:"g,omitempty"` // a weapon or trinket
}

// GearView is a rolled weapon or trinket as the client sees it.
type GearView struct {
	Slot  string    `json:"slot"` // "weapon" or "trinket"
	Base  string    `json:"base"`
	Name  string    `json:"name"`
	Rar   int       `json:"rar"` // 0 common, 1 magic, 2 rare
	Lvl   int       `json:"lvl"`
	Mods  []GearMod `json:"mods"`            // the base stat first
	Value int       `json:"value"`           // what Grimble pays
	Price int       `json:"price,omitempty"` // Grimble's wares: what he asks
}

type GearMod struct {
	K string `json:"k"`
	V int    `json:"v"`
}

// ViewGear converts a piece of gear for the wire (nil stays nil).
func ViewGear(g *game.Gear) *GearView {
	if g == nil {
		return nil
	}
	v := &GearView{Slot: g.Slot, Base: g.Base, Name: g.Name, Rar: g.Rarity, Lvl: g.Lvl, Value: g.Value(), Mods: make([]GearMod, len(g.Mods))}
	for i, m := range g.Mods {
		v.Mods[i] = GearMod{m.K, m.V}
	}
	return v
}

type Me struct {
	Res     int         `json:"res"`
	ResMax  int         `json:"resMax"`
	CD      []int       `json:"cd"`    // remaining ticks
	CDMax   []int       `json:"cdMax"` // total ticks
	Cost    []int       `json:"cost"`
	Potions int         `json:"potions"` // healing potions in the rucksack (the Q key)
	PotCD   int         `json:"potCd"`   // shared cooldown of potions and draughts
	Pack    []PackSlot  `json:"pack"`    // always PackSlots entries; empty ones are {"", 0}
	PackCap int         `json:"packCap"` // usable slots (12, or 16 with the Gnome Satchel)
	Binds   []string    `json:"binds"`   // skill id on keys 1-3 ("" = empty)
	Tree    []TreeSkill `json:"tree"`    // the class's 6 skills in tree order
	Points  int         `json:"points"`  // unspent skill points
	Respec  bool        `json:"respec"`  // at the stairs: the tree can be reset
	Gold    int         `json:"gold"`
	XP      int         `json:"xp"`
	XPNeed  int         `json:"xpNeed"`
	Lvl     int         `json:"lvl"`
	Trial   *Trial      `json:"trial,omitempty"` // a god's trial holds the next level
	Oil     int         `json:"oil"`             // seconds of light left in the lantern
	OilMax  int         `json:"oilMax"`          // seconds in a full lantern
	Weapon  *GearView   `json:"weapon,omitempty"`
	Trinket *GearView   `json:"trinket,omitempty"`
	Wares   []*GearView `json:"wares,omitempty"` // in the village: Grimble's gear for you (null = sold)
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
	Orbs    [][3]int     `json:"orbs,omitempty"`   // light orbs: x, y, seconds left
	Portal  *[3]int      `json:"portal,omitempty"` // Scroll of Return portal: x, y, seconds left
	Fires   [][2]int     `json:"fires,omitempty"`  // burning ground
	Traps   [][3]int     `json:"traps,omitempty"`  // spike traps the party has seen: x, y, 1 = armed
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

// QuestList is the party's quests (sent when they change, and on join). Delivered as an event.
type QuestList struct {
	T    string      `json:"t"`
	List []QuestInfo `json:"list"`
}

type QuestInfo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Giver    string `json:"giver"` // board, grimble
	Kind     string `json:"kind"`  // hunt, rescue, fetch, collect
	Depth    int    `json:"depth"`
	State    string `json:"state"` // offered, active, done, failed
	Progress int    `json:"progress"`
	Count    int    `json:"count"`
	Gold     int    `json:"gold"`
	XP       int    `json:"xp"`
	Point    bool   `json:"point"`
}
