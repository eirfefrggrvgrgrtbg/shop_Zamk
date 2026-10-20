// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { AdminCollections, collectionStateLabel } from './AdminCollections';
import * as adminApi from '@zamk/api-client/src/admin';

vi.mock('@zamk/api-client/src/admin');

const col = (over: Record<string, unknown> = {}) => ({
  id: 'c1', slug: 'summer', title: 'Лето', description: null, coverImageUrl: null,
  isActive: true, startsAt: null, endsAt: null, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z', ...over,
});

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/collections']}>
      <Routes>
        <Route path="/collections" element={<AdminCollections />} />
        <Route path="/collections/:id" element={<div>detail-page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('AdminCollections', () => {
  beforeEach(() => vi.resetAllMocks());
  afterEach(() => cleanup());

  it('derives the publication state label', () => {
    const now = new Date('2026-06-01T00:00:00Z');
    expect(collectionStateLabel({ isActive: false }, now)).toBe('Черновик');
    expect(collectionStateLabel({ isActive: true }, now)).toBe('Опубликована');
    expect(collectionStateLabel({ isActive: true, startsAt: '2026-07-01T00:00:00Z' }, now)).toBe('Запланирована');
    expect(collectionStateLabel({ isActive: true, endsAt: '2026-05-01T00:00:00Z' }, now)).toBe('Завершена');
  });

  it('shows the empty state', async () => {
    vi.mocked(adminApi.getAdminCollections).mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText('Подборок пока нет')).toBeDefined();
  });

  it('lists collections with their state', async () => {
    vi.mocked(adminApi.getAdminCollections).mockResolvedValue([col(), col({ id: 'c2', title: 'Осень', slug: 'autumn', isActive: false })] as never);
    renderPage();
    expect(await screen.findByText('Лето')).toBeDefined();
    expect(screen.getByText('Опубликована')).toBeDefined();
    expect(screen.getByText('Черновик')).toBeDefined();
  });

  it('shows an API error', async () => {
    vi.mocked(adminApi.getAdminCollections).mockRejectedValue(new Error('boom'));
    renderPage();
    expect((await screen.findByRole('alert')).textContent).toContain('boom');
  });

  it('creates a draft with a generated slug and opens it', async () => {
    vi.mocked(adminApi.getAdminCollections).mockResolvedValue([]);
    vi.mocked(adminApi.createAdminCollection).mockResolvedValue(col({ id: 'new-id', isActive: false }) as never);
    renderPage();
    fireEvent.click(await screen.findByText('Создать подборку'));
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Летний гардероб' } });
    expect((screen.getByLabelText('Адрес (slug)') as HTMLInputElement).value).toBe('letniy-garderob');
    fireEvent.change(screen.getByLabelText('Краткое описание'), { target: { value: 'Вещи на лето' } });
    fireEvent.click(screen.getByText('Создать черновик'));
    await waitFor(() => expect(screen.getByText('detail-page')).toBeDefined());
    expect(adminApi.createAdminCollection).toHaveBeenCalledWith({
      title: 'Летний гардероб', slug: 'letniy-garderob', description: 'Вещи на лето', isActive: false,
    });
  });

  it('reports a duplicate slug and does not navigate', async () => {
    vi.mocked(adminApi.getAdminCollections).mockResolvedValue([]);
    vi.mocked(adminApi.createAdminCollection).mockRejectedValue(Object.assign(new Error('x'), { code: 'duplicate_slug' }));
    renderPage();
    fireEvent.click(await screen.findByText('Создать подборку'));
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Лето' } });
    fireEvent.click(screen.getByText('Создать черновик'));
    expect((await screen.findByRole('alert')).textContent).toContain('уже есть');
    expect(screen.queryByText('detail-page')).toBeNull();
  });

  it('rejects an invalid slug before calling the API', async () => {
    vi.mocked(adminApi.getAdminCollections).mockResolvedValue([]);
    renderPage();
    fireEvent.click(await screen.findByText('Создать подборку'));
    fireEvent.change(screen.getByLabelText('Название'), { target: { value: 'Лето' } });
    fireEvent.change(screen.getByLabelText('Адрес (slug)'), { target: { value: 'Bad Slug' } });
    fireEvent.click(screen.getByText('Создать черновик'));
    expect((await screen.findByRole('alert')).textContent).toContain('латинские');
    expect(adminApi.createAdminCollection).not.toHaveBeenCalled();
  });
});
