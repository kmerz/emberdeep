// Grimble's shop: buy on the left, sell from your rucksack on the right. Opens when you walk into him.
import { Fb, Seg, DIM, TX, WH, GOLD, PINK } from '../term/fb';
import type { GameState } from '../state';
import { SHOP, itemInfo, sellPrice } from '../items';

export const SHOP_BOX = { x: 34, y: 6, w: 46, h: 16 };

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
  const slots = sellSlots(st), pack = st.me?.pack ?? [];
  if (!slots.length) fb.txt(b.x + 26, b.y + 3, 'nothing to sell', DIM);
  slots.slice(0, 10).forEach((slot, i) => {
    const s = pack[slot], it = itemInfo(s.item), on = side === 1 && i === sel, y = b.y + 3 + i;
    fb.txt(b.x + 25, y, on ? '▸' : ' ', GOLD);
    fb.txt(b.x + 27, y, (it.name.slice(0, 10) + ' ×' + s.n).padEnd(14), on ? WH : TX, null, on ? 1 : 0);
    fb.txt(b.x + 41, y, String(sellPrice(s.item)).padStart(3), GOLD);
  });
  const keys: Seg[] = [['ENTER', WH, null, 1], [side === 0 ? ' buy · ' : ' sell · ', DIM], ['TAB', WH, null, 1], [' sell / errands · ', DIM], ['Esc', WH, null, 1], [' close', DIM]];
  fb.segs(b.x + 2, b.y + b.h - 2, keys);
}
