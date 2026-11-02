// Sidequests: the notice board / Grimble's offers, and the journal (J).
import { Fb, RGB, Seg, DIM, TX, WH, GOLD, PINK, MINT, mul } from '../term/fb';
import type { GameState } from '../state';
import type { QuestInfo } from '../proto';
import { wrap } from '../story';
import { loreTitle } from './lore';

export const QUEST_BOX = { x: 28, y: 4, w: 58, h: 24 };

const STATE_TAG: Record<QuestInfo['state'], [string, RGB]> = {
  offered: ['', DIM], active: ['ACTIVE', GOLD], done: ['DONE ✓', MINT], failed: ['FAILED', PINK],
};

function progress(q: QuestInfo): string {
  if (q.kind === 'collect') return q.progress + '/' + q.count;
  if (q.kind === 'fetch') return q.progress > 0 ? 'found' : '';
  return '';
}

/** The quests a giver lists (its offers and the ones taken from it). */
export function giverQuests(st: GameState, giver: 'board' | 'grimble'): QuestInfo[] {
  return st.quests.filter((q) => q.giver === giver);
}

/** The journal: every quest the party has taken on, then the story pages read this run (id 'lore:N'). */
export function journalQuests(st: GameState): QuestInfo[] {
  const pages = [...st.lore.entries()].sort((a, b) => (a[0] || 5.5) - (b[0] || 5.5)) // the village sits between 5 and 6
    .map(([page, text]): QuestInfo => ({ id: 'lore:' + page, title: 'Inscription · ' + loreTitle(page).toLowerCase(), text,
      giver: 'board', kind: 'hunt', depth: page, state: 'done', progress: 0, count: 0, gold: 0, xp: 0, point: false }));
  return [...st.quests.filter((q) => q.state !== 'offered'), ...pages];
}

function drawList(fb: Fb, list: QuestInfo[], sel: number, title: string, empty: string, keys: Seg[]) {
  const b = QUEST_BOX;
  fb.box(b.x, b.y, b.w, b.h, title, GOLD);
  if (!list.length) fb.txt(b.x + 3, b.y + 2, empty, DIM);
  list.forEach((q, i) => {
    const y = b.y + 2 + i, on = i === sel;
    const story = q.id.startsWith('lore:');
    const [tag, col] = story ? ['STORY', [226, 210, 170] as RGB] : STATE_TAG[q.state];
    fb.txt(b.x + 2, y, on ? '▸' : ' ', GOLD);
    fb.txt(b.x + 4, y, q.title.slice(0, 34), q.state === 'offered' || q.state === 'active' ? (on ? WH : TX) : DIM, null, on ? 1 : 0);
    if (!story) fb.txt(b.x + 39, y, 'fl ' + q.depth, DIM);
    const p = progress(q);
    if (p && q.state === 'active') fb.txt(b.x + 45, y, p, TX);
    if (tag) fb.txt(b.x + b.w - 2 - tag.length, y, tag, col, null, 1);
  });
  const q = list[sel];
  if (q) {
    const y = b.y + 3 + Math.max(list.length, 1);
    for (let x = b.x + 2; x < b.x + b.w - 2; x++) fb.put(x, y, '─', [40, 37, 58]);
    if (q.id.startsWith('lore:')) { // a story page: just the text
      wrap(q.text, b.w - 6).slice(0, b.y + b.h - 4 - y).forEach((l, i) => fb.txt(b.x + 3, y + 1 + i, l, [226, 210, 170]));
      fb.segs(b.x + 2, b.y + b.h - 2, keys);
      return;
    }
    wrap(q.text, b.w - 6).slice(0, 5).forEach((l, i) => fb.txt(b.x + 3, y + 1 + i, l, [214, 206, 230]));
    const rew: Seg[] = [['Reward: ', DIM], [q.gold + ' gold', GOLD, null, 1], [', ' + q.xp + ' XP', TX]];
    if (q.point) rew.push([', +1 skill point', GOLD, null, 1]);
    rew.push([' for everyone', DIM]);
    fb.segs(b.x + 3, y + 7, rew);
    const hand = q.kind === 'fetch' || q.kind === 'collect' ? 'Hand it in to Grimble in the village.' : q.kind === 'rescue' ? 'Walk into them, then lead them to the stairs.' : 'Find it and slay it.';
    fb.txt(b.x + 3, y + 8, hand, mul(DIM, 0.9));
  }
  fb.segs(b.x + 2, b.y + b.h - 2, keys);
}

export function questBoardPanel(fb: Fb, st: GameState, giver: 'board' | 'grimble', sel: number) {
  const list = giverQuests(st, giver);
  const keys: Seg[] = [['↑↓', WH, null, 1], [' pick · ', DIM], ['ENTER', WH, null, 1], [' accept · ', DIM]];
  keys.push(giver === 'grimble' ? ['TAB', WH, null, 1] : ['J', WH, null, 1], [giver === 'grimble' ? ' buy/sell · ' : ' journal · ', DIM], ['Esc', WH, null, 1], [' close', DIM]);
  drawList(fb, list, sel, giver === 'board' ? "THE MAYOR'S NOTICE BOARD" : "GRIMBLE'S ERRANDS", 'nothing posted', keys);
}

export function journalPanel(fb: Fb, st: GameState, sel: number) {
  const list = journalQuests(st);
  drawList(fb, list, sel, 'JOURNAL · ' + list.filter((q) => q.state === 'active').length + ' active',
    'Nothing yet. The notice board on floor 1 has quests.', [['↑↓', WH, null, 1], [' pick · ', DIM], ['J', WH, null, 1], [' / ', DIM], ['Esc', WH, null, 1], [' close', DIM]]);
}
