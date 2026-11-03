// The rucksack panel (I): the worn weapon and trinket on top, a 4×3 grid of slots below, and the selected
// item described at the bottom. Selection: 0.. = a rucksack slot, WORN_WEAPON / WORN_TRINKET = what is worn.
import { Fb, Seg, RGB, DIM, TX, WH, GOLD, mul } from '../term/fb';
import type { GameState } from '../state';
import type { GearView } from '../proto';
import { itemInfo, packCap, rarityCol, gearCh, modText, gearTotals, RARITY } from '../items';
import { wrap } from '../story';
import { keyLabel, lastInput } from '../gamepad';

export const INV = { x: 34, y: 5, w: 46, h: 25, cols: 4 };
export const WORN_WEAPON = -1;
export const WORN_TRINKET = -2;
const SLOT_W = 9;
const WORN_Y = INV.y + 2; // weapon row; the trinket row is below it
const GRID_Y = INV.y + 6;
const UP: RGB = [126, 240, 178], DOWN: RGB = [255, 110, 140];

/** Screen cell → rucksack slot index, or -1. */
export function slotAt(cx: number, cy: number, cap: number): number {
  if (cx < INV.x + 2) return -1;
  const col = Math.floor((cx - (INV.x + 2)) / SLOT_W), row = cy - GRID_Y;
  if (col >= INV.cols || row < 0 || row >= cap / INV.cols) return -1;
  return row * INV.cols + col;
}

/** Screen cell → WORN_WEAPON / WORN_TRINKET, or 0 for neither. */
export function wornAt(cx: number, cy: number): number {
  if (cx < INV.x + 2 || cx >= INV.x + INV.w - 2) return 0;
  return cy === WORN_Y ? WORN_WEAPON : cy === WORN_Y + 1 ? WORN_TRINKET : 0;
}

/** The selected gear: a worn piece, or gear in a rucksack slot. */
function selectedGear(st: GameState, sel: number): GearView | undefined {
  if (sel === WORN_WEAPON) return st.me?.weapon;
  if (sel === WORN_TRINKET) return st.me?.trinket;
  return st.me?.pack?.[sel]?.g;
}

export function inventoryPanel(fb: Fb, st: GameState, sel: number, itemKeys: string[] = []) {
  const me = st.me, pack = me?.pack ?? [];
  const cap = packCap(me), rows = Math.ceil(cap / INV.cols);
  const used = pack.filter((s) => s.n > 0).length;
  fb.box(INV.x, INV.y, INV.w, INV.h, 'RUCKSACK · ' + used + '/' + cap, GOLD);

  // worn gear
  fb.txt(INV.x + 2, INV.y + 1, 'WORN', DIM, null, 1);
  (['weapon', 'trinket'] as const).forEach((slot, i) => {
    const g = me?.[slot], on = sel === (i === 0 ? WORN_WEAPON : WORN_TRINKET), y = WORN_Y + i;
    fb.txt(INV.x + 2, y, on ? '▸' : ' ', GOLD);
    fb.txt(INV.x + 4, y, slot === 'weapon' ? 'weapon ' : 'trinket', on ? WH : DIM);
    if (g) {
      fb.put(INV.x + 12, y, gearCh(g, slot), rarityCol(g.rar), null, 1);
      fb.txt(INV.x + 14, y, g.name.slice(0, INV.w - 16), rarityCol(g.rar), null, on ? 1 : 0);
    } else fb.txt(INV.x + 12, y, '· nothing', mul(DIM, 0.8));
  });
  const tot = gearTotals(me?.weapon, me?.trinket).map(modText).join(' · ');
  fb.txt(INV.x + 4, WORN_Y + 2, (tot || 'no gear bonuses yet').slice(0, INV.w - 6), mul(DIM, 0.9));

  // the rucksack grid
  for (let i = 0; i < cap; i++) {
    const s = pack[i];
    const x = INV.x + 2 + (i % INV.cols) * SLOT_W, y = GRID_Y + Math.floor(i / INV.cols);
    const on = i === sel;
    const br: RGB = on ? GOLD : [80, 76, 108];
    fb.put(x, y, '[', br, null, on ? 1 : 0);
    fb.put(x + 6, y, ']', br, null, on ? 1 : 0);
    if (s && s.n > 0) {
      if (s.g) {
        fb.put(x + 1, y, gearCh(s.g, s.item), rarityCol(s.g.rar), null, 1);
        fb.txt(x + 2, y, ('L' + s.g.lvl).padEnd(4), on ? WH : TX);
        continue;
      }
      const it = itemInfo(s.item);
      fb.put(x + 1, y, it.ch, it.col, null, 1);
      fb.txt(x + 2, y, ('×' + s.n).padEnd(4), on ? WH : TX);
      const bound = itemKeys.indexOf(s.item); // this item is on item key 4 or 5
      if (bound >= 0) fb.put(x + 5, y, keyLabel('world', String(4 + bound))[0], GOLD, null, 1);
    } else fb.txt(x + 3, y, '·', [70, 66, 96]);
  }

  // the selection, described
  let y = GRID_Y + rows + 1;
  const g = selectedGear(st, sel), s = sel >= 0 ? pack[sel] : undefined;
  const k = (key: string, shown?: string) => keyLabel('panel', key, shown);
  if (g) {
    fb.segs(INV.x + 2, y++, [['▸ ', GOLD], [g.name, rarityCol(g.rar), null, 1]]);
    fb.txt(INV.x + 4, y++, RARITY[g.rar] + ' ' + g.slot + ' · level ' + g.lvl + ' · Grimble pays ' + g.value, DIM);
    for (const m of g.mods) fb.txt(INV.x + 4, y++, modText(m), TX);
    if (sel >= 0) {
      const worn = me?.[g.slot];
      y = compare(fb, y, g, worn);
      fb.segs(INV.x + 4, y++, [[k('Enter', 'U'), WH, null, 1], [worn ? ': wear it (swaps with what you wear)' : ': wear it', DIM]]);
    } else fb.segs(INV.x + 4, y++, [[k('Enter', 'U'), WH, null, 1], [': take it off', DIM]]);
  } else if (sel < 0) {
    fb.txt(INV.x + 2, y, 'nothing worn here: gear drops from monsters and chests', DIM);
  } else if (s && s.n > 0) {
    const it = itemInfo(s.item);
    fb.segs(INV.x + 2, y, [['▸ ', GOLD], [it.name, it.col, null, 1], [' ×' + s.n, TX]]);
    let ly = y + 1;
    for (const l of wrap(it.desc, INV.w - 6).slice(0, 2)) fb.txt(INV.x + 4, ly++, l, DIM);
    if (it.thrown) fb.txt(INV.x + 4, ly++, lastInput.pad ? 'aims at the nearest enemy' : 'aims at the mouse cursor', mul(DIM, 0.8));
    if (it.usable) {
      const on = itemKeys.indexOf(s.item), k4 = keyLabel('world', '4'), k5 = keyLabel('world', '5');
      if (on >= 0) fb.segs(INV.x + 4, ly++, [['on key ', DIM], [on ? k5 : k4, GOLD, null, 1]]);
      else if (lastInput.pad) fb.segs(INV.x + 4, ly++, [[keyLabel('panel', 'r'), WH, null, 1], [': put on ' + k4 + ' (again: ' + k5 + ')', DIM]]);
      else fb.segs(INV.x + 4, ly++, [['4', WH, null, 1], [' / ', DIM], ['5', WH, null, 1], [' / right-click: put on a key', DIM]]);
    }
  } else fb.txt(INV.x + 2, y, 'empty slot', DIM);

  const cd = me?.potCd ?? 0;
  if (cd > 0) fb.txt(INV.x + 2, INV.y + INV.h - 3, 'potions and draughts ready in ' + (cd / 20).toFixed(1) + 's', DIM);
  fb.segs(INV.x + 2, INV.y + INV.h - 2, [[k('Enter', 'U'), WH, null, 1], [' use/wear · ', DIM], [k('x'), WH, null, 1], [' drop · ', DIM],
    [lastInput.pad ? 'D-pad' : 'arrows', WH, null, 1], [' pick · ', DIM], [k('Escape', 'I'), WH, null, 1], [' close', DIM]]);
}

/** One or two lines: how wearing g instead of worn changes the hero's stats. Returns the next free row. */
function compare(fb: Fb, y: number, g: GearView, worn: GearView | undefined): number {
  if (!worn) {
    fb.txt(INV.x + 4, y++, 'you wear no ' + g.slot + ': all of it is a gain', UP);
    return y;
  }
  const d: Record<string, number> = {};
  for (const m of g.mods) d[m.k] = (d[m.k] ?? 0) + m.v;
  for (const m of worn.mods) d[m.k] = (d[m.k] ?? 0) - m.v;
  const segs: Seg[] = [['vs worn: ', DIM]];
  let any = false;
  for (const [key, v] of Object.entries(d)) {
    if (!v) continue;
    if (any) segs.push([' · ', DIM]);
    segs.push([modText({ k: key, v }), v > 0 ? UP : DOWN]);
    any = true;
  }
  if (!any) segs.push(['the same', DIM]);
  // wrap by hand: segments onto lines of the panel's width
  let line: Seg[] = [], w = 0;
  const flush = () => { if (line.length) fb.segs(INV.x + 4, y++, line); line = []; w = 0; };
  for (const sg of segs) {
    if (w + sg[0].length > INV.w - 6 && w > 0) flush();
    line.push(sg);
    w += sg[0].length;
  }
  flush();
  return y;
}
