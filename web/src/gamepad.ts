/**
 * Gamepad support: polls the Gamepad API once per frame and turns pad input
 * into synthetic key events for the current Screen, so every screen and panel
 * works with a controller without duplicating any game logic.
 *
 * Standard-mapping pads only (Xbox / PlayStation / Switch Pro in browsers).
 * No button opens chat: a pad can't type, so A only acts as Enter in panels,
 * menus and the wipe overlay (see GameScreen.padContext).
 */

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

// standard-mapping button indices
const A = 0, B = 1, X = 2, Y = 3, LB = 4, RB = 5, LT = 6, RT = 7, SELECT = 8, START = 9, R3 = 11;
const DPAD: Array<[number, string]> = [[12, 'ArrowUp'], [13, 'ArrowDown'], [14, 'ArrowLeft'], [15, 'ArrowRight']];

type BtnMap = Partial<Record<number, string>>;

const COMMON: BtnMap = { [SELECT]: 'i', [START]: 'j', [R3]: 't' };

const MAPS: Record<PadContext, BtnMap> = {
  chat: {},
  // A is item key 5, never Enter: Enter in the world would open chat
  world: { ...COMMON, [X]: '1', [Y]: '2', [RB]: '3', [RT]: '1', [LB]: 'q', [LT]: 'e', [B]: '4', [A]: '5' },
  panel: { ...COMMON, [A]: 'Enter', [B]: 'Escape', [X]: 'x', [Y]: 'r', [RB]: 'Tab', [LB]: '1', [LT]: '2', [RT]: '3' },
  overlay: { [A]: 'Enter' },
  menu: { [A]: 'Enter', [B]: 'Escape' },
  // A = ready/start, LT = change class, host: X friendly fire, Y difficulty, RB add bot, LB remove bot,
  // B: new run (press twice; only with a save). 'q' would leave the lobby, so no button sends it
  lobby: { [A]: 'Enter', [LT]: 'c', [X]: 'f', [Y]: 'd', [RB]: 'b', [LB]: 'x', [B]: 'n' },
};

const BTN_NAMES: Record<number, string> = { [A]: 'A', [B]: 'B', [X]: 'X', [Y]: 'Y', [LB]: 'LB', [RB]: 'RB', [LT]: 'LT', [RT]: 'RT', [SELECT]: 'SEL', [START]: 'START', [R3]: 'R3' };

/** Which input the player used last. While it is the pad, on-screen hints name pad buttons instead of keys. */
export const lastInput = { pad: false };

/** The pad button that sends this key in this context ('' if none; the lowest button index wins). */
export function padLabel(ctx: PadContext, key: string): string {
  for (const [b, k] of Object.entries(MAPS[ctx])) if (k === key) return BTN_NAMES[Number(b)] ?? '';
  return '';
}

/** The label for a key in a hint: its pad button while playing with a pad, else `shown` (default: the key). */
export function keyLabel(ctx: PadContext, key: string, shown = key.toUpperCase()): string {
  return lastInput.pad ? padLabel(ctx, key) || shown : shown;
}

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
    if (!pad) { this.releaseAll(sink); this.latched.clear(); this.ctx = ctx; return; }

    if (ctx !== this.ctx) {
      this.releaseAll(sink);
      this.latched.clear();
      pad.buttons.forEach((b, i) => { if (b.pressed) this.latched.add(i); });
      this.ctx = ctx;
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
    const map = MAPS[ctx];
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
