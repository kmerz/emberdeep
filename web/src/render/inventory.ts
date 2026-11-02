// The rucksack panel (I): a 4×3 grid of slots over the map, with the selected item described below.
import { Fb, DIM, TX, WH, GOLD, mul } from '../term/fb';
import type { GameState } from '../state';
import { itemInfo, packCap } from '../items';

export const INV = { x: 36, y: 8, w: 40, h: 14, cols: 4 };
const SLOT_W = 9;

/** Screen cell → slot index, or -1. */
export function slotAt(cx: number, cy: number, cap: number): number {
  if (cx < INV.x + 2) return -1;
  const col = Math.floor((cx - (INV.x + 2)) / SLOT_W), row = cy - (INV.y + 2);
  if (col >= INV.cols || row < 0 || row >= cap / INV.cols) return -1;
  return row * INV.cols + col;
}

export function inventoryPanel(fb: Fb, st: GameState, sel: number) {
  const pack = st.me?.pack ?? [];
  const cap = packCap(st.me), rows = Math.ceil(cap / INV.cols);
  const used = pack.filter((s) => s.n > 0).length;
  fb.box(INV.x, INV.y, INV.w, INV.h, 'RUCKSACK · ' + used + '/' + cap, GOLD);
  for (let i = 0; i < cap; i++) {
    const s = pack[i];
    const x = INV.x + 2 + (i % INV.cols) * SLOT_W, y = INV.y + 2 + Math.floor(i / INV.cols);
    const on = i === sel;
    const br = on ? GOLD : [80, 76, 108];
    fb.put(x, y, '[', br, null, on ? 1 : 0);
    fb.put(x + 6, y, ']', br, null, on ? 1 : 0);
    if (s && s.n > 0) {
      const it = itemInfo(s.item);
      fb.put(x + 1, y, it.ch, it.col, null, 1);
      fb.txt(x + 2, y, ('×' + s.n).padEnd(4), on ? WH : TX);
    } else fb.txt(x + 3, y, '·', [70, 66, 96]);
  }
  const s = pack[sel];
  const y = INV.y + 3 + rows;
  if (s && s.n > 0) {
    const it = itemInfo(s.item);
    fb.segs(INV.x + 2, y, [['▸ ', GOLD], [it.name, it.col, null, 1], [' ×' + s.n, TX]]);
    fb.txt(INV.x + 4, y + 1, it.desc, DIM);
    if (it.thrown) fb.txt(INV.x + 4, y + 2, 'aims at the mouse cursor', mul(DIM, 0.8));
  } else fb.txt(INV.x + 2, y, 'empty slot', DIM);
  const cd = st.me?.potCd ?? 0;
  if (cd > 0) fb.txt(INV.x + 2, y + 4, 'potions and draughts ready in ' + (cd / 20).toFixed(1) + 's', DIM);
  fb.segs(INV.x + 2, INV.y + INV.h - 2, [['U', WH, null, 1], [' use · ', DIM], ['X', WH, null, 1], [' drop · ', DIM], ['arrows', WH, null, 1], [' pick · ', DIM], ['I', WH, null, 1], [' close', DIM]]);
}
