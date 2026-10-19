import { describe, it, expect } from 'vitest';
import {
  categoriesWithProducts,
  choosePersonalSection,
  dedupeProducts,
  hasRealImage,
  pickHeroProducts,
} from './homeSelection';
import type { Category, Product } from '../../types/catalog';

const p = (id: string, overrides: Partial<Product> = {}): Product => ({
  id,
  name: id,
  brand: 'B',
  brandId: 'b',
  price: 1,
  category: 'c1',
  image: `https://cdn.example/${id}.jpg`,
  ...overrides,
});

describe('homeSelection', () => {
  it('hasRealImage rejects placeholders and missing photos', () => {
    expect(hasRealImage(p('a'))).toBe(true);
    expect(hasRealImage(p('a', { image: 'data:image/svg+xml;utf8,<svg/>' }))).toBe(false);
    expect(hasRealImage(p('a', { image: 'https://placehold.co/400x500' }))).toBe(false);
    expect(hasRealImage(p('a', { image: '' }))).toBe(false);
  });

  it('pickHeroProducts keeps order, skips placeholders, honours the limit', () => {
    const items = [p('a', { image: 'data:x' }), p('b'), p('c'), p('d'), p('e')];
    expect(pickHeroProducts(items).map((x) => x.id)).toEqual(['b', 'c', 'd']);
    expect(pickHeroProducts([], 3)).toEqual([]);
  });

  it('categoriesWithProducts only keeps categories that have products', () => {
    const cats: Category[] = [
      { id: 'c1', slug: 'a', name: 'A', icon: '' },
      { id: 'c2', slug: 'b', name: 'B', icon: '' },
    ];
    expect(categoriesWithProducts(cats, [p('x')]).map((c) => c.id)).toEqual(['c1']);
    expect(categoriesWithProducts(cats, [])).toEqual([]);
  });

  it('dedupeProducts removes repeats and excluded ids', () => {
    const out = dedupeProducts([p('a'), p('b'), p('a'), p('c')], new Set(['b']));
    expect(out.map((x) => x.id)).toEqual(['a', 'c']);
  });

  describe('choosePersonalSection', () => {
    it('prefers the customer for_you block (own signals)', () => {
      const s = choosePersonalSection(
        [
          { type: 'popular', title: 'P', items: [p('pop')] },
          { type: 'for_you', title: 'F', items: [p('fy')] },
        ],
        [p('guest')],
      );
      expect(s?.kind).toBe('for_you');
      expect(s?.items[0].id).toBe('fy');
    });

    it('customer without for_you falls back to composer popular', () => {
      const s = choosePersonalSection([{ type: 'popular', title: 'P', items: [p('pop')] }], []);
      expect(s).toEqual({ kind: 'popular', items: [p('pop')] });
    });

    it('guest gets the general popular selection', () => {
      const s = choosePersonalSection([], [p('g1')]);
      expect(s?.kind).toBe('popular');
    });

    it('returns null when nothing is available (no artificial filling)', () => {
      expect(choosePersonalSection([], [])).toBeNull();
      expect(choosePersonalSection([{ type: 'for_you', title: 'F', items: [] }], [])).toBeNull();
    });
  });
});
