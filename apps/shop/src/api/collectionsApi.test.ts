import { describe, it, expect, vi, beforeEach } from 'vitest';

const getCollection = vi.fn();
vi.mock('@zamk/api-client/src/public', () => ({
  getCollections: vi.fn(),
  getCollection: (slug: string) => getCollection(slug),
  getBrands: vi.fn().mockResolvedValue([{ id: 'b1', name: 'Brand One' }]),
  getProducts: vi.fn(), getDirectSaleProducts: vi.fn(), getProduct: vi.fn(), getCategories: vi.fn(),
  getProductReviews: vi.fn(), getPublicSeller: vi.fn(), getProductPreviewByToken: vi.fn(),
  getSimilarProducts: vi.fn(), getPopularProducts: vi.fn(),
}));

import { fetchCollectionBySlug } from './publicCatalog';

describe('fetchCollectionBySlug', () => {
  beforeEach(() => getCollection.mockReset());

  it('maps products with brand name and price in roubles', async () => {
    getCollection.mockResolvedValue({
      id: 'c1', slug: 'summer', title: 'Лето', description: null, coverImageUrl: null,
      products: [
        { id: 'p1', title: 'Платье', priceCents: 500000, brandId: 'b1', sellerName: 'Seller X' },
        { id: 'p2', title: 'Юбка', priceCents: 300000, sellerName: 'Seller Y' },
      ],
    });
    const res = await fetchCollectionBySlug('summer');
    expect(res?.products.map((p) => [p.id, p.brand, p.price])).toEqual([
      ['p1', 'Brand One', 5000],
      ['p2', 'Seller Y', 3000],
    ]);
  });

  it('returns null for a 404 (unpublished collection) and rethrows other errors', async () => {
    getCollection.mockRejectedValueOnce(Object.assign(new Error('nf'), { status: 404 }));
    expect(await fetchCollectionBySlug('draft')).toBeNull();
    getCollection.mockRejectedValueOnce(Object.assign(new Error('boom'), { status: 500 }));
    await expect(fetchCollectionBySlug('x')).rejects.toThrow('boom');
  });
});
