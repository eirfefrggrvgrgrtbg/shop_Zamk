// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminCollectionSubmissions } from './AdminCollectionSubmissions';
import * as adminApi from '@zamk/api-client/src/admin';

vi.mock('@zamk/api-client/src/admin');

const sub = (over: Record<string, unknown> = {}) => ({
  id: 's1', collectionId: 'c1', collectionTitle: 'Лето', collectionSlug: 'summer', sellerId: 'sel1', brandName: 'Brand A',
  product: { id: 'p1', title: 'Платье', imageUrl: 'https://img/p1.jpg', status: 'published' },
  status: 'pending', sellerComment: 'Новая капсула', rejectionReason: null, createdAt: '2026-06-01T00:00:00Z', reviewedAt: null,
  reviewerName: null, ...over,
});
const view = () => render(<MemoryRouter><AdminCollectionSubmissions /></MemoryRouter>);

describe('AdminCollectionSubmissions', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(adminApi.getAdminCollectionSubmissions).mockResolvedValue([sub()] as never);
  });
  afterEach(() => cleanup());

  it('lists pending submissions with brand, product, photo, collection, comment and a product link', async () => {
    view();
    const row = await screen.findByTestId('submission-s1');
    expect(adminApi.getAdminCollectionSubmissions).toHaveBeenCalledWith('pending');
    expect(row.textContent).toContain('Brand A');
    expect(row.textContent).toContain('Платье');
    expect(row.textContent).toContain('Лето');
    expect(row.textContent).toContain('Новая капсула');
    expect((row.querySelector('img') as HTMLImageElement).src).toBe('https://img/p1.jpg');
    expect(row.querySelector('a[href="/products/p1"]')).not.toBeNull();
  });

  it('empty state', async () => {
    vi.mocked(adminApi.getAdminCollectionSubmissions).mockResolvedValue([]);
    view();
    expect((await screen.findByTestId('submissions-empty')).textContent).toContain('Нет заявок');
  });

  it('accepts a submission and refreshes the list', async () => {
    vi.mocked(adminApi.acceptAdminCollectionSubmission).mockResolvedValue(sub({ status: 'accepted' }) as never);
    vi.mocked(adminApi.getAdminCollectionSubmissions).mockResolvedValueOnce([sub()] as never).mockResolvedValueOnce([]);
    view();
    fireEvent.click(await screen.findByTestId('accept-s1'));
    await waitFor(() => expect(adminApi.acceptAdminCollectionSubmission).toHaveBeenCalledWith('s1'));
    expect((await screen.findByRole('status')).textContent).toContain('добавлен в подборку');
    await waitFor(() => expect(screen.queryByTestId('submission-s1')).toBeNull());
    expect(adminApi.getAdminCollectionSubmissions).toHaveBeenCalledTimes(2);
  });

  it('shows a clear error when acceptance fails and still refreshes', async () => {
    vi.mocked(adminApi.acceptAdminCollectionSubmission).mockRejectedValue(new Error('Товар должен быть опубликован и доступен в каталоге'));
    view();
    fireEvent.click(await screen.findByTestId('accept-s1'));
    expect((await screen.findByRole('alert')).textContent).toContain('опубликован');
    await waitFor(() => expect(adminApi.getAdminCollectionSubmissions).toHaveBeenCalledTimes(2));
  });

  it('rejects with a reason that is sent to the API', async () => {
    vi.mocked(adminApi.rejectAdminCollectionSubmission).mockResolvedValue(sub({ status: 'rejected' }) as never);
    view();
    fireEvent.click(await screen.findByTestId('reject-s1'));
    fireEvent.change(screen.getByLabelText('Причина отклонения'), { target: { value: ' Не подходит по теме ' } });
    fireEvent.click(screen.getByTestId('confirm-reject-s1'));
    await waitFor(() => expect(adminApi.rejectAdminCollectionSubmission).toHaveBeenCalledWith('s1', 'Не подходит по теме'));
    expect((await screen.findByRole('status')).textContent).toContain('отклонена');
  });

  it('status filter switches the list and shows review history', async () => {
    vi.mocked(adminApi.getAdminCollectionSubmissions).mockImplementation(async (status) =>
      status === 'rejected'
        ? ([sub({ id: 'r1', status: 'rejected', rejectionReason: 'Не подходит', reviewedAt: '2026-06-02T00:00:00Z', reviewerName: 'Редактор' })] as never)
        : ([sub()] as never),
    );
    view();
    await screen.findByTestId('submission-s1');
    fireEvent.change(screen.getByLabelText('Статус'), { target: { value: 'rejected' } });
    const review = await screen.findByTestId('submission-review-r1');
    expect(adminApi.getAdminCollectionSubmissions).toHaveBeenLastCalledWith('rejected');
    expect(review.textContent).toContain('Отклонена');
    expect(review.textContent).toContain('Редактор');
    expect(review.textContent).toContain('Не подходит');
    expect(screen.queryByTestId('accept-r1')).toBeNull();
  });

  it('load error', async () => {
    vi.mocked(adminApi.getAdminCollectionSubmissions).mockRejectedValue(new Error('boom'));
    view();
    expect((await screen.findByRole('alert')).textContent).toContain('boom');
  });
});
