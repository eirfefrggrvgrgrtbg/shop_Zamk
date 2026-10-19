import { describe, it, expect } from 'vitest';
import { formatVelocity, formatDaysOfStock } from './SellerAnalyticsProduct';
import { VariantRow } from '../api/selleranalytics';

describe('SellerAnalyticsProduct SKU formatting', () => {
  describe('formatVelocity', () => {
    it('formats small positive velocity without rounding to literal zero', () => {
      // 1 unit sold in 30 days ≈ 0.0333 units/day
      expect(formatVelocity(1 / 30)).toBe('0,03 шт/дн');
      // 2 units sold in 30 days ≈ 0.0667 units/day
      expect(formatVelocity(2 / 30)).toBe('0,07 шт/дн');
    });

    it('formats tiny velocity below 0.01 with threshold notation', () => {
      expect(formatVelocity(0.005)).toBe('< 0,01 шт/дн');
      expect(formatVelocity(1 / 365)).toBe('< 0,01 шт/дн');
    });

    it('formats normal velocity >= 1 with 1 decimal place and Russian comma', () => {
      expect(formatVelocity(1.0)).toBe('1,0 шт/дн');
      expect(formatVelocity(2.55)).toBe('2,5 шт/дн');
      expect(formatVelocity(10.2)).toBe('10,2 шт/дн');
    });

    it('returns dash for zero, negative or invalid velocity', () => {
      expect(formatVelocity(0)).toBe('—');
      expect(formatVelocity(-1)).toBe('—');
      expect(formatVelocity(NaN)).toBe('—');
      expect(formatVelocity(Infinity)).toBe('—');
    });
  });

  describe('formatDaysOfStock', () => {
    it('returns "Нет в наличии" for zero stock with positive sales instead of "≈0 дн."', () => {
      const variant: VariantRow = {
        variantId: 'v1',
        sku: 'SKU-1',
        displayName: 'M Black',
        unitsSold: 5,
        grossSalesCents: 500000,
        returnedUnits: 0,
        returnRatePercent: 0,
        availableStock: 0,
        salesVelocity: 0.167,
        daysOfStock: 0,
        stockCoverageState: 'out_of_stock',
        cohortDeliveredUnits: 5,
        cohortReturnedUnits: 0,
      };

      expect(formatDaysOfStock(variant)).toBe('Нет в наличии');
    });

    it('returns "Нет в наличии" when availableStock is 0 even if stockCoverageState is empty', () => {
      const variant: VariantRow = {
        variantId: 'v2',
        sku: 'SKU-2',
        displayName: 'L White',
        unitsSold: 1,
        grossSalesCents: 100000,
        returnedUnits: 0,
        returnRatePercent: 0,
        availableStock: 0,
        salesVelocity: 0.033,
        daysOfStock: 0,
        stockCoverageState: '',
        cohortDeliveredUnits: 1,
        cohortReturnedUnits: 0,
      };

      expect(formatDaysOfStock(variant)).toBe('Нет в наличии');
    });

    it('returns "Нет продаж за период" for positive stock with zero sales in the period', () => {
      const variant: VariantRow = {
        variantId: 'v3',
        sku: 'SKU-3',
        displayName: 'S Red',
        unitsSold: 0,
        grossSalesCents: 0,
        returnedUnits: 0,
        returnRatePercent: null,
        availableStock: 15,
        salesVelocity: 0,
        daysOfStock: null,
        stockCoverageState: 'no_sales',
        cohortDeliveredUnits: 0,
        cohortReturnedUnits: 0,
      };

      expect(formatDaysOfStock(variant)).toBe('Нет продаж за период');
    });

    it('renders approximate days of stock for normal positive sales and stock', () => {
      const variant: VariantRow = {
        variantId: 'v4',
        sku: 'SKU-4',
        displayName: 'XL Blue',
        unitsSold: 10,
        grossSalesCents: 1000000,
        returnedUnits: 0,
        returnRatePercent: 0,
        availableStock: 25,
        salesVelocity: 0.333,
        daysOfStock: 75.075,
        stockCoverageState: '',
        cohortDeliveredUnits: 10,
        cohortReturnedUnits: 0,
      };

      expect(formatDaysOfStock(variant)).toBe('≈ 75 дн.');
    });
  });
});
