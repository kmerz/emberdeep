import type { Ent } from '../proto';
import { GameState, INTERP_MS } from '../state';

/** Entities to draw at time `now`: remote things are rendered INTERP_MS behind (cell-snapped, no tweening);
 *  only the local hero is taken from the latest snapshot / prediction. */
export function displayEnts(st: GameState, now: number): Ent[] {
  const recs = st.recs;
  if (!recs.length) return [];
  const rt = now - INTERP_MS;
  let rec = recs[0];
  for (const r of recs) {
    if (r.t <= rt) rec = r; else break;
  }
  const latest = recs[recs.length - 1];
  const out: Ent[] = [];
  for (const e of rec.ents) {
    if (e.k === 'h' && e.id === st.youId) continue;
    out.push(e);
  }
  const me = latest.ents.find((e) => e.k === 'h' && e.id === st.youId);
  if (me) out.push(me);
  return out;
}
