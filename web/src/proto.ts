// Mirrors internal/proto/msg.go. Keep field names in sync.
export type ClassId = 'warrior' | 'rogue' | 'sorcerer' | 'monk';

export interface Status { k: string; t: number }
export interface Ent {
  id: number; k: 'h' | 'm' | 'i'; g: string; x: number; y: number;
  hp?: number; hpMax?: number; cls?: ClassId; name?: string;
  downed?: boolean; dead?: boolean; downT?: number; rev?: number; el?: boolean; face: number; st?: Status[];
}
export interface PartyEntry {
  id: number; name: string; cls: ClassId; lvl: number; hp: number; hpMax: number; res: number; resMax: number;
  st?: Status[]; downed?: boolean; dead?: boolean; downT?: number; connected: boolean;
}
export interface Vis { x0: number; y0: number; w: number; h: number; b: string }
export interface Objectives {
  kingAlive: boolean; stairsAt: number; stairsNeed: number; stairsLeft: number; stairsFound: boolean; gold: number; kills: number;
}
export interface Me {
  res: number; resMax: number; cd: number[]; cdMax: number[]; cost: number[]; potions: number; potCd: number;
  gold: number; xp: number; xpNeed: number; lvl: number;
}
export interface Snap {
  t: 'snap'; tick: number; ackSeq?: number; vis: Vis; ents: Ent[]; party: PartyEntry[]; obj: Objectives;
  patches?: [number, number][]; me?: Me; drops?: Ent[];
}
export interface GameEvent {
  k: string; actor?: number; target?: number; x: number; y: number; tx?: number; ty?: number;
  amount?: number; fx?: string; text?: string; r?: number;
}
export interface LobbyPlayer {
  slot: number; id: number; name: string; cls: ClassId; lvl: number; ready: boolean; pingMs: number;
  host: boolean; connected: boolean; bot?: boolean;
}
export interface LobbyMsg {
  t: 'lobby'; code: string; players: LobbyPlayer[]; inGame: boolean;
  settings: { difficulty: string; seed: string; loot: string; friendlyFire: boolean };
}
export interface LevelMsg { t: 'level'; w: number; h: number; seed: string; torches: [number, number][]; stairs: [number, number]; start: [number, number] }
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
  | { t: 'error'; msg: string };
