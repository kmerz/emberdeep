package game

import "math"

// All gameplay numbers live in this file.

const (
	TickHz = 20

	LW = 176 // level width
	LH = 62  // level height

	VisionRadius = 26.0 // cells, in the aspect-corrected metric (y counts double)
	LightRadius  = 16.0 // client-side hero light radius (informational)

	MoveTicks     = 3 // ticks between hero steps (150 ms)
	MoveTolerance = 1 // ticks of early arrival we forgive (network jitter)

	MaxHeroes = 4

	PotionCD     = 10 * TickHz // shared cooldown of healing potions and draughts
	StartPotions = 2
	StartTorches = 2

	// Co-op rules
	ThreatWindow   = 5 * TickHz
	DownedTicks    = 20 * TickHz
	ReviveTicks    = 3 * TickHz
	ReviveHPFrac   = 0.30
	StairsWindow   = 3 * TickHz
	StairsReach    = 1 // Chebyshev distance counted as "next to >"
	AggroRange     = 18.0
	PathSearch     = 2500 // cells a monster's path search may look at (enough for ~24 cells across open ground)
	LeashRange     = 34.0
	ClearedReturn  = 6 * TickHz
	IronWallThreat = 60
	GoldBase       = 6
)

// Scaling with number of players at run start.
func EnemyHPMult(n int) float64 { return 1 + 0.6*float64(n-1) }
func EliteHPMult(n int) float64 { return 1 + 0.8*float64(n-1) }
func PackSize(base, n int) int  { return base + n - 1 }

// Floors. A run goes down MaxDepth floors; the stairs on the last one end it.
const (
	MaxDepth     = 10
	BossEvery    = 5   // a boss guards every 5th floor
	FallenHPFrac = 0.5 // dead and downed heroes stand up with this much HP on the next floor
)

type FloorDef struct {
	Theme     *Theme
	Kinds     string // monster kinds rolled for packs
	Boss      byte   // 0 = no boss on this floor
	PackBonus int    // extra monsters per room
}

// Theme is the look and population of a band of floors. The client has a matching palette per ID.
type Theme struct {
	ID, Name    string
	Kinds       string  // monster mix
	Caves       bool    // irregular cave rooms instead of rectangles
	TorchRate   float64 // share of torches kept (1 = all)
	Decor       string  // decor tiles rolled on floor cells
	DecorChance float64
	Glow        byte // decor tile that gives off light (0 = none)
	Dark        bool // pitch black: heroes only see what a light reaches
	Boss        byte // this theme's own boss (0 = the usual one on every 5th floor)
}

var Themes = map[string]*Theme{
	"cathedral": {ID: "cathedral", Name: "Cathedral of Ash", Kinds: "zzzssfbr", TorchRate: 1, Decor: "~:%", DecorChance: 0.03},
	"crypt":     {ID: "crypt", Name: "Flooded Crypt", Kinds: "zzzzsssbohn", TorchRate: 0.6, Decor: "==:=%", DecorChance: 0.06},
	"caverns":   {ID: "caverns", Name: "Fungal Caverns", Kinds: "bbbbfzsoorS", Caves: true, Decor: "\":\"%", DecorChance: 0.04, Glow: '"'},
	"dark":      {ID: "dark", Name: "The Black Hollow", Kinds: "bbbzzsorrSS", Caves: true, Decor: ":%~", DecorChance: 0.03, Dark: true},
	"hive":      {ID: "hive", Name: "The Hive", Kinds: "aaaaaaAA", Caves: true, TorchRate: 0.2, Decor: ":%%", DecorChance: 0.05, Boss: 'Q'},
	"village":   {ID: "village", Name: "Gnome Village", TorchRate: 1, Decor: ":", DecorChance: 0.01},
	"abyss":     {ID: "abyss", Name: "The Abyss Gate", Kinds: "SSUIfbozh", Caves: true, TorchRate: 0, Decor: "^:%", DecorChance: 0.03, Glow: '^'},
	"ember":     {ID: "ember", Name: "Ember Depths", Kinds: "ffiiddGhnUIs", TorchRate: 0.4, Decor: "^^:%~", DecorChance: 0.04, Glow: '^'},
}

// ThemeBands: a theme covers floors from From until the next band starts.
var ThemeBands = []struct {
	From  int
	Theme string
}{{1, "cathedral"}, {3, "crypt"}, {4, "hive"}, {5, "crypt"}, {6, "caverns"}, {7, "dark"}, {8, "abyss"}, {9, "ember"}}

// Light on dark floors (radii in the aspect-corrected metric, like VisionRadius).
const (
	BareLight  = 4.5  // a hero without a torch sees only the cells next to them
	TorchLight = 12.0 // a lit torch
	OrbLight   = 11.0 // a light orb
	TorchTicks = 120 * TickHz
	OrbTicks   = 60 * TickHz
	MaxOrbs    = 8 // per floor at once; the oldest goes out first
)

func ThemeAt(depth int) *Theme {
	t := Themes[ThemeBands[0].Theme]
	for _, b := range ThemeBands {
		if depth >= b.From {
			t = Themes[b.Theme]
		}
	}
	return t
}

// MaxGlows caps the light-giving decor per floor (each one is a light on the client).
const MaxGlows = 120

func Floor(depth int) FloorDef {
	th := ThemeAt(depth)
	f := FloorDef{Theme: th, Kinds: th.Kinds, PackBonus: (depth - 1) / 3}
	if depth%BossEvery == 0 {
		f.Boss = 'K'
	}
	if th.Boss != 0 {
		f.Boss = th.Boss
	}
	if depth == MaxDepth {
		f.Boss = FinalBoss
	}
	return f
}

// Scaling with depth (floor 1 is 1.0).
func DepthHPMult(d int) float64  { return 1 + 0.15*float64(d-1) }
func DepthDmgMult(d int) float64 { return 1 + 0.08*float64(d-1) }
func DepthXPMult(d int) float64  { return 1 + 0.10*float64(d-1) }

func Sec(s float64) int { return int(math.Round(s * TickHz)) }

type ClassID string

const (
	Warrior  ClassID = "warrior"
	Rogue    ClassID = "rogue"
	Sorcerer ClassID = "sorcerer"
	Monk     ClassID = "monk"
)

var AllClasses = []ClassID{Warrior, Rogue, Sorcerer, Monk}

func ValidClass(c string) bool {
	for _, k := range AllClasses {
		if string(k) == c {
			return true
		}
	}
	return false
}

type SkillDef struct {
	ID     string
	Name   string
	Cost   int
	CD     int     // ticks
	Range  float64 // aspect-corrected cells; 0 = self
	Radius float64
	Dmg    int
	Dur    int // ticks
	Aux    int // skill specific (knockback cells, absorb, dash cells...)
	Delay  int // ticks between cast and impact
	Melee  bool
}

type ClassDef struct {
	ID        ClassID
	Name      string
	HP        int
	ResName   string
	ResShort  string
	ResMax    int
	ResStart  int
	Regen     float64            // per second
	GainDeal  int                // resource per hit dealt
	GainTaken int                // resource per hit taken
	Skills    [TreeSize]SkillDef // index 0-2 the original skills, 3-5 the newer ones; tier = index%3 + 1
}

// Skill tree
const (
	TreeSize   = 6
	MaxRank    = 5
	RankDmg    = 0.15 // +15% damage per rank above 1
	RankDur    = 0.10 // +10% duration
	RankCD     = 0.05 // -5% cooldown
	CryBuffPct = 20   // War Cry: +20% damage for heroes in range
)

// SkillTiers is the hero level that unlocks each tier.
var SkillTiers = [3]int{1, 3, 8}

func SkillTier(idx int) int { return idx%3 + 1 }

// Ranked returns the skill as it works at the given rank.
func (s SkillDef) Ranked(rank int) SkillDef {
	r := float64(max(rank, 1) - 1)
	s.Dmg = int(math.Round(float64(s.Dmg) * (1 + RankDmg*r)))
	s.Dur = int(math.Round(float64(s.Dur) * (1 + RankDur*r)))
	s.CD = int(math.Round(float64(s.CD) * (1 - RankCD*r)))
	return s
}

var Classes = map[ClassID]*ClassDef{
	Warrior: {ID: Warrior, Name: "WARRIOR", HP: 160, ResName: "Rage", ResShort: "RG", ResMax: 100, ResStart: 0, Regen: -1.5, GainDeal: 8, GainTaken: 6,
		Skills: [TreeSize]SkillDef{
			{ID: "cleave", Name: "CLEAVE", Cost: 0, CD: Sec(0.8), Range: 1.5, Dmg: 16, Melee: true},
			{ID: "ironwall", Name: "IRON WALL", Cost: 25, CD: Sec(12), Dmg: 0, Dur: Sec(5), Aux: 60},
			{ID: "leap", Name: "LEAP", Cost: 20, CD: Sec(8), Range: 14, Radius: 3.5, Dmg: 14},
			{ID: "shieldbash", Name: "SHIELD BASH", Cost: 10, CD: Sec(4), Range: 1.5, Dmg: 8, Dur: Sec(2), Melee: true},
			{ID: "warcry", Name: "WAR CRY", Cost: 20, CD: Sec(15), Radius: 8, Dur: Sec(8), Aux: CryBuffPct},
			{ID: "whirlwind", Name: "WHIRLWIND", Cost: 25, CD: Sec(3), Range: 1.5, Dmg: 14, Melee: true},
		}},
	Rogue: {ID: Rogue, Name: "ROGUE", HP: 100, ResName: "Energy", ResShort: "EN", ResMax: 100, ResStart: 100, Regen: 14, GainDeal: 0, GainTaken: 0,
		Skills: [TreeSize]SkillDef{
			{ID: "stab", Name: "STAB", Cost: 15, CD: Sec(0.5), Range: 1.5, Dmg: 14, Melee: true},
			{ID: "caltrops", Name: "CALTROPS", Cost: 30, CD: Sec(10), Range: 12, Radius: 1, Dmg: 2, Dur: Sec(6)},
			{ID: "shadowbomb", Name: "SHADOW BOMB", Cost: 40, CD: Sec(9), Range: 16, Radius: 5, Dmg: 18, Delay: 8},
			{ID: "poisonblade", Name: "POISON BLADE", Cost: 25, CD: Sec(10), Dmg: 3, Dur: Sec(5), Aux: 3},
			{ID: "smokestep", Name: "SMOKE STEP", Cost: 35, CD: Sec(14), Dur: Sec(3)},
			{ID: "knives", Name: "FAN OF KNIVES", Cost: 30, CD: Sec(4), Range: 6, Dmg: 10, Aux: 5, Melee: true}, // the cursor only sets the direction
		}},
	Sorcerer: {ID: Sorcerer, Name: "SORCERER", HP: 80, ResName: "Mana", ResShort: "MP", ResMax: 100, ResStart: 100, Regen: 6, GainDeal: 0, GainTaken: 0,
		Skills: [TreeSize]SkillDef{
			{ID: "spark", Name: "SPARK", Cost: 5, CD: Sec(0.6), Range: 16, Dmg: 12},
			{ID: "nova", Name: "EMBER NOVA", Cost: 40, CD: Sec(8), Range: 18, Radius: 9, Dmg: 26, Delay: 9},
			{ID: "blink", Name: "BLINK", Cost: 20, CD: Sec(6), Range: 14},
			{ID: "frostring", Name: "FROST RING", Cost: 25, CD: Sec(8), Radius: 7, Dmg: 4, Dur: Sec(4)},
			{ID: "light", Name: "LIGHT", Cost: 15, CD: Sec(10), Range: 16, Dur: Sec(45)},
			{ID: "chainspark", Name: "CHAIN SPARK", Cost: 30, CD: Sec(3), Range: 16, Radius: 6, Dmg: 14, Aux: 3},
		}},
	Monk: {ID: Monk, Name: "MONK", HP: 130, ResName: "Ki", ResShort: "KI", ResMax: 100, ResStart: 40, Regen: -1, GainDeal: 10, GainTaken: 0,
		Skills: [TreeSize]SkillDef{
			{ID: "palm", Name: "PALM STRIKE", Cost: 10, CD: Sec(1.5), Range: 1.5, Dmg: 16, Aux: 2, Melee: true},
			{ID: "dash", Name: "DASH", Cost: 15, CD: Sec(5), Range: 5, Dmg: 6, Aux: 4, Melee: true},
			{ID: "bell", Name: "BELL OF WARDING", Cost: 40, CD: Sec(20), Radius: 6, Dur: Sec(12), Aux: 30},
			{ID: "staffsweep", Name: "STAFF SWEEP", Cost: 10, CD: Sec(1.2), Range: 2.5, Dmg: 12, Melee: true},
			{ID: "healbreath", Name: "HEALING BREATH", Cost: 35, CD: Sec(15), Radius: 6, Dmg: 25}, // Dmg = % of max HP healed
			{ID: "fists", Name: "HUNDRED FISTS", Cost: 30, CD: Sec(6), Range: 1.5, Dmg: 7, Aux: 5, Delay: 2, Melee: true},
		}},
}

func HPPerLevel(base int) int { return base / 8 }
func XPNeed(lvl int) int      { return 300 + 20*lvl } // floor 1 ends near level 3, a full run near 20

// MonsterSpeedScale stretches every monster's step time so nothing out-runs
// the heroes (hero step = MoveTicks). Raise it to slow monsters further.
const MonsterSpeedScale = 1.25

// StepTicks is the ticks a monster takes per step (MoveTicks, scaled).
func (d MonDef) StepTicks() int { return int(math.Round(float64(d.MoveTicks) * MonsterSpeedScale)) }

type MonDef struct {
	Name      string
	HP        int
	Dmg       int
	MoveTicks int
	AtkTicks  int
	XP        int
	Role      string // "" chaser, "caller" keeps away and summons, "queen" summons and never moves, "splitter" splits when killed
	Summon    byte   // what a caller or queen raises
	Neutral   bool   // leaves heroes alone until its faction is alarmed
	Faction   string // hitting one alarms the others nearby ("" = none)

	// deep monsters
	Range   float64 // "ranged": shoots from this far; "charmer": charms from this far
	KeepOff bool    // "ranged": backs away from heroes that get close
	Trail   bool    // leaves burning ground where it walks
	Stealth bool    // unseen unless a hero is within StealthReveal cells
}

// Deep monsters
const (
	StealthReveal = 3          // a shade shows itself this close
	FireTicks     = 4 * TickHz // burning ground lasts this long (golem trail)
	FireDmg       = 4          // per second, to heroes standing in it
	BreathWind    = 20         // ticks the dragon winds up before it breathes (you can step aside)
	BreathCD      = 8 * TickHz // between breaths
	BreathRange   = 9.0        // cone length
	BreathFire    = 5 * TickHz // the cone keeps burning this long
	CharmTicks    = 2 * TickHz // a charmed hero can't act and drifts toward the charmer
	CharmCD       = 10 * TickHz
	DrainRes      = 15 // an incubus bite drains this much resource and heals itself by it
	BlinkCD       = 3 * TickHz
	BlinkChance   = 0.35 // an incubus that's hit blinks away with this chance
	FinalBoss     = 'D'  // the last floor's boss
)

// Monster roles
const (
	CallerEvery   = 5 * TickHz // a caller raises one minion this often while it sees a hero
	CallerMax     = 3          // minions alive per caller
	CallerRange   = 14.0       // it summons while a hero is this close
	CallerKeepOff = 3          // it backs away from heroes this close (cells)
	CallerHold    = 9.0        // and doesn't approach closer than this
	SplitInto     = 2          // a blob splits into this many bloblings
	AlarmRange    = 12.0       // hitting an ant alarms every ant this close
	QueenEvery    = 6 * TickHz // the queen lays a soldier this often while she sees a hero
	QueenMax      = 4          // soldiers alive per queen
	QueenRange    = 16.0
)

var MonDefs = map[byte]MonDef{
	'z':     {Name: "Zombie", HP: 34, Dmg: 6, MoveTicks: 7, AtkTicks: 14, XP: 8},
	's':     {Name: "Skeleton", HP: 24, Dmg: 7, MoveTicks: 5, AtkTicks: 12, XP: 8},
	'f':     {Name: "Fiend", HP: 16, Dmg: 5, MoveTicks: 3, AtkTicks: 10, XP: 6},
	'b':     {Name: "Bat", HP: 10, Dmg: 4, MoveTicks: 3, AtkTicks: 10, XP: 4},
	'K':     {Name: "The Tallow King", HP: 260, Dmg: 16, MoveTicks: 5, AtkTicks: 14, XP: 120},
	'M':     {Name: "Mimic", HP: 70, Dmg: 11, MoveTicks: 6, AtkTicks: 12, XP: 30},
	'r':     {Name: "Bone Rat", HP: 8, Dmg: 3, MoveTicks: 2, AtkTicks: 8, XP: 2},                                    // fast and fragile
	'h':     {Name: "Bone Hulk", HP: 90, Dmg: 16, MoveTicks: 9, AtkTicks: 20, XP: 22},                               // slow, hits hard
	'n':     {Name: "Bone Caller", HP: 30, Dmg: 4, MoveTicks: 5, AtkTicks: 14, XP: 18, Role: "caller", Summon: 'r'}, // raises bone rats
	'a':     {Name: "Worker Ant", HP: 20, Dmg: 4, MoveTicks: 6, AtkTicks: 12, XP: 4, Neutral: true, Faction: "ants"},
	'A':     {Name: "Soldier Ant", HP: 40, Dmg: 9, MoveTicks: 4, AtkTicks: 12, XP: 12, Faction: "ants"},
	'Q':     {Name: "The Ant Queen", HP: 300, Dmg: 14, MoveTicks: 99, AtkTicks: 16, XP: 140, Role: "queen", Summon: 'A', Faction: "ants"},
	'i':     {Name: "Imp", HP: 22, Dmg: 7, MoveTicks: 3, AtkTicks: 22, XP: 14, Role: "ranged", Range: 9, KeepOff: true},            // fast, shoots firebolts, keeps away
	'd':     {Name: "Drake", HP: 70, Dmg: 10, MoveTicks: 5, AtkTicks: 26, XP: 26, Role: "ranged", Range: 7},                        // spits fire, bites up close
	'G':     {Name: "Magma Golem", HP: 140, Dmg: 15, MoveTicks: 10, AtkTicks: 20, XP: 34, Trail: true},                             // slow, tough, burning trail
	'S':     {Name: "Shade", HP: 26, Dmg: 13, MoveTicks: 4, AtkTicks: 12, XP: 18, Stealth: true},                                   // invisible until close
	'U':     {Name: "Succubus", HP: 60, Dmg: 9, MoveTicks: 4, AtkTicks: 14, XP: 30, Role: "charmer", Range: 7},                     // charms a hero
	'I':     {Name: "Incubus", HP: 60, Dmg: 9, MoveTicks: 4, AtkTicks: 14, XP: 30, Role: "drainer"},                                // drains resource, blinks away
	'D':     {Name: "The Ember Dragon", HP: 520, Dmg: 22, MoveTicks: 7, AtkTicks: 18, XP: 400, Role: "dragon", Range: BreathRange}, // breathes fire
	KegKind: {Name: "Powder Keg", HP: 1, Dmg: 0, MoveTicks: 99, AtkTicks: 99, XP: 0, Role: "keg"},                                  // explodes when hit (world.go)
	'o':     {Name: "Blob", HP: 44, Dmg: 6, MoveTicks: 8, AtkTicks: 14, XP: 10, Role: "splitter"},                                  // splits when killed
}

// KegKind is the powder keg: a "monster" that never acts and blows up when broken.
const KegKind = 'B'

type Difficulty struct {
	Name    string
	DmgMult float64 // monster damage
	HPMult  float64 // monster hit points
}

// Difficulties, in the order the host's D key cycles them. Index 0 is the default.
var Difficulties = []Difficulty{
	{"NORMAL", 1.0, 1.0},
	{"NIGHTMARE", 1.5, 1.0},
	{"NOVICE", 0.6, 0.7}, // for learning the game: monsters hit softer and die sooner
}

// Loot
const GoldDropChance = 0.5

// Chests
const (
	ChestRoomChance = 0.35 // chance a room (not the start room) holds a chest
	ChestRolls      = 2    // item rolls per connected hero when a chest opens (or a mimic dies)
	ChestGold       = 25   // base gold in a chest, plus up to the same again
	MimicBite       = 1.5  // the surprise bite does this much of a normal hit
)

// MimicChance is the share of chests that are mimics on a floor (none on floor 1).
func MimicChance(depth int) float64 {
	if depth <= 1 {
		return 0
	}
	return min(0.4, 0.08+0.03*float64(depth))
}

// Gnome village and the Scroll of Return
const (
	VillageAfter = 5           // the village is an extra floor between floor 5 and floor 6
	PortalTicks  = 30 * TickHz // a Scroll of Return's portal stays open this long
	PortalWindow = 3 * TickHz  // the party gathers in it this long to go through
	NpcMoveTicks = 12
	NpcWander    = 6 // cells a gnome strays from its spot
)

// ShopStock is what Grimble sells, and for how much gold.
var ShopStock = []struct {
	Item  string
	Price int
}{{"potion", 30}, {"draught", 30}, {"torch", 15}, {"lightorb", 40}, {"firebomb", 60}, {"returnscroll", 120}, {"satchel", SatchelPrice}}

// SellPrice is what Grimble pays for one item: its Value if it has one, else half the shop price.
func SellPrice(id string) int {
	if d := Items[id]; d != nil && d.Value > 0 {
		return d.Value
	}
	for _, s := range ShopStock {
		if s.Item == id {
			return max(1, s.Price/2)
		}
	}
	return 1
}

// Rucksack
const (
	PackSlots     = 16 // the most a rucksack can ever hold (with the Gnome Satchel)
	StartPackCap  = 12 // slots a hero starts with
	SatchelSlots  = 4  // the Gnome Satchel adds this many, once
	SatchelPrice  = 200
	ThrowDelay    = 8      // ticks a thrown item flies before it lands
	PackFullEvery = TickHz // at most one "rucksack full" message per second
)

type ItemDef struct {
	ID, Name, Desc string
	Glyph          byte // plain-ASCII fallback; the client draws its own glyph by ID
	MaxStack       int
	Use            string  // "heal", "resource", "throw", "torch", "orb"; "" = can't be used
	Amount         float64 // heal/resource share of the maximum, or damage for "throw"
	Radius, Range  float64
	Value          int     // gold value (for trading later)
	DropChance     float64 // rolled per player on every kill; elites always drop potions
	Quest          bool    // a quest item: can't be sold or dropped
}

var Items = map[string]*ItemDef{
	"potion":       {ID: "potion", Name: "Healing Potion", Desc: "restores 40% HP", Glyph: '!', MaxStack: 9, Use: "heal", Amount: 0.40, DropChance: 0.18},
	"draught":      {ID: "draught", Name: "Resource Draught", Desc: "refills half your resource", Glyph: '&', MaxStack: 9, Use: "resource", Amount: 0.50, DropChance: 0.12},
	"firebomb":     {ID: "firebomb", Name: "Firebomb", Desc: "throw at the cursor: fire blast", Glyph: '*', MaxStack: 5, Use: "throw", Amount: 22, Radius: 3, Range: 12, DropChance: 0.04},
	"torch":        {ID: "torch", Name: "Torch", Desc: "burns for 2 minutes; you see in the dark", Glyph: 't', MaxStack: 5, Use: "torch", DropChance: 0.06},
	"lightorb":     {ID: "lightorb", Name: "Scroll of Light", Desc: "lights the spot at the cursor for a minute", Glyph: '?', MaxStack: 5, Use: "orb", Range: 14, DropChance: 0.04},
	"returnscroll": {ID: "returnscroll", Name: "Scroll of Return", Desc: "opens a portal back to the gnome village", Glyph: '%', MaxStack: 3, Use: "portal"}, // sold only, never dropped
	"ledger":       {ID: "ledger", Name: "The Tallow King's Ledger", Desc: "a list of every soul he melted; for Grimble", Glyph: '=', MaxStack: 1, Quest: true},
	"spoon":        {ID: "spoon", Name: "Grimble's Lucky Spoon", Desc: "it is, admittedly, a very nice spoon", Glyph: '/', MaxStack: 1, Quest: true},
	"antegg":       {ID: "antegg", Name: "Ant Egg", Desc: "warm and faintly ticking; for Grimble", Glyph: 'o', MaxStack: 9, Quest: true},
	"embershard":   {ID: "embershard", Name: "Ember Shard", Desc: "too hot to hold for long; for Grimble", Glyph: '*', MaxStack: 9, Quest: true},
	"gem":          {ID: "gem", Name: "Garnet", Desc: "worth 40 gold to a trader", Glyph: 'g', MaxStack: 20, Value: 40, DropChance: 0.05},
}

// ItemOrder fixes the order drops are rolled in (map order is random, the simulation must not be).
var ItemOrder = []string{"potion", "draught", "firebomb", "torch", "lightorb", "gem"}

// dist is the aspect-corrected metric used for radii and FOV (cells are twice as tall as wide).
func dist(ax, ay, bx, by int) float64 {
	dx := float64(ax - bx)
	dy := float64(ay-by) * 2
	return math.Sqrt(dx*dx + dy*dy)
}

func cheb(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// World dressing (world.go): furniture, urns, powder kegs, spike traps and lava pools, per theme.
type Dressing struct {
	Furniture   byte    // a low set piece that blocks the way (0 = none)
	FurnPerRoom int     // up to this many per room
	UrnChance   float64 // chance a room holds 1-2 urns
	KegChance   float64 // chance a room holds a powder keg
	Traps       int     // hidden spike traps per floor (from floor 2 on)
	LavaPools   int     // pools of burning ground that never go out
}

var Dressings = map[string]Dressing{
	"cathedral": {Furniture: TCandle, FurnPerRoom: 2, UrnChance: 0.5, KegChance: 0.25, Traps: 4},
	"crypt":     {Furniture: TCoffin, FurnPerRoom: 2, UrnChance: 0.6, KegChance: 0.2, Traps: 5},
	"caverns":   {Furniture: TStalk, FurnPerRoom: 3, UrnChance: 0.3, KegChance: 0.15, Traps: 5},
	"dark":      {UrnChance: 0.4, KegChance: 0.2, Traps: 6},
	"hive":      {Traps: 3},
	"abyss":     {UrnChance: 0.3, KegChance: 0.25, Traps: 6, LavaPools: 3},
	"ember":     {UrnChance: 0.3, KegChance: 0.35, Traps: 6, LavaPools: 5},
}

const (
	UrnGoldChance = 0.4  // a smashed urn spills a little gold...
	UrnItemChance = 0.12 // ...or, rarely, an item for whoever smashed it

	KegFuse      = 8          // ticks between a keg breaking and its blast (so chains ripple)
	KegRadius    = 3.5        // blast radius
	KegDmg       = 40         // to monsters (scaled like monster hp with depth)
	KegHeroDmg   = 14         // to heroes caught in it (scaled like monster damage)
	KegFireTicks = 3 * TickHz // the blast leaves burning ground

	TrapDmg    = 10         // spikes, to a hero (scaled like monster damage)
	TrapMonDmg = 25         // spikes, to a monster lured over them (scaled like monster hp)
	TrapRearm  = 3 * TickHz // a sprung trap resets after this long
	LavaDmg    = 6          // per second, standing in a lava pool

	BarkChance   = 0.12        // chance an ordinary monster says something when it notices you
	BarkGap      = 5 * TickHz  // at most one ordinary monster line per this long
	GnomeChatter = 25 * TickHz // a gnome says something to a passing hero at most this often
)
