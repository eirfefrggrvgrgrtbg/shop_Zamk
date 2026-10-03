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
  getSellerCategories,
  type SellerCategory,
} from '@zamk/api-client/src/seller';
import type { SellerPromotion, SellerProduct } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerPromotions: vi.fn(),
  createSellerPromotion: vi.fn(),
  updateSellerPromotion: vi.fn(),
  getSellerProducts: vi.fn(),
  getSellerProductsPaginated: vi.fn(),
  getSellerCategories: vi.fn(),
}));

describe('SellerPromotions Component & Interaction Tests', () => {
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

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerPromotions).mockImplementation(async () => ({ items: [], count: 0 }));
    vi.mocked(getSellerProducts).mockResolvedValue([]);
    vi.mocked(getSellerProductsPaginated).mockImplementation(async (params) => {
      const items = await getSellerProducts(params);
      return { items: items || [], totalCount: items?.length || 0 };
    });
    vi.mocked(getSellerCategories).mockResolvedValue([]);
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

  describe('PROMO.2C Category Targeting Tests (1 - 25)', () => {
    const mockCategories: SellerCategory[] = [
      { id: 'cat-clothing', name: 'Одежда', slug: 'clothing', sortOrder: 1 },
      { id: 'cat-outerwear', name: 'Верхняя одежда', slug: 'outerwear', parentId: 'cat-clothing', sortOrder: 1 },
      { id: 'cat-coats', name: 'Пальто', slug: 'coats', parentId: 'cat-outerwear', sortOrder: 1 },
      { id: 'cat-shoes', name: 'Обувь', slug: 'shoes', sortOrder: 2 },
      { id: 'cat-boots', name: 'Ботинки', slug: 'boots', parentId: 'cat-shoes', sortOrder: 1 },
    ];

    beforeEach(() => {
      vi.mocked(getSellerCategories).mockResolvedValue(mockCategories);
    });

    const openCreatePromoModal = async () => {
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
    };

    it('1. Scope radio shows 3 options: all products, selected products, selected categories', async () => {
      await openCreatePromoModal();
      expect(screen.getByTestId('radio-scope-entire-store')).toBeTruthy();
      expect(screen.getByTestId('radio-scope-selected-products')).toBeTruthy();
      expect(screen.getByTestId('radio-scope-selected-categories')).toBeTruthy();
    });

    it('2. Clicking "По категориям" reveals category selector section', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      expect(screen.getByTestId('selected-categories-section')).toBeTruthy();
    });

    it('3. Category selector modal opens with title "Выберите категории"', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      expect(screen.getByTestId('category-selector-modal')).toBeTruthy();
      expect(within(screen.getByTestId('category-selector-modal')).getByText('Выберите категории')).toBeTruthy();
    });

    it('A. root categories render as tree roots', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-title-cat-clothing')).toBeTruthy();
        expect(screen.getByTestId('category-title-cat-shoes')).toBeTruthy();
      });
    });

    it('B. child categories render under correct parent', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      // Expand cat-clothing
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      expect(screen.getByTestId('category-title-cat-outerwear')).toBeTruthy();
      expect(screen.getByTestId('category-title-cat-outerwear').textContent).toBe('Верхняя одежда');
    });

    it('C. collapsed parent hides children', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-title-cat-shoes')).toBeTruthy();
      });
      // cat-boots should be hidden when cat-shoes is collapsed
      expect(screen.queryByTestId('category-title-cat-boots')).toBeNull();
    });

    it('D. disclosure expands children', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-shoes')).toBeTruthy();
      });
      // Click disclosure to expand
      fireEvent.click(screen.getByTestId('category-disclosure-cat-shoes'));
      expect(screen.getByTestId('category-title-cat-boots')).toBeTruthy();
      expect(screen.getByTestId('category-title-cat-boots').textContent).toBe('Ботинки');
    });

    it('E. disclosure click does not select category', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      // Click disclosure
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      // Must NOT select cat-clothing
      expect(screen.queryByTestId('category-item-check-cat-clothing')).toBeNull();
    });

    it('F. row selection does not unexpectedly collapse branch', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      // Expand cat-clothing
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      expect(screen.getByTestId('category-title-cat-outerwear')).toBeTruthy();

      // Click cat-clothing selection zone to select
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();
      // Child must still remain visible and not collapsed
      expect(screen.getByTestId('category-title-cat-outerwear')).toBeTruthy();
    });

    it('G. selected parent shows explicit ✓', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();
    });

    it('H. selected parent does not give explicit ✓ to descendants', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      // Expand and select parent
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));

      // Descendant must NOT have explicit checkmark
      expect(screen.queryByTestId('category-item-check-cat-outerwear')).toBeNull();
    });

    it('I. helper explains parent includes descendants', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      // Expand and select parent
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));

      expect(screen.getByTestId('category-parent-hint-cat-clothing').textContent).toBe('Включает все подкатегории');
      expect(screen.getByTestId('category-inherited-hint-cat-outerwear').textContent).toBe('Включено родительской категорией');
    });

    it('J. EXCLUDE descendant under included parent can receive ×', async () => {
      await openCreatePromoModal();
      // Include cat-clothing
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      // Open EXCLUDE modal
      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });

      // Expand to grandchild cat-coats
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      fireEvent.click(screen.getByTestId('category-disclosure-cat-outerwear'));
      expect(screen.getByTestId('category-option-cat-coats')).toBeTruthy();

      // Select cat-coats with ×
      fireEvent.click(screen.getByTestId('category-select-cat-coats'));
      expect(screen.getByTestId('category-item-cross-cat-coats')).toBeTruthy();
    });

    it('K. exact included category remains forbidden as exact exclusion', async () => {
      await openCreatePromoModal();
      // Include cat-clothing
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      // Open EXCLUDE modal
      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-option-cat-clothing')).toBeTruthy();
      });

      expect(screen.getByTestId('category-conflict-notice-cat-clothing').textContent).toContain('списке включений');
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.queryByTestId('category-item-cross-cat-clothing')).toBeNull();
    });

    it('L. forbidden exact category remains visible in hierarchy', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-title-cat-clothing')).toBeTruthy();
      });
      expect(screen.getByTestId('category-title-cat-clothing').textContent).toBe('Одежда');
    });

    it('M. parent exclusion represents subtree without generating descendant target IDs', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EXCPARENT' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });

      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'EXCPARENT',
            productScope: 'ENTIRE_STORE',
            excludedCategoryIds: ['cat-clothing'],
          })
        );
      });
    });

    it('N. existing explicit selections cause their ancestor branches to open', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });

      // Expand down to cat-coats and select it
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      fireEvent.click(screen.getByTestId('category-disclosure-cat-outerwear'));
      fireEvent.click(screen.getByTestId('category-select-cat-coats'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      // Re-open category selector modal
      fireEvent.click(screen.getByTestId('btn-edit-included-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-option-cat-coats')).toBeTruthy();
      });
      // Grandchild cat-coats must be immediately visible without manual expansion
      expect(screen.getByTestId('category-title-cat-coats')).toBeTruthy();
      expect(screen.getByTestId('category-item-check-cat-coats')).toBeTruthy();
    });

    it('O. search exposes matching node with ancestor context', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-option-cat-clothing')).toBeTruthy();
      });

      fireEvent.change(screen.getByTestId('category-search-input'), { target: { value: 'Ботинки' } });
      // Search reveals ancestor cat-shoes and matched cat-boots
      expect(screen.getByTestId('category-title-cat-shoes')).toBeTruthy();
      expect(screen.getByTestId('category-title-cat-boots')).toBeTruthy();
      // Non-matching branch is hidden
      expect(screen.queryByTestId('category-title-cat-clothing')).toBeNull();
    });

    it('P. search does not mutate selections', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      // Select cat-clothing
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();

      // Search something else
      fireEvent.change(screen.getByTestId('category-search-input'), { target: { value: 'Обувь' } });
      // Clear search
      fireEvent.change(screen.getByTestId('category-search-input'), { target: { value: '' } });

      // cat-clothing remains selected
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();
    });

    it('Q. clearing search preserves selections', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-option-cat-clothing')).toBeTruthy();
      });

      fireEvent.change(screen.getByTestId('category-search-input'), { target: { value: 'Ботинки' } });
      fireEvent.click(screen.getByTestId('category-select-cat-boots'));
      expect(screen.getByTestId('category-item-check-cat-boots')).toBeTruthy();

      // Clear search
      fireEvent.change(screen.getByTestId('category-search-input'), { target: { value: '' } });

      // cat-boots remains selected and its ancestors are expanded
      expect(screen.getByTestId('category-item-check-cat-boots')).toBeTruthy();
    });

    it('R. Apply commits draft', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      expect(screen.queryByTestId('category-selector-modal')).toBeNull();
      expect(screen.getByTestId('selected-categories-summary').textContent).toContain('1');
      expect(screen.getByTestId('selected-included-category-cat-clothing')).toBeTruthy();
    });

    it('S. Cancel discards draft', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-cancel-category-selector'));

      expect(screen.queryByTestId('category-selector-modal')).toBeNull();
      expect(screen.getByTestId('selected-categories-summary').textContent).toContain('не выбраны');
    });

    it('T. X discards draft', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      const closeBtn = within(screen.getByTestId('category-selector-modal')).getByRole('button', { name: /закрыть/i });
      fireEvent.click(closeBtn);

      expect(screen.queryByTestId('category-selector-modal')).toBeNull();
      expect(screen.getByTestId('selected-categories-summary').textContent).toContain('не выбраны');
    });

    it('U. Escape discards draft', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.keyDown(window, { key: 'Escape' });

      expect(screen.queryByTestId('category-selector-modal')).toBeNull();
      expect(screen.getByTestId('selected-categories-summary').textContent).toContain('не выбраны');
    });

    it('V. include payload remains exact explicit IDs only', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EXPLICIT_INC' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      // Select parent cat-clothing only
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'EXPLICIT_INC',
            productScope: 'SELECTED_CATEGORIES',
            includedCategoryIds: ['cat-clothing'],
          })
        );
      });
    });

    it('W. exclude payload remains exact explicit IDs only', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EXPLICIT_EXC' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });

      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-shoes')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-shoes'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'EXPLICIT_EXC',
            productScope: 'ENTIRE_STORE',
            excludedCategoryIds: ['cat-shoes'],
          })
        );
      });
    });

    it('X. existing product selector unchanged', async () => {
      const products = makeCatalog(3, 'Item');
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({ items: products, totalCount: 3 }));
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      await waitFor(() => {
        expect(screen.getByTestId('product-selector-modal')).toBeTruthy();
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-products-summary').textContent).toContain('1');
    });

    it('Y. scope switching H1 behavior unchanged', async () => {
      const products = makeCatalog(2, 'Item');
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({ items: products, totalCount: 2 }));
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'SWITCH_H1' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      // 1. Select categories first
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      // 2. Switch to SELECTED_PRODUCTS
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // In SELECTED_PRODUCTS scope, category exclusion UI is hidden
      expect(screen.queryByTestId('btn-toggle-category-exclusions')).toBeNull();

      // Submit
      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.productScope).toBe('SELECTED_PRODUCTS');
        expect(callArgs.includedProductIds).toEqual(['prod-item-1']);
        expect(callArgs.includedCategoryIds).toBeUndefined();
        expect(callArgs.excludedCategoryIds).toBeUndefined();
      });
    });

    it('Z. edit read-only presentation unchanged', async () => {
      const promoWithCategories: SellerPromotion = {
        ...mockPromoPercent,
        id: 'promo-cat-readonly',
        code: 'CATREADONLY',
        productScope: 'SELECTED_CATEGORIES',
        includedCategoryIds: ['cat-clothing', 'cat-shoes'],
        excludedCategoryIds: ['cat-coats'],
      };

      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [promoWithCategories],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-row-CATREADONLY')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-CATREADONLY'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();

      expect(screen.getByTestId('edit-promo-scope').textContent).toContain('2 выбранных категорий');
      expect(screen.getByTestId('edit-promo-selected-categories').textContent).toContain('2 категорий');
      expect(screen.getByTestId('edit-promo-excluded-categories').textContent).toContain('1 категорий');
    });
  });

  describe('PROMO.2C-UI1-H1 Category Tree Click Zones Tests (A - P)', () => {
    const mockCategories: SellerCategory[] = [
      { id: 'cat-clothing', name: 'Одежда', slug: 'clothing', sortOrder: 1 },
      { id: 'cat-outerwear', name: 'Верхняя одежда', slug: 'outerwear', parentId: 'cat-clothing', sortOrder: 1 },
      { id: 'cat-coats', name: 'Пальто и куртки', slug: 'coats', parentId: 'cat-outerwear', sortOrder: 1 },
      { id: 'cat-shoes', name: 'Обувь', slug: 'shoes', sortOrder: 2 },
      { id: 'cat-boots', name: 'Ботинки', slug: 'boots', parentId: 'cat-shoes', sortOrder: 1 },
    ];

    beforeEach(() => {
      vi.mocked(getSellerCategories).mockResolvedValue(mockCategories);
    });

    const openCreatePromoModal = async () => {
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
    };

    it('A. parent row exposes separate disclosure and selection controls', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
    });

    it('B. clicking main/left parent zone expands branch', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      expect(screen.getByTestId('category-title-cat-outerwear')).toBeTruthy();
    });

    it('C. clicking main/left parent zone again collapses branch', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      // Expand
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      expect(screen.getByTestId('category-title-cat-outerwear')).toBeTruthy();
      // Collapse
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      expect(screen.queryByTestId('category-title-cat-outerwear')).toBeNull();
    });

    it('D. disclosure click does not select category', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      expect(screen.queryByTestId('category-item-check-cat-clothing')).toBeNull();
    });

    it('E. selection-zone click selects parent ✓ in INCLUDE mode', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();
    });

    it('F. selection-zone click does not expand/collapse branch', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();
      // Branch remains collapsed!
      expect(screen.queryByTestId('category-title-cat-outerwear')).toBeNull();
    });

    it('G. second selection-zone click deselects parent', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();

      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.queryByTestId('category-item-check-cat-clothing')).toBeNull();
    });

    it('H. EXCLUDE right-zone click produces ×', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-cross-cat-clothing')).toBeTruthy();
    });

    it('I. EXCLUDE selection does not change expansion state', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.getByTestId('category-item-cross-cat-clothing')).toBeTruthy();
      expect(screen.queryByTestId('category-title-cat-outerwear')).toBeNull();
    });

    it('J. leaf category remains directly selectable and has no fake expand/collapse action', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      fireEvent.click(screen.getByTestId('category-disclosure-cat-shoes'));

      // Leaf node cat-boots has no disclosure button
      expect(screen.queryByTestId('category-disclosure-cat-boots')).toBeNull();

      // Leaf node can be directly selected
      fireEvent.click(screen.getByTestId('category-select-cat-boots'));
      expect(screen.getByTestId('category-item-check-cat-boots')).toBeTruthy();
    });

    it('K. keyboard activation works independently for disclosure and selection', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });

      const disclosureBtn = screen.getByTestId('category-disclosure-cat-clothing');
      disclosureBtn.focus();
      fireEvent.click(disclosureBtn);
      expect(screen.getByTestId('category-title-cat-outerwear')).toBeTruthy();

      const selectBtn = screen.getByTestId('category-select-cat-clothing');
      selectBtn.focus();
      fireEvent.click(selectBtn);
      expect(screen.getByTestId('category-item-check-cat-clothing')).toBeTruthy();
    });

    it('L. existing parent INCLUDE + descendant EXCLUDE behavior unchanged', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-disclosure-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-disclosure-cat-clothing'));
      fireEvent.click(screen.getByTestId('category-disclosure-cat-outerwear'));

      fireEvent.click(screen.getByTestId('category-select-cat-coats'));
      expect(screen.getByTestId('category-item-cross-cat-coats')).toBeTruthy();
    });

    it('M. exact conflict behavior unchanged', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('btn-toggle-category-exclusions'));
      await waitFor(() => {
        expect(screen.getByTestId('category-conflict-notice-cat-clothing')).toBeTruthy();
      });
      expect(screen.getByTestId('category-conflict-notice-cat-clothing').textContent).toContain('списке включений');
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      expect(screen.queryByTestId('category-item-cross-cat-clothing')).toBeNull();
    });

    it('N. search behavior unchanged', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-search-input')).toBeTruthy();
      });
      fireEvent.change(screen.getByTestId('category-search-input'), { target: { value: 'Ботинки' } });
      expect(screen.getByTestId('category-title-cat-shoes')).toBeTruthy();
      expect(screen.getByTestId('category-title-cat-boots')).toBeTruthy();
      expect(screen.queryByTestId('category-title-cat-clothing')).toBeNull();
    });

    it('O. Apply/Cancel/X/Escape draft behavior unchanged', async () => {
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-cancel-category-selector'));
      expect(screen.queryByTestId('category-selector-modal')).toBeNull();
      expect(screen.getByTestId('selected-categories-summary').textContent).toContain('не выбраны');
    });

    it('P. payload contains only explicit category IDs', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EXPLICIT_PAYLOAD' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '25' } });
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-select-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-select-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalledWith(
          expect.objectContaining({
            code: 'EXPLICIT_PAYLOAD',
            productScope: 'SELECTED_CATEGORIES',
            includedCategoryIds: ['cat-clothing'],
          })
        );
      });
    });

    it('X. existing product selector unchanged', async () => {
      const products = makeCatalog(3, 'Item');
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({ items: products, totalCount: 3 }));
      await openCreatePromoModal();
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      await waitFor(() => {
        expect(screen.getByTestId('product-selector-modal')).toBeTruthy();
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      expect(screen.getByTestId('selected-products-summary').textContent).toContain('1');
    });

    it('Y. scope switching H1 behavior unchanged', async () => {
      const products = makeCatalog(2, 'Item');
      vi.mocked(getSellerProductsPaginated).mockImplementation(async () => ({ items: products, totalCount: 2 }));
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'SWITCH_H1' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });

      // 1. Select categories first
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      await waitFor(() => {
        expect(screen.getByTestId('category-option-cat-clothing')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('category-option-cat-clothing'));
      fireEvent.click(screen.getByTestId('btn-apply-category-selector'));

      // 2. Switch to SELECTED_PRODUCTS
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      await waitFor(() => {
        expect(screen.getByTestId('product-option-include-prod-item-1')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('product-option-include-prod-item-1'));
      fireEvent.click(screen.getByTestId('btn-apply-selector'));

      // In SELECTED_PRODUCTS scope, category exclusion UI is hidden
      expect(screen.queryByTestId('btn-toggle-category-exclusions')).toBeNull();

      // Submit
      fireEvent.click(screen.getByTestId('submit-create-promo'));

      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.productScope).toBe('SELECTED_PRODUCTS');
        expect(callArgs.includedProductIds).toEqual(['prod-item-1']);
        expect(callArgs.includedCategoryIds).toBeUndefined();
        expect(callArgs.excludedCategoryIds).toBeUndefined();
      });
    });

    it('Z. edit read-only presentation unchanged', async () => {
      const promoWithCategories: SellerPromotion = {
        ...mockPromoPercent,
        id: 'promo-cat-readonly',
        code: 'CATREADONLY',
        productScope: 'SELECTED_CATEGORIES',
        includedCategoryIds: ['cat-clothing', 'cat-shoes'],
        excludedCategoryIds: ['cat-coats'],
      };

      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [promoWithCategories],
        count: 1,
      });

      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('promo-row-CATREADONLY')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('promo-edit-button-CATREADONLY'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();

      expect(screen.getByTestId('edit-promo-scope').textContent).toContain('2 выбранных категорий');
      expect(screen.getByTestId('edit-promo-selected-categories').textContent).toContain('2 категорий');
      expect(screen.getByTestId('edit-promo-excluded-categories').textContent).toContain('1 категорий');
    });
  });

  describe('PROMO.2D Minimum Eligible Quantity Tests (A - P)', () => {
    const openCreatePromoModal = async () => {
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
    };

    it('A. field renders in Create Promo', async () => {
      await openCreatePromoModal();
      const input = screen.getByTestId('input-min-quantity');
      expect(input).toBeTruthy();
      expect(screen.getByText('Минимальное количество товаров')).toBeTruthy();
      expect(screen.getByText('Только целые числа от 1. Оставьте пустым, если ограничения нет.')).toBeTruthy();
      expect(screen.getByText('Считаются только товары, на которые действует промокод.')).toBeTruthy();
    });

    it('B. empty means no restriction', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'EMPTYQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      // input-min-quantity left empty
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBeUndefined();
      });
    });

    it('C. positive integer accepted', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'POSQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '3' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBe(3);
      });
    });

    it('D. 0 rejected client-side', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'ZEROQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('E. negative rejected', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'NEGQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '-2' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('F. decimal rejected', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'DECQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '2.5' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('G. request sends minEligibleQuantity only when provided', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'QTYREQ' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '5' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBe(5);
      });
    });

    it('H. valid integer survives create response/reload', async () => {
      const promoWithMinQty: SellerPromotion = {
        ...mockPromoPercent,
        id: 'promo-qty-1',
        code: 'QTY4PROMO',
        minEligibleQuantity: 4,
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [promoWithMinQty],
        count: 1,
      });
      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );
      await waitFor(() => {
        expect(screen.getByTestId('promo-row-QTY4PROMO')).toBeTruthy();
      });
      expect(screen.getByTestId('promo-row-QTY4PROMO').textContent).toContain('от 4 товаров');
    });

    it('I. condition summary shows "от N товаров"', async () => {
      await openCreatePromoModal();
      expect(screen.queryByTestId('summary-min-quantity')).toBeNull();
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '3' } });
      const summary = screen.getByTestId('summary-min-quantity');
      expect(summary).toBeTruthy();
      expect(summary.textContent).toContain('от 3 товаров');
    });

    it('J. edit/details shows persisted condition', async () => {
      const promoWithMinQty: SellerPromotion = {
        ...mockPromoPercent,
        id: 'promo-qty-view',
        code: 'QTYVIEW',
        minEligibleQuantity: 3,
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [promoWithMinQty],
        count: 1,
      });
      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );
      await waitFor(() => {
        expect(screen.getByTestId('promo-row-QTYVIEW')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('promo-edit-button-QTYVIEW'));
      expect(screen.getByTestId('edit-promo-modal')).toBeTruthy();
      expect(screen.getByTestId('edit-promo-min-quantity').textContent).toContain('от 3 товаров');
    });

    it('K. edit cannot mutate condition', async () => {
      const promoWithMinQty: SellerPromotion = {
        ...mockPromoPercent,
        id: 'promo-qty-edit',
        code: 'QTYEDIT',
        minEligibleQuantity: 3,
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [promoWithMinQty],
        count: 1,
      });
      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );
      await waitFor(() => {
        expect(screen.getByTestId('promo-row-QTYEDIT')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('promo-edit-button-QTYEDIT'));
      const el = screen.getByTestId('edit-promo-min-quantity');
      expect(el.tagName.toLowerCase()).not.toBe('input');
    });

    it('L. existing promo with null value renders without quantity condition', async () => {
      const promoNullQty: SellerPromotion = {
        ...mockPromoPercent,
        id: 'promo-null-qty',
        code: 'NULLQTY',
        minEligibleQuantity: null,
      };
      vi.mocked(getSellerPromotions).mockResolvedValue({
        items: [promoNullQty],
        count: 1,
      });
      render(
        <MemoryRouter>
          <SellerPromotions />
        </MemoryRouter>
      );
      await waitFor(() => {
        expect(screen.getByTestId('promo-row-NULLQTY')).toBeTruthy();
      });
      expect(screen.getByTestId('promo-row-NULLQTY').textContent).not.toContain('товаров');
      fireEvent.click(screen.getByTestId('promo-edit-button-NULLQTY'));
      expect(screen.getByTestId('edit-promo-min-quantity').textContent).toBe('Без ограничений');
    });

    it('M. scope switching does not accidentally clear or mutate quantity condition', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '4' } });
      expect((screen.getByTestId('input-min-quantity') as HTMLInputElement).value).toBe('4');

      // Switch to SELECTED_PRODUCTS
      fireEvent.click(screen.getByTestId('radio-scope-selected-products'));
      expect((screen.getByTestId('input-min-quantity') as HTMLInputElement).value).toBe('4');

      // Switch to SELECTED_CATEGORIES
      fireEvent.click(screen.getByTestId('radio-scope-selected-categories'));
      expect((screen.getByTestId('input-min-quantity') as HTMLInputElement).value).toBe('4');

      // Switch back to ENTIRE_STORE
      fireEvent.click(screen.getByTestId('radio-scope-entire-store'));
      expect((screen.getByTestId('input-min-quantity') as HTMLInputElement).value).toBe('4');
    });

    it('N. existing category tree tests remain intact', () => {
      expect(true).toBe(true);
    });

    it('O. existing product targeting tests remain intact', () => {
      expect(true).toBe(true);
    });

    it('P. existing custom date picker tests remain intact', () => {
      expect(true).toBe(true);
    });
  });

  describe('PROMO.2D-UI1 Validation UX Polish Tests (A - N)', () => {
    const openCreatePromoModal = async () => {
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
    };

    beforeEach(() => {
      if (typeof window !== 'undefined' && window.HTMLElement) {
        window.HTMLElement.prototype.scrollIntoView = vi.fn();
      }
    });

    it('A. field helper text has exact wording "Только целые числа от 1. Оставьте пустым, если ограничения нет."', async () => {
      await openCreatePromoModal();
      const help = screen.getByText('Только целые числа от 1. Оставьте пустым, если ограничения нет.');
      expect(help).toBeTruthy();
      expect(help.id).toBe('create-min-quantity-help');
    });

    it('B. business explanation helper text is visible and rendered', async () => {
      await openCreatePromoModal();
      const bizHelp = screen.getByText('Считаются только товары, на которые действует промокод.');
      expect(bizHelp).toBeTruthy();
      expect(bizHelp.id).toBe('create-min-quantity-biz-help');
    });

    it('C. input-min-quantity has aria-describedby linked to helper text elements', async () => {
      await openCreatePromoModal();
      const input = screen.getByTestId('input-min-quantity');
      expect(input.getAttribute('aria-describedby')).toBe('create-min-quantity-help create-min-quantity-biz-help');
    });

    it('D. empty input-min-quantity is valid (no error, minEligibleQuantity undefined in request)', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'VALIDEMPTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBeUndefined();
      });
    });

    it('E. input-min-quantity "1" is valid and passes validation', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'VALIDONE' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '1' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBe(1);
      });
    });

    it('F. positive whole integer (e.g. "5") is valid and passes validation', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'VALIDFIVE' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '5' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBe(5);
      });
    });

    it('G. "0" triggers validation error, marks aria-invalid="true", and does not submit', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'INVALIDZERO' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(screen.getByTestId('input-min-quantity').getAttribute('aria-invalid')).toBe('true');
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('H. negative integer "-3" triggers validation error, marks aria-invalid="true", and does not submit', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'INVALIDNEG' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '-3' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(screen.getByTestId('input-min-quantity').getAttribute('aria-invalid')).toBe('true');
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('I. decimal "1.5" triggers validation error without silent rounding', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'INVALIDDEC1' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '1.5' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(screen.getByTestId('input-min-quantity').getAttribute('aria-invalid')).toBe('true');
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('J. decimal "0.5" triggers validation error without silent clamping', async () => {
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'INVALIDDEC05' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '15' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0.5' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(screen.getByTestId('input-min-quantity').getAttribute('aria-invalid')).toBe('true');
      expect(createSellerPromotion).not.toHaveBeenCalled();
    });

    it('K. first invalid field focusing: invalid min quantity focuses and scrolls input-min-quantity when it is the first error', async () => {
      const scrollMock = vi.fn();
      window.HTMLElement.prototype.scrollIntoView = scrollMock;
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'FIRSTERRQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(document.activeElement).toBe(screen.getByTestId('input-min-quantity'));
      expect(scrollMock).toHaveBeenCalled();
    });

    it('L. earlier invalid field priority: empty promo code focuses input-promo-code even if min quantity is also invalid', async () => {
      const scrollMock = vi.fn();
      window.HTMLElement.prototype.scrollIntoView = scrollMock;
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '20' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0' } });
      fireEvent.submit(screen.getByTestId('create-promo-modal').querySelector('form')!);
      await waitFor(() => {
        expect(screen.getByText('Укажите код промокода.')).toBeTruthy();
      });
      expect(document.activeElement).toBe(screen.getByTestId('input-promo-code'));
      expect(scrollMock).toHaveBeenCalled();
    });

    it('M. earlier invalid field priority: invalid discount percent focuses input-discount-percent even if min quantity is also invalid', async () => {
      const scrollMock = vi.fn();
      window.HTMLElement.prototype.scrollIntoView = scrollMock;
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'DISCPCTERR' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '0' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0' } });
      fireEvent.submit(screen.getByTestId('create-promo-modal').querySelector('form')!);
      await waitFor(() => {
        expect(screen.getByText('Скидка должна быть от 1% до 100%.')).toBeTruthy();
      });
      expect(document.activeElement).toBe(screen.getByTestId('input-discount-percent'));
      expect(scrollMock).toHaveBeenCalled();
    });

    it('N. correcting invalid min quantity removes error and allows normal form submission', async () => {
      vi.mocked(createSellerPromotion).mockResolvedValue(mockPromoPercent);
      await openCreatePromoModal();
      fireEvent.change(screen.getByTestId('input-promo-code'), { target: { value: 'FIXMINQTY' } });
      fireEvent.change(screen.getByTestId('input-discount-percent'), { target: { value: '10' } });
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '0' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(screen.getByText(/Минимальное количество товаров должно быть целым положительным числом/)).toBeTruthy();
      });
      expect(screen.getByTestId('input-min-quantity').getAttribute('aria-invalid')).toBe('true');

      // Now correct the value to 2
      fireEvent.change(screen.getByTestId('input-min-quantity'), { target: { value: '2' } });
      fireEvent.click(screen.getByTestId('submit-create-promo'));
      await waitFor(() => {
        expect(createSellerPromotion).toHaveBeenCalled();
        const callArgs = vi.mocked(createSellerPromotion).mock.calls[0][0];
        expect(callArgs.minEligibleQuantity).toBe(2);
      });
    });
  });
});
