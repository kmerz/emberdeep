import type { ClassId } from './proto';
import { RGB } from './term/fb';

export interface SkillMeta { id: string; key: string; icon: string; name: string; short: string; log?: boolean; targeted: boolean; desc?: string }
export interface ClassMeta { id: ClassId; name: string; col: RGB; resCol: RGB; rl: string; skills: SkillMeta[] }

export const CLASS_LIST: ClassId[] = ['warrior', 'rogue', 'sorcerer', 'monk'];

export const CLS: Record<ClassId, ClassMeta> = {
  warrior: {
    id: 'warrior', name: 'WARRIOR', col: [255, 138, 61], resCol: [255, 138, 61], rl: 'RG',
    skills: [
      { id: 'cleave', key: '1', icon: '─╳─', name: 'CLEAVE', short: 'CLEAVE', targeted: false, desc: 'hit the 3 cells in front of you' },
      { id: 'ironwall', key: '2', icon: '[▣]', name: 'IRON WALL', short: 'WALL', log: true, targeted: false, desc: 'absorb damage and draw enemies to you' },
      { id: 'leap', key: '3', icon: ' ↑ ', name: 'LEAP', short: 'LEAP', log: true, targeted: true, desc: 'jump to the cursor and shake the ground' },
      { id: 'shieldbash', key: '', icon: '[■]', name: 'SHIELD BASH', short: 'BASH', targeted: false, desc: 'hit the enemy in front and stun it' },
      { id: 'warcry', key: '', icon: '\\o/', name: 'WAR CRY', short: 'WAR CRY', log: true, targeted: false, desc: 'pull nearby enemies onto you; allies hit 20% harder' },
      { id: 'whirlwind', key: '', icon: '(@)', name: 'WHIRLWIND', short: 'WHIRL', targeted: false, desc: 'hit every enemy next to you' },
    ],
  },
  rogue: {
    id: 'rogue', name: 'ROGUE', col: [199, 146, 255], resCol: [199, 146, 255], rl: 'EN',
    skills: [
      { id: 'stab', key: '1', icon: '─▸ ', name: 'STAB', short: 'STAB', targeted: false, desc: 'quick stab; double damage from behind' },
      { id: 'caltrops', key: '2', icon: ' ∴ ', name: 'CALTROPS', short: 'SPIKES', log: true, targeted: true, desc: 'spikes that slow and bleed enemies' },
      { id: 'shadowbomb', key: '3', icon: '(●)', name: 'SHADOW BOMB', short: 'BOMB', log: true, targeted: true, desc: 'a void blast at the cursor, after a moment' },
      { id: 'poisonblade', key: '', icon: '─▸~', name: 'POISON BLADE', short: 'POISON', log: true, targeted: false, desc: 'your next 3 stabs poison the target' },
      { id: 'smokestep', key: '', icon: '░░░', name: 'SMOKE STEP', short: 'SMOKE', log: true, targeted: false, desc: 'vanish: enemies lose track of you' },
      { id: 'knives', key: '', icon: '<<<', name: 'FAN OF KNIVES', short: 'KNIVES', targeted: true, desc: 'a cone of 5 knives toward the cursor' },
    ],
  },
  sorcerer: {
    id: 'sorcerer', name: 'SORCERER', col: [95, 215, 255], resCol: [95, 160, 255], rl: 'MP',
    skills: [
      { id: 'spark', key: '1', icon: '─•›', name: 'SPARK', short: 'SPARK', targeted: true, desc: 'a bolt that hits the first enemy' },
      { id: 'nova', key: '2', icon: '◉◉◉', name: 'EMBER NOVA', short: 'NOVA', log: true, targeted: true, desc: 'a big fire blast at the cursor, after a moment' },
      { id: 'blink', key: '3', icon: '»·»', name: 'BLINK', short: 'BLINK', targeted: true, desc: 'teleport to the cursor' },
      { id: 'frostring', key: '', icon: '*o*', name: 'FROST RING', short: 'FROST', log: true, targeted: false, desc: 'slow and chill every enemy around you' },
      { id: 'light', key: '', icon: ' ☼ ', name: 'LIGHT', short: 'LIGHT', log: true, targeted: true, desc: 'a light orb at the cursor (see in the dark)' },
      { id: 'chainspark', key: '', icon: '•›•', name: 'CHAIN SPARK', short: 'CHAIN', targeted: true, desc: 'a spark that jumps to 3 more enemies' },
    ],
  },
  monk: {
    id: 'monk', name: 'MONK', col: [255, 212, 92], resCol: [255, 212, 92], rl: 'KI',
    skills: [
      { id: 'palm', key: '1', icon: '(‖)', name: 'PALM STRIKE', short: 'PALM', targeted: false, desc: 'strike and knock the enemy back' },
      { id: 'dash', key: '2', icon: '─»»', name: 'DASH', short: 'DASH', targeted: true, desc: 'dash through enemies toward the cursor' },
      { id: 'bell', key: '3', icon: ' ∩ ', name: 'BELL OF WARDING', short: 'BELL', log: true, targeted: false, desc: 'allies near you take 30% less damage' },
      { id: 'staffsweep', key: '', icon: '─┼─', name: 'STAFF SWEEP', short: 'SWEEP', targeted: false, desc: 'swing your staff: an arc that reaches 2 cells' },
      { id: 'healbreath', key: '', icon: ' ♥ ', name: 'HEALING BREATH', short: 'HEAL', log: true, targeted: false, desc: 'heal yourself and allies near you' },
      { id: 'fists', key: '', icon: '»»»', name: 'HUNDRED FISTS', short: 'FISTS', log: true, targeted: false, desc: '5 quick blows on the enemy in front' },
    ],
  },
};

export const SKILL_BY_ID: Record<string, { cls: ClassId; meta: SkillMeta }> = {};
for (const c of CLASS_LIST) for (const s of CLS[c].skills) SKILL_BY_ID[s.id] = { cls: c, meta: s };

export const MC: Record<string, RGB> = {
  z: [150, 196, 106], s: [232, 226, 208], f: [255, 106, 77], b: [160, 140, 255], K: [255, 61, 110], M: [214, 150, 80],
  r: [200, 190, 170], h: [236, 220, 180], n: [180, 120, 255], o: [120, 230, 110],
  a: [190, 140, 90], A: [230, 90, 50], Q: [255, 170, 60], B: [230, 120, 50],
  i: [255, 120, 60], d: [230, 80, 60], G: [255, 150, 40], S: [150, 140, 200], U: [255, 110, 200], I: [180, 90, 255], D: [255, 60, 30],
};
