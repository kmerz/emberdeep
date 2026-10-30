declare const process: { argv: string[]; exit(n: number): never };
// Headless smoke test against a running server: creates a room, joins N players (the first one is rendered with
// the real GameScreen), starts the run and prints a frame as text. Usage: npm run smoke -- http://localhost:8080 [n] [cls]
(globalThis as any).localStorage = undefined;
import { GameState } from '../src/state';
import { Fb, W, H } from '../src/term/fb';
import { GameScreen } from '../src/screens/game';
import type { App } from '../src/app';

const base = process.argv[2] || 'http://localhost:8080';
const n = parseInt(process.argv[3] || '3');
const cls = process.argv[4] || 'sorcerer';
const classes = [cls, 'warrior', 'rogue', 'monk'];
const names = ['kmerz', 'nyx', 'vela', 'oddjob'];
const { code } = (await (await fetch(base + '/api/rooms', { method: 'POST' })).json()) as { code: string };
const st = new GameState();
const sent: any[] = [];
let mainWs!: WebSocket;
const app = { net: { send: (m: any) => { sent.push(m); mainWs.send(JSON.stringify(m)); } }, st, term: null, setScreen() {}, toast() {} } as unknown as App;
const game = new GameScreen(app);
let snaps = 0;
const open = (i: number) => new Promise<WebSocket>((res) => {
  const ws = new WebSocket(base.replace('http', 'ws') + '/ws?room=' + code);
  ws.onopen = () => { ws.send(JSON.stringify({ t: 'hello', name: names[i], cls: classes[i] })); res(ws); };
  ws.onmessage = (e) => {
    const m = JSON.parse(e.data as string);
    if (m.t === 'hb') ws.send(JSON.stringify({ t: 'hbAck', n: m.n }));
    if (i === 0) {
      if (m.t === 'phase' && m.phase === 'game') game.onEnter();
      if (m.t === 'snap') snaps++;
      st.handle(m, performance.now());
    }
  };
});
mainWs = await open(0);
const others: WebSocket[] = [];
for (let i = 1; i < n; i++) others.push(await open(i));
await new Promise((r) => setTimeout(r, 300));
mainWs.send(JSON.stringify({ t: 'start' }));
await new Promise((r) => setTimeout(r, 600));
others.forEach((ws, i) => { let seq = 0; setInterval(() => ws.send(JSON.stringify({ t: 'move', dir: 2 + (i % 2) * 4, seq: ++seq })), 170); });
setTimeout(() => { game.held.add('d'); }, 500);
setTimeout(() => { (game as any).cast(1); }, 1500);
setTimeout(() => mainWs.send(JSON.stringify({ t: 'chat', text: 'hello party' })), 1600);
setTimeout(() => mainWs.send(JSON.stringify({ t: 'ping', cell: [st.recs.at(-1)!.ents[0].x + 3, st.recs.at(-1)!.ents[0].y] })), 1700);
await new Promise((r) => setTimeout(r, 2800));
const fb = new Fb(W, H, [9, 9, 14]);
let t = performance.now();
for (let i = 0; i < 10; i++) { game.render(fb, t); t += 33; }
const lines: string[] = [];
for (let y = 0; y < H; y++) lines.push(fb.ch.slice(y * W, (y + 1) * W).join('').trimEnd());
console.log(lines.join('\n'));
console.log('room', code, 'snaps', snaps, 'sent', sent.length, 'pred', game.pred.x, game.pred.y, 'party', st.party.map((p) => p.name + ':' + p.hp).join(','));
const t0 = performance.now();
for (let i = 0; i < 100; i++) game.render(fb, performance.now());
console.log('frame ms', ((performance.now() - t0) / 100).toFixed(2));
process.exit(0);
