import type { App, Screen } from '../app';
import { Fb, W, DIM, TX, WH, PINK, MINT, mul } from '../term/fb';
import type { RGB } from '../term/fb';
import { CLS, CLASS_LIST } from '../classes';
import { drawTitle, keyChips } from '../render/title';
import type { ClassId, FameRun } from '../proto';
import { VERSION } from '../version';
import { fakeKey } from '../gamepad';
import type { Rect } from '../term/canvas';
import { FULL } from '../term/canvas';
import * as kbd from '../kbd';

/** Where the form box sits on the grid. */
export const FORM = { x: 50, y: 16, w: 60 };
/** The Hall of Fame box, right of the form. */
export const FAME = { x: 112, y: 16, w: 46, h: 19 };
const FAME_SHOWN = 8;

/** Play time as "42:07" or "1:02:33". */
export function fameTime(secs: number): string {
  const h = Math.floor(secs / 3600), m = Math.floor((secs % 3600) / 60), s = secs % 60;
  const p2 = (n: number) => String(n).padStart(2, '0');
  return h ? h + ':' + p2(m) + ':' + p2(s) : m + ':' + p2(s);
}

/** Party names cut to share one line of `width` cells, a space between each. */
export function fameNames(names: string[], width: number): string[] {
  const each = Math.max(2, Math.floor((width - (names.length - 1)) / Math.max(1, names.length)));
  return names.map((n) => n.slice(0, each));
}

type Field = 'name' | 'class' | 'new' | 'code' | 'join';

const store = {
  get(k: string): string | null { try { return localStorage.getItem(k); } catch { return null; } },
  set(k: string, v: string) { try { localStorage.setItem(k, v); } catch { /* ignore */ } },
};
export { store };

const CODE_RE = /^[A-HJKMNP-Z2-9]{3}-?[A-HJKMNP-Z2-9]{3}$/;

export class LandingScreen implements Screen {
  name = store.get('emberdeep:name') ?? '';
  cls: ClassId = (store.get('emberdeep:cls') as ClassId) || 'warrior';
  code = '';
  focus = 0;
  error = '';
  busy = false;
  /** Room code from the URL (/r/CODE): skips the create/join choice. */
  fixedCode: string | null = null;
  /** the best won runs, from the server (null until they arrive) */
  fame: FameRun[] | null = null;
  onSubmit: (mode: 'create' | 'join', code: string | null, name: string, cls: ClassId) => void = () => {};

  constructor(private app: App) {
    const m = location.pathname.match(/^\/r\/([A-Za-z0-9-]+)/);
    if (m) this.fixedCode = m[1].toUpperCase();
    if (!CLASS_LIST.includes(this.cls)) this.cls = 'warrior';
    this.loadFame();
  }

  private loadFame() {
    if (typeof fetch !== 'function') return;
    fetch('/api/fame')
      .then((r) => (r.ok ? r.json() : []))
      .then((f: unknown) => { this.fame = Array.isArray(f) ? (f as FameRun[]) : []; })
      .catch(() => { this.fame = this.fame ?? []; });
  }

  private fields(): Field[] {
    return this.fixedCode ? ['name', 'class', 'join'] : ['name', 'class', 'new', 'code', 'join'];
  }

  onEnter() { this.busy = false; this.loadFame(); }

  /** The row of each field on the grid (the same rows render() draws them on). */
  fieldRows(): [Field, number][] {
    const y = FORM.y;
    const r: [Field, number][] = [['name', y + 3], ['class', y + 5]];
    if (this.fixedCode) r.push(['join', y + 8]);
    else r.push(['new', y + 8], ['code', y + 12], ['join', y + 14]);
    return r;
  }

  /** The field under a cell (a row either side counts: fingers are big), or null. */
  fieldAt(cx: number, cy: number): Field | null {
    if (cx < FORM.x || cx >= FORM.x + FORM.w) return null;
    let best: Field | null = null, bd = 2;
    for (const [f, row] of this.fieldRows()) {
      const d = Math.abs(cy - row);
      if (d < bd) { best = f; bd = d; }
    }
    return best;
  }

  view(compact: boolean): Rect {
    if (!compact) return FULL;
    const h = this.fixedCode ? 15 : 19;
    return { x: FORM.x - 2, y: FORM.y - 1, w: FORM.w + 4, h: h + 4 };
  }

  onMouseDown(ev: MouseEvent, cell: [number, number]) {
    if (ev.button !== 0) return;
    const f = this.fieldAt(cell[0], cell[1]);
    if (!f) { kbd.close(); return; }
    this.focus = this.fields().indexOf(f);
    if (f === 'name' || f === 'code') {
      if ((ev as PointerEvent).pointerType !== 'touch') return; // a mouse click just focuses it: the keyboard types as before
      kbd.request({ // opens the phone keyboard
        value: f === 'name' ? this.name : this.code, max: f === 'name' ? 12 : 7,
        onInput: (v) => { if (f === 'name') this.name = v; else this.code = v.toUpperCase(); this.error = ''; },
        onDone: (enter) => { if (enter) this.onKey(fakeKey('Enter')); },
      });
      return;
    }
    kbd.close();
    if (f === 'class') this.onKey(fakeKey(cell[0] < FORM.x + FORM.w / 2 ? 'ArrowLeft' : 'ArrowRight'));
    else if (f === 'new') this.submit('create');
    else this.submit('join');
  }

  private submit(mode: 'create' | 'join') {
    if (this.busy) return;
    const name = this.name.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '');
    if (name.length < 2) { this.error = 'pick a name: 2-12 letters, digits, _ or -'; this.focus = 0; return; }
    let code: string | null = null;
    if (mode === 'join') {
      code = (this.fixedCode ?? this.code).toUpperCase().trim();
      if (!CODE_RE.test(code)) { this.error = 'that code does not look right (e.g. ASH-7Q2)'; this.focus = this.fields().indexOf('code'); return; }
      if (!code.includes('-')) code = code.slice(0, 3) + '-' + code.slice(3);
    }
    store.set('emberdeep:name', name);
    store.set('emberdeep:cls', this.cls);
    this.error = '';
    this.busy = true;
    this.onSubmit(mode, code, name, this.cls);
  }

  onKey(ev: KeyboardEvent) {
    const f = this.fields()[this.focus];
    const move = (d: number) => { const n = this.fields().length; this.focus = (this.focus + d + n) % n; };
    if (ev.key === 'Tab') { move(ev.shiftKey ? -1 : 1); ev.preventDefault(); return; }
    if (ev.key === 'ArrowDown') { move(1); ev.preventDefault(); return; }
    if (ev.key === 'ArrowUp') { move(-1); ev.preventDefault(); return; }
    if (ev.key === 'Enter') {
      ev.preventDefault();
      if (f === 'new') this.submit('create');
      else if (f === 'join' || f === 'code') this.submit('join');
      else move(1);
      return;
    }
    if (f === 'class' && (ev.key === 'ArrowLeft' || ev.key === 'ArrowRight' || ev.key === ' ')) {
      const i = CLASS_LIST.indexOf(this.cls), d = ev.key === 'ArrowLeft' ? -1 : 1;
      this.cls = CLASS_LIST[(i + d + CLASS_LIST.length) % CLASS_LIST.length];
      ev.preventDefault();
      return;
    }
    if (f === 'name' || f === 'code') {
      const cur = f === 'name' ? this.name : this.code, max = f === 'name' ? 12 : 7;
      let next = cur;
      if (ev.key === 'Backspace') next = cur.slice(0, -1);
      else if (ev.key.length === 1 && !ev.ctrlKey && !ev.metaKey && cur.length < max) next = cur + (f === 'code' ? ev.key.toUpperCase() : ev.key);
      else return;
      if (f === 'name') this.name = next; else this.code = next;
      this.error = '';
      ev.preventDefault();
    }
  }

  render(fb: Fb, now: number) {
    const t = now / 1000;
    fb.reset([9, 8, 14]);
    drawTitle(fb, t);
    const { x, y, w } = FORM, h = this.fixedCode ? 15 : 19;
    fb.box(x, y, w, h, this.fixedCode ? 'JOIN ' + this.fixedCode : 'ENTER THE DESCENT', PINK);
    const fields = this.fields();
    const foc = (f: Field) => fields[this.focus] === f;
    const blink = Math.floor(now / 450) % 2 === 0;
    const input = (row: number, label: string, val: string, f: Field, width: number) => {
      fb.txt(x + 3, row, label, DIM, null, 1);
      const on = foc(f);
      fb.txt(x + 12, row, '[', on ? PINK : DIM, null, 1);
      fb.txt(x + 13 + width, row, ']', on ? PINK : DIM, null, 1);
      fb.fill(x + 13, row, width, 1, on ? [28, 24, 42] : [20, 18, 30]);
      fb.txt(x + 14, row, val, WH, null, 1);
      if (on && blink) fb.put(x + 14 + val.length, row, '█', PINK);
    };
    let row = y + 3;
    input(row, 'NAME', this.name, 'name', 14);
    row += 2;
    const c = CLS[this.cls], onC = foc('class');
    fb.txt(x + 3, row, 'CLASS', DIM, null, 1);
    fb.segs(x + 13, row, [['◂ ', onC ? PINK : DIM], ['@ ', c.col, null, 1], [c.name.padEnd(9), c.col, null, 1], [' ▸', onC ? PINK : DIM]]);
    fb.txt(x + 13, row + 1, [c.skills[0], c.skills[3]].map((s) => s.short).join(' · ') + ' · +4 more in the tree', DIM);
    row += 3;
    const button = (r: number, key: string, label: string, f: Field) => {
      const on = foc(f);
      fb.txt(x + 13, r, ' ' + key + ' ', on ? [24, 6, 20] : [24, 22, 36], on ? PINK : [150, 144, 190], 1);
      fb.txt(x + 17, r, ' ' + label, on ? WH : TX, null, on ? 1 : 0);
    };
    if (!this.fixedCode) {
      button(row, 'N', 'NEW GAME', 'new');
      row += 2;
      fb.ctxt(row, '— or join a friend —', DIM);
      row += 2;
      input(row, 'CODE', this.code, 'code', 9);
      row += 2;
    }
    button(row, 'J', this.fixedCode ? 'JOIN ' + this.fixedCode : 'JOIN', 'join');
    if (this.busy) fb.txt(x + 40, row, 'connecting…', [255, 176, 70], null, 1);
    if (this.error) fb.ctxt(y + h + 1, this.error, [255, 90, 120], null, 1);
    this.drawFame(fb);
    keyChips(fb, 40, [['TAB', 'next field'], ['ENTER', 'select'], ['←/→', 'change class']], -1);
    fb.txt(2, 46, 'co-op for 1–4 players · create a game, then share the code or invite link', DIM);
    const rt = 'v' + VERSION + ' · 24-bit color · 160×48';
    fb.txt(W - 2 - rt.length, 46, rt, [88, 84, 118]);
    void MINT; void mul;
  }

  /** The Hall of Fame: the best runs that reached the bottom, two lines each. */
  private drawFame(fb: Fb) {
    const { x, y, w, h } = FAME;
    fb.box(x, y, w, h, 'HALL OF FAME', [255, 205, 70]);
    const runs = (this.fame ?? []).slice(0, FAME_SHOWN);
    if (!runs.length) {
      const msg = this.fame ? ['No party has reached', 'the bottom yet.', '', 'Be the first.'] : ['…'];
      msg.forEach((m, i) => fb.txt(x + Math.floor((w - m.length) / 2), y + 7 + i, m, i === 3 ? TX : DIM, null, i === 3 ? 1 : 0));
      return;
    }
    const DIFF_COL: Record<string, RGB> = { NIGHTMARE: PINK, NORMAL: TX, NOVICE: DIM };
    runs.forEach((r, i) => {
      const ry = y + 2 + i * 2, t = fameTime(r.secs);
      fb.txt(x + 2, ry, '#' + (i + 1), i === 0 ? [255, 205, 70] : TX, null, 1);
      fb.txt(x + 13 - t.length, ry, t, WH, null, 1);
      fb.txt(x + 15, ry, r.diff, DIFF_COL[r.diff] ?? TX, null, 1);
      fb.txt(x + 25, ry, r.wipes ? r.wipes + (r.wipes === 1 ? ' wipe' : ' wipes') : 'flawless', r.wipes ? DIM : MINT);
      fb.txt(x + w - 12, ry, r.date, DIM);
      const names = fameNames(r.party.map((p) => p.name), w - 6);
      let nx = x + 5;
      r.party.forEach((p, k) => {
        fb.txt(nx, ry + 1, names[k], CLS[p.cls]?.col ?? TX, null, p.bot ? 0 : 1);
        nx += names[k].length + 1;
      });
    });
  }
}
