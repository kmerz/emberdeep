/**
 * Gamepad support: polls the Gamepad API once per frame and turns pad input
 * into synthetic key events for the current Screen, so every screen and panel
 * works with a controller without duplicating any game logic.
 *
 * Standard-mapping pads only (Xbox / PlayStation / Switch Pro in browsers).
 * No button opens chat: a pad can't type, so A only acts as Enter in panels,
 * menus and the wipe overlay (see GameScreen.padContext). The world buttons
 * come from the player's bindings (bindings.ts); the right stick aims (padAim).
 */
import { BTN, BTN_NAMES, padWorldMap, kbLabel } from './bindings';

export type PadContext = 'world' | 'panel' | 'overlay' | 'menu' | 'lobby' | 'chat';

export interface PadSink {
  down(key: string, repeat: boolean): void;
  up(key: string): void;
}

/** The minimal slice of the Gamepad API we use (lets tests pass a fake). */
export interface PadLike {
  connected: boolean;
  buttons: ReadonlyArray<{ pressed: boolean }>;
  axes: ReadonlyArray<number>;
}

const { A, B, X, Y, LB, RB, START, SELECT } = BTN;
const DPAD: Array<[number, string]> = [[12, 'ArrowUp'], [13, 'ArrowDown'], [14, 'ArrowLeft'], [15, 'ArrowRight']];

type BtnMap = Partial<Record<number, string>>;

const MAPS: Record<Exclude<PadContext, 'world'>, BtnMap> = {
  chat: {},
  // LB / RB flip tabs (rucksack · skill tree · journal, or a shop's sides); Y puts the selection on a key
  panel: { [A]: 'Enter', [B]: 'Escape', [X]: 'x', [Y]: 'y', [LB]: 'PrevTab', [RB]: 'Tab', [START]: 'Escape', [SELECT]: 'Escape' },
  overlay: { [A]: 'Enter', [START]: 'Escape' },
  menu: { [A]: 'Enter', [B]: 'Escape' },
  // A = ready/start, LT = change class, host: X friendly fire, Y difficulty, RB add bot, LB remove bot,
  // B: new run (press twice; only with a save). 'q' would leave the lobby, so no button sends it
  lobby: { [A]: 'Enter', [BTN.LT]: 'c', [X]: 'f', [Y]: 'd', [RB]: 'b', [LB]: 'x', [B]: 'n' },
};

/** The buttons of a context (the world's come from the bindings). */
export function padMap(ctx: PadContext): BtnMap { return ctx === 'world' ? padWorldMap() : MAPS[ctx]; }

/** Which input the player used last. While it is the pad, on-screen hints name pad buttons instead of keys. */
export const lastInput = { pad: false };

/** The pad button that sends this key in this context ('' if none; the lowest button index wins). */
export function padLabel(ctx: PadContext, key: string): string {
  for (const [b, k] of Object.entries(padMap(ctx))) if (k === key) return BTN_NAMES[Number(b)] ?? '';
  return '';
}

/** The label for a key in a hint: its pad button while playing with a pad, else `shown` (default: the key). */
export function keyLabel(ctx: PadContext, key: string, shown?: string): string {
  if (lastInput.pad) return padLabel(ctx, key) || shown || key.toUpperCase();
  return shown ?? ((ctx === 'world' && kbLabel(key)) || key.toUpperCase());
}

/** The right stick: where the player aims (screen direction, y down), while it is pushed. */
export const padAim = { on: false, x: 0, y: 0 };
export const AIM_DEADZONE = 0.4;

export const DEADZONE = 0.35;
export const MENU_REPEAT_MS = 150;

const DIR_KEYS = ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'];

export class GamepadInput {
  private ctx: PadContext = 'menu';
  /** key emitted for each currently pressed button */
  private btnKey = new Map<number, string>();
  /** buttons held when the context changed: ignored until released */
  private latched = new Set<number>();
  private dirs = new Set<string>();
  private lastRepeat = new Map<string, number>();
  /** Set while rebinding: the next button pressed goes here instead of to the screen. */
  private capture: ((btn: number) => void) | null = null;
  private latchNow = false;

  /** Rebinding: the next button pressed (not one already held) goes to cb instead of to the screen. */
  startCapture(cb: (btn: number) => void) { this.capture = cb; this.latchNow = true; }
  stopCapture() { this.capture = null; }
  get capturing(): boolean { return this.capture !== null; }

  constructor(private getPads: () => ArrayLike<PadLike | null> = () => (navigator.getGamepads ? navigator.getGamepads() : [])) {}

  get active(): boolean { return this.btnKey.size > 0 || this.dirs.size > 0; }

  /** Release everything currently held (screen change, pad unplugged). */
  releaseAll(sink: PadSink) {
    for (const k of this.btnKey.values()) sink.up(k);
    for (const k of this.dirs) sink.up(k);
    this.btnKey.clear();
    this.dirs.clear();
    this.lastRepeat.clear();
  }

  poll(now: number, ctx: PadContext, sink: PadSink) {
    const pad = this.firstPad();
    if (!pad) { this.releaseAll(sink); this.latched.clear(); this.ctx = ctx; padAim.on = false; return; }

    if (ctx !== this.ctx) {
      this.releaseAll(sink);
      this.latched.clear();
      pad.buttons.forEach((b, i) => { if (b.pressed) this.latched.add(i); });
      this.ctx = ctx;
    }
    // ---- right stick: aim ----
    const rx = pad.axes[2] ?? 0, ry = pad.axes[3] ?? 0;
    padAim.on = Math.hypot(rx, ry) > AIM_DEADZONE;
    if (padAim.on) { padAim.x = rx; padAim.y = ry; }

    if (this.latchNow) { pad.buttons.forEach((b, i) => { if (b.pressed) this.latched.add(i); }); this.latchNow = false; }
    if (this.capture) { // rebinding: report the first new press, nothing else
      pad.buttons.forEach((b, i) => {
        if (i >= 12 && i <= 15) return;
        if (!b.pressed) { this.latched.delete(i); return; }
        if (this.latched.has(i) || !this.capture) return;
        this.latched.add(i);
        const cb = this.capture;
        this.capture = null;
        cb(i);
      });
      return;
    }
    if (ctx === 'chat') return;

    // ---- directions: stick + d-pad ----
    const want = new Set<string>();
    const ax = pad.axes[0] ?? 0, ay = pad.axes[1] ?? 0;
    if (ay < -DEADZONE) want.add('ArrowUp');
    if (ay > DEADZONE) want.add('ArrowDown');
    if (ax < -DEADZONE) want.add('ArrowLeft');
    if (ax > DEADZONE) want.add('ArrowRight');
    for (const [i, k] of DPAD) if (pad.buttons[i]?.pressed) want.add(k);

    for (const k of DIR_KEYS) {
      const held = this.dirs.has(k);
      if (want.has(k)) {
        if (ctx === 'world') {
          // movement only reads the held set; (re)asserting each poll survives panels clearing it
          sink.down(k, held);
        } else if (!held) {
          sink.down(k, false);
          this.lastRepeat.set(k, now);
        } else if (now - (this.lastRepeat.get(k) ?? 0) >= MENU_REPEAT_MS) {
          sink.down(k, false); // menus have no auto-repeat: step selection again
          this.lastRepeat.set(k, now);
        }
        this.dirs.add(k);
      } else if (held) {
        sink.up(k);
        this.dirs.delete(k);
        this.lastRepeat.delete(k);
      }
    }

    // ---- buttons: edge-triggered ----
    const map = padMap(ctx);
    pad.buttons.forEach((b, i) => {
      if (i >= 12 && i <= 15) return; // d-pad handled above
      const was = this.btnKey.get(i);
      if (b.pressed) {
        if (was !== undefined || this.latched.has(i)) return;
        const key = map[i];
        if (!key) return;
        this.btnKey.set(i, key);
        sink.down(key, false);
      } else {
        this.latched.delete(i);
        if (was !== undefined) { this.btnKey.delete(i); sink.up(was); }
      }
    });
  }

  private firstPad(): PadLike | null {
    const pads = this.getPads();
    for (let i = 0; i < pads.length; i++) {
      const p = pads[i];
      if (p && p.connected) return p;
    }
    return null;
  }
}

/** A KeyboardEvent stand-in carrying just the fields the screens read. */
export function fakeKey(key: string, repeat = false): KeyboardEvent {
  return { key, repeat, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, preventDefault() {} } as unknown as KeyboardEvent;
}
