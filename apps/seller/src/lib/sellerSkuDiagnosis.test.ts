import { describe, it, expect } from 'vitest';
import { getSkuDiagnosis } from './seller-products';

const sku = (over: Record<string, unknown> = {}) => ({ size: 'M', stock: 0, variantId: 'v1', active: true, ...over }) as any;

describe('getSkuDiagnosis — SELLER.SKU.WORKFLOW.1', () => {
  it('in stock: ok, no reason, no action', () => {
    const d = getSkuDiagnosis(sku({ total: 5, reserved: 1, available: 4 }), { status: 'published' });
    expect(d.code).toBe('ok');
    expect(d.action).toBe('none');
    expect(d.reason).toBe('');
  });

  it('disabled SKU: seller can open exactly this variant', () => {
    const d = getSkuDiagnosis(sku({ active: false, available: 5 }), { status: 'published' });
    expect(d.code).toBe('disabled');
    expect(d.owner).toBe('seller');
    expect(d.action).toBe('edit_variant');
  });

  it('disabled SKU without a persisted id has no variant link', () => {
    expect(getSkuDiagnosis(sku({ active: false, variantId: undefined }), { status: 'published' }).action).toBe('none');
  });

  it('active, no stock: warehouse owns it; supply is offered only for approved/published products', () => {
    const published = getSkuDiagnosis(sku({ total: 0, reserved: 0, available: 0 }), { status: 'published' });
    expect(published.code).toBe('no_stock');
    expect(published.owner).toBe('warehouse');
    expect(published.action).toBe('supply');
    expect(getSkuDiagnosis(sku({ total: 0, reserved: 0, available: 0 }), { status: 'draft' }).action).not.toBe('supply');
  });

  it('fully reserved stock is reserved, not "no stock"; nothing to do for the seller', () => {
    const d = getSkuDiagnosis(sku({ total: 3, reserved: 3, available: 0 }), { status: 'published' });
    expect(d.code).toBe('reserved');
    expect(d.owner).toBe('system');
    expect(d.action).toBe('none');
  });

  it('pending moderation: moderation owns it, no seller action', () => {
    for (const status of ['pending_moderation', 'in_review'] as const) {
      const d = getSkuDiagnosis(sku({ available: 5 }), { status });
      expect(d.code).toBe('product_pending');
      expect(d.owner).toBe('moderation');
      expect(d.action).toBe('none');
    }
  });

  it('blocked and hidden products: no seller action', () => {
    expect(getSkuDiagnosis(sku({ available: 5 }), { status: 'blocked' }).action).toBe('none');
    expect(getSkuDiagnosis(sku({ available: 5 }), { status: 'hidden' }).action).toBe('none');
    expect(getSkuDiagnosis(sku({ active: false }), { status: 'blocked' }).action).toBe('none');
  });

  it('missing warehouse numbers are unknown, never an invented zero', () => {
    const d = getSkuDiagnosis(sku({ stock: 0 }), { status: 'published' });
    expect(d.code).toBe('unknown');
    expect(d.action).toBe('none');
  });
});
