/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { SellerSupplyDetail } from './SellerSupplyDetail';
import * as sellerApi from '@zamk/api-client/src/seller';
import type { SellerSupply } from '@zamk/api-client/src/types';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerSupply: vi.fn(),
  shipSellerSupply: vi.fn(),
}));

describe('SellerSupplyDetail Component (SA.2)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('1. normal receiving progress renders (status: receiving)', async () => {
    const mockSupply: SellerSupply = {
      id: 'sup-101',
      sellerId: 'seller-1',
      supplyNumber: 'SUP-000101',
      status: 'receiving',
      handoffMethod: 'carrier_delivery',
      carrierName: 'СДЭК',
      trackingNumber: 'TRK-101',
      totalExpectedBoxes: 1,
      totalExpectedItems: 10,
      totalAcceptedItems: 6,
      totalRemainingItems: 4,
      isReceivingComplete: false,
      additionalReceivingHappened: false,
      createdAt: '2026-09-01T10:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
      items: [
        {
          id: 'item-1',
          supplyId: 'sup-101',
          variantId: 'var-1',
          productTitle: 'Dev Silk Dress',
          sku: 'DEV-DRESS-M',
          sellerSku: 'SKU-DRESS-M',
          colorName: 'Черный',
          sizeName: 'M',
          expectedQuantity: 10,
          acceptedQuantity: 6,
          damagedQuantity: 0,
          missingQuantity: 0,
          extraQuantity: 0,
          remainingQuantity: 4,
        },
      ],
    };

    vi.mocked(sellerApi.getSellerSupply).mockResolvedValue(mockSupply);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-101']}>
        <Routes>
          <Route path="/supplies/:id" element={<SellerSupplyDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText(/SUP-000101/).length).toBeGreaterThan(0);
    });

    // Summary assertions
    expect(screen.getByText('Идёт приёмка на складе ZAMK')).toBeTruthy();
    expect(screen.getAllByText('10').length).toBeGreaterThan(0); // totalExpectedItems
    expect(screen.getAllByText('6').length).toBeGreaterThan(0); // totalAcceptedItems
    expect(screen.getAllByText('4').length).toBeGreaterThan(0); // totalRemainingItems

    // Heading hierarchy and DOM order
    const tableHeading = screen.getByRole('heading', { level: 3, name: 'Результат приёмки' });
    const deliveryHeading = screen.getByRole('heading', { level: 3, name: 'Доставка' });
    const markingHeading = screen.getByRole('heading', { level: 3, name: 'Этикетки и маркировка' });
    expect(tableHeading).toBeTruthy();
    expect(tableHeading.compareDocumentPosition(deliveryHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(tableHeading.compareDocumentPosition(markingHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    // Item table shows product
    expect(screen.getByText('Dev Silk Dress')).toBeTruthy();
  });

  it('2. completed supply renders with full acceptance', async () => {
    const mockSupply: SellerSupply = {
      id: 'sup-102',
      sellerId: 'seller-1',
      supplyNumber: 'SUP-000102',
      status: 'completed',
      handoffMethod: 'carrier_delivery',
      totalExpectedBoxes: 1,
      totalExpectedItems: 5,
      totalAcceptedItems: 5,
      totalRemainingItems: 0,
      discrepancyCount: 0,
      isReceivingComplete: true,
      additionalReceivingHappened: false,
      createdAt: '2026-09-01T10:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
      items: [
        {
          id: 'item-2',
          supplyId: 'sup-102',
          variantId: 'var-2',
          productTitle: 'Dev Cotton T-Shirt',
          sku: 'DEV-TSHIRT-L',
          expectedQuantity: 5,
          acceptedQuantity: 5,
          damagedQuantity: 0,
          missingQuantity: 0,
          extraQuantity: 0,
          remainingQuantity: 0,
        },
      ],
    };

    vi.mocked(sellerApi.getSellerSupply).mockResolvedValue(mockSupply);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-102']}>
        <Routes>
          <Route path="/supplies/:id" element={<SellerSupplyDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText(/SUP-000102/).length).toBeGreaterThan(0);
    });

    expect(screen.getByText('Приёмка завершена')).toBeTruthy();
    expect(screen.getByText('Полностью принята')).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Результат приёмки' })).toBeTruthy();
  });

  it('3. discrepancy state renders human-readable damaged and missing quantities', async () => {
    const mockSupply: SellerSupply = {
      id: 'sup-103',
      sellerId: 'seller-1',
      supplyNumber: 'SUP-000103',
      status: 'completed_with_discrepancies',
      handoffMethod: 'carrier_delivery',
      totalExpectedBoxes: 1,
      totalExpectedItems: 5,
      totalAcceptedItems: 3,
      totalRemainingItems: 1,
      discrepancyCount: 2, // 1 damaged + 1 missing
      isReceivingComplete: true,
      additionalReceivingHappened: true,
      createdAt: '2026-09-01T10:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
      items: [
        {
          id: 'item-3',
          supplyId: 'sup-103',
          variantId: 'var-3',
          productTitle: 'Dev Wool Coat',
          sku: 'DEV-COAT-XL',
          expectedQuantity: 5,
          acceptedQuantity: 3,
          damagedQuantity: 1,
          missingQuantity: 1,
          extraQuantity: 0,
          remainingQuantity: 1,
        },
      ],
    };

    vi.mocked(sellerApi.getSellerSupply).mockResolvedValue(mockSupply);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-103']}>
        <Routes>
          <Route path="/supplies/:id" element={<SellerSupplyDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText(/SUP-000103/).length).toBeGreaterThan(0);
    });

    expect(screen.getByText('Есть расхождения')).toBeTruthy();
    expect(screen.getByText('Была дополнительная приёмка')).toBeTruthy();
    expect(screen.getAllByText('5').length).toBeGreaterThan(0); // Заявлено
    expect(screen.getAllByText('3').length).toBeGreaterThan(0); // Принято
    expect(screen.getAllByText('Повреждено').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Недостача').length).toBeGreaterThan(0);
    expect(screen.getByText('Брак и недостача')).toBeTruthy();
    expect(screen.getByRole('heading', { level: 3, name: 'Результат приёмки' })).toBeTruthy();
    expect(screen.queryByText('Ожидается')).toBeNull(); // Completed supply does NOT show "Ожидается"
  });

  it('4. no warehouse action controls exist (negative action check)', async () => {
    const mockSupply: SellerSupply = {
      id: 'sup-104',
      sellerId: 'seller-1',
      supplyNumber: 'SUP-000104',
      status: 'receiving',
      handoffMethod: 'carrier_delivery',
      totalExpectedBoxes: 1,
      totalExpectedItems: 10,
      totalAcceptedItems: 5,
      totalRemainingItems: 5,
      createdAt: '2026-09-01T10:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
      items: [],
    };

    vi.mocked(sellerApi.getSellerSupply).mockResolvedValue(mockSupply);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-104']}>
        <Routes>
          <Route path="/supplies/:id" element={<SellerSupplyDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText(/SUP-000104/).length).toBeGreaterThan(0);
    });

    // Verify absence of warehouse operational action controls
    expect(screen.queryByRole('button', { name: /сканировать/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /завершить/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /начать приёмку/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /принять/i })).toBeNull();
    expect(screen.queryByRole('button', { name: /списать/i })).toBeNull();
    expect(screen.queryByRole('textbox', { name: /штрихкод/i })).toBeNull();
  });

  it('5. empty/no discrepancy state renders cleanly', async () => {
    const mockSupply: SellerSupply = {
      id: 'sup-105',
      sellerId: 'seller-1',
      supplyNumber: 'SUP-000105',
      status: 'ready_to_ship',
      handoffMethod: 'carrier_delivery',
      totalExpectedBoxes: 1,
      totalExpectedItems: 10,
      totalAcceptedItems: 0,
      totalRemainingItems: 10,
      discrepancyCount: 0,
      isReceivingComplete: false,
      additionalReceivingHappened: false,
      createdAt: '2026-09-01T10:00:00Z',
      updatedAt: '2026-09-01T12:00:00Z',
      items: [
        {
          id: 'item-5',
          supplyId: 'sup-105',
          variantId: 'var-5',
          productTitle: 'Dev Scarf',
          sku: 'DEV-SCARF-UNI',
          expectedQuantity: 10,
          acceptedQuantity: 0,
          damagedQuantity: 0,
          missingQuantity: 0,
          extraQuantity: 0,
          remainingQuantity: 10,
        },
      ],
    };

    vi.mocked(sellerApi.getSellerSupply).mockResolvedValue(mockSupply);

    render(
      <MemoryRouter initialEntries={['/supplies/sup-105']}>
        <Routes>
          <Route path="/supplies/:id" element={<SellerSupplyDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText(/SUP-000105/).length).toBeGreaterThan(0);
    });

    // Declared items show expected 10 and accepted —
    expect(screen.getByText('Dev Scarf')).toBeTruthy();
    expect(screen.getAllByText('—').length).toBeGreaterThan(0);
    // Heading hierarchy and DOM order
    const tableHeading = screen.getByRole('heading', { level: 3, name: 'Состав поставки' });
    const deliveryHeading = screen.getByRole('heading', { level: 3, name: 'Доставка' });
    const markingHeading = screen.getByRole('heading', { level: 3, name: 'Этикетки и маркировка' });
    expect(tableHeading).toBeTruthy();
    expect(tableHeading.compareDocumentPosition(deliveryHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(tableHeading.compareDocumentPosition(markingHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // No discrepancy headers
    expect(screen.queryByText('Повреждено')).toBeNull();
    expect(screen.queryByText('Недостача')).toBeNull();
  });
});
