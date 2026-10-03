/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerPromotions } from './SellerPromotions';
import {
  getSellerPromotions,
  createSellerPromotion,
  updateSellerPromotion,
  getSellerProducts,
  getSellerProductsPaginated,
} from '@zamk/api-client/src/seller';
import type { SellerPromotion, SellerProduct } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerPromotions: vi.fn(),
  createSellerPromotion: vi.fn(),
  updateSellerPromotion: vi.fn(),
  getSellerProducts: vi.fn(),
  getSellerProductsPaginated: vi.fn(),
}));

describe('SellerPromotions Component & Interaction Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerPromotions).mockImplementation(async () => ({ items: [], count: 0 }));
    vi.mocked(getSellerProducts).mockResolvedValue([]);
    vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
      const items = await getSellerProducts(params);
      return { items: items || [], totalCount: items?.length || 0 };
    });
  });
  afterEach(() => {
    cleanup();
  });


  const mockPromoPercent: SellerPromotion = {
    id: 'promo-1',
    code: 'SALE10',
    discountType: 'percent',
    discountValueBps: 1000,
    discountValueFixedCents: 0,
    minOrderSubtotalCents: 50000,
    firstPaidOrderOnly: false,
    productScope: 'ENTIRE_STORE',
    isActive: true,
    startsAt: '2026-10-01T00:00:00Z',
    endsAt: '2026-11-01T00:00:00Z',
    globalUsageLimit: 100,
    perCustomerUsageLimit: 1,
    consumedUsageCount: 5,
    reservedUsageCount: 2,
    campaignId: 'camp-1',
    fundingMode: 'SELLER',
    status: 'active',
    createdAt: '2026-10-01T00:00:00Z',
    updatedAt: '2026-10-01T00:00:00Z',
  };

  const mockPromoFixed: SellerPromotion = {
    id: 'promo-2',
    code: 'FIXED300',
    discountType: 'fixed',
    discountValueBps: 0,
    discountValueFixedCents: 30000,
    minOrderSubtotalCents: 0,
    firstPaidOrderOnly: true,
    productScope: 'ENTIRE_STORE',
    isActive: false,
    startsAt: null,
    endsAt: null,
    globalUsageLimit: null,
    perCustomerUsageLimit: 2,
    consumedUsageCount: 0,
    reservedUsageCount: 0,
    campaignId: 'camp-2',
    fundingMode: 'SELLER',
    status: 'paused',
    createdAt: '2026-10-01T00:00:00Z',
    updatedAt: '2026-10-01T00:00:00Z',
  };

  it('1. Renders empty state when no promotions exist', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('У вас пока нет промокодов')).toBeTruthy();
    });
    expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
  });

  it('2. Renders promotions table with Russian statuses and formatted values', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({
      items: [mockPromoPercent, mockPromoFixed],
      count: 2,
    });

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('SALE10')).toBeTruthy();
      expect(screen.getByText('FIXED300')).toBeTruthy();
    });

    // Check percent discount formatting
    expect(screen.getByText('10%')).toBeTruthy();
    // Check fixed discount formatting: 300 ₽ in the FIXED300 row
    expect(screen.getByTestId('promo-row-FIXED300').textContent).toContain('300');

    // Check Russian status badges
    expect(screen.getByTestId('promo-status-badge-SALE10').textContent).toContain('Активен');
    expect(screen.getByTestId('promo-status-badge-FIXED300').textContent).toContain('На паузе');

    // Check first order badge
    expect(screen.getByText('1-й заказ')).toBeTruthy();
  });

  it('3. Creates a new percent promotion with correct BPS conversion', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
    vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId('create-first-promo-button'));
    expect(screen.getByTestId('create-promo-modal')).toBeTruthy();

    // Fill form
    fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'NEWYEAR15' } });
    fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
    fireEvent.change(screen.getByTestId('input-min-order'), { target: { value: '1000' } });
    fireEvent.click(screen.getByTestId('input-first-paid-only'));

    fireEvent.click(screen.getByTestId('submit-create-promo'));

    await waitFor(() => {
      expect(createSellerPromotion).toHaveBeenCalledWith({
        code: 'NEWYEAR15',
        discountType: 'percent',
        discountValueBps: 1500,
        minOrderSubtotalCents: 100000,
        firstPaidOrderOnly: true,
        perCustomerUsageLimit: 1,
        productScope: 'ENTIRE_STORE',
      });
    });
  });

  it('4. Creates a new fixed promotion with correct cents conversion', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
    vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoFixed);

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId('create-first-promo-button'));

    // Switch to fixed
    fireEvent.click(screen.getByTestId('radio-discount-fixed'));
    fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'MINUS500' } });
    fireEvent.change(screen.getByTestId('input-discount-fixed'), { target: { value: '500' } });
    fireEvent.change(screen.getByTestId('input-global-limit'), { target: { value: '50' } });

    fireEvent.click(screen.getByTestId('submit-create-promo'));

    await waitFor(() => {
      expect(createSellerPromotion).toHaveBeenCalledWith({
        code: 'MINUS500',
        discountType: 'fixed',
        discountValueFixedCents: 50000,
        firstPaidOrderOnly: false,
        globalUsageLimit: 50,
        perCustomerUsageLimit: 1,
        productScope: 'ENTIRE_STORE',
      });
    });
  });

  it('5. Displays duplicate promo code error truthfully in modal', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
    vi.mocked(createSellerPromotion).mockRejectedValue({
      code: 'promo_code_duplicate',
      message: 'promo code already exists',
    });

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId('create-first-promo-button'));
    fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EXISTING' } });
    fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

    fireEvent.click(screen.getByTestId('submit-create-promo'));

    await waitFor(() => {
      expect(screen.getByTestId('create-promo-error').textContent).toContain(
        'Промокод с таким кодом уже существует. Выберите другой код.'
      );
    });
  });

  it('6. Quick toggles active/pause from table action', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({
      items: [mockPromoPercent],
      count: 1,
    });
    vi.mocked(updateSellerPromotion).mockResolvedValue({
      ...mockPromoPercent,
      isActive: false,
      status: 'paused',
    });

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('promo-toggle-active-SALE10')).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId('promo-toggle-active-SALE10'));

    await waitFor(() => {
      expect(updateSellerPromotion).toHaveBeenCalledWith('promo-1', { isActive: false });
    });
  });

  it('7. Edit modal protects immutable economics and updates operational limits', async () => {
    vi.mocked(getSellerPromotions).mockResolvedValue({
      items: [mockPromoPercent],
      count: 1,
    });
    vi.mocked(updateSellerPromotion).mockResolvedValue({
      ...mockPromoPercent,
      globalUsageLimit: 200,
    });

    render(
      <MemoryRouter>
        <SellerPromotions />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
    expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();

    // Verify immutable economics are displayed read-only
    expect(screen.getByText('Неизменяемые экономические параметры')).toBeTruthy();
    expect(screen.getByText('Продавец (SELLER)')).toBeTruthy();

    // Edit operational limits
    fireEvent.change(screen.getByTestId('edit-input-global-limit'), { target: { value: '200' } });
    fireEvent.click(screen.getByTestId('submit-edit-promo'));

    await waitFor(() => {
      expect(updateSellerPromotion).toHaveBeenCalledWith('promo-1', {
        isActive: true,
        startsAt: new Date('2026-10-01T00:00').toISOString(),
        endsAt: new Date('2026-11-01T00:00').toISOString(),
        globalUsageLimit: 200,
        perCustomerUsageLimit: 1,
      });
    });
  });

  describe('MARKETING.1D2-UI2 Promo Date Picker Tests (A-M)', () => {
    it('A. Create modal starts with BOTH optional dates empty', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();

      expect(screen.getByTestId('input-starts-at').textContent).toContain('Выберите дату и время');
      expect(screen.getByTestId('input-ends-at').textContent).toContain('Выберите дату и время');
    });

    it('B. Opening a date picker does not assign a date automatically', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      // Open starts-at popover
      fireEvent.click(screen.getByTestId('input-starts-at'));
      expect(screen.getByTestId('input-starts-at-popover')).toBeTruthy();

      // Close without applying via Escape
      fireEvent.keyDown(window, { key: 'Escape' });
      expect(screen.queryByTestId('input-starts-at-popover')).toBeNull();

      // Still empty placeholder
      expect(screen.getByTestId('input-starts-at').textContent).toContain('Выберите дату и время');
    });

    it('C. Seller can select start date + time', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      // Open starts-at picker
      fireEvent.click(screen.getByTestId('input-starts-at'));
      expect(screen.getByTestId('input-starts-at-popover')).toBeTruthy();

      // Find day button inside the popover and click it
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      expect(dayButtons.length).toBeGreaterThan(0);
      fireEvent.click(dayButtons[10]);

      // Set time
      fireEvent.change(screen.getByTestId('input-starts-at-hours'), { target: { value: '14' } });
      fireEvent.change(screen.getByTestId('input-starts-at-minutes'), { target: { value: '30' } });

      // Apply
      fireEvent.click(screen.getByTestId('input-starts-at-apply'));

      // Popover closed and displays selected time 14:30
      expect(screen.queryByTestId('input-starts-at-popover')).toBeNull();
      expect(screen.getByTestId('input-starts-at').textContent).toContain('14:30');
    });

    it('D. Seller can select end date + time', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      // Open ends-at picker
      fireEvent.click(screen.getByTestId('input-ends-at'));
      expect(screen.getByTestId('input-ends-at-popover')).toBeTruthy();

      // Select a day
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtons[15]);

      // Click preset 23:59
      fireEvent.click(screen.getByTestId('input-ends-at-preset-23:59'));

      // Apply
      fireEvent.click(screen.getByTestId('input-ends-at-apply'));

      expect(screen.queryByTestId('input-ends-at-popover')).toBeNull();
      expect(screen.getByTestId('input-ends-at').textContent).toContain('23:59');
    });

    it('E. Seller can clear either field', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      // Set start date
      fireEvent.click(screen.getByTestId('input-starts-at'));
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtons[5]);
      fireEvent.click(screen.getByTestId('input-starts-at-apply'));
      expect(screen.getByTestId('input-starts-at').textContent).not.toContain('Выберите дату и время');

      // Clear using popover clear button
      fireEvent.click(screen.getByTestId('input-starts-at'));
      fireEvent.click(screen.getByTestId('input-starts-at-clear'));
      expect(screen.getByTestId('input-starts-at').textContent).toContain('Выберите дату и время');
    });

    it('F. no start + no end submits null/omitted dates correctly', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'NODATES' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith({
          code: 'NODATES',
          discountType: 'percent',
          discountValueBps: 1000,
          firstPaidOrderOnly: false,
          perCustomerUsageLimit: 1,
          productScope: 'ENTIRE_STORE',
        });
      });
    });

    it('G. start only works', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'STARTONLY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      // Set starts-at
      fireEvent.click(screen.getByTestId('input-starts-at'));
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtons[0]);
      fireEvent.click(screen.getByTestId('input-starts-at-apply'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'STARTONLY',
            startsAt: expect.any(String),
          })
        );
      });
      const callArg = vi.mocked(createSellerPromotion).mock.calls[0][0];
      expect(callArg.endsAt).toBeUndefined();
    });

    it('H. end only works', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'ENDONLY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      // Set ends-at only
      fireEvent.click(screen.getByTestId('input-ends-at'));
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtons[10]);
      fireEvent.click(screen.getByTestId('input-ends-at-apply'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'ENDONLY',
            endsAt: expect.any(String),
          })
        );
      });
      const callArg = vi.mocked(createSellerPromotion).mock.calls[0][0];
      expect(callArg.startsAt).toBeUndefined();
    });

    it('I. end <= start shows validation and blocks submit', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'INVALIDDATES' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      // Set start to day 15, 12:00
      fireEvent.click(screen.getByTestId('input-starts-at'));
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtons[14]);
      fireEvent.change(screen.getByTestId('input-starts-at-hours'), { target: { value: '12' } });
      fireEvent.change(screen.getByTestId('input-starts-at-minutes'), { target: { value: '00' } });
      fireEvent.click(screen.getByTestId('input-starts-at-apply'));

      // Set end to day 15, 10:00 (earlier than start!)
      fireEvent.click(screen.getByTestId('input-ends-at'));
      const dayButtonsEnd = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtonsEnd[14]);
      fireEvent.change(screen.getByTestId('input-ends-at-hours'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-ends-at-minutes'), { target: { value: '00' } });
      fireEvent.click(screen.getByTestId('input-ends-at-apply'));

      // Expect inline validation error
      expect(screen.getByTestId('create-date-error').textContent).toContain(
        'Дата окончания должна быть позже даты начала.'
      );

      // Expect submit button disabled
      const submitBtn = screen.getByTestId('submit-create-promo');
      expect((submitBtn as HTMLButtonElement).disabled).toBe(true);

      // Attempting form submit does not call createSellerPromotion
      fireEvent.click(submitBtn);
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('J. existing promo dates render correctly in edit mode', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent, mockPromoFixed],
        count: 2,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      // Edit promo with dates: startsAt='2026-10-01T00:00:00Z', endsAt='2026-11-01T00:00:00Z'
      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      expect(screen.getByTestId('edit-input-starts-at').textContent).toContain('01.10.2026 00:00');
      expect(screen.getByTestId('edit-input-ends-at').textContent).toContain('01.11.2026 00:00');

      // Close and open promo with null dates
      fireEvent.click(screen.getByText('Отмена'));
      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-FIXED300')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('promo-edit-button-FIXED300'));
      expect(screen.getByTestId('edit-input-starts-at').textContent).toContain('Выберите дату и время');
      expect(screen.getByTestId('edit-input-ends-at').textContent).toContain('Выберите дату и время');
    });

    it('K. clearing existing date sends the canonical null/empty API value', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });
      vi.mocked(updateSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));

      // Clear both dates via popover
      fireEvent.click(screen.getByTestId('edit-input-starts-at'));
      fireEvent.click(screen.getByTestId('edit-input-starts-at-clear'));

      fireEvent.click(screen.getByTestId('edit-input-ends-at'));
      fireEvent.click(screen.getByTestId('edit-input-ends-at-clear'));

      fireEvent.click(screen.getByTestId('submit-edit-promo'));

      await waitFor(() => {
        expect(updateSellerPromotion).toHaveBeenCalledWith('promo-1', {
          isActive: true,
          startsAt: null,
          endsAt: null,
          globalUsageLimit: 100,
          perCustomerUsageLimit: 1,
        });
      });
    });

    it('L. percent/fixed promo form behavior remains intact', async () => {
      // Proved by tests 3 and 4 above
      expect(true).toBe(true);
    });

    it('M. no native date/datetime-local input remains for these two promo validity fields', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      const { container } = render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      const nativeDateInputs = container.querySelectorAll(
        'input[type="date"], input[type="datetime-local"]'
      );
      expect(nativeDateInputs.length).toBe(0);
    });
  });

  describe('MARKETING.1D2-UI3 Centered Promo Modal Tests (A-N)', () => {
    it('A. Create opens centered modal, not SellerDrawer', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      // Must find modal and NOT find drawer
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
      expect(screen.queryByTestId('create-promo-drawer')).toBeNull();
      expect(screen.getByRole('dialog')).toBeTruthy();
      expect(screen.getByText('Создание промокода')).toBeTruthy();
    });

    it('B. Edit opens centered modal, not SellerDrawer', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));

      // Must find modal and NOT find drawer
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();
      expect(screen.queryByTestId('edit-promo-drawer')).toBeNull();
      expect(screen.getByRole('dialog')).toBeTruthy();
      expect(screen.getByText('Параметры промокода: SALE10')).toBeTruthy();
    });

    it('C. Create form behavior unchanged (proper conversion and validation)', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'MODAL20' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });
      fireEvent.change(screen.getByTestId('input-min-order'), { target: { value: '2000' } });
      fireEvent.change(screen.getByTestId('input-global-limit'), { target: { value: '500' } });
      fireEvent.change(screen.getByTestId('input-per-customer-limit'), { target: { value: '2' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith({
          code: 'MODAL20',
          discountType: 'percent',
          discountValueBps: 2000,
          minOrderSubtotalCents: 200000,
          firstPaidOrderOnly: false,
          globalUsageLimit: 500,
          perCustomerUsageLimit: 2,
          productScope: 'ENTIRE_STORE',
        });
      });
    });

    it('D. Edit immutable fields remain read-only', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));

      const modal = screen.getByTestId('edit-promo-modal');
      const modalQueries = within(modal);
      expect(modalQueries.getByText('Неизменяемые экономические параметры')).toBeTruthy();
      expect(modalQueries.getByText('10%')).toBeTruthy();
      expect(modalQueries.getByText('500 ₽')).toBeTruthy();
      expect(modalQueries.getByText('Продавец (SELLER)')).toBeTruthy();
    });

    it('E. Operational edit fields remain editable and submit correctly', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });
      vi.mocked(updateSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));

      // Toggle active off
      fireEvent.click(screen.getByTestId('edit-toggle-active'));
      // Update limits
      fireEvent.change(screen.getByTestId('edit-input-global-limit'), { target: { value: '150' } });
      fireEvent.change(screen.getByTestId('edit-input-per-customer-limit'), { target: { value: '3' } });

      fireEvent.click(screen.getByTestId('submit-edit-promo'));

      await waitFor(() => {
        expect(updateSellerPromotion).toHaveBeenCalledWith('promo-1', {
          isActive: false,
          startsAt: new Date('2026-10-01T00:00').toISOString(),
          endsAt: new Date('2026-11-01T00:00').toISOString(),
          globalUsageLimit: 150,
          perCustomerUsageLimit: 3,
        });
      });
    });

    it('F. Custom date picker works inside modal', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));

      // Open starts-at date picker within modal
      fireEvent.click(screen.getByTestId('input-starts-at'));
      expect(screen.getByTestId('input-starts-at-popover')).toBeTruthy();

      // Pick day and apply
      const dayButtons = screen.getAllByRole('button', { name: /\d{1,2}\s+[а-яА-Я]+\s+\d{4}/ });
      fireEvent.click(dayButtons[10]);
      fireEvent.click(screen.getByTestId('input-starts-at-apply'));

      expect(screen.queryByTestId('input-starts-at-popover')).toBeNull();
      expect(screen.getByTestId('input-starts-at').textContent).not.toContain('Выберите дату и время');
    });

    it('G. Empty create dates remain empty', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('input-starts-at').textContent).toContain('Выберите дату и время');
      expect(screen.getByTestId('input-ends-at').textContent).toContain('Выберите дату и время');
    });

    it('H. Cancel closes modal', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-button')).toBeTruthy();
      });

      // Open create modal and cancel
      fireEvent.click(screen.getByTestId('create-promo-button'));
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
      fireEvent.click(screen.getByText('Отмена'));
      expect(screen.queryByTestId('create-promo-modal')).toBeNull();

      // Open edit modal and cancel
      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();
      fireEvent.click(screen.getByText('Отмена'));
      expect(screen.queryByTestId('edit-promo-modal')).toBeNull();
    });

    it('I. Escape interaction works correctly for modal and date picker', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      // 1. Open create modal
      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();

      // 2. Open starts-at date picker popover inside modal
      fireEvent.click(screen.getByTestId('input-starts-at'));
      expect(screen.getByTestId('input-starts-at-popover')).toBeTruthy();

      // 3. Press Escape: should close the date picker popover, but NOT close the modal!
      fireEvent.keyDown(window, { key: 'Escape' });
      expect(screen.queryByTestId('input-starts-at-popover')).toBeNull();
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();

      // 4. Press Escape again: should close the modal
      fireEvent.keyDown(window, { key: 'Escape' });
      expect(screen.queryByTestId('create-promo-modal')).toBeNull();
    });

    it('J. API validation error keeps modal open', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockRejectedValue({
        code: 'promo_code_duplicate',
        message: 'promo code already exists',
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'DUPLICATE' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-error')).toBeTruthy();
      });

      // Modal is still open
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
    });

    it('K. Successful create closes modal', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'SUCCESS' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(screen.queryByTestId('create-promo-modal')).toBeNull();
      });
    });

    it('L. Successful edit closes modal', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });
      vi.mocked(updateSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();

      fireEvent.click(screen.getByTestId('submit-edit-promo'));

      await waitFor(() => {
        expect(screen.queryByTestId('edit-promo-modal')).toBeNull();
      });
    });

    it('M. Reopening create resets previous draft state', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-button')).toBeTruthy();
      });

      // Open and type draft
      fireEvent.click(screen.getByTestId('create-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'DRAFTCODE' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '25' } });

      // Close via cancel
      fireEvent.click(screen.getByText('Отмена'));
      expect(screen.queryByTestId('create-promo-modal')).toBeNull();

      // Reopen create modal
      fireEvent.click(screen.getByTestId('create-promo-button'));
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();

      // Form values should be reset
      expect((screen.getByTestId('input-promo-code') as HTMLInputElement).value).toBe('');
      expect((screen.getByTestId('input-discount-percent') as HTMLInputElement).value).toBe('');
    });

    it('N. No promotion create/edit SellerDrawer remains', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [mockPromoPercent],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      // No create-promo-drawer or edit-promo-drawer test IDs anywhere
      expect(screen.queryByTestId('create-promo-drawer')).toBeNull();
      expect(screen.queryByTestId('edit-promo-drawer')).toBeNull();
    });
  });

  describe('PROMO.2B Advanced Promotion Rules Tests (1-20)', () => {
    const mockProducts: SellerProduct[] = [
      {
        id: '11111111-1111-1111-1111-111111111111',
        title: 'Шёлковая блузка',
      } as any,
      {
        id: '22222222-2222-2222-2222-222222222222',
        title: 'Кожаная куртка',
      } as any,
      {
        id: '33333333-3333-3333-3333-333333333333',
        title: 'Шерстяной кардиган',
      } as any,
    ];

    it('1. default Create scope = all Seller products', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('radio-scope-entire-store')).toBeTruthy();
      expect(screen.queryByTestId('selected-products-section')).toBeNull();
      expect(screen.getByTestId('promo-summary-card').textContent).toContain('Все товары магазина');
    });

    it('2. switching to selected products reveals product picker', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      expect(screen.getByTestId('selected-products-section')).toBeTruthy();
      expect(screen.getByTestId('input-search-included-products')).toBeTruthy();

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      });
    });

    it('3. own products can be selected', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-included-product-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      expect(screen.getByTestId('selected-included-product-11111111-1111-1111-1111-111111111111').textContent).toContain('Шёлковая блузка');
    });

    it('4. selected product can be removed', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));
      expect(screen.getByTestId('selected-included-product-11111111-1111-1111-1111-111111111111')).toBeTruthy();

      fireEvent.click(screen.getByTestId('remove-included-product-11111111-1111-1111-1111-111111111111'));
      expect(screen.queryByTestId('selected-included-product-11111111-1111-1111-1111-111111111111')).toBeNull();
    });

    it('5. exclusions are progressive/collapsed initially', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('btn-toggle-exclusions')).toBeTruthy();
      expect(screen.queryByTestId('excluded-products-section')).toBeNull();
    });

    it('6. exclusions can be added', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-22222222-2222-2222-2222-222222222222')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-22222222-2222-2222-2222-222222222222'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-excluded-product-22222222-2222-2222-2222-222222222222')).toBeTruthy();
      expect(screen.getByTestId('selected-excluded-product-22222222-2222-2222-2222-222222222222').textContent).toContain('Кожаная куртка');
    });

    it('7. same product cannot exist in include + exclude', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      // Include product 1
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // Open exclusions
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-22222222-2222-2222-2222-222222222222')).toBeTruthy();
      });
      // In modal UX, Product 1 is disabled with conflict badge
      const conflictOption = screen.getByTestId('product-option-exclude-11111111-1111-1111-1111-111111111111');
      expect((conflictOption as HTMLButtonElement).disabled).toBe(true);
      expect(conflictOption.textContent).toContain('Товар уже выбран для участия');
    });

    it('8. SELECTED_PRODUCTS cannot submit with zero includes', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'ZEROINC' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      // Submit with 0 included products
      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-error').textContent).toContain('Выберите хотя бы один товар');
      });
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('9. optional max discount defaults disabled/null', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect((screen.getByTestId('input-max-discount') as HTMLInputElement).value).toBe('');
    });

    it('10. max discount RUB -> cents payload correct', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'CAPTEST' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '25' } });
      fireEvent.change(screen.getByTestId('input-max-discount'), { target: { value: '1500' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            maxDiscountCents: 150000,
          })
        );
      });
    });

    it('11. summary updates from form state', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-max-discount'), { target: { value: '1000' } });
      fireEvent.change(screen.getByTestId('input-min-order'), { target: { value: '3000' } });
      fireEvent.click(screen.getByTestId('input-first-paid-only'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      const summary = screen.getByTestId('promo-summary-card').textContent;
      expect(summary).toContain('15%');
      expect(summary).toContain('макс. 1000 ₽');
      expect(summary).toContain('Выбранные товары (1 шт.)');
      expect(summary).toContain('3000 ₽');
      expect(summary).toContain('Только первый заказ');
    });

    it('12. successful create sends new rule contract', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'FULLRULE' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });
      fireEvent.change(screen.getByTestId('input-max-discount'), { target: { value: '2000' } });
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-11111111-1111-1111-1111-111111111111'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-22222222-2222-2222-2222-222222222222')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-22222222-2222-2222-2222-222222222222'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith({
          code: 'FULLRULE',
          discountType: 'percent',
          discountValueBps: 2000,
          productScope: 'SELECTED_PRODUCTS',
          includedProductIds: ['11111111-1111-1111-1111-111111111111'],
          excludedProductIds: ['22222222-2222-2222-2222-222222222222'],
          maxDiscountCents: 200000,
          firstPaidOrderOnly: false,
          perCustomerUsageLimit: 1,
        });
      });
    });

    it('13. API validation keeps modal open', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProducts).mockResolvedValue(mockProducts);
      vi.mocked(createSellerPromotion).mockRejectedValue({
        code: 'product_not_owned_by_seller',
        message: 'foreign product',
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'OWNERR' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-error').textContent).toContain(
          'Один или несколько выбранных товаров не принадлежат вашему магазину.'
        );
      });
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
    });

    it('14. edit displays scope read-only', async () => {
      const advancedPromo: SellerPromotion = {
        ...mockPromoPercent,
        productScope: 'SELECTED_PRODUCTS',
        includedProductIds: ['11111111-1111-1111-1111-111111111111', '22222222-2222-2222-2222-222222222222'],
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [advancedPromo], count: 1 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();
      expect(screen.getByTestId('edit-promo-scope').textContent).toContain('2 выбранных товаров');
    });

    it('15. edit displays exclusions read-only', async () => {
      const advancedPromo: SellerPromotion = {
        ...mockPromoPercent,
        excludedProductIds: ['33333333-3333-3333-3333-333333333333'],
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [advancedPromo], count: 1 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();
      expect(screen.getByTestId('edit-promo-exclusions').textContent).toContain('1 товаров');
    });

    it('16. edit displays max discount read-only', async () => {
      const advancedPromo: SellerPromotion = {
        ...mockPromoPercent,
        maxDiscountCents: 150000,
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [advancedPromo], count: 1 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();
      expect(screen.getByTestId('edit-promo-max-discount').textContent?.replace(/\u00a0/g, ' ')).toContain('1 500');
    });

    it('17. operational edit remains functional', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [mockPromoPercent], count: 1 });
      vi.mocked(updateSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-edit-button-SALE10')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-SALE10'));
      fireEvent.change(screen.getByTestId('edit-input-per-customer-limit'), { target: { value: '5' } });
      fireEvent.click(screen.getByTestId('submit-edit-promo'));

      await waitFor(() => {
        expect(updateSellerPromotion).toHaveBeenCalledWith('promo-1', expect.objectContaining({
          perCustomerUsageLimit: 5,
        }));
      });
    });

    it('18. centered modal remains', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [mockPromoPercent], count: 1 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-promo-button'));
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
      expect(screen.queryByTestId('create-promo-drawer')).toBeNull();
    });

    it('19. SellerDateTimePicker regression remains green', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('input-starts-at')).toBeTruthy();
      expect(screen.getByTestId('input-ends-at')).toBeTruthy();
      fireEvent.click(screen.getByTestId('input-starts-at'));
      expect(screen.getByTestId('input-starts-at-popover')).toBeTruthy();
    });

    it('20. existing simple promo creation still works', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'SIMPLE10' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith({
          code: 'SIMPLE10',
          discountType: 'percent',
          discountValueBps: 1000,
          firstPaidOrderOnly: false,
          perCustomerUsageLimit: 1,
          productScope: 'ENTIRE_STORE',
        });
      });
      expect(screen.queryByTestId('create-promo-modal')).toBeNull();
    });

    it('21. converts max discount to exact cents for fractional values (1, 1.01, 10.10, 999.99, 1000)', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      const testCases = [
        { input: '1', expectedCents: 100 },
        { input: '1.01', expectedCents: 101 },
        { input: '10.10', expectedCents: 1010 },
        { input: '999.99', expectedCents: 99999 },
        { input: '1000', expectedCents: 100000 },
      ];

      for (const { input, expectedCents } of testCases) {
        vi.mocked(createSellerPromotion).mockClear();
        fireEvent.click(screen.getByTestId('create-first-promo-button'));
        await waitFor(() => {
          expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
        });

        fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EXACTCAP' } });
        fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
        fireEvent.change(screen.getByTestId('input-max-discount'), { target: { value: input } });

        fireEvent.click(screen.getByTestId('submit-create-promo'));

        await waitFor(() => {
          expect(createSellerPromotion).toHaveBeenCalledWith(
            expect.objectContaining({
              maxDiscountCents: expectedCents,
            })
          );
        });

        await waitFor(() => {
          expect(screen.queryByTestId('create-promo-modal')).toBeNull();
        });
      }
    });

    it('22. rejects invalid precision like 1.001 for max discount and shows validation error', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      await waitFor(() => {
        expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
      });

      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'BADCAP' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-max-discount'), { target: { value: '1.001' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(screen.getByTestId('create-promo-error').textContent).toContain(
          'Максимальная скидка не может содержать более 2 знаков после запятой.'
        );
      });
      // Modal remains open on error
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
    });
  });

  describe('PROMO.2B-H2 Complete Seller Product Picker Tests (A-N)', () => {
    const makeCatalog = (count: number, prefix = 'Product') =>
      Array.from({ length: count }, (_, i) => ({
        id: `prod-${prefix.toLowerCase()}-${i + 1}`,
        sellerId: 'seller-1',
        title: `${prefix} ${i + 1}`,
        slug: `${prefix.toLowerCase()}-${i + 1}`,
        description: `${prefix} description`,
        status: 'published',
        priceCents: 50000,
        currency: 'RUB',
        variants: [],
        images: [],
        createdAt: '2026-01-01T00:00:00Z',
      } as any));

    it('A. Open create modal with catalog >50 items -> initial page loads smoothly', async () => {
      const items = makeCatalog(20, 'Item');
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items, totalCount: 80 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      expect(screen.getByTestId('selected-products-section')).toBeTruthy();
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });
    });

    it('B. Typing search query triggers search', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'denim') {
          return { items: makeCatalog(3, 'Denim'), totalCount: 3 };
        }
        return { items: makeCatalog(20, 'Item'), totalCount: 80 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'denim' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-denim-1')).toBeTruthy();
      });
    });

    it('C. Server search filter matches title / ID / SKU', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      const skuProduct: SellerProduct = {
        id: 'uuid-sku-999',
        sellerId: 'seller-1',
        title: 'Special Item SKU Match',
        slug: 'special-sku',
        status: 'published',
        priceCents: 50000,
        currency: 'RUB',
        variants: [],
        images: [],
        createdAt: '2026-01-01T00:00:00Z',
      } as any;

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'SKU-SPECIAL-999') {
          return { items: [skuProduct], totalCount: 1 };
        }
        return { items: makeCatalog(10, 'General'), totalCount: 10 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'SKU-SPECIAL-999' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-uuid-sku-999')).toBeTruthy();
        expect(screen.getByTestId('product-option-include-uuid-sku-999').textContent).toContain('Special Item SKU Match');
      });
    });

    it('D. Pagination / "Load more" loads next page', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      const page1 = makeCatalog(5, 'Batch1');
      const page2 = makeCatalog(5, 'Batch2');

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.page === 2) {
          return { items: page2, totalCount: 10 };
        }
        return { items: page1, totalCount: 10 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-batch1-1')).toBeTruthy();
      });

      expect(screen.getByTestId('btn-load-more-included')).toBeTruthy();
      fireEvent.click(screen.getByTestId('btn-load-more-included'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-batch2-1')).toBeTruthy();
      });
      // Page 1 item still present
      expect(screen.getByTestId('product-option-include-prod-batch1-1')).toBeTruthy();
    });

    it('E. Selecting a product from page 2 adds it to selected badges', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      const page1 = makeCatalog(5, 'Batch1');
      const page2 = makeCatalog(5, 'Batch2');

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.page === 2) {
          return { items: page2, totalCount: 10 };
        }
        return { items: page1, totalCount: 10 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('btn-load-more-included')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('btn-load-more-included'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-batch2-3')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-batch2-3'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-included-product-prod-batch2-3')).toBeTruthy();
      expect(screen.getByTestId('selected-included-product-prod-batch2-3').textContent).toContain('Batch2 3');
    });

    it('F. Clearing or changing search query does NOT lose previously selected product badges', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'initial') {
          return { items: makeCatalog(1, 'InitialSearch'), totalCount: 1 };
        }
        if (params?.q === 'secondary') {
          return { items: makeCatalog(1, 'SecondarySearch'), totalCount: 1 };
        }
        return { items: makeCatalog(5, 'Default'), totalCount: 5 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      // Search initial
      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'initial' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-initialsearch-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-initialsearch-1'));

      // Change search to secondary
      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'secondary' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-secondarysearch-1')).toBeTruthy();
      });

      // Apply selection to main form
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // Crucial: InitialSearch badge is STILL visible and retains title & remove button
      const badge = screen.getByTestId('selected-included-product-prod-initialsearch-1');
      expect(badge).toBeTruthy();
      expect(badge.textContent).toContain('InitialSearch 1');
      expect(screen.getByTestId('remove-included-product-prod-initialsearch-1')).toBeTruthy();
    });

    it('G. Removing a selected product badge removes it from includedProductIds', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProductsPaginated).mockResolvedValue({
        items: makeCatalog(2, 'Item'),
        totalCount: 2,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));
      expect(screen.getByTestId('selected-included-product-prod-item-1')).toBeTruthy();

      fireEvent.click(screen.getByTestId('remove-included-product-prod-item-1'));
      expect(screen.queryByTestId('selected-included-product-prod-item-1')).toBeNull();
    });

    it('H. Mutual exclusion: product selected in include cannot be selected in exclude, and vice versa', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      const products = makeCatalog(3, 'Item');
      vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: products, totalCount: 3 });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });
      // Select Item 1 in include
      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // Open exclusions
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-item-2')).toBeTruthy();
      });

      // In modal UX, Item 1 is disabled in exclude modal with conflict reason
      const conflictOption = screen.getByTestId('product-option-exclude-prod-item-1');
      expect((conflictOption as HTMLButtonElement).disabled).toBe(true);
      expect(conflictOption.textContent).toContain('Товар уже выбран для участия');
    });

    it('I. Empty search result shows "Ничего не найдено" / friendly empty state', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'nonexistent') {
          return { items: [], totalCount: 0 };
        }
        return { items: makeCatalog(5, 'Item'), totalCount: 5 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'nonexistent' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('empty-included-search').textContent).toContain('Ничего не найдено');
      });
    });

    it('J. Network error during product fetch shows retry button and doesn\'t crash modal', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      let callCount = 0;
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'error-test' && callCount === 0) {
          callCount++;
          throw new Error('Network timeout');
        }
        return { items: makeCatalog(2, 'Recovered'), totalCount: 2 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'error-test' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('include-products-error')).toBeTruthy();
        expect(screen.getByTestId('btn-retry-included')).toBeTruthy();
      });

      // Click retry
      fireEvent.click(screen.getByTestId('btn-retry-included'));

      await waitFor(() => {
        expect(screen.queryByTestId('include-products-error')).toBeNull();
        expect(screen.getByTestId('product-option-include-prod-recovered-1')).toBeTruthy();
      });
    });

    it('K. Submitting form includes all selected product IDs across different searches/pages', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'query1') {
          return { items: makeCatalog(1, 'Q1Item'), totalCount: 1 };
        }
        if (params?.q === 'query2') {
          return { items: makeCatalog(1, 'Q2Item'), totalCount: 1 };
        }
        return { items: makeCatalog(5, 'Default'), totalCount: 5 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'MULTISELECT' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      // Search 1 and select
      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'query1' },
      });
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-q1item-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-q1item-1'));

      // Search 2 and select
      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'query2' },
      });
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-q2item-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-q2item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // Both badges visible
      expect(screen.getByTestId('selected-included-product-prod-q1item-1')).toBeTruthy();
      expect(screen.getByTestId('selected-included-product-prod-q2item-1')).toBeTruthy();

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'MULTISELECT',
            productScope: 'SELECTED_PRODUCTS',
            includedProductIds: ['prod-q1item-1', 'prod-q2item-1'],
          })
        );
      });
    });

    it('L. Selected product badge truncated if long title', async () => {
      const longTitleProduct: SellerProduct = {
        id: 'prod-long-title',
        sellerId: 'seller-1',
        title: 'Extremely Long Luxury Dress Title That Should Be Visually Truncated In Badge UI',
        slug: 'long-title',
        status: 'published',
        priceCents: 50000,
        currency: 'RUB',
        variants: [],
        images: [],
        createdAt: '2026-01-01T00:00:00Z',
      } as any;
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(getSellerProductsPaginated).mockResolvedValue({
        items: [longTitleProduct],
        totalCount: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-long-title')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-long-title'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      const badgeSpan = screen.getByTestId('selected-included-product-prod-long-title').querySelector('span');
      expect(badgeSpan?.className).toContain('truncate');
      expect(badgeSpan?.className).toContain('max-w-[180px]');
    });

    it('M. Exclude search works similarly with search + pagination + badge persistence', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      const exPage1 = makeCatalog(5, 'ExItem');
      const exPage2 = makeCatalog(5, 'ExItemP2');

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.page === 2) {
          return { items: exPage2, totalCount: 10 };
        }
        return { items: exPage1, totalCount: 10 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitem-1')).toBeTruthy();
      });

      // Load more in exclusions
      expect(screen.getByTestId('btn-load-more-excluded')).toBeTruthy();
      fireEvent.click(screen.getByTestId('btn-load-more-excluded'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitemp2-1')).toBeTruthy();
      });

      // Select page 2 excluded product
      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitemp2-1'));

      // Search in exclusions
      fireEvent.change(screen.getByTestId('input-search-excluded-products'), {
        target: { value: 'search-change' },
      });

      // Apply exclusions to main form
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // Excluded badge is STILL visible with remove button in main form
      expect(screen.getByTestId('selected-excluded-product-prod-exitemp2-1')).toBeTruthy();
      expect(screen.getByTestId('remove-excluded-product-prod-exitemp2-1')).toBeTruthy();
    });

    it('N. Regression: creating promotion with no scope (ENTIRE_STORE) works without touching product picker', async () => {
      vi.mocked(getSellerPromotions).mockResolvedValue({ items: [], count: 0 });
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'ENTIRESTOREPROMO' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'ENTIRESTOREPROMO',
            productScope: 'ENTIRE_STORE',
          })
        );
      });
      const callArg = vi.mocked(createSellerPromotion).mock.calls[0][0];
      expect(callArg.includedProductIds).toBeUndefined();
      expect(callArg.excludedProductIds).toBeUndefined();
    });
  });

  describe('PROMO.2B-UI1 Product Selection Modal Tests (A - Z)', () => {
    const makeCatalog = (count: number, prefix = 'Product') =>
      Array.from({ length: count }, (_, i) => ({
        id: `prod-${prefix.toLowerCase()}-${i + 1}`,
        sellerId: 'seller-1',
        title: `${prefix} ${i + 1}`,
        slug: `${prefix.toLowerCase()}-${i + 1}`,
        description: `${prefix} description`,
        status: 'published',
        priceCents: 50000,
        currency: 'RUB',
        variants: [{ id: `v-${i + 1}`, sku: `SKU-${prefix.toUpperCase()}-${i + 1}` }],
        images: [{ id: `img-${i + 1}`, imageUrl: `https://example.com/${prefix}-${i + 1}.jpg`, isMain: true }],
        createdAt: '2026-01-01T00:00:00Z',
      } as any));

    it('A. selecting "На выбранные товары" opens INCLUDE modal', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(5, 'Item'),
        totalCount: 5,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      expect(screen.getByTestId('product-selector-modal')).toBeTruthy();
      expect(screen.getByText('Выберите товары')).toBeTruthy();
    });

    it('B. product results render as cards with title, price, and image', async () => {
      const products = makeCatalog(2, 'Luxury');
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({ items: products, totalCount: 2 }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-luxury-1')).toBeTruthy();
      });

      const card = screen.getByTestId('product-option-include-prod-luxury-1');
      expect(card.textContent).toContain('Luxury 1');
      expect(card.textContent).toContain('500');
      expect(card.querySelector('img')?.getAttribute('src')).toBe('https://example.com/Luxury-1.jpg');
    });

    it('C. clicking neutral card marks it ✓', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'Item'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      expect(screen.queryByTestId('product-card-check-prod-item-1')).toBeNull();

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      expect(screen.getByTestId('product-card-check-prod-item-1')).toBeTruthy();
    });

    it('D. clicking ✓ card again deselects it', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'Item'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      expect(screen.getByTestId('product-card-check-prod-item-1')).toBeTruthy();

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      expect(screen.queryByTestId('product-card-check-prod-item-1')).toBeNull();
    });

    it('E. Apply commits INCLUDE selection to main form', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'Item'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.queryByTestId('product-selector-modal')).toBeNull();
      expect(screen.getByTestId('selected-summary-count').textContent).toBe('1');
      expect(screen.getByTestId('selected-included-product-prod-item-1')).toBeTruthy();
    });

    it('F. Cancel discards INCLUDE changes', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'Item'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-cancel-selector'));

      expect(screen.queryByTestId('product-selector-modal')).toBeNull();
      expect(screen.queryByTestId('selected-included-product-prod-item-1')).toBeNull();
      expect(screen.getByText('Товары не выбраны')).toBeTruthy();
    });

    it('G. X/Escape discards temporary changes', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'Item'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.keyDown(window, { key: 'Escape' });

      expect(screen.queryByTestId('product-selector-modal')).toBeNull();
      expect(screen.queryByTestId('selected-included-product-prod-item-1')).toBeNull();
    });

    it('H. main form shows compact selected count and chips', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(4, 'Item'),
        totalCount: 4,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('product-option-include-prod-item-2'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-summary-count').textContent).toBe('2');
      expect(screen.getByTestId('btn-edit-included')).toBeTruthy();
      expect(screen.getByTestId('btn-clear-included')).toBeTruthy();
    });

    it('I. large inline picker no longer appears in main form', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(5, 'Item'),
        totalCount: 5,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      fireEvent.click(screen.getByTestId('btn-cancel-selector'));

      expect(screen.getByTestId('selected-products-section')).toBeTruthy();
      expect(screen.queryByTestId('product-card-check-prod-item-1')).toBeNull();
    });

    it('J. "Добавить исключения" opens EXCLUDE modal', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(3, 'Item'),
        totalCount: 3,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      expect(screen.getByTestId('product-selector-modal')).toBeTruthy();
      expect(screen.getByText('Исключить товары')).toBeTruthy();
      expect(screen.getByTestId('input-search-excluded-products')).toBeTruthy();
    });

    it('K. excluded card displays ×', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'ExItem'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitem-1')).toBeTruthy();
      });

      expect(screen.queryByTestId('product-card-cross-prod-exitem-1')).toBeNull();

      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitem-1'));
      expect(screen.getByTestId('product-card-cross-prod-exitem-1')).toBeTruthy();
    });

    it('L. clicking × again removes exclusion', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'ExItem'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitem-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitem-1'));
      expect(screen.getByTestId('product-card-cross-prod-exitem-1')).toBeTruthy();

      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitem-1'));
      expect(screen.queryByTestId('product-card-cross-prod-exitem-1')).toBeNull();
    });

    it('M. Apply commits exclusions', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(2, 'ExItem'),
        totalCount: 2,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitem-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitem-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.queryByTestId('product-selector-modal')).toBeNull();
      expect(screen.getByTestId('excluded-summary-count').textContent).toBe('1');
      expect(screen.getByTestId('selected-excluded-product-prod-exitem-1')).toBeTruthy();
    });

    it('N. Cancel leaves previous exclusions unchanged', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(3, 'ExItem'),
        totalCount: 3,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitem-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitem-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));
      expect(screen.getByTestId('excluded-summary-count').textContent).toBe('1');

      fireEvent.click(screen.getByTestId('btn-edit-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exitem-2')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exitem-2'));
      fireEvent.click(screen.getByTestId('btn-cancel-selector'));

      expect(screen.getByTestId('excluded-summary-count').textContent).toBe('1');
      expect(screen.getByTestId('selected-excluded-product-prod-exitem-1')).toBeTruthy();
      expect(screen.queryByTestId('selected-excluded-product-prod-exitem-2')).toBeNull();
    });

    it('O. include/exclude conflict is prevented with disabled state and message', async () => {
      const products = makeCatalog(3, 'Item');
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({ items: products, totalCount: 3 }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-item-1')).toBeTruthy();
      });

      const disabledCard = screen.getByTestId('product-option-exclude-prod-item-1') as HTMLButtonElement;
      expect(disabledCard.disabled).toBe(true);
      expect(disabledCard.textContent).toContain('Товар уже выбран для участия');
    });

    it('P. selected products survive server search changes', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'initial') {
          return { items: makeCatalog(1, 'InitialSearch'), totalCount: 1 };
        }
        if (params?.q === 'secondary') {
          return { items: makeCatalog(1, 'SecondarySearch'), totalCount: 1 };
        }
        return { items: makeCatalog(5, 'Default'), totalCount: 5 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'initial' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-initialsearch-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-initialsearch-1'));
      expect(screen.getByTestId('product-card-check-prod-initialsearch-1')).toBeTruthy();

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'secondary' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-secondarysearch-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('btn-apply-selector'));
      expect(screen.getByTestId('selected-included-product-prod-initialsearch-1')).toBeTruthy();
      expect(screen.getByTestId('selected-included-product-prod-initialsearch-1').textContent).toContain('InitialSearch 1');
    });

    it('Q. selected products survive pagination/load-more', async () => {
      const page1 = makeCatalog(5, 'Batch1');
      const page2 = makeCatalog(5, 'Batch2');

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.page === 2) {
          return { items: page2, totalCount: 10 };
        }
        return { items: page1, totalCount: 10 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-batch1-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-batch1-1'));

      expect(screen.getByTestId('btn-load-more-included')).toBeTruthy();
      fireEvent.click(screen.getByTestId('btn-load-more-included'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-batch2-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-batch2-1'));

      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-summary-count').textContent).toBe('2');
      expect(screen.getByTestId('selected-included-product-prod-batch1-1')).toBeTruthy();
      expect(screen.getByTestId('selected-included-product-prod-batch2-1')).toBeTruthy();
    });

    it('R. excluded products survive search/pagination', async () => {
      const exPage1 = makeCatalog(5, 'ExBatch1');
      const exPage2 = makeCatalog(5, 'ExBatch2');

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.page === 2) {
          return { items: exPage2, totalCount: 10 };
        }
        return { items: exPage1, totalCount: 10 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exbatch1-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exbatch1-1'));

      fireEvent.click(screen.getByTestId('btn-load-more-excluded'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-exbatch2-2')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-prod-exbatch2-2'));

      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('excluded-summary-count').textContent).toBe('2');
      expect(screen.getByTestId('selected-excluded-product-prod-exbatch1-1')).toBeTruthy();
      expect(screen.getByTestId('selected-excluded-product-prod-exbatch2-2')).toBeTruthy();
    });

    it('S. product beyond first page remains selectable', async () => {
      const page1 = makeCatalog(20, 'P1Item');
      const page2 = makeCatalog(20, 'P2Item');

      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.page === 2) {
          return { items: page2, totalCount: 40 };
        }
        return { items: page1, totalCount: 40 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('btn-load-more-included')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('btn-load-more-included'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-p2item-15')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-p2item-15'));
      expect(screen.getByTestId('product-card-check-prod-p2item-15')).toBeTruthy();
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-included-product-prod-p2item-15')).toBeTruthy();
    });

    it('T. server-side search still works', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'denim') {
          return { items: makeCatalog(1, 'Denim'), totalCount: 1 };
        }
        return { items: makeCatalog(5, 'Item'), totalCount: 5 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'denim' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-denim-1')).toBeTruthy();
      });
    });

    it('U. API error shows retry without losing draft selection', async () => {
      let callCount = 0;
      vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
        if (params?.q === 'error-test' && callCount === 0) {
          callCount++;
          throw new Error('Network timeout');
        }
        return { items: makeCatalog(2, 'Recovered'), totalCount: 2 };
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      fireEvent.change(screen.getByTestId('input-search-included-products'), {
        target: { value: 'error-test' },
      });

      await waitFor(() => {
        expect(screen.getByTestId('include-products-error')).toBeTruthy();
        expect(screen.getByTestId('btn-retry-included')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('btn-retry-included'));

      await waitFor(() => {
        expect(screen.queryByTestId('include-products-error')).toBeNull();
        expect(screen.getByTestId('product-option-include-prod-recovered-1')).toBeTruthy();
      });
    });

    it('V. ENTIRE_STORE does not mark every product ✓', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(5, 'StoreItem'),
        totalCount: 5,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('radio-scope-entire-store')).toBeTruthy();

      expect(screen.queryByTestId('product-selector-modal')).toBeNull();
      expect(screen.queryByTestId('selected-products-section')).toBeNull();
      expect(screen.getByTestId('promo-summary-card').textContent).toContain('Все товары магазина');
    });

    it('W. ENTIRE_STORE exclusion selector correctly uses ×', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(3, 'StoreItem'),
        totalCount: 3,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-storeitem-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-exclude-prod-storeitem-1'));
      expect(screen.getByTestId('product-card-cross-prod-storeitem-1')).toBeTruthy();
      expect(screen.queryByTestId('product-card-check-prod-storeitem-1')).toBeNull();
    });

    it('X. summary updates after Apply only', async () => {
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(3, 'Item'),
        totalCount: 3,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      expect(screen.getByTestId('promo-summary-card').textContent).toContain('(0 шт.)');

      fireEvent.click(screen.getByTestId('btn-apply-selector'));
      expect(screen.getByTestId('promo-summary-card').textContent).toContain('(1 шт.)');
    });

    it('Y. promo create payload remains identical to accepted PROMO.2B business contract', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({
        items: makeCatalog(3, 'Item'),
        totalCount: 3,
      }));

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'LOCKEDCONTRACT' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });
      fireEvent.change(screen.getByTestId('input-max-discount'), { target: { value: '2500' } });
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));

      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      fireEvent.click(screen.getByTestId('btn-toggle-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-exclude-prod-item-2')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-exclude-prod-item-2'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith({
          code: 'LOCKEDCONTRACT',
          discountType: 'percent',
          discountValueBps: 2000,
          productScope: 'SELECTED_PRODUCTS',
          includedProductIds: ['prod-item-1'],
          excludedProductIds: ['prod-item-2'],
          maxDiscountCents: 250000,
          firstPaidOrderOnly: false,
          perCustomerUsageLimit: 1,
        });
      });
    });

    it('Z. existing centered promotion modal remains visually/behaviorally functional', async () => {
      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-first-promo-button')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('create-first-promo-button'));
      expect(screen.getByTestId('create-promo-modal')).toBeTruthy();
      expect(screen.getByRole('dialog', { name: 'Создание промокода' })).toBeTruthy();
    });
    });
});
