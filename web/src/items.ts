// Client-side info for rucksack items, keyed by the server's item id (internal/game/balance.go Items).
import type { RGB } from './term/fb';

export interface ItemInfo { name: string; ch: string; col: RGB; desc: string; thrown?: boolean; usable: boolean }

export const ITEMS: Record<string, ItemInfo> = {
  potion: { name: 'Healing Potion', ch: '!', col: [255, 92, 122], desc: 'restores 40% HP', usable: true },
  draught: { name: 'Resource Draught', ch: '&', col: [110, 190, 255], desc: 'refills half your resource', usable: true },
  firebomb: { name: 'Firebomb', ch: '*', col: [255, 140, 50], desc: 'throw at the cursor: fire blast', thrown: true, usable: true },
  gem: { name: 'Garnet', ch: '♦', col: [230, 60, 90], desc: 'worth 40 gold to a trader', usable: false },
};

export const PACK_SLOTS = 12;

export function itemInfo(id: string): ItemInfo {
  return ITEMS[id] ?? { name: id, ch: '?', col: [200, 200, 200], desc: '', usable: false };
}
