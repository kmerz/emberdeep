// A small client log for chasing lag, stuck keys and sound problems. It keeps the last lines in memory and in
// this browser (so the log of the page before a reload is still there), counts what happens each second, and
// writes a line by itself when something looks wrong (a long frame, a gap in snapshots, the hero snapped back).
//
// F9 or /debug shows the live numbers; /log saves the log as a file; in the console: emberdeep.log(), .download().

const STORE = 'emberdeep:debuglog';
const MAX = 600;
/** a frame this long is a hitch */
const HITCH_MS = 200;
/** no snapshot for this long while in the game is a gap */
const SNAP_GAP_MS = 400;

const browser = typeof window !== 'undefined' && typeof document !== 'undefined';
const visible = () => !browser || document.visibilityState === 'visible';

const lines: string[] = [];
try {
  const prev = JSON.parse(localStorage.getItem(STORE) ?? '[]') as string[];
  if (Array.isArray(prev) && prev.length) lines.push(...prev.slice(-MAX / 2), '──── page loaded ' + new Date().toISOString() + ' ────');
} catch { /* nothing kept, or a private window */ }

let saveTimer = 0;
function persist() {
  if (saveTimer || !browser) return;
  saveTimer = window.setTimeout(() => {
    saveTimer = 0;
    try { localStorage.setItem(STORE, JSON.stringify(lines.slice(-MAX))); } catch { /* full or blocked */ }
  }, 1000);
}

function stamp(): string {
  const d = new Date();
  const p = (n: number, w = 2) => String(n).padStart(w, '0');
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`;
}

/** Add a line to the log. */
export function dlog(tag: string, msg: string) {
  const l = `${stamp()} [${tag}] ${msg}`;
  lines.push(l);
  if (lines.length > MAX) lines.splice(0, lines.length - MAX);
  if (dbg.console) console.debug(l);
  persist();
}

/** Counters for the current second; `last` holds the finished second shown in the overlay. */
interface Counts { frames: number; maxFrame: number; snaps: number; maxSnapGap: number; keys: number; pad: number; touch: number; sent: number; moves: number; corrections: number }
const zero = (): Counts => ({ frames: 0, maxFrame: 0, snaps: 0, maxSnapGap: 0, keys: 0, pad: 0, touch: 0, sent: 0, moves: 0, corrections: 0 });

export const dbg = {
  overlay: false,
  console: (() => { try { return new URLSearchParams(location.search).has('debug'); } catch { return false; } })(),
  cur: zero(),
  last: zero(),
  /** what the overlay shows besides the counters (filled by main each frame) */
  info: {} as Record<string, string | number>,
  inGame: false,
};
if (dbg.console) dbg.overlay = true;

let secStart = 0;
let lastFrame = 0;
let lastSnap = 0;
let summaryAt = 0;

/** Once per animation frame (before the frame is skipped or drawn). */
export function frameTick(now: number) {
  if (lastFrame) {
    const dt = now - lastFrame;
    dbg.cur.maxFrame = Math.max(dbg.cur.maxFrame, dt);
    // a hidden tab stops frames for a while: that is no hitch
    if (dt > HITCH_MS && visible()) dlog('hitch', `frame took ${Math.round(dt)} ms`);
  }
  lastFrame = now;
  dbg.cur.frames++;
  if (!secStart) secStart = now;
  if (now - secStart >= 1000) {
    const c = dbg.cur;
    dbg.last = c;
    dbg.cur = zero();
    secStart = now;
    // a line every 10 s in the game, or at once when the second looked bad
    const bad = dbg.inGame && (c.corrections > 2 || c.maxSnapGap > SNAP_GAP_MS || c.snaps < 10);
    if (dbg.inGame && (bad || now - summaryAt >= 10000)) {
      summaryAt = now;
      dlog(bad ? 'slow' : 'stats', `fps ${c.frames} maxFrame ${Math.round(c.maxFrame)}ms · snaps ${c.snaps}/s maxGap ${Math.round(c.maxSnapGap)}ms · ` +
        `in key ${c.keys} pad ${c.pad} touch ${c.touch} · sent ${c.sent} (moves ${c.moves}) · snapbacks ${c.corrections} · ` +
        Object.entries(dbg.info).map(([k, v]) => k + ' ' + v).join(' · '));
    }
  }
}

/** A snapshot arrived. */
export function snapTick(now: number) {
  if (lastSnap && dbg.inGame) {
    const gap = now - lastSnap;
    dbg.cur.maxSnapGap = Math.max(dbg.cur.maxSnapGap, gap);
    if (gap > SNAP_GAP_MS && visible()) dlog('net', `no snapshot for ${Math.round(gap)} ms`);
  }
  lastSnap = now;
  dbg.cur.snaps++;
}

export function resetSnapClock() { lastSnap = 0; }

/** The whole log as text. */
export function logText(): string { return lines.join('\n') + '\n'; }

/** Save the log as a file. */
export function downloadLog() {
  const a = document.createElement('a');
  a.href = URL.createObjectURL(new Blob([logText()], { type: 'text/plain' }));
  a.download = 'emberdeep-' + new Date().toISOString().replace(/[:.]/g, '-') + '.log';
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

export function clearLog() { lines.length = 0; persist(); }

/** The overlay lines (F9). */
export function overlayLines(): string[] {
  const c = dbg.last;
  return [
    `fps ${c.frames}  max frame ${Math.round(c.maxFrame)} ms`,
    `snaps ${c.snaps}/s  max gap ${Math.round(c.maxSnapGap)} ms  snapbacks ${c.corrections}`,
    `input/s key ${c.keys} pad ${c.pad} touch ${c.touch}  sent ${c.sent} (moves ${c.moves})`,
    ...Object.entries(dbg.info).map(([k, v]) => `${k}: ${v}`),
    'F9 hide · /log save the log',
  ];
}

if (browser) (window as unknown as { emberdeep: object }).emberdeep = {
  log: () => console.log(logText()),
  download: downloadLog,
  clear: clearLog,
  verbose: (on = true) => { dbg.console = on; },
};
