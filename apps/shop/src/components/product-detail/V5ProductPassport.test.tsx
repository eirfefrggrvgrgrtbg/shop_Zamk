/**
 * @vitest-environment jsdom
 */
import { describe, it, expect } from 'vitest';
import { buildPassportRows } from './v5PassportRows';

describe('buildPassportRows: real data only', () => {
  const base = { id: '8f14e45f-ceea-4d2b-9f0a-0123456789ab', brand: 'Maison', materialComposition: undefined, materials: undefined };

  it('shows the selected variant seller SKU as "Артикул варианта", upper-cased, and no product code next to it', () => {
    const rows = buildPassportRows(base, { sellerSku: 'abc-1' });
    expect(rows.find((r) => r.label === 'Артикул варианта')?.value).toBe('ABC-1');
    expect(rows.some((r) => r.label === 'Код товара')).toBe(false);
  });

  it('before a variant is resolved shows only the technical "Код товара" (id tail), never labelled as an article', () => {
    const rows = buildPassportRows(base, null);
    expect(rows.find((r) => r.label === 'Код товара')?.value).toBe('0123456789ab');
    expect(rows.some((r) => r.label.startsWith('Артикул'))).toBe(false);
  });

  it('a selected variant without a seller SKU falls back to the product code (no empty or stale SKU row)', () => {
    const rows = buildPassportRows(base, { sellerSku: undefined });
    expect(rows.some((r) => r.label === 'Артикул варианта')).toBe(false);
    expect(rows.some((r) => r.label === 'Код товара')).toBe(true);
  });

  it('skips the placeholder brand and UUID brands', () => {
    expect(buildPassportRows({ ...base, brand: 'Бренд не указан' }, null).some((r) => r.label === 'Бренд')).toBe(false);
    expect(buildPassportRows({ ...base, brand: base.id }, null).some((r) => r.label === 'Бренд')).toBe(false);
  });

  it('joins the composition, or falls back to the plain material, or omits both', () => {
    const withComp = buildPassportRows({ ...base, materialComposition: [{ materialName: 'Шерсть', percentage: 80 }, { material: 'Полиамид', percentage: 20 }] }, null);
    expect(withComp.find((r) => r.label === 'Состав')?.value).toBe('Шерсть 80% · Полиамид 20%');
    expect(buildPassportRows({ ...base, materials: 'Хлопок' }, null).find((r) => r.label === 'Материал')?.value).toBe('Хлопок');
    expect(buildPassportRows(base, null).map((r) => r.label)).toEqual(['Бренд', 'Код товара']);
  });
});
