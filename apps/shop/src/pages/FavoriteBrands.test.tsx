/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { FavoriteBrands } from './FavoriteBrands';

const ctx = vi.hoisted(() => ({
  value: {} as Record<string, unknown>,
}));
vi.mock('../contexts/FavoriteBrandsContext', () => ({ useFavoriteBrands: () => ctx.value }));
vi.mock('../components/account/AccountLayout', () => ({
  AccountLayout: ({ children, title }: { children: React.ReactNode; title: string }) => (
    <div><h1>{title}</h1>{children}</div>
  ),
}));

const toggle = vi.fn();
const reload = vi.fn();
const base = { isLoading: false, error: null, pendingIds: [], toggleFavoriteBrand: toggle, reloadFavoriteBrands: reload };
const brand = { id: 'b-1', name: 'Acne Studios', slug: 'acne-studios', description: 'Stockholm', addedAt: '2026-01-01T00:00:00Z' };

const renderPage = () => render(<MemoryRouter><FavoriteBrands /></MemoryRouter>);

describe('FavoriteBrands account page (MARKETING.M1.11)', () => {
  beforeEach(() => { vi.clearAllMocks(); toggle.mockResolvedValue(true); });
  afterEach(cleanup);

  it('lists saved brands with a storefront link', () => {
    ctx.value = { ...base, brands: [brand] };
    renderPage();
    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.getByRole('link', { name: /Acne Studios/ }).getAttribute('href')).toBe('/brand/acne-studios');
  });

  it('removes a brand through the context', async () => {
    ctx.value = { ...base, brands: [brand] };
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /Удалить Acne Studios/ }));
    await waitFor(() => expect(toggle).toHaveBeenCalledWith('b-1'));
  });

  it('shows an alert if removal fails', async () => {
    toggle.mockResolvedValue(false);
    ctx.value = { ...base, brands: [brand] };
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: /Удалить Acne Studios/ }));
    expect((await screen.findByRole('alert')).textContent).toMatch(/Не удалось удалить/);
  });

  it('renders an empty state with a link to brands', () => {
    ctx.value = { ...base, brands: [] };
    renderPage();
    expect(screen.getByText('Любимых брендов пока нет')).toBeDefined();
    expect(screen.getByRole('link', { name: /Смотреть бренды/ }).getAttribute('href')).toBe('/brands');
  });

  it('renders a server error state with retry', () => {
    ctx.value = { ...base, brands: [], error: 'Не удалось загрузить любимые бренды.' };
    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    expect(reload).toHaveBeenCalled();
  });

  it('does not render internal seller data', () => {
    ctx.value = { ...base, brands: [{ ...brand, sellerId: 'secret-seller', contactEmail: 'x@y.z' }] };
    const { container } = renderPage();
    expect(container.textContent).not.toMatch(/secret-seller|x@y\.z/);
  });
});
