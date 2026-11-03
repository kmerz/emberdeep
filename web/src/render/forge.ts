// Bramwick's forge in the village: temper (one rarity up, one more bonus) or reforge (new bonuses) worn gear.
import { Fb, Seg, DIM, TX, WH, GOLD, PINK } from '../term/fb';
import type { GearView } from '../proto';
import type { GameState } from '../state';
import { rarityCol, gearCh, modText, temperPrice, reforgePrice, RARITY } from '../items';

export const FORGE_BOX = { x: 34, y: 5, w: 52, h: 21 };
const ROW_Y = 11; // the first work row, from the box top

export interface ForgeRow {
  id: string;     // what to send with 'buy': temper0, temper1, reforge0, reforge1
  label: string;
  gear?: GearView;
  price: number;
  why: string;    // why it can't be done ('' = it can)
}

/** The four jobs, in order: temper weapon, temper trinket, reforge weapon, reforge trinket. */
export function forgeRows(st: GameState): ForgeRow[] {
  const me = st.me, rows: ForgeRow[] = [];
  for (const kind of ['temper', 'reforge'] as const) {
    (['weapon', 'trinket'] as const).forEach((slot, i) => {
      const g = me?.[slot];
      const row: ForgeRow = { id: kind + i, label: (kind === 'temper' ? 'Temper ' : 'Reforge ') + slot, gear: g, price: 0, why: '' };
      if (!g) row.why = 'nothing worn';
      else if (g.rar >= 3) row.why = "he won't touch a unique";
      else if (kind === 'temper' && g.rar >= 2) row.why = 'already rare';
      else if (kind === 'reforge' && g.rar === 0) row.why = 'no bonuses to reforge';
      else row.price = kind === 'temper' ? temperPrice(g) : reforgePrice(g);
      rows.push(row);
    });
  }
  return rows;
}

export function forgePanel(fb: Fb, st: GameState, sel: number) {
  const b = FORGE_BOX, gold = st.me?.gold ?? 0;
  fb.box(b.x, b.y, b.w, b.h, "BRAMWICK'S FORGE · you have " + gold + ' gold', GOLD);
  let y = b.y + 2;
  for (const slot of ['weapon', 'trinket'] as const) {
    const g = st.me?.[slot];
    fb.txt(b.x + 2, y, slot.toUpperCase(), DIM, null, 1);
    if (g) {
      fb.put(b.x + 11, y, gearCh(g, slot), rarityCol(g.rar), null, 1);
      fb.segs(b.x + 13, y, [[g.name.slice(0, 30), rarityCol(g.rar), null, 1], [' · ' + (RARITY[g.rar] ?? '') + ' · lvl ' + g.lvl, DIM]]);
      fb.txt(b.x + 13, y + 1, g.mods.map(modText).join(' · ').slice(0, b.w - 15), TX);
    } else fb.txt(b.x + 11, y, 'nothing worn', DIM);
    y += 3;
  }
  fb.txt(b.x + 2, b.y + ROW_Y - 1, 'WORK', DIM, null, 1);
  const rows = forgeRows(st);
  rows.forEach((r, i) => {
    const on = i === sel, ry = b.y + ROW_Y + i;
    fb.txt(b.x + 2, ry, on ? '▸' : ' ', GOLD);
    const can = !r.why && gold >= r.price;
    fb.txt(b.x + 4, ry, r.label, r.why ? DIM : on ? WH : TX, null, on ? 1 : 0);
    if (r.why) fb.txt(b.x + 22, ry, r.why, DIM);
    else fb.txt(b.x + 22, ry, String(r.price).padStart(4) + ' gold', can ? GOLD : PINK);
  });
  const r = rows[sel];
  const help = !r ? '' : r.id.startsWith('temper')
    ? 'One rarity up (common → magic → rare): keeps what it has, adds a bonus.'
    : 'Rolls its bonuses again. They may come out better, or worse.';
  fb.txt(b.x + 2, b.y + ROW_Y + 5, help.slice(0, b.w - 4), DIM);
  const keys: Seg[] = [['ENTER', WH, null, 1], [' do it · ', DIM], ['Esc', WH, null, 1], [' close', DIM]];
  fb.segs(b.x + 2, b.y + b.h - 2, keys);
}

/** The work row under a screen cell, or -1. */
export function forgeAt(cx: number, cy: number): number {
  const b = FORGE_BOX, i = cy - (b.y + ROW_Y);
  return cx > b.x && cx < b.x + b.w - 1 && i >= 0 && i < 4 ? i : -1;
}
