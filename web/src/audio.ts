// Sound: a tiny WebAudio synth, no sample files. Game events trigger short effects; each floor has its own
// generated music (music.ts), plus a few random ambient noises (drips in the crypt, crackles in the ember depths).
import type { GameEvent } from './proto';
import { Music } from './music';

const SOUND_KEY = 'emberdeep:sound';
const OLD_MUTE_KEY = 'emberdeep:muted';

/** What plays: music and effects, effects only, or nothing. M cycles through them. */
export type SoundMode = 'all' | 'sfx' | 'off';
export const SOUND_MODES: SoundMode[] = ['all', 'sfx', 'off'];
const MUSIC_VOL = 0.8;
const BOSSES = 'KQD';

type Wave = OscillatorType;

function loadMode(): SoundMode {
  try {
    const m = localStorage.getItem(SOUND_KEY) as SoundMode | null;
    if (m && SOUND_MODES.includes(m)) return m;
    return localStorage.getItem(OLD_MUTE_KEY) === '1' ? 'off' : 'all';
  } catch { return 'all'; }
}

export class Sfx {
  mode = loadMode();
  get muted() { return this.mode === 'off'; }
  private music: Music | null = null;
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private bus: GainNode | null = null;
  private noiseBuf: AudioBuffer | null = null;
  private last = new Map<string, number>();
  private theme = '';
  private nextAmbient = 0;
  private vol = 1; // loudness of the sound being played (distance falloff)

  /** Browsers only allow audio after a user gesture: call this from key and mouse handlers. */
  unlock() {
    if (this.ctx) {
      if (this.ctx.state === 'suspended') void this.ctx.resume();
      return;
    }
    const AC = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!AC) return;
    try { this.ctx = new AC(); } catch { return; }
    const c = this.ctx;
    this.master = c.createGain();
    this.master.gain.value = this.muted ? 0 : 0.45;
    this.master.connect(c.destination);
    this.bus = c.createGain();
    this.bus.connect(this.master);
    const n = c.sampleRate;
    this.noiseBuf = c.createBuffer(1, n, n);
    const d = this.noiseBuf.getChannelData(0);
    for (let i = 0; i < n; i++) d[i] = Math.random() * 2 - 1;
    this.music = new Music(c, this.master, this.noiseBuf);
    this.music.setVolume(this.mode === 'all' ? MUSIC_VOL : 0);
    if (this.floor) this.music.setFloor(...this.floor);
  }

  /** M: music and effects → effects only → off → ... (remembered in this browser). Returns the new mode. */
  cycleSound(): SoundMode {
    this.mode = SOUND_MODES[(SOUND_MODES.indexOf(this.mode) + 1) % SOUND_MODES.length];
    try { localStorage.setItem(SOUND_KEY, this.mode); } catch { /* private window */ }
    if (this.ctx && this.master) this.master.gain.setTargetAtTime(this.muted ? 0 : 0.45, this.ctx.currentTime, 0.05);
    this.music?.setVolume(this.mode === 'all' ? MUSIC_VOL : 0);
    return this.mode;
  }

  // ---- building blocks ----

  private tone(f: number, dur: number, type: Wave = 'square', vol = 0.2, to = f, delay = 0) {
    const c = this.ctx, bus = this.bus;
    if (!c || !bus) return;
    const t = c.currentTime + delay, v = Math.max(0.0002, vol * this.vol);
    const o = c.createOscillator(), g = c.createGain();
    o.type = type;
    o.frequency.setValueAtTime(f, t);
    if (to !== f) o.frequency.exponentialRampToValueAtTime(Math.max(20, to), t + dur);
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(v, t + Math.min(0.01, dur / 4));
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    o.connect(g).connect(bus);
    o.start(t);
    o.stop(t + dur + 0.05);
  }

  private noise(dur: number, vol = 0.2, freq = 1200, type: BiquadFilterType = 'bandpass', to = freq, delay = 0, q = 1) {
    const c = this.ctx, bus = this.bus;
    if (!c || !bus || !this.noiseBuf) return;
    const t = c.currentTime + delay, v = Math.max(0.0002, vol * this.vol);
    const s = c.createBufferSource(), f = c.createBiquadFilter(), g = c.createGain();
    s.buffer = this.noiseBuf;
    f.type = type;
    f.Q.value = q;
    f.frequency.setValueAtTime(freq, t);
    if (to !== freq) f.frequency.exponentialRampToValueAtTime(Math.max(20, to), t + dur);
    g.gain.setValueAtTime(0.0001, t);
    g.gain.exponentialRampToValueAtTime(v, t + Math.min(0.01, dur / 4));
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
    s.connect(f).connect(g).connect(bus);
    s.start(t, Math.random() * 0.5);
    s.stop(t + dur + 0.05);
  }

  /** Plays a named sound at loudness k (0..1). The same sound is held back for 40 ms so a big AoE stays one hit. */
  play(name: string, k = 1) {
    if (!this.ctx || this.muted || k < 0.06) return;
    const now = performance.now(), prev = this.last.get(name) ?? -1e9;
    if (now - prev < 40) return;
    this.last.set(name, now);
    this.vol = k;
    switch (name) {
      case 'hit': this.noise(0.07, 0.35, 900, 'bandpass', 400); this.tone(180, 0.08, 'triangle', 0.25, 90); break;
      case 'crit': this.noise(0.1, 0.45, 2500, 'bandpass', 600); this.tone(900, 0.12, 'square', 0.12, 250); break;
      case 'hurt': this.tone(150, 0.18, 'square', 0.2, 70); this.noise(0.12, 0.3, 500, 'lowpass', 150); break;
      case 'kill': this.noise(0.22, 0.35, 700, 'lowpass', 120); this.tone(320, 0.2, 'sawtooth', 0.1, 60); break;
      case 'bosskill':
        this.noise(1.4, 0.6, 1200, 'lowpass', 60); this.tone(110, 1.2, 'sawtooth', 0.25, 30);
        [262, 330, 392, 523].forEach((f, i) => this.tone(f, 0.5, 'triangle', 0.14, f, 0.5 + i * 0.12));
        break;
      case 'swish': this.noise(0.09, 0.22, 3000, 'highpass', 1200); break;
      case 'zap': this.tone(1300, 0.09, 'square', 0.1, 380); break;
      case 'charge': this.tone(180, 0.45, 'sine', 0.22, 900); this.noise(0.45, 0.08, 400, 'bandpass', 2000); break;
      case 'boom': this.noise(0.8, 0.6, 900, 'lowpass', 60); this.tone(90, 0.6, 'sine', 0.4, 35); break;
      case 'thud': this.noise(0.25, 0.5, 400, 'lowpass', 60); this.tone(70, 0.25, 'sine', 0.35, 40); break;
      case 'whoosh': this.noise(0.25, 0.25, 400, 'bandpass', 3000, 0, 3); break;
      case 'blink': this.tone(1500, 0.18, 'sine', 0.14, 300); this.noise(0.15, 0.1, 5000, 'highpass'); break;
      case 'shout': this.tone(110, 0.4, 'sawtooth', 0.16, 98); this.tone(165, 0.4, 'sawtooth', 0.12, 147); break;
      case 'chime': this.tone(1400, 0.4, 'triangle', 0.12, 1900); this.noise(0.3, 0.1, 6000, 'highpass'); break;
      case 'bell': this.tone(660, 1.3, 'sine', 0.2); this.tone(990, 1.0, 'sine', 0.1); this.tone(1320, 0.6, 'sine', 0.06); break;
      case 'clang': this.tone(330, 0.25, 'square', 0.1, 320); this.tone(495, 0.3, 'triangle', 0.12); break;
      case 'tick2': this.tone(2200, 0.03, 'square', 0.06); this.tone(2600, 0.03, 'square', 0.05, 2600, 0.06); break;
      case 'gulp': this.tone(300, 0.08, 'sine', 0.2, 600); this.tone(350, 0.08, 'sine', 0.18, 700, 0.1); this.tone(420, 0.1, 'sine', 0.16, 840, 0.2); break;
      case 'coin': this.tone(1320, 0.07, 'square', 0.08); this.tone(1760, 0.18, 'square', 0.08, 1760, 0.07); break;
      case 'pick': this.tone(660, 0.1, 'triangle', 0.18, 990); break;
      case 'equip': this.noise(0.08, 0.12, 2500, 'bandpass'); this.tone(330, 0.12, 'square', 0.06, 220, 0.03); break;
      case 'beam': [784, 988, 1175, 1568].forEach((f, i) => this.tone(f, 0.6, 'sine', 0.12, f * 1.01, i * 0.07)); this.noise(0.9, 0.05, 7000, 'highpass'); break;
      case 'chest': this.noise(0.25, 0.2, 300, 'bandpass', 900, 0, 4); [784, 988, 1175].forEach((f, i) => this.tone(f, 0.15, 'square', 0.06, f, 0.2 + i * 0.07)); break;
      case 'levelup': [523, 659, 784, 1047].forEach((f, i) => this.tone(f, 0.35, 'triangle', 0.2, f, i * 0.09)); break;
      case 'down': this.tone(440, 0.7, 'sawtooth', 0.12, 110); break;
      case 'dead': this.tone(98, 1.6, 'sine', 0.3); this.tone(147, 1.6, 'sine', 0.12); break;
      case 'revive': this.tone(330, 0.3, 'sine', 0.2, 660); this.tone(880, 0.4, 'triangle', 0.1, 880, 0.25); break;
      case 'mimic': this.tone(90, 0.5, 'sawtooth', 0.25, 55); this.noise(0.4, 0.3, 300, 'lowpass', 100); break;
      case 'rumble': this.noise(1.0, 0.4, 150, 'lowpass', 1400); this.tone(55, 1.0, 'sawtooth', 0.12, 80); break;
      case 'charm': this.tone(600, 0.25, 'sine', 0.14, 900); this.tone(900, 0.35, 'sine', 0.12, 600, 0.25); break;
      case 'drain': this.tone(800, 0.4, 'sine', 0.15, 200); break;
      case 'summon': this.tone(220, 0.5, 'sine', 0.14, 233); this.tone(311, 0.5, 'sine', 0.1, 293); break;
      case 'squelch': this.tone(200, 0.15, 'sine', 0.25, 600); this.noise(0.1, 0.15, 400, 'lowpass'); break;
      case 'fanfare': [392, 523, 659, 784].forEach((f, i) => this.tone(f, i === 3 ? 0.5 : 0.14, 'square', 0.09, f, i * 0.11)); break;
      case 'quest': this.tone(659, 0.12, 'triangle', 0.15); this.tone(880, 0.25, 'triangle', 0.15, 880, 0.12); break;
      case 'fail': this.tone(392, 0.2, 'square', 0.08); this.tone(311, 0.4, 'square', 0.08, 311, 0.2); break;
      case 'descend': this.noise(1.2, 0.3, 2000, 'lowpass', 100); [131, 196, 262].forEach((f) => this.tone(f, 1.4, 'triangle', 0.1)); break;
      case 'alarm': for (let i = 0; i < 4; i++) this.tone(1800, 0.05, 'square', 0.06, 1500, i * 0.08); break;
      case 'portal': this.tone(400, 1.0, 'sine', 0.12, 1200); this.tone(600, 1.0, 'sine', 0.08, 1800); break;
      case 'buzz': this.tone(110, 0.15, 'square', 0.1); break;
      case 'tick': this.tone(1000, 0.05, 'square', 0.08); break;
      case 'heart': this.tone(60, 0.12, 'sine', 0.4, 45); this.tone(55, 0.14, 'sine', 0.3, 40, 0.18); break;
      case 'hiss': this.noise(0.4, 0.2, 6000, 'highpass', 3000); break;
      case 'smash': this.noise(0.12, 0.35, 2200, 'bandpass', 900, 0, 2); this.tone(1800, 0.08, 'triangle', 0.08, 1200, 0.03); break;
      case 'spikes': this.tone(1600, 0.08, 'square', 0.1, 500); this.noise(0.1, 0.3, 3000, 'bandpass', 1200); break;
      case 'click': this.tone(2400, 0.02, 'square', 0.06); break;
      // a few quick "syllables": the voice of whoever speaks
      case 'voice-boss': for (let i = 0; i < 4; i++) this.tone(70 + Math.random() * 30, 0.09, 'sawtooth', 0.12, 60, i * 0.1); break;
      case 'voice-gnome': for (let i = 0; i < 4; i++) this.tone(700 + Math.random() * 300, 0.05, 'triangle', 0.06, 650, i * 0.07); break;
      case 'voice': for (let i = 0; i < 3; i++) this.tone(180 + Math.random() * 80, 0.07, 'square', 0.05, 150, i * 0.09); break;
      case 'streak': this.tone(523, 0.1, 'square', 0.08); this.tone(784, 0.2, 'square', 0.08, 1047, 0.08); break;
      // ambience
      case 'drip': this.tone(1800 + Math.random() * 800, 0.12, 'sine', 0.06, 900); break;
      case 'crackle': for (let i = 0; i < 3; i++) this.noise(0.03, 0.12, 3000, 'highpass', 3000, Math.random() * 0.3); break;
      case 'bubble': this.tone(150 + Math.random() * 100, 0.15, 'sine', 0.08, 400); break;
      case 'creak': this.noise(0.5, 0.05, 250, 'bandpass', 180, 0, 8); break;
      case 'skitter': for (let i = 0; i < 5; i++) this.noise(0.02, 0.06, 4000, 'highpass', 4000, i * 0.05); break;
      case 'whisper': this.noise(1.2, 0.04, 800, 'bandpass', 1600, 0, 6); break;
    }
    this.vol = 1;
  }

  /** Turns one game event into a sound. pos is the local hero, for distance falloff. */
  event(e: GameEvent, me: number, pos: { x: number; y: number } | null) {
    if (!this.ctx || this.muted) return;
    const mine = e.actor === me || e.target === me;
    let k = 1;
    if (!mine && pos) k = Math.max(0, 1 - Math.hypot(e.x - pos.x, (e.y - pos.y) * 2) / 70) * 0.7;
    switch (e.k) {
      case 'hit':
        if ((e.target ?? 0) < 10000) { if (e.target === me) this.play('hurt'); }
        else this.play(e.fx === 'crit' ? 'crit' : 'hit', e.actor === me ? 1 : k * 0.6);
        break;
      case 'kill': this.play(BOSSES.includes(e.fx ?? '') ? 'bosskill' : 'kill', BOSSES.includes(e.fx ?? '') ? 1 : k); break;
      case 'cast': this.play(castSound(e.fx ?? ''), k); break;
      case 'strike': this.play(castSound('stab'), k * 0.7); break;
      case 'impact': this.play(e.fx === 'leap' ? 'thud' : 'boom', Math.max(k, 0.4)); break;
      case 'potion': case 'use': if (mine) this.play(e.fx === 'torch' ? 'whoosh' : e.fx === 'lightorb' ? 'chime' : 'gulp'); break;
      case 'pickup': case 'gearpick': if (mine) this.play(e.fx === 'gold' ? 'coin' : 'pick'); break;
      case 'equip': case 'unequip': if (e.actor === me) this.play('equip'); break;
      case 'shrine': this.play(e.fx === 'chance' ? 'charge' : e.fx ? 'bell' : 'click', mine ? 1 : 0.7); break;
      case 'raredrop': if (e.actor === me) this.play('beam'); break;
      case 'trade': if (mine) this.play('coin'); break;
      case 'chest': this.play('chest', k); break;
      case 'levelup': this.play('levelup', mine ? 1 : 0.5); break;
      case 'down': this.play('down', mine ? 1 : 0.6); break;
      case 'dead': this.play('dead'); break;
      case 'revive': this.play('revive', mine ? 1 : 0.6); break;
      case 'mimic': this.play('mimic', Math.max(k, 0.5)); break;
      case 'breath': this.play('rumble'); break;
      case 'breathfire': this.play('boom'); break;
      case 'charm': this.play('charm', mine ? 1 : k); break;
      case 'drain': if (mine) this.play('drain'); break;
      case 'summon': this.play('summon', k); break;
      case 'goblin': this.play('charge'); break;
      case 'escape': this.play('blink'); break;
      case 'stir': this.play('rumble'); break;
      case 'split': this.play('squelch', k); break;
      case 'shot': this.play('zap', k * 0.5); break;
      case 'blink': this.play('blink', k); break;
      case 'questdone': this.play('fanfare'); break;
      case 'questnew': case 'questprogress': this.play('quest'); break;
      case 'questfail': this.play('fail'); break;
      case 'descend': case 'village': case 'ascend': this.play('descend'); break;
      case 'alarm': this.play('alarm'); break;
      case 'portal': this.play('portal'); break;
      case 'packfull': if (mine) this.play('buzz'); break;
      case 'fuse': this.play('hiss', Math.max(k, 0.4)); break;
      case 'urn': this.play('smash', k); break;
      case 'trap': this.play('spikes', mine ? 1 : k); break;
      case 'trapseen': if (e.actor === me) this.play('click'); break;
      case 'bark': this.play(e.fx && 'KQD'.includes(e.fx) ? 'voice-boss' : e.fx === 'g' ? 'voice-gnome' : 'voice', Math.max(k, 0.3)); break;
    }
  }

  private floor: [string, number, string] | null = null;

  /** Sets the floor for ambience and music ('' theme = silence). */
  setTheme(theme: string, depth = 0, seed = '') {
    if (!this.floor || this.floor[0] !== theme || this.floor[1] !== depth || this.floor[2] !== seed) {
      this.floor = [theme, depth, seed];
      this.music?.setFloor(theme, depth, seed);
    }
    if (theme === this.theme) return;
    this.theme = theme;
    this.nextAmbient = performance.now() + 2000;
  }

  /** How close a fight is (0..1): the music's drums and bass follow it. */
  setDanger(d: number) { this.music?.setDanger(d); }

  /** Call every frame: random ambient sounds for the current theme, and a heartbeat when hp is low. */
  update(now: number, lowHp: boolean) {
    if (!this.ctx || this.muted) return;
    if (lowHp) this.beat(now);
    if (this.mode === 'all') this.music?.update();
    if (now < this.nextAmbient) return;
    this.nextAmbient = now + 2500 + Math.random() * 5000;
    const pick: Record<string, string[]> = {
      cathedral: ['creak', 'whisper'], crypt: ['drip', 'drip', 'creak'], caverns: ['bubble', 'drip'],
      dark: ['whisper', 'skitter'], hive: ['skitter', 'skitter'], abyss: ['whisper', 'crackle'], ember: ['crackle', 'crackle', 'bubble'],
    };
    const opts = pick[this.theme];
    if (opts) this.play(opts[Math.floor(Math.random() * opts.length)], 0.6);
  }

  private beatAt = 0;
  private beat(now: number) {
    if (now < this.beatAt) return;
    this.beatAt = now + 900;
    this.play('heart', 0.8);
  }
}

function castSound(fx: string): string {
  switch (fx) {
    case 'cleave': case 'stab': case 'palm': case 'shieldbash': case 'staffsweep': case 'fist': case 'whirlwind': return 'swish';
    case 'spark': case 'chainspark': case 'knives': case 'light': return 'zap';
    case 'nova': case 'shadowbomb': case 'firebomb': return 'charge';
    case 'blink': case 'smokestep': return 'blink';
    case 'leap': case 'dash': return 'whoosh';
    case 'warcry': return 'shout';
    case 'frostring': return 'chime';
    case 'healbreath': case 'bell': return 'bell';
    case 'ironwall': return 'clang';
    case 'caltrops': case 'poisonblade': return 'tick2';
    default: return 'swish';
  }
}

export const sfx = new Sfx();
