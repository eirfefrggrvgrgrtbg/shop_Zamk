import { describe, expect, it } from 'vitest';
import type { Product } from '../../types/catalog';
import { brandHref, chooseForYouSection, pickArrivals, pickBrandsForHome, pickIndexCategories } from './v5Selection';

const p = (id: string, image = `https://img.test/${id}.jpg`): Product => ({
  id, name: `N${id}`, brand: 'B', brandId: 'b', price: 100, image, category: 'c1',
});

describe('v5Selection', () => {
  it('pickArrivals keeps order, drops items without a real photo, caps at 3', () => {
    const items = [p('1'), p('2', 'data:image/svg+xml;x'), p('3'), p('4'), p('5')];
    expect(pickArrivals(items).map((x) => x.id)).toEqual(['1', '3', '4']);
  });

  it('customer with for_you block gets for_you', () => {
    const s = chooseForYouSection({
      isCustomer: true,
      customerBlocks: [{ type: 'for_you', title: 'x', items: [p('7'), p('8')] }],
      guestPopular: [p('9')], newest: [], exclude: new Set(),
    });
    expect(s?.kind).toBe('for_you');
    expect(s?.items.map((x) => x.id)).toEqual(['7', '8']);
  });

  it('customer without signals falls back to popular, never labelled personal', () => {
    const s = chooseForYouSection({
      isCustomer: true,
      customerBlocks: [{ type: 'popular', title: 'x', items: [p('7')] }],
      guestPopular: [], newest: [], exclude: new Set(),
    });
    expect(s?.kind).toBe('popular');
    expect(s?.title).toBe('Популярное');
  });

  it('guest gets popular and ignores customer blocks', () => {
    const s = chooseForYouSection({
      isCustomer: false,
      customerBlocks: [{ type: 'for_you', title: 'x', items: [p('1')] }],
      guestPopular: [p('2')], newest: [], exclude: new Set(),
    });
    expect(s?.kind).toBe('popular');
    expect(s?.items.map((x) => x.id)).toEqual(['2']);
  });

  it('excludes arrivals and falls back to «Выбор ZAMK», or null when nothing is left', () => {
    const exclude = new Set(['1', '2']);
    const sel = chooseForYouSection({ isCustomer: false, customerBlocks: [], guestPopular: [p('1')], newest: [p('1'), p('2'), p('3')], exclude });
    expect(sel?.kind).toBe('selection');
    expect(sel?.items.map((x) => x.id)).toEqual(['3']);
    expect(chooseForYouSection({ isCustomer: false, customerBlocks: [], guestPopular: [], newest: [p('1')], exclude })).toBeNull();
  });

  it('brand helpers', () => {
    expect(brandHref({ id: 'i', slug: 's' })).toBe('/brand/s');
    expect(brandHref({ id: 'i' })).toBe('/brand/i');
    expect(pickIndexCategories([{ id: 'c1', slug: 'a', name: 'A', icon: '' }, { id: 'c2', slug: 'b', name: 'B', icon: '' }], [p('1')]).map((c) => c.id)).toEqual(['c1']);
  });
});

describe('photoFit', () => {
  it('covers 3:4, 4:5 and 2:3, shows square and landscape whole', async () => {
    const { photoFit } = await import('./v5Selection');
    expect(photoFit(1200, 1600)).toBe('cover');
    expect(photoFit(1200, 1500)).toBe('cover');
    expect(photoFit(1000, 1500)).toBe('cover');
    expect(photoFit(1000, 1000)).toBe('contain');
    expect(photoFit(1500, 1000)).toBe('contain');
    expect(photoFit(0, 0)).toBe('cover');
  });
});

describe('pickBrandsForHome', () => {
  const mk = (n: number) => Array.from({ length: n }, (_, i) => ({ id: `b${i}`, name: `Brand ${i}`, description: '', country: '', image: '' }));

  it('no brands -> no lead, nothing invented', () => {
    expect(pickBrandsForHome([])).toEqual({ lead: null, rest: [], hiddenCount: 0 });
  });

  it('one brand is the lead with an empty list', () => {
    const r = pickBrandsForHome(mk(1));
    expect(r.lead?.brand.id).toBe('b0');
    expect(r.rest).toHaveLength(0);
    expect(r.hiddenCount).toBe(0);
  });

  it('lead is the brand of the newest arrival when it exists, else the first; keeps archive numbers', () => {
    const r = pickBrandsForHome(mk(4), 'b2');
    expect(r.lead?.brand.id).toBe('b2');
    expect(r.lead?.index).toBe(2);
    expect(r.rest.map((e) => e.brand.id)).toEqual(['b0', 'b1', 'b3']);
    expect(pickBrandsForHome(mk(3), 'unknown').lead?.brand.id).toBe('b0');
  });

  it('limits the home selection to 7 and reports how many are hidden', () => {
    const r = pickBrandsForHome(mk(12));
    expect(1 + r.rest.length).toBe(7);
    expect(r.hiddenCount).toBe(5);
  });
});
