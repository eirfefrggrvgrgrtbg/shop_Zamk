import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { GlobalMoneyStrip } from './GlobalMoneyStrip';
import * as sellerApi from '@zamk/api-client/src/seller';
import type { SellerBalance, PayoutBatchListResponse } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerBalance: vi.fn(),
  getSellerPayouts: vi.fn(),
}));

const createMockBalance = (overrides: Partial<SellerBalance> = {}): SellerBalance => ({
  grossSalesCents: 0,
  commissionCents: 0,
  adjustmentsCents: 0,
  frozenCents: 0,
  availableCents: 0,
  paidCents: 0,
  currency: 'RUB',
  ...overrides,
});

describe('GlobalMoneyStrip - Financial Truth Labels', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(sellerApi.getSellerPayouts).mockResolvedValue({
      items: [],
      totalCount: 0,
    } as PayoutBatchListResponse);
  });

  it('Case A: availableCents = +10000 renders "Доступно к выплате" and positive amount', async () => {
    vi.mocked(sellerApi.getSellerBalance).mockResolvedValue(
      createMockBalance({ availableCents: 10000 })
    );

    render(
      <MemoryRouter>
        <GlobalMoneyStrip />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Доступно к выплате')).toBeTruthy();
    });

    expect(screen.queryByText('Баланс')).toBeNull();
    expect(screen.getByText(/100/)).toBeTruthy();
  });

  it('Case B: availableCents = 0 renders "Доступно к выплате"', async () => {
    vi.mocked(sellerApi.getSellerBalance).mockResolvedValue(
      createMockBalance({ availableCents: 0 })
    );

    render(
      <MemoryRouter>
        <GlobalMoneyStrip />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Доступно к выплате')).toBeTruthy();
    });

    expect(screen.queryByText('Баланс')).toBeNull();
    expect(screen.getAllByText(/0\s*₽/).length).toBeGreaterThanOrEqual(1);
  });

  it('Case C: availableCents = -10000 renders "Баланс", exact negative amount, and no "Доступно к выплате"', async () => {
    vi.mocked(sellerApi.getSellerBalance).mockResolvedValue(
      createMockBalance({ availableCents: -10000 })
    );

    render(
      <MemoryRouter>
        <GlobalMoneyStrip />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Баланс')).toBeTruthy();
    });

    expect(screen.queryByText('Доступно к выплате')).toBeNull();
    expect(screen.getByText(/[-−]\s*100/)).toBeTruthy();
  });

  it('Case D: no debt wording appears when balance is negative', async () => {
    vi.mocked(sellerApi.getSellerBalance).mockResolvedValue(
      createMockBalance({ availableCents: -1182090 })
    );

    const { container } = render(
      <MemoryRouter>
        <GlobalMoneyStrip />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Баланс')).toBeTruthy();
    });

    const textContent = container.textContent || '';
    expect(textContent).not.toMatch(/долг/i);
    expect(textContent).not.toMatch(/задолженност/i);
    expect(textContent).not.toMatch(/погашени/i);
    expect(textContent).not.toMatch(/удержа/i);
  });
});
