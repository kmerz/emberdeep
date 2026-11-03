// HUD: a port of design/Main.dc.html lines 285-396 driven by real state.
import { Fb, RGB, Seg, DIM, TX, WH, PINK, GOLD, MINT, PAN, W, mul, mix, ramp, c01 } from '../term/fb';
import { GameState, LW, LH, clock, EVENT_COL } from '../state';
import { CLS, SKILL_BY_ID } from '../classes';
import type { Ent, HeroSummary, PartyEntry } from '../proto';
import type { Cam } from './fx';
import { VW, VH } from './map';
import { palette } from './themes';
import { itemInfo } from '../items';
import { keyLabel, lastInput } from '../gamepad';

export interface UiState { chatActive: boolean; chatText: string; }

export function statusBar(fb: Fb, st: GameState) {
  fb.fill(0, 0, W, 1, [17, 16, 27]);
  fb.segs(0, 0, [
    [' ◆ EMBERDEEP ', [22, 6, 20], PINK, 1],
    [' ' + palette(st.level?.theme).name + ' ', [236, 228, 255], [44, 36, 68], 1],
    [' Depth ' + String(st.level?.depth ?? 1).padStart(2, '0') + ' ', [196, 186, 230], [32, 28, 50]],
    [' seed ' + (st.level?.seed ?? '----') + ' ', [130, 124, 160], [22, 21, 34]],
  ]);
  const n = st.party.filter((p) => p.connected).length;
  const right: [string, RGB, RGB | null, number?][] = [
    [' ● ' + n + '/4 in party ', MINT, [22, 21, 34], 1],
    [' ' + clock(st.tick) + ' ', [196, 186, 230], [32, 28, 50]],
    [' ' + st.pingMs + ' ms ', [196, 186, 230], [44, 36, 68]],
    [' 24-bit ', [8, 20, 28], [95, 215, 255], 1],
  ];
  fb.segs(W - right.reduce((a, r) => a + r[0].length, 0), 0, right);
}

function statusSegs(p: PartyEntry): [string, RGB, (RGB | null)?, number?][] {
  const out: [string, RGB, (RGB | null)?, number?][] = [];
  if (p.dead) return [['✖ FALLEN', PINK, null, 1]];
  if (p.downed) return [['▼ DOWNED ' + (p.downT ?? 0) + 's', PINK, null, 1]];
  if (!p.connected) return [['… reconnecting', [255, 176, 70]]];
  for (const s of p.st ?? []) {
    if (s.k === 'wall') out.push(['▲ Iron Wall', GOLD], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'bell') out.push(['◆ Bell of Warding', GOLD], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'slow') out.push(['▼ Slowed', [150, 196, 106]], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'torch') out.push(['¡ Lantern', [255, 170, 70]], [' ' + Math.floor(s.t / 60) + ':' + String(s.t % 60).padStart(2, '0'), DIM], ['   ', DIM]);
    else if (s.k === 'cry') out.push(['! War Cry', [255, 138, 61]], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'hidden') out.push(['░ Hidden', [199, 146, 255]], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'poison') out.push(['~ Poison', [126, 240, 178]], [' ×' + s.t, DIM], ['   ', DIM]);
    else if (s.k === 'haste') out.push(['» Haste', [140, 230, 255]], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'fury') out.push(['‼ Fury', [255, 90, 60]], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'blood') out.push(['♥ Blood', [220, 40, 70]], [' ' + s.t + 's', DIM], ['   ', DIM]);
    else if (s.k === 'charmed') out.push(['♥ Charmed', [255, 110, 200]], [' ' + s.t + 's', DIM], ['   ', DIM]);
  }
  if (!out.length) out.push(['◇ steady', DIM]);
  return out;
}

export function partyPanel(fb: Fb, st: GameState) {
  fb.box(113, 1, 47, 18, 'PARTY · ' + st.party.length + '/4');
  st.party.slice(0, 4).forEach((p, i) => {
    const y = 2 + i * 4, x = 115, c = CLS[p.cls];
    const dim = p.dead || p.downed;
    const col = dim ? mul(c.col, 0.5) : c.col;
    fb.put(x, y, '@', col, mul(c.col, 0.3), 1);
    fb.txt(x + 2, y, p.name, dim ? DIM : WH, null, 1);
    let cx2 = x + 3 + p.name.length;
    fb.txt(cx2, y, c.name, mul(c.col, 0.9)); cx2 += c.name.length + 1;
    if (p.id === st.youId) fb.txt(cx2, y, ' YOU ', [24, 10, 6], c.col, 1);
    const lv = 'LV ' + p.lvl;
    fb.txt(159 - lv.length, y, lv, DIM);
    fb.txt(x, y + 1, 'HP', DIM);
    fb.bar(x + 3, y + 1, 30, p.hp / Math.max(1, p.hpMax), p.downed ? [140, 40, 80] : [255, 77, 109], [40, 20, 34]);
    fb.txt(150, y + 1, (p.hp + '/' + p.hpMax).padStart(9), TX);
    fb.txt(x, y + 2, c.rl, DIM);
    fb.bar(x + 3, y + 2, 30, p.res / Math.max(1, p.resMax), c.resCol, mul(c.resCol, 0.14));
    fb.txt(150, y + 2, (p.res + '/' + p.resMax).padStart(9), TX);
    fb.segs(x, y + 3, statusSegs(p));
  });
}

export function pickTarget(st: GameState, ents: Ent[], hero: { x: number; y: number } | null): Ent | null {
  const mons = ents.filter((e) => e.k === 'm');
  const last = mons.find((e) => e.id === st.lastTarget);
  if (last) return last;
  if (!hero) return null;
  let best: Ent | null = null, bd = 14;
  for (const m of mons) {
    if (m.g === 'B') continue; // a powder keg is only a target once you hit it
    const d = Math.hypot(m.x - hero.x, (m.y - hero.y) * 2);
    if (d < bd) { best = m; bd = d; }
  }
  return best;
}

export function targetPanel(fb: Fb, st: GameState, t: Ent | null) {
  fb.box(113, 19, 47, 7, 'TARGET', PINK);
  if (!t) { fb.txt(115, 21, 'no target', DIM); return; }
  const frac = (t.hp ?? 0) / Math.max(1, t.hpMax ?? 1);
  const mc = t.el ? PINK : [214, 208, 234];
  fb.put(115, 20, t.g, t.el ? [255, 61, 110] : mc, t.el ? [64, 10, 32] : null, 1);
  fb.txt(117, 20, (t.name || '').toUpperCase(), t.el ? [255, 120, 160] : TX, null, 1);
  if (t.el) fb.txt(152, 20, ' ELITE ', [24, 6, 16], PINK, 1);
  fb.txt(115, 21, (t.hp ?? 0) + ' / ' + (t.hpMax ?? 0) + ' HP', [255, 150, 90]);
  fb.bar(115, 22, 36, frac, [255, 61, 110], [40, 14, 26]);
  fb.txt(152, 22, (Math.round(frac * 100) + '%').padStart(4), WH, null, 1);
  const stt = (t.st ?? []).map((s) => s.k).join(' · ');
  fb.txt(115, 23, stt || (t.el ? 'slay it to open the way' : 'hostile'), DIM);
}

export function minimap(fb: Fb, st: GameState, cam: Cam, ents: Ent[], now: number) {
  fb.box(113, 26, 47, 12, 'MAP · D01');
  const MC4: RGB[] = [PAN, [40, 37, 58], [70, 64, 100], [116, 104, 156]];
  const ov = new Map<number, RGB>();
  const mpx = (lx: number, ly: number) => Math.floor((ly * 20) / LH) * 45 + Math.floor((lx * 45) / LW);
  const cls: number[] = new Array(45 * 20).fill(0);
  const fl = new Int16Array(900), wl = new Int16Array(900), vi = new Int16Array(900);
  for (let y = 0; y < LH; y++) for (let x = 0; x < LW; x++) {
    const t = st.tiles[y * LW + x];
    if (!t) continue;
    const k = mpx(x, y);
    if (t === 35 || t === 84 || t === 79) wl[k]++;
    else { fl[k]++; if (st.vis[y * LW + x]) vi[k]++; }
  }
  for (let k = 0; k < 900; k++) cls[k] = vi[k] > 0 ? 3 : fl[k] > 0 ? 2 : wl[k] > 0 ? 1 : 0;
  const lv = st.level;
  if (lv && st.tileAt(lv.stairs[0], lv.stairs[1])) ov.set(mpx(lv.stairs[0], lv.stairs[1]), [95, 230, 255]);
  if (lv?.up && st.tileAt(lv.up[0], lv.up[1])) ov.set(mpx(lv.up[0], lv.up[1]), [160, 220, 150]);
  for (const e of ents) if (e.k === 'm' && e.el) ov.set(mpx(e.x, e.y), [255, 61, 110]);
  for (const p of st.pings) {
    const age = (now - p.t0) / 1000;
    if (age < 4) { const c = CLS[p.cls].col; ov.set(mpx(p.x, p.y), Math.floor(age * 3) % 2 ? mul(c, 0.5) : c); }
  }
  for (const e of ents) {
    if (e.k !== 'h') continue;
    const c = CLS[e.cls!].col;
    if (e.downed && Math.floor(now / 250) % 2 === 0) continue; // downed heroes blink at 2 Hz
    ov.set(mpx(e.x, e.y), e.dead ? mul(c, 0.3) : e.downed ? mul(c, 0.55) : c);
  }
  const vx0 = Math.floor((cam.x * 45) / LW), vx1 = Math.floor(((cam.x + VW - 1) * 45) / LW);
  const vy0 = Math.floor((cam.y * 20) / LH), vy1 = Math.floor(((cam.y + VH - 1) * 20) / LH);
  const pix = (px: number, py: number): RGB => {
    const k = py * 45 + px;
    const o = ov.get(k);
    if (o) return o;
    const col = MC4[cls[k]];
    const edge = ((px === vx0 || px === vx1) && py >= vy0 && py <= vy1) || ((py === vy0 || py === vy1) && px >= vx0 && px <= vx1);
    return edge ? mix(col, [200, 190, 240], 0.55) : col;
  };
  for (let r = 0; r < 10; r++) for (let c = 0; c < 45; c++) fb.put(114 + c, 27 + r, '▀', pix(c, r * 2), pix(c, r * 2 + 1));
}

export function objectives(fb: Fb, st: GameState) {
  fb.box(113, 38, 47, 9, 'OBJECTIVES');
  const o = st.obj;
  if (!o) return;
  if (o.village) {
    fb.txt(115, 39, '◆ Trade with Grimble (walk into him)', TX);
  } else if (o.boss) {
    fb.txt(115, 39, ('◆ Slay ' + (o.bossName || 'the boss')).slice(0, 40), TX);
    fb.txt(156, 39, o.kingAlive ? '0/1' : '1/1', o.kingAlive ? DIM : MINT);
  } else {
    const d = st.level?.depth ?? 1, max = st.level?.maxDepth ?? 10;
    fb.txt(115, 39, '◆ Reach the bottom', TX);
    fb.txt(160 - 2 - (d + '/' + max).length, 39, d + '/' + max, DIM);
  }
  fb.txt(115, 40, '◆ Find the stairs', TX);
  fb.txt(o.stairsFound ? 154 : 157, 40, o.stairsFound ? 'found' : '?', o.stairsFound ? [95, 230, 255] : DIM);
  const need = o.stairsNeed, all = o.stairsAt >= need && need > 0;
  fb.txt(115, 41, (all ? '◆ ' : '◇ ') + o.stairsAt + '/' + need + (o.stairsUp ? ' at the stairs up' : ' at the stairs'), all ? TX : DIM);
  if (o.stairsLeft > 0) {
    const t = (o.stairsUp ? 'up ' : 'down ') + (o.stairsLeft / 20).toFixed(1);
    fb.txt(159 - t.length, 41, t, GOLD, null, 1);
  }
  if (o.portalNeed) fb.segs(115, 42, [['◎ ', [150, 230, 255]], [o.portalAt + '/' + o.portalNeed + ' at the portal', TX]]);
  else {
    const active = st.quests.filter((q) => q.state === 'active').length;
    if (active) fb.segs(115, 42, [['◆ ', GOLD], [active + (active === 1 ? ' quest' : ' quests'), TX], [' (J)', DIM]]);
    else if (st.level?.depth === 1 && st.quests.some((q) => q.giver === 'board' && q.state === 'offered')) fb.segs(115, 42, [['◇ ', DIM], ['read the notice board ', DIM], ['≡', GOLD]]);
  }
  fb.txt(115, 43, 'PARTY LOOT', DIM, null, 1);
  fb.segs(115, 44, [[String(o.gold), GOLD, null, 1], [' gold  ·  ', DIM], [String(o.kills), TX, null, 1], [' slain', DIM]]);
  const ev = eventNote(st, performance.now());
  if (ev) fb.segs(115, 45, ev);
}

/** One line about the floor event for the HUD (null = none): what it is, and how the goblin chase goes. */
export function eventNote(st: GameState, now: number): Seg[] | null {
  const ev = st.level?.event ?? '', col = EVENT_COL[ev] ?? GOLD;
  if (ev === 'goblin') {
    const g = st.goblin;
    const left = Math.max(0, Math.ceil((st.goblinUntil - now) / 1000));
    return [['$ ', col, null, 1], ['Treasure Goblin ', TX], g === 'running' ? [left + 's to catch it', PINK, null, 1]
      : g === 'caught' ? ['caught!', MINT, null, 1] : g === 'escaped' ? ['escaped', DIM] : ['hiding somewhere', DIM]];
  }
  if (ev === 'stir') return [['☠ ', col, null, 1], ['The dead stir', TX]];
  if (ev === 'cursed') return [['✠ ', col, null, 1], ['Cursed: ', TX], ['gold ×2', GOLD], [', monsters hit harder', DIM]];
  return null;
}

export function logPanel(fb: Fb, st: GameState, now: number) {
  fb.box(0, 35, 64, 12, 'LOG · PARTY CHAT');
  st.log.slice(-10).forEach((ln, i) => {
    const y = 36 + i;
    if (now - ln.at < 500) fb.fill(1, y, 62, 1, [34, 28, 52]);
    fb.txt(2, y, ln.ts, [88, 84, 118]);
    let lx = 8;
    for (const sg of ln.segs) {
      if (lx >= 62) break;
      const s = sg[0].slice(0, 62 - lx);
      fb.txt(lx, y, s, sg[1], null, sg[2] ?? (sg[1] === PINK ? 1 : 0));
      lx += s.length;
    }
  });
}

/** Hotbar slot width and where each slot starts (skills 1-3, then a gap, then potion and the two item keys). */
const SLOT_W = 7;
const slotX = (i: number) => 67 + i * SLOT_W + (i >= 3 ? 1 : 0);

export function hotbar(fb: Fb, st: GameState, youCls: keyof typeof CLS | null, itemKeys: string[] = []) {
  const me = st.party.find((p) => p.id === st.youId);
  if (!youCls || !st.me) { fb.box(65, 35, 47, 12, 'HOTBAR'); return; }
  const c = CLS[youCls], m = st.me;
  fb.box(65, 35, 47, 12, (me?.name ?? '').toUpperCase() + ' · ' + c.name, c.col);
  const wall = !!me?.st?.some((s) => s.k === 'wall');
  const count = (id: string) => (m.pack ?? []).reduce((a, s) => a + (s.item === id ? s.n : 0), 0);
  type Slot = { k: string; ic: string; n: string; st: number; sub: string; col?: RGB };
  const slots: Slot[] = [0, 1, 2].map((i) => {
    const s = SKILL_BY_ID[m.binds?.[i] ?? '']?.meta;
    if (!s) return { k: keyLabel('world', String(i + 1)), ic: ' · ', n: 'EMPTY', st: 2, sub: '(' + keyLabel('world', 't') + ')' };
    const cd = m.cd[i] ?? 0;
    let state = 0, sub = 'ready';
    if (s.id === 'ironwall' && wall) { state = 1; sub = (me!.st!.find((q) => q.k === 'wall')!.t) + 's'; }
    else if (cd > 0) { state = 2; sub = (cd / 20).toFixed(1) + 's'; }
    else if (m.res < (m.cost[i] ?? 0)) { state = 2; sub = 'no ' + c.rl; }
    else if ((m.cost[i] ?? 0) > 0) sub = m.cost[i] + ' ' + c.rl;
    return { k: keyLabel('world', String(i + 1)), ic: s.icon, n: s.short, st: state, sub };
  });
  // Q: a potion, or a draught when that helps (the server decides); shows both counts
  const drs = count('draught');
  slots.push({ k: keyLabel('world', 'q'), ic: ' ! ', n: 'POTION', st: m.potCd > 0 || m.potions + drs <= 0 ? 2 : 0,
    sub: m.potCd > 0 ? (m.potCd / 20).toFixed(1) + 's' : '×' + m.potions + (drs ? ' &' + drs : '') });
  ['4', '5'].forEach((key, i) => {
    const id = itemKeys[i] ?? '';
    if (!id) { slots.push({ k: keyLabel('world', key), ic: ' · ', n: 'ITEM', st: 2, sub: '(' + keyLabel('world', 'i') + ')' }); return; }
    const it = itemInfo(id), n = count(id);
    const cool = (id === 'potion' || id === 'draught') && m.potCd > 0;
    slots.push({ k: keyLabel('world', key), ic: ' ' + it.ch + ' ', n: it.short ?? it.name.toUpperCase(), st: n <= 0 || cool ? 2 : 0,
      sub: cool ? (m.potCd / 20).toFixed(1) + 's' : '×' + n, col: it.col });
  });
  slots.forEach((s, i) => {
    const x = slotX(i), y = 37, w = SLOT_W;
    const bc: RGB = s.st === 1 ? GOLD : s.st === 2 ? [52, 48, 72] : [100, 94, 136];
    const ic: RGB = s.st === 1 ? GOLD : s.st === 2 ? [78, 74, 100] : WH;
    const inner: RGB = s.st === 2 ? [20, 19, 30] : s.st === 1 ? [44, 36, 16] : PAN;
    fb.put(x, y, '╭', bc);
    for (let j = 1; j < w - 1; j++) fb.put(x + j, y, '─', bc);
    fb.txt(x + 1, y, s.k, s.st === 2 ? DIM : WH, null, 1); // a key or a pad button (up to 2 letters)
    fb.put(x + w - 1, y, '╮', bc);
    fb.put(x, y + 1, '│', bc);
    for (let j = 1; j < w - 1; j++) fb.put(x + j, y + 1, ' ', null, inner);
    fb.txt(x + 2, y + 1, s.ic, s.col && s.st !== 2 ? s.col : ic, null, 1);
    fb.put(x + w - 1, y + 1, '│', bc);
    fb.put(x, y + 2, '╰', bc);
    for (let j = 1; j < w - 1; j++) fb.put(x + j, y + 2, '─', bc);
    fb.put(x + w - 1, y + 2, '╯', bc);
    const nm = s.n.slice(0, w), sub = s.sub.slice(0, w);
    fb.txt(x + Math.floor((w - nm.length) / 2), y + 3, nm, s.st === 2 ? DIM : TX, null, s.st === 1 ? 1 : 0);
    fb.txt(x + Math.floor((w - sub.length) / 2), y + 4, sub, s.st === 1 ? GOLD : DIM);
  });
  if (m.trial) { // a god holds the next level: show the trial instead of the XP bar
    const t = m.trial, done = t.kills >= t.need, god = t.god.split(' ')[0];
    fb.segs(67, 43, done
      ? [['Ω ', [255, 236, 170], null, 1], [god + ' waits: pray at an altar', [255, 236, 170]]]
      : [['Ω ', [255, 236, 170], null, 1], [god + "'s trial: ", [255, 200, 90]], [t.kills + '/' + t.need + ' foes', TX, null, 1], [' · XP kept', DIM]]);
  } else {
    fb.txt(67, 43, 'XP', DIM);
    fb.bar(70, 43, 34, Math.min(1, m.xp / Math.max(1, m.xpNeed)), [177, 120, 255], [30, 22, 48]);
    fb.txt(106, 43, (Math.min(100, Math.round((100 * m.xp) / Math.max(1, m.xpNeed))) + '%').padStart(4), TX);
  }
  if (m.oilMax) { // the lantern: it burns by itself on dark floors
    const sec = m.oil ?? 0, low = sec <= 30;
    fb.segs(96, 45, [['lantern ', DIM], [Math.floor(sec / 60) + ':' + String(sec % 60).padStart(2, '0'), low ? PINK : [255, 170, 70], null, low ? 1 : 0]]);
  }
  if ((m.points ?? 0) > 0) fb.segs(67, 45, [[String(m.points), GOLD, null, 1], [m.points === 1 ? ' skill point' : ' skill points', GOLD], [' to spend (' + keyLabel('world', 't') + ')', DIM]]);
  fb.segs(67, 44, [['LV ' + m.lvl, TX, null, 1], ['   gold ', DIM], [String(m.gold), GOLD, null, 1], ['   rucksack ', DIM], [(m.pack ?? []).filter((s) => s.n > 0).length + '/' + (m.packCap || 12), TX, null, 1], [' (' + keyLabel('world', 'i') + ')', DIM]]);
}

export function commandLine(fb: Fb, ui: UiState, now: number) {
  fb.fill(0, 47, W, 1, [14, 13, 22]);
  fb.txt(1, 47, '»', PINK, null, 1);
  if (ui.chatActive) {
    fb.txt(3, 47, ui.chatText, WH);
    if (Math.floor(now / 400) % 2 === 0) fb.put(3 + ui.chatText.length, 47, '█', PINK);
  } else {
    fb.txt(3, 47, 'press ENTER to chat · /help', [88, 84, 118]);
  }
  // a pad can't chat or ping, so it gets its own list
  const hints = lastInput.pad
    ? [['LT', 'revive (hold)'], ['LB', 'potion'], ['X Y RB', 'skills'], ['B A', 'items'], ['SEL', 'rucksack'], ['R3', 'skill tree']]
    : [['ENTER', 'chat'], ['E', 'revive (hold)'], ['Q', 'potion'], ['1-3', 'skills'], ['4 5', 'items'], ['Alt+Click', 'ping']];
  let hx = W - 1;
  for (let hi = hints.length - 1; hi >= 0; hi--) {
    const lab = ' ' + hints[hi][1] + '  ';
    hx -= lab.length;
    fb.txt(hx, 47, lab, DIM);
    const key = ' ' + hints[hi][0] + ' ';
    hx -= key.length;
    fb.txt(hx, 47, key, [24, 22, 36], [150, 144, 190], 1);
  }
}

/** "DEPTH N" in the middle of the map for a moment after a new floor arrives. */
export function depthBanner(fb: Fb, st: GameState, now: number, vw = VW) {
  if (!st.level || !st.levelAt) return;
  const age = now - st.levelAt;
  if (age > BANNER_MS) return;
  const pal = palette(st.level.theme);
  const fade = age < BANNER_MS - 700 ? 1 : c01((BANNER_MS - age) / 700);
  const txt = st.level.village ? ' SAFE HAVEN ' : ' DEPTH ' + String(st.level.depth).padStart(2, '0') + ' ';
  const name = pal.name.toUpperCase().split('').join(' ');
  const width = Math.max(name.length, pal.tagline.length) + 8;
  const x0 = Math.floor((vw - width) / 2);
  // a dark band behind the card so it reads over a lit room
  for (let y = 6; y <= 14; y++) for (let x = x0; x < x0 + width; x++) {
    const i = y * W + x;
    fb.setBg(i, mix(fb.getBg(i), [6, 5, 10], 0.75 * fade));
  }
  // the rules grow out from the depth label, the name types itself in, the flavor line follows
  const grow = c01(age / 400), rule = Math.floor(((width - txt.length) / 2 - 2) * grow);
  const tx = Math.floor((vw - txt.length) / 2);
  fb.txt(tx - rule - 1, 8, '─'.repeat(rule), mul(GOLD, 0.6 * fade));
  fb.txt(tx + txt.length + 1, 8, '─'.repeat(rule), mul(GOLD, 0.6 * fade));
  fb.txt(tx, 8, txt, mul([24, 6, 20], fade), mul(GOLD, fade), 1);
  const shown = name.slice(0, Math.floor(c01((age - 250) / 700) * name.length));
  fb.txt(Math.floor((vw - name.length) / 2), 10, shown, mul(WH, fade), null, 1);
  const tagA = c01((age - 1000) / 600) * fade;
  if (tagA > 0) fb.txt(Math.floor((vw - pal.tagline.length) / 2), 12, pal.tagline, mul(DIM, 0.4 + 0.8 * tagA), null, 0);
  const evName = EVENT_NAME[st.level.event ?? ''];
  const evA = c01((age - 1500) / 600) * fade;
  if (evName && evA > 0) fb.txt(Math.floor((vw - evName.length) / 2), 14, evName, mul(EVENT_COL[st.level.event!] ?? GOLD, evA), null, 1);
}

const BANNER_MS = 4500;
/** The floor event's line on the depth card. */
const EVENT_NAME: Record<string, string> = { goblin: '◆ A TREASURE GOBLIN LURKS ◆', stir: '◆ THE DEAD STIR ◆', cursed: '◆ CURSED FLOOR · DOUBLE GOLD ◆' };

export const OVERLAY_BOX = { x: 6, y: 8, w: 100, h: 20 };

/** Big numbers fit the summary columns: 12345 → 12.3k. */
export function short(n: number): string {
  return n >= 10000 ? (n / 1000).toFixed(1) + 'k' : String(n);
}

/** The columns of the summary table: header, x offset in the box, and the value. */
const SUMMARY_COLS: [string, number, (h: HeroSummary) => number][] = [
  ['LV', 40, (h) => h.lvl], ['KILLS', 45, (h) => h.kills], ['DAMAGE', 52, (h) => h.dmg], ['TAKEN', 60, (h) => h.taken],
  ['DOWNS', 67, (h) => h.downs], ['REVIVES', 74, (h) => h.revives], ['POTIONS', 83, (h) => h.potions], ['GOLD', 92, (h) => h.gold],
];

/** The wipe and cleared screens: what happened, every hero's run with a title, and what comes next. */
export function overlayBox(fb: Fb, kind: 'wipe' | 'cleared', st: GameState, now: number) {
  const { x, y, w, h } = OVERLAY_BOX;
  const wipe = kind === 'wipe';
  fb.box(x, y, w, h, wipe ? 'PARTY WIPED · THE RUN SO FAR' : 'DEPTH CLEARED · THE RUN', wipe ? PINK : MINT);
  const mid = (row: number, s: string, fg: RGB, b = 0) => fb.txt(x + Math.floor((w - s.length) / 2), row, s, fg, PAN, b);
  const blink = Math.floor(now / 500) % 2 === 0;
  if (wipe) {
    mid(y + 2, 'Every hero has fallen.', TX, 1);
    mid(y + 3, 'Depth ' + (st.level?.depth ?? 1) + ' can be retried, with everyone as they arrived.', DIM);
  } else {
    mid(y + 2, 'Depth ' + (st.level?.depth ?? 10) + ' cleared — the party reached the bottom.', TX, 1);
    mid(y + 3, 'Gold collected: ' + (st.obj?.gold ?? 0) + ' · slain: ' + (st.obj?.kills ?? 0) + (st.fameRank ? ' · Hall of Fame #' + st.fameRank : ''), GOLD);
  }
  const rows = st.summary.slice(0, 4);
  if (rows.length) {
    fb.txt(x + 3, y + 5, 'HERO', DIM, null, 1);
    for (const [head, cx] of SUMMARY_COLS) fb.txt(x + cx, y + 5, head, DIM, null, 1);
    rows.forEach((r, i) => {
      const ry = y + 7 + i * 2, c = CLS[r.cls], you = r.id === st.youId;
      if (you) fb.fill(x + 1, ry, w - 2, 1, [24, 21, 36]);
      fb.put(x + 3, ry, '@', c?.col ?? TX, null, 1);
      fb.segs(x + 5, ry, [[r.name, WH, null, 1], [' ' + r.title, GOLD]]);
      for (const [head, cx, val] of SUMMARY_COLS) {
        const v = short(val(r));
        fb.txt(x + cx + head.length - v.length, ry, v, val(r) ? TX : DIM, null, 0);
      }
      const note: [string, RGB][] = [['best hit ' + r.bestHit, DIM]];
      if (r.lastDowned) note.push([' · last downed by ', DIM], [r.lastDowned.slice(0, 50), [255, 150, 160]]);
      else note.push([' · never downed', MINT]);
      fb.segs(x + 5, ry + 1, note);
    });
  }
  const host = st.isHost;
  const next = wipe ? (host ? 'Press ENTER to restart the floor' : 'Waiting for the host to restart…')
    : host ? 'Press ENTER to return to the lobby' : 'Returning to the lobby…';
  mid(y + h - 2, next, blink ? (wipe ? PINK : MINT) : mul(wipe ? PINK : MINT, 0.6), 1);
  void ramp; void c01;
}

/** Columns the touch buttons cover at the top right of the zoomed view (the HUD stays left of them). */
export const COMPACT_TOP_BTNS = 36;
const HUD_BG: RGB = [12, 11, 20];

/**
 * The HUD of the zoomed phone view (cols × rows from the top left of the grid): your bars, level, gold and depth
 * on row 0; the target (or the stairs) and the party on row 1; the newest log line, or the chat being typed,
 * on the bottom row. Skills and items are on the touch buttons.
 */
export function compactHud(fb: Fb, st: GameState, cols: number, rows: number, target: Ent | null, ui: UiState, now: number, reserve = COMPACT_TOP_BTNS) {
  const me = st.party.find((p) => p.id === st.youId), m = st.me;
  const hw = Math.max(30, cols - reserve);
  fb.fill(0, 0, cols, 2, HUD_BG);
  if (me) {
    const c = CLS[me.cls];
    let x = 1;
    fb.put(x, 0, '♥', [255, 77, 109], null, 1); x += 2;
    fb.bar(x, 0, 12, me.hp / Math.max(1, me.hpMax), me.downed ? [140, 40, 80] : [255, 77, 109], [40, 20, 34]); x += 13;
    const hp = me.hp + '/' + me.hpMax;
    fb.txt(x, 0, hp, TX); x += hp.length + 2;
    fb.bar(x, 0, 8, me.res / Math.max(1, me.resMax), c.resCol, mul(c.resCol, 0.14)); x += 9;
    const res = String(me.res);
    fb.txt(x, 0, res, TX); x += res.length + 2;
    if (m) {
      x = fb.segs(x, 0, [['LV ' + m.lvl, WH, null, 1], [' ' + Math.min(99, Math.round((100 * m.xp) / Math.max(1, m.xpNeed))) + '%', [177, 120, 255]], ['  ' + m.gold + 'g', GOLD, null, 1]]) + 2;
    }
    const lv = st.level;
    const depth = lv?.village ? 'HAVEN' : 'D' + String(lv?.depth ?? 1).padStart(2, '0');
    if (x + depth.length < hw) fb.txt(x, 0, depth, [196, 186, 230], [44, 36, 68], 1);
  }
  // row 1: what you are fighting, or the way down; then the rest of the party
  let x = 1;
  if (me?.downed) x = fb.segs(x, 1, [['▼ DOWNED ' + (me.downT ?? 0) + 's', PINK, null, 1], [' · wait for a revive', DIM]]) + 2;
  else if (me?.dead) x = fb.segs(x, 1, [['✖ FALLEN', PINK, null, 1]]) + 2;
  else if (target) {
    const name = (target.name || '').slice(0, 16);
    fb.txt(x, 1, name, target.el ? [255, 120, 160] : TX, null, 1); x += name.length + 1;
    fb.bar(x, 1, 10, (target.hp ?? 0) / Math.max(1, target.hpMax ?? 1), [255, 150, 90], [50, 24, 20]); x += 12;
  } else if (st.goblin === 'running' && st.level?.event === 'goblin') {
    const ev = eventNote(st, now)!;
    x = fb.segs(x, 1, ev) + 2;
  } else if (st.obj) {
    const o = st.obj, need = o.stairsNeed;
    const t = o.stairsLeft > 0 ? ' ' + Math.ceil(o.stairsLeft / 20) + 's' : '';
    const goal = o.boss && o.kingAlive ? '◆ slay ' + (o.bossName || 'the boss') : o.stairsFound ? '◆ stairs ' + o.stairsAt + '/' + need + t : '◇ find the stairs';
    fb.txt(x, 1, goal.slice(0, 30), o.stairsLeft > 0 ? GOLD : DIM); x += Math.min(30, goal.length) + 2;
  }
  for (const p of st.party) {
    if (p.id === st.youId || x + 8 > hw) continue;
    const c = CLS[p.cls], gone = p.dead || p.downed || !p.connected;
    fb.put(x, 1, '@', gone ? mul(c.col, 0.45) : c.col, null, 1);
    if (p.dead) fb.txt(x + 1, 1, '✖', PINK);
    else if (p.downed) fb.txt(x + 1, 1, '▼', PINK, null, 1);
    else fb.bar(x + 1, 1, 5, p.hp / Math.max(1, p.hpMax), [255, 77, 109], [40, 20, 34]);
    x += 8;
  }
  // bottom row: chat input, else the newest log line for a few seconds
  const y = rows - 1, lw = Math.max(20, cols - 32);
  if (ui.chatActive) {
    fb.fill(0, y, cols, 1, [14, 13, 22]);
    fb.txt(1, y, '»', PINK, null, 1);
    const t = ui.chatText.slice(-(cols - 5));
    fb.txt(3, y, t, WH);
    if (Math.floor(now / 400) % 2 === 0) fb.put(3 + t.length, y, '█', PINK);
    return;
  }
  const ln = st.log[st.log.length - 1];
  if (ln && now - ln.at < 6000) {
    const fade = c01((6000 - (now - ln.at)) / 800);
    let lx = 1;
    fb.fill(0, y, Math.min(cols, lw + 2), 1, mul(HUD_BG, fade));
    for (const sg of ln.segs) {
      if (lx >= lw) break;
      const part = sg[0].slice(0, lw - lx);
      fb.txt(lx, y, part, mul(sg[1], fade), null, sg[2] ?? 0);
      lx += part.length;
    }
  }
}
