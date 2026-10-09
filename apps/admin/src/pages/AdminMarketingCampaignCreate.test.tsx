import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminMarketingCampaignCreate } from './AdminMarketingCampaignCreate';
import * as adminApi from '@zamk/api-client/src/admin';
import type { AdminSeller } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/admin');

const mockSellers: AdminSeller[] = [
  {
    id: '33333333-3333-4333-8333-333333333333',
    ownerName: 'Иван Иванов',
    ownerEmail: 'ivan@seller.ru',
    brandName: 'YUNIS Fashion',
    storeName: 'YUNIS Store',
    status: 'active',
  } as unknown as AdminSeller,
  {
    id: '44444444-4444-4444-8444-444444444444',
    ownerName: 'Петр Петров',
    ownerEmail: 'petr@seller.ru',
    brandName: 'Nordic Wool',
    storeName: 'Nordic Store',
    status: 'active',
  } as unknown as AdminSeller,
];

describe('AdminMarketingCampaignCreate', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(adminApi.getAdminSellers).mockResolvedValue({
      items: mockSellers,
      total: mockSellers.length,
      page: 1,
      pageSize: 100,
    } as any);
  });

  const mount = () =>
    render(
      <MemoryRouter initialEntries={['/marketing/campaigns/new']}>
        <Routes>
          <Route path="/marketing/campaigns/new" element={<AdminMarketingCampaignCreate />} />
          <Route path="/marketing/campaigns" element={<div>Список кампаний</div>} />
          <Route path="/marketing/campaigns/:id" element={<div>Детали кампании созданы</div>} />
        </Routes>
      </MemoryRouter>
    );

  it('Q: ZAMK owner create does not require seller and submits fundingMode zamk with sellerId null', async () => {
    vi.mocked(adminApi.createAdminCampaign).mockResolvedValue({
      id: 'new-camp-123',
      title: 'Spring Promo',
    } as any);

    mount();

    fireEvent.change(screen.getByTestId('campaign-title-input'), {
      target: { value: 'Spring Promo' },
    });
    fireEvent.change(screen.getByTestId('campaign-budget-input'), {
      target: { value: '50000' },
    });

    fireEvent.click(screen.getByTestId('submit-create-campaign'));

    await waitFor(() => {
      expect(adminApi.createAdminCampaign).toHaveBeenCalledWith(
        expect.objectContaining({
          purpose: 'advertising',
          title: 'Spring Promo',
          fundingMode: 'zamk',
          sellerId: null,
          plannedBudgetCents: 5000000,
          discountType: 'percent',
        })
      );
    });

    await screen.findByText('Детали кампании созданы');
  });

  it('R & S: Seller owner requires seller, picker displays human-readable info without UUID, and submits fundingMode seller', async () => {
    vi.mocked(adminApi.createAdminCampaign).mockResolvedValue({
      id: 'seller-camp-456',
      title: 'YUNIS Drop',
    } as any);

    mount();

    fireEvent.change(screen.getByTestId('campaign-title-input'), {
      target: { value: 'YUNIS Drop' },
    });

    // Select seller owner radio
    fireEvent.click(screen.getByTestId('campaign-owner-seller'));

    // Attempt to submit without picking seller -> validation error
    fireEvent.click(screen.getByTestId('submit-create-campaign'));
    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.getByText('Выберите продавца')).toBeTruthy();
    expect(adminApi.createAdminCampaign).not.toHaveBeenCalled();

    // Seller search input
    const searchInput = screen.getByTestId('seller-search-input');
    fireEvent.focus(searchInput);

    // Verify human-readable brand and email are displayed, and raw UUID is NOT displayed
    await screen.findByText('YUNIS Fashion');
    expect(screen.getByText('ivan@seller.ru')).toBeTruthy();
    expect(screen.queryByText('33333333-3333-4333-8333-333333333333')).toBeNull();

    // Pick the seller
    fireEvent.click(screen.getByTestId(`seller-option-${mockSellers[0].id}`));

    // Submit now
    fireEvent.click(screen.getByTestId('submit-create-campaign'));

    await waitFor(() => {
      expect(adminApi.createAdminCampaign).toHaveBeenCalledWith(
        expect.objectContaining({
          purpose: 'advertising',
          title: 'YUNIS Drop',
          fundingMode: 'seller',
          sellerId: '33333333-3333-4333-8333-333333333333',
          discountType: 'percent',
        })
      );
    });
  });

  it('validates empty title and date ranges before submit', async () => {
    mount();
    // Submit with empty title
    fireEvent.click(screen.getByTestId('submit-create-campaign'));
    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.getByText('Укажите название кампании')).toBeTruthy();
    expect(adminApi.createAdminCampaign).not.toHaveBeenCalled();

    // Set title and invalid dates (ends before starts)
    fireEvent.change(screen.getByTestId('campaign-title-input'), {
      target: { value: 'Invalid Dates Campaign' },
    });
    fireEvent.change(screen.getByLabelText('Дата начала'), {
      target: { value: '2026-06-30' },
    });
    fireEvent.change(screen.getByLabelText('Дата окончания'), {
      target: { value: '2026-06-01' },
    });

    fireEvent.click(screen.getByTestId('submit-create-campaign'));
    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.getByText('Дата окончания не может быть раньше даты начала')).toBeTruthy();
    expect(adminApi.createAdminCampaign).not.toHaveBeenCalled();
  });

  it('T: no purpose selector exists in visible UI and human advertising context is shown', () => {
    mount();
    expect(screen.queryByLabelText(/цель кампании|назначение|purpose/i)).toBeNull();
    expect(screen.queryByText('promotion')).toBeNull();
    // Context label "Рекламная кампания" is shown
    expect(screen.getAllByText('Рекламная кампания').length).toBeGreaterThan(0);
    // No option to choose promo-campaign
    expect(screen.queryByRole('option', { name: /промо-кампания/i })).toBeNull();
  });

  it('U: no ROAS or actual spend fields in live summary', () => {
    mount();
    expect(screen.queryByText(/roas/i)).toBeNull();
    expect(screen.queryByText(/фактический расход|actual spend/i)).toBeNull();
  });

  it('V: budget input uses text/numeric inputMode and does not use raw type="number"', () => {
    mount();
    const budgetInput = screen.getByTestId('campaign-budget-input');
    expect(budgetInput.getAttribute('type')).not.toBe('number');
    expect(budgetInput.getAttribute('inputmode')).toBe('numeric');
  });

  it('has breadcrumb and cancel back links to /marketing/campaigns', () => {
    mount();
    const backLinks = screen.getAllByRole('link', { name: /кампании|отмена/i });
    expect(backLinks.some(l => l.getAttribute('href') === '/marketing/campaigns')).toBe(true);
  });

  it('W: create payload matches canonical API contract with date ranges and description', async () => {
    vi.mocked(adminApi.createAdminCampaign).mockResolvedValue({
      id: 'full-camp-789',
      title: 'Influencer Launch',
    } as any);

    mount();

    fireEvent.change(screen.getByTestId('campaign-title-input'), {
      target: { value: 'Influencer Launch' },
    });
    fireEvent.change(screen.getByLabelText('Канал'), {
      target: { value: 'influencer' },
    });
    fireEvent.change(screen.getByLabelText('Тип кампании'), {
      target: { value: 'drop' },
    });
    fireEvent.change(screen.getByTestId('campaign-budget-input'), {
      target: { value: '120000' },
    });
    fireEvent.change(screen.getByLabelText('Дата начала'), {
      target: { value: '2026-06-01' },
    });
    fireEvent.change(screen.getByLabelText('Дата окончания'), {
      target: { value: '2026-06-30' },
    });
    fireEvent.change(screen.getByPlaceholderText('Дополнительное описание кампании...'), {
      target: { value: 'Exclusive summer drop' },
    });

    fireEvent.click(screen.getByTestId('submit-create-campaign'));

    await waitFor(() => {
      expect(adminApi.createAdminCampaign).toHaveBeenCalledWith(
        expect.objectContaining({
          purpose: 'advertising',
          title: 'Influencer Launch',
          fundingMode: 'zamk',
          sellerId: null,
          campaignChannel: 'influencer',
          campaignType: 'drop',
          plannedBudgetCents: 12000000,
          startsAt: '2026-06-01T00:00:00.000Z',
          endsAt: '2026-06-30T00:00:00.000Z',
          description: 'Exclusive summer drop',
          discountType: 'percent',
        })
      );
    });
  });
});
