declare const process: { argv: string[]; exit(n: number): never };
// Creates a room over HTTP, joins two players, renders landing + lobby screens as text.
(globalThis as any).localStorage = undefined;
import { GameState } from '../src/state';
import { Fb, W, H } from '../src/term/fb';
import { LandingScreen } from '../src/screens/landing';
import { LobbyScreen } from '../src/screens/lobby';
import type { App } from '../src/app';

(globalThis as any).location = { pathname: '/', origin: 'http://localhost', protocol: 'http:', host: 'localhost' };
const base = process.argv[2] || 'http://localhost:8080';
const dump = (fb: Fb) => { const l: string[] = []; for (let y = 10; y < 40; y++) l.push(fb.ch.slice(y * W, (y + 1) * W).join('').trimEnd()); return l.join('\n'); };
const fb = new Fb(W, H, [9, 8, 14]);
const sent: any[] = [];
const st = new GameState();
const app = { net: { send: (m: any) => sent.push(m) }, st, term: null, setScreen() {}, toast() {} } as unknown as App;
const landing = new LandingScreen(app);
landing.name = 'kmerz';
landing.render(fb, 1000);
console.log(dump(fb));
const { code } = (await (await fetch(base + '/api/rooms', { method: 'POST' })).json()) as { code: string };
const open = (name: string, cls: string, onMsg: (m: any) => void) => new Promise<WebSocket>((res) => {
  const ws = new WebSocket(base.replace('http', 'ws') + '/ws?room=' + code);
  ws.onopen = () => { ws.send(JSON.stringify({ t: 'hello', name, cls })); res(ws); };
  ws.onmessage = (e) => { const m = JSON.parse(e.data as string); if (m.t === 'hb') ws.send(JSON.stringify({ t: 'hbAck', n: m.n })); onMsg(m); };
});
const a = await open('kmerz', 'warrior', (m) => st.handle(m, performance.now()));
const b = await open('nyx', 'rogue', () => {});
await new Promise((r) => setTimeout(r, 2500));
b.send(JSON.stringify({ t: 'ready' }));
const lobby = new LobbyScreen(app);
await new Promise((r) => setTimeout(r, 300));
lobby.render(fb, 2000);
console.log(dump(fb));
a.close(); b.close();
process.exit(0);
