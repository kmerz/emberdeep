import type { App, Screen } from '../app';
import { Fb, RGB, W, DIM, TX, WH, PINK, MINT, AMB, mul } from '../term/fb';
import { CLS, CLASS_LIST } from '../classes';
import { drawTitle, keyChips } from '../render/title';
import { STORY, STORY_TITLE, wrap } from '../story';
import { keyLabel, type PadContext } from '../gamepad';
import { VERSION } from '../version';
import { FULL, type Rect } from '../term/canvas';
import { layout } from '../mobile';
import type { TouchBtn } from '../touch';

const STORY_BOX = { x: 22, y: 39, w: 116, h: 7 };
const TYPE_CPS = 70;      // story letters per second
const PAGE_HOLD = 9000;   // ms a finished page stays before the next one
const NEW_RUN_MS = 3000;  // the second N press must come this soon

export class LobbyScreen implements Screen {
  toast = '';
  toastAt = 0;
  private page = 0;
  private pageAt = 0; // when the current page started typing (0 = not yet)
  private newRunArmed = 0; // when N was pressed once: a second press within NEW_RUN_MS forgets the save
  constructor(private app: App) {}

  private turn(d: number) {
    this.page = (this.page + d + STORY.length) % STORY.length;
    this.pageAt = performance.now();
  }

  /** The backstory: typed out page by page under the lobby box, turning by itself. */
  private drawStory(fb: Fb, now: number) {
    const b = STORY_BOX;
    if (!this.pageAt) this.pageAt = now;
    const lines = wrap(STORY[this.page], b.w - 6);
    const total = lines.reduce((a, l) => a + l.length, 0);
    const shown = Math.floor(((now - this.pageAt) / 1000) * TYPE_CPS);
    if (shown > total + (PAGE_HOLD / 1000) * TYPE_CPS) this.turn(1);
    fb.box(b.x, b.y, b.w, b.h, STORY_TITLE + ' · ' + (this.page + 1) + '/' + STORY.length, AMB);
    let left = shown;
    lines.slice(0, b.h - 2).forEach((l, i) => {
      const part = l.slice(0, Math.max(0, left));
      left -= l.length;
      fb.txt(b.x + 3, b.y + 1 + i, part, [214, 206, 230]);
      if (left < 0 && part.length < l.length && Math.floor(now / 300) % 2 === 0) fb.put(b.x + 3 + part.length, b.y + 1 + i, '▌', AMB);
    });
    const hint = ' ← → turn the page ';
    fb.txt(b.x + b.w - hint.length - 2, b.y + b.h - 1, hint, DIM);
  }

  padContext(): PadContext { return 'lobby'; }

  /** Phones: the lobby box, its keys and the toast line (the story is left out). */
  view(compact: boolean): Rect { return compact ? { x: 36, y: 15, w: 88, h: 27 } : FULL; }

  touchButtons(): TouchBtn[] {
    const st = this.app.st, lob = st.lobby;
    const me = lob?.players.find((p) => p.id === st.youId);
    const b: TouchBtn[] = [
      { key: 'Enter', label: me?.ready ? (st.isHost ? 'START' : 'UNREADY') : 'READY', area: 'bot' },
      { key: 'c', label: 'CLASS', area: 'bot' },
      { key: 'i', label: 'INVITE', area: 'bot', tap: () => this.invite() },
    ];
    if (st.isHost) {
      b.push({ key: 'f', label: 'FF', sub: lob?.settings?.friendlyFire ? 'on' : 'off', area: 'bot' }, { key: 'd', label: 'DIFF', area: 'bot' },
        { key: 'b', label: '+BOT', area: 'bot' }, { key: 'x', label: '−BOT', area: 'bot' });
      if (lob?.save) b.push({ key: 'n', label: 'NEW RUN', sub: 'tap twice', area: 'bot' });
    }
    b.push({ key: 'q', label: 'LEAVE', area: 'bot' });
    return b;
  }

  /** Copy the invite link, or open the share sheet on phones. */
  invite() {
    const st = this.app.st;
    let url = st.inviteUrl;
    if (!url || url.startsWith('/')) url = location.origin + '/r/' + st.code;
    if (layout.touch && navigator.share) { navigator.share({ title: 'Emberdeep', text: 'Join my Emberdeep game: ' + st.code, url }).catch(() => {}); return; }
    const done = () => this.say('invite link copied: ' + url);
    if (navigator.clipboard?.writeText) navigator.clipboard.writeText(url).then(done, () => this.say('copy failed — ' + url));
    else this.say(url);
  }

  private say(msg: string) { this.toast = msg; this.toastAt = performance.now(); }

  onKey(ev: KeyboardEvent) {
    const st = this.app.st, net = this.app.net;
    const lob = st.lobby;
    const me = lob?.players.find((p) => p.id === st.youId);
    const k = ev.key.toLowerCase();
    if (ev.repeat) return;
    if (ev.key === 'Enter') {
      if (me?.ready && st.isHost) net.send({ t: 'start' });
      else net.send({ t: 'ready' });
      ev.preventDefault();
    } else if (k === 'c' && me) {
      const i = CLASS_LIST.indexOf(me.cls);
      net.send({ t: 'setClass', cls: CLASS_LIST[(i + 1) % CLASS_LIST.length] });
    } else if (k === 'i') this.invite();
    else if (k === 'f' && st.isHost) net.send({ t: 'setting', key: 'friendlyFire' });
    else if (k === 'd' && st.isHost) net.send({ t: 'setting', key: 'difficulty' });
    else if (k === 'b' && st.isHost) net.send({ t: 'setting', key: 'addBot' });
    else if (k === 'x' && st.isHost) net.send({ t: 'setting', key: 'removeBot' });
    else if (k === 'n' && st.isHost && lob?.save) {
      const now = performance.now();
      if (now - this.newRunArmed < NEW_RUN_MS) { net.send({ t: 'setting', key: 'newRun' }); this.newRunArmed = 0; this.say('the save is gone: the next start is a new descent'); }
      else { this.newRunArmed = now; this.say('press ' + keyLabel('lobby', 'n', 'N') + ' again to delete the save and start a new run'); }
    }
    else if (k === 'q') this.app.setScreen('landing');
    else if (ev.key === 'ArrowRight') this.turn(1);
    else if (ev.key === 'ArrowLeft') this.turn(-1);
  }

  render(fb: Fb, now: number) {
    const st = this.app.st, lob = st.lobby;
    fb.reset([9, 8, 14]);
    drawTitle(fb, now / 1000);
    const players = lob?.players ?? [];
    fb.box(38, 16, 84, 18, `LOBBY · ${st.code} · ${players.length}/4`, PINK);
    for (const [x, s] of [[41, 'SLOT'], [47, 'PLAYER'], [61, 'CLASS'], [74, 'LVL'], [81, 'STATUS'], [99, 'PING']] as [number, string][]) fb.txt(x, 18, s, DIM, null, 1);
    for (let x = 41; x <= 118; x++) fb.put(x, 19, '─', [40, 37, 58]);
    for (let i = 0; i < 4; i++) {
      const y = 20 + i, p = players[i];
      if (!p) { fb.txt(42, y, String(i + 1), [60, 56, 84]); fb.txt(47, y, '· open slot ·', [70, 66, 96]); continue; }
      const c = CLS[p.cls];
      const you = p.id === st.youId;
      if (you) fb.fill(39, y, 82, 1, [28, 24, 42]);
      fb.txt(42, y, String(p.slot), DIM);
      fb.put(47, y, '@', p.connected ? c.col : mul(c.col, 0.4), mul(c.col, 0.3), 1);
      fb.txt(49, y, p.name, WH, null, 1);
      if (you) fb.txt(49 + p.name.length + 1, y, 'you', DIM);
      else if (p.host) fb.txt(49 + p.name.length + 1, y, 'host', DIM);
      fb.txt(61, y, c.name, c.col, null, 1);
      fb.txt(74, y, String(p.lvl), TX);
      if (p.saved) fb.txt(74 + String(p.lvl).length + 1, y, '↺', AMB); // continues a saved hero
      if (!p.connected) fb.txt(81, y, '… reconnecting', AMB, null, 1);
      else if (p.ready) fb.txt(81, y, p.bot ? '● READY (bot)' : '● READY', MINT, null, 1);
      else fb.txt(81, y, '○ choosing class…', Math.floor(now / 500) % 2 === 0 ? AMB : mul(AMB, 0.55), null, 1);
      fb.txt(99, y, p.bot ? '—' : p.pingMs >= 0 ? p.pingMs + ' ms' : '… ms', DIM);
    }
    const s = lob?.settings;
    fb.txt(41, 25, 'DIFFICULTY', DIM);
    fb.segs(57, 25, [['◂ ', DIM], [s?.difficulty ?? '', PINK, null, 1], [' ▸', DIM]]);
    fb.txt(41, 26, 'SEED', DIM);
    fb.segs(57, 26, [[s?.seed ?? '', TX, null, 1], [' · same seed on restart', DIM]]);
    fb.txt(41, 27, 'LOOT', DIM);
    fb.txt(57, 27, s?.loot ?? '', TX);
    fb.txt(41, 28, 'FRIENDLY FIRE', DIM);
    fb.txt(57, 28, s?.friendlyFire ? 'on' : 'off', s?.friendlyFire ? PINK : TX);
    const sv = lob?.save;
    if (sv) {
      fb.txt(41, 29, 'SAVED RUN', AMB, null, 1);
      const where = sv.village ? 'the gnome village' : 'depth ' + sv.depth;
      const segs: [string, RGB, null?, number?][] = [[where, WH, null, 1]];
      for (const h of sv.heroes) segs.push([' · ', DIM], [h.name + ' ', TX], [CLS[h.cls]?.name.toLowerCase() ?? h.cls, CLS[h.cls]?.col ?? TX], [' ' + h.lvl, TX]);
      let x = 57;
      for (const sg of segs) { const t = sg[0].slice(0, Math.max(0, 119 - x)); fb.txt(x, 29, t, sg[1], null, sg[3] ?? 0); x += t.length; }
      const hint = 'starting continues it · same name + class = your saved hero' + (st.isHost ? ' · ' + keyLabel('lobby', 'n', 'N') + ' twice: new run' : '');
      fb.txt(41, 30, hint.slice(0, 79), DIM);
    }
    const waiting = players.filter((p) => !p.ready && !p.bot).map((p) => p.name);
    if (waiting.length) {
      fb.txt(41, 31, 'waiting for ', DIM);
      let x = 53;
      waiting.forEach((n, i) => { fb.txt(x, 31, n, [255, 212, 92], null, 1); x += n.length; if (i < waiting.length - 1) { fb.txt(x, 31, ', ', DIM); x += 2; } });
      fb.txt(x, 31, ' — the descent starts when everyone is ready', DIM);
    } else fb.txt(41, 31, 'everyone is ready — descending…', MINT, null, 1);
    const me = players.find((p) => p.id === st.youId);
    const enterLabel = me?.ready ? (st.isHost ? 'start now' : 'unready') : 'ready up';
    const keys: [string, string][] = [['ENTER', enterLabel], ['C', 'change class'], ['I', 'copy invite link']];
    if (st.isHost) keys.push(['F', 'friendly fire'], ['D', 'difficulty'], ['B', 'add bot'], ['X', 'remove bot']);
    if (st.isHost && sv) keys.push(['N', 'new run']);
    keys.push(['Q', 'leave']);
    keyChips(fb, 36, keys, 0);
    if (this.toast && now - this.toastAt < 4000) fb.ctxt(38, this.toast, MINT);
    if (!layout.compact) this.drawStory(fb, now);
    let url = st.inviteUrl;
    if (!url || url.startsWith('/')) url = location.origin + '/r/' + st.code;
    fb.txt(2, 46, 'invite: ' + url, DIM);
    const rt = 'v' + VERSION + ' · 24-bit color · 160×48 min';
    fb.txt(W - 2 - rt.length, 46, rt, [88, 84, 118]);
  }
}
