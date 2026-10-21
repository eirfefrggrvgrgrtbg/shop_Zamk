import type { Category } from '../types/catalog';

export type SizeSystem = NonNullable<Category['sizeSystem']>;

/** A size label is a short human string ("S", "42", "52/54", "One Size"); never an identifier. */
export const MAX_SIZE_LABEL_LENGTH = 12;
/** A system title is a short human name ("International"). */
export const MAX_SYSTEM_NAME_LENGTH = 40;
/** "All products": at most this many distinct systems are listed; more would be an endless technical list. */
export const MAX_SYSTEMS_IN_ALL_MODE = 4;

const UUID_FRAGMENT = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}/i;
const CONTROL_CHARS = /[\u0000-\u001f\u007f]/;

function isHumanLabel(raw: unknown, maxLength: number): raw is string {
  if (typeof raw !== 'string') return false;
  const text = raw.trim();
  return text.length > 0 && text.length <= maxLength && !UUID_FRAGMENT.test(text) && !CONTROL_CHARS.test(text);
}

/**
 * Keeps only what a buyer can read: a human system name and human size labels (trimmed, de-duplicated inside
 * the system, original order). A system without a valid name or without any valid size is dropped.
 * Systems are NEVER matched by name: two systems with the same title stay two systems (identity is `code`).
 */
export function sanitizeSizeSystem(system: Category['sizeSystem']): SizeSystem | null {
  if (!system || typeof system.code !== 'string' || system.code.trim() === '') return null;
  if (!isHumanLabel(system.name, MAX_SYSTEM_NAME_LENGTH)) return null;
  const seen = new Set<string>();
  const values: SizeSystem['values'] = [];
  for (const v of system.values ?? []) {
    if (!isHumanLabel(v?.value, MAX_SIZE_LABEL_LENGTH)) continue;
    const value = v.value.trim();
    if (seen.has(value)) continue;
    seen.add(value);
    values.push({ value, sortOrder: v.sortOrder });
  }
  return values.length > 0 ? { code: system.code, name: system.name.trim(), values } : null;
}

export type SizeFilterView =
  /** the selected category's own system */
  | { kind: 'category'; groups: SizeSystem[] }
  /** "all products" with a few distinct, clearly named systems */
  | { kind: 'systems'; groups: SizeSystem[] }
  /** "all products" with too many / indistinguishable systems: ask for a category instead of listing them */
  | { kind: 'prompt'; groups: [] }
  /** the category has no (valid) size grid */
  | { kind: 'none'; groups: [] };

export interface SizeFilterModel {
  view: SizeFilterView;
  /** Every valid system the current scope knows — used to validate sizes that come from the URL. */
  validSystems: SizeSystem[];
}

export function buildSizeFilterModel(categories: Category[], activeCategory: string): SizeFilterModel {
  if (activeCategory !== 'all') {
    const own = sanitizeSizeSystem(categories.find((c) => c.id === activeCategory)?.sizeSystem);
    return own
      ? { view: { kind: 'category', groups: [own] }, validSystems: [own] }
      : { view: { kind: 'none', groups: [] }, validSystems: [] };
  }

  const byCode = new Map<string, SizeSystem>();
  for (const c of categories) {
    const sys = sanitizeSizeSystem(c.sizeSystem);
    if (sys && !byCode.has(sys.code)) byCode.set(sys.code, sys);
  }
  const systems = [...byCode.values()];
  if (systems.length === 0) return { view: { kind: 'none', groups: [] }, validSystems: [] };

  const distinctNames = new Set(systems.map((s) => s.name.toLowerCase()));
  const readable = systems.length <= MAX_SYSTEMS_IN_ALL_MODE && distinctNames.size === systems.length;
  return readable
    ? { view: { kind: 'systems', groups: systems }, validSystems: systems }
    : { view: { kind: 'prompt', groups: [] }, validSystems: systems };
}
