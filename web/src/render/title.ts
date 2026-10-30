// Title backdrop: a port of design/Title.dc.html (glow, rising embers, half-block gradient logo).
import { Fb, RGB, W, c01, hash, mix, ramp, DIM } from '../term/fb';

const FONT: Record<string, string[]> = {
  E: ['#####', '#....', '####.', '#....', '#....', '#####'],
  M: ['#...#', '##.##', '#.#.#', '#...#', '#...#', '#...#'],
  B: ['####.', '#...#', '####.', '#...#', '#...#', '####.'],
  R: ['####.', '#...#', '####.', '#.#..', '#..#.', '#...#'],
  D: ['####.', '#...#', '#...#', '#...#', '#...#', '####.'],
  P: ['####.', '#...#', '####.', '#....', '#....', '#....'],
};
const WORD = 'EMBERDEEP';
const LOGO_W = 9 * 12 - 2;
const GRAD: RGB[] = [[255, 225, 77], [255, 138, 31], [255, 46, 136], [177, 61, 255]];

function on(px: number, py: number): boolean {
  if (px < 0 || py < 0 || py >= 12) return false;
  const li = Math.floor(px / 12);
  if (li >= 9) return false;
  const lx = px % 12;
  if (lx >= 10) return false;
  return FONT[WORD[li]][py >> 1][lx >> 1] === '#';
}

export function drawTitle(fb: Fb, tc: number) {
  for (let y = 0; y < fb.h; y++) {
    const t = c01((y - 18) / 29);
    const bg = mix([9, 8, 14], [52, 14, 26], t * t);
    for (let x = 0; x < W; x++) fb.setBg(y * W + x, bg);
  }
  for (let i = 0; i < 110; i++) {
    const sp = 2.2 + hash(i, 1, 3) * 6, ph = hash(i, 2, 3);
    const ex = Math.round(hash(i, 3, 3) * 159 + Math.sin(tc * 1.2 + i) * 1.6);
    const ey = Math.floor(47.99 - ((tc * sp + ph * 56) % 56));
    if (ey < 0 || ey > 47) continue;
    const h = 1 - ey / 47;
    fb.put(ex, ey, h < 0.18 ? '*' : h < 0.5 ? '•' : '·', ramp([[255, 210, 120], [255, 120, 50], [255, 46, 136], [60, 26, 70]], h * 1.15), null, 1);
  }
  const x0 = Math.floor((W - LOGO_W - 2) / 2), y0 = 4, sheen = ((tc * 55) % 320) - 60;
  const col = (px: number, py: number): RGB | null => {
    if (on(px, py)) {
      let c = ramp(GRAD, px / LOGO_W);
      if (!on(px, py - 1)) c = mix(c, [255, 255, 255], 0.3);
      if (Math.abs(px - (sheen + py * 1.5)) < 3) c = mix(c, [255, 255, 255], 0.65);
      return c;
    }
    if (on(px - 2, py - 2)) return [70, 18, 64];
    return null;
  };
  for (let r = 0; r < 7; r++) for (let px = 0; px < LOGO_W + 2; px++) {
    const top = col(px, r * 2), bot = col(px, r * 2 + 1);
    if (!top && !bot) continue;
    const bi = (y0 + r) * W + x0 + px;
    const cell = fb.getBg(bi);
    fb.put(x0 + px, y0 + r, '▀', top || cell, bot || cell);
  }
  fb.ctxt(12, 'A  C O - O P  D E S C E N T  F O R  1 – 4  P L A Y E R S', [196, 186, 230], null, 1);
  fb.ctxt(13, 'in your browser · share a code · no install', DIM);
}

export function keyChips(fb: Fb, y: number, keys: [string, string][], hotIndex = 0) {
  const total = keys.reduce((a, k) => a + k[0].length + 2 + k[1].length + 4, 0) - 3;
  let kx = Math.floor((W - total) / 2);
  keys.forEach((k, i) => {
    const hot = i === hotIndex;
    fb.txt(kx, y, ' ' + k[0] + ' ', hot ? [24, 6, 20] : [24, 22, 36], hot ? [255, 46, 136] : [150, 144, 190], 1);
    kx += k[0].length + 2;
    fb.txt(kx, y, ' ' + k[1] + '   ', hot ? [240, 236, 255] : DIM, null, hot ? 1 : 0);
    kx += k[1].length + 4;
  });
}
