declare const process: { exit(n: number): never };
// Unit tests for the pure client logic (interpolation, prediction, state handling). Run with: npm test
import { GameState } from '../src/state';
import { displayEnts } from '../src/render/interp';
import { Predictor, MOVE_MS } from '../src/render/predict';
import type { Ent, Snap } from '../src/proto';
import { Fx } from '../src/render/fx';
import { Fb } from '../src/term/fb';
import { VW, VH } from '../src/render/map';

let failed = 0;
function eq(a: unknown, b: unknown, msg: string) {
  if (JSON.stringify(a) !== JSON.stringify(b)) { failed++; console.error('FAIL', msg, 'got', JSON.stringify(a), 'want', JSON.stringify(b)); }
  else console.log('ok  ', msg);
}

const hero = (id: number, x: number, y: number): Ent => ({ id, k: 'h', g: '@', x, y, cls: 'rogue', name: 'h' + id, face: 2 });
const snap = (tick: number, ents: Ent[], ack = 0): Snap => ({
  t: 'snap', tick, ackSeq: ack, ents, party: [], obj: { kingAlive: true, stairsAt: 0, stairsNeed: 1, stairsLeft: 0, stairsFound: false, gold: 0, kills: 0 },
  vis: { x0: 0, y0: 0, w: 0, h: 0, b: '' },
});

// --- interpolation: remote heroes render 100 ms behind, cell-snapped; the local hero is never delayed
{
  const st = new GameState();
  st.youId = 1;
  st.handle(snap(1, [hero(1, 5, 5), hero(2, 10, 10)]), 1000);
  st.handle(snap(2, [hero(1, 6, 5), hero(2, 11, 10)]), 1050);
  st.handle(snap(3, [hero(1, 7, 5), hero(2, 12, 10)]), 1100);
  st.handle(snap(4, [hero(1, 8, 5), hero(2, 13, 10)]), 1150);
  let e = displayEnts(st, 1160); // render time 1060 -> latest snap at or before is t=1050
  eq(e.find((x) => x.id === 2)!.x, 11, 'remote hero is 100 ms behind');
  eq(e.find((x) => x.id === 1)!.x, 8, 'local hero uses the latest snapshot');
  e = displayEnts(st, 1300); // everything older than 1200 -> newest snapshot
  eq(e.find((x) => x.id === 2)!.x, 13, 'remote hero catches up when idle');
  e = displayEnts(st, 1000); // before the first snapshot -> oldest
  eq(e.find((x) => x.id === 2)!.x, 10, 'before the first snapshot we show the oldest');
}

// --- prediction: apply at once, replay unacked moves over the server position
{
  const st = new GameState();
  st.youId = 1;
  const p = new Predictor(st);
  for (let y = 0; y < 20; y++) for (let x = 0; x < 20; x++) st.tiles[y * 176 + x] = '.'.charCodeAt(0);
  st.handle(snap(1, [hero(1, 5, 5)]), 0);
  p.onSnap();
  const m1 = p.move(2, 1000) as any; // east
  eq([p.x, p.y], [6, 5], 'move is applied immediately');
  eq(p.move(2, 1000 + MOVE_MS - 10), null, 'moves are rate limited client-side');
  const m2 = p.move(2, 1000 + MOVE_MS) as any;
  eq([p.x, p.y], [7, 5], 'second move predicted');
  // the server acked the first move only: replay the second on top
  st.handle(snap(2, [hero(1, 6, 5)], m1.seq), 1100);
  p.onSnap();
  eq([p.x, p.y], [7, 5], 'unacked move replayed over the server position');
  // the server rejected the second (e.g. blocked): ack both, position stays at 6
  st.handle(snap(3, [hero(1, 6, 5)], m2.seq), 1200);
  p.onSnap();
  eq([p.x, p.y], [6, 5], 'rejected move is corrected by the snapshot');
  // walls stop prediction
  st.tiles[5 * 176 + 7] = '#'.charCodeAt(0);
  p.move(2, 5000);
  eq([p.x, p.y], [6, 5], 'predicted move into a wall is ignored');
}

// --- vis bitset and explore events
{
  const st = new GameState();
  st.handle({ t: 'explore', tiles: [[3, 4, 46], [5, 6, 35]] }, 0);
  eq([st.tileAt(3, 4), st.tileAt(5, 6), st.tileAt(0, 0)], [46, 35, 0], 'explore events fill the tile map');
  const s = snap(1, []);
  s.vis = { x0: 2, y0: 3, w: 3, h: 2, b: btoa(String.fromCharCode(0b000101, 0)) }; // bits 0 and 2 -> (2,3) and (4,3)
  st.handle(s, 0);
  eq([st.isVisible(2, 3), st.isVisible(3, 3), st.isVisible(4, 3)], [true, false, true], 'visibility bitset decodes');
}

// --- effects pipeline: every signature effect draws glyphs, lights and (where specified) shake
{
  const st = new GameState();
  st.youId = 1;
  for (let y = 0; y < 62; y++) for (let x = 0; x < 176; x++) { st.tiles[y * 176 + x] = '.'.charCodeAt(0); st.vis[y * 176 + x] = 1; }
  const cam = { x: 0, y: 0 };
  const sample = (kind: string, ev: any, times: number[], check: (fb: Fb, t: number) => void) => {
    const fx = new Fx();
    fx.add(ev, 0, 1);
    for (const t of times) {
      const fb = new Fb(VW, VH, [7, 8, 12]);
      fx.drawUnder(fb, cam, t, st);
      fx.drawOver(fb, cam, t, st);
      check(fb, t);
    }
    return fx;
  };
  const has = (fb: Fb, chars: string) => fb.ch.some((c) => chars.includes(c));
  const colorOf = (fb: Fb, chars: string): number[] | null => { const i = fb.ch.findIndex((c) => chars.includes(c)); return i < 0 ? null : fb.getFg(i); };
  // Leap: shock ring with tangent glyphs in grey-gold, dust, shake for nearby heroes only
  const leap = sample('leap', { k: 'impact', fx: 'leap', x: 40, y: 20, r: 3.5, actor: 1 }, [150], (fb) => {
    eq(has(fb, '─╲│╱'), true, 'leap draws the tangent shock ring');
    eq(has(fb, '░'), true, 'leap scatters dust');
    const c = colorOf(fb, '─╲│╱')!;
    eq(c[0] > c[2] && c[1] > c[2] && c[0] < 255, true, 'leap ring is grey-gold');
  });
  eq(leap.shake(100, 42, 20).some((v) => v !== 0), true, 'leap shakes the camera for nearby heroes');
  eq(leap.shake(100, 90, 20), [0, 0], 'leap does not shake distant heroes');
  eq(leap.lights(100, st, []).length, 1, 'leap contributes a light');
  // Bell: two staggered gold rings, then a circle echo, plus a warm light on allies
  const bell = new Fx();
  bell.add({ k: 'cast', fx: 'bell', x: 40, y: 20, r: 6, actor: 1 }, 0, 1);
  const fbA = new Fb(VW, VH, [7, 8, 12]); bell.drawUnder(fbA, cam, 400, st);
  eq(has(fbA, '─╲│╱'), true, 'bell draws expanding rings');
  const fbB = new Fb(VW, VH, [7, 8, 12]); bell.drawUnder(fbB, cam, 1200, st);
  eq(has(fbB, '○'), true, 'bell draws the circle echo');
  const lights = bell.lights(300, st, [{ x: 41, y: 20 }, { x: 80, y: 40 }]);
  eq(lights.filter((l) => l.c[0] === 1 && l.c[1] === 0.85).length >= 1, true, 'bell pulses warm light on allies in range');
  eq(lights.some((l) => l.x === 80), false, 'bell does not light distant allies');
  // Shadow Bomb uses the void palette, Ember Nova the ember palette (same drawBlast pipeline)
  const cols: Record<string, number[]> = {};
  for (const [fxId, pal] of [['nova', 'ember'], ['shadowbomb', 'void']] as const) {
    sample(fxId, { k: 'impact', fx: fxId, x: 40, y: 20, r: 6, actor: 1 }, [450], (fb) => {
      cols[pal] = colorOf(fb, '█▓▒░')!;
    });
  }
  eq(cols.void[2] > cols.void[0], true, 'void rift is cool teal/violet (blue > red)');
  eq(cols.ember[0] > cols.ember[2], true, 'ember nova is warm (red > blue)');
  const nova = sample('nova', { k: 'impact', fx: 'nova', x: 40, y: 20, r: 9, actor: 1 }, [10], () => {});
  eq(nova.shake(100, 40, 20).some((v) => v !== 0), true, 'ember nova shakes the camera');
  eq(nova.heat(100).length, 1, 'ember nova scorches the ground');
  // events from other players are delayed by the interpolation window so FX line up with remote entities
  const fxd = new Fx();
  fxd.add({ k: 'impact', fx: 'nova', x: 1, y: 1, actor: 2 }, 1000, 1);
  fxd.add({ k: 'impact', fx: 'nova', x: 1, y: 1, actor: 1 }, 1000, 1);
  eq(fxd.list.map((f) => f.t0), [1100, 1000], 'remote FX start 100 ms later than our own');
}

if (failed) { console.error(failed + ' test(s) failed'); process.exit(1); }
console.log('all client tests passed');
process.exit(0);
