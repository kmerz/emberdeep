import { GameState } from '../state';

const SOLID = new Set(['#', 'T', 'O', ' '].map((c) => c.charCodeAt(0)));
export const DIRS: [number, number][] = [[0, -1], [1, -1], [1, 0], [1, 1], [0, 1], [-1, 1], [-1, 0], [-1, -1]];
export const MOVE_MS = 160; // a little slower than the server's 150 ms step so we never trip its speed check

interface Pending { seq: number; dir: number }

/** Local-hero prediction: apply moves at once, keep unacked ones, replay them over each server snapshot. */
export class Predictor {
  x = 0;
  y = 0;
  face = 2;
  seq = 0;
  lastSend = 0;
  pending: Pending[] = [];
  ready = false;

  constructor(private st: GameState) {}

  reset() { this.ready = false; this.pending = []; }

  private blocked(x: number, y: number): boolean {
    const t = this.st.tileAt(x, y);
    if (t === 0 || SOLID.has(t)) return true;
    const rec = this.st.recs[this.st.recs.length - 1];
    if (rec) for (const e of rec.ents) if ((e.k === 'm' || (e.k === 'h' && e.id !== this.st.youId) || (e.k === 'i' && e.g === 'C') || e.k === 'n') && e.x === x && e.y === y) return true;
    return false;
  }

  /** Called for every snapshot. */
  onSnap() {
    const rec = this.st.recs[this.st.recs.length - 1];
    if (!rec) return;
    const me = rec.ents.find((e) => e.k === 'h' && e.id === this.st.youId);
    if (!me) return;
    this.x = me.x; this.y = me.y; this.face = me.face;
    this.pending = this.pending.filter((p) => p.seq > this.st.ackSeq);
    if (me.downed || me.dead) this.pending = [];
    for (const p of this.pending) this.apply(p.dir);
    this.ready = true;
  }

  private apply(dir: number): boolean {
    const [dx, dy] = DIRS[dir];
    this.face = dir;
    if (this.blocked(this.x + dx, this.y + dy)) return false;
    this.x += dx; this.y += dy;
    return true;
  }

  /** Try to move; returns the message to send, or null if not allowed yet. */
  move(dir: number, now: number): object | null {
    if (!this.ready || now - this.lastSend < MOVE_MS) return null;
    const me = this.st.recs[this.st.recs.length - 1]?.ents.find((e) => e.k === 'h' && e.id === this.st.youId);
    if (me?.st?.some((s) => s.k === 'charmed')) return null; // the server won't let a charmed hero move
    this.lastSend = now;
    const seq = ++this.seq;
    this.apply(dir);
    this.pending.push({ seq, dir });
    return { t: 'move', dir, seq };
  }

  nextSeq(): number { return ++this.seq; }
}
