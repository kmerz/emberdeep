// Grimble's shop: buy on the left, sell from your rucksack on the right. Opens when you walk into him.
import { Fb, Seg, DIM, TX, WH, GOLD, PINK } from '../term/fb';
import type { GearView } from '../proto';
import type { GameState } from '../state';
import { SHOP, itemInfo, sellPrice, rarityCol, gearCh, modText } from '../items';
import { gearDiff } from './inventory';

export const SHOP_BOX = { x: 34, y: 5, w: 46, h: 24 };

/** The buy side: Grimble's goods, then his gear for you (sold pieces stay as empty rows). */
export function buyCount(st: GameState): number { return SHOP.length + (st.me?.wares?.length ?? 0); }
export function wareAt(st: GameState, sel: number): GearView | null | undefined { return st.me?.wares?.[sel - SHOP.length]; }

/** Rucksack slots that hold something (what the sell side lists). */
export function sellSlots(st: GameState): number[] {
  return (st.me?.pack ?? []).map((s, i) => (s.n > 0 ? i : -1)).filter((i) => i >= 0);
}

export function shopPanel(fb: Fb, st: GameState, side: 0 | 1 | 2, sel: number) {
  const b = SHOP_BOX, gold = st.me?.gold ?? 0;
  fb.box(b.x, b.y, b.w, b.h, "GRIMBLE'S GOODS · you have " + gold + ' gold', GOLD);
  fb.txt(b.x + 2, b.y + 1, 'BUY', side === 0 ? GOLD : DIM, null, 1);
  fb.txt(b.x + 26, b.y + 1, 'SELL', side === 1 ? GOLD : DIM, null, 1);
  SHOP.forEach((s, i) => {
    const it = itemInfo(s.item), on = side === 0 && i === sel, y = b.y + 3 + i;
    fb.txt(b.x + 2, y, on ? '▸' : ' ', GOLD);
    fb.txt(b.x + 4, y, it.name.slice(0, 16), gold >= s.price ? (on ? WH : TX) : DIM, null, on ? 1 : 0);
    fb.txt(b.x + 21, y, String(s.price).padStart(3), gold >= s.price ? GOLD : PINK);
  });
  const wares = st.me?.wares ?? [];
  if (wares.length) fb.txt(b.x + 2, b.y + 4 + SHOP.length, 'GEAR FOR YOU', side === 0 ? GOLD : DIM, null, 1);
  wares.forEach((g, i) => {
    const on = side === 0 && SHOP.length + i === sel, y = b.y + 5 + SHOP.length + i;
    fb.txt(b.x + 2, y, on ? '▸' : ' ', GOLD);
    if (!g) { fb.txt(b.x + 4, y, '· sold', DIM); return; }
    const price = g.price ?? 0;
    fb.put(b.x + 4, y, gearCh(g, g.slot), rarityCol(g.rar), null, 1);
    fb.txt(b.x + 6, y, g.name.slice(0, 13), on ? WH : rarityCol(g.rar), null, on ? 1 : 0);
    fb.txt(b.x + 20, y, String(price).padStart(4), gold >= price ? GOLD : PINK);
  });
  const g = side === 0 ? wareAt(st, sel) : undefined;
  if (g) { // the selected piece, in full
    let y = b.y + 6 + SHOP.length + wares.length;
    fb.segs(b.x + 2, y++, [[g.name, rarityCol(g.rar), null, 1], [' · level ' + g.lvl + ' ' + g.slot, DIM]]);
    fb.txt(b.x + 4, y++, g.mods.map(modText).join(' · ').slice(0, b.w - 6), TX);
    for (const line of gearDiff(g, st.me?.[g.slot], b.w - 6).slice(0, 2)) fb.segs(b.x + 4, y++, line);
  }
  const slots = sellSlots(st), pack = st.me?.pack ?? [];
  if (!slots.length) fb.txt(b.x + 26, b.y + 3, 'nothing to sell', DIM);
  slots.slice(0, 12).forEach((slot, i) => {
    const s = pack[slot], it = itemInfo(s.item), on = side === 1 && i === sel, y = b.y + 3 + i;
    fb.txt(b.x + 25, y, on ? '▸' : ' ', GOLD);
    if (s.g) fb.txt(b.x + 27, y, s.g.name.slice(0, 13).padEnd(14), on ? WH : rarityCol(s.g.rar), null, on ? 1 : 0);
    else fb.txt(b.x + 27, y, (it.name.slice(0, 10) + ' ×' + s.n).padEnd(14), on ? WH : TX, null, on ? 1 : 0);
    fb.txt(b.x + 41, y, String(s.g ? s.g.value : sellPrice(s.item)).padStart(3), GOLD);
  });
  const keys: Seg[] = [['ENTER', WH, null, 1], [side === 0 ? ' buy · ' : ' sell · ', DIM], ['TAB', WH, null, 1], [' sell / errands · ', DIM], ['Esc', WH, null, 1], [' close', DIM]];
  fb.segs(b.x + 2, b.y + b.h - 2, keys);
}

/** Screen cell → what it hits in the shop: a side's header ('tab'), a row on a side ('row'), or null. */
export function shopAt(st: GameState, cx: number, cy: number): { kind: 'tab' | 'row'; side: 0 | 1; sel: number } | null {
  const b = SHOP_BOX;
  if (cx < b.x + 1 || cx >= b.x + b.w - 1) return null;
  const side: 0 | 1 = cx >= b.x + 25 ? 1 : 0;
  if (cy === b.y + 1) return { kind: 'tab', side, sel: 0 };
  if (side === 1) {
    const i = cy - (b.y + 3);
    return i >= 0 && i < Math.min(12, sellSlots(st).length) ? { kind: 'row', side, sel: i } : null;
  }
  if (cy >= b.y + 3 && cy < b.y + 3 + SHOP.length) return { kind: 'row', side, sel: cy - (b.y + 3) };
  const w = cy - (b.y + 5 + SHOP.length);
  return w >= 0 && w < (st.me?.wares?.length ?? 0) ? { kind: 'row', side, sel: SHOP.length + w } : null;
}
