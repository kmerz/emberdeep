declare const process: { exit(n: number): never };
// Unit tests for the pure client logic (interpolation, prediction, state handling). Run with: npm test
import { GameState } from '../src/state';
import { displayEnts } from '../src/render/interp';
import { Predictor, MOVE_MS } from '../src/render/predict';
import type { Ent, Snap } from '../src/proto';

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

if (failed) { console.error(failed + ' test(s) failed'); process.exit(1); }
console.log('all client tests passed');
process.exit(0);
