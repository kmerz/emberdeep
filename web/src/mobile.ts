// Phones and tablets: which layout to draw (the full 160×48 grid or a zoomed crop) and whether to show touch controls.
import { W, H } from './term/fb';
import { CELL_W, CELL_H, pageSize } from './term/canvas';

const STORE = 'emberdeep:layout';
/** Below this scale the full grid gets too small to read (text under ~9px), so phones get the zoomed layout. */
export const COMPACT_BELOW = 0.6;
/** Rows of the zoomed game view; its width follows the screen's shape. */
export const COMPACT_ROWS = 24;

export const layout = {
  /** draw the zoomed crop instead of the whole grid */
  compact: false,
  /** show the on-screen stick and buttons */
  touch: false,
  /** columns of the zoomed game view */
  cols: 80,
};

function override(): string {
  try {
    const q = new URLSearchParams(location.search).get('layout');
    if (q === 'compact' || q === 'full') localStorage.setItem(STORE, q);
    else if (q === 'auto') localStorage.removeItem(STORE);
    return localStorage.getItem(STORE) ?? '';
  } catch { return ''; }
}

/** How big the full grid would be on a screen of this size (CSS px), held sideways. */
export function landscapeScale(w: number, h: number): number {
  const long = Math.max(w, h), short = Math.min(w, h);
  return Math.min(long / (W * CELL_W), short / (H * CELL_H));
}

/** Columns for the zoomed game view on a page of this shape: as wide as the screen, 64-112. */
export function compactCols(w: number, h: number): number {
  const cols = Math.round((COMPACT_ROWS * CELL_H * (w / Math.max(1, h))) / CELL_W);
  return Math.max(64, Math.min(112, cols));
}

const forced = override();

/** Re-check the layout (on load and when the page is resized or turned). */
export function updateLayout() {
  // the device screen, not the window: it does not change when the phone keyboard opens
  layout.compact = forced ? forced === 'compact' : landscapeScale(screen.width, screen.height) < COMPACT_BELOW;
  const p = pageSize();
  layout.cols = compactCols(Math.max(p.w, p.h), Math.min(p.w, p.h));
  document.body.classList.toggle('compact', layout.compact);
  document.body.classList.toggle('touch', layout.touch);
}

export function setTouch(on: boolean) {
  if (layout.touch === on) return;
  layout.touch = on;
  document.body.classList.toggle('touch', on);
}

try { layout.touch = matchMedia('(pointer: coarse)').matches; } catch { /* old browser */ }
