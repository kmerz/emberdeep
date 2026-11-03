import { Net } from './net';
import { GameState } from './state';
import type { Fb } from './term/fb';
import type { TermCanvas } from './term/canvas';
import type { PadContext } from './gamepad';
import type { Rect } from './term/canvas';
import type { TouchBtn } from './touch';

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
  /** The part of the grid to show: the whole grid unless `compact` (phones), then a readable crop. */
  view?(compact: boolean): Rect;
  /** On-screen buttons for touch play right now (besides the stick). */
  touchButtons?(): TouchBtn[];
  /** Whether the touch stick is active (it sends arrow keys). */
  touchStick?(): boolean;
  /** A finger held still on the canvas. */
  onLongPress?(cell: [number, number]): void;
}

export interface App {
  net: Net;
  st: GameState;
  term: TermCanvas;
  setScreen(name: 'landing' | 'lobby' | 'game'): void;
  toast(msg: string): void;
}
