import { describe, it, expect } from 'vitest';
import {
  buildSizeFilterModel,
  sanitizeSizeSystem,
  MAX_SIZE_LABEL_LENGTH,
  MAX_SYSTEMS_IN_ALL_MODE,
} from './catalogSizes';
import type { Category } from '../types/catalog';

const sys = (code: string, name: string, values: string[]) => ({
  code,
  name,
  values: values.map((value, sortOrder) => ({ value, sortOrder })),
});
const cat = (id: string, sizeSystem?: Category['sizeSystem']): Category => ({ id, slug: id, name: id, icon: '', sizeSystem } as Category);

describe('sanitizeSizeSystem (CATALOG.C6)', () => {
  it('keeps a normal system untouched', () => {
    expect(sanitizeSizeSystem(sys('INT', 'International', ['XS', 'S', 'M']))).toEqual(sys('INT', 'International', ['XS', 'S', 'M']));
  });

  it('drops UUID-like and over-long size labels but keeps the human ones, and de-duplicates', () => {
    const long = 'X'.repeat(MAX_SIZE_LABEL_LENGTH + 1);
    const out = sanitizeSizeSystem(
      sys('INT', 'International', ['S', '8f2c1d3e-4b5a-4c6d-9e7f-0a1b2c3d4e5f', long, ' M ', 'S', '', '\u0007'])
    );
    expect(out?.values.map((v) => v.value)).toEqual(['S', 'M']);
  });

  it('drops a system whose title is an identifier or too long, or that has no valid size left', () => {
    expect(sanitizeSizeSystem(sys('a', '8f2c1d3e-4b5a-4c6d-9e7f-0a1b2c3d4e5f', ['S']))).toBeNull();
    expect(sanitizeSizeSystem(sys('a', 'N'.repeat(60), ['S']))).toBeNull();
    expect(sanitizeSizeSystem(sys('a', 'Name', ['8f2c1d3e-4b5a-4c6d-9e7f-0a1b2c3d4e5f']))).toBeNull();
    expect(sanitizeSizeSystem(undefined)).toBeNull();
  });

  it('does not decide by words like "test": only by validity', () => {
    expect(sanitizeSizeSystem(sys('t', 'SC sys', ['S', 'M']))?.name).toBe('SC sys');
  });
});

describe('buildSizeFilterModel (CATALOG.C6)', () => {
  const many = Array.from({ length: 51 }, (_, i) => cat(`c${i}`, sys(`sc-${i}`, 'SC sys', ['S', 'M', 'L'])));

  it('"all products" with dozens of systems asks for a category instead of listing them', () => {
    const m = buildSizeFilterModel(many, 'all');
    expect(m.view.kind).toBe('prompt');
    expect(m.view.groups).toHaveLength(0);
    expect(m.validSystems).toHaveLength(51); // still known, so sizes from a URL can be validated
  });

  it('a category with its own system shows only that system', () => {
    const m = buildSizeFilterModel(many, 'c7');
    expect(m.view.kind).toBe('category');
    expect(m.view.groups.map((g) => g.code)).toEqual(['sc-7']);
  });

  it('"all products" lists a few distinct, differently named systems, each kept apart', () => {
    const cats = [cat('a', sys('INT', 'International', ['S'])), cat('b', sys('EU', 'European', ['40'])), cat('c', sys('ONE', 'One Size', ['OS']))];
    const m = buildSizeFilterModel(cats, 'all');
    expect(m.view.kind).toBe('systems');
    expect(m.view.groups.map((g) => g.code)).toEqual(['INT', 'EU', 'ONE']);
  });

  it('the same system used by many categories is listed once', () => {
    const cats = Array.from({ length: 30 }, (_, i) => cat(`c${i}`, sys('INT', 'International', ['S', 'M'])));
    const m = buildSizeFilterModel(cats, 'all');
    expect(m.view.kind).toBe('systems');
    expect(m.view.groups).toHaveLength(1);
  });

  it('different systems with the same name are never merged, and are not listed as indistinguishable titles', () => {
    const cats = [cat('a', sys('x-1', 'SC sys', ['S'])), cat('b', sys('x-2', 'SC sys', ['S']))];
    const all = buildSizeFilterModel(cats, 'all');
    expect(all.view.kind).toBe('prompt');
    expect(all.validSystems.map((s) => s.code)).toEqual(['x-1', 'x-2']);
    // inside a category each keeps its own code
    expect(buildSizeFilterModel(cats, 'b').view.groups.map((g) => g.code)).toEqual(['x-2']);
  });

  it('more than the allowed number of distinct systems becomes a prompt', () => {
    const cats = Array.from({ length: MAX_SYSTEMS_IN_ALL_MODE + 1 }, (_, i) => cat(`c${i}`, sys(`s${i}`, `System ${i}`, ['S'])));
    expect(buildSizeFilterModel(cats, 'all').view.kind).toBe('prompt');
  });

  it('a category without sizes, or whose system has no valid sizes, has none', () => {
    expect(buildSizeFilterModel([cat('bags')], 'bags').view.kind).toBe('none');
    expect(buildSizeFilterModel([cat('x', sys('u', 'Weird', ['8f2c1d3e-4b5a-4c6d-9e7f-0a1b2c3d4e5f']))], 'x').view.kind).toBe('none');
  });
});
