import type { ServerMsg } from './proto';
import { dlog, dbg } from './debug';

let stale = 0;
/** how long opening a connection may take */
const CONNECT_MS = 6000;

export type Handler = (m: ServerMsg) => void;

export class Net {
  ws: WebSocket | null = null;
  onMsg: Handler = () => {};
  onClose: () => void = () => {};
  open = false;
  /** when the last message arrived (performance.now) */
  lastMsgAt = 0;

  connect(room: string | null, hello: object): Promise<void> {
    return new Promise((resolve, reject) => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      const q = room ? `?room=${encodeURIComponent(room)}` : '';
      if (this.ws && this.ws.readyState <= WebSocket.OPEN) { // never two sockets at once
        dlog('net', 'a new connection while the old one is still ' + (this.ws.readyState === WebSocket.OPEN ? 'open' : 'connecting') + ': closing it');
        this.close();
      }
      const ws = new WebSocket(`${proto}://${location.host}/ws${q}`);
      this.ws = ws;
      dlog('net', 'connecting to room ' + (room ?? '-'));
      // on a bad line a handshake can hang for a minute: give up on it and let the caller try again
      const timer = setTimeout(() => {
        if (ws !== this.ws || this.open) return;
        dlog('net', `no answer within ${CONNECT_MS / 1000} s: trying again`);
        this.close();
        reject(new Error('connection timed out'));
      }, CONNECT_MS);
      ws.onopen = () => {
        clearTimeout(timer);
        if (ws !== this.ws) { dlog('net', 'an old socket opened late: closing it'); ws.close(); return; } // a newer connect() took over
        dlog('net', 'connected'); this.open = true; this.lastMsgAt = performance.now(); this.send(hello); resolve();
      };
      ws.onerror = () => { dlog('net', 'socket error'); if (ws === this.ws && !this.open) reject(new Error('connection failed')); };
      ws.onmessage = (ev) => {
        if (ws !== this.ws) { stale++; if (stale === 1 || stale % 100 === 0) dlog('net', `ignored ${stale} message(s) from an old socket`); return; }
        this.lastMsgAt = performance.now();
        try { this.onMsg(JSON.parse(ev.data as string) as ServerMsg); } catch (e) { console.error('bad message', e); dlog('error', 'handling a message failed: ' + String((e as Error)?.stack ?? e).slice(0, 300)); }
      };
      ws.onclose = (ev) => {
        dlog('net', `closed (code ${ev.code}${ev.reason ? ', ' + ev.reason : ''})${ws !== this.ws ? ' — an old socket' : ''}`);
        if (ws !== this.ws) return; // an old socket closing late must not look like the current one dropping
        this.open = false; this.onClose();
      };
    });
  }

  send(m: object) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(m));
      dbg.cur.sent++;
      if ((m as { t?: string }).t === 'move') dbg.cur.moves++;
    }
  }
  /** The connection went silent: close it and handle it like a dropped connection (reconnect). */
  drop(why: string) {
    if (!this.ws) return;
    dlog('net', 'dropping the connection: ' + why);
    this.close();
    this.onClose();
  }

  /** Close the connection. Its close event comes later and is ignored: only the current socket counts. */
  close() {
    const ws = this.ws;
    this.ws = null;
    this.open = false;
    if (ws) { dlog('net', 'closing the connection'); ws.close(); }
  }
}
