import type { App, Screen } from '../app';
import { Fb, RGB, W, H, DIM, PINK, WH, TX } from '../term/fb';
import { LW, LH, BARK_MS } from '../state';
import { Fx, streakLabel } from '../render/fx';
import { sfx } from '../audio';
import { VERSION } from '../version';
import { VW, VH, buildLights, drawEnts, drawMap } from '../render/map';
import { displayEnts } from '../render/interp';
import { Predictor, DIRS } from '../render/predict';
import { inventoryPanel, slotAt, wornAt, INV, WORN_WEAPON, WORN_TRINKET } from '../render/inventory';
import { lorePanel } from '../render/lore';
import { treePanel, PANEL_ORDER } from '../render/tree';
import { shopPanel, sellSlots } from '../render/shop';
import { questBoardPanel, journalPanel, giverQuests, journalQuests } from '../render/quests';
import { itemInfo, packCap, SHOP } from '../items';
import { UiState, statusBar, partyPanel, pickTarget, targetPanel, minimap, objectives, logPanel, hotbar, commandLine, overlayBox, depthBanner } from '../render/hud';
import { CLS, SKILL_BY_ID } from '../classes';
import type { Ent, GameEvent } from '../proto';
import { keyLabel, lastInput, type PadContext } from '../gamepad';

/** The two item keys (4 and 5) and where this browser remembers what is on them. */
export const ITEM_KEYS = ['4', '5'];
const ITEM_STORE = ['emberdeep:itemkey', 'emberdeep:itemkey5'];
function loadItemKeys(): string[] {
  return ITEM_STORE.map((k) => { try { return localStorage.getItem(k) ?? ''; } catch { return ''; } });
}
// lamp oil and torches no longer live in the rucksack: forget a key bound to them in an older version
const RETIRED = new Set(['torch', 'oil']);

/** How close a fight is, for the music: enemies within 10 cells (a boss counts as a fight on its own). */
export function dangerAt(ents: { k: string; g: string; x: number; y: number; dead?: boolean }[], p: { x: number; y: number }): number {
  let d = 0;
  for (const e of ents) {
    if (e.k !== 'm' || e.g === 'B' || Math.hypot(e.x - p.x, (e.y - p.y) * 2) > 20) continue;
    d += 'KQD'.includes(e.g) ? 1 : 0.25;
  }
  return Math.min(1, d);
}

export class GameScreen implements Screen {
  fx = new Fx();
  pred: Predictor;
  mb = new Fb(VW, VH, [7, 8, 12]);
  ui: UiState = { chatActive: false, chatText: '' };
  held = new Set<string>();
  mouse: [number, number] = [-1, -1];
  reviving = 0;
  private lastRevSend = 0;
  private invOpen = false;
  private invSel = 0;
  private treeOpen = false;
  private treeSel = 0;
  private shopOpen = false;
  private shopSide: 0 | 1 | 2 = 0; // buy, sell, quests
  private shopSel = 0;
  private boardOpen = false;
  private loreOpen: { page: number; text: string } | null = null; // an inscription being read
  private boardSel = 0;
  private journalOpen = false;
  private journalSel = 0;
  private levelSeen = 0;
  private yp: { x: number; y: number } | null = null; // where we drew ourselves last frame (for sound falloff)
  private stairsSec = 0;
  /** The rucksack item on key 4 (an item id such as 'firebomb'; '' = none). Remembered in this browser. */
  itemKeys = loadItemKeys().map((id) => (RETIRED.has(id) ? '' : id));

  constructor(private app: App) {
    this.pred = new Predictor(app.st);
  }

  onEnter() {
    const st = this.app.st;
    this.fx.clear();
    this.pred.reset();
    this.held.clear();
    this.invOpen = false;
    this.treeOpen = false;
    this.shopOpen = false;
    this.boardOpen = this.journalOpen = false;
    st.onSnap = () => this.pred.onSnap();
    this.fx.onStreak = (actor, n) => {
      sfx.play('streak', actor === st.youId ? 1 : 0.5);
      if (n >= 3) st.pushLog([[st.nameOf(actor), st.colOf(actor), 1], [' · ', DIM], [streakLabel(n), WH, 1]]);
    };
    st.onEvents = (evs: GameEvent[], now: number) => {
      for (const e of evs) {
        this.fx.add(e, now, st.youId);
        sfx.event(e, st.youId, this.yp);
        if (e.k === 'shop' && e.actor === st.youId) { this.closePanels(); this.shopOpen = true; this.shopSel = 0; }
        if (e.k === 'questboard' && e.actor === st.youId) { this.closePanels(); this.boardOpen = true; this.boardSel = 0; }
        if (e.k === 'lore' && e.actor === st.youId && e.text) { this.closePanels(); this.loreOpen = { page: e.amount ?? 0, text: e.text }; }
      }
    };
  }

  onLeave() { sfx.setTheme(''); }

  /** Gamepad button map to use: panels and the wipe overlay get menu-style buttons; chat gets none. */
  padContext(): PadContext {
    if (this.ui.chatActive) return 'chat';
    if (this.invOpen || this.treeOpen || this.shopOpen || this.boardOpen || this.journalOpen || this.loreOpen) return 'panel';
    if (this.app.st.overlay === 'wipe') return 'overlay';
    return 'world';
  }

  // ---- input ----
  onKey(ev: KeyboardEvent) {
    const st = this.app.st, net = this.app.net;
    if (this.ui.chatActive) {
      if (ev.key === 'Enter') { this.sendChat(); ev.preventDefault(); }
      else if (ev.key === 'Escape') { this.ui.chatActive = false; this.ui.chatText = ''; ev.preventDefault(); }
      else if (ev.key === 'Backspace') { this.ui.chatText = this.ui.chatText.slice(0, -1); ev.preventDefault(); }
      else if (ev.key.length === 1 && !ev.ctrlKey && !ev.metaKey && this.ui.chatText.length < 120) { this.ui.chatText += ev.key; ev.preventDefault(); }
      return;
    }
    if (st.overlay === 'wipe' && ev.key === 'Enter') { if (st.isHost) net.send({ t: 'start' }); ev.preventDefault(); return; }
    const k = ev.key.toLowerCase();
    if (this.loreOpen) { // reading: Esc or Enter closes; walking away closes it too
      this.loreOpen = null;
      if (ev.key === 'Escape' || ev.key === 'Enter' || k === ' ') { ev.preventDefault(); return; }
    }
    if (this.boardOpen && this.questKey(ev, k, 'board')) { ev.preventDefault(); return; }
    if (this.journalOpen && this.journalKey(ev, k)) { ev.preventDefault(); return; }
    if (this.shopOpen && this.shopSide === 2 && ev.key !== 'Tab' && this.questKey(ev, k, 'grimble')) { ev.preventDefault(); return; }
    if (this.shopOpen && this.shopKey(ev, k)) { ev.preventDefault(); return; }
    if (k === 'j' && !ev.repeat) { const o = !this.journalOpen; this.closePanels(); this.journalOpen = o; this.journalSel = 0; ev.preventDefault(); return; }
    if (this.treeOpen && this.treeKey(ev, k)) { ev.preventDefault(); return; }
    if (this.invOpen && this.invKey(ev, k)) { ev.preventDefault(); return; }
    if (k === 'i' && !ev.repeat) { this.invOpen = true; this.treeOpen = false; this.held.clear(); ev.preventDefault(); return; }
    if (k === 't' && !ev.repeat) { this.treeOpen = true; this.invOpen = false; this.held.clear(); ev.preventDefault(); return; }
    if (ev.key === 'Enter') { this.ui.chatActive = true; this.ui.chatText = ''; this.held.clear(); ev.preventDefault(); return; }
    if (['arrowup', 'arrowdown', 'arrowleft', 'arrowright', 'w', 'a', 's', 'd'].includes(k)) { this.held.add(k); ev.preventDefault(); return; }
    if (ev.repeat) return;
    if (k === '1' || k === '2' || k === '3') { this.cast(parseInt(k) - 1); ev.preventDefault(); }
    else if (k === '4' || k === '5') { this.useItemKey(ITEM_KEYS.indexOf(k)); ev.preventDefault(); }
    else if (k === 'q') { net.send({ t: 'potion', seq: this.pred.nextSeq() }); ev.preventDefault(); }
    else if (k === 'e') { this.startRevive(); ev.preventDefault(); }
    else if (k === 'm') {
      const mode = sfx.cycleSound();
      st.pushLog([[mode === 'all' ? 'Music and sound on' : mode === 'sfx' ? 'Music off, sound on' : 'Sound off', TX], ['  (M: music+sound → sound → off)', DIM]]);
      ev.preventDefault();
    }
  }

  onKeyUp(ev: KeyboardEvent) {
    const k = ev.key.toLowerCase();
    this.held.delete(k);
    if (k === 'e' && this.reviving) {
      this.app.net.send({ t: 'revive', target: this.reviving, hold: false, seq: this.pred.nextSeq() });
      this.reviving = 0;
    }
  }

  onMouseMove(_ev: MouseEvent, cell: [number, number]) { this.mouse = cell; }

  onMouseDown(ev: MouseEvent, cell: [number, number]) {
    this.mouse = cell;
    if (this.invOpen && ev.button === 0) {
      const s = slotAt(cell[0], cell[1], packCap(this.app.st.me));
      if (s >= 0) { this.invSel = s; ev.preventDefault(); return; }
      const w = wornAt(cell[0], cell[1]);
      if (w) { this.invSel = w; ev.preventDefault(); return; }
    }
    if (this.invOpen && ev.button === 2) { // right-click a slot: put that item on key 4 (again: key 5)
      const s = slotAt(cell[0], cell[1], packCap(this.app.st.me));
      if (s >= 0) { this.invSel = s; this.bindItem(s); ev.preventDefault(); return; }
    }
    const w = this.cellToWorld(cell);
    if (!w) return;
    if (ev.altKey || ev.button === 1) { this.app.net.send({ t: 'ping', cell: w }); ev.preventDefault(); return; }
    if (ev.button === 0 && !this.ui.chatActive) this.cast(0);
  }

  private closePanels() { this.invOpen = this.treeOpen = this.shopOpen = this.boardOpen = this.journalOpen = false; this.loreOpen = null; }

  /** Keys for a quest list (the notice board, or Grimble's QUESTS tab). */
  private questKey(ev: KeyboardEvent, k: string, giver: 'board' | 'grimble'): boolean {
    const list = giverQuests(this.app.st, giver), n = Math.max(1, list.length);
    const close = () => { if (giver === 'board') this.boardOpen = false; else this.shopOpen = false; };
    if (ev.key === 'Escape') { close(); return true; }
    const sel = giver === 'board' ? this.boardSel : this.shopSel;
    const set = (v: number) => { if (giver === 'board') this.boardSel = v; else this.shopSel = v; };
    if (k === 'arrowup') { set((sel + n - 1) % n); return true; }
    if (k === 'arrowdown') { set((sel + 1) % n); return true; }
    if (ev.repeat) return false;
    if (ev.key === 'Enter' || k === 'u') {
      const q = list[sel];
      if (q && q.state === 'offered') this.app.net.send({ t: 'accept', skill: q.id, seq: this.pred.nextSeq() });
      return true;
    }
    if (k === 'j' && giver === 'board') { this.boardOpen = false; this.journalOpen = true; this.journalSel = 0; return true; }
    return false;
  }

  private journalKey(ev: KeyboardEvent, k: string): boolean {
    const n = Math.max(1, journalQuests(this.app.st).length);
    if (k === 'j' || ev.key === 'Escape') { if (!ev.repeat) this.journalOpen = false; return true; }
    if (k === 'arrowup') { this.journalSel = (this.journalSel + n - 1) % n; return true; }
    if (k === 'arrowdown') { this.journalSel = (this.journalSel + 1) % n; return true; }
    return false;
  }

  /** Keys while Grimble's shop is open. Returns true when the key was used by the panel. */
  private shopKey(ev: KeyboardEvent, k: string): boolean {
    const st = this.app.st, net = this.app.net;
    if (ev.key === 'Escape') { this.shopOpen = false; return true; }
    if (ev.key === 'Tab') { this.shopSide = ((this.shopSide + 1) % 3) as 0 | 1 | 2; this.shopSel = 0; return true; }
    const n = this.shopSide === 0 ? SHOP.length : Math.min(10, sellSlots(st).length);
    if (k === 'arrowup') { this.shopSel = (this.shopSel + n - 1) % Math.max(1, n); return true; }
    if (k === 'arrowdown') { this.shopSel = (this.shopSel + 1) % Math.max(1, n); return true; }
    if (ev.repeat) return false;
    if (ev.key === 'Enter' || k === 'u') {
      if (this.shopSide === 0) {
        const s = SHOP[this.shopSel], me = this.app.st.me;
        if (s?.item === 'oil' && me?.oilMax && (me.oil ?? 0) > me.oilMax - 75) this.app.st.pushLog([['Your lantern is already full', DIM]]);
        else if (s) net.send({ t: 'buy', skill: s.item, seq: this.pred.nextSeq() });
      }
      else { const slot = sellSlots(st)[this.shopSel]; if (slot !== undefined) net.send({ t: 'sell', slot, seq: this.pred.nextSeq() }); }
      return true;
    }
    return false;
  }

  /** Keys while the skill tree is open. Returns true when the key was used by the panel. */
  private treeKey(ev: KeyboardEvent, k: string): boolean {
    const net = this.app.net, me = this.app.st.me;
    if (k === 't' || ev.key === 'Escape') { if (!ev.repeat) this.treeOpen = false; return true; }
    if (k === 'arrowleft' || k === 'arrowright') { this.treeSel = this.treeSel ^ 1; return true; } // the other skill of the tier
    if (k === 'arrowup') { this.treeSel = (this.treeSel + 4) % 6; return true; }
    if (k === 'arrowdown') { this.treeSel = (this.treeSel + 2) % 6; return true; }
    if (ev.repeat) return false;
    const id = me?.tree?.[PANEL_ORDER[this.treeSel]]?.id;
    if (!id) return false;
    if (ev.key === 'Enter' || k === 'u') { net.send({ t: 'learn', skill: id, seq: this.pred.nextSeq() }); return true; }
    if (k === '1' || k === '2' || k === '3') { net.send({ t: 'bind', slot: parseInt(k) - 1, skill: id, seq: this.pred.nextSeq() }); return true; }
    if (k === 'r' && me?.respec) { net.send({ t: 'respec', seq: this.pred.nextSeq() }); return true; }
    return false;
  }

  /** Keys while the rucksack is open. Returns true when the key was used by the panel. */
  private invKey(ev: KeyboardEvent, k: string): boolean {
    const cols = INV.cols, n = packCap(this.app.st.me);
    if (k === 'i' || ev.key === 'Escape') { if (!ev.repeat) this.invOpen = false; return true; }
    if (this.invSel < 0) { // the worn weapon / trinket rows
      if (k === 'arrowup' || k === 'arrowdown') {
        const up = k === 'arrowup';
        if (this.invSel === WORN_WEAPON) this.invSel = up ? n - cols : WORN_TRINKET;
        else this.invSel = up ? WORN_WEAPON : 0;
        return true;
      }
      if (k === 'arrowleft' || k === 'arrowright') return true;
      if (ev.repeat) return false;
      if (k === 'u' || ev.key === 'Enter') { this.app.net.send({ t: 'unequip', slot: this.invSel === WORN_WEAPON ? 0 : 1, seq: this.pred.nextSeq() }); return true; }
      return k === 'x' || k === 'r' || k === '4' || k === '5';
    }
    if (k === 'arrowleft') { this.invSel = (this.invSel + n - 1) % n; return true; }
    if (k === 'arrowright') { this.invSel = (this.invSel + 1) % n; return true; }
    if (k === 'arrowup') { this.invSel = this.invSel < cols ? WORN_TRINKET : this.invSel - cols; return true; }
    if (k === 'arrowdown') { this.invSel = this.invSel + cols >= n ? WORN_WEAPON : this.invSel + cols; return true; }
    if (ev.repeat) return false;
    if (k === 'u' || ev.key === 'Enter') { this.useItem(this.invSel); return true; }
    if (k === 'x') { this.app.net.send({ t: 'drop', slot: this.invSel, seq: this.pred.nextSeq() }); return true; }
    if (k === '4' || k === '5') { this.bindItem(this.invSel, ITEM_KEYS.indexOf(k)); return true; }
    if (k === 'r') { this.bindItem(this.invSel); return true; } // the pad's Y button in panels
    return false;
  }

  /** Puts the item in a rucksack slot on item key `which` (0 = key 4, 1 = key 5). Without `which` (right-click,
   *  the pad's Y) it goes on key 4, or moves to key 5 when it is already on 4. */
  private bindItem(slot: number, which?: number) {
    const st = this.app.st, s = st.me?.pack?.[slot];
    if (!s || s.n <= 0) return;
    const it = itemInfo(s.item);
    if (it.gear) { st.pushLog([['Gear is worn, not put on a key: press ', DIM], [keyLabel('panel', 'Enter', 'U'), WH, 1]]); return; }
    if (!it.usable) { st.pushLog([[it.name, it.col], [' cannot be used', DIM]]); return; }
    const k = which ?? (this.itemKeys[0] === s.item ? 1 : 0);
    this.itemKeys = this.itemKeys.map((id, i) => (i === k ? s.item : id === s.item ? '' : id)); // one key per item
    this.itemKeys.forEach((id, i) => { try { localStorage.setItem(ITEM_STORE[i], id); } catch { /* private window */ } });
    st.pushLog([[it.name, it.col, 1], [' is on key ', DIM], [keyLabel('world', ITEM_KEYS[k]), WH, 1]]);
  }

  /** Key 4 or 5: use (or throw) the bound item from wherever it sits in the rucksack. */
  private useItemKey(which: number) {
    const st = this.app.st, id = this.itemKeys[which] ?? '', key = ITEM_KEYS[which];
    if (!id) {
      st.pushLog(lastInput.pad
        ? [['That item key is empty: open the rucksack (', DIM], ['SEL', WH, 1], ['), pick an item and press ', DIM], ['Y', WH, 1]]
        : [['Key ' + key + ' is empty: open the rucksack (', DIM], ['I', WH, 1], ['), pick an item and press ', DIM], [key, WH, 1], [' or right-click it', DIM]]);
      return;
    }
    const slot = (st.me?.pack ?? []).findIndex((s) => s.item === id && s.n > 0);
    if (slot < 0) { const it = itemInfo(id); st.pushLog([['No ', DIM], [it.name, it.col], [' left in your rucksack', DIM]]); return; }
    this.useItem(slot);
  }

  private useItem(slot: number) {
    const st = this.app.st, s = st.me?.pack?.[slot];
    const me = this.youEnt();
    if (!s || s.n <= 0 || !me) return;
    const it = itemInfo(s.item);
    if (!it.usable) { st.pushLog([[it.name, it.col], [' cannot be used', DIM]]); return; }
    let tx = 0, ty = 0;
    if (it.thrown) { // aim like a targeted skill: the mouse cursor, else the nearest monster, else ahead
      const px = this.pred.ready ? this.pred.x : me.x, py = this.pred.ready ? this.pred.y : me.y;
      const w = this.cellToWorld(this.mouse);
      const near = w ? null : this.nearestMonster(px, py);
      if (w) [tx, ty] = w;
      else if (near) { tx = near.x; ty = near.y; }
      else { const d = DIRS[this.pred.face]; tx = px + d[0] * 4; ty = py + d[1] * 4; }
    }
    this.app.net.send({ t: 'use', slot, x: tx, y: ty, seq: this.pred.nextSeq() });
  }

  private sendChat() {
    const st = this.app.st;
    let text = this.ui.chatText.trim();
    this.ui.chatActive = false; this.ui.chatText = '';
    if (!text) return;
    if (text.startsWith('/')) {
      const [cmd, ...rest] = text.split(' ');
      if (cmd === '/p') text = rest.join(' ').trim();
      else if (cmd === '/leave' || cmd === '/quit') { this.app.setScreen('landing'); return; }
      else if (cmd === '/save') {
        if (st.isHost) this.app.net.send({ t: 'save' });
        else st.pushLog([['Only the host can save. The game also saves by itself on every new floor.', DIM]]);
        return;
      }
      else if (cmd === '/help') {
        st.pushLog([['commands: ', DIM], ['/p text', [214, 208, 234]], [' party chat · ', DIM], ['/save', [214, 208, 234]], [' save + back to the lobby (host) · ', DIM], ['/leave', [214, 208, 234]], [' leave the game · ', DIM], ['/help', [214, 208, 234]], [' · emberdeep v' + VERSION, DIM]]);
        st.pushLog([['keys: WASD move · 1-3 skills · 4 5 items · Q potion · hold E revive · Alt+click ping · M music / sound / off', DIM]]);
        st.pushLog([['in the skill tree (T) press 1, 2 or 3 on a skill to put it on that key; in the rucksack (I) press 4 or 5, or right-click an item', DIM]]);
        st.pushLog([['gamepad: stick move · X/Y/RB or RT skills · B A items · LB potion · hold LT revive · Select pack · Start journal · R3 skills · in panels A ok, B back, Y item key', DIM]]);
        return;
      } else { st.pushLog([['unknown command ' + cmd + ' (try /help)', PINK]]); return; }
      if (!text) return;
    }
    this.app.net.send({ t: 'chat', text });
  }

  private cellToWorld(cell: [number, number]): [number, number] | null {
    const [cx, cy] = cell;
    if (cx < 0 || cx >= VW || cy < 1 || cy > VH) return null;
    return [cx + this.cam.x, cy - 1 + this.cam.y];
  }

  private cam = { x: 0, y: 0 };
  private youEnt(): Ent | undefined {
    const rec = this.app.st.recs[this.app.st.recs.length - 1];
    return rec?.ents.find((e) => e.k === 'h' && e.id === this.app.st.youId);
  }

  private cast(slot: number) {
    const st = this.app.st;
    const me = this.youEnt();
    if (!me || !st.party.length) return;
    const meta = SKILL_BY_ID[st.me?.binds?.[slot] ?? '']?.meta;
    if (!meta) { st.pushLog([['Key ' + (slot + 1) + ' is empty: open the skill tree with ', DIM], ['T', WH, 1]]); return; }
    const px = this.pred.ready ? this.pred.x : me.x, py = this.pred.ready ? this.pred.y : me.y;
    let tx: number, ty: number;
    const w = this.cellToWorld(this.mouse);
    if (w) { [tx, ty] = w; }
    else {
      const near = this.nearestMonster(px, py);
      if (near) { tx = near.x; ty = near.y; }
      else { const d = DIRS[this.pred.face]; tx = px + d[0] * 4; ty = py + d[1] * 4; }
    }
    if (!meta.targeted && w === null) { const d = DIRS[this.pred.face]; tx = px + d[0]; ty = py + d[1]; }
    if (!meta.targeted) {
      const rec = st.recs[st.recs.length - 1];
      const adj = (rec?.ents ?? []).filter((e) => e.k === 'm' && e.g !== 'B' && Math.max(Math.abs(e.x - px), Math.abs(e.y - py)) === 1);
      if (adj.length) {
        const aimed = w !== null && adj.some((e) => e.x === w[0] && e.y === w[1]);
        if (!aimed) {
          let pick = adj.find((e) => e.id === st.lastTarget);
          if (!pick) {
            const ax = tx - px, ay = ty - py;
            const al = Math.hypot(ax, ay) || 1;
            let bs = -2;
            for (const e of adj) {
              const ex = e.x - px, ey = e.y - py;
              const sc = (ex * ax + ey * ay) / (Math.hypot(ex, ey) * al);
              if (sc > bs) { bs = sc; pick = e; }
            }
          }
          if (pick) { tx = pick.x; ty = pick.y; }
        }
      }
    }
    this.app.net.send({ t: 'cast', slot, x: tx, y: ty, seq: this.pred.nextSeq() });
  }

  private nearestMonster(px: number, py: number): Ent | null {
    const rec = this.app.st.recs[this.app.st.recs.length - 1];
    let best: Ent | null = null, bd = 1e9;
    for (const e of rec?.ents ?? []) {
      if (e.k !== 'm' || e.g === 'B') continue; // never aim at a powder keg by accident
      const d = Math.hypot(e.x - px, (e.y - py) * 2);
      if (d < bd) { best = e; bd = d; }
    }
    return best;
  }

  private startRevive() {
    const rec = this.app.st.recs[this.app.st.recs.length - 1];
    if (!rec) return;
    const px = this.pred.x, py = this.pred.y;
    const t = rec.ents.find((e) => e.k === 'h' && e.id !== this.app.st.youId && e.downed && Math.max(Math.abs(e.x - px), Math.abs(e.y - py)) <= 1);
    if (t) { this.reviving = t.id; this.app.net.send({ t: 'revive', target: t.id, hold: true, seq: this.pred.nextSeq() }); this.lastRevSend = performance.now(); }
  }

  private pumpMovement(now: number) {
    if (this.ui.chatActive || !this.pred.ready || this.held.size === 0) return;
    const h = this.held;
    let dx = 0, dy = 0;
    if (h.has('arrowleft') || h.has('a')) dx -= 1;
    if (h.has('arrowright') || h.has('d')) dx += 1;
    if (h.has('arrowup') || h.has('w')) dy -= 1;
    if (h.has('arrowdown') || h.has('s')) dy += 1;
    if (!dx && !dy) return;
    const dir = DIRS.findIndex((d) => d[0] === dx && d[1] === dy);
    if (dir < 0) return;
    if (this.reviving) { this.reviving = 0; }
    const m = this.pred.move(dir, now);
    if (m) this.app.net.send(m);
  }

  /** Speech bubbles over whoever is talking; they follow the speaker and fade out. */
  private drawBarks(mb: Fb, ents: Ent[], now: number) {
    const st = this.app.st;
    for (const [id, b] of st.barks) {
      const age = now - b.at;
      if (age > BARK_MS) { st.barks.delete(id); continue; }
      const e = ents.find((n) => n.id === id);
      const wx = e ? e.x : b.x, wy = e ? e.y : b.y;
      if (!st.isVisible(wx, wy)) continue;
      const s = ' ' + b.text + ' ';
      const fd = age > BARK_MS - 500 ? (BARK_MS - age) / 500 : 1;
      const boss = !!b.kind && 'KQD'.includes(b.kind) || !!e?.el;
      const fg: RGB = b.kind === 'g' ? [170, 240, 150] : boss ? [255, 150, 180] : [236, 230, 250];
      const bg: RGB = b.kind === 'g' ? [18, 40, 22] : boss ? [50, 10, 26] : [30, 28, 44];
      const x = Math.max(0, Math.min(VW - s.length, wx - this.cam.x - Math.floor(s.length / 2)));
      const y = Math.max(0, wy - this.cam.y - 2);
      mb.txt(x, y, s, fg.map((v) => v * fd) as RGB, bg.map((v) => v * fd) as RGB, boss ? 1 : 0);
    }
  }

  // ---- drawing ----
  render(fb: Fb, now: number) {
    const st = this.app.st;
    const t = now / 1000;
    this.pumpMovement(now);
    void this.lastRevSend;
    fb.reset([9, 9, 14]);
    if (st.levelAt !== this.levelSeen) { // new floor: drop old effects and unacked moves
      this.levelSeen = st.levelAt; this.fx.clear(); this.pred.reset();
    }
    const ents = displayEnts(st, now);
    const you = this.youEnt();
    const yp = you ? (this.pred.ready && !you.downed && !you.dead ? { x: this.pred.x, y: this.pred.y } : { x: you.x, y: you.y }) : null;
    this.yp = yp;
    // sound: the floor's music, ambient noises, a heartbeat at low hp, and a tick per second of the stairs countdown
    const lv = st.level;
    sfx.setTheme(lv?.theme ?? '', lv?.village ? 0 : lv?.depth ?? 0, lv?.seed ?? '');
    sfx.setDanger(yp ? dangerAt(ents, yp) : 0);
    const pe = st.party.find((p) => p.id === st.youId);
    sfx.update(now, !!pe && !pe.downed && !pe.dead && pe.hp > 0 && pe.hp < pe.hpMax * 0.25);
    const sec = Math.ceil((st.obj?.stairsLeft ?? 0) / 20);
    if (sec !== this.stairsSec) { if (sec > 0) sfx.play('tick'); this.stairsSec = sec; }
    // camera: follow ourselves, or a living party member when we are dead
    let fx0 = yp;
    if (!fx0 || you?.dead) { const o = ents.find((e) => e.k === 'h' && !e.dead); if (o) fx0 = { x: o.x, y: o.y }; }
    if (fx0) {
      this.cam.x = Math.max(0, Math.min(LW - VW, fx0.x - (VW >> 1)));
      this.cam.y = Math.max(0, Math.min(LH - VH, fx0.y - (VH >> 1)));
    }
    this.fx.prune(now);
    const heroes = ents.filter((e) => e.k === 'h' && !e.dead).map((e) => {
      const torch = !!e.st?.some((s) => s.k === 'torch');
      return e.id === st.youId && yp ? { ...yp, torch } : { x: e.x, y: e.y, torch };
    });
    const lights = buildLights(st, this.cam, t, heroes, this.fx.lights(now, st, heroes));
    const mb = this.mb;
    mb.reset([7, 8, 12]);
    drawMap(mb, st, this.cam, t, lights, this.fx.heat(now));
    // caltrops patches
    for (const p of st.latest?.patches ?? []) {
      for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) {
        const x = p[0] + dx - this.cam.x, y = p[1] + dy - this.cam.y;
        if (x >= 0 && y >= 0 && x < VW && y < VH && st.isVisible(p[0] + dx, p[1] + dy) && st.tileAt(p[0] + dx, p[1] + dy) !== 35)
          mb.put(x, y, '∴', [199, 146, 255], null, 1);
      }
    }
    // spike traps the party has seen: dull when armed, bright for a moment after they spring
    for (const tr of st.latest?.traps ?? []) {
      const x = tr[0] - this.cam.x, y = tr[1] - this.cam.y;
      if (x >= 0 && y >= 0 && x < VW && y < VH && st.isVisible(tr[0], tr[1]))
        mb.put(x, y, tr[2] ? '×' : '‡', tr[2] ? [170, 80, 76] : [255, 120, 110], tr[2] ? null : [60, 12, 12], 1);
    }
    this.fx.drawUnder(mb, this.cam, now, st);
    drawEnts(mb, st, this.cam, ents, yp, { youId: st.youId, now, flashes: this.fx.flashes, hitCol: [255, 200, 160] });
    // pings
    st.pings = st.pings.filter((p) => now - p.t0 < 4000);
    for (const p of st.pings) {
      const c = CLS[p.cls].col, age = (now - p.t0) / 1000;
      const pulse = 0.55 + 0.45 * Math.sin(age * 9);
      mb.put(p.x - this.cam.x, p.y - this.cam.y, '◇', c.map((v) => v * pulse), null, 1);
    }
    this.fx.drawOver(mb, this.cam, now, st);
    this.drawBarks(mb, ents, now);
    // compose with camera shake
    const [ox, oy] = yp ? this.fx.shake(now, yp.x, yp.y) : [0, 0];
    for (let y = 0; y < VH; y++) for (let x = 0; x < VW; x++) {
      const sx = x + ox, sy = y + oy, di = (y + 1) * W + x;
      if (sx < 0 || sy < 0 || sx >= VW || sy >= VH) { fb.ch[di] = ' '; fb.setBg(di, [7, 8, 12]); continue; }
      const si = sy * VW + sx;
      fb.ch[di] = mb.ch[si];
      fb.setFg(di, mb.getFg(si)); fb.setBg(di, mb.getBg(si)); fb.bold[di] = mb.bold[si];
    }
    statusBar(fb, st);
    partyPanel(fb, st);
    targetPanel(fb, st, pickTarget(st, ents, yp));
    minimap(fb, st, this.cam, ents.map((e) => (e.id === st.youId && yp ? { ...e, x: yp.x, y: yp.y } : e)), now);
    objectives(fb, st);
    logPanel(fb, st, now);
    hotbar(fb, st, you?.cls ?? null, this.itemKeys);
    commandLine(fb, this.ui, now);
    if (this.invOpen) inventoryPanel(fb, st, this.invSel, this.itemKeys);
    if (this.treeOpen) treePanel(fb, st, this.treeSel, you?.cls ?? null);
    if (this.shopOpen) {
      // walking away from Grimble closes the shop
      const near = you && ents.some((e) => e.k === 'n' && e.trader && Math.max(Math.abs(e.x - (yp?.x ?? you.x)), Math.abs(e.y - (yp?.y ?? you.y))) <= 1);
      if (!near) this.shopOpen = false;
      else if (this.shopSide === 2) questBoardPanel(fb, st, 'grimble', this.shopSel);
      else shopPanel(fb, st, this.shopSide, this.shopSel);
    }
    if (this.boardOpen) {
      const near = you && ents.some((e) => e.k === 'n' && e.board && Math.max(Math.abs(e.x - (yp?.x ?? you.x)), Math.abs(e.y - (yp?.y ?? you.y))) <= 1);
      if (!near) this.boardOpen = false;
      else questBoardPanel(fb, st, 'board', this.boardSel);
    }
    if (this.journalOpen) journalPanel(fb, st, this.journalSel);
    if (this.loreOpen) lorePanel(fb, this.loreOpen.page, this.loreOpen.text);
    depthBanner(fb, st, now);
    if (st.overlay) overlayBox(fb, st.overlay, st, now);
    void H; void SKILL_BY_ID;
  }
}
