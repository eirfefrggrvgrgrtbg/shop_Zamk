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
}));

const INT = { code: 'INT', name: 'International', values: ['XS', 'S', 'M', 'L'].map((value, i) => ({ value, sortOrder: i })) };
const EU = { code: 'EU', name: 'European', values: ['38', '40', '42'].map((value, i) => ({ value, sortOrder: i })) };
const categories = [
  { id: 'c-tops', slug: 'tops', name: 'Топы', icon: '', sizeSystem: INT },
  { id: 'c-shoes', slug: 'shoes', name: 'Обувь', icon: '', sizeSystem: EU },
];
const brands = [
  { id: 'b-1', name: 'Brand One', description: '', country: '', image: '' },
  { id: 'b-2', name: 'Brand Two', description: '', country: '', image: '' },
];
const product = { id: 'p1', name: 'Filtered Product', price: 1000, category: 'c-tops', brand: 'B', brandId: 'b-1', sellerId: 's', image: 'x.jpg', images: [], isNew: false };

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
const first = (testId: string) => screen.getAllByTestId(testId)[0];
const clickText = (t: string) => fireEvent.click(screen.getAllByText(t)[0]);
const pushes = () => entries.filter((e) => e.type === 'PUSH').length;
const back = () => act(() => { nav(-1); });
const forward = () => act(() => { nav(1); });

describe('Catalog filters: URL is the single source of truth (CATALOG.C2.1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    entries = [];
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false, user: null } as any);
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue(categories as any);
    vi.mocked(publicCatalog.fetchBrands).mockResolvedValue(brands as any);
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [product as any], totalCount: 1 });
  });
  afterEach(() => { cleanup(); vi.useRealTimers(); });

  it('direct link / reload with a full filter set restores everything and does not touch history', async () => {
    renderAt('/catalog?categoryId=c-tops&brandId=b-1&minPriceCents=100000&maxPriceCents=500000&size=S,M&sizeSystem=INT&sort=price_asc');
    await waitFor(() => expect(lastApi()).toMatchObject({
      categoryId: 'c-tops', brandId: 'b-1', minPriceCents: 100000, maxPriceCents: 500000, size: 'S,M', sizeSystem: 'INT', sort: 'price_asc',
    }));
    await waitFor(() => expect(first('size-filter-INT-M').getAttribute('aria-pressed')).toBe('true'));
    expect((screen.getAllByDisplayValue('1000')[0] as HTMLInputElement).value).toBe('1000');
    expect((screen.getAllByDisplayValue('5000')[0] as HTMLInputElement).value).toBe('5000');
    // no normalising writes at all
    await new Promise((r) => setTimeout(r, 50));
    expect(entries).toHaveLength(1);
    expect(vi.mocked(publicCatalog.fetchProducts)).toHaveBeenCalledTimes(1);
  });

  it('every filter change updates URL and API together, each as exactly one history entry', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(first('size-filter-INT-S')).toBeTruthy());
    clickText('Топы');
    await waitFor(() => expect(params().get('categoryId')).toBe('c-tops'));
    clickText('Brand One');
    await waitFor(() => expect(params().get('brandId')).toBe('b-1'));
    fireEvent.click(first('size-filter-INT-M'));
    await waitFor(() => expect(params().get('size')).toBe('M'));
    fireEvent.click(screen.getAllByText('Только в наличии')[0]);
    await waitFor(() => expect(params().get('inStock')).toBe('true'));
    await waitFor(() => expect(lastApi()).toMatchObject({ categoryId: 'c-tops', brandId: 'b-1', size: 'M', sizeSystem: 'INT', inStock: 'true' }));
    expect(pushes()).toBe(4);
    expect(entries.filter((e) => e.type === 'REPLACE')).toHaveLength(0);
  });

  it('price edits commit one history entry after the user pauses', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(first('size-filter-INT-S')).toBeTruthy());
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const input = screen.getAllByText('От')[0].parentElement!.querySelector('input')!;
    fireEvent.change(input, { target: { value: '1' } });
    fireEvent.change(input, { target: { value: '12' } });
    fireEvent.change(input, { target: { value: '120' } });
    expect(pushes()).toBe(0);
    await act(async () => { vi.advanceTimersByTime(600); });
    expect(params().get('minPriceCents')).toBe('12000');
    expect(pushes()).toBe(1);
  });

  it('several Back / Forward steps restore ALL filters each time', async () => {
    renderAt('/catalog');
    await waitFor(() => expect(first('size-filter-INT-S')).toBeTruthy());
    clickText('Топы');                                   // 1
    await waitFor(() => expect(params().get('categoryId')).toBe('c-tops'));
    clickText('Brand Two');                              // 2
    await waitFor(() => expect(params().get('brandId')).toBe('b-2'));
    fireEvent.click(first('size-filter-INT-L'));         // 3
    await waitFor(() => expect(params().get('size')).toBe('L'));

    back();
    await waitFor(() => expect(lastApi()).toMatchObject({ categoryId: 'c-tops', brandId: 'b-2' }));
    expect(lastApi().size).toBeUndefined();
    expect(first('size-filter-INT-L').getAttribute('aria-pressed')).toBe('false');
    back();
    await waitFor(() => expect(lastApi().brandId).toBeUndefined());
    expect(lastApi().categoryId).toBe('c-tops');
    back();
    await waitFor(() => expect(lastApi().categoryId).toBeUndefined());

    forward(); forward(); forward();
    await waitFor(() => expect(lastApi()).toMatchObject({ categoryId: 'c-tops', brandId: 'b-2', size: 'L', sizeSystem: 'INT' }));
    expect(first('size-filter-INT-L').getAttribute('aria-pressed')).toBe('true');
    expect(entries.filter((e) => e.type === 'REPLACE')).toHaveLength(0);
  });

  it('category change with an incompatible size drops it in the same single history entry', async () => {
    renderAt('/catalog?categoryId=c-tops&size=S&sizeSystem=INT');
    await waitFor(() => expect(lastApi().size).toBe('S'));
    const before = pushes();
    clickText('Обувь');
    await waitFor(() => expect(lastApi().categoryId).toBe('c-shoes'));
    expect(lastApi().size).toBeUndefined();
    expect(params().get('size')).toBeNull();
    expect(params().get('sizeSystem')).toBeNull();
    expect(pushes() - before).toBe(1);
  });

  it('"Все товары" and "Сбросить все" clear category/brand/price/size/stock predictably, keeping sort', async () => {
    renderAt('/catalog?categoryId=c-tops&brandId=b-1&minPriceCents=100000&size=S&sizeSystem=INT&inStock=true&sort=price_asc');
    await waitFor(() => expect(lastApi().size).toBe('S'));
    fireEvent.click(screen.getAllByText('Сбросить все')[0]);
    await waitFor(() => expect(lastApi().categoryId).toBeUndefined());
    expect(lastApi()).toMatchObject({ sort: 'price_asc' });
    expect([...params().keys()]).toEqual(['sort']);
    clickText('Топы');
    await waitFor(() => expect(params().get('categoryId')).toBe('c-tops'));
    clickText('Все товары');
    await waitFor(() => expect(params().get('categoryId')).toBeNull());
  });

  it('rapid filter changes with slow API: only the latest response is shown', async () => {
    const resolvers: Array<() => void> = [];
    vi.mocked(publicCatalog.fetchProducts).mockImplementation((p: any) =>
      new Promise((resolve) => {
        resolvers.push(() => resolve({ items: [{ ...product, id: `p-${p.brandId ?? 'none'}`, name: `Result ${p.brandId ?? 'none'}` } as any], totalCount: 1 }));
      }),
    );
    renderAt('/catalog');
    await waitFor(() => expect(resolvers.length).toBe(1));
    resolvers.splice(0).forEach((r) => r()); // initial load (also loads categories/brands)
    await waitFor(() => expect(screen.getByText('Result none')).toBeTruthy());
    await waitFor(() => expect(screen.getAllByText('Brand One').length).toBeGreaterThan(0));
    clickText('Brand One');
    clickText('Brand Two');
    await waitFor(() => expect(params().get('brandId')).toBe('b-2'));
    await waitFor(() => expect(resolvers.length).toBe(2));
    resolvers[1](); // newest first
    resolvers[0](); // stale response arrives last
    await waitFor(() => expect(screen.getByText('Result b-2')).toBeTruthy());
    expect(screen.queryByText('Result b-1')).toBeNull();
  });

  it('settles without loops: a stable URL causes no writes and no repeated requests', async () => {
    renderAt('/catalog?categoryId=c-tops&size=S&sizeSystem=INT');
    await waitFor(() => expect(lastApi().size).toBe('S'));
    const calls = vi.mocked(publicCatalog.fetchProducts).mock.calls.length;
    await new Promise((r) => setTimeout(r, 150));
    expect(vi.mocked(publicCatalog.fetchProducts).mock.calls.length).toBe(calls);
    expect(entries).toHaveLength(1);
  });

  it('a crafted URL with a size foreign to the category is repaired with a single REPLACE', async () => {
    renderAt('/catalog?categoryId=c-tops&size=42&sizeSystem=EU');
    await waitFor(() => expect(params().get('size')).toBeNull());
    expect(entries.filter((e) => e.type === 'REPLACE')).toHaveLength(1);
    expect(pushes()).toBe(0);
  });
});
