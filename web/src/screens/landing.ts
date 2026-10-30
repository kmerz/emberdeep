import type { App, Screen } from '../app';
import { Fb, W, DIM, TX, WH, PINK, MINT, mul } from '../term/fb';
import { CLS, CLASS_LIST } from '../classes';
import { drawTitle, keyChips } from '../render/title';
import type { ClassId } from '../proto';

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
  onSubmit: (mode: 'create' | 'join', code: string | null, name: string, cls: ClassId) => void = () => {};

  constructor(private app: App) {
    const m = location.pathname.match(/^\/r\/([A-Za-z0-9-]+)/);
    if (m) this.fixedCode = m[1].toUpperCase();
    if (!CLASS_LIST.includes(this.cls)) this.cls = 'warrior';
  }

  private fields(): Field[] {
    return this.fixedCode ? ['name', 'class', 'join'] : ['name', 'class', 'new', 'code', 'join'];
  }

  onEnter() { this.busy = false; }

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
    const x = 50, y = 16, w = 60, h = this.fixedCode ? 15 : 19;
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
    fb.txt(x + 13, row + 1, c.skills.map((s) => s.short).join(' · ') + ' · potion', DIM);
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
    keyChips(fb, 40, [['TAB', 'next field'], ['ENTER', 'select'], ['←/→', 'change class']], -1);
    fb.txt(2, 46, 'co-op for 1–4 players · create a game, then share the code or invite link', DIM);
    const rt = 'v0.1 · 24-bit color · 160×48';
    fb.txt(W - 2 - rt.length, 46, rt, [88, 84, 118]);
    void MINT; void mul;
  }
}
