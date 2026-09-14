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

describe('SellerSupplyUnitLabels Component (SUP.2A1, SUP.2A2 & SUP.2A3)', () => {
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

  it('SUP.2A1 & U: grouping remains intact with product and variant headers', async () => {
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

    expect(screen.getByRole('heading', { level: 2, name: 'Брюки карго' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 2, name: 'Куртка оверсайз' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Синий · L' })).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Красный · M' })).toBeTruthy();
    expect(screen.getByText('SUP-2026-000201 · 1 из 2')).toBeTruthy();
    expect(screen.getByText('SUP-2026-000201 · 1 из 3')).toBeTruthy();
  });

  it('SUP.2A2 & V: modal, preview, and variant navigation remain intact when selection mode is OFF', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-JACKET-RED-M-02')).toBeTruthy();
    });

    // Click label card opens modal (K: selection mode OFF opens modal)
    fireEvent.click(screen.getByText('ZMU-JACKET-RED-M-02').closest('.unit-label-card')!);

    const dialog = screen.getByRole('dialog', { name: 'Предпросмотр этикетки' });
    expect(dialog).toBeTruthy();
    expect(dialog.textContent).toContain('Куртка оверсайз');
    expect(dialog.textContent).toContain('2 из 3');

    // Navigation works
    const prevBtn = screen.getByRole('button', { name: 'Предыдущая этикетка' });
    fireEvent.click(prevBtn);
    expect(dialog.textContent).toContain('1 из 3');
    expect(dialog.textContent).toContain('ZMU-JACKET-RED-M-01');

    // Escape closes modal
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('A, B, C, D, L: "Выбрать" enters selection mode, allows cross-variant selection, updates count, and disables modal', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-PANTS-BLUE-L-01')).toBeTruthy();
    });

    // Enter selection mode
    const selectModeBtn = screen.getByRole('button', { name: /Выбрать$/i });
    fireEvent.click(selectModeBtn);

    expect(screen.getByText('Выбрано: 0')).toBeTruthy();
    const printZeroBtn = screen.getByRole('button', { name: 'Печать выбранных (0)' }) as HTMLButtonElement;
    expect(printZeroBtn.disabled).toBe(true);

    // Click Pants #1 (u-1)
    fireEvent.click(screen.getByText('ZMU-PANTS-BLUE-L-01').closest('.unit-label-card')!);
    // Modal must NOT open (L)
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.getByText('Выбрано: 1')).toBeTruthy();

    // Click Jacket #2 (u-4) - across variants (C)
    fireEvent.click(screen.getByText('ZMU-JACKET-RED-M-02').closest('.unit-label-card')!);
    expect(screen.getByText('Выбрано: 2')).toBeTruthy();
    const printTwoBtn = screen.getByRole('button', { name: 'Печать выбранных (2)' }) as HTMLButtonElement;
    expect(printTwoBtn.disabled).toBe(false);

    // Clicking again deselects
    fireEvent.click(screen.getByText('ZMU-PANTS-BLUE-L-01').closest('.unit-label-card')!);
    expect(screen.getByText('Выбрано: 1')).toBeTruthy();

    // Click "Готово" clears selection and exits selection mode
    fireEvent.click(screen.getByRole('button', { name: 'Готово' }));
    expect(screen.queryByText(/Выбрано:/)).toBeNull();
    expect(screen.getByRole('button', { name: /Выбрать$/i })).toBeTruthy();
  });

  it('E, F, Q: "Печать выбранных (N)" prints ONLY selected ZMUs, excludes others, and marks them printRequested', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    let printedText: string | null = null;
    const printSpy = vi.spyOn(window, 'print').mockImplementation(() => {
      const targetContainer = document.querySelector('.print-target-container');
      if (targetContainer) {
        printedText = targetContainer.textContent;
      }
    });

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-PANTS-BLUE-L-01')).toBeTruthy();
    });

    // Enter selection mode
    fireEvent.click(screen.getByRole('button', { name: /Выбрать$/i }));

    // Select Pants #2 (u-2) and Jacket #1 (u-3)
    fireEvent.click(screen.getByText('ZMU-PANTS-BLUE-L-02').closest('.unit-label-card')!);
    fireEvent.click(screen.getByText('ZMU-JACKET-RED-M-01').closest('.unit-label-card')!);

    const printSelectedBtn = screen.getByRole('button', { name: 'Печать выбранных (2)' });
    fireEvent.click(printSelectedBtn);

    expect(printSpy).toHaveBeenCalledTimes(1);
    expect(printedText).toBeTruthy();

    // Included selected ZMUs
    expect(printedText).toContain('ZMU-PANTS-BLUE-L-02');
    expect(printedText).toContain('ZMU-JACKET-RED-M-01');

    // Excluded unselected ZMUs
    expect(printedText).not.toContain('ZMU-PANTS-BLUE-L-01');
    expect(printedText).not.toContain('ZMU-JACKET-RED-M-02');
    expect(printedText).not.toContain('ZMU-JACKET-RED-M-03');

    // Progress mark "Отправлено на печать" is marked for the selected items (Q)
    const pantsCard2 = screen.getByText('ZMU-PANTS-BLUE-L-02').closest('.unit-label-card');
    const jacketCard1 = screen.getByText('ZMU-JACKET-RED-M-01').closest('.unit-label-card');
    expect(pantsCard2?.textContent).toContain('Отправлено на печать');
    expect(jacketCard1?.textContent).toContain('Отправлено на печать');

    // Unselected items are NOT marked printRequested
    const pantsCard1 = screen.getByText('ZMU-PANTS-BLUE-L-01').closest('.unit-label-card');
    expect(pantsCard1?.textContent).not.toContain('Отправлено на печать');

    printSpy.mockRestore();
  });

  it('G, H, P: variant group print prints only that variant and marks all units in it printRequested', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    let printedText: string | null = null;
    const printSpy = vi.spyOn(window, 'print').mockImplementation(() => {
      const targetContainer = document.querySelector('.print-target-container');
      if (targetContainer) {
        printedText = targetContainer.textContent;
      }
    });

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Печать группы: Синий · L/i })).toBeTruthy();
    });

    const printPantsBtn = screen.getByRole('button', { name: /Печать группы: Синий · L/i });
    fireEvent.click(printPantsBtn);

    expect(printSpy).toHaveBeenCalledTimes(1);
    expect(printedText).toBeTruthy();

    // Contains only Pants
    expect(printedText).toContain('ZMU-PANTS-BLUE-L-01');
    expect(printedText).toContain('ZMU-PANTS-BLUE-L-02');

    // Never crosses into Jackets (H)
    expect(printedText).not.toContain('ZMU-JACKET');

    // Pants marked printRequested (P)
    const pants1 = screen.getByText('ZMU-PANTS-BLUE-L-01').closest('.unit-label-card');
    const pants2 = screen.getByText('ZMU-PANTS-BLUE-L-02').closest('.unit-label-card');
    expect(pants1?.textContent).toContain('Отправлено на печать');
    expect(pants2?.textContent).toContain('Отправлено на печать');

    // Jackets not marked
    const jacket1 = screen.getByText('ZMU-JACKET-RED-M-01').closest('.unit-label-card');
    expect(jacket1?.textContent).not.toContain('Отправлено на печать');

    printSpy.mockRestore();
  });

  it('I, J, R, O: bulk print prints all units and marks all; single print prints one and marks one', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    let targetContainerPresent = false;
    let singlePrintText: string | null = null;
    const printSpy = vi.spyOn(window, 'print').mockImplementation(() => {
      const targetContainer = document.querySelector('.print-target-container');
      targetContainerPresent = !!targetContainer;
      if (targetContainer) {
        singlePrintText = targetContainer.textContent;
      }
    });

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Печать всех \(5\)/i })).toBeTruthy();
    });

    // 1. Single print check (J & O)
    fireEvent.click(screen.getByText('ZMU-JACKET-RED-M-03').closest('.unit-label-card')!);
    fireEvent.click(screen.getByRole('button', { name: /Печать этой этикетки/i }));
    expect(printSpy).toHaveBeenCalledTimes(1);
    expect(singlePrintText).toContain('ZMU-JACKET-RED-M-03');
    expect(singlePrintText).not.toContain('ZMU-JACKET-RED-M-01');

    // Close modal
    fireEvent.keyDown(window, { key: 'Escape' });

    // Jacket 3 is marked printRequested (O)
    const jacket3 = screen.getByText('ZMU-JACKET-RED-M-03').closest('.unit-label-card');
    expect(jacket3?.textContent).toContain('Отправлено на печать');

    // 2. Bulk print check (I & R)
    fireEvent.click(screen.getByRole('button', { name: /Печать всех \(5\)/i }));
    expect(printSpy).toHaveBeenCalledTimes(2);
    // Bulk print does not use isolated single target container
    expect(targetContainerPresent).toBe(false);

    // All units now marked printRequested (R)
    expect(screen.getAllByText('🖨 Отправлено на печать').length).toBe(5);

    printSpy.mockRestore();
  });

  it('M, N, S, T: opening modal and navigating marks viewed, marks are excluded from print, and cause no API mutation', async () => {
    vi.mocked(apiClient.getSellerSupplyUnitLabels).mockResolvedValue(mockMultiVariantResponse);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-201/unit-labels']}>
        <Routes>
          <Route path="/supplies/:id/unit-labels" element={<SellerSupplyUnitLabels />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('ZMU-PANTS-BLUE-L-01')).toBeTruthy();
    });

    // Initially not viewed
    expect(screen.queryByText('✓ Просмотрено')).toBeNull();

    // Click Pants #1 (M: opening modal marks viewed)
    fireEvent.click(screen.getByText('ZMU-PANTS-BLUE-L-01').closest('.unit-label-card')!);
    expect(screen.getByRole('dialog')).toBeTruthy();

    // Navigate to Pants #2 (N: navigating marks newly displayed unit as viewed)
    const nextBtn = screen.getByRole('button', { name: 'Следующая этикетка' });
    fireEvent.click(nextBtn);

    // Close modal
    fireEvent.click(screen.getAllByRole('button', { name: 'Закрыть' })[0]);

    // Both Pants #1 and Pants #2 should have "✓ Просмотрено"
    const pants1 = screen.getByText('ZMU-PANTS-BLUE-L-01').closest('.unit-label-card');
    const pants2 = screen.getByText('ZMU-PANTS-BLUE-L-02').closest('.unit-label-card');
    expect(pants1?.textContent).toContain('Просмотрено');
    expect(pants2?.textContent).toContain('Просмотрено');

    // Jacket #1 was NOT viewed
    const jacket1 = screen.getByText('ZMU-JACKET-RED-M-01').closest('.unit-label-card');
    expect(jacket1?.textContent).not.toContain('Просмотрено');

    // (S: marks have .no-print)
    const badges = document.querySelectorAll('.no-print');
    expect(badges.length).toBeGreaterThan(0);

    // (T: strictly read-only API call, no mutation)
    expect(apiClient.getSellerSupplyUnitLabels).toHaveBeenCalledTimes(1);
  });
});
