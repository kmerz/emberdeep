// Map viewport renderer: a port of the lighting and tile drawing in design/Main.dc.html (lines 176-220).
import { Fb, RGB, add, dist, mix, c01, mul, hash } from '../term/fb';
import { GameState, LW, LH } from '../state';
import { MC, CLS } from '../classes';
import type { Ent } from '../proto';
import type { Cam, Light } from './fx';
import { palette } from './themes';

export const VW = 112;
export const VH = 34;

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
  '>': { ch: '>', fg: [120, 240, 255], b: true },
  '=': { ch: '≈', fg: [70, 140, 170], lit: true },
  '"': { ch: '♣', fg: [120, 230, 140], b: true },
  '^': { ch: '^', fg: [255, 120, 50], b: true },
};

export function buildLights(st: GameState, cam: Cam, t: number, heroes: { x: number; y: number }[], extra: Light[]): Light[] {
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
      if (!inView(p[0], p[1], gl.r) || st.tileAt(p[0], p[1]) === 0) return;
      out.push({ x: p[0], y: p[1], r: gl.r, c: gl.c, i: gl.i * (1 - gl.flicker + gl.flicker * Math.sin(t * 6.1 + i * 2.3)) });
    });
    const s = lv.stairs;
    if (st.tileAt(s[0], s[1]) !== 0 && inView(s[0], s[1], 9)) out.push({ x: s[0], y: s[1], r: 9, c: [0.27, 0.86, 1], i: 0.7 });
  }
  for (const h of heroes) if (inView(h.x, h.y, 16)) out.push({ x: h.x, y: h.y, r: 16, c: pal.hero, i: 0.5 });
  for (const l of extra) if (inView(l.x, l.y, l.r)) out.push(l);
  return out;
}

export function drawMap(mb: Fb, st: GameState, cam: Cam, t: number, lights: Light[], heat: { x: number; y: number; k: number; rad: number }[]) {
  const fr7 = Math.floor(t * 7);
  const pal = palette(st.level?.theme);
  const floorG: Glyph = { ...GLY['.'], fg: pal.floorFg };
  const glyph = (c: string) => (c === '.' ? floorG : GLY[c] || floorG);
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
          const g = glyph(c);
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
      const g = glyph(c);
      let bg: RGB = [pal.bgBase[0] + Lr * pal.bgLight[0], pal.bgBase[1] + Lg * pal.bgLight[1], pal.bgBase[2] + Lb * pal.bgLight[2]];
      let fg: RGB = g.lit ? [g.fg[0] * (0.25 + Lr), g.fg[1] * (0.25 + Lg), g.fg[2] * (0.25 + Lb)] : g.fg;
      if (c === '~') bg = add(bg, [26, 0, 6]);
      if (c === '=') bg = add(bg, [0, 10, 18]);
      if (c === '^') bg = add(bg, [40 + 20 * Math.sin(t * 3 + wx * 0.7), 8, 0]);
      if (c === '>') bg = [10, 44, 58];
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
      mb.put(x, y, g.ch, fg, bg, g.b ? 1 : 0);
      void i;
    }
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
  const order = (e: Ent) => (e.k === 'i' ? 0 : e.k === 'm' ? 1 : 2);
  const list = ents.slice().sort((a, b) => order(a) - order(b));
  for (const e of list) {
    let ex = e.x, ey = e.y;
    if (e.k === 'h' && e.id === o.youId && youPos) { ex = youPos.x; ey = youPos.y; }
    const x = ex - cam.x, y = ey - cam.y;
    if (!inV(x, y)) continue;
    if (e.k === 'i') {
      const g = GLY[e.g];
      if (g) mb.put(x, y, g.ch, g.fg, null, 1);
    } else if (e.k === 'm') {
      let fg = MC[e.g] || [200, 200, 200], bg: RGB | null = null;
      const fl = o.flashes.get(e.id);
      if (fl !== undefined && o.now - fl >= 0 && o.now - fl < 130) { fg = [255, 255, 255]; bg = [255, 138, 31]; }
      if (e.el) bg = bg || [64, 10, 32];
      mb.put(x, y, e.g, fg, bg, 1);
      if (e.el) mb.txt(x - 1, y - 1, (e.name || '').replace(/^The /, '').toUpperCase(), [255, 110, 150], null, 1);
    } else if (e.k === 'h') {
      const col = (e.cls && CLS[e.cls].col) || [220, 220, 220];
      const you = e.id === o.youId;
      const wall = !!e.st?.some((s) => s.k === 'wall');
      if (e.dead) {
        mb.put(x, y, 'x', mul(col, 0.25), null, 0);
      } else if (e.downed) {
        mb.put(x, y, 'x', mul(col, 0.5), mul(col, 0.12), 1);
        const lab = String(e.downT ?? 0);
        mb.txt(x - Math.floor(lab.length / 2), y - 2, lab, [255, 46, 136], null, 1);
        if (e.rev && e.rev > 0) mb.bar(x - 2, y - 3, 5, e.rev, [126, 240, 178], [20, 40, 30]);
      } else {
        if (you || wall) {
          const bc: RGB = wall ? [255, 212, 92] : mul([255, 212, 92], 0.8);
          mb.put(x - 1, y, '[', bc, null, 1);
          mb.put(x + 1, y, ']', bc, null, 1);
        }
        let bg = mul(col, 0.3);
        if (o.flashes.has(e.id) && o.now - o.flashes.get(e.id)! < 150) bg = mix(bg, o.hitCol, 0.6);
        mb.put(x, y, wall && !you ? '@' : '@', col, bg, 1);
        if (wall) { mb.put(x - 1, y, '[', [255, 212, 92], null, 1); mb.put(x + 1, y, ']', [255, 212, 92], null, 1); }
      }
      const nm = e.name || '';
      mb.txt(x - Math.floor(nm.length / 2), y - 1, nm, mul(col, e.dead ? 0.35 : 0.8), null, 0);
    }
  }
}
