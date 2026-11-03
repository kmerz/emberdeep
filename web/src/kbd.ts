// The phone keyboard: a hidden <input> that gets focus when a text field on the canvas is tapped.
// Browsers only open the keyboard from a user gesture, so request() is followed by flush() in a pointerup handler.

export interface KbdTarget {
  value: string;
  max: number;
  onInput(v: string): void;
  /** Enter (true) or the keyboard was closed some other way (false). */
  onDone(enter: boolean): void;
}

let el: HTMLInputElement | null = null;
let target: KbdTarget | null = null;
let pending: KbdTarget | null = null;
let pendingAt = 0;
/** A request older than this was not followed by a tap (e.g. Enter on a real keyboard): drop it. */
const PENDING_MS = 1000;

function input(): HTMLInputElement {
  if (el) return el;
  el = document.createElement('input');
  el.id = 'kbd';
  el.type = 'text';
  el.autocomplete = 'off';
  el.spellcheck = false;
  el.setAttribute('autocapitalize', 'off');
  el.setAttribute('autocorrect', 'off');
  el.enterKeyHint = 'done';
  document.body.appendChild(el);
  el.addEventListener('input', () => target?.onInput(el!.value));
  el.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === 'Escape' || e.key === 'Tab') { e.preventDefault(); finish(e.key !== 'Escape'); }
  });
  el.addEventListener('blur', () => finish(false));
  return el;
}

function finish(enter: boolean) {
  const t = target;
  target = null;
  if (el && document.activeElement === el) el.blur();
  t?.onDone(enter);
}

/** Open the keyboard for this field at the end of the current tap. */
export function request(t: KbdTarget) { pending = t; pendingAt = performance.now(); }

/** Called from pointerup: opens the keyboard a tap asked for. */
export function flush() {
  if (!pending) return;
  const t = pending;
  pending = null;
  if (performance.now() - pendingAt > PENDING_MS) return;
  const i = input();
  if (target && target !== t) finish(false);
  i.maxLength = t.max;
  i.value = t.value;
  target = t;
  i.focus();
}

/** Close the keyboard (without Enter). */
export function close() { pending = null; if (target) finish(false); }

export function isKbd(t: EventTarget | null): boolean { return !!el && t === el; }
export function kbdOpen(): boolean { return target !== null; }
/** The keyboard is open or about to open (a tap asked for it just now). */
export function busy(): boolean { return target !== null || (pending !== null && performance.now() - pendingAt <= PENDING_MS); }
