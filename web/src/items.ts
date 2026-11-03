// Client-side info for rucksack items, keyed by the server's item id (internal/game/balance.go Items).
import type { RGB } from './term/fb';
import type { GearView } from './proto';

export interface ItemInfo { name: string; ch: string; col: RGB; desc: string; thrown?: boolean; usable: boolean; short?: string /* hotbar label, max 7 */; gear?: boolean }

export const ITEMS: Record<string, ItemInfo> = {
  potion: { name: 'Healing Potion', ch: '!', col: [255, 92, 122], desc: 'restores 40% HP', usable: true, short: 'POTION' },
  draught: { name: 'Resource Draught', ch: '&', col: [110, 190, 255], desc: 'refills half your rage, energy, mana or ki; Q drinks one when your HP is fine and your resource is low', usable: true, short: 'DRAUGHT' },
  firebomb: { name: 'Firebomb', ch: '*', col: [255, 140, 50], desc: 'throw at the cursor: fire blast', thrown: true, usable: true, short: 'BOMB' },
  oil: { name: 'Lamp Oil', ch: '¡', col: [255, 170, 70], desc: '2.5 minutes of light; goes straight into your lantern', usable: false },
  lightorb: { name: 'Scroll of Light', ch: '?', col: [230, 230, 170], desc: 'lights the spot at the cursor for a minute', thrown: true, usable: true, short: 'LIGHT' },
  returnscroll: { name: 'Scroll of Return', ch: '%', col: [150, 230, 255], desc: 'opens a portal back to the gnome village', usable: true, short: 'RETURN' },
  satchel: { name: 'Gnome Satchel', ch: '▣', col: [190, 150, 90], desc: '+4 rucksack slots (one per hero)', usable: false },
  ledger: { name: "The Tallow King's Ledger", ch: '=', col: [230, 200, 150], desc: 'a list of every soul he melted; for Grimble', usable: false },
  spoon: { name: "Grimble's Lucky Spoon", ch: '/', col: [220, 220, 240], desc: 'it is, admittedly, a very nice spoon', usable: false },
  antegg: { name: 'Ant Egg', ch: 'o', col: [240, 230, 200], desc: 'warm and faintly ticking; for Grimble', usable: false },
  embershard: { name: 'Ember Shard', ch: '*', col: [255, 110, 40], desc: 'too hot to hold for long; for Grimble', usable: false },
  gem: { name: 'Garnet', ch: '♦', col: [230, 60, 90], desc: 'a trade good: Grimble pays 40 gold for one', usable: false },
  weapon: { name: 'Weapon', ch: ')', col: [200, 196, 186], desc: 'wear it', usable: true, gear: true },
  trinket: { name: 'Trinket', ch: '"', col: [200, 196, 186], desc: 'wear it', usable: true, gear: true },
  key: { name: 'Vault Key', ch: '⌐', col: [255, 212, 92], desc: "opens this floor's locked vault door: walk into it", usable: false },
  coin: { name: 'Old Coin', ch: '¢', col: [214, 190, 120], desc: 'a coin of a kingdom nobody remembers; Grimble pays 15 gold', usable: false },
  ring: { name: 'Silver Ring', ch: '°', col: [210, 220, 235], desc: "someone's wedding ring; Grimble pays 80 gold", usable: false },
  idol: { name: 'Gold Idol', ch: '♀', col: [255, 200, 60], desc: 'heavy, grinning and very old; Grimble pays 200 gold', usable: false },
};

export const PACK_SLOTS = 12; // slots a hero starts with (me.packCap has the real number)

export function packCap(me: { packCap?: number } | null | undefined): number { return me?.packCap || PACK_SLOTS; }

export function itemInfo(id: string): ItemInfo {
  return ITEMS[id] ?? { name: id, ch: '?', col: [200, 200, 200], desc: '', usable: false };
}

/** Grimble's wares (mirrors ShopStock in internal/game/balance.go; the server checks every price). */
export const SHOP: { item: string; price: number }[] = [
  { item: 'potion', price: 30 }, { item: 'draught', price: 30 }, { item: 'oil', price: 15 },
  { item: 'lightorb', price: 40 }, { item: 'firebomb', price: 60 }, { item: 'returnscroll', price: 120 },
  { item: 'satchel', price: 200 },
];
/** Trade goods: what Grimble pays (mirrors Value in balance.go). */
export const VALUE: Record<string, number> = { gem: 40, coin: 15, ring: 80, idol: 200 };

/** What Grimble pays for one item (mirrors SellPrice). */
export function sellPrice(id: string): number {
  if (VALUE[id]) return VALUE[id];
  const s = SHOP.find((x) => x.item === id);
  return s ? Math.max(1, Math.floor(s.price / 2)) : 1;
}

// ---- gear (mirrors internal/game/gear.go) ----

export const RARITY = ['common', 'magic', 'rare'];
export const RARITY_COL: RGB[] = [[200, 196, 186], [110, 160, 255], [255, 205, 70]];
export function rarityCol(r: number | undefined): RGB { return RARITY_COL[r ?? 0] ?? RARITY_COL[0]; }

/** Glyph for a piece of gear by its base. */
const GEAR_CH: Record<string, string> = {
  axe: ')', sword: ')', dagger: ')', kris: ')', wand: '/', rod: '/', staff: '/', knuckles: ')',
  amulet: '"', charm: '"', talisman: '"',
};
export function gearCh(g: GearView | undefined, slot = 'weapon'): string { return (g && GEAR_CH[g.base]) || (slot === 'trinket' ? '"' : ')'); }

const STAT: Record<string, [string, boolean]> = { // label, shown as a percentage
  dmg: ['damage', true], crit: ['critical hit chance', true], leech: ['of damage healed', true], cdr: ['shorter cooldowns', true],
  res: ['resource gain', true], armor: ['less damage taken', true], hp: ['max HP', false],
};
/** "+12% damage", "+20 max HP". */
export function modText(m: { k: string; v: number }): string {
  const [label, pct] = STAT[m.k] ?? [m.k, false];
  return (m.v >= 0 ? '+' : '') + m.v + (pct ? '% ' : ' ') + label;
}
/** The caps on the hero's totals (GearCaps). */
export const GEAR_CAPS: Record<string, number> = { crit: 40, leech: 15, cdr: 35, armor: 50 };

/** What the hero's worn gear adds up to, stat by stat, in display order. */
export function gearTotals(...worn: (GearView | undefined)[]): { k: string; v: number }[] {
  const t: Record<string, number> = {};
  for (const g of worn) for (const m of g?.mods ?? []) t[m.k] = (t[m.k] ?? 0) + m.v;
  return Object.keys(STAT).filter((k) => t[k]).map((k) => ({ k, v: Math.min(t[k], GEAR_CAPS[k] ?? Infinity) }));
}
