/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { Catalog } from './Catalog';
import * as authContext from '../contexts/AuthContext';
import * as publicCatalog from '../api/publicCatalog';

(window as any).IntersectionObserver = class { observe() {} unobserve() {} disconnect() {} };

vi.mock('../contexts/AuthContext', () => ({ useAuth: vi.fn() }));
vi.mock('../contexts/FavoritesContext', () => ({ useFavorites: () => ({ isFavorite: () => false, toggleFavorite: vi.fn() }) }));
vi.mock('../contexts/ToastContext', () => ({ useToast: () => ({ showToast: vi.fn() }) }));
vi.mock('../contexts/CartContext', () => ({ useCart: () => ({ addItem: vi.fn() }) }));
vi.mock('../api/publicCatalog', () => ({
  fetchProducts: vi.fn(),
  fetchCategories: vi.fn(),
  fetchBrands: vi.fn().mockResolvedValue([]),
}));

const INT = { code: 'INT', name: 'International', values: ['XS', 'S', 'M', 'L'].map((value, i) => ({ value, sortOrder: i })) };
const EU = { code: 'EU', name: 'European', values: ['38', '40', '42'].map((value, i) => ({ value, sortOrder: i })) };
const categories = [
  { id: 'c-tops', slug: 'tops', name: 'Топы', icon: '', sizeSystem: INT },
  { id: 'c-shoes', slug: 'shoes', name: 'Обувь', icon: '', sizeSystem: EU },
  { id: 'c-bags', slug: 'bags', name: 'Сумки', icon: '' },
];
const product = { id: 'p1', name: 'Sized Product', price: 1000, category: 'c-tops', brand: 'B', brandId: 'b1', sellerId: 's', image: 'x.jpg', images: [], isNew: false };

function Loc() {
  const l = useLocation();
  return <div data-testid="loc">{l.pathname + l.search}</div>;
}
const renderAt = (url: string) =>
  render(<MemoryRouter initialEntries={[url]}><Catalog /><Loc /></MemoryRouter>);
const lastParams = () => (vi.mocked(publicCatalog.fetchProducts).mock.calls.at(-1)?.[0] ?? {}) as Record<string, any>;
const loc = () => decodeURIComponent(screen.getByTestId('loc').textContent || '');
const btn = (code: string, v: string) => screen.getAllByTestId(`size-filter-${code}-${v}`)[0];

describe('Catalog size filter (CATALOG.C2)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false, user: null } as any);
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue(categories as any);
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [product as any], totalCount: 1 });
  });
  afterEach(cleanup);

  it('selects several sizes, sends them to the API and keeps them in the URL; removal and reset work', async () => {
    renderAt('/catalog?categoryId=c-tops');
    await waitFor(() => expect(btn('INT', 'S')).toBeTruthy());
    fireEvent.click(btn('INT', 'S'));
    fireEvent.click(btn('INT', 'M'));
    await waitFor(() => expect(lastParams()).toMatchObject({ size: 'S,M', sizeSystem: 'INT', categoryId: 'c-tops' }));
    expect(loc()).toContain('size=S,M');
    expect(btn('INT', 'S').getAttribute('aria-pressed')).toBe('true');

    fireEvent.click(screen.getAllByLabelText('Снять фильтр размера S')[0]);
    await waitFor(() => expect(lastParams().size).toBe('M'));
    expect(loc()).toContain('size=M');

    fireEvent.click(screen.getAllByText('Сбросить размеры')[0]);
    await waitFor(() => expect(lastParams().size).toBeUndefined());
    expect(loc()).not.toContain('size=');
  });

  it('restores selected sizes from the URL after reload', async () => {
    renderAt('/catalog?categoryId=c-tops&size=S,L&sizeSystem=INT');
    await waitFor(() => expect(lastParams()).toMatchObject({ size: 'S,L', sizeSystem: 'INT' }));
    await waitFor(() => expect(btn('INT', 'L').getAttribute('aria-pressed')).toBe('true'));
  });

  it('shows only the sizes of the current category system', async () => {
    renderAt('/catalog?categoryId=c-shoes');
    await waitFor(() => expect(btn('EU', '40')).toBeTruthy());
    expect(screen.queryAllByTestId('size-filter-INT-S')).toHaveLength(0);
  });

  it('drops invalid sizes when the category changes to another system', async () => {
    renderAt('/catalog?categoryId=c-tops&size=S&sizeSystem=INT');
    await waitFor(() => expect(lastParams().size).toBe('S'));
    fireEvent.click(screen.getAllByText('Обувь')[0]);
    await waitFor(() => expect(lastParams().size).toBeUndefined());
    expect(lastParams().categoryId).toBe('c-shoes');
    expect(loc()).not.toContain('size=');
  });

  it('removes sizes from a crafted URL that do not exist for the category', async () => {
    renderAt('/catalog?categoryId=c-tops&size=S,42&sizeSystem=INT');
    await waitFor(() => expect(loc()).toContain('size=S'));
    await waitFor(() => expect(loc()).not.toContain('42'));
  });

  it('never mixes systems under "all categories": picking another system replaces the selection', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(btn('INT', 'S')).toBeTruthy());
    expect(btn('EU', '38')).toBeTruthy();
    fireEvent.click(btn('INT', 'S'));
    await waitFor(() => expect(lastParams()).toMatchObject({ size: 'S', sizeSystem: 'INT' }));
    fireEvent.click(btn('EU', '40'));
    await waitFor(() => expect(lastParams()).toMatchObject({ size: '40', sizeSystem: 'EU' }));
  });

  it('explains when the category has no size grid', async () => {
    renderAt('/catalog?categoryId=c-bags');
    await waitFor(() => expect(screen.getAllByTestId('size-filter-empty').length).toBeGreaterThan(0));
  });

  it('shows the empty state when no product matches the selected sizes', async () => {
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [], totalCount: 0 });
    renderAt('/catalog?categoryId=c-tops&size=XS&sizeSystem=INT');
    await waitFor(() => expect(screen.queryByText('Sized Product')).toBeNull());
    await waitFor(() => expect(lastParams().size).toBe('XS'));
    expect(await screen.findByText('Ничего не найдено')).toBeTruthy();
  });
});
