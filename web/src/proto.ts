// Mirrors internal/proto/msg.go. Keep field names in sync.
export type ClassId = 'warrior' | 'rogue' | 'sorcerer' | 'monk';

export interface Status { k: string; t: number }
export interface Ent {
  id: number; k: 'h' | 'm' | 'i' | 'n'; g: string; x: number; y: number; // n = a friendly gnome
  hp?: number; hpMax?: number; cls?: ClassId; name?: string;
  downed?: boolean; dead?: boolean; downT?: number; rev?: number; el?: boolean; face: number; st?: Status[];
  it?: string; // rucksack item id for items on the floor
  trader?: boolean; // gnomes: this one trades
  board?: boolean;  // the mayor's notice board
  follow?: boolean; // a lost gnome being rescued (hp/hpMax set)
  rar?: number; // items: rarity (2 = rare: a loot beam)
  nm?: string;  // items: a gear piece's name
}
export interface PartyEntry {
  id: number; name: string; cls: ClassId; lvl: number; hp: number; hpMax: number; res: number; resMax: number;
  st?: Status[]; downed?: boolean; dead?: boolean; downT?: number; connected: boolean;
}
export interface Vis { x0: number; y0: number; w: number; h: number; b: string }
export interface Objectives {
  boss: boolean; bossName?: string; village?: boolean; portalAt?: number; portalNeed?: number; kingAlive: boolean; stairsAt: number; stairsNeed: number; stairsLeft: number; stairsUp?: boolean; stairsFound: boolean; gold: number; kills: number;
}
export interface Me {
  res: number; resMax: number; cd: number[]; cdMax: number[]; cost: number[]; potions: number; potCd: number;
  gold: number; xp: number; xpNeed: number; lvl: number;
  pack: PackSlot[]; // always 16 entries; empty ones are { item: '', n: 0 }
  oil?: number; oilMax?: number; // seconds of light in the lantern, and in a full one
  trial?: { god: string; need: number; kills: number }; // a god's trial holds the next level
  packCap: number;  // usable slots: 12, or 16 with the Gnome Satchel
  binds: string[];  // skill id on keys 1-3 ('' = empty)
  tree: { id: string; rank: number }[]; // the class's 6 skills in tree order
  points: number;   // unspent skill points
  respec: boolean;  // at the stairs: the tree can be reset
  weapon?: GearView; trinket?: GearView; // what the hero wears
}
export interface PackSlot { item: string; n: number; g?: GearView /* a weapon or trinket */ }
/** A rolled weapon or trinket (internal/game/gear.go). */
export interface GearView {
  slot: 'weapon' | 'trinket'; base: string; name: string;
  rar: number; // 0 common, 1 magic, 2 rare
  lvl: number; mods: { k: string; v: number }[]; // the base stat first
  value: number; // what Grimble pays
}
export interface Snap {
  t: 'snap'; tick: number; ackSeq?: number; vis: Vis; ents: Ent[]; party: PartyEntry[]; obj: Objectives;
  patches?: [number, number][]; orbs?: [number, number, number][]; portal?: [number, number, number]; fires?: [number, number][]; me?: Me; drops?: Ent[];
  traps?: [number, number, number][]; // spike traps the party has seen: x, y, 1 = armed
}
export interface GameEvent {
  k: string; actor?: number; target?: number; x: number; y: number; tx?: number; ty?: number;
  amount?: number; fx?: string; text?: string; r?: number;
}
export interface LobbyPlayer {
  slot: number; id: number; name: string; cls: ClassId; lvl: number; ready: boolean; pingMs: number;
  host: boolean; connected: boolean; bot?: boolean;
  saved?: boolean; // continues their saved hero
}
/** A saved game in the lobby: the next start continues it. */
export interface SaveInfo { depth: number; village?: boolean; heroes: { name: string; cls: ClassId; lvl: number }[] }
export interface LobbyMsg {
  t: 'lobby'; code: string; players: LobbyPlayer[]; inGame: boolean; save?: SaveInfo;
  settings: { difficulty: string; seed: string; loot: string; friendlyFire: boolean };
}
export interface LevelMsg { t: 'level'; w: number; h: number; seed: string; torches: [number, number][]; stairs: [number, number]; up?: [number, number]; start: [number, number]; depth: number; maxDepth: number; theme: string; glows: [number, number][]; dark?: boolean; village?: boolean }
export type ServerMsg =
  | { t: 'welcome'; you: number; token: string; code: string; inviteUrl: string; isHost: boolean }
  | LobbyMsg
  | { t: 'phase'; phase: 'lobby' | 'game' }
  | LevelMsg
  | { t: 'explore'; tiles: [number, number, number][] }
  | Snap
  | { t: 'events'; tick: number; list: GameEvent[] }
  | { t: 'chat'; from: number; name: string; cls: ClassId; text: string }
  | { t: 'ping'; from: number; name: string; cls: ClassId; cell: [number, number] }
  | { t: 'hb'; n: number }
  | { t: 'overlay'; kind: 'wipe' | 'cleared' }
  | { t: 'error'; msg: string }
  | { t: 'quests'; list: QuestInfo[] };

export interface QuestInfo {
  id: string; title: string; text: string; giver: 'board' | 'grimble'; kind: 'hunt' | 'rescue' | 'fetch' | 'collect';
  depth: number; state: 'offered' | 'active' | 'done' | 'failed'; progress: number; count: number;
  gold: number; xp: number; point: boolean;
}
