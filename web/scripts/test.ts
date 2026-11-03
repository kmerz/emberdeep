declare const process: { exit(n: number): never };
// Unit tests for the pure client logic (interpolation, prediction, state handling). Run with: npm test
import { GameState, LOG_W, wrapSegs } from '../src/state';
import { FORM, STYLES, makeSong, noteOf } from '../src/music';
import { journalQuests } from '../src/render/quests';
import { loreTitle } from '../src/render/lore';
import { dangerAt, camAxis } from '../src/screens/game';
import { LandingScreen, fameTime, fameNames } from '../src/screens/landing';
import { viewCell } from '../src/term/canvas';
import { stickKeys } from '../src/touch';
import { treeSelAt } from '../src/render/tree';
import { questRowAt } from '../src/render/quests';
import { shopAt } from '../src/render/shop';
import { SHOP, modText, temperPrice, reforgePrice, itemInfo } from '../src/items';
import { forgeRows, forgeAt, FORGE_BOX } from '../src/render/forge';
import { menuRows, menuAt, MENU_BOX } from '../src/render/menu';
import { compactCols, landscapeScale } from '../src/mobile';
import { displayEnts } from '../src/render/interp';
import { Predictor, MOVE_MS } from '../src/render/predict';
import type { Ent, Snap } from '../src/proto';
import { Fx, streakLabel } from '../src/render/fx';
import { depthBanner, hotbar, pickTarget, compactHud, overlayBox, OVERLAY_BOX, short, eventNote } from '../src/render/hud';
import { Fb } from '../src/term/fb';
import { VW, VH } from '../src/render/map';
import { GamepadInput, keyLabel, lastInput, padLabel, padAim, type PadLike, type PadContext } from '../src/gamepad';
import { translateKey, bindKey, bindPad, binds, setBinds, defaults, padWorldMap, kbLabel, BTN } from '../src/bindings';
import { coneTarget, step8, aheadCell } from '../src/screens/game';
import { controlsAt, cellEditable, CONTROLS_BOX } from '../src/render/controls';

let failed = 0;
function eq(a: unknown, b: unknown, msg: string) {
  if (JSON.stringify(a) !== JSON.stringify(b)) { failed++; console.error('FAIL', msg, 'got', JSON.stringify(a), 'want', JSON.stringify(b)); }
  else console.log('ok  ', msg);
}

const hero = (id: number, x: number, y: number): Ent => ({ id, k: 'h', g: '@', x, y, cls: 'rogue', name: 'h' + id, face: 2 });
const snap = (tick: number, ents: Ent[], ack = 0): Snap => ({
  t: 'snap', tick, ackSeq: ack, ents, party: [], obj: { boss: false, kingAlive: true, stairsAt: 0, stairsNeed: 1, stairsLeft: 0, stairsFound: false, gold: 0, kills: 0 },
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

// --- gamepad: pad input becomes key events for the current screen
{
  const mk = (pressed: number[] = [], axes: number[] = [0, 0]): PadLike => ({
    connected: true, axes,
    buttons: Array.from({ length: 17 }, (_, i) => ({ pressed: pressed.includes(i) })),
  });
  let cur: PadLike | null = mk();
  const gp = new GamepadInput(() => [cur]);
  let log: string[] = [];
  const sink = { down: (k: string, r: boolean) => log.push('d:' + k + (r ? '*' : '')), up: (k: string) => log.push('u:' + k) };
  const step = (now: number, ctx: PadContext) => { log = []; gp.poll(now, ctx, sink); return log; };

  eq(step(0, 'world'), [], 'idle pad emits nothing');
  cur = mk([], [0.2, -0.2]);
  eq(step(30, 'world'), [], 'stick inside the deadzone is ignored');
  cur = mk([], [0.9, -0.9]);
  eq(step(60, 'world'), ['d:ArrowUp', 'd:ArrowRight'], 'stick pushes held arrow keys (diagonal)');
  eq(step(90, 'world'), ['d:ArrowUp*', 'd:ArrowRight*'], 'world movement re-asserts held keys every poll');
  cur = mk();
  eq(step(120, 'world'), ['u:ArrowUp', 'u:ArrowRight'], 'releasing the stick releases the keys');

  cur = mk([2]);
  eq(step(150, 'world'), ['d:2'], 'X casts skill 2 in the world');
  eq(step(180, 'world'), [], 'a held button does not repeat');
  cur = mk();
  eq(step(210, 'world'), ['u:2'], 'releasing the button sends keyup');
  cur = mk([0]);
  eq(step(240, 'world'), ['d:1'], 'A is skill 1 (never Enter: that would open chat)');
  cur = mk();
  step(250, 'world');
  cur = mk([6]);
  eq(step(270, 'world'), ['d:Tab'], 'LT shows the map');
  cur = mk();
  eq(step(300, 'world'), ['u:Tab'], 'and lets go');
  cur = mk([1]);
  eq(step(310, 'world'), ['d:g'], 'B pings');
  cur = mk();
  eq(step(320, 'world'), ['u:g'], 'releasing B sends keyup');
  cur = mk([5]);
  eq(step(321, 'world'), ['d:4'], 'RB uses the item on key 4');
  cur = mk();
  step(322, 'world');

  eq(step(330, 'panel'), [], 'switching context emits nothing by itself');
  cur = mk([0]);
  eq(step(360, 'panel'), ['d:Enter'], 'A confirms in a panel');
  cur = mk([1]);
  eq(step(390, 'panel'), ['u:Enter', 'd:Escape'], 'B closes a panel');
  cur = mk();
  step(420, 'panel');

  cur = mk([0]);
  step(450, 'world');
  eq(step(480, 'panel'), [], 'a button held across a context change is ignored until released');
  cur = mk();
  step(510, 'panel');
  cur = mk([0]);
  eq(step(540, 'panel'), ['d:Enter'], '...then works again');
  cur = mk();
  step(570, 'panel');

  cur = mk([], [0, 1]);
  eq(step(600, 'panel'), ['d:ArrowDown'], 'menu: first step is immediate');
  eq(step(660, 'panel'), [], 'menu: no repeat before the delay');
  eq(step(760, 'panel'), ['d:ArrowDown'], 'menu: repeats while held');
  eq(step(800, 'world'), ['u:ArrowDown'], 'context change releases held keys');
  cur = mk();
  step(830, 'world');

  cur = mk([0]);
  eq(step(860, 'chat'), [], 'no buttons act while chatting');
  cur = mk();
  step(890, 'chat');

  cur = mk([], [1, 0]);
  step(920, 'world');
  cur = null;
  eq(step(950, 'world'), ['u:ArrowRight'], 'unplugging the pad releases held keys');
  eq(step(980, 'lobby'), [], 'no pad, no events');
}

// --- game feel: kill streaks, death bursts, remains on the floor
{
  eq([2, 3, 4, 7].map(streakLabel), ['DOUBLE KILL', 'TRIPLE KILL', 'QUAD KILL', 'MASSACRE ×7'], 'streak labels');
  const fx = new Fx();
  const got: number[] = [];
  fx.onStreak = (_a, n) => got.push(n);
  const kill = (t: number, actor = 1) => fx.add({ k: 'kill', actor, target: 20000 + t, x: 5, y: 5, fx: 's' }, t, 1);
  kill(1000); kill(1500); kill(2000); kill(4000);
  eq(got, [2, 3], 'kills within 1.5 s chain into a streak; a pause resets it');
  eq(fx.list.filter((f) => f.kind === 'streak').length, 1, 'a new streak popup replaces the old one');
  eq(fx.list.filter((f) => f.kind === 'burst').length, 4, 'every kill bursts');
  const fb = new Fb(VW, VH, [0, 0, 0]);
  const st = new GameState();
  st.tiles.fill(46); st.vis.fill(1);
  fx.drawUnder(fb, { x: 0, y: 0 }, 1030, st);
  eq(fb.ch[5 * VW + 5], '*', 'a dying monster pops on its cell');

  // remains: corpses where monsters die, never on stairs; reset on a new floor
  st.tiles[6 * 176 + 6] = 62; // '>'
  st.markRemains([{ k: 'kill', x: 3, y: 3, fx: 's' }, { k: 'kill', x: 6, y: 6, fx: 's' }, { k: 'kill', x: 8, y: 8, fx: 'S' }]);
  eq(st.remains.get(3 * 176 + 3)?.[0], '%', 'a skeleton leaves bones');
  eq(st.remains.has(6 * 176 + 6), false, 'nothing lands on the stairs');
  eq(st.remains.has(8 * 176 + 8), false, 'a shade leaves nothing');
  st.handle({ t: 'level', w: 176, h: 62, seed: 'x', torches: [], stairs: [1, 1], start: [2, 2], depth: 2, maxDepth: 10, theme: 'crypt', glows: [] }, 5000);
  eq(st.remains.size, 0, 'a new floor starts clean');

  // the floor card: depth label, then the theme name typed in, then the flavor line
  const big = new Fb(160, 48, [0, 0, 0]);
  depthBanner(big, st, 5000 + 2000);
  const row = (y: number) => Array.from({ length: 160 }, (_, x) => big.ch[y * 160 + x]).join('');
  eq(row(8).includes('DEPTH 02'), true, 'banner shows the depth');
  eq(row(10).includes('F L O O D E D'), true, 'banner shows the theme name');
  eq(row(12).includes('counts your steps'), true, 'banner shows the flavor line');
}

// --- a world that reacts: speech bubbles, scorch marks, kegs are never auto-targeted
{
  const st = new GameState();
  st.tiles.fill(46);
  st.handle({ t: 'events', tick: 1, list: [{ k: 'bark', actor: 20001, x: 4, y: 4, text: 'Kneel, little wicks.', fx: 'K' }] }, 1000);
  eq(st.barks.get(20001)?.text, 'Kneel, little wicks.', 'a bark opens a speech bubble');
  eq(st.log.some((l) => l.segs.some((s) => s[0] === 'Kneel, little wicks.')), true, 'a boss line also goes to the log');
  st.handle({ t: 'events', tick: 2, list: [{ k: 'bark', actor: 20002, x: 6, y: 4, text: '*blorp*', fx: 'o' }] }, 1000);
  eq(st.log.some((l) => l.segs.some((s) => s[0] === '*blorp*')), false, 'an ordinary monster line stays out of the log');
  st.handle({ t: 'events', tick: 3, list: [{ k: 'impact', actor: 1, x: 10, y: 10, fx: 'keg', r: 3.5 }] }, 1000);
  eq(st.remains.get(10 * 176 + 10)?.[0], '░', 'a keg blast leaves a scorch mark');
  const keg: Ent = { id: 30000, k: 'm', g: 'B', x: 5, y: 5, face: 0 };
  const rat: Ent = { id: 30001, k: 'm', g: 'r', x: 9, y: 5, face: 0 };
  eq(pickTarget(st, [keg, rat], { x: 4, y: 5 })?.id, 30001, 'the target panel skips a nearer powder keg');
}

// --- item key 4: the hotbar shows the bound item and how many are left in the rucksack
{
  const st = new GameState();
  st.youId = 1;
  st.party = [{ id: 1, name: 'vela', cls: 'rogue', lvl: 1, hp: 10, hpMax: 10, res: 0, resMax: 100, connected: true }];
  st.me = {
    res: 0, resMax: 100, cd: [0, 0, 0], cdMax: [1, 1, 1], cost: [0, 0, 0], potions: 0, potCd: 0, gold: 0, xp: 0, xpNeed: 10, lvl: 1,
    pack: [{ item: 'firebomb', n: 2 }, { item: 'potion', n: 1 }, { item: 'firebomb', n: 1 }], packCap: 12,
    binds: ['stab', '', ''], tree: [], points: 0, respec: false,
  };
  const row = (fb: Fb, y: number) => Array.from({ length: 160 }, (_, x) => fb.ch[y * 160 + x]).join('');
  let fb = new Fb(160, 48, [0, 0, 0]);
  hotbar(fb, st, 'rogue', ['firebomb', 'potion']);
  eq(row(fb, 40).includes('BOMB'), true, 'hotbar names the item on key 4');
  eq(row(fb, 41).includes('×3'), true, 'hotbar counts the item across rucksack stacks');
  eq(row(fb, 40).split('POTION').length - 1, 2, 'key 5 holds a second item (next to the Q potion slot)');
  eq(row(fb, 37).includes('5'), true, 'the sixth slot is labelled 5');
  fb = new Fb(160, 48, [0, 0, 0]);
  hotbar(fb, st, 'rogue', []);
  eq(row(fb, 41).includes('(I)'), true, 'an empty item key says how to fill it');
  for (let y = 35; y < 47; y++) eq(row(fb, y).slice(112).trim(), '', 'the hotbar stays inside its box (row ' + y + ')');
}

// --- hit log: your hits on one monster fold into one line; hits on you and your kills are logged too
{
  const st = new GameState();
  st.youId = 1;
  const z = { id: 20005, k: 'm' as const, g: 'z', x: 5, y: 5, name: 'Zombie', face: 0 };
  const k = { id: 20006, k: 'm' as const, g: 'B', x: 6, y: 5, name: 'Powder Keg', face: 0 };
  st.recs.push({ t: 0, snap: {} as never, ents: [z, k] });
  const text = (i: number) => st.log[i].segs.map((s) => s[0]).join('');
  const hit = (actor: number, target: number, amount: number, fx = '', at = 1000) =>
    st.handle({ t: 'events', tick: 1, list: [{ k: 'hit', actor, target, x: 5, y: 5, amount, fx }] }, at);
  hit(1, 20005, 12);
  eq(text(st.log.length - 1), 'You hit Zombie for 12', 'first hit gets its own line');
  hit(1, 20005, 20, 'crit', 1500);
  hit(1, 20005, 9, '', 2000);
  eq(st.log.length, 1, 'repeat hits on the same monster share one line');
  eq(text(0), 'You hit Zombie ×3 · 41 dmg · crit', 'the line counts hits, damage and crits');
  hit(1, 20005, 5, '', 9000);
  eq(st.log.length, 2, 'after a pause a new line starts');
  hit(20005, 1, 7, '', 9100);
  eq(text(2), 'Zombie hits you for 7', 'hits on you are logged');
  hit(0, 1, 10, '', 9150);
  eq(text(3), 'You are hurt for 10', 'trap and fire damage is logged without an attacker');
  hit(2, 20005, 30, '', 9200);
  eq(st.log.length, 4, "an ally's hits stay out of the log");
  hit(1, 20006, 1, '', 9300);
  eq(st.log.length, 4, 'hitting a powder keg is not logged as a hit');
  st.recs.push({ t: 1, snap: {} as never, ents: [k] }); // the zombie is gone from the newest snapshot
  hit(1, 20005, 6, '', 9400);
  eq(text(1), 'You hit Zombie ×2 · 11 dmg', 'the killing blow still finds the name in the previous snapshot');
  st.handle({ t: 'events', tick: 1, list: [{ k: 'kill', actor: 1, target: 20005, x: 5, y: 5, text: 'Zombie', fx: 'z' }] }, 9500);
  eq(text(st.log.length - 1), 'You slay the Zombie', 'your kills are logged');
}

// --- long log lines wrap instead of running out of the log box; hints name pad buttons while playing with a pad
{
  const lines = wrapSegs([['keys: ', [1, 1, 1]], ['WASD move · 1-3 skills · 4 item · Q potion · hold E revive · Alt+click ping', [2, 2, 2]]], LOG_W);
  eq(lines.length, 2, 'a long line becomes two');
  eq(lines.every((l) => l.reduce((a, s) => a + s[0].length, 0) <= LOG_W), true, 'no wrapped line is wider than the log');
  eq(lines.map((l) => l.map((s) => s[0]).join('')).join(' '), 'keys: WASD move · 1-3 skills · 4 item · Q potion · hold E revive · Alt+click ping', 'wrapping loses no text');
  eq(lines[1][0][1], [2, 2, 2], 'the second line keeps the color');
  const st = new GameState();
  st.pushLog([['x'.repeat(LOG_W + 5), [1, 1, 1]]]);
  eq(st.log.length, 2, 'an unbreakable word is cut hard');
  eq(padLabel('world', 'q'), 'LB', 'the potion is LB on the pad');
  eq(padLabel('world', '1'), 'A', 'skill 1 is A');
  eq(padLabel('panel', 'Tab'), 'RB', 'in panels RB flips to the next tab');
  lastInput.pad = false;
  eq(keyLabel('world', 'e'), 'E', 'keyboard hints show the key');
  lastInput.pad = true;
  eq(keyLabel('world', 'q'), 'LB', 'pad hints show the button');
  eq(keyLabel('world', 'm'), 'M', 'a key with no button falls back to the key');
  lastInput.pad = false;
}

// --- music: every floor has its own song, the same for the whole party; deeper floors play faster
{
  const a = makeSong('crypt', 3, 'ABC123'), b = makeSong('crypt', 3, 'ABC123'), c = makeSong('crypt', 5, 'ABC123');
  eq(JSON.stringify(a), JSON.stringify(b), 'the same floor gives the same song');
  eq(JSON.stringify(a?.melodies) !== JSON.stringify(c?.melodies), true, 'another floor gives another melody');
  eq((c?.bpm ?? 0) > (a?.bpm ?? 0), true, 'deeper floors are faster');
  eq(makeSong('nowhere', 1, 'x'), null, 'no song without a theme');
  eq(makeSong('cathedral', 1, 'x')?.drums.every((d) => d.length === 0), true, 'the cathedral has no drums');
  eq(makeSong('ember', 9, 'x')?.drums.some((d) => d.includes('kick')), true, 'the ember depths have drums');
  const song = makeSong('hive', 4, 'x')!;
  eq(song.melodies.every((m) => m.every((n) => n.step >= 0 && n.step + n.len <= 32)), true, 'melody notes stay inside their two bars');
  eq(noteOf(STYLES.village, 0), 60, 'degree 0 is the root');
  eq(noteOf(STYLES.village, 7), 72, 'degree 7 is an octave up');
  eq(noteOf(STYLES.village, 2, -1), 52, 'octave shifts work');
  eq(FORM.some((f) => f.drums) && FORM.some((f) => !f.drums), true, 'the song has quiet and full sections');
  eq(dangerAt([], { x: 0, y: 0 }), 0, 'no monsters, no danger');
  eq(dangerAt([{ k: 'm', g: 'z', x: 3, y: 0 }, { k: 'm', g: 'z', x: 4, y: 0 }], { x: 0, y: 0 }), 0.5, 'two monsters nearby: half');
  eq(dangerAt([{ k: 'm', g: 'K', x: 3, y: 0 }], { x: 0, y: 0 }), 1, 'a boss is a fight');
  eq(dangerAt([{ k: 'm', g: 'z', x: 60, y: 0 }, { k: 'm', g: 'B', x: 1, y: 0 }], { x: 0, y: 0 }), 0, 'far monsters and kegs do not count');
}

// --- story and gods: inscriptions open a page and land in the journal; a trial replaces the XP bar
{
  const st = new GameState();
  st.youId = 1;
  st.handle({ t: 'events', tick: 1, list: [{ k: 'lore', actor: 1, x: 0, y: 0, amount: 3, text: 'Scratched into a coffin lid.' }] }, 1000);
  eq(st.lore.get(3), 'Scratched into a coffin lid.', 'reading an inscription keeps the page');
  st.handle({ t: 'events', tick: 1, list: [{ k: 'lore', actor: 1, x: 0, y: 0, amount: 0, text: 'Spoonhollow.' }] }, 1000);
  const j = journalQuests(st);
  eq(j.map((q) => q.id), ['lore:3', 'lore:0'], 'the journal lists story pages in floor order (the village after floor 5)');
  st.party = [{ id: 1, name: 'vela', cls: 'rogue', lvl: 4, hp: 10, hpMax: 10, res: 0, resMax: 100, connected: true }];
  st.me = {
    res: 0, resMax: 100, cd: [0, 0, 0], cdMax: [1, 1, 1], cost: [0, 0, 0], potions: 0, potCd: 0, gold: 0, xp: 500, xpNeed: 100, lvl: 4,
    pack: [], packCap: 12, binds: ['stab', '', ''], tree: [], points: 0, respec: false, trial: { god: 'Vess Who Walks Unseen', need: 10, kills: 4 },
  };
  const row = (fb: Fb, y: number) => Array.from({ length: 160 }, (_, x) => fb.ch[y * 160 + x]).join('');
  let fb = new Fb(160, 48, [0, 0, 0]);
  hotbar(fb, st, 'rogue', []);
  eq(row(fb, 43).includes("Vess's trial: 4/10 foes"), true, 'the hotbar shows the trial instead of the XP bar');
  st.me.trial = { god: 'Vess Who Walks Unseen', need: 10, kills: 10 };
  fb = new Fb(160, 48, [0, 0, 0]);
  hotbar(fb, st, 'rogue', []);
  eq(row(fb, 43).includes('pray at an altar'), true, 'a finished hunt says to pray');
  st.handle({ t: 'events', tick: 1, list: [{ k: 'trial', actor: 1, x: 0, y: 0, amount: 10, fx: 'Vess Who Walks Unseen', text: 'Every shadow has a price.' }] }, 1000);
  eq(st.log.some((l) => l.segs.some((s) => s[0].includes('slay 10 foes'))), true, 'the trial is explained in the log');
  eq(loreTitle(0), 'SPOONHOLLOW', 'the village page has its name');
}

// --- phones: the view crop, the touch stick, the zoomed camera and HUD, tapping the landing form
{
  const box = { left: 100, top: 50, width: 800, height: 240 };
  eq(viewCell({ x: 0, y: 0, w: 160, h: 48 }, box, 100, 50), [0, 0], 'full view: top left corner');
  eq(viewCell({ x: 0, y: 0, w: 160, h: 48 }, box, 899, 289), [159, 47], 'full view: bottom right corner');
  eq(viewCell({ x: 34, y: 4, w: 48, h: 27 }, box, 500, 170), [58, 17], 'a cropped view maps into its rectangle');
  eq(stickKeys(5, 5), [], 'stick: small moves are ignored');
  eq(stickKeys(40, 0), ['ArrowRight'], 'stick: right');
  eq(stickKeys(30, 30), ['ArrowRight', 'ArrowDown'], 'stick: down-right is a diagonal');
  eq(stickKeys(0, -40), ['ArrowUp'], 'stick: up');
  eq(stickKeys(-30, -30), ['ArrowLeft', 'ArrowUp'], 'stick: up-left');
  eq(stickKeys(-40, 2), ['ArrowLeft'], 'stick: left');
  eq(stickKeys(30, 30, true).length, 1, 'stick in menus: one arrow at a time');
  eq(camAxis(5, 40, 80, 176), 0, 'camera stops at the left edge');
  eq(camAxis(100, 40, 80, 176), 60, 'camera centers the hero');
  eq(camAxis(175, 40, 80, 176), 96, 'camera stops at the right edge, the hero stays on screen');
  eq(landscapeScale(390, 844) < 0.6, true, 'a phone gets the zoomed layout');
  eq(landscapeScale(768, 1024) >= 0.6, true, 'a tablet keeps the full grid');
  eq(compactCols(844, 390), 104, 'zoomed view width follows a wide phone');
  eq(compactCols(400, 400), 64, 'zoomed view is at least 64 columns');
  const land = Object.create(LandingScreen.prototype) as LandingScreen;
  land.fixedCode = null;
  eq(land.fieldAt(60, 19), 'name', 'tap on the name row');
  eq(land.fieldAt(60, 22), 'class', 'tap just under the class row');
  eq(land.fieldAt(60, 24), 'new', 'tap on NEW GAME');
  eq(land.fieldAt(60, 30), 'join', 'tap on JOIN');
  eq(land.fieldAt(20, 19), null, 'tap beside the form');
  land.fixedCode = 'ASH-7Q2';
  eq(land.fieldAt(60, 24), 'join', 'invite link: JOIN sits under the class');

  const st = new GameState();
  st.youId = 1;
  st.party = [{ id: 1, name: 'vela', cls: 'rogue', lvl: 3, hp: 7, hpMax: 10, res: 40, resMax: 100, connected: true },
    { id: 2, name: 'bo', cls: 'monk', lvl: 3, hp: 0, hpMax: 10, res: 0, resMax: 100, connected: true, downed: true }];
  st.me = { res: 40, resMax: 100, cd: [0, 0, 0], cdMax: [1, 1, 1], cost: [0, 0, 0], potions: 1, potCd: 0, gold: 77, xp: 5, xpNeed: 10, lvl: 3,
    pack: [], packCap: 12, binds: ['stab', '', ''], tree: [], points: 0, respec: false };
  st.pushLog([['You found a key', [255, 255, 255]]], 1000);
  const fb = new Fb(160, 48, [0, 0, 0]);
  const row = (y: number, w = 160) => Array.from({ length: w }, (_, x) => fb.ch[y * 160 + x]).join('');
  const rat: Ent = { id: 30001, k: 'm', g: 'r', x: 9, y: 5, face: 0, name: 'cave rat', hp: 3, hpMax: 6 };
  compactHud(fb, st, 104, 24, rat, { chatActive: false, chatText: '' }, 2000);
  eq(row(0).includes('7/10'), true, 'zoomed HUD: your HP');
  eq(row(0).includes('LV 3') && row(0).includes('77g'), true, 'zoomed HUD: level and gold');
  eq(row(1).includes('cave rat'), true, 'zoomed HUD: the target');
  eq(row(1).includes('▼'), true, 'zoomed HUD: a downed ally');
  eq(row(23).includes('You found a key'), true, 'zoomed HUD: the newest log line');
  eq(row(0).slice(104).trim() + row(23).slice(104).trim(), '', 'zoomed HUD stays inside its columns');
  compactHud(fb, st, 104, 24, null, { chatActive: true, chatText: 'hi all' }, 2000);
  eq(row(23).includes('» hi all'), true, 'zoomed HUD: the chat being typed');
}

// --- tapping panels: which row or box is under a finger
{
  eq(treeSelAt(40, 6), 0, 'tree: top left skill box');
  eq(treeSelAt(70, 6), 1, 'tree: top right skill box');
  eq(treeSelAt(40, 16), 4, 'tree: bottom left skill box');
  eq(treeSelAt(40, 10), -1, 'tree: the gap between tiers');
  eq(treeSelAt(33, 6), -1, 'tree: the tier labels');
  eq(questRowAt(3, 40, 6), 0, 'quests: first row');
  eq(questRowAt(3, 40, 8), 2, 'quests: last row');
  eq(questRowAt(3, 40, 9), -1, 'quests: below the list');
  const st = new GameState();
  st.me = { res: 0, resMax: 1, cd: [0, 0, 0], cdMax: [1, 1, 1], cost: [0, 0, 0], potions: 0, potCd: 0, gold: 0, xp: 0, xpNeed: 1, lvl: 1,
    pack: [{ item: 'potion', n: 1 }], packCap: 12, binds: ['', '', ''], tree: [], points: 0, respec: false };
  eq(shopAt(st, 40, 6), { kind: 'tab', side: 0, sel: 0 }, 'shop: the BUY header');
  eq(shopAt(st, 62, 6), { kind: 'tab', side: 1, sel: 0 }, 'shop: the SELL header');
  eq(shopAt(st, 40, 9), { kind: 'row', side: 0, sel: 1 }, 'shop: a buy row');
  eq(shopAt(st, 62, 8), { kind: 'row', side: 1, sel: 0 }, 'shop: the one thing to sell');
  eq(shopAt(st, 62, 9), null, 'shop: below the sell list');
  eq(shopAt(st, 40, 8 + SHOP.length), null, 'shop: past the goods, no gear on offer');
}

// --- the run summary on the wipe and cleared screens, and the death recap in the log
{
  const st = new GameState();
  st.youId = 1;
  st.handle({ t: 'overlay', kind: 'cleared', summary: [
    { id: 1, name: 'vela', cls: 'rogue', lvl: 9, title: 'the Butcher', dmg: 23456, kills: 140, taken: 900, downs: 2, revives: 1, potions: 7, gold: 520, bestHit: 88, lastDowned: "an Imp's firebolt" },
    { id: 2, name: 'bo', cls: 'monk', lvl: 8, title: 'the Lifeline', dmg: 9000, kills: 60, taken: 1200, downs: 0, revives: 4, potions: 2, gold: 100, bestHit: 40 },
  ] }, 1000);
  eq(st.overlay, 'cleared', 'the overlay arrives');
  eq(st.summary.length, 2, 'with the summary');
  const fb = new Fb(160, 48, [0, 0, 0]);
  overlayBox(fb, 'cleared', st, 1000);
  const b = OVERLAY_BOX;
  const row = (y: number) => Array.from({ length: 160 }, (_, x) => fb.ch[y * 160 + x]).join('');
  eq(row(b.y + 7).includes('vela the Butcher') && row(b.y + 7).includes('23.5k') && row(b.y + 7).includes('140'), true, 'summary: a hero with title and numbers');
  eq(row(b.y + 8).includes("last downed by an Imp's firebolt"), true, 'summary: what downed them last');
  eq(row(b.y + 10).includes('never downed'), true, 'summary: a hero who never went down');
  for (let y = b.y; y < b.y + b.h; y++) eq(row(y).slice(0, b.x).trim() + row(y).slice(b.x + b.w).trim(), '', 'summary stays in its box (row ' + y + ')');
  eq([short(999), short(10000), short(123456)], ['999', '10.0k', '123.5k'], 'big numbers are shortened');
  st.party = [{ id: 1, name: 'vela', cls: 'rogue', lvl: 1, hp: 0, hpMax: 10, res: 0, resMax: 100, connected: true }];
  st.handle({ t: 'events', tick: 5, list: [{ k: 'down', target: 1, x: 0, y: 0, text: 'a spike trap', amount: 12 }] }, 2000);
  const last = st.log[st.log.length - 1].segs.map((g) => g[0]).join('');
  eq(last, 'vela is DOWN! · a spike trap · 12 dmg', 'the log says what downed a hero');
}

// --- floor events: told on arrival, the goblin chase on the HUD, the dead stir in the log
{
  const st = new GameState();
  st.youId = 1;
  const lvl = { t: 'level' as const, w: 176, h: 62, seed: 'x', torches: [], stairs: [1, 1] as [number, number], start: [2, 2] as [number, number],
    depth: 3, maxDepth: 10, theme: 'crypt', glows: [], event: 'goblin', eventText: 'A Treasure Goblin is somewhere on this floor.' };
  st.handle(lvl, 1000);
  const text = (l: { segs: [string, unknown, unknown?][] }) => l.segs.map((g) => g[0]).join('');
  eq(st.log.some((l) => text(l).includes('A Treasure Goblin is somewhere')), true, 'the event is told on arrival');
  eq(eventNote(st, 1000)!.map((g) => g[0]).join(''), '$ Treasure Goblin hiding somewhere', 'the goblin hides until it sees you');
  st.handle({ t: 'events', tick: 1, list: [{ k: 'goblin', target: 9, x: 0, y: 0 }] }, 2000);
  eq(eventNote(st, 12000)!.map((g) => g[0]).join(''), '$ Treasure Goblin 20s to catch it', 'then the chase is on');
  st.handle({ t: 'events', tick: 2, list: [{ k: 'escape', target: 9, x: 0, y: 0 }] }, 3000);
  eq(st.goblin, 'escaped', 'it can get away');
  st.handle({ ...lvl, event: 'stir', eventText: 'The dead stir.' }, 4000);
  eq(st.goblin, 'hiding', 'a new floor forgets the old goblin');
  const n = st.log.length;
  st.handle({ t: 'events', tick: 3, list: [{ k: 'summon', target: 50, x: 0, y: 0, fx: 'stir', text: 'Zombie' }, { k: 'summon', target: 51, x: 0, y: 0, fx: 'stir', text: 'Skeleton' },
    { k: 'stir', target: 1, x: 0, y: 0, amount: 2 }] }, 5000);
  eq(st.log.length - n, 1, 'a wave of the dead is one log line');
  eq(text(st.log[st.log.length - 1]), 'The dead stir: 2 claw their way up', 'and it says how many');
  st.handle({ ...lvl, event: undefined, eventText: undefined }, 6000);
  eq(eventNote(st, 6000), null, 'no event, no line');
}

{ // Hall of Fame and the new unique effects
  eq(fameTime(2527), '42:07', 'minutes and seconds');
  eq(fameTime(3753), '1:02:33', 'hours when it took that long');
  eq(fameNames(['kmerz', 'nyx'], 40).join(' '), 'kmerz nyx', 'short names stay whole');
  const four = fameNames(['abcdefghijkl', 'abcdefghijkl', 'abcdefghijkl', 'abcdefghijkl'], 40);
  eq(four.join(' ').length <= 40, true, 'four long names share one line');
  eq(modText({ k: 'execute', v: 50 }), '+50% damage to badly hurt monsters', 'execute reads');
  eq(modText({ k: 'gold', v: 50 }), '+50% gold found', 'gold reads');
  const st = new GameState();
  st.handle({ t: 'overlay', kind: 'cleared', summary: [] }, 0);
  st.handle({ t: 'events', tick: 1, list: [{ k: 'fame', x: 0, y: 0, amount: 3 }] }, 10);
  eq(st.fameRank, 3, 'the run made the Hall of Fame at #3');
  const line = st.log[st.log.length - 1].segs.map((g) => g[0]).join('');
  eq(line, 'The run enters the Hall of Fame at #3', 'and the log says so');
}

{ // Bramwick's forge and the new quest items
  eq(temperPrice({ lvl: 6, rar: 0 }), 102, 'common to magic at level 6 costs a magic piece\'s worth');
  eq(temperPrice({ lvl: 6, rar: 1 }), 272, 'magic to rare');
  eq(reforgePrice({ lvl: 6, rar: 2 }), 136, 'a reforge costs half the value');
  const st = new GameState();
  const g = (rar: number) => ({ slot: 'weapon' as const, base: 'axe', name: 'Axe', rar, lvl: 6, mods: [{ k: 'dmg', v: 20 }], value: 0 });
  st.me = { weapon: g(1), gold: 500 } as unknown as typeof st.me;
  const rows = forgeRows(st);
  eq(rows.map((r) => r.id).join(','), 'temper0,temper1,reforge0,reforge1', 'four jobs in order');
  eq(rows[0].price, 272, 'temper the magic weapon');
  eq(rows[1].why, 'nothing worn', 'no trinket to temper');
  eq(rows[2].price, 51, 'reforge the magic weapon');
  st.me = { weapon: g(3), trinket: g(0) } as unknown as typeof st.me;
  const r2 = forgeRows(st);
  eq(r2[0].why, "he won't touch a unique", 'uniques are left alone');
  eq(r2[3].why, 'no bonuses to reforge', 'a common piece has nothing to reforge');
  eq(forgeAt(FORGE_BOX.x + 5, FORGE_BOX.y + 13), 2, 'tap the third job');
  eq(forgeAt(FORGE_BOX.x + 5, FORGE_BOX.y + 3), -1, 'the gear lines are not jobs');
  for (const id of ['batwing', 'gnomebell', 'abysssilk']) eq(itemInfo(id).name !== id, true, id + ' has a name');
}

{ // the game menu
  eq(padLabel('world', 'Escape'), 'START', 'START opens the menu');
  eq(padLabel('world', 'j'), 'L3', 'the journal moved to L3');
  const host = menuRows(true, 'all', false), guest = menuRows(false, 'off', true);
  eq(host.map((r) => r.id).join(','), 'resume,journal,sound,controls,save,leave', 'menu rows in order');
  eq(!!host[4].off, false, 'the host can save');
  eq(!!guest[4].off, true, 'a guest cannot');
  eq(guest[2].label, 'Sound: off', 'the sound row says the mode');
  eq(guest[5].label.startsWith('Leave?'), true, 'leaving asks again');
  eq(menuAt(MENU_BOX.x + 6, MENU_BOX.y + 3, 6), 0, 'tap the first row');
  eq(menuAt(MENU_BOX.x + 6, MENU_BOX.y + 13, 6), 5, 'tap the last row');
  eq(menuAt(MENU_BOX.x + 6, MENU_BOX.y + 4, 6), -1, 'between rows is nothing');
}

{ // rebinding keys and buttons
  setBinds(defaults(), false);
  eq(translateKey('w'), 'ArrowUp', 'W walks up');
  eq(translateKey('W'), 'ArrowUp', 'with caps lock too');
  eq(translateKey('F5'), 'F5', 'keys the game never uses pass through');
  bindKey('potion', 0, 'f');
  eq(translateKey('f'), 'q', 'F is now the potion');
  eq(translateKey('q'), null, 'and Q does nothing');
  eq(kbLabel('q'), 'F', 'hints show the new key');
  bindKey('skill1', 0, 'f');
  eq(binds.kb.skill1.join(), 'f', 'skill 1 takes F');
  eq(binds.kb.potion.join(), '1', '...and the potion gets skill 1\'s old key (a swap)');
  bindKey('up', 1, 'i');
  eq(binds.kb.up.join(), 'w,i', 'the second key of an action');
  eq(binds.kb.bag.join(), 'arrowup', 'the rucksack gets the arrow it lost');
  eq(translateKey('ArrowUp', true), 'i', 'in a panel, panel toggles translate (here: the rucksack on ↑)...');
  eq(translateKey('i', true), 'i', '...other bindings do not');
  eq(translateKey('x', true), 'x', '...a panel\'s own letters stay as they are');
  bindKey('menu', 0, 'p');
  eq(binds.kb.menu.join(), 'escape', 'the menu stays on ESC');
  bindPad('potion', BTN.A);
  eq(padWorldMap()[BTN.A], 'q', 'A drinks a potion now');
  eq(padWorldMap()[BTN.LB], '1', '...and LB is skill 1 (a swap)');
  bindPad('skill2', BTN.START);
  eq(padWorldMap()[BTN.START], 'Escape', 'START stays the menu');
  setBinds(defaults(), false);
  eq(cellEditable(0, 2), false, 'moving has no pad button (the stick)');
  eq(cellEditable(4, 2), true, 'skill 1 has one');
  eq(controlsAt(CONTROLS_BOX.x + 5, CONTROLS_BOX.y + 4)?.row, 0, 'tap the first action');
  eq(controlsAt(CONTROLS_BOX.x + 66, CONTROLS_BOX.y + 8)?.col, 2, 'tap a pad cell');
}

{ // aiming with the right stick, or the way you face
  const m = (id: number, x: number, y: number) => ({ id, k: 'm', g: 'z', x, y } as unknown as Ent);
  const ents = [m(1, 15, 10), m(2, 10, 14), m(3, 12, 10)];
  eq(coneTarget(ents, 10, 10, [1, 0], 0)?.id, 3, 'the closest enemy to the right');
  eq(coneTarget(ents, 10, 10, [1, 0], 1)?.id, 1, 'the one last hit while it is in the cone');
  eq(coneTarget(ents, 10, 10, [0, 1], 0)?.id, 2, 'down');
  eq(coneTarget(ents, 10, 10, [-1, 0], 0), null, 'nothing to the left');
  eq(coneTarget([{ ...m(4, 11, 10), g: 'B' } as Ent], 10, 10, [1, 0], 0), null, 'never a powder keg');
  eq(step8([1, 0]), [1, 0], 'right is one step right');
  eq(step8([1, 2]), [1, 1], 'a screen diagonal is a cell diagonal');
  eq(step8([0, -1]), [0, -1], 'up');
  eq(aheadCell(10, 10, [0, 1], 8), [10, 14], 'eight columns down is four rows');
  // the pad: the right stick aims, and rebinding catches the next new press
  let cur: PadLike = { connected: true, axes: [0, 0, 0.9, 0], buttons: Array.from({ length: 17 }, () => ({ pressed: false })) };
  const gp = new GamepadInput(() => [cur]);
  const sink = { down: () => {}, up: () => {} };
  gp.poll(0, 'world', sink);
  eq(padAim.on && padAim.x > 0.8, true, 'the right stick aims');
  const press = (i: number[]) => { cur = { ...cur, axes: [0, 0, 0, 0], buttons: Array.from({ length: 17 }, (_, j) => ({ pressed: i.includes(j) })) }; };
  press([0]);
  gp.poll(10, 'panel', sink);
  let got = -1;
  gp.startCapture((b) => { got = b; });
  gp.poll(20, 'panel', sink);
  eq(got, -1, 'the button still held from opening the capture does not count');
  press([0, 3]);
  gp.poll(30, 'panel', sink);
  eq(got, 3, 'the next new press is caught (Y)');
  eq(gp.capturing, false, 'and capture ends');
  eq(padAim.on, false, 'a stick at rest does not aim');
}

if (failed) { console.error(failed + ' test(s) failed'); process.exit(1); }
console.log('all client tests passed');
process.exit(0);
