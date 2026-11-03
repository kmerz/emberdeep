// The game menu (Esc, or START on a pad): the things a pad can't type. The game keeps running behind it.
import { Fb, DIM, TX, WH, GOLD, PINK } from '../term/fb';
import { keyLabel } from '../gamepad';

export const MENU_BOX = { x: 58, y: 13, w: 44, h: 14 };
const ROW_Y = 3;

export type MenuId = 'resume' | 'journal' | 'sound' | 'save' | 'leave';
export interface MenuRow { id: MenuId; label: string; note?: string; off?: boolean }

const SOUND_NAME: Record<string, string> = { all: 'music and sound', sfx: 'sound only', off: 'off' };

/** The menu's rows. `confirm`: leave was picked once and waits for a second press. */
export function menuRows(host: boolean, sound: string, confirm: boolean): MenuRow[] {
  return [
    { id: 'resume', label: 'Resume' },
    { id: 'journal', label: 'Journal', note: '(' + keyLabel('world', 'j', 'J') + ')' },
    { id: 'sound', label: 'Sound: ' + (SOUND_NAME[sound] ?? sound) },
    host ? { id: 'save', label: 'Save and return to the lobby' } : { id: 'save', label: 'Save', note: 'the host saves', off: true },
    { id: 'leave', label: confirm ? 'Leave? Press again to confirm' : 'Leave the game' },
  ];
}

export function menuPanel(fb: Fb, rows: MenuRow[], sel: number) {
  const b = MENU_BOX;
  fb.box(b.x, b.y, b.w, b.h, 'MENU', GOLD);
  rows.forEach((r, i) => {
    const on = i === sel, y = b.y + ROW_Y + i * 2;
    fb.txt(b.x + 3, y, on ? '▸' : ' ', GOLD);
    const col = r.off ? DIM : r.id === 'leave' && r.label.startsWith('Leave?') ? PINK : on ? WH : TX;
    fb.txt(b.x + 5, y, r.label, col, null, on ? 1 : 0);
    if (r.note) fb.txt(b.x + 6 + r.label.length, y, r.note, DIM);
  });
  fb.txt(b.x + 3, b.y + b.h - 2, 'The game keeps running while this is open.', DIM);
}

/** The row under a screen cell, or -1. */
export function menuAt(cx: number, cy: number, n: number): number {
  const b = MENU_BOX, d = cy - (b.y + ROW_Y);
  if (cx <= b.x || cx >= b.x + b.w - 1 || d < 0 || d % 2) return -1;
  return d / 2 < n ? d / 2 : -1;
}
