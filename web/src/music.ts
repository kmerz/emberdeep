// Music: a small generative score per floor, made from oscillators like the rest of the sound.
//
// Each theme has a style (key, scale, tempo, instruments, drums). The floor's seed and depth turn the style into a
// song: a four-chord loop, a bass line and two melodies. Everyone in a party hears the same song, every floor has
// its own, and deeper floors play faster and fuller. The song moves through sections (intro, verse, full, breakdown)
// so it never sits on one loop for long, and drums and bass come in when enemies are near (setDanger).

type Wave = OscillatorType;
type Drums = 'none' | 'soft' | 'tribal' | 'march';

export interface Style {
  root: number;      // MIDI note of the key
  mode: number[];    // scale, semitones above the root
  bpm: number;
  pad: Wave; lead: Wave; bass: Wave;
  drums: Drums;
  density: number;   // how busy the melody is (0..1)
}

const MAJOR = [0, 2, 4, 5, 7, 9, 11], AEOLIAN = [0, 2, 3, 5, 7, 8, 10], PHRYGIAN = [0, 1, 3, 5, 7, 8, 10];
const DORIAN = [0, 2, 3, 5, 7, 9, 10], LOCRIAN = [0, 1, 3, 5, 6, 8, 10], HARMONIC = [0, 2, 3, 5, 7, 8, 11];
const PHRYG_DOM = [0, 1, 4, 5, 7, 8, 10];

export const STYLES: Record<string, Style> = {
  village: { root: 60, mode: MAJOR, bpm: 92, pad: 'sine', lead: 'triangle', bass: 'triangle', drums: 'soft', density: 0.55 },
  cathedral: { root: 57, mode: AEOLIAN, bpm: 70, pad: 'triangle', lead: 'sine', bass: 'sine', drums: 'none', density: 0.35 },
  crypt: { root: 55, mode: PHRYGIAN, bpm: 64, pad: 'sine', lead: 'triangle', bass: 'sine', drums: 'soft', density: 0.3 },
  hive: { root: 62, mode: DORIAN, bpm: 116, pad: 'sawtooth', lead: 'square', bass: 'square', drums: 'tribal', density: 0.6 },
  caverns: { root: 59, mode: DORIAN, bpm: 84, pad: 'triangle', lead: 'sine', bass: 'triangle', drums: 'soft', density: 0.4 },
  dark: { root: 52, mode: LOCRIAN, bpm: 56, pad: 'sine', lead: 'sine', bass: 'sine', drums: 'none', density: 0.2 },
  abyss: { root: 50, mode: PHRYG_DOM, bpm: 100, pad: 'sawtooth', lead: 'sawtooth', bass: 'sawtooth', drums: 'march', density: 0.5 },
  ember: { root: 53, mode: HARMONIC, bpm: 112, pad: 'sawtooth', lead: 'square', bass: 'sawtooth', drums: 'tribal', density: 0.6 },
};

/** A note: step in the pattern, scale degree (0 = root, 7 = an octave up), length in steps. */
export interface Note { step: number; deg: number; len: number }

export interface Song {
  style: Style;
  bpm: number;
  chords: number[];        // scale degree of each bar's chord (a 4-bar loop)
  bass: (number | null)[]; // 16 steps: degree above the chord root, or rest
  melodies: Note[][];      // phrases of 32 steps (2 bars)
  drums: string[][];       // 16 steps: which drums hit
  depth: number;
}

function rng(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function hashStr(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
  return h >>> 0;
}

const DRUMS: Record<Drums, string[][]> = {
  none: Array.from({ length: 16 }, () => []),
  soft: Array.from({ length: 16 }, (_, i) => (i === 0 ? ['kick'] : i === 8 ? ['hat'] : i === 12 ? ['hat'] : [])),
  tribal: Array.from({ length: 16 }, (_, i) => [...(i === 0 || i === 6 || i === 10 ? ['kick'] : []), ...(i === 4 || i === 12 ? ['tom'] : []), ...(i % 2 === 0 ? ['hat'] : [])]),
  march: Array.from({ length: 16 }, (_, i) => [...(i === 0 || i === 8 ? ['kick'] : []), ...(i === 4 || i === 12 ? ['snare'] : []), ...(i % 4 === 2 ? ['hat'] : [])]),
};

/** Builds the song for a floor. The same theme, depth and seed always give the same song. */
export function makeSong(theme: string, depth: number, seed: string): Song | null {
  const style = STYLES[theme];
  if (!style) return null;
  const r = rng(hashStr(theme + ':' + depth + ':' + seed));
  const deep = Math.min(1, Math.max(0, (depth - 1) / 9));
  // chords: start home, then three of the common moves
  const moves = [3, 4, 5, 2, 6, 3, 4];
  const chords = [0, ...Array.from({ length: 3 }, () => moves[Math.floor(r() * moves.length)])];
  // bass: the root on the bar, the fifth or root halfway, a few passing notes on deeper floors
  const bass: (number | null)[] = Array.from({ length: 16 }, (_, i) => {
    if (i === 0) return 0;
    if (i === 8) return r() < 0.5 ? 4 : 0;
    if (i % 2 === 0 && r() < 0.15 + 0.35 * deep) return [0, 2, 4, 7][Math.floor(r() * 4)];
    return null;
  });
  // melodies: a random walk on the scale; chord tones on strong beats; rests between phrases
  const melody = (): Note[] => {
    const out: Note[] = [];
    let deg = 4 + Math.floor(r() * 3), step = 0;
    while (step < 32) {
      const strong = step % 8 === 0;
      if (!strong && r() > style.density + 0.2 * deep) { step += 2; continue; }
      const len = [2, 2, 4, 4, 6, 8][Math.floor(r() * 6)];
      deg = Math.max(0, Math.min(11, deg + [-2, -1, -1, 0, 1, 1, 2, 3][Math.floor(r() * 8)]));
      if (strong) deg = [0, 2, 4, 7, 9][Math.floor(r() * 5)];
      out.push({ step, deg, len: Math.min(len, 32 - step) });
      step += len;
    }
    return out;
  };
  return { style, bpm: Math.round(style.bpm * (1 + 0.2 * deep)), chords, bass, melodies: [melody(), melody()], drums: DRUMS[style.drums], depth };
}

/** Song sections, 4 bars each: which layers play. The form loops. */
export const FORM: { pad: boolean; bass: boolean; lead: number; drums: boolean }[] = [
  { pad: true, bass: false, lead: -1, drums: false }, // intro
  { pad: true, bass: true, lead: 0, drums: false },   // verse
  { pad: true, bass: true, lead: 1, drums: true },    // full
  { pad: true, bass: true, lead: 0, drums: true },
  { pad: true, bass: false, lead: 1, drums: false },  // breakdown
];

/** MIDI note of scale degree deg (degrees past the scale wrap into higher octaves). */
export function noteOf(style: Style, deg: number, octave = 0): number {
  const n = style.mode.length, o = Math.floor(deg / n), d = ((deg % n) + n) % n;
  return style.root + style.mode[d] + 12 * (o + octave);
}

const hz = (midi: number) => 440 * Math.pow(2, (midi - 69) / 12);

export class Music {
  private song: Song | null = null;
  private key = '';
  private step = 0;      // 16th notes since the song started
  private nextT = 0;     // audio time of the next step
  private vol = 1;       // 0 = music off
  private danger = 0;    // 0..1, smoothed
  private wantDanger = 0;
  private out: GainNode;
  private echo: DelayNode;
  private noise: AudioBuffer;

  constructor(private ctx: AudioContext, dest: AudioNode, noise: AudioBuffer) {
    this.noise = noise;
    this.out = ctx.createGain();
    this.out.gain.value = 0;
    this.out.connect(dest);
    // a quiet echo gives the lead some room
    this.echo = ctx.createDelay(1);
    const fb = ctx.createGain(), wet = ctx.createGain();
    fb.gain.value = 0.3;
    wet.gain.value = 0.25;
    this.echo.connect(fb).connect(this.echo);
    this.echo.connect(wet).connect(this.out);
  }

  /** Switches to the song for this floor ('' theme = silence). Fades between songs. */
  setFloor(theme: string, depth: number, seed: string) {
    const key = theme + ':' + depth + ':' + seed;
    if (key === this.key) return;
    this.key = key;
    const t = this.ctx.currentTime;
    this.out.gain.cancelScheduledValues(t);
    this.out.gain.setTargetAtTime(0, t, 0.3);
    this.song = makeSong(theme, depth, seed);
    if (!this.song) return;
    this.step = 0;
    this.nextT = t + 1.2;
    this.echo.delayTime.value = (60 / this.song.bpm / 4) * 3;
    this.out.gain.setTargetAtTime(this.vol, t + 1.2, 1.5);
  }

  /** How dangerous it is right now (0 = calm, 1 = a fight): drums and bass follow it. */
  setDanger(d: number) { this.wantDanger = Math.max(0, Math.min(1, d)); }

  setVolume(v: number) {
    this.vol = v;
    this.out.gain.cancelScheduledValues(this.ctx.currentTime);
    this.out.gain.setTargetAtTime(this.song ? v : 0, this.ctx.currentTime, 0.2);
  }

  /** Call every frame: schedules the next quarter second of notes. */
  update() {
    const s = this.song, c = this.ctx;
    if (!s) return;
    this.danger += (this.wantDanger - this.danger) * 0.02;
    const stepDur = 60 / s.bpm / 4;
    if (this.nextT < c.currentTime - 0.1) this.nextT = c.currentTime + 0.05; // the tab was asleep: don't catch up
    while (this.nextT < c.currentTime + 0.25) {
      this.playStep(s, this.step, this.nextT, stepDur);
      this.step++;
      this.nextT += stepDur;
    }
  }

  private playStep(s: Song, step: number, t: number, stepDur: number) {
    const bar = Math.floor(step / 16), inBar = step % 16;
    const sec = FORM[Math.floor(bar / 4) % FORM.length];
    const chord = s.chords[bar % 4];
    const fight = this.danger > 0.3;
    if (inBar === 0 && sec.pad) this.pad(s, chord, t, stepDur * 16);
    const b = s.bass[inBar];
    if (b !== null && (sec.bass || fight)) this.voice(s.style.bass, hz(noteOf(s.style, chord + b, -2)), t, stepDur * 3, 0.09 + 0.05 * this.danger, 500);
    if (fight && inBar % 4 === 2) this.voice(s.style.bass, hz(noteOf(s.style, chord, -2)), t, stepDur, 0.05 * this.danger, 400); // a pulse in a fight
    if (sec.lead >= 0) {
      const ph = s.melodies[sec.lead], local = step % 32;
      for (const n of ph) if (n.step === local) this.lead(s, hz(noteOf(s.style, chord + n.deg)), t, stepDur * n.len);
    }
    if (sec.drums || fight) for (const d of s.drums[inBar]) this.drum(d, t, sec.drums ? 0.6 + 0.4 * this.danger : this.danger);
  }

  private env(g: GainNode, t: number, peak: number, attack: number, dur: number) {
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(Math.max(0.0002, peak), t + attack);
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
  }

  private pad(s: Song, chord: number, t: number, dur: number) {
    const c = this.ctx, f = c.createBiquadFilter(), g = c.createGain();
    f.type = 'lowpass';
    f.frequency.setValueAtTime(500, t);
    f.frequency.linearRampToValueAtTime(1100, t + dur * 0.4);
    f.frequency.linearRampToValueAtTime(400, t + dur);
    const peak = s.style.pad === 'sine' ? 0.06 : 0.03;
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(peak, t + dur * 0.3);
    g.gain.setValueAtTime(peak, t + dur * 0.7);
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur * 1.05);
    f.connect(g).connect(this.out);
    for (const d of [0, 2, 4]) {
      const o = c.createOscillator();
      o.type = s.style.pad;
      o.frequency.value = hz(noteOf(s.style, chord + d, -1));
      o.detune.value = (d - 2) * 3;
      o.connect(f);
      o.start(t);
      o.stop(t + dur * 1.1);
    }
  }

  private voice(type: Wave, f: number, t: number, dur: number, vol: number, cutoff: number) {
    const c = this.ctx, o = c.createOscillator(), fl = c.createBiquadFilter(), g = c.createGain();
    o.type = type;
    o.frequency.value = f;
    fl.type = 'lowpass';
    fl.frequency.value = cutoff;
    this.env(g, t, vol, 0.01, dur);
    o.connect(fl).connect(g).connect(this.out);
    o.start(t);
    o.stop(t + dur + 0.05);
  }

  private lead(s: Song, f: number, t: number, dur: number) {
    const c = this.ctx, o = c.createOscillator(), g = c.createGain(), fl = c.createBiquadFilter();
    o.type = s.style.lead;
    o.frequency.value = f;
    const vib = c.createOscillator(), vg = c.createGain(); // a little vibrato on long notes
    vib.frequency.value = 5;
    vg.gain.value = dur > 0.5 ? f * 0.006 : 0;
    vib.connect(vg).connect(o.frequency);
    fl.type = 'lowpass';
    fl.frequency.value = s.style.lead === 'sine' ? 4000 : 1800;
    const vol = s.style.lead === 'sine' || s.style.lead === 'triangle' ? 0.07 : 0.03;
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(vol, t + 0.03);
    g.gain.exponentialRampToValueAtTime(vol * 0.6, t + Math.min(0.3, dur * 0.5));
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur * 0.95);
    o.connect(fl).connect(g);
    g.connect(this.out);
    g.connect(this.echo);
    o.start(t); vib.start(t);
    o.stop(t + dur + 0.05); vib.stop(t + dur + 0.05);
  }

  private drum(kind: string, t: number, k: number) {
    const c = this.ctx;
    if (k < 0.05) return;
    if (kind === 'kick' || kind === 'tom') {
      const o = c.createOscillator(), g = c.createGain();
      o.type = 'sine';
      o.frequency.setValueAtTime(kind === 'kick' ? 120 : 220, t);
      o.frequency.exponentialRampToValueAtTime(kind === 'kick' ? 40 : 90, t + 0.15);
      this.env(g, t, (kind === 'kick' ? 0.25 : 0.12) * k, 0.005, 0.2);
      o.connect(g).connect(this.out);
      o.start(t);
      o.stop(t + 0.25);
      return;
    }
    const s = c.createBufferSource(), f = c.createBiquadFilter(), g = c.createGain();
    s.buffer = this.noise;
    f.type = kind === 'hat' ? 'highpass' : 'bandpass';
    f.frequency.value = kind === 'hat' ? 7000 : 1800;
    this.env(g, t, (kind === 'hat' ? 0.04 : 0.12) * k, 0.003, kind === 'hat' ? 0.04 : 0.12);
    s.connect(f).connect(g).connect(this.out);
    s.start(t, Math.random() * 0.5);
    s.stop(t + 0.2);
  }
}
