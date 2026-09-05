// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AdminProductDetail } from './AdminProductDetail';
import * as adminProductsApi from '../api/adminProducts';

// Mock API calls
vi.mock('../api/adminProducts', () => ({
  getAdminProduct: vi.fn(),
  getAdminProductModerationHistory: vi.fn().mockResolvedValue({ items: [] }),
  hideProduct: vi.fn().mockResolvedValue(undefined),
  blockProduct: vi.fn().mockResolvedValue(undefined),
  publishProduct: vi.fn().mockResolvedValue(undefined),
  getAdminProductErrorMessage: (_err: any, fallback: string) => fallback,
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  createProductPreviewLink: vi.fn().mockResolvedValue({
    pageUrl: 'https://shop.zamk.test/preview?token=abc',
    expiresAt: new Date(Date.now() + 900000).toISOString(),
  }),
}));

const mockProduct: adminProductsApi.AdminProductView = {
  id: 'prod-uuid-1',
  title: 'Premium Italian Leather Jacket',
  description: 'Handcrafted genuine leather jacket with quilted lining.',
  slug: 'premium-leather-jacket',
  categoryId: 'cat-1',
  sellerId: 'seller-1',
  sellerName: 'Milano Fashion House',
  price: 45000,
  priceCents: 4500000,
  currency: 'RUB',
  status: 'published',
  statusLabel: 'Опубликован',
  sellerStatus: 'active',
  stock: 10,
  reservedStock: 2,
  actualVisibility: true,
  createdAt: '2026-09-01T10:00:00Z',
  updatedAt: '2026-09-01T10:00:00Z',
  gallery: [],
  variants: [
    {
      id: 'var-1',
      label: 'Black / M',
      sku: 'ZMK-BLK-M',
      sellerSku: 'MIL-100-M',
      barcode: '2000000000018',
      color: 'Black',
      size: 'M',
      price: 45000,
      priceCents: 4500000,
      isActive: true,
      inStock: true,
      totalStock: 10,
    },
  ],
};

describe('AdminProductDetail Responsibility Boundary Hardening', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminProductsApi.getAdminProduct).mockResolvedValue(mockProduct);
  });

  it('renders seller-owned commercial fields in strict read-only form', async () => {
    render(
      <MemoryRouter initialEntries={['/products/prod-uuid-1']}>
        <Routes>
          <Route path="/products/:productId" element={<AdminProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Premium Italian Leather Jacket')).toBeDefined();
    });

    // Check store name
    expect(screen.getAllByText('Milano Fashion House').length).toBeGreaterThanOrEqual(1);

    // Verify absence of "Редактировать" button
    expect(screen.queryByText('Редактировать')).toBeNull();

    // Verify absence of input fields for editing
    expect(screen.queryByLabelText('Название товара')).toBeNull();
    expect(screen.queryByLabelText('Описание товара')).toBeNull();
    expect(screen.queryByLabelText('Категория')).toBeNull();
    expect(screen.queryByLabelText('Цена (₽)')).toBeNull();
  });

  it('preserves legitimate operational and moderation controls', async () => {
    render(
      <MemoryRouter initialEntries={['/products/prod-uuid-1']}>
        <Routes>
          <Route path="/products/:productId" element={<AdminProductDetail />} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Premium Italian Leather Jacket')).toBeDefined();
    });

    // Legitimate admin actions: Hide and Block should be present
    const hideBtns = screen.getAllByText('Скрыть');
    expect(hideBtns.length).toBeGreaterThanOrEqual(1);

    const blockBtns = screen.getAllByText('Заблокировать');
    expect(blockBtns.length).toBeGreaterThanOrEqual(1);

    // Clicking "Скрыть" button opens the action reason modal
    fireEvent.click(hideBtns[0]);
    expect(screen.getByText('Скрыть товар с витрины')).toBeDefined();
    expect(screen.getByPlaceholderText('Причина действия...')).toBeDefined();
  });
});
