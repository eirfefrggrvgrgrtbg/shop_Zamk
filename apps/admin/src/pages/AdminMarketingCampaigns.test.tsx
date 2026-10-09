import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminMarketingCampaigns } from './AdminMarketingCampaigns';
import { AdminMarketingCampaignDetail } from './AdminMarketingCampaignDetail';
import { AdminProtectedRoute } from '../components/AdminProtectedRoute';
import * as adminApi from '@zamk/api-client/src/admin';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import type { AdminCampaign, AdminCampaignTrackingLink } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/admin');
vi.mock('../contexts/AdminAuthContext');

const mockCampaigns: AdminCampaign[] = [
  {
    id: '11111111-1111-4111-8111-111111111111',
    purpose: 'advertising',
    sellerId: null,
    title: 'Platform Summer Sale 2026',
    description: 'Special summer platform campaign',
    fundingMode: 'zamk',
    status: 'active',
    campaignChannel: 'telegram',
    campaignType: 'seasonal_sale',
    plannedBudgetCents: 5000000,
    discountType: 'percent',
    startsAt: '2026-06-01T00:00:00Z',
    endsAt: '2026-08-31T23:59:59Z',
    createdAt: '2026-05-15T10:00:00Z',
    updatedAt: '2026-05-15T10:00:00Z',
    trackingLinkCount: 2,
  },
  {
    id: '22222222-2222-4222-8222-222222222222',
    purpose: 'advertising',
    sellerId: '33333333-3333-4333-8333-333333333333',
    title: 'Seller Brand Drop',
    description: 'Exclusive influencer drop for seller',
    fundingMode: 'seller',
    status: 'draft',
    campaignChannel: 'influencer',
    campaignType: 'drop',
    plannedBudgetCents: 2000000,
    discountType: 'percent',
    startsAt: '2026-07-01T00:00:00Z',
    endsAt: '2026-07-15T23:59:59Z',
    createdAt: '2026-06-20T10:00:00Z',
    updatedAt: '2026-06-20T10:00:00Z',
    trackingLinkCount: 0,
  },
  {
    id: '33333333-3333-4333-8333-333333333333',
    purpose: 'promotion',
    sellerId: '44444444-4444-4444-8444-444444444444',
    title: 'Autumn Promo Discount Campaign',
    description: 'Seller co-funded promo discount',
    fundingMode: 'cofunded',
    status: 'active',
    campaignChannel: 'vk',
    campaignType: 'seasonal_sale',
    plannedBudgetCents: 1500000,
    discountType: 'percent',
    startsAt: '2026-09-01T00:00:00Z',
    endsAt: '2026-09-30T23:59:59Z',
    createdAt: '2026-08-20T10:00:00Z',
    updatedAt: '2026-08-20T10:00:00Z',
    trackingLinkCount: 1,
  },
];

const mockTrackingLinks: AdminCampaignTrackingLink[] = [
  {
    id: 'aaaa1111-1111-4111-8111-111111111111',
    campaignId: '11111111-1111-4111-8111-111111111111',
    token: 'test_token_landing_abc123',
    targetType: 'landing',
    landingPath: '/catalog/sale',
    isActive: true,
    createdAt: '2026-05-16T12:00:00Z',
    updatedAt: '2026-05-16T12:00:00Z',
  },
  {
    id: 'bbbb2222-2222-4222-8222-222222222222',
    campaignId: '11111111-1111-4111-8111-111111111111',
    token: 'test_token_disabled_xyz789',
    targetType: 'product',
    targetProductId: '55555555-5555-4555-8555-555555555555',
    isActive: false,
    createdAt: '2026-05-17T12:00:00Z',
    updatedAt: '2026-05-18T12:00:00Z',
  },
];

function createMockAuth(permissions: string[]) {
  const permSet = new Set(permissions);
  return {
    user: { id: 'admin-user-id', email: 'admin@zamk.ru', role: 'admin' },
    staff: { userId: 'admin-user-id', roleCode: 'admin', permissions, status: 'active' },
    permissions,
    isAuthenticated: true,
    isLoading: false,
    error: null,
    hasPermission: (p: string) => permSet.has(p),
    hasAnyPermission: (perms: string[]) => perms.some((p) => permSet.has(p)),
    isOwner: () => false,
    isCoOwner: () => false,
    login: vi.fn(),
    logout: vi.fn(),
    refreshSession: vi.fn(),
    changePassword: vi.fn(),
  };
}

describe('Admin Marketing Operational Workspace (ADS.2A / C2.4)', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', ''); };
    HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); };
    vi.mocked(adminApi.getAdminMarketingCampaignMetrics).mockResolvedValue({ campaigns: [] });
    vi.mocked(adminApi.getAdminCampaigns).mockResolvedValue(mockCampaigns);
    vi.mocked(adminApi.getAdminCampaign).mockResolvedValue(mockCampaigns[0]);
    vi.mocked(adminApi.getAdminCampaignTrackingLinks).mockResolvedValue(mockTrackingLinks);
  });

  afterEach(() => {
    cleanup();
  });

  describe('Campaign List (AdminMarketingCampaigns)', () => {
    it('read-only user can view campaigns list but cannot see create button', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Platform Summer Sale 2026')).toBeTruthy();
      });

      expect(screen.getByText('Platform Summer Sale 2026')).toBeTruthy();
      expect(screen.getByText('Seller Brand Drop')).toBeTruthy();
      expect(screen.getByText('Autumn Promo Discount Campaign')).toBeTruthy();
      expect(screen.getByText('ZAMK Платформа')).toBeTruthy();
      expect(screen.getAllByText('Продавец').length).toBeGreaterThan(0);
      expect(screen.queryByTestId('create-campaign-button')).toBeNull();
    });

    it('write user can see create button and navigate to new campaign page', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.write']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
            <Route path="/marketing/campaigns/new" element={<div>Новая кампания страница</div>} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-campaign-button')).toBeTruthy();
      });

      const btn = screen.getByTestId('create-campaign-button');
      expect(btn.getAttribute('href')).toBe('/marketing/campaigns/new');
      fireEvent.click(btn);
      expect(screen.getByText('Новая кампания страница')).toBeTruthy();
    });

    it('renders human labels for both advertising and promotion without raw purpose enums', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByText('Platform Summer Sale 2026');
      // Advertising human label
      expect(screen.getAllByText('Рекламная кампания').length).toBeGreaterThan(0);
      // Promotion human label
      expect(screen.getAllByText('Промо-кампания').length).toBeGreaterThan(0);

      // Raw UUIDs and raw lowercase purpose strings are NOT rendered as unformatted text
      expect(screen.queryByText('11111111-1111-4111-8111-111111111111')).toBeNull();
    });

    it('renders formatted compact date ranges for campaigns', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByText('Platform Summer Sale 2026');
      const row = screen.getByTestId(`campaign-row-${mockCampaigns[0].id}`);
      // Formatted date in Russian locale
      expect(within(row).getByText(/01\.06\.26/)).toBeTruthy();
    });

    it('handles long campaign names safely without breaking table structure', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);
      const longTitle = 'Очень длинное название специальной маркетинговой кампании с промокодами и блогерами сезона 2026 года для тестирования переносов';
      vi.mocked(adminApi.getAdminCampaigns).mockResolvedValue([
        { ...mockCampaigns[0], title: longTitle },
      ]);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByText(longTitle);
      expect(screen.getByText(longTitle)).toBeTruthy();
    });

    it('filters compose: search, purpose, status, channel, and type', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByText('Platform Summer Sale 2026');

      // 1. Filter by search
      fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'seller' } });
      expect(screen.queryByText('Platform Summer Sale 2026')).toBeNull();
      expect(screen.getByText('Seller Brand Drop')).toBeTruthy();

      // 2. Filter by purpose
      fireEvent.change(screen.getByRole('searchbox'), { target: { value: '' } });
      fireEvent.change(screen.getByLabelText('Назначение'), { target: { value: 'promotion' } });
      expect(screen.queryByText('Platform Summer Sale 2026')).toBeNull();
      expect(screen.getByText('Autumn Promo Discount Campaign')).toBeTruthy();

      // 3. Status filter with no results
      fireEvent.change(screen.getByLabelText('Статус'), { target: { value: 'draft' } });
      expect(screen.getByText('Ничего не найдено')).toBeTruthy();
    });

    it('row click navigates to campaign detail; ignores text selection', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
            <Route path="/marketing/campaigns/:id" element={<div>Открыта кампания</div>} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByText('Platform Summer Sale 2026');
      const row = screen.getByTestId(`campaign-row-${mockCampaigns[0].id}`);

      // Ignored during selection
      const selectionSpy = vi.spyOn(window, 'getSelection').mockReturnValue({
        toString: () => 'highlighted text',
      } as any);
      fireEvent.click(row);
      expect(screen.queryByText('Открыта кампания')).toBeNull();
      selectionSpy.mockRestore();

      // Standard click navigates
      fireEvent.click(row);
      expect(screen.getByText('Открыта кампания')).toBeTruthy();
    });

    it('zero state shows informative empty message and create button for write users', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.write']) as any);
      vi.mocked(adminApi.getAdminCampaigns).mockResolvedValue([]);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByText('Кампаний пока нет');
      expect(screen.getByRole('heading', { name: 'Кампании' })).toBeTruthy();
      expect(screen.getByRole('searchbox')).toBeTruthy();
      expect(screen.getAllByTestId('create-campaign-button').length).toBeGreaterThan(0);
    });

    it('shows recoverable error on network failure', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);
      vi.mocked(adminApi.getAdminCampaigns).mockRejectedValueOnce(new Error('offline'));

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route path="/marketing/campaigns" element={<AdminMarketingCampaigns />} />
          </Routes>
        </MemoryRouter>
      );

      await screen.findByRole('alert');
      expect(screen.getByText('offline')).toBeTruthy();
      fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
      await screen.findByText('Platform Summer Sale 2026');
    });
  });

  describe('Campaign Detail & Tracking Links (AdminMarketingCampaignDetail)', () => {
    it('read-only user can view detail with human read-only purpose and tracking links', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns/11111111-1111-4111-8111-111111111111']}>
          <Routes>
            <Route path="/marketing/campaigns/:id" element={<AdminMarketingCampaignDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('admin-marketing-campaign-detail-page')).toBeTruthy();
      });

      expect(screen.getByTestId('campaign-title').textContent).toContain('Platform Summer Sale 2026');
      expect(screen.getByTestId('campaign-purpose-badge').textContent).toBe('Рекламная кампания');
      expect(screen.getByText('test_token_landing_abc123')).toBeTruthy();
      expect(screen.getByText('/catalog/sale')).toBeTruthy();

      // Read-only user cannot create or disable links or change status
      expect(screen.queryByTestId('create-tracking-link-button')).toBeNull();
      expect(screen.queryByTestId('disable-link-aaaa1111-1111-4111-8111-111111111111')).toBeNull();
      expect(screen.queryByTestId('pause-campaign-button')).toBeNull();
    });

    it('write user can create tracking link and disable active tracking link', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['marketing.campaigns.write']) as any);
      vi.mocked(adminApi.createAdminTrackingLink).mockResolvedValue({
        id: 'new-link-id',
        campaignId: '11111111-1111-4111-8111-111111111111',
        token: 'new_token_123',
        targetType: 'landing',
        landingPath: '/catalog/summer',
        isActive: true,
        createdAt: '2026-05-19T10:00:00Z',
        updatedAt: '2026-05-19T10:00:00Z',
      });
      vi.mocked(adminApi.disableAdminTrackingLink).mockResolvedValue({
        ...mockTrackingLinks[0],
        isActive: false,
      });

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns/11111111-1111-4111-8111-111111111111']}>
          <Routes>
            <Route path="/marketing/campaigns/:id" element={<AdminMarketingCampaignDetail />} />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('create-tracking-link-button')).toBeTruthy();
      });

      // 1. Create link
      fireEvent.click(screen.getByTestId('create-tracking-link-button'));
      expect(screen.getByTestId('create-tracking-link-modal')).toBeTruthy();

      fireEvent.change(screen.getByTestId('link-landing-path-input'), {
        target: { value: '/catalog/summer' },
      });
      fireEvent.click(screen.getByTestId('submit-create-link'));

      await waitFor(() => {
        expect(adminApi.createAdminTrackingLink).toHaveBeenCalledWith(
          '11111111-1111-4111-8111-111111111111',
          expect.objectContaining({
            targetType: 'landing',
            landingPath: '/catalog/summer',
          })
        );
      });

      // 2. Disable link
      const disableBtn = screen.getByTestId('disable-link-aaaa1111-1111-4111-8111-111111111111');
      expect(disableBtn).toBeTruthy();

      vi.spyOn(window, 'confirm').mockReturnValue(true);
      fireEvent.click(disableBtn);

      await waitFor(() => {
        expect(adminApi.disableAdminTrackingLink).toHaveBeenCalledWith(
          '11111111-1111-4111-8111-111111111111',
          'aaaa1111-1111-4111-8111-111111111111'
        );
      });
    });
  });

  describe('Route Protection', () => {
    it('blocks user without marketing permissions', async () => {
      vi.mocked(useAdminAuth).mockReturnValue(createMockAuth(['orders.read', 'inventory.read']) as any);

      render(
        <MemoryRouter initialEntries={['/marketing/campaigns']}>
          <Routes>
            <Route
              path="/marketing/campaigns"
              element={
                <AdminProtectedRoute permission={['marketing.campaigns.read', 'marketing.campaigns.write']}>
                  <AdminMarketingCampaigns />
                </AdminProtectedRoute>
              }
            />
          </Routes>
        </MemoryRouter>
      );

      expect(screen.getByText('Недостаточно прав')).toBeTruthy();
      expect(screen.queryByTestId('admin-marketing-campaigns-page')).toBeNull();
    });
  });
});
