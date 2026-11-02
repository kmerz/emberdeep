import type { App, Screen } from '../app';
import { Fb, W, H, DIM, PINK } from '../term/fb';
import { LW, LH } from '../state';
import { Fx } from '../render/fx';
import { VW, VH, buildLights, drawEnts, drawMap } from '../render/map';
import { displayEnts } from '../render/interp';
import { Predictor, DIRS } from '../render/predict';
import { UiState, statusBar, partyPanel, pickTarget, targetPanel, minimap, objectives, logPanel, hotbar, commandLine, overlayBox } from '../render/hud';
import { CLS, SKILL_BY_ID } from '../classes';
import type { Ent, GameEvent } from '../proto';

export class GameScreen implements Screen {
  fx = new Fx();
  pred: Predictor;
  mb = new Fb(VW, VH, [7, 8, 12]);
  ui: UiState = { chatActive: false, chatText: '' };
  held = new Set<string>();
  mouse: [number, number] = [-1, -1];
  reviving = 0;
  private lastRevSend = 0;

  constructor(private app: App) {
    this.pred = new Predictor(app.st);
  }

  onEnter() {
    const st = this.app.st;
    this.fx.clear();
    this.pred.reset();
    this.held.clear();
    st.onSnap = () => this.pred.onSnap();
    st.onEvents = (evs: GameEvent[], now: number) => { for (const e of evs) this.fx.add(e, now, st.youId); };
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
    if (ev.key === 'Enter') { this.ui.chatActive = true; this.ui.chatText = ''; this.held.clear(); ev.preventDefault(); return; }
    if (['arrowup', 'arrowdown', 'arrowleft', 'arrowright', 'w', 'a', 's', 'd'].includes(k)) { this.held.add(k); ev.preventDefault(); return; }
    if (ev.repeat) return;
    if (k === '1' || k === '2' || k === '3') { this.cast(parseInt(k) - 1); ev.preventDefault(); }
    else if (k === 'q') { net.send({ t: 'potion', seq: this.pred.nextSeq() }); ev.preventDefault(); }
    else if (k === 'e') { this.startRevive(); ev.preventDefault(); }
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
    const w = this.cellToWorld(cell);
    if (!w) return;
    if (ev.altKey || ev.button === 1) { this.app.net.send({ t: 'ping', cell: w }); ev.preventDefault(); return; }
    if (ev.button === 0 && !this.ui.chatActive) this.cast(0);
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
      else if (cmd === '/help') {
        st.pushLog([['commands: ', DIM], ['/p text', [214, 208, 234]], [' party chat · ', DIM], ['/leave', [214, 208, 234]], [' leave the game · ', DIM], ['/help', [214, 208, 234]]]);
        st.pushLog([['keys: WASD move · 1-3 skills · Q potion · E revive · Alt+click ping', DIM]]);
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
    const meta = CLS[me.cls!].skills[slot];
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
    this.app.net.send({ t: 'cast', slot, x: tx, y: ty, seq: this.pred.nextSeq() });
  }

  private nearestMonster(px: number, py: number): Ent | null {
    const rec = this.app.st.recs[this.app.st.recs.length - 1];
    let best: Ent | null = null, bd = 1e9;
    for (const e of rec?.ents ?? []) {
      if (e.k !== 'm') continue;
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

  // ---- drawing ----
  render(fb: Fb, now: number) {
    const st = this.app.st;
    const t = now / 1000;
    this.pumpMovement(now);
    void this.lastRevSend;
    fb.reset([9, 9, 14]);
    const ents = displayEnts(st, now);
    const you = this.youEnt();
    const yp = you ? (this.pred.ready && !you.downed && !you.dead ? { x: this.pred.x, y: this.pred.y } : { x: you.x, y: you.y }) : null;
    // camera: follow ourselves, or a living party member when we are dead
    let fx0 = yp;
    if (!fx0 || you?.dead) { const o = ents.find((e) => e.k === 'h' && !e.dead); if (o) fx0 = { x: o.x, y: o.y }; }
    if (fx0) {
      this.cam.x = Math.max(0, Math.min(LW - VW, fx0.x - (VW >> 1)));
      this.cam.y = Math.max(0, Math.min(LH - VH, fx0.y - (VH >> 1)));
    }
    this.fx.prune(now);
    const heroes = ents.filter((e) => e.k === 'h' && !e.dead).map((e) => (e.id === st.youId && yp ? yp : { x: e.x, y: e.y }));
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
    hotbar(fb, st, you?.cls ?? null);
    commandLine(fb, this.ui, now);
    if (st.overlay) overlayBox(fb, st.overlay, st, now);
    void H; void SKILL_BY_ID;
  }
}
