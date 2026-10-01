import { describe, it, expect } from 'vitest';
import { formatVariantLabel } from './seller-variants';

describe('formatVariantLabel', () => {
  it('A: renders Color × Size variants canonically', () => {
    expect(formatVariantLabel({ colorName: 'Черный', size: 'L' })).toBe('Черный · L');
    expect(formatVariantLabel({ colorName: 'Черный', size: 'XL' })).toBe('Черный · XL');
    expect(formatVariantLabel({ colorName: 'Белый', size: 'L' })).toBe('Белый · L');
    expect(formatVariantLabel({ colorName: 'Белый', size: 'XL' })).toBe('Белый · XL');
  });

  it('prefers colorName over persisted color string', () => {
    expect(formatVariantLabel({ colorName: 'Черный', color: 'Старый черный', size: 'M' })).toBe('Черный · M');
    expect(formatVariantLabel({ color: 'Синий', size: 'S' })).toBe('Синий · S');
  });

  it('B: no-color variant shows size only', () => {
    expect(formatVariantLabel({ size: 'L' })).toBe('L');
    expect(formatVariantLabel({ colorName: '', color: '', size: 'XL' })).toBe('XL');
    expect(formatVariantLabel({ colorName: null, color: null, size: 'M' })).toBe('M');
  });

  it('C: no-size variant shows color only', () => {
    expect(formatVariantLabel({ colorName: 'Черный' })).toBe('Черный');
    expect(formatVariantLabel({ color: 'Белый', size: '' })).toBe('Белый');
    expect(formatVariantLabel({ colorName: 'Красный', size: null })).toBe('Красный');
  });

  it('D: neither color nor size -> "Единый вариант"', () => {
    expect(formatVariantLabel({})).toBe('Единый вариант');
    expect(formatVariantLabel({ colorName: '', color: '', size: '' })).toBe('Единый вариант');
    expect(formatVariantLabel({ colorName: null, color: null, size: null })).toBe('Единый вариант');
    expect(formatVariantLabel({ colorName: '   ', size: '   ' })).toBe('Единый вариант');
  });
});
