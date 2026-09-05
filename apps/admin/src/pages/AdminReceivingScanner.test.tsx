import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { AdminReceivingScanner } from './AdminReceivingScanner';
import { MemoryRouter } from 'react-router-dom';
import * as AdminAuthContext from '../contexts/AdminAuthContext';
import * as adminOrdersApi from '../api/adminOrders';

vi.mock('../utils/audio', () => ({
  playBeepSound: vi.fn(),
}));

vi.mock('../api/adminOrders', () => ({
  resolveReceivingCode: vi.fn(),
  startReceiving: vi.fn(),
  scanItem: vi.fn(),
  confirmReceiving: vi.fn(),
  recordDiscrepancy: vi.fn(),
}));

const mockFulfillment = {
  id: 'ful-123',
  orderId: 'ord-123',
  orderNumber: '1001',
  receivingCode: 'FUL-2026-1001',
  status: 'packed',
  sellerName: 'Test Seller',
};

const mockSession = {
  id: 'sess-123',
  fulfillmentId: 'ful-123',
  status: 'active',
  version: 1,
  canConfirm: true,
  items: [
    {
      id: 'item-1',
      sku: 'SKU-001',
      barcode: '123456789012',
      productTitle: 'Sneakers',
      expectedQuantity: 1,
      scannedQuantity: 1,
    },
  ],
};

function setupAuth(perms: string[]) {
  vi.spyOn(AdminAuthContext, 'useAdminAuth').mockReturnValue({
    hasPermission: (perm: string) => perms.includes(perm),
    hasAnyPermission: (permList: string[]) => permList.some((p) => perms.includes(p)),
    user: null,
    isAuthenticated: true,
    isLoading: false,
    staff: null,
    login: vi.fn(),
    logout: vi.fn(),
  } as any);
}

describe('AdminReceivingScanner Permissions Contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminOrdersApi.resolveReceivingCode).mockResolvedValue(mockFulfillment as any);
    vi.mocked(adminOrdersApi.startReceiving).mockResolvedValue(mockSession as any);
  });

  it('A. orders.read only: read visible and all mutation controls disabled', async () => {
    setupAuth(['orders.read']);

    render(
      <MemoryRouter>
        <AdminReceivingScanner />
      </MemoryRouter>
    );

    // Initial search input and button are disabled for read-only roles
    const searchInput = screen.getByTestId('receiving-code-input') as HTMLInputElement;
    const searchButton = screen.getByTestId('receiving-search-submit') as HTMLButtonElement;
    expect(searchInput).toBeDefined();
    expect(searchInput.disabled).toBe(true);
    expect(searchButton.disabled).toBe(true);

    // Focus attempt does not make input active
    searchInput.focus();
    expect(document.activeElement).not.toBe(searchInput);

    // Enter key or form submission does NOT invoke resolveReceivingCode
    fireEvent.keyDown(searchInput, { key: 'Enter', code: 'Enter' });
    const form = searchInput.closest('form');
    if (form) {
      fireEvent.submit(form);
    }
    expect(adminOrdersApi.resolveReceivingCode).not.toHaveBeenCalled();
    expect(adminOrdersApi.startReceiving).not.toHaveBeenCalled();
  });

  it('B. warehouse.receiving only: start/scan/discrepancy controls available, confirm/finalize disabled', async () => {
    setupAuth(['orders.read', 'warehouse.receiving']);

    render(
      <MemoryRouter>
        <AdminReceivingScanner />
      </MemoryRouter>
    );

    // Start input and button are available
    const searchInput = screen.getByTestId('receiving-code-input') as HTMLInputElement;
    const searchButton = screen.getByTestId('receiving-search-submit') as HTMLButtonElement;
    expect(searchInput.disabled).toBe(false);
    fireEvent.change(searchInput, { target: { value: 'FUL-2026-1001' } });
    expect(searchButton.disabled).toBe(false);

    // Start receiving
    fireEvent.click(searchButton);

    // Now workspace screen is loaded
    await waitFor(() => {
      expect(screen.getByText('Sneakers')).toBeDefined();
    });

    // Scan input & scan submit are available
    const scanInput = screen.getByTestId('receiving-item-barcode-input') as HTMLInputElement;
    expect(scanInput.disabled).toBe(false);

    // Discrepancy is available
    const discrepancyButton = screen.getByTestId('receiving-discrepancy') as HTMLButtonElement;
    expect(discrepancyButton.disabled).toBe(false);

    // Confirm is DISABLED because user lacks shipments.create
    const confirmButton = screen.getByTestId('receiving-confirm') as HTMLButtonElement;
    expect(confirmButton.disabled).toBe(true);
  });

  it('C. shipments.create only: must NOT accidentally gain start/scan receiving actions', async () => {
    setupAuth(['orders.read', 'shipments.create']);

    render(
      <MemoryRouter>
        <AdminReceivingScanner />
      </MemoryRouter>
    );

    // Start/search is disabled because user lacks warehouse.receiving
    const searchInput = screen.getByTestId('receiving-code-input') as HTMLInputElement;
    const searchButton = screen.getByTestId('receiving-search-submit') as HTMLButtonElement;
    expect(searchInput.disabled).toBe(true);
    expect(searchButton.disabled).toBe(true);

    // Enter key does NOT invoke resolveReceivingCode
    fireEvent.keyDown(searchInput, { key: 'Enter', code: 'Enter' });
    const form = searchInput.closest('form');
    if (form) {
      fireEvent.submit(form);
    }
    expect(adminOrdersApi.resolveReceivingCode).not.toHaveBeenCalled();
  });

  it('D. warehouse.receiving + shipments.create: full receiving flow controls enabled', async () => {
    setupAuth(['orders.read', 'warehouse.receiving', 'shipments.create']);

    render(
      <MemoryRouter>
        <AdminReceivingScanner />
      </MemoryRouter>
    );

    // Start is available
    const searchInput = screen.getByTestId('receiving-code-input') as HTMLInputElement;
    const searchButton = screen.getByTestId('receiving-search-submit') as HTMLButtonElement;
    expect(searchInput.disabled).toBe(false);
    fireEvent.change(searchInput, { target: { value: 'FUL-2026-1001' } });
    expect(searchButton.disabled).toBe(false);

    // Transition to workspace
    fireEvent.click(searchButton);

    await waitFor(() => {
      expect(screen.getByText('Sneakers')).toBeDefined();
    });

    // Scan is available
    const scanInput = screen.getByTestId('receiving-item-barcode-input') as HTMLInputElement;
    expect(scanInput.disabled).toBe(false);

    // Discrepancy is available
    const discrepancyButton = screen.getByTestId('receiving-discrepancy') as HTMLButtonElement;
    expect(discrepancyButton.disabled).toBe(false);

    // Confirm is ENABLED because session.canConfirm is true AND user has shipments.create
    const confirmButton = screen.getByTestId('receiving-confirm') as HTMLButtonElement;
    expect(confirmButton.disabled).toBe(false);
  });
});
