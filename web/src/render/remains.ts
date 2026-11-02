// What a monster leaves on the floor when it dies, and what it bleeds when hit hard. Client-only decoration:
// the server never hears about it, and it is forgotten on the next floor.
import type { RGB } from '../term/fb';

export type Remain = [string, RGB];

const BONES: RGB = [190, 182, 160];
const RED: RGB = [150, 30, 45];
const SLIME: RGB = [80, 170, 70];
const ICHOR: RGB = [160, 150, 60];

const REMAINS: Record<string, Remain> = {
  z: ['%', [120, 140, 90]], s: ['%', BONES], r: [',', BONES], h: ['%', [220, 210, 190]], n: ['%', [150, 110, 200]],
  f: [',', [190, 60, 50]], b: [',', [110, 90, 150]], K: ['≈', [230, 220, 170]], M: ['░', [110, 70, 40]],
  a: [':', [150, 110, 70]], A: [':', [190, 90, 50]], Q: ['%', [255, 170, 60]],
  i: ['·', [130, 125, 120]], d: ['%', [180, 60, 50]], G: [':', [150, 70, 30]], U: [',', [200, 90, 160]], I: [',', [140, 70, 200]],
  D: ['%', [255, 120, 60]], o: ['·', SLIME],
};

const BLEED: Record<string, RGB> = {
  z: [90, 60, 40], f: RED, b: RED, U: RED, I: RED, d: RED, D: RED, K: [220, 210, 150],
  o: SLIME, a: ICHOR, A: ICHOR, Q: ICHOR,
};

/** The glyph a dead monster of this kind leaves behind (null: nothing, like a shade). */
export function remainsFor(kind: string): Remain | null {
  return REMAINS[kind] ?? null;
}

/** The splatter color for a heavy hit on this kind ('' = a hero), or null if it does not bleed. */
export function bleedFor(kind: string): RGB | null {
  return kind === '' ? RED : BLEED[kind] ?? null;
}

/** Tiles remains may lie on: open floor and floor decor, not walls, stairs, water or embers. */
export function remainsOk(tile: number): boolean {
  const c = String.fromCharCode(tile);
  return tile !== 0 && !'#TO>=^'.includes(c);
}
