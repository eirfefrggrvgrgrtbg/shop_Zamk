import { describe, it, expect } from 'vitest';
import {
  cartSignature,
  isNetworkError,
  isPromoRejection,
  normalizePromoCode,
  promoErrorMessage,
  PROMO_GENERIC_MESSAGE,
  PROMO_NETWORK_MESSAGE,
} from './promo';

describe('promo helpers', () => {
  it('maps backend reasons to customer-facing messages', () => {
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_expired' })).toContain('истёк');
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_not_found' })).toContain('не найден');
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_global_limit' })).toContain('исчерпан');
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_customer_limit' })).toContain('уже использовали');
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_not_applicable' })).toContain('не подходит');
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_full_discount_not_allowed' })).toContain('всю стоимость');
  });

  it('never leaks unknown backend text and hides budget details', () => {
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'pq: some sql detail' })).toBe(PROMO_GENERIC_MESSAGE);
    expect(promoErrorMessage({ code: 'invalid_promo', rawMessage: 'promo_budget_exhausted' })).not.toMatch(/бюджет/i);
  });

  it('distinguishes rejections from network errors', () => {
    expect(isPromoRejection({ code: 'invalid_promo' })).toBe(true);
    expect(isPromoRejection({ code: 'invalid_order', rawMessage: 'promo_expired' })).toBe(true);
    expect(isPromoRejection({ code: 'invalid_order', rawMessage: 'insufficient stock' })).toBe(false);
    expect(isPromoRejection({ code: 'NETWORK_ERROR' })).toBe(false);
    expect(isNetworkError({ code: 'TIMEOUT_ERROR' })).toBe(true);
    expect(promoErrorMessage({ code: 'NETWORK_ERROR' })).toBe(PROMO_NETWORK_MESSAGE);
  });

  it('normalizes codes and builds an order-independent cart signature', () => {
    expect(normalizePromoCode('  save10 ')).toBe('SAVE10');
    const a = cartSignature([{ productVariantId: 'v1', quantity: 1 }, { productVariantId: 'v2', quantity: 2 }]);
    const b = cartSignature([{ productVariantId: 'v2', quantity: 2 }, { productVariantId: 'v1', quantity: 1 }]);
    expect(a).toBe(b);
    expect(cartSignature([{ productVariantId: 'v1', quantity: 2 }, { productVariantId: 'v2', quantity: 2 }])).not.toBe(a);
  });
});
