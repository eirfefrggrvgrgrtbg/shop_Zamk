import { describe, it, expect } from 'vitest';
import type { ProductStudioVariant } from '../../contexts/ProductStudioContext';
import {
  computeRequiredVariantTuples,
  reconcileProductStudioVariantMatrix,
  toggleProductStudioVariantTuple,
  isVariantLockedByStock,
} from './productStudioMatrixHelper';
import { buildProductStudioUpdateRequest, getProductStudioSaveErrorMessage } from './productStudioSaveProduct';
import { ApiError } from '@zamk/api-client';

// CATALOG.C5 — incomplete color x size matrix.
const black = { id: 'col-black', name: 'Чёрный' };
const white = { id: 'col-white', name: 'Белый' };
const S = { id: 'sz-s', label: 'S' };
const M = { id: 'sz-m', label: 'M' };

const persisted = (id: string, colorId: string, sizeValueId: string, extra: Partial<ProductStudioVariant> = {}): ProductStudioVariant => ({
  id,
  colorId,
  sizeValueId,
  sellerSku: `SKU-${id}`,
  priceCents: 990000,
  isActive: true,
  ...extra,
});

const UUID_BS = '11111111-1111-4111-8111-111111111111';
const UUID_BM = '22222222-2222-4222-8222-222222222222';
const UUID_WS = '33333333-3333-4333-8333-333333333333';
const UUID_WM = '44444444-4444-4444-8444-444444444444';

describe('incomplete matrix: seller keeps only the combinations that are produced', () => {
  it('starts from the full matrix, switching cells off leaves black/S and white/M only', () => {
    const full = computeRequiredVariantTuples('COLOR_AND_SIZE', [black, white], [S, M]);
    expect(full).toHaveLength(4);
    let variants: ProductStudioVariant[] = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', [black, white], [S, M], []);

    let res = toggleProductStudioVariantTuple(variants, { colorId: 'col-black', sizeValueId: 'sz-m' }, true);
    expect(res.allowed).toBe(true);
    variants = res.variants;
    res = toggleProductStudioVariantTuple(variants, { colorId: 'col-white', sizeValueId: 'sz-s' }, true);
    expect(res.allowed).toBe(true);
    variants = res.variants;

    expect(variants.map((v) => `${v.colorId}/${v.sizeValueId}`).sort()).toEqual(['col-black/sz-s', 'col-white/sz-m']);

    // last remaining cell of each axis cannot be switched off
    expect(toggleProductStudioVariantTuple(variants, { colorId: 'col-black', sizeValueId: 'sz-s' }, true).reason).toBe('LAST_CELL_IN_AXIS');

    // only the selected combinations are sent on save
    const payload = buildProductStudioUpdateRequest({ title: 't', description: '', variants } as any, { title: 't', description: '' } as any);
    expect(payload.variants).toHaveLength(2);
  });

  it('re-hydrated incomplete matrix is preserved (no self-healing) when sizes/colors are re-applied', () => {
    const variants = [persisted(UUID_BS, 'col-black', 'sz-s'), persisted(UUID_WM, 'col-white', 'sz-m')];
    const next = reconcileProductStudioVariantMatrix('COLOR_AND_SIZE', [black, white], [S, M], variants, [black, white], [S, M]);
    expect(next.map((v) => v.id).sort()).toEqual([UUID_BS, UUID_WM].sort());
  });

  it('adding a combination later keeps existing ids, SKUs and prices and creates exactly one new variant', () => {
    const variants = [persisted(UUID_BS, 'col-black', 'sz-s'), persisted(UUID_WM, 'col-white', 'sz-m')];
    const res = toggleProductStudioVariantTuple(variants, { colorId: 'col-black', sizeValueId: 'sz-m', size: 'M' }, false);
    expect(res.variants).toHaveLength(3);
    expect(res.variants.find((v) => v.id === UUID_BS)).toEqual(variants[0]);
    expect(res.variants.find((v) => v.id === UUID_WM)).toEqual(variants[1]);

    const payload = buildProductStudioUpdateRequest({ title: 't', description: '', variants: res.variants } as any, { title: 't', description: '' } as any);
    const ids = payload.variants!.map((v) => v.id);
    expect(ids).toContain(UUID_BS);
    expect(ids).toContain(UUID_WM);
    expect(ids.filter((id) => id === undefined)).toHaveLength(1);
  });

  it('switching an already selected cell on does not duplicate the combination', () => {
    const variants = [persisted(UUID_BS, 'col-black', 'sz-s'), persisted(UUID_WM, 'col-white', 'sz-m')];
    const res = toggleProductStudioVariantTuple(variants, { colorId: 'col-black', sizeValueId: 'sz-s' }, false);
    expect(res.variants).toHaveLength(2);
  });

  it('switching a cell off and on again restores the same variant (id, SKU, price)', () => {
    const bm = persisted(UUID_BM, 'col-black', 'sz-m', { priceCents: 1234500 });
    const variants = [persisted(UUID_BS, 'col-black', 'sz-s'), bm, persisted(UUID_WS, 'col-white', 'sz-s'), persisted(UUID_WM, 'col-white', 'sz-m')];
    const off = toggleProductStudioVariantTuple(variants, { colorId: 'col-black', sizeValueId: 'sz-m' }, true);
    expect(off.variants).toHaveLength(3);
    const on = toggleProductStudioVariantTuple(off.variants, { colorId: 'col-black', sizeValueId: 'sz-m' }, false, undefined, [bm]);
    expect(on.variants.find((v) => v.colorId === 'col-black' && v.sizeValueId === 'sz-m')).toEqual(bm);
  });

  it('a variant with stock or reserve cannot be switched off', () => {
    const withStock = persisted(UUID_BM, 'col-black', 'sz-m', { totalStock: 3, reservedStock: 0 });
    const withReserve = persisted(UUID_WS, 'col-white', 'sz-s', { totalStock: 1, reservedStock: 1 });
    expect(isVariantLockedByStock(withStock)).toBe(true);
    expect(isVariantLockedByStock(withReserve)).toBe(true);
    expect(isVariantLockedByStock(persisted(UUID_BS, 'col-black', 'sz-s', { totalStock: 0, reservedStock: 0 }))).toBe(false);

    const variants = [persisted(UUID_BS, 'col-black', 'sz-s'), withStock, withReserve, persisted(UUID_WM, 'col-white', 'sz-m')];
    const res = toggleProductStudioVariantTuple(variants, { colorId: 'col-black', sizeValueId: 'sz-m' }, true);
    expect(res.allowed).toBe(false);
    expect(res.reason).toBe('HAS_STOCK');
    expect(res.variants).toBe(variants);
  });

  it('stock fields are never sent to the backend', () => {
    const variants = [persisted(UUID_BS, 'col-black', 'sz-s', { totalStock: 5, reservedStock: 2 })];
    const payload = buildProductStudioUpdateRequest({ title: 't', description: '', variants } as any, { title: 't', description: '' } as any);
    expect(Object.keys(payload.variants![0])).not.toContain('totalStock');
    expect(Object.keys(payload.variants![0])).not.toContain('reservedStock');
  });
});

describe('save error message', () => {
  it('shows the backend reason when a variant deactivation is refused', () => {
    const msg = 'Нельзя убрать «Чёрный / M»: есть незавершённые заказы. Вариант сохранён без изменений.';
    const err = new ApiError('x', 'variant_deactivation_blocked', 409, { error: { code: 'variant_deactivation_blocked', message: msg } }, msg);
    expect(getProductStudioSaveErrorMessage(err)).toBe(msg);
  });

  it('keeps the generic message for other errors', () => {
    expect(getProductStudioSaveErrorMessage(new ApiError('boom', 'internal_error', 500, {}, 'secret detail'))).toBe(
      'Не удалось сохранить товар. Попробуйте ещё раз.'
    );
    expect(getProductStudioSaveErrorMessage(undefined)).toBe('Не удалось сохранить товар. Попробуйте ещё раз.');
  });
});
