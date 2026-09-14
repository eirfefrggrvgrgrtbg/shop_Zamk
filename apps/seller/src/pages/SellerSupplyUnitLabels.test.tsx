/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { SellerSupplyUnitLabels } from './SellerSupplyUnitLabels';
import * as apiClient from '@zamk/api-client';
import type { SellerSupplyUnitLabelsResponse } from '@zamk/api-client';

vi.mock('@zamk/api-client', async () => {
  const actual = await vi.importActual<any>('@zamk/api-client');
  return {
    ...actual,
    getSellerSupplyUnitLabels: vi.fn(),
  };
});

describe('SellerSupplyUnitLabels Component (SUP.2A1)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockMultiVariantResponse: SellerSupplyUnitLabelsResponse = {
    supplyId: 'sup-201',
    supplyNumber: 'SUP-2026-000201',
    serialized: true,
    totalUnits: 5,
    units: [
      {
        inventoryUnitId: 'u-1',
        unitCode: 'ZMU-PANTS-BLUE-L-01',
        unitIndex: 1,
        supplyItemId: 'item-1',
        productVariantId: 'var-pants-blue-l',
        productTitle: 'Брюки карго',
        colorName: 'Синий',
        sizeName: 'L',
        sellerSku: 'SKU-PANTS-BL-L',
        variantBarcode: 'ZMK-PANTS-01',
      },
      {
        inventoryUnitId: 'u-2',
        unitCode: 'ZMU-PANTS-BLUE-L-02',
        unitIndex: 2,
        supplyItemId: 'item-1',
        productVariantId: 'var-pants-blue-l',
        productTitle: 'Брюки карго',
        colorName: 'Синий',
        sizeName: 'L',
        sellerSku: 'SKU-PANTS-BL-L',
        variantBarcode: 'ZMK-PANTS-01',
      },
      {
        inventoryUnitId: 'u-3',
        unitCode: 'ZMU-JACKET-RED-M-01',
        unitIndex: 1,
        supplyItemId: 'item-2',
        productVariantId: 'var-jacket-red-m',
        productTitle: 'Куртка оверсайз',
        colorName: 'Красный',
        sizeName: 'M',
        sellerSku: 'SKU-JKT-RD-M',
        variantBarcode: 'ZMK-JKT-02',
      },
      {
        inventoryUnitId: 'u-4',
        unitCode: 'ZMU-JACKET-RED-M-02',
        unitIndex: 2,
        supplyItemId: 'item-2',
        productVariantId: 'var-jacket-red-m',
        productTitle: 'Куртка оверсайз',
        colorName: 'Красный',
        sizeName: 'M',
        sellerSku: 'SKU-JKT-RD-M',
        variantBarcode: 'ZMK-JKT-02',
      },
      {
        inventoryUnitId: 'u-5',
        unitCode: 'ZMU-JACKET-RED-M-03',
        unitIndex: 3,
        supplyItemId: 'item-2',
        productVariantId: 'var-jacket-red-m',
        productTitle: 'Куртка оверсайз',
        colorName: 'Красный',
        sizeName: 'M',
        sellerSku: 'SKU-JKT-RD-M',
        variantBarcode: 'ZMK-JKT-02',
      },
    ],
  };

  it('A & B & C: labels from different variants render in separate variant groups with correct product titles and attributes', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: 'Брюки карго' })).toBeTruthy();
    });

    // Product headings
    expect(screen.getByRole('heading', { level: 2, name: 'Брюки карго' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 2, name: 'Куртка оверсайз' })).toBeTruthy();

    // Variant headings
    expect(screen.getByRole('heading', { level: 3, name: 'Синий · L' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Красный · M' })).toBeTruthy();

    // Group badge counts
    expect(screen.getAllByText('2 этикетки').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('3 этикетки').length).toBeGreaterThanOrEqual(1);
  });

  it('D & F: each ZMU remains strictly within its correct variant group without cross-contamination', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    const { container } = render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-PANTS-BLUE-L-01')).toBeTruthy();
    });

    // Check all ZMUs exist in the DOM
    expect(screen.getByText('ZMU-PANTS-BLUE-L-01')).toBeTruthy();
    expect(screen.getByText('ZMU-PANTS-BLUE-L-02')).toBeTruthy();
    expect(screen.getByText('ZMU-JACKET-RED-M-01')).toBeTruthy();
    expect(screen.getByText('ZMU-JACKET-RED-M-02')).toBeTruthy();
    expect(screen.getByText('ZMU-JACKET-RED-M-03')).toBeTruthy();

    // Find sections
    const sections = container.querySelectorAll('section');
    expect(sections.length).toBe(2);

    // Section 1: Pants
    const pantsSection = sections[0];
    expect(pantsSection.textContent).toContain('Брюки карго');
    expect(pantsSection.textContent).toContain('ZMU-PANTS-BLUE-L-01');
    expect(pantsSection.textContent).toContain('ZMU-PANTS-BLUE-L-02');
    expect(pantsSection.textContent).not.toContain('ZMU-JACKET-RED-M-01');

    // Section 2: Jackets
    const jacketsSection = sections[1];
    expect(jacketsSection.textContent).toContain('Куртка оверсайз');
    expect(jacketsSection.textContent).toContain('ZMU-JACKET-RED-M-01');
    expect(jacketsSection.textContent).toContain('ZMU-JACKET-RED-M-02');
    expect(jacketsSection.textContent).toContain('ZMU-JACKET-RED-M-03');
    expect(jacketsSection.textContent).not.toContain('ZMU-PANTS-BLUE-L-01');
  });

  it('E: position counters reset per variant group (1 из 2, 2 из 2 vs 1 из 3, 2 из 3, 3 из 3)', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: 'Брюки карго' })).toBeTruthy();
    });

    // Variant 1 counters: 1 из 2, 2 из 2
    expect(screen.getByText('SUP-2026-000201 · 1 из 2')).toBeTruthy();
    expect(screen.getByText('SUP-2026-000201 · 2 из 2')).toBeTruthy();

    // Variant 2 counters: 1 из 3, 2 из 3, 3 из 3
    expect(screen.getByText('SUP-2026-000201 · 1 из 3')).toBeTruthy();
    expect(screen.getByText('SUP-2026-000201 · 2 из 3')).toBeTruthy();
    expect(screen.getByText('SUP-2026-000201 · 3 из 3')).toBeTruthy();

    // Old global sequential counter 1/5, 5/5 should NOT exist
    expect(screen.queryByText(/· 1\/5/)).toBeNull();
    expect(screen.queryByText(/· 5\/5/)).toBeNull();
  });

  it('G & H: bulk print action is available, triggers window.print, and mutates no API', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);
    const printSpy = vi.spyOn(window, 'print').mockImplementation(() => {});

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Печать \(5 шт\)/)).toBeTruthy();
    });

    const printButton = screen.getByRole('button', { name: /Печать \(5 шт\)/i });
    fireEvent.click(printButton);

    expect(printSpy).toHaveBeenCalledTimes(1);
    // Verified read-only call only once
    expect(apiClient.getSellerSupplyUnitLabels).toHaveBeenCalledTimes(1);

    printSpy.mockRestore();
  });

  it('I & J: single-variant supply renders correctly and preserves ZMU code values', async () => {
    const mockSingleVariant: SellerSupplyUnitLabelsResponse = {
      supplyId: 'sup-single',
      supplyNumber: 'SUP-SINGLE-01',
      serialized: true,
      totalUnits: 1,
      units: [
        {
          inventoryUnitId: 'u-single',
          unitCode: 'ZMU-BR8XJV54XCMX48ZZ',
          unitIndex: 1,
          supplyItemId: 'item-single',
          productVariantId: 'var-single',
          productTitle: 'Брюки карго с карманами',
          colorName: 'Тёмно-серый',
          sizeName: 'M',
          sellerSku: 'SKU-CARGO-GRY-M',
          variantBarcode: 'ZMK-CARGO-01',
        },
      ],
    };

    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockSingleVariant);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-single/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-BR8XJV54XCMX48ZZ')).toBeTruthy();
    });

    expect(screen.getByRole('heading', { level: 2, name: 'Брюки карго с карманами' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Тёмно-серый · M' })).toBeTruthy();
    expect(screen.getByText('SUP-SINGLE-01 · 1 из 1')).toBeTruthy();
    expect(screen.getAllByText('Арт: SKU-CARGO-GRY-M').length).toBeGreaterThan(0);
    expect(screen.getAllByText('ZMK: ZMK-CARGO-01').length).toBeGreaterThan(0);
  });
});
