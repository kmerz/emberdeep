import { Net } from './net';
import { GameState } from './state';
import { Fb, W, H, PINK, WH } from './term/fb';
import { TermCanvas, FULL } from './term/canvas';
import type { App, Screen } from './app';
import { GameScreen } from './screens/game';
import { LandingScreen, store } from './screens/landing';
import { LobbyScreen } from './screens/lobby';
import type { ClassId } from './proto';
import { GamepadInput, fakeKey, lastInput } from './gamepad';
import { sfx } from './audio';
import { layout, updateLayout, setTouch } from './mobile';
import { TouchControls, type TouchBtn } from './touch';
import * as kbd from './kbd';

const canvas = document.getElementById('screen') as HTMLCanvasElement;
const term = new TermCanvas(canvas);
const st = new GameState();
const net = new Net();
const fb = new Fb(W, H, [9, 9, 14]);

let session: { code: string; name: string; cls: ClassId } | null = null;
let fatal = '';
let reconnectUntil = 0;
let leaving = false;

const screens: Record<string, Screen & { onEnter?(): void }> = {};
let current: Screen = null as unknown as Screen;
let currentName = '';

const app: App = {
  net, st, term,
  setScreen(name) {
    if (name === currentName) return;
    if (name === 'landing' && (currentName === 'lobby' || currentName === 'game') && session) {
      // the player chose to leave the room: tell the server so it frees the slot now (no reconnect hold)
      leaving = true; session = null; net.send({ t: 'leave' }); net.close(); st.reset();
      history.replaceState(null, '', '/'); landing.fixedCode = null;
    }
    current?.onLeave?.();
    currentName = name;
    current = screens[name];
    current.onEnter?.();
  },
  toast() {},
};
const landing = new LandingScreen(app);
const lobby = new LobbyScreen(app);
const game = new GameScreen(app);
screens.landing = landing; screens.lobby = lobby; screens.game = game;
app.setScreen('landing');

// ---- session / reconnect ----

async function connect(code: string, name: string, cls: ClassId) {
  const token = store.get(`emberdeep:${code}:token`) ?? '';
  fatal = '';
  await net.connect(code, { t: 'hello', name, cls, token });
}

landing.onSubmit = async (mode, code, name, cls) => {
  try {
    if (mode === 'create') {
      const r = await fetch('/api/rooms', { method: 'POST' });
      if (!r.ok) throw new Error(r.status === 503 ? 'the server is full, try again later' : 'could not create a game');
      code = ((await r.json()) as { code: string }).code;
    }
    session = { code: code!, name, cls };
    leaving = false;
    await connect(code!, name, cls);
  } catch (e) {
    landing.error = (e as Error).message || 'connection failed';
    landing.busy = false;
    session = null;
  }
};

net.onMsg = (m) => {
  if (m.t === 'hb') { net.send({ t: 'hbAck', n: m.n }); return; }
  if (m.t === 'error') fatal = m.msg;
  st.handle(m, performance.now());
  if (m.t === 'welcome' && session) {
    store.set(`emberdeep:${m.code}:token`, m.token);
    reconnectUntil = 0;
    if (location.pathname !== '/r/' + m.code) history.replaceState(null, '', '/r/' + m.code);
    if (currentName === 'landing') app.setScreen('lobby');
  } else if (m.t === 'phase') {
    app.setScreen(m.phase === 'game' ? 'game' : 'lobby');
  }
};

net.onClose = () => {
  if (!session || leaving) return;
  if (fatal) { toLanding(fatal); return; }
  // connection dropped: try to resume with our session token for up to 60 s
  if (!reconnectUntil) reconnectUntil = performance.now() + 60000;
  tryReconnect();
};

function toLanding(msg: string) {
  session = null;
  reconnectUntil = 0;
  st.reset();
  landing.error = msg;
  landing.busy = false;
  if (location.pathname.startsWith('/r/') && /full|no such room|name/.test(msg)) { /* keep the link so the user can retry */ }
  app.setScreen('landing');
}

function tryReconnect() {
  if (!session) return;
  if (performance.now() > reconnectUntil) { toLanding('connection lost'); return; }
  setTimeout(async () => {
    if (!session || leaving) return;
    try { await connect(session.code, session.name, session.cls); } catch { tryReconnect(); }
  }, 1500);
}

// browsers only start audio after a user gesture (a touch only counts once the finger lifts)
for (const t of ['keydown', 'pointerdown', 'pointerup', 'touchend']) window.addEventListener(t, () => sfx.unlock(), { capture: true });
window.addEventListener('pointerdown', (e) => {
  lastInput.pad = false;
  if (e.pointerType === 'touch') setTouch(true);
}, { capture: true });
// the phone keyboard may only open at the end of a tap
window.addEventListener('pointerup', () => kbd.flush(), { capture: true });
window.addEventListener('keydown', (e) => {
  if (kbd.isKbd(e.target)) return; // typing into a field: the hidden input reports the text itself
  lastInput.pad = false;
  setTouch(false); // a real keyboard: hide the touch controls until the next touch
  const k = current.mapKey ? current.mapKey(e) : e;
  if (k) current.onKey?.(k);
  else if (e.key === 'Tab') e.preventDefault(); // an unbound Tab must not move the focus
});
window.addEventListener('keyup', (e) => {
  if (kbd.isKbd(e.target)) return;
  const k = current.mapKey ? current.mapKey(e) : e;
  if (k) current.onKeyUp?.(k);
});

// canvas: pointer events cover mouse and touch alike
let press: { id: number; x: number; y: number; timer: number } | null = null;
canvas.addEventListener('pointerdown', (e) => {
  e.preventDefault(); // no emulated mouse events afterwards (they would steal focus from the phone keyboard)
  const cell = term.cellAt(e);
  if (e.pointerType === 'touch') {
    if (press) clearTimeout(press.timer);
    const id = e.pointerId;
    press = { id, x: e.clientX, y: e.clientY, timer: window.setTimeout(() => { if (press?.id === id) { current.onLongPress?.(cell); press = null; } }, 550) };
  }
  current.onMouseDown?.(e, cell);
});
canvas.addEventListener('pointermove', (e) => {
  if (press && e.pointerId === press.id && Math.hypot(e.clientX - press.x, e.clientY - press.y) > 12) { clearTimeout(press.timer); press = null; }
  if (e.pointerType !== 'touch') current.onMouseMove?.(e, term.cellAt(e));
});
for (const t of ['pointerup', 'pointercancel']) canvas.addEventListener(t, (e) => { if (press && (e as PointerEvent).pointerId === press.id) { clearTimeout(press.timer); press = null; } });
canvas.addEventListener('auxclick', (e) => e.preventDefault());
canvas.addEventListener('contextmenu', (e) => e.preventDefault());
document.addEventListener('gesturestart', (e) => e.preventDefault()); // iOS pinch zoom

updateLayout();
window.addEventListener('resize', updateLayout);
window.addEventListener('orientationchange', updateLayout);

const touch = new TouchControls({
  down: (key: string, repeat: boolean) => { lastInput.pad = false; current.onKey?.(fakeKey(key, repeat)); },
  up: (key: string) => current.onKeyUp?.(fakeKey(key)),
});

function toggleFullscreen() {
  const d = document as Document & { webkitFullscreenElement?: Element; webkitExitFullscreen?: () => void };
  const el = document.documentElement as HTMLElement & { webkitRequestFullscreen?: () => void };
  if (document.fullscreenElement || d.webkitFullscreenElement) { (document.exitFullscreen?.() ?? d.webkitExitFullscreen?.()); return; }
  const req = el.requestFullscreen as (() => Promise<void>) | undefined; // missing on iPhone Safari
  const p = req ? req.call(el) : (el.webkitRequestFullscreen?.(), undefined);
  // phones: also try to hold landscape (only allowed in fullscreen, and not on every browser)
  p?.then(() => (screen.orientation as ScreenOrientation & { lock?: (o: string) => Promise<void> }).lock?.('landscape')).catch(() => {});
}
const fsEl = document.documentElement as HTMLElement & { webkitRequestFullscreen?: unknown };
const canFullscreen = typeof (fsEl.requestFullscreen as unknown) === 'function' || typeof fsEl.webkitRequestFullscreen === 'function';
const FS_BTN: TouchBtn = { key: 'fullscreen', label: '⛶', area: 'top', tap: toggleFullscreen };

const pad = new GamepadInput();
game.padInput = pad;
const padSink = {
  down: (key: string, repeat: boolean) => {
    // playing with the pad: forget the mouse cursor, so skills and thrown items aim at the nearest enemy
    game.mouse = [-1, -1];
    lastInput.pad = true;
    current.onKey?.(fakeKey(key, repeat));
  },
  up: (key: string) => current.onKeyUp?.(fakeKey(key)),
};
window.addEventListener('gamepaddisconnected', () => pad.releaseAll(padSink));

let last = 0;
function frame(now: number) {
  requestAnimationFrame(frame);
  if (now - last < 30) return;
  last = now;
  const ctx = current.padContext?.() ?? 'menu';
  pad.poll(now, ctx, padSink);
  if (pad.active) setTouch(false);
  current.render(fb, now);
  term.setView(current.view?.(layout.compact) ?? FULL);
  touch.show(layout.touch && ctx !== 'chat' && !kbd.kbdOpen());
  if (layout.touch) {
    const btns = current.touchButtons?.() ?? [];
    touch.update(now, ctx, canFullscreen && ctx !== 'panel' ? [...btns, FS_BTN] : btns, current.touchStick?.() ?? false);
  }
  if (reconnectUntil) {
    const left = Math.max(0, Math.ceil((reconnectUntil - now) / 1000));
    const msg = ` connection lost — reconnecting (${left}s) `;
    const v = term.view; // keep it inside the part of the grid that is shown
    fb.txt(v.x + Math.max(0, Math.floor((v.w - msg.length) / 2)), v.y + Math.floor(v.h / 2), msg, WH, PINK, 1);
  }
  term.draw(fb);
}
requestAnimationFrame(frame);
void H;
