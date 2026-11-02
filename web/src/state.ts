import type { Ent, GameEvent, LevelMsg, LobbyMsg, Me, Objectives, PartyEntry, ServerMsg, Snap, ClassId } from './proto';
import { CLS, SKILL_BY_ID } from './classes';
import { DIM, GOLD, PINK, RGB, TX } from './term/fb';
import { palette } from './render/themes';
import { itemInfo } from './items';

export const LW = 176;
export const LH = 62;

export type LogSeg = [string, RGB, number?];
export interface LogLine { ts: string; segs: LogSeg[]; at: number }
export interface PingMark { x: number; y: number; cls: ClassId; name: string; t0: number }

export interface SnapRec { t: number; snap: Snap; ents: Ent[] }

export const INTERP_MS = 100;

export function clock(tick: number): string {
  const s = Math.floor(tick / 20);
  return String(Math.floor(s / 60)).padStart(2, '0') + ':' + String(s % 60).padStart(2, '0');
}

export class GameState {
  youId = 0;
  code = '';
  inviteUrl = '';
  isHost = false;
  token = '';
  level: LevelMsg | null = null;
  tiles = new Uint8Array(LW * LH); // 0 = unexplored
  vis = new Uint8Array(LW * LH);
  recs: SnapRec[] = [];
  latest: Snap | null = null;
  latestAt = 0;
  party: PartyEntry[] = [];
  me: Me | null = null;
  obj: Objectives | null = null;
  tick = 0;
  ackSeq = 0;
  log: LogLine[] = [];
  pings: PingMark[] = [];
  overlay: 'wipe' | 'cleared' | null = null;
  levelAt = 0; // when the current level arrived (for the depth banner)
  lobby: LobbyMsg | null = null;
  phase: 'lobby' | 'game' = 'lobby';
  pingMs = 0;
  lastTarget = 0; // monster id last attacked by us
  errors: string[] = [];
  // hooks wired by the screens
  onEvents: (evs: GameEvent[], now: number) => void = () => {};
  onPing: (p: PingMark) => void = () => {};
  onSnap: (s: Snap) => void = () => {};

  reset() {
    this.level = null;
    this.tiles.fill(0);
    this.vis.fill(0);
    this.recs = [];
    this.latest = null;
    this.party = [];
    this.me = null;
    this.obj = null;
    this.pings = [];
    this.overlay = null;
    this.log = [];
    this.lastTarget = 0;
  }

  nameOf(id: number): string {
    const p = this.party.find((q) => q.id === id);
    if (p) return p.name;
    const l = this.lobby?.players.find((q) => q.id === id);
    return l ? l.name : '';
  }
  clsOf(id: number): ClassId | null {
    const p = this.party.find((q) => q.id === id);
    if (p) return p.cls;
    const l = this.lobby?.players.find((q) => q.id === id);
    return l ? l.cls : null;
  }
  colOf(id: number): RGB {
    const c = this.clsOf(id);
    return c ? CLS[c].col : TX;
  }
  tileAt(x: number, y: number): number {
    return x < 0 || y < 0 || x >= LW || y >= LH ? 0 : this.tiles[y * LW + x];
  }
  isVisible(x: number, y: number) {
    return x >= 0 && y >= 0 && x < LW && y < LH && this.vis[y * LW + x] === 1;
  }

  pushLog(segs: LogSeg[], now = performance.now()) {
    this.log.push({ ts: clock(this.tick), segs, at: now });
    if (this.log.length > 300) this.log.splice(0, this.log.length - 200);
  }

  /** Turns server events into log lines (FX are handled by the fx pipeline through onEvents). */
  logEvents(evs: GameEvent[], now: number) {
    for (const e of evs) {
      const an = e.actor && e.actor < 10000 ? this.nameOf(e.actor) : '';
      const ac = e.actor && e.actor < 10000 ? this.colOf(e.actor) : TX;
      const you = e.actor === this.youId;
      switch (e.k) {
        case 'cast': {
          if (e.fx === 'firebomb') { this.pushLog([[an, ac], [' throws a ', DIM], ['FIREBOMB', [255, 140, 50], 1]], now); break; }
          const sk = e.fx ? SKILL_BY_ID[e.fx] : undefined;
          if (sk && sk.meta.log) this.pushLog([[an, ac], [' casts ', DIM], [sk.meta.name, CLS[sk.cls].col]], now);
          break;
        }
        case 'kill':
          if (e.fx === 'K') this.pushLog([['The Tallow King', PINK, 1], [' falls to ', DIM], [an || 'the party', ac]], now);
          else if (e.fx === 'M') this.pushLog([['The mimic', PINK, 1], [' falls and spills its treasure', DIM]], now);
          break;
        case 'pickup':
          if (e.fx === 'gold') this.pushLog([[you ? 'You' : an, you ? TX : ac], [' pick' + (you ? '' : 's') + ' up ', DIM], [(e.amount || 0) + ' gold', GOLD, 1]], now);
          else if (you && e.fx) { const it = itemInfo(e.fx); this.pushLog([['You pick up ', DIM], [(e.amount || 1) > 1 ? e.amount + '× ' + it.name : it.name, it.col, 1]], now); }
          break;
        case 'packfull': if (you) this.pushLog([['Your rucksack is full', PINK]], now); break;
        case 'chest': this.pushLog([[you ? 'You' : an, you ? TX : ac], [you ? ' open a ' : ' opens a ', DIM], ['chest', GOLD, 1]], now); break;
        case 'summon':
          this.pushLog(e.fx === 'Q' ? [['The Ant Queen', [255, 170, 60]], [' lays a ', DIM], [e.text || 'soldier', TX]]
            : [['A Bone Caller', [180, 120, 255]], [' raises a ', DIM], [e.text || 'minion', TX]], now);
          break;
        case 'alarm': this.pushLog([['The hive is alarmed!', [255, 170, 60], 1], [' ' + (e.amount ?? 0) + ' ants turn on you', DIM]], now); break;
        case 'say': if (you && e.text) this.pushLog([[e.text, [140, 220, 120]]], now); break;
        case 'shop': if (you) this.pushLog([['Grimble', [255, 212, 92], 1], [': Have a look, have a look!', [140, 220, 120]]], now); break;
        case 'trade': if (you && e.fx) {
          const it = itemInfo(e.fx), amt = e.amount ?? 0;
          this.pushLog([[amt < 0 ? 'You buy a ' : 'You sell a ', DIM], [it.name, it.col], [' (' + (amt < 0 ? '' : '+') + amt + ' gold)', GOLD]], now);
          break;
        }
          break;
        case 'portal': this.pushLog([['A portal opens', [150, 230, 255], 1], [': gather in it to return to the village', DIM]], now); break;
        case 'village': this.pushLog([['You reach the ', DIM], ['Gnome Village', [140, 220, 120], 1], [': safe at last', DIM]], now); break;
        case 'split': this.pushLog([['The blob', [120, 230, 110]], [' splits!', DIM]], now); break;
        case 'mimic': this.pushLog([['The chest was a ', DIM], ['MIMIC', PINK, 1], ['!', DIM]], now); break;
        case 'use': {
          if (!e.fx) break;
          const it = itemInfo(e.fx), who: LogSeg = [you ? 'You' : an, you ? TX : ac];
          if (e.fx === 'torch') this.pushLog([who, [you ? ' light a ' : ' lights a ', DIM], ['torch', it.col]], now);
          else if (e.fx === 'lightorb') this.pushLog([who, [you ? ' read a ' : ' reads a ', DIM], [it.name, it.col]], now);
          else this.pushLog([who, [you ? ' drink a ' : ' drinks a ', DIM], [it.name, it.col]], now);
          break;
        }
        case 'drop': if (e.fx) { const it = itemInfo(e.fx); this.pushLog([[you ? 'You' : an, you ? TX : ac], [you ? ' drop ' : ' drops ', DIM], [(e.amount || 1) + '× ' + it.name, it.col]], now); } break;
        case 'down': this.pushLog([[this.nameOf(e.target || 0), this.colOf(e.target || 0)], [' is DOWN!', PINK, 1]], now); break;
        case 'revive': this.pushLog([[an, ac], [' revives ', DIM], [this.nameOf(e.target || 0), this.colOf(e.target || 0)]], now); break;
        case 'dead': this.pushLog([[this.nameOf(e.target || 0), this.colOf(e.target || 0)], [' has fallen', PINK]], now); break;
        case 'levelup':
          this.pushLog([[an, ac], [' reaches level ', DIM], [String(e.amount), GOLD, 1]], now);
          if (you) this.pushLog([['+1 skill point · open the skill tree with ', DIM], ['T', TX, 1]], now);
          break;
        case 'learn':
          if (you && e.fx) {
            const sk = SKILL_BY_ID[e.fx];
            if (sk) this.pushLog([[(e.amount ?? 1) > 1 ? 'You raise ' : 'You learn ', DIM], [sk.meta.name, CLS[sk.cls].col, 1], [(e.amount ?? 1) > 1 ? ' to rank ' + e.amount : '', DIM]], now);
          }
          break;
        case 'respec': if (you) this.pushLog([['Your skill points are back · spend them with ', DIM], ['T', TX, 1]], now); break;
        case 'absorb': break;
        case 'msg': this.pushLog([[e.text || '', TX]], now); break;
        case 'descend': this.pushLog([['The party descends to ', DIM], ['depth ' + (e.amount || 0), GOLD, 1]], now); break;
      }
      if (e.k === 'hit' && e.actor === this.youId && e.target && e.target >= 10000) this.lastTarget = e.target;
    }
  }

  applyVis(s: Snap) {
    this.vis.fill(0);
    const v = s.vis;
    if (!v || !v.w) return;
    const bin = atob(v.b);
    for (let y = 0; y < v.h; y++) {
      for (let x = 0; x < v.w; x++) {
        const i = y * v.w + x;
        if ((bin.charCodeAt(i >> 3) >> (i & 7)) & 1) this.vis[(v.y0 + y) * LW + v.x0 + x] = 1;
      }
    }
  }

  handle(m: ServerMsg, now: number) {
    switch (m.t) {
      case 'welcome':
        this.youId = m.you; this.code = m.code; this.inviteUrl = m.inviteUrl; this.isHost = m.isHost; this.token = m.token;
        break;
      case 'lobby':
        this.lobby = m;
        this.isHost = m.players.some((p) => p.id === this.youId && p.host);
        { const me = m.players.find((p) => p.id === this.youId); if (me && me.pingMs >= 0) this.pingMs = me.pingMs; }
        break;
      case 'phase':
        this.phase = m.phase;
        if (m.phase === 'game') this.reset();
        break;
      case 'level':
        // a new floor (or a retry): forget everything about the old map
        if (m.theme !== this.level?.theme) {
          this.pushLog([['You enter the ', DIM], [palette(m.theme).name, TX], [' · Depth ' + String(m.depth).padStart(2, '0'), DIM]], now);
          if (m.dark) this.pushLog([['It is pitch black. Light a torch from your rucksack (', DIM], ['I', TX, 1], [').', DIM]], now);
        }
        this.level = m; this.tiles.fill(0); this.vis.fill(0);
        this.recs = []; this.latest = null; this.pings = []; this.overlay = null; this.lastTarget = 0;
        this.levelAt = now;
        break;
      case 'explore':
        for (const [x, y, t] of m.tiles) this.tiles[y * LW + x] = t;
        break;
      case 'snap': {
        this.latest = m; this.latestAt = now; this.tick = m.tick;
        this.party = m.party; this.obj = m.obj;
        if (m.me) this.me = m.me;
        this.ackSeq = m.ackSeq ?? 0;
        this.applyVis(m);
        const ents = m.drops && m.drops.length ? m.ents.concat(m.drops) : m.ents;
        this.recs.push({ t: now, snap: m, ents });
        if (this.recs.length > 12) this.recs.shift();
        this.onSnap(m);
        break;
      }
      case 'events':
        this.logEvents(m.list, now);
        this.onEvents(m.list, now);
        break;
      case 'chat': {
        const c = CLS[m.cls]?.col ?? TX;
        this.pushLog([['<' + m.name + '> ', c], [m.text, TX]], now);
        break;
      }
      case 'ping': {
        const p: PingMark = { x: m.cell[0], y: m.cell[1], cls: m.cls, name: m.name, t0: now };
        this.pings.push(p);
        this.pushLog([[m.name, CLS[m.cls]?.col ?? TX], [' pinged ⟶ here', DIM]], now);
        this.onPing(p);
        break;
      }
      case 'overlay': this.overlay = m.kind; break;
      case 'error': this.errors.push(m.msg); break;
    }
  }
}
