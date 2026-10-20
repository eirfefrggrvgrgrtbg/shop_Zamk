/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerCollections } from './SellerCollections';
import {
  createSellerCollectionSubmission,
  getSellerCollectionOptions,
  getSellerCollectionSubmissions,
  getSellerProductsPaginated,
} from '@zamk/api-client/src/seller';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerCollectionOptions: vi.fn(),
  getSellerCollectionSubmissions: vi.fn(),
  createSellerCollectionSubmission: vi.fn(),
  getSellerProductsPaginated: vi.fn(),
}));

const sub = (over: Record<string, unknown> = {}) => ({
  id: 's1', collectionId: 'c1', collectionTitle: 'Лето', product: { id: 'p1', title: 'Платье', imageUrl: null, status: 'published' },
  status: 'pending', sellerComment: null, rejectionReason: null, createdAt: '2026-06-01T00:00:00Z', reviewedAt: null, ...over,
});
const render_ = () => render(<MemoryRouter><SellerCollections /></MemoryRouter>);

describe('SellerCollections', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(getSellerCollectionOptions).mockResolvedValue([{ id: 'c1', slug: 'summer', title: 'Лето', description: 'Лёгкие вещи' }]);
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [
        { id: 'p1', title: 'Платье', status: 'published' },
        { id: 'p2', title: 'Черновик-товар', status: 'draft' },
      ],
      totalCount: 2,
    } as never);
    vi.mocked(getSellerCollectionSubmissions).mockResolvedValue([]);
  });
  afterEach(() => cleanup());

  it('offers published collections and only published own products', async () => {
    render_();
    const productSelect = (await screen.findByLabelText('Товар')) as HTMLSelectElement;
    expect([...productSelect.options].map((o) => o.text)).toEqual(['Выберите товар', 'Платье']);
    expect(screen.getByLabelText('Подборка')).toBeDefined();
    expect(screen.getByTestId('no-submissions')).toBeDefined();
  });

  it('submits a request with an optional comment and refreshes "my submissions"', async () => {
    vi.mocked(createSellerCollectionSubmission).mockResolvedValue(sub() as never);
    vi.mocked(getSellerCollectionSubmissions).mockResolvedValueOnce([]).mockResolvedValueOnce([sub({ sellerComment: 'Новая капсула' })] as never);
    render_();
    fireEvent.change(await screen.findByLabelText('Подборка'), { target: { value: 'c1' } });
    fireEvent.change(screen.getByLabelText('Товар'), { target: { value: 'p1' } });
    fireEvent.change(screen.getByLabelText(/Комментарий/), { target: { value: ' Новая капсула ' } });
    fireEvent.click(screen.getByTestId('submission-submit'));

    await waitFor(() => expect(createSellerCollectionSubmission).toHaveBeenCalledWith({ collectionId: 'c1', productId: 'p1', comment: 'Новая капсула' }));
    expect((await screen.findByRole('status')).textContent).toContain('Заявка отправлена');
    expect((await screen.findByTestId('submission-status-s1')).textContent).toBe('На рассмотрении');
  });

  it('requires a collection and a product before sending', async () => {
    render_();
    fireEvent.click(await screen.findByTestId('submission-submit'));
    expect((await screen.findByRole('alert')).textContent).toContain('Выберите подборку');
    fireEvent.change(screen.getByLabelText('Подборка'), { target: { value: 'c1' } });
    fireEvent.click(screen.getByTestId('submission-submit'));
    expect((await screen.findByRole('alert')).textContent).toContain('Выберите товар');
    expect(createSellerCollectionSubmission).not.toHaveBeenCalled();
  });

  it('shows the server error (e.g. duplicate) and keeps the form', async () => {
    vi.mocked(createSellerCollectionSubmission).mockRejectedValue(new Error('Заявка на этот товар в эту подборку уже ожидает рассмотрения'));
    render_();
    fireEvent.change(await screen.findByLabelText('Подборка'), { target: { value: 'c1' } });
    fireEvent.change(screen.getByLabelText('Товар'), { target: { value: 'p1' } });
    fireEvent.click(screen.getByTestId('submission-submit'));
    expect((await screen.findByRole('alert')).textContent).toContain('уже ожидает');
    expect((screen.getByLabelText('Товар') as HTMLSelectElement).value).toBe('p1');
  });

  it('shows statuses and the rejection reason', async () => {
    vi.mocked(getSellerCollectionSubmissions).mockResolvedValue([
      sub({ id: 'a', status: 'accepted' }),
      sub({ id: 'b', status: 'rejected', rejectionReason: 'Не подходит по теме' }),
    ] as never);
    render_();
    expect((await screen.findByTestId('submission-status-a')).textContent).toBe('Принята');
    expect(screen.getByTestId('submission-status-b').textContent).toBe('Отклонена');
    expect(screen.getByTestId('submission-reason-b').textContent).toContain('Не подходит по теме');
    expect(screen.queryByTestId('submission-reason-a')).toBeNull();
  });

  it('empty states: no collections / no products / load error', async () => {
    vi.mocked(getSellerCollectionOptions).mockResolvedValue([]);
    render_();
    expect(await screen.findByTestId('no-collections')).toBeDefined();
    cleanup();

    vi.mocked(getSellerCollectionOptions).mockResolvedValue([{ id: 'c1', slug: 's', title: 'Лето' }]);
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [{ id: 'p2', title: 'x', status: 'draft' }], totalCount: 1 } as never);
    render_();
    expect(await screen.findByTestId('no-products')).toBeDefined();
    cleanup();

    vi.mocked(getSellerCollectionOptions).mockRejectedValue(new Error('down'));
    render_();
    expect((await screen.findByRole('alert')).textContent).toContain('Не удалось загрузить');
  });
});
