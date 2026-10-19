import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { InventoryTab, formatDaysOfStock, formatVelocity } from './InventoryTab';
import * as api from '../../api/selleranalytics';

vi.mock('../../api/selleranalytics', () => ({
  getAnalyticsInventory: vi.fn(),
}));

describe('InventoryTab Component', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // 1. Both "Мало на складе" and "Нет продаж за период" visible for the same variant when applicable
  it('1. Both "Мало на складе" and "Нет продаж за период" visible for the same variant when applicable', async () => {
    const mockData = {
      items: [
        {
          productId: 'prod-1',
          variantId: 'var-1',
          productTitle: 'Брюки шелковые',
          sku: 'PNT-SLK-01',
          displayName: 'M Черный',
          available: 2,
          onHand: 2,
          reserved: 0,
          inbound: 0,
          unitsSold: 0,
          salesVelocity: 0,
          daysOfStock: null,
          stockCoverageState: 'no_sales',
          isPhysicalLowStock: true,
          hasNoSalesInPeriod: true,
          isForecastLowCoverage: false,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Брюки шелковые')).toBeTruthy();
    expect(screen.getByText('Мало на складе')).toBeTruthy();
    expect(screen.getByText('Нет продаж за период')).toBeTruthy();
  });

  // 2. Physical low-stock summary count includes no-sales variants with 1–10 available units
  it('2. Physical low-stock summary count includes no-sales variants with 1–10 available units', async () => {
    const mockData = {
      items: [
        {
          productId: 'p-1',
          variantId: 'v-1',
          productTitle: 'Платье 1',
          sku: 'SKU-1',
          displayName: 'S',
          available: 2, // low stock + no sales
          onHand: 2,
          reserved: 0,
          inbound: 0,
          unitsSold: 0,
          salesVelocity: 0,
          daysOfStock: null,
          stockCoverageState: 'no_sales',
          isPhysicalLowStock: true,
          hasNoSalesInPeriod: true,
          isForecastLowCoverage: false,
        },
        {
          productId: 'p-2',
          variantId: 'v-2',
          productTitle: 'Платье 2',
          sku: 'SKU-2',
          displayName: 'M',
          available: 25, // sufficient stock + no sales
          onHand: 25,
          reserved: 0,
          inbound: 0,
          unitsSold: 0,
          salesVelocity: 0,
          daysOfStock: null,
          stockCoverageState: 'no_sales',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: true,
          isForecastLowCoverage: false,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Платье 1')).toBeTruthy();
    // Low stock card must count 1 (v-1) even though it has no sales!
    // Both cards exist:
    const lowStockCard = screen.getByTitle('Товары с физическим доступным остатком от 1 до 10 шт. на складе');
    expect(lowStockCard.textContent).toContain('1');
    const noSalesCard = screen.getByTitle('Товары с доступным остатком, по которым не было доставленных продаж за период');
    expect(noSalesCard.textContent).toContain('2');
  });

  // 3. Out-of-stock rows are not counted as merely low-stock
  it('3. Out-of-stock rows are not counted as merely low-stock', async () => {
    const mockData = {
      items: [
        {
          productId: 'p-1',
          variantId: 'v-zero',
          productTitle: 'Топ базовый',
          sku: 'TOP-0',
          displayName: 'XS',
          available: 0,
          onHand: 0,
          reserved: 0,
          inbound: 5,
          unitsSold: 0,
          salesVelocity: 0,
          daysOfStock: null,
          stockCoverageState: 'out_of_stock',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: true,
          isForecastLowCoverage: false,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Топ базовый')).toBeTruthy();
    const outCard = screen.getByTitle('Товары с нулевым доступным остатком на складе');
    expect(outCard.textContent).toContain('1');
    const lowStockCard = screen.getByTitle('Товары с физическим доступным остатком от 1 до 10 шт. на складе');
    expect(lowStockCard.textContent).toContain('0');
    expect(screen.queryByText('Мало на складе')).toBeNull();
    expect(screen.getAllByText('Нет в наличии').length).toBeGreaterThanOrEqual(1);
  });

  // 4. Forecast warning visible for high-stock, high-velocity items
  it('4. Forecast warning visible for high-stock, high-velocity items', async () => {
    const mockData = {
      items: [
        {
          productId: 'p-fast',
          variantId: 'v-fast',
          productTitle: 'Хит продаж худи',
          sku: 'HUD-FAST',
          displayName: 'L Черный',
          available: 30, // > 10, so not physical low stock!
          onHand: 30,
          reserved: 0,
          inbound: 0,
          unitsSold: 1500,
          salesVelocity: 50.0,
          daysOfStock: 0.6,
          stockCoverageState: '',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: false,
          isForecastLowCoverage: true,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Хит продаж худи')).toBeTruthy();
    expect(screen.getByText('Запас < 7 дн.')).toBeTruthy();
    expect(screen.queryByText('Мало на складе')).toBeNull();
    expect(screen.getByText('< 1 дн.')).toBeTruthy();
  });

  // 5. Simultaneous low-stock and urgent-forecast warnings
  it('5. Simultaneous low-stock and urgent-forecast warnings', async () => {
    const mockData = {
      items: [
        {
          productId: 'p-crit',
          variantId: 'v-crit',
          productTitle: 'Юбка плиссе',
          sku: 'SKU-CRIT',
          displayName: 'S Серый',
          available: 8,
          onHand: 8,
          reserved: 0,
          inbound: 0,
          unitsSold: 48,
          salesVelocity: 1.6,
          daysOfStock: 5.0,
          stockCoverageState: '',
          isPhysicalLowStock: true,
          hasNoSalesInPeriod: false,
          isForecastLowCoverage: true,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Юбка плиссе')).toBeTruthy();
    expect(screen.getByText('Мало на складе')).toBeTruthy();
    expect(screen.getByText('Запас < 7 дн.')).toBeTruthy();
    expect(screen.getByText('≈ 5 дн.')).toBeTruthy();
  });

  // 6. Empty/no-sales forecast shows an honest unavailable state
  it('6. Empty/no-sales forecast shows an honest unavailable state', async () => {
    const mockData = {
      items: [
        {
          productId: 'p-idle',
          variantId: 'v-idle',
          productTitle: 'Куртка демисезонная',
          sku: 'JKT-IDLE',
          displayName: 'XL Синий',
          available: 20,
          onHand: 20,
          reserved: 0,
          inbound: 0,
          unitsSold: 0,
          salesVelocity: 0,
          daysOfStock: null,
          stockCoverageState: 'no_sales',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: true,
          isForecastLowCoverage: false,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Куртка демисезонная')).toBeTruthy();
    const cells = screen.getAllByText('—');
    expect(cells.length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText(/≈ 0 дн\./)).toBeNull();
  });

  // 7. Current stock quantities and inbound counts remain unchanged
  it('7. Current stock quantities and inbound counts remain unchanged', async () => {
    const mockData = {
      items: [
        {
          productId: 'p-stock',
          variantId: 'v-stock',
          productTitle: 'Кардиган шерстяной',
          sku: 'CRD-01',
          displayName: 'M Бежевый',
          available: 7,
          onHand: 10,
          reserved: 3,
          inbound: 19,
          unitsSold: 2,
          salesVelocity: 0.07,
          daysOfStock: 100,
          stockCoverageState: '',
          isPhysicalLowStock: true,
          hasNoSalesInPeriod: false,
          isForecastLowCoverage: false,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Кардиган шерстяной')).toBeTruthy();
    expect(screen.getByText('7 шт.')).toBeTruthy();
    expect(screen.getByText('10 шт.')).toBeTruthy();
    expect(screen.getByText('3 шт.')).toBeTruthy();
    expect(screen.getByText('+19 шт.')).toBeTruthy();
    expect(screen.getByText('Мало на складе')).toBeTruthy();
  });

  // 8. Human-readable product and variant metadata remain visible
  it('8. Human-readable product and variant metadata remain visible', async () => {
    const mockData = {
      items: [
        {
          productId: 'prod-meta',
          variantId: 'var-meta',
          productTitle: 'Тренч классический ZAMK',
          sku: 'TRN-BEG-42',
          displayName: '42 Бежевый',
          available: 15,
          onHand: 15,
          reserved: 0,
          inbound: 0,
          unitsSold: 3,
          salesVelocity: 0.1,
          daysOfStock: 150,
          stockCoverageState: '',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: false,
          isForecastLowCoverage: false,
        },
      ],
    };
    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Тренч классический ZAMK')).toBeTruthy();
    expect(screen.getByText('42 Бежевый')).toBeTruthy();
    expect(screen.getByText('TRN-BEG-42')).toBeTruthy();
    expect(screen.getByText('Нормальный запас')).toBeTruthy();
  });

  // 9. Loading and error states remain functional
  it('9. Loading and error states remain functional', async () => {
    (api.getAnalyticsInventory as any).mockRejectedValue(new Error('Сетевая ошибка'));

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Не удалось загрузить данные по остаткам')).toBeTruthy();
    expect(screen.getByText('Сетевая ошибка')).toBeTruthy();
  });

  // 10. Russian tooltip explanations are accurate
  it('10. Russian tooltip explanations are accurate', async () => {
    (api.getAnalyticsInventory as any).mockResolvedValue({ items: [] });

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Нет в наличии')).toBeTruthy();
    expect(screen.getByTitle('Товары с физическим доступным остатком от 1 до 10 шт. на складе')).toBeTruthy();
    expect(screen.getByTitle('Товары с доступным остатком, по которым не было доставленных продаж за период')).toBeTruthy();
    expect(screen.getByTitle('Товары с нулевым доступным остатком на складе')).toBeTruthy();
  });

  // 11. No NaN, Infinity or misleading zero-day forecast
  it('11. No NaN, Infinity or misleading zero-day forecast', () => {
    // formatDaysOfStock helper checks
    expect(formatDaysOfStock({ available: 0, unitsSold: 0, salesVelocity: 0, daysOfStock: 0 } as any)).toBe('—');
    expect(formatDaysOfStock({ available: 10, unitsSold: 0, salesVelocity: 0, daysOfStock: null } as any)).toBe('—');
    expect(formatDaysOfStock({ available: 10, unitsSold: 1, salesVelocity: 0.1, daysOfStock: NaN } as any)).toBe('—');
    expect(formatDaysOfStock({ available: 10, unitsSold: 1, salesVelocity: 0.1, daysOfStock: Infinity } as any)).toBe('—');
    expect(formatDaysOfStock({ available: 30, unitsSold: 1500, salesVelocity: 50, daysOfStock: 0.4 } as any)).toBe('< 1 дн.');
    expect(formatDaysOfStock({ available: 30, unitsSold: 10, salesVelocity: 0.33, daysOfStock: 90.9 } as any)).toBe('≈ 91 дн.');

    // formatVelocity helper checks
    expect(formatVelocity(0)).toBe('—');
    expect(formatVelocity(NaN)).toBe('—');
    expect(formatVelocity(0.005)).toBe('< 0,01 шт/дн');
    expect(formatVelocity(0.03)).toBe('0,03 шт/дн');
    expect(formatVelocity(1.5)).toBe('1,5 шт/дн');
  });

  // 12. Existing Seller Analytics rendering regressions PASS
  it('12. Existing Seller Analytics rendering regressions PASS', async () => {
    const mockData = {
      items: [
        {
          productId: 'prod-1',
          variantId: 'var-1',
          productTitle: 'Платье миди шелковое',
          sku: 'DRS-SLK-BLK-M',
          displayName: 'M Черный',
          available: 23,
          onHand: 25,
          reserved: 2,
          inbound: 19,
          unitsSold: 5,
          salesVelocity: 0.17,
          daysOfStock: 135.3,
          stockCoverageState: '',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: false,
          isForecastLowCoverage: false,
        },
        {
          productId: 'prod-2',
          variantId: 'var-2',
          productTitle: 'Брюки классические',
          sku: 'PNT-CLS-WHT-L',
          displayName: 'L Белый',
          available: 2,
          onHand: 2,
          reserved: 0,
          inbound: 0,
          unitsSold: 0,
          salesVelocity: 0,
          daysOfStock: null,
          stockCoverageState: 'no_sales',
          isPhysicalLowStock: true,
          hasNoSalesInPeriod: true,
          isForecastLowCoverage: false,
        },
        {
          productId: 'prod-3',
          variantId: 'var-3',
          productTitle: 'Шарф кашемировый',
          sku: 'SKU-FALLBACK',
          displayName: '',
          available: 0,
          onHand: 0,
          reserved: 0,
          inbound: 5,
          unitsSold: 1,
          salesVelocity: 0.03,
          daysOfStock: null,
          stockCoverageState: 'out_of_stock',
          isPhysicalLowStock: false,
          hasNoSalesInPeriod: false,
          isForecastLowCoverage: false,
        },
      ],
    };

    (api.getAnalyticsInventory as any).mockResolvedValue(mockData);

    render(
      <MemoryRouter>
        <InventoryTab from="2026-09-01T00:00:00Z" to="2026-10-01T00:00:00Z" />
      </MemoryRouter>
    );

    expect(await screen.findByText('Платье миди шелковое')).toBeTruthy();
    expect(screen.getByText('M Черный')).toBeTruthy();
    expect(screen.getByText('DRS-SLK-BLK-M')).toBeTruthy();
    expect(screen.getByText('Нормальный запас')).toBeTruthy();
    expect(screen.getByText('≈ 135 дн.')).toBeTruthy();

    expect(screen.getByText('Брюки классические')).toBeTruthy();
    expect(screen.getByText('L Белый')).toBeTruthy();
    expect(screen.getByText('PNT-CLS-WHT-L')).toBeTruthy();
    expect(screen.getByText('Нет продаж за период')).toBeTruthy();
    expect(screen.getByText('Мало на складе')).toBeTruthy();

    expect(screen.getByText('Шарф кашемировый')).toBeTruthy();
    expect(screen.getByText('SKU-FALLBACK')).toBeTruthy();
    expect(screen.getAllByText('Нет в наличии').length).toBeGreaterThanOrEqual(1);

    expect(screen.getByText('25 шт.')).toBeTruthy();
    expect(screen.getByText('+24')).toBeTruthy();
  });
});
