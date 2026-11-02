import type { Ent, GameEvent, LevelMsg, LobbyMsg, Me, Objectives, PartyEntry, ServerMsg, Snap, ClassId, QuestInfo } from './proto';
import { CLS, MC, SKILL_BY_ID } from './classes';
import { DIM, GOLD, PINK, RGB, TX } from './term/fb';
import { palette } from './render/themes';
import { VALUE, itemInfo } from './items';
import { Remain, bleedFor, remainsFor, remainsOk } from './render/remains';

const MINT_LOG: RGB = [126, 240, 178];
const WH_LOG: RGB = [236, 232, 248];
const CRIT_LOG: RGB = [255, 200, 60];
const HURT_LOG: RGB = [255, 110, 110];

/** Characters of text a log line has room for (the log panel in hud.ts). */
export const LOG_W = 54;

/** Splits colored segments into lines of at most width characters, breaking at spaces where it can. */
export function wrapSegs(segs: LogSeg[], width: number): LogSeg[][] {
  const lines: LogSeg[][] = [[]];
  let used = 0;
  for (const sg of segs) {
    let text = sg[0];
    while (used + text.length > width) {
      const room = width - used;
      let cut = text.lastIndexOf(' ', room);
      if (cut <= 0) cut = used === 0 ? room : 0; // a word longer than a line: hard cut; else move it to the next line
      if (cut > 0) lines[lines.length - 1].push([text.slice(0, cut), sg[1], sg[2]]);
      text = text.slice(cut).replace(/^ +/, '');
      lines.push([]);
      used = 0;
    }
    if (text) { lines[lines.length - 1].push([text, sg[1], sg[2]]); used += text.length; }
  }
  return lines.filter((l) => l.length);
}

export const LW = 176;
export const LH = 62;

export type LogSeg = [string, RGB, number?];
export interface LogLine { ts: string; segs: LogSeg[]; at: number; key?: string; n?: number; dmg?: number; crits?: number }
/** Hits on the same target within this window update one log line instead of adding new ones. */
export const HIT_MERGE_MS = 3000;
export interface PingMark { x: number; y: number; cls: ClassId; name: string; t0: number }

export interface SnapRec { t: number; snap: Snap; ents: Ent[] }

export const INTERP_MS = 100;
export const BARK_MS = 3200; // how long a speech bubble stays up

export interface Bark { text: string; at: number; x: number; y: number; kind: string }

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
  quests: QuestInfo[] = []; // the party's sidequests
  lobby: LobbyMsg | null = null;
  phase: 'lobby' | 'game' = 'lobby';
  pingMs = 0;
  lastTarget = 0; // monster id last attacked by us
  errors: string[] = [];
  remains = new Map<number, Remain>(); // corpses and blood on this floor, by tile index (client-only)
  barks = new Map<number, Bark>();     // speech bubbles, by the speaker's entity id
  // hooks wired by the screens
  onEvents: (evs: GameEvent[], now: number) => void = () => {};
  onPing: (p: PingMark) => void = () => {};
  onSnap: (s: Snap) => void = () => {};

  reset() {
    this.level = null;
    this.quests = [];
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
    this.remains.clear();
    this.barks.clear();
  }

  /** Leaves corpses where monsters die and splatters on heavy hits. */
  markRemains(evs: GameEvent[]) {
    const put = (x: number, y: number, r: Remain, over: boolean) => {
      if (x < 0 || y < 0 || x >= LW || y >= LH || !remainsOk(this.tileAt(x, y))) return;
      const i = y * LW + x;
      if (!over && this.remains.has(i)) return;
      this.remains.delete(i); // re-insert so the newest stays when we trim
      this.remains.set(i, r);
      if (this.remains.size > 600) this.remains.delete(this.remains.keys().next().value!);
    };
    for (const e of evs) {
      if (e.k === 'kill') {
        const r = remainsFor(e.fx ?? '');
        if (r) put(e.x, e.y, r, true);
      } else if (e.k === 'urn') {
        put(e.x, e.y, ["'", [150, 100, 60]], true); // shards
      } else if (e.k === 'impact' && e.fx === 'keg') { // a scorch mark
        put(e.x, e.y, ['░', [70, 44, 30]], true);
        for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) put(e.x + dx, e.y + dy, [':', [80, 50, 34]], false);
      } else if (e.k === 'hit' && e.target) {
        const heavy = e.fx === 'crit' || (e.amount ?? 0) >= 18;
        if (!heavy) continue;
        let kind = '';
        if (e.target >= 10000) kind = this.recs[this.recs.length - 1]?.ents.find((n) => n.id === e.target)?.g ?? '?';
        const col = bleedFor(kind);
        if (!col) continue;
        put(e.x + Math.floor(Math.random() * 3) - 1, e.y + Math.floor(Math.random() * 3) - 1, [Math.random() < 0.5 ? ',' : '.', col], false);
      }
    }
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

  /** A short-lived snapshot entity by id (for names and glyphs of monsters in events). */
  entById(id: number): Ent | undefined {
    // a monster killed this tick is already gone from the newest snapshot: look one back too
    for (let i = this.recs.length - 1; i >= Math.max(0, this.recs.length - 2); i--) {
      const e = this.recs[i].ents.find((n) => n.id === id);
      if (e) return e;
    }
    return undefined;
  }

  /** Logs a hit, folding repeats (same attacker and target, within HIT_MERGE_MS) into one line with a running total. */
  logHit(key: string, head: LogSeg[], dmg: number, crit: boolean, now: number) {
    let ln = this.log.slice(-4).find((l) => l.key === key && now - l.at < HIT_MERGE_MS);
    if (!ln) {
      ln = { ts: clock(this.tick), segs: [], at: now, key, n: 0, dmg: 0, crits: 0 };
      this.log.push(ln);
      if (this.log.length > 300) this.log.splice(0, this.log.length - 200);
    }
    ln.n! += 1; ln.dmg! += dmg; if (crit) ln.crits! += 1;
    ln.at = now; ln.ts = clock(this.tick);
    const tail: LogSeg[] = ln.n === 1 ? [[' for ', DIM], [String(dmg), WH_LOG, 1]]
      : [[' ×' + ln.n, TX], [' · ', DIM], [ln.dmg + ' dmg', WH_LOG, 1]];
    if (ln.crits) tail.push([ln.crits > 1 ? ' · ' + ln.crits + ' crits' : ' · crit', CRIT_LOG, 1]);
    ln.segs = [...head, ...tail];
  }

  pushLog(segs: LogSeg[], now = performance.now()) {
    const ts = clock(this.tick);
    for (const line of wrapSegs(segs, LOG_W)) this.log.push({ ts, segs: line, at: now });
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
        case 'hit': {
          const dmg = e.amount ?? 0;
          if (dmg <= 0 || !e.target) break;
          const crit = e.fx === 'crit';
          if (you && e.target >= 10000) {
            // your hits on a monster (not on a powder keg, which has its own line)
            const m = this.entById(e.target);
            if (!m || m.g === 'B') break;
            this.logHit('you>' + e.target, [['You hit ', DIM], [m.name || 'something', MC[m.g] ?? TX]], dmg, crit, now);
          } else if (e.target === this.youId) {
            // what hits you: a monster, an ally, a trap or a fire
            // no attacker: a trap, fire or a keg blast
            if (!e.actor) { this.logHit('0>you', [['You are hurt', HURT_LOG, 1]], dmg, crit, now); break; }
            const src = e.actor >= 10000 ? this.entById(e.actor) : undefined;
            const who: LogSeg = e.actor < 10000 ? [this.nameOf(e.actor) || 'An ally', this.colOf(e.actor)]
              : [src?.name || 'Something', src ? MC[src.g] ?? TX : TX];
            this.logHit(e.actor + '>you', [who, [' hits ', DIM], ['you', HURT_LOG, 1]], dmg, crit, now);
          }
          break;
        }
        case 'kill':
          if (e.fx === 'K') this.pushLog([['The Tallow King', PINK, 1], [' falls to ', DIM], [an || 'the party', ac]], now);
          else if (e.fx === 'M') this.pushLog([['The mimic', PINK, 1], [' falls and spills its treasure', DIM]], now);
          else if (you && e.fx !== 'B') this.pushLog([['You slay the ', DIM], [e.text || 'monster', MC[e.fx ?? ''] ?? TX, 1]], now);
          break;
        case 'pickup':
          if (e.fx === 'gold') this.pushLog([[you ? 'You' : an, you ? TX : ac], [' pick' + (you ? '' : 's') + ' up ', DIM], [(e.amount || 0) + ' gold', GOLD, 1]], now);
          else if (you && e.fx === 'oil') this.pushLog([['You fill your lantern with ', DIM], ['lamp oil', [255, 170, 70], 1]], now);
          else if (you && e.fx) {
            const it = itemInfo(e.fx), v = VALUE[e.fx];
            this.pushLog([['You pick up ', DIM], [(e.amount || 1) > 1 ? e.amount + '× ' + it.name : it.name, it.col, 1], ...(v ? [[' · Grimble pays ' + v + ' gold', GOLD] as LogSeg] : [])], now);
          }
          break;
        case 'packfull': if (you) this.pushLog([['Your rucksack is full', PINK]], now); break;
        case 'chest': this.pushLog([[you ? 'You' : an, you ? TX : ac], [you ? ' open a ' : ' opens a ', DIM], ['chest', GOLD, 1]], now); break;
        case 'summon':
          this.pushLog(e.fx === 'Q' ? [['The Ant Queen', [255, 170, 60]], [' lays a ', DIM], [e.text || 'soldier', TX]]
            : [['A Bone Caller', [180, 120, 255]], [' raises a ', DIM], [e.text || 'minion', TX]], now);
          break;
        case 'breath': this.pushLog([['The Ember Dragon', [255, 60, 30], 1], [' draws a deep breath…', DIM]], now); break;
        case 'charm': this.pushLog([[this.nameOf(e.target || 0), this.colOf(e.target || 0)], [' is charmed by the ', DIM], ['Succubus', [255, 110, 200], 1]], now); break;
        case 'drain': if (e.target === this.youId) this.pushLog([['The Incubus', [180, 90, 255]], [' drains ' + (e.amount ?? 0) + ' of your power', DIM]], now); break;
        case 'questnew': this.pushLog([['New quest: ', GOLD, 1], [e.text || '', TX], ['  (J: journal)', DIM]], now); break;
        case 'questprogress': this.pushLog([['Quest: ', GOLD], [e.text || '', TX], [' · ' + (e.amount ?? 0) + ' found', DIM]], now); break;
        case 'questdone':
          this.pushLog([['Quest done: ', MINT_LOG, 1], [e.text || '', TX], [' · +' + (e.amount ?? 0) + ' gold, +' + (e.tx ?? 0) + ' XP' + (e.ty ? ', +1 skill point' : ''), GOLD]], now);
          break;
        case 'questfail': this.pushLog([['Quest failed: ', PINK, 1], [e.text || '', TX]], now); break;
        case 'alarm': this.pushLog([['The hive is alarmed!', [255, 170, 60], 1], [' ' + (e.amount ?? 0) + ' ants turn on you', DIM]], now); break;
        case 'say': if (you && e.text) this.pushLog([[e.text, [140, 220, 120]]], now); break;
        case 'bark': {
          if (!e.actor || !e.text) break;
          this.barks.set(e.actor, { text: e.text, at: now, x: e.x, y: e.y, kind: e.fx ?? '' });
          // bosses and hunt targets also speak into the log
          const ent = this.recs[this.recs.length - 1]?.ents.find((n) => n.id === e.actor);
          if ((!!e.fx && 'KQD'.includes(e.fx)) || ent?.el) this.pushLog([[(ent?.name ?? 'Someone') + ': ', PINK, 1], [e.text, TX]], now);
          break;
        }
        case 'oillow': if (you) this.pushLog([['Your lantern is running low', PINK, 1], [': pick up lamp oil, or buy some from Grimble', DIM]], now); break;
        case 'trapseen': if (you) this.pushLog([['You spot a ', DIM], ['spike trap', [230, 70, 70], 1], [': step around it', DIM]], now); break;
        case 'fuse': if (you) this.pushLog([['The powder keg', [230, 120, 50], 1], [' starts to fizz…', DIM]], now); break;
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
          if (e.fx === 'lightorb') this.pushLog([who, [you ? ' read a ' : ' reads a ', DIM], [it.name, it.col]], now);
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
          if (m.dark) this.pushLog([['It is pitch black. Your lantern burns oil while you are here: ', DIM], ['keep an eye on it', [255, 170, 70]], ['.', DIM]], now);
        }
        this.level = m; this.tiles.fill(0); this.vis.fill(0);
        this.recs = []; this.latest = null; this.pings = []; this.overlay = null; this.lastTarget = 0;
        this.remains.clear();
        this.barks.clear();
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
        this.markRemains(m.list);
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
      case 'quests': this.quests = m.list; break;
      case 'error': this.errors.push(m.msg); break;
    }
  }
}
