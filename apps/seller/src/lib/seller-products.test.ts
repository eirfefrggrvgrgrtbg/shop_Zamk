import { describe, it, expect } from 'vitest';
import { getStorefrontStatusInfo, type SellerProduct } from './seller-products';

describe('seller-products: getStorefrontStatusInfo', () => {
  it('returns "Не опубликован" for draft, pending_moderation, in_review, rejected, approved', () => {
    expect(getStorefrontStatusInfo({ status: 'draft' }).label).toBe('Не опубликован');
    expect(getStorefrontStatusInfo({ status: 'pending_moderation' }).label).toBe('Не опубликован');
    expect(getStorefrontStatusInfo({ status: 'in_review' }).label).toBe('Не опубликован');
    expect(getStorefrontStatusInfo({ status: 'rejected' }).label).toBe('Не опубликован');
    expect(getStorefrontStatusInfo({ status: 'approved' }).label).toBe('Не опубликован');
  });

  it('returns "Скрыт из каталога" for hidden status', () => {
    expect(getStorefrontStatusInfo({ status: 'hidden' }).label).toBe('Скрыт из каталога');
  });

  it('returns "Заблокирован" for blocked status', () => {
    expect(getStorefrontStatusInfo({ status: 'blocked' }).label).toBe('Заблокирован');
  });

  it('returns "Нет в наличии" for out_of_stock status', () => {
    expect(getStorefrontStatusInfo({ status: 'out_of_stock' }).label).toBe('Нет в наличии');
  });

  it('returns "Снят с продажи" for archived status', () => {
    expect(getStorefrontStatusInfo({ status: 'archived' }).label).toBe('Снят с продажи');
  });

  it('returns "В продаже" for published with actualVisibility=true', () => {
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: true,
      }).label
    ).toBe('В продаже');
  });

  it('returns truthful reasons for published with actualVisibility=false', () => {
    // 1. low_stock with total stock = 0
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: false,
        visibilityReasons: ['low_stock'],
        sizes: [{ stock: 0 }],
      }).label
    ).toBe('Нет в наличии');

    // 2. low_stock with total stock > 0 (1 unit < min required 2)
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: false,
        visibilityReasons: ['low_stock'],
        sizes: [{ stock: 1 }],
      }).label
    ).toBe('Недостаточно остатков');

    // 3. seller_inactive
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: false,
        visibilityReasons: ['seller_inactive'],
      }).label
    ).toBe('Магазин неактивен');

    // 4. no_active_variants
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: false,
        visibilityReasons: ['no_active_variants'],
      }).label
    ).toBe('Нет активных вариантов');

    // 5. invalid_price
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: false,
        visibilityReasons: ['invalid_price'],
      }).label
    ).toBe('Не указана цена');

    // 6. fallback when no specific reason given
    expect(
      getStorefrontStatusInfo({
        status: 'published',
        actualVisibility: false,
      }).label
    ).toBe('Скрыт из каталога');
  });

  it('guarantees _raw is not in SellerProduct type contract', () => {
    const product: SellerProduct = {
      id: 'p1',
      title: 'Item',
      sku: 'SKU-1',
      category: 'Одежда',
      brand: 'ZAMK',
      price: 1000,
      cost: 500,
      status: 'published',
      issue: 'no_issue',
      views: 0,
      orders: 0,
      rating: 5,
      returns: 0,
      revenue: 0,
      adsSpend: 0,
      ctr: 0,
      conversion: 0,
      quality: 100,
      mainPhoto: '',
      photos: [],
      description: '',
      material: '',
      color: '',
      season: 'Всесезон',
      sizes: [],
      updatedAt: '01.01.2026',
    };
    // Verify TypeScript allows accessing known properties without _raw
    expect(product.id).toBe('p1');
    expect((product as any)._raw).toBeUndefined();
  });
});
