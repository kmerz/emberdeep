// On-screen controls for phones and tablets: a floating stick on the left and buttons on the right.
// Like the gamepad they only send synthetic keys to the current screen, so no game logic is duplicated.
import { MENU_REPEAT_MS, type PadContext, type PadSink } from './gamepad';

/** One on-screen button. `area` places it (see the #touch rules in index.html). */
export interface TouchBtn {
  key: string;
  label: string;
  sub?: string;
  /** greyed out (on cooldown, nothing on the key, ...) - it still works */
  off?: boolean;
  area: 'a1' | 'a2' | 'a3' | 'aq' | 'a4' | 'a5' | 'ae' | 'top' | 'bot' | 'side' | 'ok' | 'back';
  /** run on release instead of sending a key (things browsers only allow from a finished tap, e.g. fullscreen) */
  tap?: () => void;
}

export const STICK_DEAD = 16;   // px the thumb must move before the stick counts
export const STICK_RADIUS = 52; // px: how far the knob follows the thumb

const R = 'ArrowRight', D = 'ArrowDown', L = 'ArrowLeft', U = 'ArrowUp';
const EIGHT = [[R], [R, D], [D], [D, L], [L], [L, U], [U], [U, R]];

/** Arrow keys for a stick offset: 8 directions in 45° sectors, or only 4 (menus). */
export function stickKeys(dx: number, dy: number, four = false, dead = STICK_DEAD): string[] {
  if (Math.hypot(dx, dy) < dead) return [];
  const a = Math.atan2(dy, dx);
  if (four) return [[R, D, L, U][((Math.round(a / (Math.PI / 2)) % 4) + 4) % 4]];
  return EIGHT[((Math.round(a / (Math.PI / 4)) % 8) + 8) % 8];
}

export class TouchControls {
  private root: HTMLDivElement;
  private zone: HTMLDivElement;
  private base: HTMLDivElement;
  private knob: HTMLDivElement;
  private boxes: Record<'top' | 'bot' | 'side' | 'abs', HTMLDivElement>;
  private sig = '';
  private els: HTMLElement[] = [];
  private btns: TouchBtn[] = [];
  private ctx: PadContext | '' = '';
  /** pointer id → the key it holds down */
  private pressed = new Map<number, string>();
  private stickId = -1;
  private ox = 0;
  private oy = 0;
  private dirs = new Set<string>();
  private lastRepeat = new Map<string, number>();

  constructor(private sink: PadSink) {
    const div = (cls: string, parent: HTMLElement) => { const d = document.createElement('div'); d.className = cls; parent.appendChild(d); return d; };
    this.root = div('', document.body);
    this.root.id = 'touch';
    this.zone = div('t-zone', this.root);
    this.base = div('t-base', this.root);
    this.knob = div('t-knob', this.base);
    this.boxes = { top: div('t-top', this.root), bot: div('t-bot', this.root), side: div('t-side', this.root), abs: div('t-abs', this.root) };
    this.zone.addEventListener('pointerdown', (e) => this.stickDown(e));
    this.zone.addEventListener('pointermove', (e) => this.stickMove(e));
    for (const t of ['pointerup', 'pointercancel']) this.zone.addEventListener(t, (e) => this.stickUp(e as PointerEvent));
  }

  /** Show or hide everything (hiding lets go of whatever is held). */
  show(on: boolean) {
    if (!on && this.root.style.display !== 'none') this.releaseAll();
    this.root.style.display = on ? '' : 'none';
  }

  /** Once per frame: the buttons for what is on screen now, and whether the stick is active. */
  update(now: number, ctx: PadContext, btns: TouchBtn[], stick: boolean) {
    if (ctx !== this.ctx) { this.releaseAll(); this.ctx = ctx; }
    this.zone.style.display = stick ? '' : 'none';
    if (!stick && this.stickId >= 0) this.stickRelease();
    const sig = btns.map((b) => b.area + ':' + b.key).join(',');
    if (sig !== this.sig) this.build(btns, sig);
    btns.forEach((b, i) => {
      const el = this.els[i];
      const lab = el.firstChild as HTMLElement, sub = el.lastChild as HTMLElement;
      if (lab.textContent !== b.label) lab.textContent = b.label;
      const s = b.sub ?? '';
      if (sub.textContent !== s) sub.textContent = s;
      el.classList.toggle('off', !!b.off);
    });
    this.btns = btns;
    // held directions: movement re-asserts them every frame (panels and chat clear the held set), menus step on a timer
    for (const k of this.dirs) {
      if (ctx === 'world') this.sink.down(k, true);
      else if (now - (this.lastRepeat.get(k) ?? 0) >= MENU_REPEAT_MS * 2) { this.sink.down(k, false); this.lastRepeat.set(k, now); }
    }
  }

  private build(btns: TouchBtn[], sig: string) {
    this.releaseAll();
    for (const el of this.els) el.remove();
    this.els = btns.map((b, i) => {
      const el = document.createElement('div');
      el.className = 't-btn ' + b.area;
      el.innerHTML = '<b></b><i></i>';
      const box = b.area === 'top' || b.area === 'bot' || b.area === 'side' ? this.boxes[b.area] : this.boxes.abs;
      box.appendChild(el);
      el.addEventListener('pointerdown', (e) => {
        e.preventDefault();
        const cur = this.btns[i];
        if (!cur) return;
        el.setPointerCapture?.(e.pointerId);
        el.classList.add('on');
        try { navigator.vibrate?.(8); } catch { /* not allowed here */ }
        if (cur.tap) { this.pressed.set(e.pointerId, ''); return; }
        this.pressed.set(e.pointerId, cur.key);
        this.sink.down(cur.key, false);
      });
      const up = (e: PointerEvent) => {
        el.classList.remove('on');
        if (!this.pressed.has(e.pointerId)) return;
        const k = this.pressed.get(e.pointerId)!;
        this.pressed.delete(e.pointerId);
        if (k) this.sink.up(k);
        else if (e.type === 'pointerup') this.btns[i]?.tap?.();
      };
      el.addEventListener('pointerup', up);
      el.addEventListener('pointercancel', up);
      return el;
    });
    this.sig = sig;
  }

  private releaseAll() {
    for (const k of this.pressed.values()) if (k) this.sink.up(k);
    this.pressed.clear();
    for (const el of this.els) el.classList.remove('on');
    this.stickRelease();
  }

  private setDirs(want: string[], now: number) {
    for (const k of [...this.dirs]) if (!want.includes(k)) { this.dirs.delete(k); this.lastRepeat.delete(k); this.sink.up(k); }
    for (const k of want) if (!this.dirs.has(k)) { this.dirs.add(k); this.lastRepeat.set(k, now); this.sink.down(k, false); }
  }

  private stickDown(e: PointerEvent) {
    e.preventDefault();
    if (this.stickId >= 0) return;
    this.stickId = e.pointerId;
    this.zone.setPointerCapture?.(e.pointerId);
    this.ox = e.clientX; this.oy = e.clientY;
    this.base.style.left = e.clientX + 'px';
    this.base.style.top = e.clientY + 'px';
    this.base.style.display = 'block';
    this.knob.style.transform = '';
  }

  private stickMove(e: PointerEvent) {
    if (e.pointerId !== this.stickId) return;
    let dx = e.clientX - this.ox, dy = e.clientY - this.oy;
    const d = Math.hypot(dx, dy);
    if (d > STICK_RADIUS) { // drag the base along so turning around is quick
      this.ox += (dx / d) * (d - STICK_RADIUS); this.oy += (dy / d) * (d - STICK_RADIUS);
      this.base.style.left = this.ox + 'px'; this.base.style.top = this.oy + 'px';
      dx = e.clientX - this.ox; dy = e.clientY - this.oy;
    }
    this.knob.style.transform = `translate(${dx}px, ${dy}px)`;
    this.setDirs(stickKeys(dx, dy, this.ctx !== 'world'), performance.now());
  }

  private stickUp(e: PointerEvent) {
    if (e.pointerId === this.stickId) this.stickRelease();
  }

  private stickRelease() {
    this.stickId = -1;
    this.base.style.display = 'none';
    this.setDirs([], 0);
  }
}
