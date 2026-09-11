/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerInventory } from './SellerInventory';
import { request } from '@zamk/api-client/src/client';

vi.mock('@zamk/api-client/src/client', () => ({
  request: vi.fn(),
}));

describe('SellerInventory - Wire JSON to UI Pipeline Test', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('preserves forecast end-to-end from raw backend JSON response through api-client to UI', async () => {
    // Exact wire JSON payload as emitted by the Go backend HTTP handler
    const rawBackendResponse = {
      items: [
        {
          variantId: '2d262098-a7ce-4f82-b6d4-c93639f46d98',
          productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
          productTitle: 'худи',
          sku: 'SKU-HOODIE-RED-L',
          optionValues: {
            Размер: 'L',
            Цвет: 'Красный',
          },
          onHand: 3,
          reserved: 2,
          available: 1,
          inbound: 0,
          availabilityStatus: 'Заканчивается',
          forecast: {
            state: 'warning',
            daysOfCover: 8.5,
            calculatedAt: '2026-09-11T13:01:26.999353+03:00',
          },
        },
        {
          variantId: '6d2a446e-b3a8-49c6-b703-d67d44f43968',
          productId: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
          productTitle: 'худи',
          sku: 'SKU-HOODIE-BLUE-M',
          optionValues: {
            Размер: 'M',
            Цвет: 'Синий',
          },
          onHand: 0,
          reserved: 0,
          available: 0,
          inbound: 0,
          availabilityStatus: 'Нет в наличии',
          forecast: {
            state: 'no_sales',
            daysOfCover: null,
            calculatedAt: '2026-09-11T13:01:26.999353+03:00',
          },
        },
        {
          variantId: '3b37fd2c-40d7-364b-892d-5ef4e3905afd',
          productId: '2a6fa985-dae0-39eb-ad87-253f982e84f1',
          productTitle: 'Dev Wool Coat',
          sku: 'DEV-SKU-0',
          optionValues: {
            Размер: 'XL',
            Цвет: 'Графит',
          },
          onHand: 22,
          reserved: 0,
          available: 22,
          inbound: 0,
          availabilityStatus: 'В наличии',
          forecast: {
            state: 'healthy',
            daysOfCover: 34,
            calculatedAt: '2026-09-11T13:01:26.999353+03:00',
          },
        },
        {
          variantId: 'var-without-forecast',
          productId: 'prod-legacy',
          productTitle: 'Legacy Item',
          sku: 'LEGACY-001',
          onHand: 5,
          reserved: 0,
          available: 5,
          inbound: 0,
          availabilityStatus: 'В наличии',
        },
      ],
      totalCount: 4,
    };

    // Mock the wire request call
    vi.mocked(request).mockResolvedValueOnce(rawBackendResponse);

    render(
      <MemoryRouter>
        <SellerInventory />
      </MemoryRouter>
    );

    // Wait for rows to load
    await waitFor(() => {
      expect(screen.getByTestId('inventory-row-2d262098-a7ce-4f82-b6d4-c93639f46d98')).toBeTruthy();
    });

    // Row 1: warning (8.5 days) -> ≈ 9 дней запаса with link to /supplies/new + Красный · L + SKU
    const row1 = screen.getByTestId('inventory-row-2d262098-a7ce-4f82-b6d4-c93639f46d98');
    expect(row1.textContent).toContain('худи');
    expect(row1.textContent).toContain('Красный · L');
    expect(row1.textContent).toContain('SKU-HOODIE-RED-L');
    const forecastCell1 = row1.querySelector('[data-testid="forecast-value"]');
    expect(forecastCell1?.textContent).toBe('≈ 9 дней запаса');
    const link1 = forecastCell1?.querySelector('a');
    expect(link1?.getAttribute('href')).toBe('/supplies/new');
    expect(link1?.className).toContain('text-amber-600');

    // Row 2: no_sales -> Нет продаж + Синий · M
    const row2 = screen.getByTestId('inventory-row-6d2a446e-b3a8-49c6-b703-d67d44f43968');
    expect(row2.textContent).toContain('худи');
    expect(row2.textContent).toContain('Синий · M');
    const forecastCell2 = row2.querySelector('[data-testid="forecast-value"]');
    expect(forecastCell2?.textContent).toBe('Нет продаж');

    // Row 3: healthy (34 days) -> ≈ 34 дня запаса + Графит · XL
    const row3 = screen.getByTestId('inventory-row-3b37fd2c-40d7-364b-892d-5ef4e3905afd');
    expect(row3.textContent).toContain('Dev Wool Coat');
    expect(row3.textContent).toContain('Графит · XL');
    const forecastCell3 = row3.querySelector('[data-testid="forecast-value"]');
    expect(forecastCell3?.textContent).toBe('≈ 34 дня запаса');

    // Row 4: no forecast -> -
    const row4 = screen.getByTestId('inventory-row-var-without-forecast');
    const forecastCell4 = row4.querySelector('[data-testid="forecast-value"]');
    expect(forecastCell4?.textContent).toBe('-');
  });
});
