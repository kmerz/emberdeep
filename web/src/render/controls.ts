// CONTROLS (from the game menu): every world action with its two keyboard keys and its pad button.
import { Fb, DIM, TX, WH, GOLD, PINK, MINT } from '../term/fb';
import { ACTIONS, binds, keyName, BTN_NAMES } from '../bindings';

export const CONTROLS_BOX = { x: 36, y: 4, w: 88, h: 31 };
const ROW_Y = 4;
const COL_X = [36, 50, 64]; // KEY, ALT KEY, PAD (from the box's left edge)
export const RESET_ROWS = 2; // "reset keyboard", "reset gamepad" after the actions
export const CONTROL_ROWS = ACTIONS.length + RESET_ROWS;

/** Can column c (0 key, 1 alt key, 2 pad) of row r be changed? */
export function cellEditable(r: number, c: number): boolean {
  const a = ACTIONS[r];
  if (!a) return r < CONTROL_ROWS && c === 0; // the reset rows have one "cell"
  return !a.fixed && !(c === 2 && a.noPad);
}

export interface Capture { kind: 'kb' | 'pad'; row: number; slot: number; until: number }

export function controlsPanel(fb: Fb, row: number, col: number, cap: Capture | null, now: number) {
  const b = CONTROLS_BOX;
  fb.box(b.x, b.y, b.w, b.h, 'CONTROLS', GOLD);
  fb.txt(b.x + 3, b.y + 2, 'ACTION', DIM, null, 1);
  ['KEY', 'ALT KEY', 'GAMEPAD'].forEach((h, i) => fb.txt(b.x + COL_X[i], b.y + 2, h, DIM, null, 1));
  ACTIONS.forEach((a, r) => {
    const y = b.y + ROW_Y + r, on = r === row;
    if (on) fb.fill(b.x + 1, y, b.w - 2, 1, [24, 21, 36]);
    fb.txt(b.x + 3, y, a.label, on ? WH : TX, null, on ? 1 : 0);
    const kb = binds.kb[a.id] ?? [], pad = binds.pad[a.id];
    const cells = [kb[0] ? keyName(kb[0]) : '·', kb[1] ? keyName(kb[1]) : '·', a.noPad ? '—' : pad !== null && pad !== undefined ? BTN_NAMES[pad] ?? '?' : '·'];
    cells.forEach((v, c) => {
      const sel = on && c === col, busy = sel && cap;
      const txt = busy ? (cap!.kind === 'kb' ? 'press a key' : 'press a button') : v;
      const fg = busy ? PINK : !cellEditable(r, c) ? DIM : sel ? WH : c === 2 ? MINT : GOLD;
      if (sel) fb.txt(b.x + COL_X[c] - 2, y, '▸', busy && Math.floor(now / 300) % 2 ? DIM : GOLD);
      fb.txt(b.x + COL_X[c], y, txt, fg, null, sel ? 1 : 0);
    });
  });
  ['Reset keyboard to the defaults', 'Reset gamepad to the defaults'].forEach((label, i) => {
    const r = ACTIONS.length + i, y = b.y + ROW_Y + r + 1, on = r === row;
    if (on) fb.fill(b.x + 1, y, b.w - 2, 1, [24, 21, 36]);
    fb.txt(b.x + 3, y, (on ? '▸ ' : '  ') + label, on ? WH : TX, null, on ? 1 : 0);
  });
  const hint = cap
    ? cap.kind === 'kb' ? 'Press the new key · ESC cancels · BACKSPACE clears' : 'Press the new button · START or ESC cancels · ' + Math.max(0, Math.ceil((cap.until - now) / 1000)) + 's'
    : '←/→ column · ENTER change · DEL / X clear · ESC / B back';
  fb.txt(b.x + 3, b.y + b.h - 2, hint, cap ? PINK : DIM);
}

/** The row and column under a screen cell, or null. */
export function controlsAt(cx: number, cy: number): { row: number; col: number } | null {
  const b = CONTROLS_BOX, d = cy - (b.y + ROW_Y);
  if (cx <= b.x || cx >= b.x + b.w - 1 || d < 0) return null;
  if (d < ACTIONS.length) {
    let col = 0;
    COL_X.forEach((x, i) => { if (cx >= b.x + x - 2) col = i; });
    return { row: d, col };
  }
  const r = d - 1; // a blank line before the reset rows
  return r >= ACTIONS.length && r < CONTROL_ROWS ? { row: r, col: 0 } : null;
}
