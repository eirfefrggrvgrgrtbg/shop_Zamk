import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminMarketingCampaignDetail } from './AdminMarketingCampaignDetail';
import * as adminApi from '@zamk/api-client/src/admin';
import { useAdminAuth } from '../contexts/AdminAuthContext';
import type { AdminCampaign } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/admin');
vi.mock('../contexts/AdminAuthContext');

const ID = '11111111-1111-4111-8111-111111111111';

const base = (over: Partial<AdminCampaign> = {}): AdminCampaign => ({
  id: ID,
  purpose: 'advertising',
  sellerId: null,
  title: 'Summer',
  description: 'desc',
  fundingMode: 'zamk',
  status: 'approved',
  campaignChannel: 'telegram',
  campaignType: 'seasonal_sale',
  plannedBudgetCents: 5000000,
  discountType: 'percent',
  startsAt: '2026-06-01T00:00:00Z',
  endsAt: '2026-08-31T23:59:59Z',
  createdAt: '2026-05-15T10:00:00Z',
  updatedAt: '2026-05-15T10:00:00Z',
  trackingLinkCount: 0,
  ...over,
} as AdminCampaign);

function auth(perms: string[]) {
  const set = new Set(perms);
  return {
    hasPermission: (p: string) => set.has(p),
    hasAnyPermission: (ps: string[]) => ps.some(p => set.has(p)),
  } as any;
}

const mount = () =>
  render(
    <MemoryRouter initialEntries={[`/marketing/campaigns/${ID}`]}>
      <Routes>
        <Route path="/marketing/campaigns/:id" element={<AdminMarketingCampaignDetail />} />
      </Routes>
    </MemoryRouter>
  );

describe('AdminMarketingCampaignDetail (M1.1)', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(useAdminAuth).mockReturnValue(auth(['marketing.campaigns.read', 'marketing.campaigns.write']));
    vi.mocked(adminApi.getAdminCampaign).mockResolvedValue(base());
    vi.mocked(adminApi.getAdminCampaignTrackingLinks).mockResolvedValue([]);
    vi.mocked(adminApi.getAdminMarketingCampaignMetrics).mockResolvedValue({ campaigns: [] });
  });
  afterEach(() => cleanup());

  it('shows real 30-day metrics for the campaign and zeros instead of invented values when there is no traffic', async () => {
    vi.mocked(adminApi.getAdminMarketingCampaignMetrics).mockResolvedValue({
      campaigns: [{ campaignId: ID, name: 'Summer', visits: 200, paidOrders: 5, revenueCents: 1250000, conversionRateBps: 250 }],
    });
    mount();
    await waitFor(() => expect(screen.getByTestId('metric-visits').textContent).toBe('200'));
    expect(screen.getByTestId('metric-orders').textContent).toBe('5');
    expect(screen.getByTestId('metric-conversion').textContent).toBe('2.50%');
    expect(screen.getByTestId('campaign-metrics').textContent).toContain('ROAS не рассчитывается');
  });

  it('shows metrics as unavailable when the metrics API fails, keeping the campaign visible', async () => {
    vi.mocked(adminApi.getAdminMarketingCampaignMetrics).mockRejectedValue(new Error('boom'));
    mount();
    await waitFor(() => expect(screen.getByTestId('metrics-unavailable')).toBeTruthy());
    expect(screen.getByTestId('campaign-title').textContent).toBe('Summer');
  });

  it('offers only real status transitions: no pause, activate only when approved, end with confirmation', async () => {
    vi.mocked(adminApi.updateAdminCampaign).mockResolvedValue(base({ status: 'ended' }));
    const confirm = vi.spyOn(window, 'confirm');
    mount();
    await waitFor(() => expect(screen.getByTestId('end-campaign-button')).toBeTruthy());
    expect(screen.queryByTestId('pause-campaign-button')).toBeNull();
    expect(screen.getByTestId('activate-campaign-button')).toBeTruthy();

    confirm.mockReturnValueOnce(false);
    fireEvent.click(screen.getByTestId('end-campaign-button'));
    expect(adminApi.updateAdminCampaign).not.toHaveBeenCalled();

    confirm.mockReturnValueOnce(true);
    fireEvent.click(screen.getByTestId('end-campaign-button'));
    await waitFor(() => expect(adminApi.updateAdminCampaign).toHaveBeenCalledWith(ID, { status: 'ended' }));
    await waitFor(() => expect(screen.getByTestId('campaign-status-badge').textContent).toBe('Завершена'));
    expect(screen.queryByTestId('end-campaign-button')).toBeNull();
    expect(screen.queryByTestId('activate-campaign-button')).toBeNull();
    expect(screen.queryByTestId('edit-campaign-button')).toBeNull();
  });

  it('an active campaign can only be ended, not re-activated or paused', async () => {
    vi.mocked(adminApi.getAdminCampaign).mockResolvedValue(base({ status: 'active' }));
    mount();
    await waitFor(() => expect(screen.getByTestId('end-campaign-button')).toBeTruthy());
    expect(screen.queryByTestId('activate-campaign-button')).toBeNull();
    expect(screen.queryByTestId('pause-campaign-button')).toBeNull();
  });

  it('does not show a success when the status update fails', async () => {
    vi.mocked(adminApi.updateAdminCampaign).mockRejectedValue(new Error('forbidden'));
    mount();
    await waitFor(() => expect(screen.getByTestId('activate-campaign-button')).toBeTruthy());
    fireEvent.click(screen.getByTestId('activate-campaign-button'));
    await waitFor(() => expect(screen.getByTestId('detail-error').textContent).toContain('forbidden'));
    expect(screen.queryByTestId('detail-success')).toBeNull();
    expect(screen.getByTestId('campaign-status-badge').textContent).toBe('Одобрена');
  });

  it('explains a rejected transition and resyncs the stale status from the server', async () => {
    const conflict = Object.assign(new Error('campaign status transition is not allowed: ended -> active'), { code: 'invalid_status_transition', status: 409 });
    vi.mocked(adminApi.updateAdminCampaign).mockRejectedValue(conflict);
    vi.mocked(adminApi.getAdminCampaign).mockResolvedValueOnce(base()).mockResolvedValue(base({ status: 'ended' }));
    mount();
    await waitFor(() => expect(screen.getByTestId('activate-campaign-button')).toBeTruthy());
    fireEvent.click(screen.getByTestId('activate-campaign-button'));
    await waitFor(() => expect(screen.getByTestId('detail-error').textContent).toContain('Такой переход статуса недоступен'));
    await waitFor(() => expect(screen.getByTestId('campaign-status-badge').textContent).toBe('Завершена'));
    expect(screen.queryByTestId('activate-campaign-button')).toBeNull();
  });

  it('read-only users get no edit or status actions', async () => {
    vi.mocked(useAdminAuth).mockReturnValue(auth(['marketing.campaigns.read']));
    mount();
    await waitFor(() => expect(screen.getByTestId('campaign-title')).toBeTruthy());
    for (const t of ['edit-campaign-button', 'activate-campaign-button', 'end-campaign-button']) {
      expect(screen.queryByTestId(t)).toBeNull();
    }
  });

  it('promotion campaigns are read-only in admin (the API only updates advertising campaigns)', async () => {
    vi.mocked(adminApi.getAdminCampaign).mockResolvedValue(base({ purpose: 'promotion' }));
    mount();
    await waitFor(() => expect(screen.getByTestId('promotion-readonly-note')).toBeTruthy());
    expect(screen.queryByTestId('edit-campaign-button')).toBeNull();
    expect(screen.queryByTestId('end-campaign-button')).toBeNull();
  });

  it('edit sends only changed fields with inclusive end date and validates the plan', async () => {
    vi.mocked(adminApi.updateAdminCampaign).mockResolvedValue(base({ title: 'Summer 2', plannedBudgetCents: 7000000 }));
    mount();
    await waitFor(() => expect(screen.getByTestId('edit-campaign-button')).toBeTruthy());
    fireEvent.click(screen.getByTestId('edit-campaign-button'));

    fireEvent.change(screen.getByTestId('edit-budget-input'), { target: { value: '0' } });
    fireEvent.click(screen.getByTestId('save-campaign-button'));
    expect(screen.getByTestId('edit-error').textContent).toContain('больше нуля');

    fireEvent.change(screen.getByTestId('edit-budget-input'), { target: { value: '70000' } });
    fireEvent.change(screen.getByLabelText('Дата окончания'), { target: { value: '2026-05-01' } });
    fireEvent.click(screen.getByTestId('save-campaign-button'));
    expect(screen.getByTestId('edit-error').textContent).toContain('не может быть раньше');
    expect(adminApi.updateAdminCampaign).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Дата окончания'), { target: { value: '2026-09-15' } });
    fireEvent.change(screen.getByTestId('edit-title-input'), { target: { value: 'Summer 2' } });
    fireEvent.click(screen.getByTestId('save-campaign-button'));
    await waitFor(() => expect(adminApi.updateAdminCampaign).toHaveBeenCalledWith(ID, {
      title: 'Summer 2',
      plannedBudgetCents: 7000000,
      endsAt: '2026-09-15T23:59:59.000Z',
    }));
    await waitFor(() => expect(screen.getByTestId('campaign-title').textContent).toBe('Summer 2'));
  });

  it('keeps the edit form open and shows the API error when saving fails', async () => {
    vi.mocked(adminApi.updateAdminCampaign).mockRejectedValue(new Error('invalid_parameter'));
    mount();
    await waitFor(() => expect(screen.getByTestId('edit-campaign-button')).toBeTruthy());
    fireEvent.click(screen.getByTestId('edit-campaign-button'));
    fireEvent.change(screen.getByTestId('edit-title-input'), { target: { value: 'New' } });
    fireEvent.click(screen.getByTestId('save-campaign-button'));
    await waitFor(() => expect(screen.getByTestId('edit-error').textContent).toContain('invalid_parameter'));
    expect(screen.getByTestId('campaign-edit-form')).toBeTruthy();
    expect(screen.getByTestId('campaign-title').textContent).toBe('Summer');
  });

  it('shows a load error state when the campaign cannot be fetched', async () => {
    vi.mocked(adminApi.getAdminCampaign).mockRejectedValue(new Error('network'));
    mount();
    await waitFor(() => expect(screen.getByTestId('campaign-not-found')).toBeTruthy());
    expect(screen.getByTestId('detail-load-error').textContent).toBe('network');
  });
});
