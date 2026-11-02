// Per-floor palettes, keyed by the theme id the server sends with each level.
// "cathedral" holds the original numbers from design/Main.dc.html.
import type { RGB } from '../term/fb';

type V3 = [number, number, number];

export interface Palette {
  name: string;
  wallTop: RGB; wallFace: RGB;     // lit walls, drawn as '▀' (top half / bottom half)
  pillarTop: RGB; pillarFace: RGB;
  memWallTop: RGB; memWallFace: RGB; // explored but not in view
  memFloorFg: RGB; memFloorBg: RGB;
  floorFg: RGB;                    // the '·' glyph
  bgBase: RGB;                     // floor background with no light
  bgLight: RGB;                    // how much each light channel adds to the background
  ambient: V3;                     // light everywhere, before torches and heroes
  torch: V3; torchR: number;
  torchGlyph: [RGB, RGB];          // torch flame flickers between these
  hero: V3;
  glow?: { c: V3; r: number; i: number; flicker: number };
}

export const THEMES: Record<string, Palette> = {
  cathedral: {
    name: 'Cathedral of Ash',
    wallTop: [100, 90, 134], wallFace: [48, 42, 70],
    pillarTop: [128, 116, 160], pillarFace: [72, 64, 98],
    memWallTop: [44, 46, 66], memWallFace: [26, 28, 40],
    memFloorFg: [40, 44, 64], memFloorBg: [10, 11, 17],
    floorFg: [118, 112, 142],
    bgBase: [13, 12, 20], bgLight: [50, 38, 34],
    ambient: [0.07, 0.07, 0.1],
    torch: [1, 0.62, 0.28], torchR: 15,
    torchGlyph: [[255, 196, 96], [255, 110, 40]],
    hero: [1, 0.9, 0.78],
  },
  crypt: {
    name: 'Flooded Crypt',
    wallTop: [78, 104, 116], wallFace: [36, 52, 62],
    pillarTop: [104, 132, 140], pillarFace: [56, 76, 86],
    memWallTop: [36, 50, 60], memWallFace: [20, 28, 36],
    memFloorFg: [34, 50, 60], memFloorBg: [8, 12, 16],
    floorFg: [104, 130, 136],
    bgBase: [9, 14, 18], bgLight: [30, 44, 50],
    ambient: [0.05, 0.08, 0.1],
    torch: [0.62, 0.86, 1], torchR: 13,
    torchGlyph: [[200, 240, 255], [110, 180, 230]],
    hero: [0.86, 0.94, 1],
  },
  caverns: {
    name: 'Fungal Caverns',
    wallTop: [86, 76, 96], wallFace: [40, 34, 48],
    pillarTop: [110, 98, 120], pillarFace: [60, 52, 70],
    memWallTop: [40, 36, 50], memWallFace: [22, 20, 30],
    memFloorFg: [38, 40, 52], memFloorBg: [9, 9, 14],
    floorFg: [108, 104, 118],
    bgBase: [10, 9, 14], bgLight: [36, 40, 40],
    ambient: [0.04, 0.04, 0.06],
    torch: [1, 0.62, 0.28], torchR: 15,
    torchGlyph: [[255, 196, 96], [255, 110, 40]],
    hero: [0.92, 0.86, 1],
    glow: { c: [0.55, 1, 0.6], r: 5, i: 0.55, flicker: 0.1 },
  },
  dark: {
    name: 'The Black Hollow',
    wallTop: [92, 86, 100], wallFace: [44, 40, 52],
    pillarTop: [110, 102, 120], pillarFace: [58, 52, 66],
    memWallTop: [26, 26, 34], memWallFace: [14, 14, 20],
    memFloorFg: [26, 28, 36], memFloorBg: [5, 5, 8],
    floorFg: [112, 106, 122],
    bgBase: [6, 6, 9], bgLight: [44, 36, 30],
    ambient: [0, 0, 0],
    torch: [1, 0.62, 0.28], torchR: 15,
    torchGlyph: [[255, 196, 96], [255, 110, 40]],
    hero: [0.7, 0.66, 0.6],
  },
  ember: {
    name: 'Ember Depths',
    wallTop: [120, 70, 64], wallFace: [58, 28, 26],
    pillarTop: [146, 90, 76], pillarFace: [80, 40, 34],
    memWallTop: [58, 36, 36], memWallFace: [32, 18, 18],
    memFloorFg: [60, 38, 36], memFloorBg: [14, 8, 8],
    floorFg: [150, 108, 96],
    bgBase: [18, 9, 8], bgLight: [58, 30, 20],
    ambient: [0.1, 0.05, 0.04],
    torch: [1, 0.5, 0.2], torchR: 15,
    torchGlyph: [[255, 180, 80], [255, 80, 30]],
    hero: [1, 0.86, 0.7],
    glow: { c: [1, 0.36, 0.1], r: 4, i: 0.7, flicker: 0.35 },
  },
};

export function palette(theme: string | undefined): Palette {
  return THEMES[theme ?? ''] ?? THEMES.cathedral;
}
