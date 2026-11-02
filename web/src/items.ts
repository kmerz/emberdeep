// Client-side info for rucksack items, keyed by the server's item id (internal/game/balance.go Items).
import type { RGB } from './term/fb';

export interface ItemInfo { name: string; ch: string; col: RGB; desc: string; thrown?: boolean; usable: boolean }

export const ITEMS: Record<string, ItemInfo> = {
  potion: { name: 'Healing Potion', ch: '!', col: [255, 92, 122], desc: 'restores 40% HP', usable: true },
  draught: { name: 'Resource Draught', ch: '&', col: [110, 190, 255], desc: 'refills half your resource', usable: true },
  firebomb: { name: 'Firebomb', ch: '*', col: [255, 140, 50], desc: 'throw at the cursor: fire blast', thrown: true, usable: true },
  torch: { name: 'Torch', ch: '/', col: [255, 170, 70], desc: 'burns for 2 minutes; you see in the dark', usable: true },
  lightorb: { name: 'Scroll of Light', ch: '?', col: [230, 230, 170], desc: 'lights the spot at the cursor for a minute', thrown: true, usable: true },
  returnscroll: { name: 'Scroll of Return', ch: '%', col: [150, 230, 255], desc: 'opens a portal back to the gnome village', usable: true },
  satchel: { name: 'Gnome Satchel', ch: '▣', col: [190, 150, 90], desc: '+4 rucksack slots (one per hero)', usable: false },
  ledger: { name: "The Tallow King's Ledger", ch: '=', col: [230, 200, 150], desc: 'a list of every soul he melted; for Grimble', usable: false },
  spoon: { name: "Grimble's Lucky Spoon", ch: '/', col: [220, 220, 240], desc: 'it is, admittedly, a very nice spoon', usable: false },
  antegg: { name: 'Ant Egg', ch: 'o', col: [240, 230, 200], desc: 'warm and faintly ticking; for Grimble', usable: false },
  embershard: { name: 'Ember Shard', ch: '*', col: [255, 110, 40], desc: 'too hot to hold for long; for Grimble', usable: false },
  gem: { name: 'Garnet', ch: '♦', col: [230, 60, 90], desc: 'worth 40 gold to a trader', usable: false },
};

export const PACK_SLOTS = 12; // slots a hero starts with (me.packCap has the real number)

export function packCap(me: { packCap?: number } | null | undefined): number { return me?.packCap || PACK_SLOTS; }

export function itemInfo(id: string): ItemInfo {
  return ITEMS[id] ?? { name: id, ch: '?', col: [200, 200, 200], desc: '', usable: false };
}

/** Grimble's wares (mirrors ShopStock in internal/game/balance.go; the server checks every price). */
export const SHOP: { item: string; price: number }[] = [
  { item: 'potion', price: 30 }, { item: 'draught', price: 30 }, { item: 'torch', price: 15 },
  { item: 'lightorb', price: 40 }, { item: 'firebomb', price: 60 }, { item: 'returnscroll', price: 120 },
  { item: 'satchel', price: 200 },
];
const VALUE: Record<string, number> = { gem: 40 };

/** What Grimble pays for one item (mirrors SellPrice). */
export function sellPrice(id: string): number {
  if (VALUE[id]) return VALUE[id];
  const s = SHOP.find((x) => x.item === id);
  return s ? Math.max(1, Math.floor(s.price / 2)) : 1;
}
