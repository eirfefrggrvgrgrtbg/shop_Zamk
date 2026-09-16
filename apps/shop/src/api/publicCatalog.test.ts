import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fetchProducts } from './publicCatalog';
import * as publicApi from '@zamk/api-client/src/public';
import * as customerApi from '@zamk/api-client/src/customer';

vi.mock('@zamk/api-client/src/public', () => ({
  getProducts: vi.fn(),
  getDirectSaleProducts: vi.fn(),
  getProduct: vi.fn(),
  getCategories: vi.fn().mockResolvedValue([]),
  getBrands: vi.fn().mockResolvedValue([]),
  getProductReviews: vi.fn().mockResolvedValue([]),
  getPublicSeller: vi.fn(),
  getProductPreviewByToken: vi.fn(),
  getSimilarProducts: vi.fn(),
}));

vi.mock('@zamk/api-client/src/customer', () => ({
  getCustomerCatalog: vi.fn(),
  getCustomerRecentlyViewed: vi.fn(),
  getCustomerForYou: vi.fn(),
}));

describe('fetchProducts - PER.6B2 Catalog Routing and Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('1. Anonymous + DEFAULT sort -> calls public getProducts', async () => {
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-1', title: 'Product 1', priceCents: 1000, createdAt: '2026-01-01', status: 'published', slug: 'prod-1' },
      ],
      totalCount: 1,
    });

    const res = await fetchProducts({ limit: 24 }, { isCustomer: false });

    expect(customerApi.getCustomerCatalog).not.toHaveBeenCalled();
    expect(publicApi.getProducts).toHaveBeenCalledWith({ limit: 24 });
    expect(res.items.length).toBe(1);
    expect(res.items[0].id).toBe('prod-1');
  });

  it('2. Anonymous + Explicit sort (newest / price_asc) -> calls public getProducts', async () => {
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-1', title: 'Product 1', priceCents: 1000, createdAt: '2026-01-01', status: 'published', slug: 'prod-1' },
      ],
      totalCount: 1,
    });

    await fetchProducts({ limit: 24, sort: 'price_asc' }, { isCustomer: false });

    expect(customerApi.getCustomerCatalog).not.toHaveBeenCalled();
    expect(publicApi.getProducts).toHaveBeenCalledWith({ limit: 24, sort: 'price_asc' });
  });

  it('3. Customer + DEFAULT sort -> calls customer getCustomerCatalog', async () => {
    vi.mocked(customerApi.getCustomerCatalog).mockResolvedValue({
      items: [
        { id: 'prod-affinity', title: 'Personalized Product', priceCents: 2000, createdAt: '2026-01-01', status: 'published', slug: 'prod-aff' },
        { id: 'prod-regular', title: 'Regular Product', priceCents: 1500, createdAt: '2026-01-01', status: 'published', slug: 'prod-reg' },
      ],
      totalCount: 2,
    });

    const res = await fetchProducts({ limit: 24 }, { isCustomer: true });

    expect(customerApi.getCustomerCatalog).toHaveBeenCalledWith({ limit: 24 });
    expect(publicApi.getProducts).not.toHaveBeenCalled();
    expect(res.items.length).toBe(2);
    expect(res.items[0].id).toBe('prod-affinity');
    expect(res.items[1].id).toBe('prod-regular');
  });

  it('4a. Customer + Explicit sort (price_asc) -> calls public getProducts, NOT customer catalog', async () => {
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-cheap', title: 'Cheap Product', priceCents: 500, createdAt: '2026-01-01', status: 'published', slug: 'prod-ch' },
      ],
      totalCount: 1,
    });

    const res = await fetchProducts({ limit: 24, sort: 'price_asc' }, { isCustomer: true });

    expect(customerApi.getCustomerCatalog).not.toHaveBeenCalled();
    expect(publicApi.getProducts).toHaveBeenCalledWith({ limit: 24, sort: 'price_asc' });
    expect(res.items[0].id).toBe('prod-cheap');
  });

  it('4b. Customer + Explicit sort (price_desc) -> calls public getProducts, NOT customer catalog', async () => {
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-expensive', title: 'Expensive Product', priceCents: 5000, createdAt: '2026-01-01', status: 'published', slug: 'prod-exp' },
      ],
      totalCount: 1,
    });

    const res = await fetchProducts({ limit: 24, sort: 'price_desc' }, { isCustomer: true });

    expect(customerApi.getCustomerCatalog).not.toHaveBeenCalled();
    expect(publicApi.getProducts).toHaveBeenCalledWith({ limit: 24, sort: 'price_desc' });
    expect(res.items[0].id).toBe('prod-expensive');
  });

  it('4c. Customer + Explicit sort (newest) -> calls public getProducts, NOT customer catalog', async () => {
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-newest', title: 'Newest Product', priceCents: 3000, createdAt: '2026-02-01', status: 'published', slug: 'prod-new' },
      ],
      totalCount: 1,
    });

    const res = await fetchProducts({ limit: 24, sort: 'newest' }, { isCustomer: true });

    expect(customerApi.getCustomerCatalog).not.toHaveBeenCalled();
    expect(publicApi.getProducts).toHaveBeenCalledWith({ limit: 24, sort: 'newest' });
    expect(res.items[0].id).toBe('prod-newest');
  });

  it('5. Non-Customer auth (seller/admin) + DEFAULT sort -> calls public getProducts', async () => {
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-1', title: 'Product 1', priceCents: 1000, createdAt: '2026-01-01', status: 'published', slug: 'prod-1' },
      ],
      totalCount: 1,
    });

    await fetchProducts({ limit: 24 }, { isCustomer: false });

    expect(customerApi.getCustomerCatalog).not.toHaveBeenCalled();
    expect(publicApi.getProducts).toHaveBeenCalledWith({ limit: 24 });
  });

  it('6. Filter & Pagination parameters forwarded identically to getCustomerCatalog', async () => {
    vi.mocked(customerApi.getCustomerCatalog).mockResolvedValue({
      items: [],
      totalCount: 0,
    });

    const filters = {
      limit: 24,
      offset: 48,
      categoryId: 'cat-coats',
      brandId: 'brand-zamk',
      minPriceCents: 100000,
      maxPriceCents: 500000,
      size: 'L',
      inStock: 'true',
      q: 'wool',
    };

    await fetchProducts(filters, { isCustomer: true });

    expect(customerApi.getCustomerCatalog).toHaveBeenCalledWith(filters);
  });

  it('7. Fail-open fallback: if getCustomerCatalog fails, fallback once to getProducts with same params', async () => {
    vi.mocked(customerApi.getCustomerCatalog).mockRejectedValue(new Error('500 Internal Server Error'));
    vi.mocked(publicApi.getProducts).mockResolvedValue({
      items: [
        { id: 'prod-fallback', title: 'Fallback Public Product', priceCents: 1000, createdAt: '2026-01-01', status: 'published', slug: 'prod-fb' },
      ],
      totalCount: 1,
    });

    const filters = { limit: 24, categoryId: 'cat-coats' };
    const res = await fetchProducts(filters, { isCustomer: true });

    expect(customerApi.getCustomerCatalog).toHaveBeenCalledWith(filters);
    expect(publicApi.getProducts).toHaveBeenCalledWith(filters);
    expect(res.items.length).toBe(1);
    expect(res.items[0].id).toBe('prod-fallback');
  });

  it('8. Array order returned by backend is strictly preserved 1:1', async () => {
    const items = [
      { id: 'z-last-id', title: 'Z Product', priceCents: 100, createdAt: '2026-01-01', status: 'published', slug: 'z' },
      { id: 'a-first-id', title: 'A Product', priceCents: 900, createdAt: '2026-01-01', status: 'published', slug: 'a' },
      { id: 'm-middle-id', title: 'M Product', priceCents: 500, createdAt: '2026-01-01', status: 'published', slug: 'm' },
    ];
    vi.mocked(customerApi.getCustomerCatalog).mockResolvedValue({
      items,
      totalCount: 3,
    });

    const res = await fetchProducts({ limit: 24 }, { isCustomer: true });

    expect(res.items.map((p) => p.id)).toEqual(['z-last-id', 'a-first-id', 'm-middle-id']);
  });
});
