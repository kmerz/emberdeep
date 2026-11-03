/**
 * Key and button bindings for the world (walking about, fighting). Every action has a fixed "game key": the
 * key the game screen acts on (the same key the default keyboard layout uses). A real key press is translated
 * to its action's game key before the screen sees it; pad buttons send game keys straight away. Panels and
 * menus keep their own fixed keys. The player can rebind both in the game menu (CONTROLS); the choice is
 * kept in this browser.
 */

// standard-mapping pad buttons
export const BTN = { A: 0, B: 1, X: 2, Y: 3, LB: 4, RB: 5, LT: 6, RT: 7, SELECT: 8, START: 9, L3: 10, R3: 11 } as const;
export const BTN_NAMES: Record<number, string> = { 0: 'A', 1: 'B', 2: 'X', 3: 'Y', 4: 'LB', 5: 'RB', 6: 'LT', 7: 'RT', 8: 'SELECT', 9: 'START', 10: 'L3', 11: 'R3' };

export type ActionId = 'up' | 'down' | 'left' | 'right' | 'skill1' | 'skill2' | 'skill3' | 'potion' | 'item4' | 'item5'
  | 'revive' | 'ping' | 'map' | 'bag' | 'tree' | 'journal' | 'sound' | 'chat' | 'menu';

export interface ActionDef {
  id: ActionId;
  label: string;
  key: string;        // the game key the screen acts on (as ev.key)
  kb: string[];       // default keyboard keys (lowercase ev.key), up to two
  pad: number | null; // default pad button
  noPad?: boolean;    // a pad can't do this (movement is the stick, chat needs typing)
  fixed?: boolean;    // can't be rebound (the menu: Esc and START always open it)
}

export const ACTIONS: ActionDef[] = [
  { id: 'up', label: 'Move up', key: 'ArrowUp', kb: ['w', 'arrowup'], pad: null, noPad: true },
  { id: 'down', label: 'Move down', key: 'ArrowDown', kb: ['s', 'arrowdown'], pad: null, noPad: true },
  { id: 'left', label: 'Move left', key: 'ArrowLeft', kb: ['a', 'arrowleft'], pad: null, noPad: true },
  { id: 'right', label: 'Move right', key: 'ArrowRight', kb: ['d', 'arrowright'], pad: null, noPad: true },
  { id: 'skill1', label: 'Skill 1 (pad: also revive)', key: '1', kb: ['1'], pad: BTN.A },
  { id: 'skill2', label: 'Skill 2', key: '2', kb: ['2'], pad: BTN.X },
  { id: 'skill3', label: 'Skill 3', key: '3', kb: ['3'], pad: BTN.Y },
  { id: 'potion', label: 'Potion', key: 'q', kb: ['q'], pad: BTN.LB },
  { id: 'item4', label: 'Item 4', key: '4', kb: ['4'], pad: BTN.RB },
  { id: 'item5', label: 'Item 5', key: '5', kb: ['5'], pad: BTN.RT },
  { id: 'revive', label: 'Revive (hold)', key: 'e', kb: ['e'], pad: null },
  { id: 'ping', label: 'Ping', key: 'g', kb: ['g'], pad: BTN.B },
  { id: 'map', label: 'Map (zoomed view)', key: 'Tab', kb: ['tab'], pad: BTN.LT },
  { id: 'bag', label: 'Rucksack', key: 'i', kb: ['i'], pad: BTN.SELECT },
  { id: 'tree', label: 'Skill tree', key: 't', kb: ['t'], pad: BTN.R3 },
  { id: 'journal', label: 'Journal', key: 'j', kb: ['j'], pad: BTN.L3 },
  { id: 'sound', label: 'Music / sound / off', key: 'm', kb: ['m'], pad: null },
  { id: 'chat', label: 'Chat', key: 'Enter', kb: ['enter'], pad: null, noPad: true },
  { id: 'menu', label: 'Menu', key: 'Escape', kb: ['escape'], pad: BTN.START, fixed: true },
];
const BY_ID = new Map(ACTIONS.map((a) => [a.id, a]));
/** Panel toggles: the only bindings that also work while a panel is open (to close it again). */
const TOGGLES = new Set<ActionId>(['bag', 'tree', 'journal']);

export interface Binds { kb: Record<string, string[]>; pad: Record<string, number | null> }

const STORE = 'emberdeep:binds';
export function defaults(): Binds {
  const b: Binds = { kb: {}, pad: {} };
  for (const a of ACTIONS) { b.kb[a.id] = [...a.kb]; b.pad[a.id] = a.pad; }
  return b;
}

function load(): Binds {
  const b = defaults();
  try {
    const s = JSON.parse(localStorage.getItem(STORE) ?? 'null') as Partial<Binds> | null;
    for (const a of ACTIONS) {
      if (a.fixed) continue;
      const k = s?.kb?.[a.id];
      if (Array.isArray(k) && k.every((x) => typeof x === 'string')) b.kb[a.id] = k.slice(0, 2);
      const p = s?.pad?.[a.id];
      if (!a.noPad && (p === null || (typeof p === 'number' && BTN_NAMES[p]))) b.pad[a.id] = p;
    }
  } catch { /* nothing saved, or a private window */ }
  return b;
}

export let binds: Binds = load();

function save() { try { localStorage.setItem(STORE, JSON.stringify(binds)); } catch { /* ignore */ } }

/** Replace the bindings (tests, and "reset"). */
export function setBinds(b: Binds, persist = true) { binds = b; if (persist) save(); }

export function resetKeyboard() { const d = defaults(); binds = { kb: d.kb, pad: binds.pad }; save(); }
export function resetPad() { const d = defaults(); binds = { kb: binds.kb, pad: d.pad }; save(); }

/** Bind keyboard key `key` (lowercase ev.key) as action `id`'s key number `slot` (0 or 1). An action that had
 *  the key gets this action's old key in its place (a swap), so nothing is left unreachable by accident. */
export function bindKey(id: ActionId, slot: number, key: string) {
  const a = BY_ID.get(id);
  if (!a || a.fixed || key === 'escape') return;
  const mine = binds.kb[id] ?? [];
  const old = mine[slot];
  for (const o of ACTIONS) {
    if (o.id === id) continue;
    const list = binds.kb[o.id] ?? [];
    const i = list.indexOf(key);
    if (i < 0) continue;
    if (old && !list.includes(old)) list[i] = old; else list.splice(i, 1);
    binds.kb[o.id] = list;
  }
  const next = mine.filter((k) => k !== key);
  next.splice(Math.min(slot, next.length), slot < next.length ? 1 : 0, key);
  binds.kb[id] = next.slice(0, 2);
  save();
}

/** Clear action `id`'s keyboard key number `slot`. */
export function unbindKey(id: ActionId, slot: number) {
  const a = BY_ID.get(id);
  if (!a || a.fixed) return;
  binds.kb[id] = (binds.kb[id] ?? []).filter((_, i) => i !== slot);
  save();
}

/** Bind pad button `btn` to action `id`; the action that had it gets this action's old button. */
export function bindPad(id: ActionId, btn: number | null) {
  const a = BY_ID.get(id);
  if (!a || a.fixed || a.noPad || btn === BTN.START) return;
  const old = binds.pad[id] ?? null;
  if (btn !== null) for (const o of ACTIONS) if (o.id !== id && binds.pad[o.id] === btn) binds.pad[o.id] = o.fixed ? btn : old;
  binds.pad[id] = btn;
  save();
}

/** The game key a real key press stands for in the world: the bound action's key, null if the key is one the
 *  game knows but no longer bound to anything (it does nothing), or the key itself if the game never uses it.
 *  `panel`: only panel toggles translate (so a panel's own letters keep working). */
export function translateKey(raw: string, panel = false): string | null {
  const k = raw.toLowerCase();
  if (k === 'escape') return 'Escape';
  for (const a of ACTIONS) {
    if ((binds.kb[a.id] ?? []).includes(k)) return panel && !TOGGLES.has(a.id) ? raw : a.key;
  }
  if (panel) return raw;
  for (const a of ACTIONS) if (a.key.toLowerCase() === k || a.kb.includes(k)) return null;
  return raw;
}

/** Pad buttons → game keys in the world. */
export function padWorldMap(): Partial<Record<number, string>> {
  const m: Partial<Record<number, string>> = {};
  for (const a of ACTIONS) { const b = binds.pad[a.id]; if (b !== null && b !== undefined) m[b] = a.key; }
  return m;
}

const KEY_NAMES: Record<string, string> = {
  arrowup: '↑', arrowdown: '↓', arrowleft: '←', arrowright: '→', ' ': 'SPACE', escape: 'ESC', enter: 'ENTER', tab: 'TAB',
  backspace: 'BKSP', shift: 'SHIFT', control: 'CTRL', alt: 'ALT', meta: 'META', capslock: 'CAPS',
};
/** A key as shown on screen: "Q", "↑", "SPACE". */
export function keyName(k: string): string { return KEY_NAMES[k] ?? k.toUpperCase(); }

/** The keyboard key shown in hints for a game key ('' when that action has no key). */
export function kbLabel(gameKey: string): string {
  const a = ACTIONS.find((x) => x.key === gameKey || x.key.toLowerCase() === gameKey.toLowerCase());
  if (!a) return '';
  const k = binds.kb[a.id]?.[0];
  return k ? keyName(k) : '';
}

export function action(id: ActionId): ActionDef { return BY_ID.get(id)!; }

/** A key event standing for another key; preventDefault still reaches the real event. */
export function withKey(ev: KeyboardEvent, key: string): KeyboardEvent {
  return { key, repeat: ev.repeat, ctrlKey: ev.ctrlKey, metaKey: ev.metaKey, altKey: ev.altKey, shiftKey: ev.shiftKey,
    preventDefault: () => ev.preventDefault() } as unknown as KeyboardEvent;
}
