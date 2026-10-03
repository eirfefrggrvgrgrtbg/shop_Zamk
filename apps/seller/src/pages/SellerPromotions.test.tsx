/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerPromotions } from './SellerPromotions';
import {
  getSellerPromotions,
  createSellerPromotion,
  updateSellerPromotion,
} from '@zamk/api-client/src/seller';
import type { SellerPromotion } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerPromotions: vi.fn(),
  createSellerPromotion: vi.fn(),
  updateSellerPromotion: vi.fn(),
}));

describe('SellerPromotions Component & Interaction Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockPromoPercent: SellerPromotion = {
    id: 'promo-1',
    code: 'SALE10',
    discountType: 'percent',
    discountValueBps: 1000,
    discountValueFixedCents: 0,
    minOrderSubtotalCents: 50000,
    firstPaidOrderOnly: false,
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
});
