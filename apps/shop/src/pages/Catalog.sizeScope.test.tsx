/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent } from '@testing-library/react';
import { MemoryRouter, useLocation, useNavigate } from 'react-router-dom';
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
  fetchColors: vi.fn(),
}));

const sys = (code: string, name: string, values: string[]) => ({ code, name, values: values.map((value, sortOrder) => ({ value, sortOrder })) });
const UUID = '8f2c1d3e-4b5a-4c6d-9e7f-0a1b2c3d4e5f';

// dozens of same-named test systems + one real category, one without sizes, one whose system is polluted
const noisy = Array.from({ length: 40 }, (_, i) => ({ id: `noise-${i}`, slug: `n${i}`, name: `Шум ${i}`, icon: '', sizeSystem: sys(`sc-${i}`, 'SC sys', ['S', 'M', 'L']) }));
const categories = [
  { id: 'c-tops', slug: 'tops', name: 'Топы', icon: '', sizeSystem: sys('INT', 'International', ['XS', 'S', 'M', UUID, 'X'.repeat(30)]) },
  { id: 'c-shoes', slug: 'shoes', name: 'Обувь', icon: '', sizeSystem: sys('EU', 'European', ['38', '40']) },
  { id: 'c-bags', slug: 'bags', name: 'Сумки', icon: '' },
  ...noisy,
];
const product = { id: 'p1', name: 'Sized Product', price: 1000, category: 'c-tops', brand: 'B', brandId: 'b1', sellerId: 's', image: 'x.jpg', images: [], isNew: false };

function Probe() {
  const l = useLocation();
  const nav = useNavigate();
  return (
    <>
      <div data-testid="loc">{l.pathname + l.search}</div>
      <button data-testid="history-back" onClick={() => nav(-1)} />
      <button data-testid="history-forward" onClick={() => nav(1)} />
    </>
  );
}
const renderAt = (url: string) => render(<MemoryRouter initialEntries={[url]}><Catalog /><Probe /></MemoryRouter>);
const lastParams = () => (vi.mocked(publicCatalog.fetchProducts).mock.calls.at(-1)?.[0] ?? {}) as Record<string, any>;
const loc = () => decodeURIComponent(screen.getByTestId('loc').textContent || '');
const sizeButtons = () => screen.queryAllByTestId(/^size-filter-(?!empty|prompt)[^-]+.*-.+$/).filter((el) => el.tagName === 'BUTTON');

describe('Catalog size filter scope (CATALOG.C6)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(authContext.useAuth).mockReturnValue({ isAuthenticated: false, user: null } as any);
    vi.mocked(publicCatalog.fetchCategories).mockResolvedValue(categories as any);
    vi.mocked(publicCatalog.fetchProducts).mockResolvedValue({ items: [product as any], totalCount: 1 });
    vi.mocked(publicCatalog.fetchColors).mockResolvedValue([{ id: 'col-black', name: 'Чёрный', hex: '#000000' }] as any);
  });
  afterEach(cleanup);

  it('"All products" with many systems shows a prompt and no size buttons at all', async () => {
    renderAt('/catalog');
    expect((await screen.findAllByTestId('size-filter-prompt')).length).toBeGreaterThan(0);
    expect(screen.getAllByText('Выберите категорию, чтобы увидеть доступные размеры').length).toBeGreaterThan(0);
    expect(sizeButtons()).toHaveLength(0);
    expect(screen.queryByText('SC sys')).toBeNull();
  });

  it('a category shows exactly its own system, without UUID or over-long sizes', async () => {
    renderAt('/catalog?categoryId=c-tops');
    await waitFor(() => expect(screen.getAllByTestId('size-filter-INT-S').length).toBeGreaterThan(0));
    expect(screen.getAllByTestId('size-filter').length).toBeGreaterThan(0);
    const labels = sizeButtons().map((b) => b.textContent);
    expect(new Set(labels)).toEqual(new Set(['XS', 'S', 'M']));
    expect(document.body.textContent).not.toContain(UUID);
    expect(document.body.textContent).not.toContain('X'.repeat(30));
  });

  it('a category with a same-named test system uses its own code in the URL and the API call', async () => {
    renderAt('/catalog?categoryId=noise-7');
    await waitFor(() => expect(screen.getAllByTestId('size-filter-sc-7-M').length).toBeGreaterThan(0));
    fireEvent.click(screen.getAllByTestId('size-filter-sc-7-M')[0]);
    await waitFor(() => expect(lastParams()).toMatchObject({ categoryId: 'noise-7', size: 'M', sizeSystem: 'sc-7' }));
    expect(screen.queryByTestId('size-filter-sc-8-M')).toBeNull();
  });

  it('a category without a size grid explains it and renders no size buttons', async () => {
    renderAt('/catalog?categoryId=c-bags');
    expect((await screen.findAllByTestId('size-filter-empty')).length).toBeGreaterThan(0);
    expect(sizeButtons()).toHaveLength(0);
  });

  it('size + colour + stock go to the API together and live in the URL', async () => {
    renderAt('/catalog?categoryId=c-tops');
    await waitFor(() => expect(screen.getAllByTestId('size-filter-INT-S').length).toBeGreaterThan(0));
    fireEvent.click(screen.getAllByTestId('size-filter-INT-S')[0]);
    fireEvent.click(await screen.findByTestId('color-filter-col-black'));
    fireEvent.click(screen.getAllByText('Только в наличии')[0]);
    await waitFor(() => expect(lastParams()).toMatchObject({ categoryId: 'c-tops', size: 'S', sizeSystem: 'INT', colorId: 'col-black', inStock: 'true' }));
    expect(loc()).toMatch(/size=S/);
    expect(loc()).toMatch(/colorId=col-black/);
    expect(loc()).toMatch(/inStock=true/);
  });

  it('changing the category drops the incompatible size in one history step; Back/Forward restore it', async () => {
    renderAt('/catalog?categoryId=c-tops&size=S&sizeSystem=INT');
    await waitFor(() => expect(lastParams()).toMatchObject({ size: 'S', sizeSystem: 'INT' }));
    fireEvent.click(screen.getAllByText('Обувь')[0]);
    await waitFor(() => expect(loc()).toContain('categoryId=c-shoes'));
    expect(loc()).not.toContain('size=');
    await waitFor(() => expect(lastParams().size).toBeUndefined());

    fireEvent.click(screen.getByTestId('history-back'));
    await waitFor(() => expect(loc()).toContain('categoryId=c-tops'));
    expect(loc()).toContain('size=S');
    await waitFor(() => expect(lastParams()).toMatchObject({ categoryId: 'c-tops', size: 'S', sizeSystem: 'INT' }));

    fireEvent.click(screen.getByTestId('history-forward'));
    await waitFor(() => expect(loc()).toContain('categoryId=c-shoes'));
    expect(loc()).not.toContain('size=');
  });

  it('a size from the URL stays valid under "all products" even though the buttons are hidden behind the prompt', async () => {
    renderAt('/catalog?size=M&sizeSystem=sc-3');
    await waitFor(() => expect(lastParams()).toMatchObject({ size: 'M', sizeSystem: 'sc-3' }));
    expect((await screen.findAllByTestId('size-filter-prompt')).length).toBeGreaterThan(0);
    expect(loc()).toContain('size=M');
    expect(screen.getAllByText('Сбросить размеры').length).toBeGreaterThan(0);
  });

  it('a crafted size system in the URL is still cleared', async () => {
    renderAt('/catalog?size=M&sizeSystem=does-not-exist');
    await waitFor(() => expect(loc()).not.toContain('sizeSystem='));
  });
});
