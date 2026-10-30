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

	PotionHealFrac = 0.40
	PotionCD       = 10 * TickHz
	StartPotions   = 2
	MaxPotions     = 9

	// Co-op rules
	ThreatWindow   = 5 * TickHz
	DownedTicks    = 20 * TickHz
	ReviveTicks    = 3 * TickHz
	ReviveHPFrac   = 0.30
	StairsWindow   = 3 * TickHz
	StairsReach    = 1 // Chebyshev distance counted as "next to >"
	AggroRange     = 18.0
	LeashRange     = 34.0
	ClearedReturn  = 6 * TickHz
	IronWallThreat = 60
	GoldBase       = 6
)

// Scaling with number of players at run start.
func EnemyHPMult(n int) float64 { return 1 + 0.6*float64(n-1) }
func EliteHPMult(n int) float64 { return 1 + 0.8*float64(n-1) }
func PackSize(base, n int) int  { return base + n - 1 }

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
	Regen     float64 // per second
	GainDeal  int     // resource per hit dealt
	GainTaken int     // resource per hit taken
	Skills    [3]SkillDef
}

var Classes = map[ClassID]*ClassDef{
	Warrior: {ID: Warrior, Name: "WARRIOR", HP: 160, ResName: "Rage", ResShort: "RG", ResMax: 100, ResStart: 0, Regen: -1.5, GainDeal: 8, GainTaken: 6,
		Skills: [3]SkillDef{
			{ID: "cleave", Name: "CLEAVE", Cost: 0, CD: Sec(0.8), Range: 1.5, Dmg: 16, Melee: true},
			{ID: "ironwall", Name: "IRON WALL", Cost: 25, CD: Sec(12), Dmg: 0, Dur: Sec(5), Aux: 60},
			{ID: "leap", Name: "LEAP", Cost: 20, CD: Sec(8), Range: 14, Radius: 3.5, Dmg: 14},
		}},
	Rogue: {ID: Rogue, Name: "ROGUE", HP: 100, ResName: "Energy", ResShort: "EN", ResMax: 100, ResStart: 100, Regen: 14, GainDeal: 0, GainTaken: 0,
		Skills: [3]SkillDef{
			{ID: "stab", Name: "STAB", Cost: 15, CD: Sec(0.5), Range: 1.5, Dmg: 14, Melee: true},
			{ID: "caltrops", Name: "CALTROPS", Cost: 30, CD: Sec(10), Range: 12, Radius: 1, Dmg: 2, Dur: Sec(6)},
			{ID: "shadowbomb", Name: "SHADOW BOMB", Cost: 40, CD: Sec(9), Range: 16, Radius: 5, Dmg: 18, Delay: 8},
		}},
	Sorcerer: {ID: Sorcerer, Name: "SORCERER", HP: 80, ResName: "Mana", ResShort: "MP", ResMax: 100, ResStart: 100, Regen: 6, GainDeal: 0, GainTaken: 0,
		Skills: [3]SkillDef{
			{ID: "spark", Name: "SPARK", Cost: 5, CD: Sec(0.6), Range: 16, Dmg: 12},
			{ID: "nova", Name: "EMBER NOVA", Cost: 40, CD: Sec(8), Range: 18, Radius: 9, Dmg: 26, Delay: 9},
			{ID: "blink", Name: "BLINK", Cost: 20, CD: Sec(6), Range: 14},
		}},
	Monk: {ID: Monk, Name: "MONK", HP: 130, ResName: "Ki", ResShort: "KI", ResMax: 100, ResStart: 40, Regen: -1, GainDeal: 10, GainTaken: 0,
		Skills: [3]SkillDef{
			{ID: "palm", Name: "PALM STRIKE", Cost: 10, CD: Sec(1.5), Range: 1.5, Dmg: 16, Aux: 2, Melee: true},
			{ID: "dash", Name: "DASH", Cost: 15, CD: Sec(5), Range: 5, Dmg: 6, Aux: 4, Melee: true},
			{ID: "bell", Name: "BELL OF WARDING", Cost: 40, CD: Sec(20), Radius: 6, Dur: Sec(12), Aux: 30},
		}},
}

func HPPerLevel(base int) int { return base / 8 }
func XPNeed(lvl int) int      { return 40 * lvl }

type MonDef struct {
	Name      string
	HP        int
	Dmg       int
	MoveTicks int
	AtkTicks  int
	XP        int
}

var MonDefs = map[byte]MonDef{
	'z': {Name: "Zombie", HP: 34, Dmg: 6, MoveTicks: 7, AtkTicks: 14, XP: 8},
	's': {Name: "Skeleton", HP: 24, Dmg: 7, MoveTicks: 5, AtkTicks: 12, XP: 8},
	'f': {Name: "Fiend", HP: 16, Dmg: 5, MoveTicks: 3, AtkTicks: 10, XP: 6},
	'b': {Name: "Bat", HP: 10, Dmg: 4, MoveTicks: 3, AtkTicks: 10, XP: 4},
	'K': {Name: "The Tallow King", HP: 260, Dmg: 16, MoveTicks: 5, AtkTicks: 14, XP: 120},
}

type Difficulty struct {
	Name    string
	DmgMult float64
}

var Difficulties = []Difficulty{{"NORMAL", 1.0}, {"NIGHTMARE", 1.5}}

// Loot
const (
	GoldDropChance   = 0.5
	PotionDropChance = 0.18 // rolled per player on every kill
)

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
