// Map viewport renderer: a port of the lighting and tile drawing in design/Main.dc.html (lines 176-220).
import { Fb, RGB, add, dist, mix, c01, mul, hash } from '../term/fb';
import { GameState, LW, LH } from '../state';
import { MC, CLS } from '../classes';
import type { Ent } from '../proto';
import type { Cam, Light } from './fx';
import { palette } from './themes';
import { ITEMS, rarityCol, gearCh } from '../items';

export const VW = 112;
export const VH = 34;
const GEAR_LABEL_RANGE = 8; // gear on the floor this close to you shows its name

interface Glyph { ch: string; fg: RGB; lit?: boolean; b?: boolean }
const GLY: Record<string, Glyph> = {
  '.': { ch: '·', fg: [118, 112, 142], lit: true },
  '+': { ch: '+', fg: [214, 150, 80], lit: true, b: true },
  '~': { ch: ',', fg: [170, 36, 54], lit: true },
  ':': { ch: ':', fg: [120, 108, 96], lit: true },
  '%': { ch: '%', fg: [214, 204, 184], lit: true },
  '$': { ch: '$', fg: [255, 212, 92], b: true },
  '!': { ch: '!', fg: [255, 92, 122], b: true },
  '&': { ch: '&', fg: [110, 190, 255], b: true }, // resource draught
  '*': { ch: '*', fg: [255, 140, 50], b: true },  // firebomb
  'g': { ch: '♦', fg: [230, 60, 90], b: true },   // garnet
  'C': { ch: '■', fg: [214, 150, 80], b: true },   // closed chest (or a mimic pretending)
  't': { ch: '¡', fg: [255, 170, 70], b: true },   // lamp oil
  '?': { ch: '?', fg: [230, 230, 170], b: true },  // scroll of light
  '>': { ch: '>', fg: [120, 240, 255], b: true },
  '<': { ch: '<', fg: [160, 220, 150], b: true }, // stairs up
  'L': { ch: '▓', fg: [214, 170, 80], lit: true, b: true }, // a locked vault door
  'V': { ch: '■', fg: [255, 220, 110], b: true },  // the vault's treasure chest
  '=': { ch: '≈', fg: [70, 140, 170], lit: true },
  '"': { ch: '♣', fg: [120, 230, 140], b: true },
  '^': { ch: '^', fg: [255, 120, 50], b: true },
  // furniture (solid on the server, drawn as a lit glyph on the floor)
  '|': { ch: '†', fg: [236, 206, 150], lit: true, b: true }, // candle stand
  '_': { ch: 'Π', fg: [150, 162, 172], lit: true, b: true }, // coffin
  'P': { ch: '♠', fg: [110, 200, 170], lit: true, b: true }, // giant mushroom stalk
  'u': { ch: '∪', fg: [196, 136, 84], b: true },             // an urn (walk into it to smash it)
};

/** Monster glyphs that differ from their kind letter. */
const MON_CH: Record<string, string> = { B: '¤' };

const GNOME: RGB = [140, 220, 120];
const GNOME_TRADER: RGB = [255, 212, 92];

export function buildLights(st: GameState, cam: Cam, t: number, heroes: { x: number; y: number; torch?: boolean }[], extra: Light[]): Light[] {
  const out: Light[] = [];
  const inView = (x: number, y: number, r: number) =>
    x + r >= cam.x && x - r <= cam.x + VW && y + r / 2 >= cam.y && y - r / 2 <= cam.y + VH;
  const lv = st.level;
  const pal = palette(lv?.theme);
  if (lv) {
    lv.torches.forEach((p, i) => {
      if (!inView(p[0], p[1], pal.torchR) || st.tileAt(p[0], p[1]) === 0) return;
      out.push({ x: p[0], y: p[1], r: pal.torchR, c: pal.torch, i: 0.95 * (0.8 + 0.2 * Math.sin(t * 8.3 + i * 1.7) * Math.cos(t * 5.1 + i * 0.9)) });
    });
    const gl = pal.glow;
    if (gl) (lv.glows ?? []).forEach((p, i) => {
      if (!inView(p[0], p[1], gl.r) || st.tileAt(p[0], p[1]) === 0 || st.tileAt(p[0], p[1]) === 124) return;
      out.push({ x: p[0], y: p[1], r: gl.r, c: gl.c, i: gl.i * (1 - gl.flicker + gl.flicker * Math.sin(t * 6.1 + i * 2.3)) });
    });
    // candle stands are in the glow list too, even on themes without glowing decor
    (lv.glows ?? []).forEach((p, i) => {
      if (st.tileAt(p[0], p[1]) !== 124 || !inView(p[0], p[1], 7)) return; // '|'
      out.push({ x: p[0], y: p[1], r: 7, c: [1, 0.7, 0.36], i: 0.55 * (0.85 + 0.15 * Math.sin(t * 9.7 + i * 2.9)) });
    });
    const s = lv.stairs;
    if (st.tileAt(s[0], s[1]) !== 0 && inView(s[0], s[1], 9)) out.push({ x: s[0], y: s[1], r: 9, c: [0.27, 0.86, 1], i: 0.7 });
  }
  const dark = !!lv?.dark;
  heroes.forEach((h, k) => {
    // a lit torch is a warm flickering light; on the dark floor a hero without one barely glows
    const flick = 0.9 + 0.1 * Math.sin(t * 9.1 + k * 2.1);
    const l = h.torch ? { r: dark ? 13 : 18, c: [1, 0.72, 0.4], i: 0.75 * flick }
      : dark ? { r: 5, c: pal.hero, i: 0.45 } : { r: 16, c: pal.hero, i: 0.5 };
    if (inView(h.x, h.y, l.r)) out.push({ x: h.x, y: h.y, ...l });
  });
  for (const f of st.latest?.fires ?? []) {
    if (inView(f[0], f[1], 4)) out.push({ x: f[0], y: f[1], r: 4, c: [1, 0.45, 0.15], i: 0.35 + 0.1 * Math.sin(t * 11 + f[0]) });
  }
  for (const o of st.latest?.orbs ?? []) {
    if (inView(o[0], o[1], 12)) out.push({ x: o[0], y: o[1], r: 12, c: [0.75, 0.85, 1], i: Math.min(1, o[2] / 3) * (0.8 + 0.05 * Math.sin(t * 4)) });
  }
  for (const l of extra) if (inView(l.x, l.y, l.r)) out.push(l);
  return out;
}

export function drawMap(mb: Fb, st: GameState, cam: Cam, t: number, lights: Light[], heat: { x: number; y: number; k: number; rad: number }[]) {
  const fr7 = Math.floor(t * 7);
  const pal = palette(st.level?.theme);
  const floorG: Glyph = { ...GLY['.'], fg: pal.floorFg };
  const glyph = (c: string) => (c === '.' ? floorG : GLY[c] || floorG);
  // corpses and blood lie on top of the floor tile and take its light
  const groundGlyph = (c: string, i: number): Glyph => {
    const rm = st.remains.get(i);
    return rm ? { ch: rm[0], fg: rm[1], lit: true } : glyph(c);
  };
  for (let y = 0; y < VH; y++) {
    const wy = y + cam.y;
    if (wy < 0 || wy >= LH) continue;
    for (let x = 0; x < VW; x++) {
      const wx = x + cam.x;
      if (wx < 0 || wx >= LW) continue;
      const tc = st.tiles[wy * LW + wx];
      if (tc === 0) continue;
      const c = String.fromCharCode(tc);
      const solid = c === '#' || c === 'T' || c === 'O';
      const i = y * VW + x;
      if (!st.vis[wy * LW + wx]) {
        if (solid) mb.put(x, y, '▀', pal.memWallTop, pal.memWallFace, 0);
        else {
          const g = groundGlyph(c, wy * LW + wx);
          mb.put(x, y, g.ch, mix(g.fg, pal.memFloorFg, 0.72), pal.memFloorBg, 0);
        }
        continue;
      }
      let Lr = pal.ambient[0], Lg = pal.ambient[1], Lb = pal.ambient[2];
      for (let li = 0; li < lights.length; li++) {
        const s = lights[li];
        const d = dist(wx, wy, s.x, s.y);
        if (d >= s.r) continue;
        let f = 1 - d / s.r;
        f = f * f * s.i;
        Lr += s.c[0] * f; Lg += s.c[1] * f; Lb += s.c[2] * f;
      }
      Lr = Math.round(Lr * 14) / 14; Lg = Math.round(Lg * 14) / 14; Lb = Math.round(Lb * 14) / 14;
      if (solid) {
        const top = c === 'O' ? pal.pillarTop : pal.wallTop, face = c === 'O' ? pal.pillarFace : pal.wallFace;
        mb.put(x, y, '▀',
          [top[0] * (0.2 + Lr * 1.1), top[1] * (0.2 + Lg * 1.1), top[2] * (0.2 + Lb * 1.1)],
          [face[0] * (0.2 + Lr * 1.1), face[1] * (0.2 + Lg * 1.1), face[2] * (0.2 + Lb * 1.1)], 0);
        if (c === 'T') mb.put(x, y, 'Ψ', mix(pal.torchGlyph[0], pal.torchGlyph[1], 0.5 + 0.5 * Math.sin(t * 13 + x)), null, 1);
        continue;
      }
      const g = groundGlyph(c, wy * LW + wx);
      let bg: RGB = [pal.bgBase[0] + Lr * pal.bgLight[0], pal.bgBase[1] + Lg * pal.bgLight[1], pal.bgBase[2] + Lb * pal.bgLight[2]];
      let fg: RGB = g.lit ? [g.fg[0] * (0.25 + Lr), g.fg[1] * (0.25 + Lg), g.fg[2] * (0.25 + Lb)] : g.fg;
      if (c === '~') bg = add(bg, [26, 0, 6]);
      if (c === '=') bg = add(bg, [0, 10, 18]);
      if (c === '^') bg = add(bg, [40 + 20 * Math.sin(t * 3 + wx * 0.7), 8, 0]);
      if (c === '>') bg = [10, 44, 58];
      else if (c === '<') bg = [22, 44, 24];
      let scorched = false;
      for (const hh of heat) {
        const h = c01(1 - dist(wx, wy, hh.x, hh.y) / hh.rad) * hh.k;
        if (h > 0) {
          bg = add(bg, [92 * h, 24 * h, 10 * h]);
          if (h > 0.12 && hash(wx, wy, fr7) > 0.85) {
            mb.put(x, y, '·', mix([255, 120, 40], [255, 200, 120], hash(wx, wy, fr7 + 1)), bg, 1);
            scorched = true;
            break;
          }
          fg = mix(fg, [60, 30, 26], h * 0.6);
        }
      }
      if (scorched) continue;
      let ch = g.ch;
      if (c === '=' && !st.remains.has(wy * LW + wx)) { // water ripples
        const w = Math.sin(t * 1.7 + wx * 0.55 + wy * 0.9);
        if (w > 0.75) ch = '~';
        fg = mul(fg, 0.85 + 0.25 * w);
      } else if (c === '"') { // fungus breathes
        fg = mul(fg, 0.75 + 0.3 * Math.sin(t * 1.9 + wx * 0.37 + wy * 0.61));
      } else if (c === '|') { // a candle flame flickers
        fg = mul(fg, 0.85 + 0.2 * Math.sin(t * 11 + wx * 3.1));
      }
      mb.put(x, y, ch, fg, bg, g.b ? 1 : 0);
      void i;
    }
  }
  drawParticles(mb, st, cam, t);
}

/** Embers rising from cracks, lava and torches: client-only, seeded per cell so they look the same every frame. */
function drawParticles(mb: Fb, st: GameState, cam: Cam, t: number) {
  const open = (wx: number, wy: number) => {
    const tc = st.tileAt(wx, wy);
    return st.isVisible(wx, wy) && tc !== 0 && tc !== 35 && tc !== 84 && tc !== 79; // not # T O
  };
  const rise = (wx: number, wy: number, k: number, speed: number, hgt: number, col: RGB, spread: number) => {
    const ph = (t * speed + k * 7.3) % 1;
    const px = wx + Math.round(Math.sin(t * 2 + k * 11) * spread), py = wy - 1 - Math.floor(ph * hgt);
    if (!open(px, py)) return;
    const x = px - cam.x, y = py - cam.y;
    if (x < 0 || y < 0 || x >= VW || y >= VH) return;
    mb.put(x, y, ph < 0.5 ? "'" : '·', mul(col, 1 - ph * 0.8), null, 1);
  };
  for (let y = 0; y < VH; y++) {
    const wy = y + cam.y;
    for (let x = 0; x < VW; x++) {
      const wx = x + cam.x;
      const tc = st.tiles[wy * LW + wx];
      if (tc !== 94 || !st.vis[wy * LW + wx]) continue; // '^' ember cracks
      const k = hash(wx, wy, 3);
      if (k > 0.55) rise(wx, wy, k, 0.45, 4, [255, 150, 60], 0.4);
    }
  }
  for (const f of st.latest?.fires ?? []) {
    const k = hash(f[0], f[1], 5);
    if (k > 0.5) rise(f[0], f[1], k, 0.7, 3, [255, 190, 90], 0.6);
  }
  // torch sparks drop into the room below the wall
  for (const p of st.level?.torches ?? []) {
    if (!st.isVisible(p[0], p[1] + 1)) continue;
    const k = hash(p[0], p[1], 9), ph = (t * 0.9 + k) % 1;
    if (ph > 0.35) continue;
    const px = p[0] + Math.round(Math.sin(t * 3 + k * 20)), py = p[1] + 1 + Math.floor(ph * 4);
    const x = px - cam.x, y = py - cam.y;
    if (x >= 0 && y >= 0 && x < VW && y < VH && open(px, py)) mb.put(x, y, '·', mul([255, 200, 110], 1 - ph * 2), null, 1);
  }
}

export interface DrawEntsOpts {
  youId: number;
  now: number;
  flashes: Map<number, number>;
  hitCol: RGB;
}

/** Items, monsters, heroes and their tags. Positions in world coordinates; cam converts to viewport. */
export function drawEnts(mb: Fb, st: GameState, cam: Cam, ents: Ent[], youPos: { x: number; y: number } | null, o: DrawEntsOpts) {
  const inV = (x: number, y: number) => x >= 0 && y >= 0 && x < VW && y < VH;
  const order = (e: Ent) => (e.k === 'i' ? 0 : e.k === 'm' || e.k === 'n' ? 1 : 2);
  const list = ents.slice().sort((a, b) => order(a) - order(b));
  const pos = (e: Ent): [number, number] => {
    if (e.k === 'h' && e.id === o.youId && youPos) return [youPos.x - cam.x, youPos.y - cam.y];
    return [e.x - cam.x, e.y - cam.y];
  };
  // Pass 1: decorations (brackets, labels, timers); entity glyphs are drawn over them afterwards.
  for (const e of list) {
    const [x, y] = pos(e);
    if (!inV(x, y)) continue;
    if (e.k === 'm') {
      if (e.el) mb.txt(x - 1, y - 1, (e.name || '').replace(/^The /, '').toUpperCase(), [255, 110, 150], null, 1);
    } else if (e.k === 'i' && e.nm && youPos && Math.max(Math.abs(e.x - youPos.x), Math.abs(e.y - youPos.y)) <= GEAR_LABEL_RANGE) {
      mb.txt(x - Math.floor(e.nm.length / 2), y - 1, e.nm, mul(rarityCol(e.rar), 0.9), null, (e.rar ?? 0) >= 2 ? 1 : 0); // gear nearby: its name
    } else if (e.k === 'n') {
      const nm = e.name || '';
      const hot = e.trader || e.board || e.g === 'A';
      mb.txt(x - Math.floor(nm.length / 2), y - 1, nm, hot ? GNOME_TRADER : mul(GNOME, 0.8), null, hot ? 1 : 0);
      if (e.follow && e.hpMax) mb.bar(x - 2, y - 2, 5, (e.hp ?? 0) / e.hpMax, [126, 240, 178], [20, 40, 30]);
    } else if (e.k === 'h') {
      const col = (e.cls && CLS[e.cls].col) || [220, 220, 220];
      const you = e.id === o.youId;
      const wall = !!e.st?.some((s) => s.k === 'wall');
      if (e.downed && !e.dead) {
        const lab = String(e.downT ?? 0);
        mb.txt(x - Math.floor(lab.length / 2), y - 2, lab, [255, 46, 136], null, 1);
        if (e.rev && e.rev > 0) mb.bar(x - 2, y - 3, 5, e.rev, [126, 240, 178], [20, 40, 30]);
      } else if (!e.dead && (you || wall)) {
        const bc: RGB = wall ? [255, 212, 92] : mul([255, 212, 92], 0.8);
        mb.put(x - 1, y, '[', bc, null, 1);
        mb.put(x + 1, y, ']', bc, null, 1);
      }
      const nm = e.name || '';
      mb.txt(x - Math.floor(nm.length / 2), y - 1, nm, mul(col, e.dead ? 0.35 : 0.8), null, 0);
    }
  }
  // burning ground (golem trails, dragon breath)
  for (const f of st.latest?.fires ?? []) {
    const x = f[0] - cam.x, y = f[1] - cam.y;
    if (!inV(x, y)) continue;
    const k = hash(f[0], f[1], Math.floor(o.now / 140));
    mb.put(x, y, k > 0.5 ? '^' : '"', mix([255, 90, 20], [255, 210, 80], k), mix([70, 14, 4], [110, 30, 6], k), 1);
  }
  // the Scroll of Return's portal: a swirl on its cell
  const pt = st.latest?.portal;
  if (pt) {
    const x = pt[0] - cam.x, y = pt[1] - cam.y;
    if (inV(x, y)) mb.put(x, y, '◎', mix([90, 200, 255], [220, 250, 255], 0.5 + 0.5 * Math.sin(o.now / 120)), [20, 40, 70], 1);
  }
  // loot beams: a pillar of light over rare things on the floor
  for (const e of list) {
    if (e.k !== 'i' || (e.rar ?? 0) < 2) continue;
    const [x, y] = pos(e);
    const col = rarityCol(e.rar), H = 7;
    for (let dy = 0; dy <= H; dy++) {
      const yy = y - dy;
      if (!inV(x, yy)) continue;
      const i = yy * mb.w + x;
      const shimmer = 0.75 + 0.25 * Math.sin(o.now / 160 - dy * 0.9);
      const k = (dy === 0 ? 0.55 : 0.5 * (1 - dy / (H + 1))) * shimmer;
      mb.setBg(i, mix(mb.getBg(i), col, k));
      for (const dx of [-1, 1]) if (dy > 0 && inV(x + dx, yy)) { const j = yy * mb.w + x + dx; mb.setBg(j, mix(mb.getBg(j), col, k * 0.25)); }
    }
  }
  // Pass 2: glyphs, items then monsters then heroes.
  for (const e of list) {
    const [x, y] = pos(e);
    if (!inV(x, y)) continue;
    if (e.k === 'n') {
      if (e.board) mb.put(x, y, '≡', GNOME_TRADER, [60, 40, 16], 1);
      else if (e.g === 'A') mb.put(x, y, 'Ω', [255, 236, 170], [50, 40, 20], 1); // an altar of the Four
      else if (e.g === 'i') mb.put(x, y, '¶', [226, 210, 170], [40, 34, 26], 1); // an inscription
      else mb.put(x, y, 'g', e.trader ? GNOME_TRADER : GNOME, e.follow ? [20, 50, 30] : null, 1);
      continue;
    }
    if (e.k === 'i') {
      const info = e.it ? ITEMS[e.it] : undefined; // rucksack items by id; gold and chests by glyph
      const g = info?.gear ? { ch: gearCh(undefined, e.it), fg: rarityCol(e.rar) } : info ? { ch: info.ch, fg: info.col } : GLY[e.g];
      if (g) mb.put(x, y, g.ch, g.fg, null, 1);
    } else if (e.k === 'm') {
      let fg = MC[e.g] || [200, 200, 200], bg: RGB | null = null;
      const fl = o.flashes.get(e.id);
      if (fl !== undefined && o.now - fl >= 0 && o.now - fl < 130) { fg = [255, 255, 255]; bg = [255, 138, 31]; }
      if (e.el) bg = bg || [64, 10, 32];
      mb.put(x, y, MON_CH[e.g] ?? e.g, fg, bg, 1);
    } else if (e.k === 'h') {
      const col = (e.cls && CLS[e.cls].col) || [220, 220, 220];
      if (e.dead) {
        mb.put(x, y, 'x', mul(col, 0.25), null, 0);
      } else if (e.downed) {
        mb.put(x, y, 'x', mul(col, 0.5), mul(col, 0.12), 1);
      } else {
        let bg = mul(col, 0.3);
        if (o.flashes.has(e.id) && o.now - o.flashes.get(e.id)! < 150) bg = mix(bg, o.hitCol, 0.6);
        mb.put(x, y, '@', col, bg, 1);
      }
    }
  }
}
