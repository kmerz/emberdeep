import { Net } from './net';
import { GameState } from './state';
import { Fb, W, H, PINK, WH } from './term/fb';
import { TermCanvas } from './term/canvas';
import type { App, Screen } from './app';
import { GameScreen } from './screens/game';
import { LandingScreen, store } from './screens/landing';
import { LobbyScreen } from './screens/lobby';
import type { ClassId } from './proto';
import { GamepadInput, fakeKey } from './gamepad';

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

window.addEventListener('keydown', (e) => current.onKey?.(e));
window.addEventListener('keyup', (e) => current.onKeyUp?.(e));
canvas.addEventListener('mousedown', (e) => current.onMouseDown?.(e, term.cellAt(e)));
canvas.addEventListener('mousemove', (e) => current.onMouseMove?.(e, term.cellAt(e)));
canvas.addEventListener('auxclick', (e) => e.preventDefault());
canvas.addEventListener('contextmenu', (e) => e.preventDefault());

const pad = new GamepadInput();
const padSink = {
  down: (key: string, repeat: boolean) => current.onKey?.(fakeKey(key, repeat)),
  up: (key: string) => current.onKeyUp?.(fakeKey(key)),
};
window.addEventListener('gamepaddisconnected', () => pad.releaseAll(padSink));

let last = 0;
function frame(now: number) {
  requestAnimationFrame(frame);
  if (now - last < 30) return;
  last = now;
  pad.poll(now, current.padContext?.() ?? 'menu', padSink);
  current.render(fb, now);
  if (reconnectUntil) {
    const left = Math.max(0, Math.ceil((reconnectUntil - now) / 1000));
    const msg = ` connection lost — reconnecting (${left}s) `;
    fb.txt(Math.floor((W - msg.length) / 2), 24, msg, WH, PINK, 1);
  }
  term.draw(fb);
}
requestAnimationFrame(frame);
void H;
