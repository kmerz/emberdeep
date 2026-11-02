import { Net } from './net';
import { GameState } from './state';
import type { Fb } from './term/fb';
import type { TermCanvas } from './term/canvas';
import type { PadContext } from './gamepad';

export interface Screen {
  render(fb: Fb, now: number): void;
  onKey?(ev: KeyboardEvent): void;
  onKeyUp?(ev: KeyboardEvent): void;
  onMouseDown?(ev: MouseEvent, cell: [number, number]): void;
  onMouseMove?(ev: MouseEvent, cell: [number, number]): void;
  onEnter?(): void;
  onLeave?(): void;
  /** Which gamepad button map applies right now (default: 'menu'). */
  padContext?(): PadContext;
}

export interface App {
  net: Net;
  st: GameState;
  term: TermCanvas;
  setScreen(name: 'landing' | 'lobby' | 'game'): void;
  toast(msg: string): void;
}
