/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { SellerProducts } from './SellerProducts';
import { getSellerProductsPaginated, getSellerMe, getSellerCategories, getSellerProduct } from '@zamk/api-client/src/seller';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerProductsPaginated: vi.fn(),
  getSellerMe: vi.fn(),
  getSellerCategories: vi.fn(),
  submitSellerProductModeration: vi.fn(),
  deleteSellerProduct: vi.fn(),
  archiveSellerProduct: vi.fn(),
  getSellerProduct: vi.fn(),
}));

// SELLER.SKU.WORKFLOW.1: from the product drawer to the exact variant.

const variants = [
  { id: 'v-ok', size: 'S', isActive: true, totalStock: 5, reservedStock: 1, availableStock: 4, priceCents: 150000 },
  { id: 'v-empty', size: 'M', isActive: true, totalStock: 0, reservedStock: 0, availableStock: 0 },
  { id: 'v-off', size: 'L', isActive: false, totalStock: 2, reservedStock: 0, availableStock: 2 },
  { id: 'v-res', size: 'XL', isActive: true, totalStock: 3, reservedStock: 3, availableStock: 0 },
];

const product = (over: Record<string, any> = {}): any => ({
  id: 'p-1', title: 'Худи', slug: 'hoodie', status: 'published', priceCents: 150000,
  actualVisibility: true, visibilityReasons: [], variants, ...over,
});

const renderPage = (me: string = 'active') => {
  vi.mocked(getSellerMe).mockResolvedValue({ seller: { id: 's', status: me, brandName: 'B' } } as any);
  return render(
    <MemoryRouter initialEntries={['/products']}>
      <Routes>
        <Route path="/products" element={<SellerProducts />} />
        <Route path="*" element={<div />} />
      </Routes>
    </MemoryRouter>
  );
};

const openDrawer = async (title = 'Худи') => {
  await waitFor(() => expect(screen.getByText(title)).toBeTruthy());
  fireEvent.click(screen.getByText(title));
  return screen.findByTestId('drawer-sku-list');
};

describe('SellerProducts drawer — SKU workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerCategories).mockResolvedValue([]);
    vi.mocked(getSellerProduct).mockResolvedValue({} as any);
  });

  it('shows each SKU with its own state, reason and responsible party', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [product()], totalCount: 1, summary: {} } as any);
    renderPage();
    const list = await openDrawer();
    const rows = within(list).getAllByTestId('drawer-sku-row');
    expect(rows.map((r) => r.getAttribute('data-sku-code'))).toEqual(['ok', 'no_stock', 'disabled', 'reserved']);
    expect(rows[0].textContent).toContain('Свободно 4 шт.');
    expect(rows[0].textContent).toContain('в резерве 1');
    expect(rows[1].textContent).toContain('Склад ZAMK');
    expect(rows[2].textContent).toContain('Действие продавца');
    expect(rows[3].textContent).toContain('зарезервирован');
    expect(rows[3].textContent).toContain('Система ZAMK');
  });

  it('links to exactly this variant in Studio, and only where the seller can act', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [product()], totalCount: 1, summary: {} } as any);
    renderPage();
    const list = await openDrawer();
    const rows = within(list).getAllByTestId('drawer-sku-row');
    expect(within(rows[0]).queryByTestId('drawer-sku-action')).toBeNull();
    expect(within(rows[1]).getByTestId('drawer-sku-action').getAttribute('href')).toBe('/supplies/new');
    expect(within(rows[2]).getByTestId('drawer-sku-action').getAttribute('href')).toBe('/products/p-1/edit?variant=v-off');
    expect(within(rows[3]).queryByTestId('drawer-sku-action')).toBeNull();
  });

  it('pending moderation: no seller action, moderation named', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [product({ status: 'pending_moderation', actualVisibility: false, visibilityReasons: ['moderation_required'], variants: [variants[0]] })],
      totalCount: 1, summary: {},
    } as any);
    renderPage();
    const list = await openDrawer();
    expect(within(list).queryByTestId('drawer-sku-action')).toBeNull();
    expect(list.textContent).toContain('Решение модерации ZAMK');
  });

  it('unloaded warehouse data is shown as unknown, not as zero', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({
      items: [product({ variants: [{ id: 'v', size: 'M', isActive: true }] })], totalCount: 1, summary: {},
    } as any);
    renderPage();
    const list = await openDrawer();
    expect(within(list).getByTestId('drawer-sku-stock').textContent).toContain('Остаток не загружен');
    expect(list.textContent).not.toContain('Свободно 0');
  });

  it('the seller has no way to type a physical stock figure in the drawer', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [product()], totalCount: 1, summary: {} } as any);
    renderPage();
    const list = await openDrawer();
    expect(list.querySelectorAll('input, textarea, select').length).toBe(0);
  });

  it('a blocked shop gets no variant actions', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [product()], totalCount: 1, summary: {} } as any);
    renderPage('blocked');
    const list = await openDrawer();
    expect(within(list).queryAllByTestId('drawer-sku-action')).toHaveLength(0);
  });

  it('an unapproved edit of a published product is shown as waiting for ZAMK, not as applied', async () => {
    vi.mocked(getSellerProductsPaginated).mockResolvedValue({ items: [product()], totalCount: 1, summary: {} } as any);
    vi.mocked(getSellerProduct).mockResolvedValue({
      variants: variants.map((v) => ({ id: v.id, isActive: true })),
      pendingRevision: { id: 'r', variantsChanged: true, variants: variants.map((v) => ({ id: v.id, isActive: true })) },
    } as any);
    renderPage();
    const list = await openDrawer();
    await screen.findByTestId('drawer-pending-revision');
    const rows = within(list).getAllByTestId('drawer-sku-row');
    expect(within(rows[2]).getByTestId('drawer-sku-pending').textContent).toContain('будет включён');
    expect(within(rows[2]).getByTestId('drawer-sku-pending').textContent).toContain('Решение модерации ZAMK');
    expect(within(rows[0]).queryByTestId('drawer-sku-pending')).toBeNull();
    // the live state of the SKU is unchanged until the backend says so
    expect(rows[2].getAttribute('data-sku-code')).toBe('disabled');
  });
});
