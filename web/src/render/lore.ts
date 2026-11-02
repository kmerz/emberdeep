// The story page panel: what an inscription says, opened by walking into one.
import { Fb, DIM, WH } from '../term/fb';
import { wrap } from '../story';
import { keyLabel } from '../gamepad';

export const LORE_BOX = { x: 30, y: 9, w: 52, h: 14 };
const PARCHMENT: [number, number, number] = [226, 210, 170];

/** Page titles: the floor the inscription is on (0 = the gnome village). */
export function loreTitle(page: number): string {
  return page === 0 ? 'SPOONHOLLOW' : 'DEPTH ' + String(page).padStart(2, '0');
}

export function lorePanel(fb: Fb, page: number, text: string) {
  const b = LORE_BOX;
  fb.box(b.x, b.y, b.w, b.h, 'INSCRIPTION · ' + loreTitle(page), PARCHMENT);
  wrap(text, b.w - 6).slice(0, b.h - 4).forEach((l, i) => fb.txt(b.x + 3, b.y + 2 + i, l, PARCHMENT));
  const close = keyLabel('panel', 'Escape', 'Esc');
  fb.segs(b.x + 3, b.y + b.h - 2, [[close, WH, null, 1], [' close · kept in your journal (', DIM], [keyLabel('world', 'j', 'J'), WH, null, 1], [')', DIM]]);
}
