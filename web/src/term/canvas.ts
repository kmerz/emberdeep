import { Fb, W, H } from './fb';

export const CELL_W = 9;
export const CELL_H = 18;
const FONT = "'JetBrains Mono', ui-monospace, Menlo, Consolas, monospace";

const q = (v: number) => (v < 0 ? 0 : v > 255 ? 255 : Math.round(v));
const css = (n: number) => '#' + (n | 0x1000000).toString(16).slice(1);

/** A rectangle of cells. */
export interface Rect { x: number; y: number; w: number; h: number }
export const FULL: Rect = { x: 0, y: 0, w: W, h: H };

/** Map a point on the canvas (client coordinates) to a framebuffer cell, through the shown view. */
export function viewCell(view: Rect, box: { left: number; top: number; width: number; height: number }, cx: number, cy: number): [number, number] {
  return [view.x + Math.floor(((cx - box.left) / box.width) * view.w), view.y + Math.floor(((cy - box.top) / box.height) * view.h)];
}

/** The size of the visible page area in CSS px (shrinks while a phone keyboard is open). */
export function pageSize(): { w: number; h: number } {
  const vv = window.visualViewport;
  return vv ? { w: vv.width, h: vv.height } : { w: window.innerWidth, h: window.innerHeight };
}

/** Blits a framebuffer to a canvas, redrawing only the cells that changed since the last frame. */
export class TermCanvas {
  ctx: CanvasRenderingContext2D;
  dpr = 1;
  /** The part of the framebuffer that is shown (the whole grid, or a crop on phones). */
  view: Rect = { ...FULL };
  private lastCh: string[] = new Array(W * H).fill('');
  private lastFg = new Int32Array(W * H).fill(-1);
  private lastBg = new Int32Array(W * H).fill(-1);
  private lastB = new Uint8Array(W * H).fill(255);

  constructor(public canvas: HTMLCanvasElement) {
    this.ctx = canvas.getContext('2d', { alpha: false })!;
    this.resize();
    window.addEventListener('resize', () => this.resize());
    window.addEventListener('orientationchange', () => this.resize());
    window.visualViewport?.addEventListener('resize', () => this.resize());
    window.visualViewport?.addEventListener('scroll', () => this.resize());
  }

  /** Show only this rectangle of the framebuffer, scaled up to fit the page. */
  setView(r: Rect) {
    const v = this.view;
    if (v.x === r.x && v.y === r.y && v.w === r.w && v.h === r.h) return;
    this.view = { ...r };
    this.resize();
  }

  resize() {
    const dpr = Math.max(1, window.devicePixelRatio || 1);
    const cssW = this.view.w * CELL_W, cssH = this.view.h * CELL_H;
    const page = pageSize();
    // follow the visible area (a phone keyboard shrinks and scrolls it), so the canvas stays in sight above it
    const wrap = this.canvas.parentElement, vv = window.visualViewport;
    if (wrap && vv) {
      Object.assign(wrap.style, { top: vv.offsetTop + 'px', left: vv.offsetLeft + 'px', width: vv.width + 'px', height: vv.height + 'px', right: 'auto', bottom: 'auto' });
    }
    const scale = Math.min(page.w / cssW, page.h / cssH);
    this.dpr = dpr;
    this.canvas.width = Math.ceil(cssW * dpr);
    this.canvas.height = Math.ceil(cssH * dpr);
    this.canvas.style.width = Math.floor(cssW * scale) + 'px';
    this.canvas.style.height = Math.floor(cssH * scale) + 'px';
    this.lastCh.fill('');
    this.lastBg.fill(-1);
  }

  /** Map a mouse event to a cell coordinate. */
  cellAt(ev: MouseEvent): [number, number] {
    return viewCell(this.view, this.canvas.getBoundingClientRect(), ev.clientX, ev.clientY);
  }

  draw(fb: Fb) {
    const c = this.ctx, d = this.dpr;
    c.textBaseline = 'middle';
    c.textAlign = 'center';
    let lastFont = '';
    const v = this.view;
    for (let vy = 0; vy < v.h; vy++) for (let vx = 0; vx < v.w; vx++) {
      const fx = v.x + vx, fy = v.y + vy;
      if (fx < 0 || fy < 0 || fx >= W || fy >= H) continue;
      const i = fy * W + fx;
      const bg = (q(fb.bg[i * 3]) << 16) | (q(fb.bg[i * 3 + 1]) << 8) | q(fb.bg[i * 3 + 2]);
      const fg = (q(fb.fg[i * 3]) << 16) | (q(fb.fg[i * 3 + 1]) << 8) | q(fb.fg[i * 3 + 2]);
      const ch = fb.ch[i], b = fb.bold[i];
      if (bg === this.lastBg[i] && ch === this.lastCh[i] && (ch === ' ' || (fg === this.lastFg[i] && b === this.lastB[i]))) continue;
      this.lastBg[i] = bg; this.lastCh[i] = ch; this.lastFg[i] = fg; this.lastB[i] = b;
      const x = vx * CELL_W * d, y = vy * CELL_H * d;
      const cw = CELL_W * d, chh = CELL_H * d;
      const x0 = Math.floor(x), y0 = Math.floor(y), x1 = Math.floor(x + cw), y1 = Math.floor(y + chh);
      c.fillStyle = css(bg);
      c.fillRect(x0, y0, x1 - x0, y1 - y0);
      if (ch === ' ') continue;
      const fgc = css(fg);
      // block elements are drawn as rectangles so they tile without seams
      const hh = y1 - y0, ww = x1 - x0;
      if (ch === '▀') { c.fillStyle = fgc; c.fillRect(x0, y0, ww, Math.round(hh / 2)); continue; }
      if (ch === '▄') { c.fillStyle = fgc; c.fillRect(x0, y0 + Math.round(hh / 2), ww, hh - Math.round(hh / 2)); continue; }
      if (ch === '█') { c.fillStyle = fgc; c.fillRect(x0, y0, ww, hh); continue; }
      const sh = ch === '░' ? 0.25 : ch === '▒' ? 0.5 : ch === '▓' ? 0.75 : 0;
      if (sh) {
        const m = (a: number, bb: number) => Math.round(bb + (a - bb) * sh);
        c.fillStyle = css((m((fg >> 16) & 255, (bg >> 16) & 255) << 16) | (m((fg >> 8) & 255, (bg >> 8) & 255) << 8) | m(fg & 255, bg & 255));
        c.fillRect(x0, y0, ww, hh);
        continue;
      }
      const e = '▏▎▍▌▋▊▉'.indexOf(ch);
      if (e >= 0) { c.fillStyle = fgc; c.fillRect(x0, y0, Math.round((ww * (e + 1)) / 8), hh); continue; }
      const font = `${b ? 700 : 400} ${Math.round(15 * d)}px ${FONT}`;
      if (font !== lastFont) { c.font = font; lastFont = font; }
      c.fillStyle = fgc;
      c.fillText(ch, x + cw / 2, y + chh / 2 + d);
    }
  }
}
