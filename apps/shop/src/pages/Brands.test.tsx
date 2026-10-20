/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Brands } from './Brands';
import type { Brand } from '../types/catalog';


const favState = vi.hoisted(() => ({
  ids: [] as string[],
  toggle: vi.fn(async (_id: string) => true),
  user: { id: 'u-1' } as { id: string } | null,
}));
vi.mock('../contexts/FavoriteBrandsContext', () => ({
  useFavoriteBrands: () => ({
    isFavoriteBrand: (id: string) => favState.ids.includes(id),
    toggleFavoriteBrand: favState.toggle,
    pendingIds: [],
  }),
}));
vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({ user: favState.user }),
}));

const mockFetchBrands = vi.fn();
vi.mock('../api/publicCatalog', () => ({
  fetchBrands: () => mockFetchBrands(),
}));

const sampleBrands: Brand[] = [
  {
    id: 'b-1',
    name: 'Acne Studios',
    slug: 'acne-studios',
    description: 'Stockholm-based multidisciplinary luxury fashion house.',
    image: 'https://cdn.zamk.test/acne-logo.png',
    country: 'SE',
    isActive: true,
  },
  {
    id: 'b-2',
    name: 'Balenciaga Archive',
    slug: 'balenciaga-archive',
    description: 'Couture heritage meets radical modern tailoring.',
    image: '',
    country: 'FR',
    isActive: true,
  },
  {
    id: 'b-3',
    name: 'Raf Simons',
    slug: 'raf-simons',
    description: '',
    image: '',
    country: 'BE',
    isActive: true,
  },
  {
    id: 'b-4',
    name: 'Волчок',
    slug: 'volchok',
    description: 'Российский стритвир с мрачной эстетикой.',
    image: 'https://cdn.zamk.test/volchok-logo.png',
    country: 'RU',
    isActive: true,
  },
  {
    id: 'b-5',
    name: '032c Workshop',
    slug: '032c-workshop',
    description: 'Berlin apparel and culture research platform.',
    image: '',
    country: 'DE',
    isActive: true,
  },
];

const renderBrands = () =>
  render(
    <MemoryRouter initialEntries={['/brands']}>
      <Brands />
    </MemoryRouter>,
  );

describe('Brands catalog page (MARKETING.M1.9)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    favState.ids = [];
    favState.toggle.mockImplementation(async () => true);
  });

  afterEach(() => {
    cleanup();
  });

  it('1. Renders loading state and then brand cards with archive-core details', async () => {
    mockFetchBrands.mockResolvedValue(sampleBrands);

    renderBrands();

    expect(screen.getByText('Загрузка брендов...')).toBeDefined();

    expect(await screen.findByRole('heading', { level: 1, name: 'Бренды' })).toBeDefined();

    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.getByText('Balenciaga Archive')).toBeDefined();
    expect(screen.getByText('Raf Simons')).toBeDefined();
    expect(screen.getByText('Волчок')).toBeDefined();
    expect(screen.getByText('032c Workshop')).toBeDefined();

    const acneCard = screen.getByTestId('brand-card-b-1');
    expect(acneCard.getAttribute('href')).toBe('/brand/acne-studios');
    const acneImg = acneCard.querySelector('img');
    expect(acneImg?.getAttribute('src')).toBe('https://cdn.zamk.test/acne-logo.png');

    const balenciagaCard = screen.getByTestId('brand-card-b-2');
    expect(balenciagaCard.querySelector('img')).toBeNull();
    expect(balenciagaCard.textContent).toContain('B');

    const rafCard = screen.getByTestId('brand-card-b-3');
    expect(rafCard.textContent).not.toContain('нет описания');
  });

  it('2. Shows friendly message when brand catalog is completely empty', async () => {
    mockFetchBrands.mockResolvedValue([]);

    renderBrands();

    expect(await screen.findByText('Каталог брендов пуст')).toBeDefined();
    expect(screen.getByText(/В данный момент активные бренды не опубликованы/i)).toBeDefined();
  });

  it('3. Displays error message if backend fetch fails', async () => {
    mockFetchBrands.mockRejectedValue(new Error('Network error'));

    renderBrands();

    expect(await screen.findByText(/Не удалось загрузить бренды/i)).toBeDefined();
  });

  it('4. Searches brands by name in Latin and Cyrillic (case-insensitive)', async () => {
    mockFetchBrands.mockResolvedValue(sampleBrands);

    renderBrands();

    await screen.findByText('Acne Studios');

    const searchInput = screen.getByPlaceholderText('Поиск по названию бренда...');

    fireEvent.change(searchInput, { target: { value: 'acne' } });
    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.queryByText('Balenciaga Archive')).toBeNull();
    expect(screen.queryByText('Волчок')).toBeNull();

    fireEvent.change(searchInput, { target: { value: 'ВОЛЧОК' } });
    expect(screen.queryByText('Acne Studios')).toBeNull();
    expect(screen.getByText('Волчок')).toBeDefined();

    const clearBtn = screen.getByLabelText('Очистить поиск');
    fireEvent.click(clearBtn);
    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.getByText('Волчок')).toBeDefined();
  });

  it('5. Filters brands via alphabet navigation and deactivates unused letters', async () => {
    mockFetchBrands.mockResolvedValue(sampleBrands);

    renderBrands();

    await screen.findByText('Acne Studios');

    const allBtn = screen.getByRole('button', { name: 'Все' });
    expect(allBtn.getAttribute('aria-pressed')).toBe('true');

    const aBtn = screen.getByRole('button', { name: 'A' }) as HTMLButtonElement;
    expect(aBtn.disabled).toBe(false);

    const zBtn = screen.getByRole('button', { name: 'Z' }) as HTMLButtonElement;
    expect(zBtn.disabled).toBe(true);

    const vCyrillicBtn = screen.getByRole('button', { name: 'В' }) as HTMLButtonElement;
    expect(vCyrillicBtn.disabled).toBe(false);

    const hashBtn = screen.getByRole('button', { name: '#' }) as HTMLButtonElement;
    expect(hashBtn.disabled).toBe(false);

    fireEvent.click(aBtn);
    expect(aBtn.getAttribute('aria-pressed')).toBe('true');
    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.queryByText('Balenciaga Archive')).toBeNull();
    expect(screen.queryByText('Волчок')).toBeNull();

    fireEvent.click(hashBtn);
    expect(screen.getByText('032c Workshop')).toBeDefined();
    expect(screen.queryByText('Acne Studios')).toBeNull();

    fireEvent.click(vCyrillicBtn);
    expect(screen.getByText('Волчок')).toBeDefined();
    expect(screen.queryByText('032c Workshop')).toBeNull();

    fireEvent.click(vCyrillicBtn);
    expect(screen.getByText('Acne Studios')).toBeDefined();
    expect(screen.getByText('Волчок')).toBeDefined();
  });

  it('6. Shows empty search state and resets filters on button click', async () => {
    mockFetchBrands.mockResolvedValue(sampleBrands);

    renderBrands();

    await screen.findByText('Acne Studios');

    const searchInput = screen.getByPlaceholderText('Поиск по названию бренда...');
    fireEvent.change(searchInput, { target: { value: 'NonexistentBrand' } });

    expect(screen.getByText('Ничего не найдено')).toBeDefined();
    expect(screen.getByText(/По запросу «NonexistentBrand» подходящих брендов не найдено/i)).toBeDefined();

    const resetBtns = screen.getAllByRole('button', { name: /Сбросить фильтры/i });
    expect(resetBtns.length).toBeGreaterThan(0);
    fireEvent.click(resetBtns[0]);

    await waitFor(() => {
      expect(screen.getByText('Acne Studios')).toBeDefined();
    });
    expect(screen.getByText('Волчок')).toBeDefined();
  });

  it('favorite icon toggles by brand id without navigating, and reflects saved state', async () => {
    favState.ids = ['b-2'];
    mockFetchBrands.mockResolvedValue(sampleBrands);
    render(
      <MemoryRouter>
        <Brands />
      </MemoryRouter>,
    );
    const saved = await screen.findByTestId('brand-favorite-b-2');
    expect(saved.getAttribute('aria-pressed')).toBe('true');
    const unsaved = screen.getByTestId('brand-favorite-b-1');
    expect(unsaved.getAttribute('aria-pressed')).toBe('false');
    fireEvent.click(unsaved);
    await waitFor(() => expect(favState.toggle).toHaveBeenCalledWith('b-1'));
  });
});
