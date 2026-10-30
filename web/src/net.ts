import type { ServerMsg } from './proto';

export type Handler = (m: ServerMsg) => void;

export class Net {
  ws: WebSocket | null = null;
  onMsg: Handler = () => {};
  onClose: () => void = () => {};
  open = false;

  connect(room: string | null, hello: object): Promise<void> {
    return new Promise((resolve, reject) => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      const q = room ? `?room=${encodeURIComponent(room)}` : '';
      const ws = new WebSocket(`${proto}://${location.host}/ws${q}`);
      this.ws = ws;
      ws.onopen = () => { this.open = true; this.send(hello); resolve(); };
      ws.onerror = () => { if (!this.open) reject(new Error('connection failed')); };
      ws.onmessage = (ev) => {
        try { this.onMsg(JSON.parse(ev.data as string) as ServerMsg); } catch (e) { console.error('bad message', e); }
      };
      ws.onclose = () => { this.open = false; this.onClose(); };
    });
  }

  send(m: object) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(m));
  }
  close() { this.ws?.close(); }
}
