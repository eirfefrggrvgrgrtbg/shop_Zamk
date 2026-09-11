/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerInventory } from './SellerInventory';
import { getSellerInventory } from '@zamk/api-client/src/seller';
import type { SellerInventoryItem } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerInventory: vi.fn(),
}));

describe('SellerInventory - Forecast Read Model and UX', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const baseItem: SellerInventoryItem = {
    variantId: 'var-1',
    productId: 'prod-1',
    productTitle: 'Платье шелковое',
    sku: 'SKU-001',
    onHand: 10,
    reserved: 2,
    available: 8,
    inbound: 5,
    availabilityStatus: 'В наличии',
    optionValues: { Цвет: 'Черный', Размер: 'M' },
  };

  it('renders "Мало данных" for insufficient_data state', async () => {
    const item: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-insufficient',
      forecast: {
        state: 'insufficient_data',
        daysOfCover: null,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-insufficient')).toBeTruthy();
    });

    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).toBe('Мало данных');
  });

  it('renders "Нет продаж" for no_sales state', async () => {
    const item: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-nosales',
      forecast: {
        state: 'no_sales',
        daysOfCover: null,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-nosales')).toBeTruthy();
    });

    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).toBe('Нет продаж');
  });

  it('renders "≈ 21 день запаса" for healthy state (21.2 days)', async () => {
    const item: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-healthy',
      forecast: {
        state: 'healthy',
        daysOfCover: 21.2,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-healthy')).toBeTruthy();
    });

    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).toBe('≈ 21 день запаса');
  });

  it('renders "≈ 9 дней запаса" for warning state (9.1 days) with replenishment link', async () => {
    const item: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-warning',
      forecast: {
        state: 'warning',
        daysOfCover: 9.1,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-warning')).toBeTruthy();
    });

    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).toBe('≈ 9 дней запаса');
    const link = forecastCell.querySelector('a');
    expect(link).not.toBeNull();
    expect(link?.getAttribute('href')).toBe('/supplies/new');
    expect(link?.className).toContain('text-amber-600');
  });

  it('renders "≈ 4 дня запаса" for critical state (4.2 days) with replenishment link', async () => {
    const item: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-critical',
      forecast: {
        state: 'critical',
        daysOfCover: 4.2,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-critical')).toBeTruthy();
    });

    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).toBe('≈ 4 дня запаса');
    const link = forecastCell.querySelector('a');
    expect(link).not.toBeNull();
    expect(link?.getAttribute('href')).toBe('/supplies/new');
    expect(link?.className).toContain('text-red-600');
  });

  it('FAIL 1 Regression: preserves forecast visibility when available < 2 without replacing with "-"', async () => {
    const outOfStockItem: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-lowstock',
      available: 0,
      onHand: 2,
      reserved: 2,
      availabilityStatus: 'Нет в наличии',
      forecast: {
        state: 'critical',
        daysOfCover: 4.2,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [outOfStockItem],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-lowstock')).toBeTruthy();
    });

    // Existing stock status remains visible as primary status
    const statusBadge = screen.getByTestId('status-badge');
    expect(statusBadge.textContent).toBe('Нет в наличии');

    // Forecast MUST NOT be hidden or replaced with "-"
    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).not.toBe('-');
    expect(forecastCell.textContent).toBe('≈ 4 дня запаса');
  });

  it('renders safely when forecast is missing / null', async () => {
    const itemWithoutForecast: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-nofc',
      forecast: undefined,
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [itemWithoutForecast],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-nofc')).toBeTruthy();
    });

    const forecastCell = screen.getByTestId('forecast-value');
    expect(forecastCell.textContent).toBe('-');
  });

  it('preserves existing inventory columns and values unchanged', async () => {
    const item: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-full',
      onHand: 42,
      reserved: 7,
      available: 35,
      inbound: 12,
      availabilityStatus: 'В наличии',
      forecast: {
        state: 'healthy',
        daysOfCover: 15.0,
        calculatedAt: '2026-09-11T12:00:00Z',
      },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-full')).toBeTruthy();
    });

    expect(screen.getByText('Платье шелковое')).toBeTruthy();
    expect(screen.getByText('Черный · M')).toBeTruthy();
    expect(screen.getByText('SKU-001')).toBeTruthy();
    expect(screen.getByTestId('onhand-value').textContent).toBe('42');
    expect(screen.getByTestId('reserved-value').textContent).toBe('7');
    expect(screen.getByTestId('available-value').textContent).toBe('35');
    expect(screen.getByTestId('inbound-value').textContent).toBe('+12');
    expect(screen.getByTestId('status-badge').textContent).toBe('В наличии');
    expect(screen.getByTestId('forecast-value').textContent).toBe('≈ 15 дней запаса');
  });

  it('renders canonical variant label (Color · Size) and distinguishes multiple variants of same product', async () => {
    const item1: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-red-l',
      productTitle: 'худи',
      sku: 'SKU-4444-645916',
      optionValues: { Цвет: 'Красный', Размер: 'L' },
    };
    const item2: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-grey-xl',
      productTitle: 'худи',
      sku: 'SKU-4444-645917',
      optionValues: { Цвет: 'Серый', Размер: 'XL' },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [item1, item2],
      totalCount: 2,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-red-l')).toBeTruthy();
      expect(screen.getByTestId('inventory-row-var-grey-xl')).toBeTruthy();
    });

    const row1 = screen.getByTestId('inventory-row-var-red-l');
    expect(row1.textContent).toContain('худи');
    expect(row1.textContent).toContain('Красный · L');
    expect(row1.textContent).toContain('SKU-4444-645916');

    const row2 = screen.getByTestId('inventory-row-var-grey-xl');
    expect(row2.textContent).toContain('худи');
    expect(row2.textContent).toContain('Серый · XL');
    expect(row2.textContent).toContain('SKU-4444-645917');
  });

  it('renders "—" when SKU is empty or null', async () => {
    const itemWithoutSKU: SellerInventoryItem = {
      ...baseItem,
      variantId: 'var-no-sku',
      sku: '',
      optionValues: { Цвет: 'Красный' },
    };

    vi.mocked(getSellerInventory).mockResolvedValueOnce({
      items: [itemWithoutSKU],
      totalCount: 1,
    });

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-var-no-sku')).toBeTruthy();
    });

    const row = screen.getByTestId('inventory-row-var-no-sku');
    expect(row.textContent).toContain('—');
    expect(row.textContent).toContain('Красный');
  });
});
