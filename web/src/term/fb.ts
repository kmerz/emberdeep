// Cell framebuffer: a TypeScript port of the helpers in design/Main.dc.html (fbNew/put/txt/segs/fill/box/bar...).
export type RGB = number[];

export const W = 160;
export const H = 48;

export function hx(h: string): RGB {
  const n = parseInt(h.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
export const cl = (v: number) => (v < 0 ? 0 : v > 255 ? 255 : v);
export const c01 = (t: number) => (t < 0 ? 0 : t > 1 ? 1 : t);
export const mix = (a: RGB, b: RGB, t: number): RGB => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t];
export const mul = (a: RGB, k: number): RGB => [a[0] * k, a[1] * k, a[2] * k];
export const add = (a: RGB, b: RGB): RGB => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
export function ramp(p: RGB[], t: number): RGB {
  t = c01(t) * (p.length - 1);
  const i = Math.min(p.length - 2, Math.floor(t));
  return mix(p[i], p[i + 1], t - i);
}
export function hash(x: number, y: number, s: number): number {
  let h = (Math.imul(x | 0, 374761393) + Math.imul(y | 0, 668265263) + Math.imul(s | 0, 982451653)) | 0;
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  h ^= h >>> 16;
  return (h >>> 0) / 4294967296;
}
/** Aspect-corrected distance (cells are twice as tall as wide) - same rule as the server. */
export function dist(ax: number, ay: number, bx: number, by: number): number {
  const dx = ax - bx, dy = (ay - by) * 2;
  return Math.sqrt(dx * dx + dy * dy);
}

export const PAN: RGB = [13, 12, 20];
export const BRD: RGB = [58, 54, 86];
export const DIM: RGB = [120, 114, 150];
export const TX: RGB = [214, 208, 234];
export const WH: RGB = [240, 236, 255];
export const PINK: RGB = [255, 46, 136];
export const GOLD: RGB = [255, 212, 92];
export const MINT: RGB = [126, 240, 178];
export const AMB: RGB = [255, 176, 70];

export class Fb {
  w: number;
  h: number;
  ch: string[];
  fg: Float32Array;
  bg: Float32Array;
  bold: Uint8Array;
  constructor(w: number, h: number, bg: RGB) {
    this.w = w;
    this.h = h;
    const n = w * h;
    this.ch = new Array(n).fill(' ');
    this.fg = new Float32Array(n * 3);
    this.bg = new Float32Array(n * 3);
    this.bold = new Uint8Array(n);
    this.reset(bg);
  }
  reset(bg: RGB) {
    const n = this.w * this.h;
    this.ch.fill(' ');
    this.bold.fill(0);
    for (let i = 0; i < n; i++) {
      this.fg[i * 3] = 200; this.fg[i * 3 + 1] = 200; this.fg[i * 3 + 2] = 210;
      this.bg[i * 3] = bg[0]; this.bg[i * 3 + 1] = bg[1]; this.bg[i * 3 + 2] = bg[2];
    }
  }
  put(x: number, y: number, ch: string | null, fg?: RGB | null, bg?: RGB | null, b?: number | null) {
    x = Math.round(x); y = Math.round(y);
    if (x < 0 || y < 0 || x >= this.w || y >= this.h) return;
    const i = y * this.w + x;
    if (ch != null) this.ch[i] = ch;
    if (fg) { this.fg[i * 3] = fg[0]; this.fg[i * 3 + 1] = fg[1]; this.fg[i * 3 + 2] = fg[2]; }
    if (bg) { this.bg[i * 3] = bg[0]; this.bg[i * 3 + 1] = bg[1]; this.bg[i * 3 + 2] = bg[2]; }
    if (b != null) this.bold[i] = b ? 1 : 0;
  }
  setBg(i: number, c: RGB) { this.bg[i * 3] = c[0]; this.bg[i * 3 + 1] = c[1]; this.bg[i * 3 + 2] = c[2]; }
  setFg(i: number, c: RGB) { this.fg[i * 3] = c[0]; this.fg[i * 3 + 1] = c[1]; this.fg[i * 3 + 2] = c[2]; }
  getBg(i: number): RGB { return [this.bg[i * 3], this.bg[i * 3 + 1], this.bg[i * 3 + 2]]; }
  getFg(i: number): RGB { return [this.fg[i * 3], this.fg[i * 3 + 1], this.fg[i * 3 + 2]]; }
  txt(x: number, y: number, s: string, fg?: RGB | null, bg?: RGB | null, b?: number | null) {
    for (let i = 0; i < s.length; i++) this.put(x + i, y, s[i], fg, bg, b);
  }
  ctxt(y: number, s: string, fg?: RGB | null, bg?: RGB | null, b?: number | null) {
    this.txt(Math.floor((this.w - s.length) / 2), y, s, fg, bg, b);
  }
  segs(x: number, y: number, arr: Seg[]): number {
    for (const s of arr) {
      this.txt(x, y, s[0], s[1], s[2] || null, s[3] || 0);
      x += s[0].length;
    }
    return x;
  }
  fill(x: number, y: number, w: number, h: number, bg: RGB) {
    for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) this.put(x + i, y + j, ' ', null, bg);
  }
  box(x: number, y: number, w: number, h: number, title?: string, accent?: RGB) {
    this.fill(x, y, w, h, PAN);
    for (let i = 1; i < w - 1; i++) { this.put(x + i, y, '─', BRD); this.put(x + i, y + h - 1, '─', BRD); }
    for (let j = 1; j < h - 1; j++) { this.put(x, y + j, '│', BRD); this.put(x + w - 1, y + j, '│', BRD); }
    this.put(x, y, '╭', BRD); this.put(x + w - 1, y, '╮', BRD);
    this.put(x, y + h - 1, '╰', BRD); this.put(x + w - 1, y + h - 1, '╯', BRD);
    if (title) {
      this.put(x + 2, y, ' ', BRD);
      this.txt(x + 3, y, title, accent || [236, 228, 255], null, 1);
      this.put(x + 3 + title.length, y, ' ', BRD);
    }
  }
  bar(x: number, y: number, w: number, frac: number, fg: RGB, bgc: RGB) {
    const n = Math.round(c01(frac) * w * 8);
    for (let i = 0; i < w; i++) {
      const v = n - i * 8;
      const ch = v >= 8 ? '█' : v <= 0 ? ' ' : EIGHTHS[v];
      this.put(x + i, y, ch, fg, bgc);
    }
  }
}
export type Seg = [string, RGB, (RGB | null)?, number?];
const EIGHTHS = ' ▏▎▍▌▋▊▉';
