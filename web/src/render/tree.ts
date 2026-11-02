// The skill-tree panel (T): 3 tiers with 2 skills each, drawn over the map like the rucksack.
// Rules mirror internal/game (SkillTiers, MaxRank, canLearn); the server has the final say.
import { Fb, Seg, DIM, TX, WH, GOLD, PINK, mul } from '../term/fb';
import type { GameState } from '../state';
import { CLS, SKILL_BY_ID } from '../classes';

export const SKILL_TIERS = [1, 3, 8];
export const MAX_RANK = 5;
export const TREE = { x: 30, y: 4, w: 52, h: 25 };
const BOX_W = 23;

/** Tree order is the server's: index i is in tier i % 3 + 1; the two skills of tier t are t-1 and t+2. */
export function tierOf(i: number) { return (i % 3) + 1; }
/** The panel lists skills row by row: [0, 3], [1, 4], [2, 5]. */
export const PANEL_ORDER = [0, 3, 1, 4, 2, 5];

/** Why a skill can't take a point right now ('' = it can). */
export function lockReason(st: GameState, i: number): string {
  const m = st.me;
  if (!m?.tree) return '…';
  const t = tierOf(i), rank = m.tree[i]?.rank ?? 0;
  if (rank >= MAX_RANK) return 'max rank';
  if (m.lvl < SKILL_TIERS[t - 1]) return 'opens at level ' + SKILL_TIERS[t - 1];
  if (t > 1 && rank === 0 && (m.tree[t - 2]?.rank ?? 0) === 0 && (m.tree[t + 1]?.rank ?? 0) === 0) return 'needs a tier ' + (t - 1) + ' skill';
  if ((m.points ?? 0) <= 0) return 'no points left';
  return '';
}

export function treePanel(fb: Fb, st: GameState, sel: number, cls: keyof typeof CLS | null) {
  const m = st.me;
  if (!m?.tree || !cls) return;
  const c = CLS[cls];
  fb.box(TREE.x, TREE.y, TREE.w, TREE.h, 'SKILLS · ' + c.name + ' · ' + m.points + (m.points === 1 ? ' point' : ' points'), c.col);
  for (let row = 0; row < 3; row++) {
    const y = TREE.y + 2 + row * 5;
    const tlv = SKILL_TIERS[row];
    fb.txt(TREE.x + 2, y + 1, 'TIER ' + (row + 1), m.lvl >= tlv ? DIM : mul(DIM, 0.6), null, 1);
    fb.txt(TREE.x + 2, y + 2, 'lv ' + tlv, mul(DIM, 0.8));
    for (let col = 0; col < 2; col++) {
      const i = PANEL_ORDER[row * 2 + col];
      const t = m.tree[i];
      const meta = SKILL_BY_ID[t?.id ?? '']?.meta;
      if (!meta) continue;
      const x = TREE.x + 9 + col * (BOX_W + 1);
      const on = PANEL_ORDER[sel] === i;
      const lock = lockReason(st, i);
      const learned = (t?.rank ?? 0) > 0;
      const br = on ? GOLD : learned ? [100, 94, 136] : [52, 48, 72];
      fb.txt(x, y, '╭' + '─'.repeat(BOX_W - 2) + '╮', br);
      fb.txt(x, y + 1, '│', br); fb.txt(x + BOX_W - 1, y + 1, '│', br);
      fb.txt(x, y + 2, '│', br); fb.txt(x + BOX_W - 1, y + 2, '│', br);
      fb.txt(x, y + 3, '╰' + '─'.repeat(BOX_W - 2) + '╯', br);
      fb.txt(x + 2, y + 1, meta.icon, learned ? WH : DIM, null, 1);
      fb.txt(x + 6, y + 1, meta.name.slice(0, BOX_W - 8), learned ? TX : DIM, null, learned ? 1 : 0);
      const pips = '●'.repeat(t.rank) + '○'.repeat(MAX_RANK - t.rank);
      fb.txt(x + 2, y + 2, pips, learned ? GOLD : mul(DIM, 0.7));
      const key = m.binds.indexOf(t.id);
      if (key >= 0) fb.txt(x + BOX_W - 8, y + 2, 'key ' + (key + 1), c.col, null, 1);
      else if (lock && lock !== 'no points left' && lock !== 'max rank' && !learned) fb.txt(x + BOX_W - 8, y + 2, '  lock', mul(DIM, 0.8));
    }
  }
  // the selected skill
  const i = PANEL_ORDER[sel], t = m.tree[i], meta = SKILL_BY_ID[t?.id ?? '']?.meta;
  const y = TREE.y + 17;
  if (meta) {
    fb.segs(TREE.x + 2, y, [['▸ ', GOLD], [meta.name, c.col, null, 1], ['  rank ' + t.rank + '/' + MAX_RANK, TX]]);
    fb.txt(TREE.x + 4, y + 1, meta.desc ?? '', DIM);
    const lock = lockReason(st, i);
    fb.txt(TREE.x + 4, y + 2, lock ? lock : t.rank > 0 ? 'ENTER: raise to rank ' + (t.rank + 1) + ' (+15% damage, −5% cooldown)' : 'ENTER: learn it', lock ? PINK : GOLD);
  }
  const keys: Seg[] = [['ENTER', WH, null, 1], [' learn · ', DIM], ['1 2 3', WH, null, 1], [' put on key · ', DIM]];
  if (m.respec) keys.push(['R', WH, null, 1], [' reset · ', DIM]);
  keys.push(['T', WH, null, 1], [' close', DIM]);
  fb.segs(TREE.x + 2, TREE.y + TREE.h - 2, keys);
  if (m.respec) fb.txt(TREE.x + 2, TREE.y + TREE.h - 3, 'at the stairs: R resets every point for free', mul(GOLD, 0.8));
}
