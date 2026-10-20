/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, act, waitFor } from '@testing-library/react';
import { PromoProvider, usePromo } from './PromoContext';

const validatePromo = vi.fn();
vi.mock('@zamk/api-client/src/customer', () => ({
  validatePromo: (code: string) => validatePromo(code),
}));

let auth = { isAuthenticated: true, isInitializing: false };
vi.mock('./AuthContext', () => ({ useAuth: () => auth }));

let cart = { items: [{ productVariantId: 'v1', quantity: 1 }] as any[], isLoadingCart: false };
vi.mock('./CartContext', () => ({ useCart: () => cart }));

let api: ReturnType<typeof usePromo>;
function Probe() {
  api = usePromo();
  return (
    <div>
      <span data-testid='code'>{api.appliedCode ?? ''}</span>
      <span data-testid='discount'>{api.discount}</span>
      <span data-testid='checking'>{String(api.isChecking)}</span>
      <span data-testid='unconfirmed'>{String(api.isUnconfirmed)}</span>
      <span data-testid='error'>{api.error ?? ''}</span>
    </div>
  );
}

const view = () => (
  <PromoProvider>
    <Probe />
  </PromoProvider>
);
const text = (id: string) => screen.getByTestId(id).textContent;
const apiError = (code: string, rawMessage: string) => Object.assign(new Error(rawMessage), { code, rawMessage });

describe('PromoContext', () => {
  beforeEach(() => {
    validatePromo.mockReset();
    sessionStorage.clear();
    auth = { isAuthenticated: true, isInitializing: false };
    cart = { items: [{ productVariantId: 'v1', quantity: 1 }], isLoadingCart: false };
  });
  afterEach(() => cleanup());

  it('applies a valid code and exposes the server discount', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 50000 });
    render(view());
    let ok = false;
    await act(async () => { ok = await api.apply(' save10 '); });
    expect(ok).toBe(true);
    expect(validatePromo).toHaveBeenCalledWith('SAVE10');
    expect(text('code')).toBe('SAVE10');
    expect(text('discount')).toBe('500');
    expect(sessionStorage.getItem('zamk_promo_code')).toBe('SAVE10');
  });

  it.each([
    ['promo_not_found', 'не найден'],
    ['promo_expired', 'истёк'],
    ['promo_global_limit', 'исчерпан'],
    ['promo_customer_limit', 'уже использовали'],
    ['promo_not_applicable', 'не подходит'],
    ['promo_full_discount_not_allowed', 'всю стоимость'],
  ])('rejects %s with a clear reason and applies nothing', async (reason, fragment) => {
    validatePromo.mockRejectedValue(apiError('invalid_promo', reason));
    render(view());
    await act(async () => { await api.apply('BAD'); });
    expect(text('code')).toBe('');
    expect(text('discount')).toBe('0');
    expect(text('error')).toContain(fragment);
  });

  it('keeps the previous code when a replacement fails', async () => {
    validatePromo.mockResolvedValueOnce({ code: 'OLD', discountCents: 10000 });
    render(view());
    await act(async () => { await api.apply('OLD'); });
    validatePromo.mockRejectedValueOnce(apiError('invalid_promo', 'promo_expired'));
    await act(async () => { await api.apply('NEW'); });
    expect(text('code')).toBe('OLD');
    expect(text('discount')).toBe('100');
    expect(text('error')).toContain('истёк');
  });

  it('removes the code', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 50000 });
    render(view());
    await act(async () => { await api.apply('SAVE10'); });
    act(() => api.remove());
    expect(text('code')).toBe('');
    expect(text('discount')).toBe('0');
    expect(sessionStorage.getItem('zamk_promo_code')).toBeNull();
  });

  it('hides the discount after the cart changes until the server confirms the new amount', async () => {
    validatePromo.mockResolvedValueOnce({ code: 'SAVE10', discountCents: 50000 });
    const { rerender } = render(view());
    await act(async () => { await api.apply('SAVE10'); });
    expect(text('discount')).toBe('500');

    let resolve!: (v: unknown) => void;
    validatePromo.mockReturnValueOnce(new Promise((r) => { resolve = r; }));
    cart = { items: [{ productVariantId: 'v1', quantity: 3 }], isLoadingCart: false };
    rerender(view());

    // In flight: no stale discount is shown.
    expect(text('discount')).toBe('0');
    expect(text('checking')).toBe('true');

    await act(async () => { resolve({ code: 'SAVE10', discountCents: 150000 }); });
    await waitFor(() => expect(text('discount')).toBe('1500'));
    expect(text('checking')).toBe('false');
  });

  it('drops the code when re-validation after a cart change rejects it', async () => {
    validatePromo.mockResolvedValueOnce({ code: 'SAVE10', discountCents: 50000 });
    const { rerender } = render(view());
    await act(async () => { await api.apply('SAVE10'); });

    validatePromo.mockRejectedValueOnce(apiError('invalid_promo', 'promo_min_subtotal'));
    cart = { items: [{ productVariantId: 'v1', quantity: 1 }, { productVariantId: 'v2', quantity: 1 }], isLoadingCart: false };
    rerender(view());
    await waitFor(() => expect(text('code')).toBe(''));
    expect(text('discount')).toBe('0');
    expect(text('error')).toContain('Сумма товаров');
  });

  it('keeps the code unconfirmed (discount hidden) on a network error and allows retry', async () => {
    validatePromo.mockResolvedValueOnce({ code: 'SAVE10', discountCents: 50000 });
    const { rerender } = render(view());
    await act(async () => { await api.apply('SAVE10'); });

    validatePromo.mockRejectedValueOnce(apiError('NETWORK_ERROR', 'down'));
    cart = { items: [{ productVariantId: 'v1', quantity: 2 }], isLoadingCart: false };
    rerender(view());
    await waitFor(() => expect(text('unconfirmed')).toBe('true'));
    expect(text('code')).toBe('SAVE10');
    expect(text('discount')).toBe('0');

    validatePromo.mockResolvedValueOnce({ code: 'SAVE10', discountCents: 100000 });
    act(() => api.retry());
    await waitFor(() => expect(text('discount')).toBe('1000'));
  });

  it('restores a stored code after reload and re-confirms it with the server', async () => {
    sessionStorage.setItem('zamk_promo_code', 'SAVE10');
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 50000 });
    render(view());
    expect(text('discount')).toBe('0'); // not shown before the server confirms
    await waitFor(() => expect(text('discount')).toBe('500'));
  });

  it('drops the code after checkout rejected it', async () => {
    validatePromo.mockResolvedValue({ code: 'SAVE10', discountCents: 50000 });
    render(view());
    await act(async () => { await api.apply('SAVE10'); });
    act(() => api.rejectAfterCheckout(apiError('invalid_order', 'promo_global_limit')));
    expect(text('code')).toBe('');
    expect(text('discount')).toBe('0');
    expect(text('error')).toContain('исчерпан');
  });

  it('does not call the API for guests and forgets any stored code', async () => {
    auth = { isAuthenticated: false, isInitializing: false };
    sessionStorage.setItem('zamk_promo_code', 'SAVE10');
    render(view());
    await waitFor(() => expect(text('code')).toBe(''));
    expect(validatePromo).not.toHaveBeenCalled();
  });
});
