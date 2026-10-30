import { Net } from './net';
import { GameState } from './state';
import { Fb, W, H } from './term/fb';
import { TermCanvas } from './term/canvas';
import type { App, Screen } from './app';
import { GameScreen } from './screens/game';

const canvas = document.getElementById('screen') as HTMLCanvasElement;
const term = new TermCanvas(canvas);
const st = new GameState();
const net = new Net();
const fb = new Fb(W, H, [9, 9, 14]);

let screen: Screen;
const game = new GameScreen({ net, st, term, setScreen: () => {}, toast: () => {} } as App);
screen = game;

net.onMsg = (m) => {
  if (m.t === 'hb') { net.send({ t: 'hbAck', n: m.n }); return; }
  st.handle(m, performance.now());
};
window.addEventListener('keydown', (e) => screen.onKey?.(e));
window.addEventListener('keyup', (e) => screen.onKeyUp?.(e));
canvas.addEventListener('mousedown', (e) => screen.onMouseDown?.(e, term.cellAt(e)));
canvas.addEventListener('mousemove', (e) => screen.onMouseMove?.(e, term.cellAt(e)));
canvas.addEventListener('auxclick', (e) => e.preventDefault());

let last = 0;
function frame(now: number) {
  requestAnimationFrame(frame);
  if (now - last < 30) return;
  last = now;
  screen.render(fb, now);
  term.draw(fb);
}

net.connect(null, { t: 'hello', name: 'solo', cls: 'sorcerer' }).then(() => { game.onEnter(); });
requestAnimationFrame(frame);
