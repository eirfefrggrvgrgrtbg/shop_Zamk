// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminCollectionDetail } from './AdminCollectionDetail';
import * as adminApi from '@zamk/api-client/src/admin';

vi.mock('@zamk/api-client/src/admin');

const detail = (over: Record<string, unknown> = {}) => ({
  id: 'c1', slug: 'summer', title: 'Лето', description: 'Описание', coverImageUrl: null,
  isActive: false, startsAt: null, endsAt: null, createdAt: '', updatedAt: '',
  items: [
    { productId: 'p1', title: 'Платье', status: 'published', sortOrder: 0 },
    { productId: 'p2', title: 'Пальто', status: 'hidden', sortOrder: 1 },
    { productId: 'p3', title: 'Рубашка', status: 'published', sortOrder: 2 },
  ],
  ...over,
});

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/collections/c1']}>
      <Routes>
        <Route path="/collections/:id" element={<AdminCollectionDetail />} />
      </Routes>
    </MemoryRouter>,
  );
}
const order = () => within(screen.getByTestId('collection-items')).getAllByRole('listitem').map((li) => li.textContent ?? '');

describe('AdminCollectionDetail', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(adminApi.getAdminCollection).mockResolvedValue(detail() as never);
    vi.mocked(adminApi.updateAdminCollection).mockResolvedValue({} as never);
    vi.mocked(adminApi.replaceAdminCollectionItems).mockResolvedValue(undefined);
  });
  afterEach(() => cleanup());

  it('loads the collection with its items and warns about unpublished products', async () => {
    renderPage();
    expect(await screen.findByTestId('collection-items')).toBeDefined();
    expect(order()[0]).toContain('Платье');
    expect(screen.getByRole('note').textContent).toContain('(1)');
    expect(screen.getByTestId('collection-state').textContent).toBe('Черновик');
  });

  it('shows not found for a missing collection', async () => {
    vi.mocked(adminApi.getAdminCollection).mockRejectedValue(Object.assign(new Error('x'), { status: 404 }));
    renderPage();
    expect((await screen.findByRole('alert')).textContent).toContain('не найдена');
  });

  it('reorders items and saves the exact order together with metadata and publication', async () => {
    renderPage();
    await screen.findByTestId('collection-items');
    fireEvent.click(screen.getByLabelText('Выше: Рубашка'));
    fireEvent.click(screen.getByLabelText('Выше: Рубашка'));
    expect(order().map((t) => t.replace(/\d+|↑|↓|Убрать|Скрыт/g, '').trim().slice(0, 7))).toEqual(['Рубашка', 'Платье', 'Пальто']);

    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Лето 2026' } });
    fireEvent.click(screen.getByTestId('collection-active'));
    fireEvent.click(screen.getByTestId('collection-save'));

    await waitFor(() => expect(adminApi.replaceAdminCollectionItems).toHaveBeenCalledTimes(1));
    expect(adminApi.replaceAdminCollectionItems).toHaveBeenCalledWith('c1', ['p3', 'p1', 'p2']);
    expect(adminApi.updateAdminCollection).toHaveBeenCalledWith('c1', expect.objectContaining({
      title: 'Лето 2026', slug: 'summer', isActive: true, description: 'Описание', coverImageUrl: null,
    }));
  });

  it('removes an item', async () => {
    renderPage();
    await screen.findByTestId('collection-items');
    fireEvent.click(screen.getByLabelText('Убрать: Пальто'));
    fireEvent.click(screen.getByTestId('collection-save'));
    await waitFor(() => expect(adminApi.replaceAdminCollectionItems).toHaveBeenCalledWith('c1', ['p1', 'p3']));
  });

  it('finds published products by search and adds each only once', async () => {
    vi.mocked(adminApi.getAdminProducts).mockResolvedValue({
      items: [{ id: 'p9', title: 'Юбка', status: 'published' }, { id: 'p1', title: 'Платье', status: 'published' }],
      totalCount: 2,
    } as never);
    renderPage();
    await screen.findByTestId('collection-items');
    fireEvent.change(screen.getByLabelText(/Добавить товар/), { target: { value: 'юб' } });
    const results = await screen.findByTestId('collection-search-results');
    expect(adminApi.getAdminProducts).toHaveBeenCalledWith(expect.objectContaining({ q: 'юб', status: 'published' }));

    // already present product cannot be added twice
    expect((within(results).getByText('Добавлен') as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(within(results).getByText('Добавить'));
    expect(order()).toHaveLength(4);
    fireEvent.click(screen.getByTestId('collection-save'));
    await waitFor(() => expect(adminApi.replaceAdminCollectionItems).toHaveBeenCalledWith('c1', ['p1', 'p2', 'p3', 'p9']));
  });

  it('blocks save with an invalid period and does not call the API', async () => {
    renderPage();
    await screen.findByTestId('collection-items');
    fireEvent.change(screen.getByLabelText(/Показывать с/), { target: { value: '2026-06-02T10:00' } });
    fireEvent.change(screen.getByLabelText(/Показывать до/), { target: { value: '2026-06-01T10:00' } });
    fireEvent.click(screen.getByTestId('collection-save'));
    expect((await screen.findByRole('alert')).textContent).toContain('позже');
    expect(adminApi.updateAdminCollection).not.toHaveBeenCalled();
  });

  it('shows duplicate slug errors from the API and keeps the edits', async () => {
    vi.mocked(adminApi.updateAdminCollection).mockRejectedValue(Object.assign(new Error('x'), { code: 'duplicate_slug' }));
    renderPage();
    await screen.findByTestId('collection-items');
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Новое имя' } });
    fireEvent.click(screen.getByTestId('collection-save'));
    expect((await screen.findByRole('alert')).textContent).toContain('уже есть');
    expect((screen.getByLabelText('Название') as HTMLInputElement).value).toBe('Новое имя');
    expect(adminApi.replaceAdminCollectionItems).not.toHaveBeenCalled();
  });
});
