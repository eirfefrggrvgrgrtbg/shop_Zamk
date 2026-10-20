import { describe, it, expect } from 'vitest';
import { buildProductStudioUpdateRequest } from './productStudioSaveProduct';
import { reconcileProductStudioVariantMatrix } from './productStudioMatrixHelper';
import type { ProductStudioDraft, ProductStudioVariant } from '../../contexts/ProductStudioContext';

const draft = (variants: ProductStudioVariant[]): ProductStudioDraft => ({ title: 'T', description: 'D', variants } as any);

describe('CATALOG.C4 shade / legacy color in Studio save and matrix', () => {
  it('payload carries colorId and shadeName; undefined shade is omitted, empty string clears', () => {
    const base = draft([]);
    const payload = buildProductStudioUpdateRequest(
      draft([
        { id: 'a1b2c3d4-0000-4000-8000-000000000001', colorId: 'col-1', shadeName: 'Угольный', sizeValueId: 's', sellerSku: 'A', isActive: true },
        { id: 'a1b2c3d4-0000-4000-8000-000000000002', colorId: 'col-1', shadeName: '', sizeValueId: 'm', sellerSku: 'B', isActive: true },
        { id: 'a1b2c3d4-0000-4000-8000-000000000003', colorId: 'col-1', sizeValueId: 'l', sellerSku: 'C', isActive: true },
      ]),
      base
    );
    const vs = payload.variants as any[];
    expect(vs[0]).toMatchObject({ colorId: 'col-1', shadeName: 'Угольный' });
    expect(vs[1].shadeName).toBe('');
    expect(vs[2].shadeName).toBeUndefined();
    expect(vs.every((v) => !('color' in v))).toBe(true); // legacy text is never sent: the server keeps it
  });

  it('adopts colorless legacy variants into the FIRST chosen color (same size), without duplicates', () => {
    const legacy: ProductStudioVariant[] = [
      { id: 'L-S', sizeValueId: 'sz-s', size: 'S', sellerSku: 'SKU-S', priceCents: 10, legacyColorText: 'Black', isActive: true },
      { id: 'L-M', sizeValueId: 'sz-m', size: 'M', sellerSku: 'SKU-M', priceCents: 20, legacyColorText: 'Black', isActive: true },
    ];
    const colors = [{ id: 'c-white', name: 'Белый' }, { id: 'c-red', name: 'Красный' }];
    const sizes = [{ id: 'sz-s', label: 'S' }, { id: 'sz-m', label: 'M' }];
    const out = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', colors, sizes, legacy, [], sizes);
    const white = out.filter((v) => v.colorId === 'c-white');
    expect(white.map((v) => v.id).sort()).toEqual(['L-M', 'L-S']);
    expect(white.find((v) => v.id === 'L-S')).toMatchObject({ sellerSku: 'SKU-S', priceCents: 10, legacyColorText: 'Black' });
    const red = out.filter((v) => v.colorId === 'c-red');
    expect(red).toHaveLength(2);
    expect(red.every((v) => !['L-S', 'L-M'].includes(v.id as string))).toBe(true);
    expect(out).toHaveLength(4);
  });

  it('does not touch colorless variants while no color is chosen', () => {
    const legacy: ProductStudioVariant[] = [{ id: 'L-S', sizeValueId: 'sz-s', size: 'S', isActive: true }];
    const out = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', [], [{ id: 'sz-s', label: 'S' }], legacy, [], [{ id: 'sz-s', label: 'S' }]);
    expect(out).toHaveLength(1);
    expect(out[0].colorId).toBeUndefined();
  });
});
