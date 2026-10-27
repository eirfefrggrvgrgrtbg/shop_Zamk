/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Navbar } from './Navbar';

const auth = { isAuthenticated: false, openAuthModal: vi.fn() };
vi.mock('../../contexts/CartContext', () => ({ useCart: () => ({ totalItems: 0 }) }));
vi.mock('../../contexts/FavoritesContext', () => ({ useFavorites: () => ({ favorites: [] }) }));
vi.mock('../../contexts/AuthContext', () => ({ useAuth: () => auth }));
vi.mock('../../contexts/SearchContext', () => ({ useSearch: () => ({ openSearch: vi.fn() }) }));
vi.mock('../auth/ProfileMenu', () => ({ ProfileMenu: () => null }));
vi.mock('../notifications/NotificationBell', () => ({ NotificationBell: () => null }));
vi.mock('./Sidebar', () => ({ Sidebar: () => null }));
vi.mock('@zamk/api-client/src/public', () => ({ getNavHighlightAuctions: () => Promise.resolve([]) }));

const renderNav = () =>
  render(
    <MemoryRouter initialEntries={['/catalog']}>
      <Navbar />
    </MemoryRouter>
  );

describe('Navbar responsive contract', () => {
  afterEach(cleanup);

  it('inline links appear only from lg (1024px) and the burger covers everything below', () => {
    renderNav();
    const nav = screen.getByRole('navigation', { name: 'Основная навигация' });
    // Tailwind: hidden below lg, flex from lg. At 768–1023 the links would collide with the centered logo.
    expect(nav.className).toContain('hidden');
    expect(nav.className).toContain('lg:flex');
    expect(nav.className).not.toMatch(/(^|\s)md:flex/);
    const burger = screen.getByRole('button', { name: 'Открыть меню' });
    expect(burger.className).toContain('lg:hidden');
    expect(burger.className).not.toMatch(/(^|\s)md:hidden/);
  });

  it('desktop navigation keeps all three sections and the logo link', () => {
    renderNav();
    const nav = screen.getByRole('navigation', { name: 'Основная навигация' });
    expect(within(nav).getByText('Каталог').closest('a')?.getAttribute('href')).toBe('/catalog');
    expect(within(nav).getByText('Вещи ZAMK').closest('a')?.getAttribute('href')).toBe('/zamk');
    expect(within(nav).getByText('Бренды').closest('a')?.getAttribute('href')).toBe('/brands');
    expect(screen.getAllByText('ЗАМК').length).toBeGreaterThan(0);
  });

  it('the burger drawer exposes every section, favorites and login', () => {
    renderNav();
    fireEvent.click(screen.getByRole('button', { name: 'Открыть меню' }));
    const drawer = screen.getByTestId('drawer-portal');
    const hrefs = Array.from(drawer.querySelectorAll('a')).map((a) => a.getAttribute('href'));
    for (const h of ['/', '/catalog', '/zamk', '/brands', '/favorites']) expect(hrefs).toContain(h);
    fireEvent.click(within(drawer).getByRole('button', { name: /Вход/ }));
    expect(auth.openAuthModal).toHaveBeenCalledWith('login');
  });

  it('mobile bottom navigation keeps its five controls below md', () => {
    const { container } = renderNav();
    const bottom = Array.from(container.querySelectorAll('div.fixed')).find((e) => e.className.includes('bottom-4')) as HTMLElement;
    expect(bottom.className).toContain('md:hidden');
    expect(bottom.querySelectorAll('a, button').length).toBe(5);
  });
});
