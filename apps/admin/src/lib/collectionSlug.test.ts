import { describe, it, expect } from 'vitest';
import { SLUG_PATTERN, moveItem, slugifyTitle } from './collectionSlug';

describe('collection slug helpers', () => {
  it('transliterates titles into API-valid slugs', () => {
    expect(slugifyTitle('Летний гардероб 2026')).toBe('letniy-garderob-2026');
    expect(slugifyTitle('  Новая   волна! ')).toBe('novaya-volna');
    expect(SLUG_PATTERN.test(slugifyTitle('Тёмная палитра / Осень'))).toBe(true);
    expect(slugifyTitle('!!!')).toBe('');
  });

  it('validates slugs like the API', () => {
    expect(SLUG_PATTERN.test('summer-edit')).toBe(true);
    expect(SLUG_PATTERN.test('Summer')).toBe(false);
    expect(SLUG_PATTERN.test('a--b')).toBe(false);
    expect(SLUG_PATTERN.test('')).toBe(false);
  });

  it('moves items without mutating and ignores impossible moves', () => {
    const items = ['a', 'b', 'c'];
    expect(moveItem(items, 1, -1)).toEqual(['b', 'a', 'c']);
    expect(moveItem(items, 1, 1)).toEqual(['a', 'c', 'b']);
    expect(moveItem(items, 0, -1)).toBe(items);
    expect(moveItem(items, 2, 1)).toBe(items);
    expect(items).toEqual(['a', 'b', 'c']);
  });
});
