// One effect pipeline: every effect is {kind, x, y, t0, actor, palette...} and contributes glyphs, lights and
// optional camera shake. drawBlast is a port of design/Main.dc.html (lines 135-170).
import type { GameEvent } from '../proto';
import { CLS } from '../classes';
import { Fb, RGB, add, c01, dist, hash, hx, mix, mul, ramp, DIM, PINK } from '../term/fb';
import { GameState, INTERP_MS } from '../state';

export interface Pal { title: string; ramp: RGB[]; shock: RGB }
const mk = (title: string, r: string[], shock: string): Pal => ({ title, ramp: r.map(hx), shock: hx(shock) });
export const PALS: Record<string, Pal> = {
  ember: mk('Ember Nova', ['#fffbe6', '#ffe14d', '#ff8a1f', '#ff2e88', '#7b2cff', '#1d0b3f'], '#5ff5ff'),
  void: mk('Void Rift', ['#ffffff', '#b9fff4', '#2de2c4', '#8a3dff', '#3a0f7a', '#12062c'], '#ff4fd8'),
  frost: mk('Glacial Burst', ['#ffffff', '#d6f5ff', '#6fd0ff', '#3b6bff', '#2a2290', '#0b0b2e'], '#ffe9a8'),
};

export interface Light { x: number; y: number; r: number; c: RGB; i: number }
export interface Cam { x: number; y: number }
export type IsOpen = (vx: number, vy: number) => boolean;

export function drawBlast(mb: Fb, cx: number, cy: number, e: number, P: Pal, isOpen: IsOpen, seed: number, scale: number) {
  if (e < 0) return;
  const fr = Math.floor(e * 24);
  const R = (3 + 15 * (1 - Math.pow(1 - c01(e / 0.5), 3))) * scale;
  const fade = e < 0.45 ? 1 : c01(1 - (e - 0.45) / 0.75);
  const rs = (2 + 50 * e) * scale, shockA = c01(1 - e / 0.75), flash = e < 0.1 ? 1 - e / 0.1 : 0;
  const hole = e > 0.3 ? c01((e - 0.3) * 1.6) : 0;
  const shockCol = mix(P.shock, [255, 255, 255], 0.25);
  const w = mb.w, h = mb.h;
  if (flash > 0) {
    for (let i = 0; i < w * h; i++) {
      mb.setBg(i, add(mb.getBg(i), mul([120, 90, 130], flash)));
      mb.setFg(i, add(mb.getFg(i), mul([90, 80, 90], flash)));
    }
  }
  const reach = Math.max(R + 4, shockA > 0 ? rs + 2 : 0);
  const x0 = Math.max(0, Math.floor(cx - reach)), x1 = Math.min(w - 1, Math.ceil(cx + reach));
  const y0 = Math.max(0, Math.floor(cy - reach / 2 - 1)), y1 = Math.min(h - 1, Math.ceil(cy + reach / 2 + 1));
  for (let y = y0; y <= y1; y++) for (let x = x0; x <= x1; x++) {
    if (!isOpen(x, y)) continue;
    const i = y * w + x;
    const d0 = dist(x, y, cx, cy), d = d0 + (hash(x, y, fr + seed) - 0.5) * 2.2 * scale;
    let drawn = false;
    if (fade > 0 && d < R) {
      const q = d / R;
      let dens = (1 - q) * 1.5 * fade + 0.15, ci = q * 0.75 + (1 - fade) * 0.7;
      if (q < hole) { dens *= 0.35; ci += 0.35; }
      const ch = dens > 0.95 ? '█' : dens > 0.7 ? '▓' : dens > 0.48 ? '▒' : dens > 0.28 ? '░' : '·';
      mb.put(x, y, ch, ramp(P.ramp, ci), mul(ramp(P.ramp, Math.min(1, ci + 0.3)), 0.5 * fade + 0.1), 0);
      drawn = dens > 0.48;
    }
    if (shockA > 0 && !drawn && Math.abs(d0 - rs) < 1.15 * Math.max(scale, 0.8)) {
      mb.put(x, y, tangent(x, y, cx, cy), mix(mb.getBg(i), shockCol, shockA), null, 1);
    }
  }
  for (let i = 0; i < 46; i++) {
    const ang = hash(i, 1, seed + 7) * Math.PI * 2, spd = (12 + hash(i, 2, seed + 7) * 36) * scale, life = 0.55 + hash(i, 3, seed + 7) * 1.3;
    if (e > life) continue;
    const dd = (spd * (1 - Math.exp(-e * 3))) / 3;
    const px = Math.round(cx + Math.cos(ang) * dd), py = Math.round(cy + (Math.sin(ang) * dd) / 2);
    if (px < 0 || py < 0 || px >= w || py >= h || !isOpen(px, py)) continue;
    const a = e / life;
    mb.put(px, py, a < 0.25 ? '*' : a < 0.5 ? '+' : a < 0.78 ? '•' : '·', ramp(P.ramp, 0.12 + a * 0.75), null, 1);
  }
}

/** Glyph tangent to a ring around (cx,cy): ─ ╲ │ ╱ */
function tangent(x: number, y: number, cx: number, cy: number): string {
  const deg = ((Math.atan2((y - cy) * 2, x - cx) * 180) / Math.PI + 90 + 360) % 180;
  return deg < 22.5 || deg >= 157.5 ? '─' : deg < 67.5 ? '╲' : deg < 112.5 ? '│' : '╱';
}

interface Effect {
  kind: string; x: number; y: number; tx: number; ty: number; t0: number; dur: number;
  actor: number; r: number; pal: string; amount: number; seed: number; crit?: boolean; text?: string; hero?: boolean;
}

const GREYGOLD: RGB = [200, 180, 120];
const WARM: RGB = [1, 0.85, 0.5];
let seedCtr = 1;
const nr = (c: RGB): RGB => [c[0] / 255, c[1] / 255, c[2] / 255];

export class Fx {
  list: Effect[] = [];
  flashes = new Map<number, number>(); // entity id -> time of last hit

  clear() { this.list = []; }

  add(ev: GameEvent, now: number, me: number) {
    const t0 = now + (ev.actor === me ? 0 : INTERP_MS);
    const base = { x: ev.x, y: ev.y, tx: ev.tx ?? ev.x, ty: ev.ty ?? ev.y, t0, actor: ev.actor ?? 0, r: ev.r ?? 0, pal: 'ember', amount: ev.amount ?? 0, seed: seedCtr++ };
    const push = (kind: string, dur: number, extra: Partial<Effect> = {}) => this.list.push({ ...base, kind, dur, ...extra });
    switch (ev.k) {
      case 'cast':
        switch (ev.fx) {
          case 'nova': push('charge', Math.max(300, (ev.amount ?? 9) * 50), { pal: 'ember' }); break;
          case 'shadowbomb': push('charge', Math.max(300, (ev.amount ?? 8) * 50), { pal: 'void' }); break;
          case 'spark': push('bolt', 170); break;
          case 'blink': push('blink', 420); break;
          case 'leap': push('trail', 260); break;
          case 'dash': push('trail', 240); break;
          case 'cleave': case 'stab': case 'palm': push('slash', 180, { text: ev.fx }); break;
          case 'ironwall': push('flare', 600, { pal: 'wall' }); break;
          case 'caltrops': push('flare', 400, { pal: 'caltrops' }); break;
          case 'bell': push('bell', 1900); break;
        }
        break;
      case 'impact':
        if (ev.fx === 'nova') push('blast', 2000, { pal: 'ember', r: ev.r ?? 9 });
        else if (ev.fx === 'shadowbomb') push('blast', 2000, { pal: 'void', r: ev.r ?? 5 });
        else if (ev.fx === 'leap') push('leap', 1100);
        break;
      case 'hit':
        if (ev.target) this.flashes.set(ev.target, t0);
        if ((ev.amount ?? 0) > 0) push('dmg', 1300, { crit: ev.fx === 'crit', hero: (ev.target ?? 0) < 10000, text: String(ev.amount) });
        break;
      case 'potion': push('dmg', 1300, { text: '+' + ev.amount, pal: 'heal' }); break;
      case 'revive': push('flare', 900, { pal: 'revive', x: ev.x, y: ev.y }); break;
      case 'levelup': push('dmg', 1800, { text: 'LEVEL UP', pal: 'level' }); break;
      case 'absorb': push('dmg', 900, { text: '(' + ev.amount + ')', pal: 'absorb' }); break;
    }
    if (this.list.length > 160) this.list.splice(0, this.list.length - 120);
  }

  prune(now: number) {
    this.list = this.list.filter((f) => now - f.t0 < f.dur);
  }

  private blastScale(f: Effect) { return f.pal === 'void' ? Math.max(0.5, f.r / 9) : Math.max(0.6, f.r / 9.5); }

  lights(now: number, st: GameState, heroes: { x: number; y: number }[]): Light[] {
    const out: Light[] = [];
    for (const f of this.list) {
      const e = (now - f.t0) / 1000;
      if (e < 0) continue;
      const P = PALS[f.pal] ?? PALS.ember;
      switch (f.kind) {
        case 'charge': {
          const T = f.dur / 1000;
          out.push({ x: f.x, y: f.y, r: 9, c: nr(P.ramp[1]), i: 0.9 * c01(e / (T * 0.8)) });
          if (e > T * 0.45) {
            const pp = c01((e - T * 0.45) / (T * 0.55));
            const ph = pp * pp;
            out.push({ x: f.x + (f.tx - f.x) * ph, y: f.y + (f.ty - f.y) * ph, r: 11, c: nr(P.ramp[1]), i: 1.3 });
          }
          break;
        }
        case 'blast': {
          const sc = this.blastScale(f);
          out.push({ x: f.tx, y: f.ty, r: 48 * sc, c: nr(ramp(P.ramp, Math.min(0.85, e * 0.7))), i: 2.8 * Math.exp(-e * 2.2) });
          break;
        }
        case 'bolt': {
          const p = c01(e / (f.dur / 1000));
          out.push({ x: f.x + (f.tx - f.x) * p, y: f.y + (f.ty - f.y) * p, r: 7, c: nr(PALS.ember.ramp[1]), i: 0.9 });
          break;
        }
        case 'leap': out.push({ x: f.tx, y: f.ty, r: 12, c: nr(GREYGOLD), i: 0.9 * Math.exp(-e * 3.5) }); break;
        case 'bell': {
          const k = e < 0.2 ? e / 0.2 : Math.max(0, 1 - (e - 0.2) / 1.5);
          for (const h of heroes) if (dist(h.x, h.y, f.x, f.y) <= f.r + 0.5) out.push({ x: h.x, y: h.y, r: 8, c: WARM, i: 0.9 * k });
          out.push({ x: f.x, y: f.y, r: f.r * 2, c: WARM, i: 0.5 * k });
          break;
        }
        case 'blink': out.push({ x: f.tx, y: f.ty, r: 7, c: nr([95, 215, 255]), i: 0.8 * (1 - e / 0.42) }); break;
        case 'flare': {
          const c = f.pal === 'wall' ? WARM : f.pal === 'revive' ? nr([126, 240, 178]) : nr([199, 146, 255]);
          out.push({ x: f.tx, y: f.ty, r: 7, c, i: 0.8 * (1 - e / (f.dur / 1000)) });
          break;
        }
      }
    }
    void st;
    return out;
  }

  /** Ground scorch around fresh Ember Nova impacts (heatK as in the design). */
  heat(now: number): { x: number; y: number; k: number; rad: number }[] {
    const out: { x: number; y: number; k: number; rad: number }[] = [];
    for (const f of this.list) {
      if (f.kind !== 'blast' || f.pal !== 'ember') continue;
      const e = (now - f.t0) / 1000;
      if (e < 0) continue;
      const k = e < 0.35 ? e / 0.35 : Math.exp(-(e - 0.35) * 0.55);
      out.push({ x: f.tx, y: f.ty, k, rad: 14 * this.blastScale(f) });
    }
    return out;
  }

  /** Camera shake for the local hero at (hx,hy). */
  shake(now: number, hx_: number, hy: number): [number, number] {
    let ox = 0, oy = 0;
    for (const f of this.list) {
      const e = (now - f.t0) / 1000;
      if (e < 0) continue;
      if (f.kind === 'blast' && f.pal === 'ember' && e < 0.4 && dist(hx_, hy, f.tx, f.ty) < 34) {
        const kx = 1 - e / 0.4;
        ox += Math.round(Math.sin(e * 95) * 2.4 * kx);
        oy += e < 0.14 ? (Math.floor(e * 40) % 2 ? 1 : -1) : 0;
      } else if (f.kind === 'leap' && e < 0.25 && Math.abs(hx_ - f.tx) <= 12 && Math.abs(hy - f.ty) <= 7) {
        ox += Math.round(Math.sin(e * 110) * (1 - e / 0.25));
      }
    }
    return [Math.max(-3, Math.min(3, ox)), Math.max(-1, Math.min(1, oy))];
  }

  /** Ground-level glyph effects, drawn after the map and before entities. */
  drawUnder(mb: Fb, cam: Cam, now: number, st: GameState) {
    const isOpen: IsOpen = (vx, vy) => {
      const wx = vx + cam.x, wy = vy + cam.y;
      if (!st.isVisible(wx, wy)) return false;
      const t = st.tileAt(wx, wy);
      return t !== 0 && t !== 35 && t !== 84 && t !== 79; // not # T O
    };
    for (const f of this.list) {
      const e = (now - f.t0) / 1000;
      if (e < 0) continue;
      const sx = f.x - cam.x, sy = f.y - cam.y, tx = f.tx - cam.x, ty = f.ty - cam.y;
      switch (f.kind) {
        case 'blast':
          drawBlast(mb, tx, ty, e, PALS[f.pal] ?? PALS.ember, isOpen, f.seed & 7, this.blastScale(f));
          break;
        case 'leap': this.drawLeap(mb, tx, ty, e, isOpen, f.seed); break;
        case 'bell': this.drawBell(mb, sx, sy, e, f.r, isOpen); break;
        case 'slash': this.drawSlash(mb, sx, sy, tx, ty, e, f); break;
        case 'blink': {
          const a = c01(1 - e / 0.42);
          for (const [px, py] of [[sx, sy], [tx, ty]]) {
            const rr = e * 14;
            for (let k = 0; k < 10; k++) {
              const ang = (k / 10) * Math.PI * 2 + f.seed;
              const x = Math.round(px + Math.cos(ang) * rr), y = Math.round(py + Math.sin(ang) * rr / 2);
              if (isOpen(x, y)) mb.put(x, y, a > 0.5 ? '*' : '·', mix([20, 40, 60], [140, 235, 255], a), null, 1);
            }
          }
          break;
        }
        case 'trail': {
          const p = c01(e / (f.dur / 1000));
          const n = 9;
          for (let k = 0; k <= n; k++) {
            const q = (k / n) * p;
            const x = Math.round(sx + (tx - sx) * q), y = Math.round(sy + (ty - sy) * q - (f.pal === 'leapArc' ? 0 : Math.sin(q * Math.PI) * 2));
            if (isOpen(x, y)) mb.put(x, y, k === n ? '•' : '·', mix([60, 50, 30], GREYGOLD, k / n), null, 1);
          }
          break;
        }
        case 'flare': {
          const a = c01(1 - e / (f.dur / 1000));
          const col: RGB = f.pal === 'wall' ? [255, 212, 92] : f.pal === 'revive' ? [126, 240, 178] : [199, 146, 255];
          const rr = 1 + e * 5;
          for (let k = 0; k < 12; k++) {
            const ang = (k / 12) * Math.PI * 2;
            const x = Math.round(tx + Math.cos(ang) * rr), y = Math.round(ty + Math.sin(ang) * rr / 2);
            if (isOpen(x, y)) mb.put(x, y, f.pal === 'caltrops' ? '∴' : '·', mul(col, 0.4 + 0.6 * a), null, 1);
          }
          break;
        }
      }
    }
  }

  private drawLeap(mb: Fb, cx: number, cy: number, e: number, isOpen: IsOpen, seed: number) {
    const rs = 1 + 16 * (1 - Math.pow(1 - c01(e / 0.7), 2));
    const a = c01(1 - e / 0.75);
    if (a > 0) {
      const rr = Math.ceil(rs) + 2;
      for (let y = Math.floor(cy - rr / 2) - 1; y <= cy + rr / 2 + 1; y++) for (let x = cx - rr; x <= cx + rr; x++) {
        if (!isOpen(x, y)) continue;
        const d0 = dist(x, y, cx, cy);
        if (Math.abs(d0 - rs) < 1.1) {
          const i = y * mb.w + x;
          if (i < 0 || i >= mb.w * mb.h) continue;
          mb.put(x, y, tangent(x, y, cx, cy), mix(mb.getBg(i), GREYGOLD, a), null, 1);
        }
      }
    }
    for (let i = 0; i < 34; i++) {
      const ang = hash(i, 4, seed) * Math.PI * 2, spd = 5 + hash(i, 5, seed) * 14, life = 0.4 + hash(i, 6, seed) * 0.6;
      if (e > life) continue;
      const dd = spd * (1 - Math.exp(-e * 4)) / 2.5;
      const px = Math.round(cx + Math.cos(ang) * dd), py = Math.round(cy + Math.sin(ang) * dd / 2);
      if (!isOpen(px, py)) continue;
      const k = 1 - e / life;
      mb.put(px, py, '░', mul([170, 150, 100], 0.4 + 0.6 * k), mul([60, 52, 38], k), 0);
    }
  }

  private drawBell(mb: Fb, cx: number, cy: number, e: number, R: number, isOpen: IsOpen) {
    const gold: RGB = [255, 212, 92];
    const ring = (t0: number, dur: number, rad: number) => {
      const t = (e - t0) / dur;
      if (t < 0 || t > 1) return;
      const rs = 1 + (rad - 1) * (1 - Math.pow(1 - t, 2)), a = 1 - t;
      const rr = Math.ceil(rs) + 2;
      for (let y = Math.floor(cy - rr / 2) - 1; y <= cy + rr / 2 + 1; y++) for (let x = cx - rr; x <= cx + rr; x++) {
        if (!isOpen(x, y) || Math.abs(dist(x, y, cx, cy) - rs) >= 1.1) continue;
        const i = y * mb.w + x;
        mb.put(x, y, tangent(x, y, cx, cy), mix(mb.getBg(i), gold, a), null, 1);
      }
    };
    ring(0, 0.8, R);
    ring(0.28, 0.8, R); // expands twice
    const t = (e - 0.75) / 0.9;
    if (t >= 0 && t <= 1) { // the "○" echo
      const rs = R * (1.05 + 0.25 * t), a = 1 - t, n = Math.round(rs * 3);
      for (let k = 0; k < n; k++) {
        const ang = (k / n) * Math.PI * 2;
        const x = Math.round(cx + Math.cos(ang) * rs), y = Math.round(cy + Math.sin(ang) * rs / 2);
        if (isOpen(x, y)) mb.put(x, y, '○', mix([50, 40, 20], gold, a), null, 1);
      }
    }
  }

  private drawSlash(mb: Fb, sx: number, sy: number, tx: number, ty: number, e: number, f: Effect) {
    const a = c01(1 - e / 0.18);
    const col = mix([60, 50, 50], [255, 245, 220], a);
    const dx = Math.sign(tx - sx), dy = Math.sign(ty - sy);
    const glyph = dx === 0 ? '─' : dy === 0 ? '│' : dx * dy > 0 ? '╱' : '╲';
    const cells: [number, number][] = [[sx + dx, sy + dy]];
    if (f.text === 'cleave') {
      if (dx === 0 || dy === 0) cells.push([sx + dx + dy, sy + dy + dx], [sx + dx - dy, sy + dy - dx]);
      else cells.push([sx + dx, sy], [sx, sy + dy]);
    }
    for (const [x, y] of cells) mb.put(x, y, f.text === 'palm' ? '»' : glyph, col, null, 1);
  }

  /** Projectiles, charge swirls and floating numbers: drawn on top of entities. */
  drawOver(mb: Fb, cam: Cam, now: number, st: GameState) {
    for (const f of this.list) {
      const e = (now - f.t0) / 1000;
      if (e < 0) continue;
      const sx = f.x - cam.x, sy = f.y - cam.y, tx = f.tx - cam.x, ty = f.ty - cam.y;
      const P = PALS[f.pal] ?? PALS.ember;
      if (f.kind === 'charge') {
        const T = f.dur / 1000;
        if (e < T * 0.55) {
          for (let q = 0; q < 4; q++) {
            const a = e * 10 + (q * Math.PI) / 2;
            mb.put(sx + Math.round(Math.cos(a) * 2), sy + Math.round(Math.sin(a)), '•', ramp(P.ramp, 0.15 + q * 0.1), null, 1);
          }
        }
        if (e > T * 0.45) {
          const pp = c01((e - T * 0.45) / (T * 0.55)), ph = pp * pp;
          const arc = f.pal === 'void' ? -Math.sin(pp * Math.PI) * 3 : 0;
          const px = sx + (tx - sx) * ph, py = sy + (ty - sy) * ph + arc;
          for (let k = 8; k >= 1; k--) {
            const q = Math.max(0, ph - k * 0.045);
            const x = Math.round(sx + (tx - sx) * q), y = Math.round(sy + (ty - sy) * q + (f.pal === 'void' ? -Math.sin(Math.sqrt(q) * Math.PI) * 3 : 0));
            if (k < 8) mb.put(x, y, k < 3 ? '•' : '·', ramp(P.ramp, k / 9), null, 1);
          }
          mb.put(px, py, '●', [255, 255, 255], mul(P.ramp[1], 0.55), 1);
        }
      } else if (f.kind === 'bolt') {
        const p = c01(e / (f.dur / 1000));
        for (let k = 4; k >= 0; k--) {
          const q = Math.max(0, p - k * 0.08);
          mb.put(Math.round(sx + (tx - sx) * q), Math.round(sy + (ty - sy) * q), k === 0 ? '●' : k < 3 ? '•' : '·', k === 0 ? [255, 255, 255] : ramp(PALS.ember.ramp, 0.15 + k * 0.12), k === 0 ? mul(PALS.ember.ramp[1], 0.5) : null, 1);
        }
      } else if (f.kind === 'dmg') {
        const age = e, yy = ty - 1 - Math.floor(age * 2.6), s = f.text ?? '';
        const xx = tx - Math.floor(s.length / 2), fd = age > 0.8 ? c01(1 - (age - 0.8) / 0.5) : 1;
        if (f.pal === 'heal') mb.txt(xx, yy, s, mix([20, 40, 30], [126, 240, 178], fd), null, 1);
        else if (f.pal === 'level') mb.txt(xx, yy - 1, s, mix([20, 20, 10], [255, 212, 92], fd), null, 1);
        else if (f.pal === 'absorb') mb.txt(xx, yy, s, mix([20, 20, 20], [255, 212, 92], fd), null, 1);
        else if (f.crit) mb.txt(xx - 1, yy, ' ' + s + ' ', [255, 255, 255], mix([9, 9, 14], PINK, fd), 1);
        else if (f.hero) mb.txt(xx, yy, s, mix([40, 20, 24], [255, 90, 120], fd), null, 1);
        else mb.txt(xx, yy, s, mix([40, 30, 20], [255, 225, 77], fd), null, 1);
      }
    }
    void st; void DIM; void CLS;
  }
}
