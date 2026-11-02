import type { ClassId } from './proto';
import { RGB } from './term/fb';

export interface SkillMeta { id: string; key: string; icon: string; name: string; short: string; log?: boolean; targeted: boolean }
export interface ClassMeta { id: ClassId; name: string; col: RGB; resCol: RGB; rl: string; skills: SkillMeta[] }

export const CLASS_LIST: ClassId[] = ['warrior', 'rogue', 'sorcerer', 'monk'];

export const CLS: Record<ClassId, ClassMeta> = {
  warrior: {
    id: 'warrior', name: 'WARRIOR', col: [255, 138, 61], resCol: [255, 138, 61], rl: 'RG',
    skills: [
      { id: 'cleave', key: '1', icon: '─╳─', name: 'CLEAVE', short: 'CLEAVE', targeted: false },
      { id: 'ironwall', key: '2', icon: '[▣]', name: 'IRON WALL', short: 'IRON WALL', log: true, targeted: false },
      { id: 'leap', key: '3', icon: ' ↑ ', name: 'LEAP', short: 'LEAP', log: true, targeted: true },
    ],
  },
  rogue: {
    id: 'rogue', name: 'ROGUE', col: [199, 146, 255], resCol: [199, 146, 255], rl: 'EN',
    skills: [
      { id: 'stab', key: '1', icon: '─▸ ', name: 'STAB', short: 'STAB', targeted: false },
      { id: 'caltrops', key: '2', icon: ' ∴ ', name: 'CALTROPS', short: 'CALTROPS', log: true, targeted: true },
      { id: 'shadowbomb', key: '3', icon: '(●)', name: 'SHADOW BOMB', short: 'BOMB', log: true, targeted: true },
    ],
  },
  sorcerer: {
    id: 'sorcerer', name: 'SORCERER', col: [95, 215, 255], resCol: [95, 160, 255], rl: 'MP',
    skills: [
      { id: 'spark', key: '1', icon: '─•›', name: 'SPARK', short: 'SPARK', targeted: true },
      { id: 'nova', key: '2', icon: '◉◉◉', name: 'EMBER NOVA', short: 'NOVA', log: true, targeted: true },
      { id: 'blink', key: '3', icon: '»·»', name: 'BLINK', short: 'BLINK', targeted: true },
    ],
  },
  monk: {
    id: 'monk', name: 'MONK', col: [255, 212, 92], resCol: [255, 212, 92], rl: 'KI',
    skills: [
      { id: 'palm', key: '1', icon: '(‖)', name: 'PALM STRIKE', short: 'PALM', targeted: false },
      { id: 'dash', key: '2', icon: '─»»', name: 'DASH', short: 'DASH', targeted: true },
      { id: 'bell', key: '3', icon: ' ∩ ', name: 'BELL OF WARDING', short: 'BELL', log: true, targeted: false },
    ],
  },
};

export const SKILL_BY_ID: Record<string, { cls: ClassId; meta: SkillMeta }> = {};
for (const c of CLASS_LIST) for (const s of CLS[c].skills) SKILL_BY_ID[s.id] = { cls: c, meta: s };

export const MC: Record<string, RGB> = {
  z: [150, 196, 106], s: [232, 226, 208], f: [255, 106, 77], b: [160, 140, 255], K: [255, 61, 110], M: [214, 150, 80],
  r: [200, 190, 170], h: [236, 220, 180], n: [180, 120, 255], o: [120, 230, 110],
};
