declare const process: { argv: string[]; exit(n: number): never };
// Headless smoke test: connects to a running server, feeds real messages through GameState and renders
// frames with the real screens, then prints one frame as text. Usage: node .smoke.mjs ws://localhost:8080 [cls]
import { GameState } from '../src/state';
import { Fb, W, H } from '../src/term/fb';
import { GameScreen } from '../src/screens/game';
import type { App } from '../src/app';

const url = process.argv[2] || 'ws://localhost:8080/ws';
const cls = process.argv[3] || 'sorcerer';
const st = new GameState();
const sent: any[] = [];
const ws = new WebSocket(url);
const app = { net: { send: (m: any) => { sent.push(m); ws.send(JSON.stringify(m)); } }, st, term: null, setScreen() {}, toast() {} } as unknown as App;
const game = new GameScreen(app);
ws.onopen = () => ws.send(JSON.stringify({ t: 'hello', name: 'smoke', cls }));
let snaps = 0;
ws.onmessage = (ev) => {
  const m = JSON.parse(ev.data as string);
  if (m.t === 'hb') { ws.send(JSON.stringify({ t: 'hbAck', n: m.n })); return; }
  if (m.t === 'phase' && m.phase === 'game') game.onEnter();
  if (m.t === 'snap') snaps++;
  st.handle(m, performance.now());
};
const fb = new Fb(W, H, [9, 9, 14]);
function dump() {
  const lines: string[] = [];
  for (let y = 0; y < H; y++) lines.push(fb.ch.slice(y * W, (y + 1) * W).join(''));
  return lines.join('\n');
}
setTimeout(() => { game.held.add('d'); }, 800);
setTimeout(() => { (game as any).cast(1); }, 1500);
setTimeout(() => {
  let t = performance.now();
  for (let i = 0; i < 10; i++) { game.render(fb, t); t += 33; }
  console.log(dump());
  console.log('snaps', snaps, 'sent', sent.length, 'pos', game.pred.x, game.pred.y, 'ents', st.recs.at(-1)?.ents.length);
  // timing of a frame
  const t0 = performance.now();
  for (let i = 0; i < 100; i++) game.render(fb, performance.now());
  console.log('frame ms', ((performance.now() - t0) / 100).toFixed(2));
  process.exit(0);
}, 2600);
