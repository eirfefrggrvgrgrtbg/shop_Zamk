/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, act } from '@testing-library/react';
import { MemoryRouter, useLocation, useNavigate, useNavigationType } from 'react-router-dom';
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
  fetchBrands: vi.fn(),
  fetchColors: vi.fn(),
}));

const INT = { code: 'INT', name: 'International', values: ['S', 'M'].map((value, i) => ({ value, sortOrder: i })) };
const colors = [
  { id: 'col-black', name: 'Чёрный', hex: '#000000' },
  { id: 'col-white', name: 'Белый', hex: '#FFFFFF' },
  { id: 'col-nohex', name: 'Без образца' },
];
const product = { id: 'p1', name: 'Colored Product', price: 1000, category: 'c1', brand: 'B', brandId: 'b1', sellerId: 's', image: 'x.jpg', images: [], isNew: false };

type Entry = { url: string; type: string };
let entries: Entry[] = [];
let nav: ReturnType<typeof useNavigate>;
function Probe() {
  const l = useLocation();
  const type = useNavigationType();
  nav = useNavigate();
  const url = l.pathname + l.search;
  if (entries.at(-1)?.url !== url || entries.at(-1)?.type !== type) entries.push({ url, type });
  return <div data-testid="loc">{url}</div>;
}
const renderAt = (url: string) => render(<MemoryRouter initialEntries={[url]}><Catalog /><Probe /></MemoryRouter>);
const params = () => new URLSearchParams(decodeURIComponent(screen.getByTestId('loc').textContent || '').split('?')[1] || '');
const lastApi = () => (vi.mocked(publicCatalog.fetchProducts).mock.calls.at(-1)?.[0] ?? {}) as Record<string, any>;
const color = (id: string) => screen.getAllByTestId(`color-filter-${id}`)[0];
const pushes = () => entries.filter((e) => e.type === 'PUSH').length;

describe('Catalog color filter (CATALOG.C3)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    entries = [];
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false, user: null } as any);
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue([{ id: 'c1', slug: 'c1', name: 'Топы', icon: '', sizeSystem: INT }] as any);
    vi.mocked(publicCatalog.fetchBrands).mockResolvedValue([]);
    vi.mocked(publicCatalog.fetchColors).mockResolvedValue(colors);
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [product as any], totalCount: 1 });
  });
  afterEach(cleanup);

  it('lists dictionary colors with swatches only where a hex exists', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(color('col-black')).toBeTruthy());
    expect(color('col-black').textContent).toContain('Чёрный');
    expect((color('col-black').querySelector('span[aria-hidden]') as HTMLElement).style.backgroundColor).toBe('rgb(0, 0, 0)');
    expect((color('col-nohex').querySelector('span[aria-hidden]') as HTMLElement).style.backgroundColor).toBe('');
  });

  it('selects several colors, sends them to the API, keeps them in the URL; one entry per change', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(color('col-black')).toBeTruthy());
    fireEvent.click(color('col-black'));
    await waitFor(() => expect(params().get('colorId')).toBe('col-black'));
    fireEvent.click(color('col-white'));
    await waitFor(() => expect(params().get('colorId')).toBe('col-black,col-white'));
    await waitFor(() => expect(lastApi().colorId).toBe('col-black,col-white'));
    expect(color('col-white').getAttribute('aria-pressed')).toBe('true');
    expect(pushes()).toBe(2);
  });

  it('removes a single color via its tag, and resets all colors', async () => {
    renderAt('/catalog?colorId=col-black,col-white');
    await waitFor(() => expect(lastApi().colorId).toBe('col-black,col-white'));
    fireEvent.click(screen.getAllByLabelText('Снять фильтр цвета Чёрный')[0]);
    await waitFor(() => expect(lastApi().colorId).toBe('col-white'));
    fireEvent.click(screen.getAllByText('Сбросить цвета')[0]);
    await waitFor(() => expect(lastApi().colorId).toBeUndefined());
    expect(params().get('colorId')).toBeNull();
  });

  it('direct link / reload restores colors together with size, and does not touch history', async () => {
    renderAt('/catalog?categoryId=c1&colorId=col-black&size=M&sizeSystem=INT&sort=price_asc');
    await waitFor(() => expect(lastApi()).toMatchObject({ categoryId: 'c1', colorId: 'col-black', size: 'M', sizeSystem: 'INT', sort: 'price_asc' }));
    await waitFor(() => expect(color('col-black').getAttribute('aria-pressed')).toBe('true'));
    await new Promise((r) => setTimeout(r, 50));
    expect(entries).toHaveLength(1);
  });

  it('Back / Forward restore the color selection', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(color('col-black')).toBeTruthy());
    fireEvent.click(color('col-black'));
    await waitFor(() => expect(params().get('colorId')).toBe('col-black'));
    fireEvent.click(color('col-white'));
    await waitFor(() => expect(params().get('colorId')).toBe('col-black,col-white'));
    act(() => { nav(-1); });
    await waitFor(() => expect(lastApi().colorId).toBe('col-black'));
    expect(color('col-white').getAttribute('aria-pressed')).toBe('false');
    act(() => { nav(-1); });
    await waitFor(() => expect(lastApi().colorId).toBeUndefined());
    act(() => { nav(1); }); act(() => { nav(1); });
    await waitFor(() => expect(lastApi().colorId).toBe('col-black,col-white'));
    expect(entries.filter((e) => e.type === 'REPLACE')).toHaveLength(0);
  });

  it('"Сбросить все" clears colors with the other filters', async () => {
    renderAt('/catalog?colorId=col-black&size=S&sizeSystem=INT&categoryId=c1');
    await waitFor(() => expect(lastApi().colorId).toBe('col-black'));
    fireEvent.click(screen.getAllByText('Сбросить все')[0]);
    await waitFor(() => expect(lastApi().colorId).toBeUndefined());
    expect([...params().keys()]).toEqual([]);
  });

  it('rapid color changes with slow API show only the latest response', async () => {
    const resolvers: Array<() => void> = [];
    vi.mocked(publicCatalog.fetchProducts).mockImplementation((p: any) =>
      new Promise((resolve) => resolvers.push(() => resolve({ items: [{ ...product, id: `p-${p.colorId ?? 'none'}`, name: `Result ${p.colorId ?? 'none'}` } as any], totalCount: 1 }))),
    );
    renderAt('/catalog');
    await waitFor(() => expect(resolvers.length).toBe(1));
    resolvers.splice(0).forEach((r) => r());
    await waitFor(() => expect(screen.getByText('Result none')).toBeTruthy());
    fireEvent.click(color('col-black'));
    fireEvent.click(color('col-white'));
    await waitFor(() => expect(resolvers.length).toBe(2));
    resolvers[1](); resolvers[0]();
    await waitFor(() => expect(screen.getByText('Result col-black,col-white')).toBeTruthy());
    expect(screen.queryByText('Result col-black')).toBeNull();
  });

  it('shows the empty state when nothing matches the selected colors', async () => {
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [], totalCount: 0 });
    renderAt('/catalog?colorId=col-white');
    expect(await screen.findByText('Ничего не найдено')).toBeTruthy();
  });

  it('the catalog still works when the color list cannot be loaded', async () => {
    vi.mocked(publicCatalog.fetchColors).mockRejectedValue(new Error('boom'));
    renderAt('/catalog');
    await waitFor(() => expect(screen.getByText('Colored Product')).toBeTruthy());
    expect(screen.getAllByTestId('color-filter-empty').length).toBeGreaterThan(0);
  });
});
