import { describe, it, expect } from 'vitest';
import { resolveCategoryDisplayName, adaptProductList } from './adapter';

describe('adapter - Category Name Resolution & Display Purity', () => {
  const categoryMap = {
    'c741aa40-4f5f-4b58-8581-5cfae5e77c16': 'Худи',
    'cat-shoes-id-1234': 'Обувь',
    'cat-bag-id-5678': 'Сумки',
  };

  it('1. Resolves direct string categoryName on product without UUID leak', () => {
    const p = { id: 'p1', title: 'Лонг', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16', categoryName: 'Худи' };
    expect(resolveCategoryDisplayName(p, categoryMap)).toBe('Худи');
  });

  it('2. Resolves nested category.name', () => {
    const p = { id: 'p1', title: 'Лонг', category: { id: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16', name: 'Худи' } };
    expect(resolveCategoryDisplayName(p, categoryMap)).toBe('Худи');
  });

  it('3. Resolves categoryId using taxonomy categoryMap when categoryName is missing', () => {
    const p = { id: 'p1', title: 'Лонг', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16' };
    expect(resolveCategoryDisplayName(p, categoryMap)).toBe('Худи');
  });

  it('4. Resolves multiple products with same categoryId to the same label ("Худи")', () => {
    const p1 = { id: 'p1', title: 'Лонг Черный', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16' };
    const p2 = { id: 'p2', title: 'Лонг Белый', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16' };
    expect(resolveCategoryDisplayName(p1, categoryMap)).toBe('Худи');
    expect(resolveCategoryDisplayName(p2, categoryMap)).toBe('Худи');
  });

  it('5. Resolves different categoryIds to their respective labels', () => {
    const p1 = { id: 'p1', title: 'Лонг', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16' };
    const p2 = { id: 'p2', title: 'Ботинки', categoryId: 'cat-shoes-id-1234' };
    const p3 = { id: 'p3', title: 'Рюкзак', categoryId: 'cat-bag-id-5678' };
    expect(resolveCategoryDisplayName(p1, categoryMap)).toBe('Худи');
    expect(resolveCategoryDisplayName(p2, categoryMap)).toBe('Обувь');
    expect(resolveCategoryDisplayName(p3, categoryMap)).toBe('Сумки');
  });

  it('6. Resolves human string categoryId (non-UUID) directly if not in categoryMap', () => {
    const p = { id: 'p1', title: 'Лонг', categoryId: 'Одежда' };
    expect(resolveCategoryDisplayName(p, {})).toBe('Одежда');
  });

  it('7. Returns "Категория не определена" ONLY when categoryId is an unresolved UUID and not in categoryMap', () => {
    const p = { id: 'p1', title: 'Неизвестный', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16' };
    expect(resolveCategoryDisplayName(p, {})).toBe('Категория не определена');
    expect(resolveCategoryDisplayName(p, undefined)).toBe('Категория не определена');
  });

  it('8. adaptProductList correctly attaches resolved category name for all products', () => {
    const apiProducts = [
      { id: 'p1', title: 'Лонг', categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16', priceCents: 10000 },
      { id: 'p2', title: 'Ботинки', categoryId: 'cat-shoes-id-1234', priceCents: 20000 },
      { id: 'p3', title: 'Удаленный', categoryId: '00000000-0000-0000-0000-000000000000', priceCents: 30000 },
    ];
    const adapted = adaptProductList(apiProducts, categoryMap);
    expect(adapted[0].category).toBe('Худи');
    expect(adapted[1].category).toBe('Обувь');
    expect(adapted[2].category).toBe('Категория не определена');
  });
});
