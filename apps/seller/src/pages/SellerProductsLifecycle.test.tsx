/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SellerProducts } from './SellerProducts';
import {
  getSellerProducts,
  getSellerMe,
  deleteSellerProduct,
  archiveSellerProduct,
  submitSellerProductModeration,
} from '@zamk/api-client/src/seller';

vi.mock('@zamk/api-client/src/seller', () => ({
  getSellerProducts: vi.fn(),
  getSellerMe: vi.fn(),
  submitSellerProductModeration: vi.fn(),
  deleteSellerProduct: vi.fn(),
  archiveSellerProduct: vi.fn(),
}));

describe('SELLER ASSORTMENT.1C2 - Lifecycle Actions UI & Safety Matrix', () => {
  const baseProductRaw: any = {
    id: 'prod-lifecycle-1',
    title: 'Платье миди',
    slug: 'dress-midi',
    status: 'draft',
    priceCents: 250000,
    totalStock: 10,
    availableStock: 10,
    description: 'Элегантное платье из шелка.',
    variants: [
      { id: 'v1', size: 'S', colorName: 'Черный', totalStock: 5, availableStock: 5 },
      { id: 'v2', size: 'M', colorName: 'Черный', totalStock: 5, availableStock: 5 },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getSellerMe).mockResolvedValue({
      seller: { id: 'seller-1', status: 'active', brandName: 'Test Brand' },
    } as any);
  });

  // ==========================================
  // UI MATRIX (C - I)
  // ==========================================
  describe('UI Matrix: Actions by Status', () => {
    it('C: draft menu shows "Удалить черновик" and "В архив"', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open row action menu
      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.getByText('Удалить черновик')).toBeTruthy();
        expect(screen.getByText('В архив')).toBeTruthy();
      });
    });

    it('D: rejected menu shows "В архив", NO delete', async () => {
      const rejectedProd = { ...baseProductRaw, status: 'rejected' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([rejectedProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.getByText('В архив')).toBeTruthy();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
      });
    });

    it('D1: approved menu shows "В архив", NO delete', async () => {
      const approvedProd = { ...baseProductRaw, status: 'approved' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([approvedProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.getByText('В архив')).toBeTruthy();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
        expect(screen.queryByText('Снять с продажи')).toBeNull();
      });
    });

    it('D2: out_of_stock menu shows "В архив", NO delete', async () => {
      const outOfStockProd = { ...baseProductRaw, status: 'out_of_stock' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([outOfStockProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.getByText('В архив')).toBeTruthy();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
        expect(screen.queryByText('Снять с продажи')).toBeNull();
      });
    });

    it('E: published menu shows "Снять с продажи", NO hard delete', async () => {
      const publishedProd = {
        ...baseProductRaw,
        status: 'published',
        actualVisibility: true,
        storefrontUrl: 'http://shop.zamk/products/dress-midi',
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([publishedProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.getByText('Снять с продажи')).toBeTruthy();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
        expect(screen.queryByText('В архив')).toBeNull();
      });
    });

    it('F: pending_moderation shows NO archive, NO delete', async () => {
      const pendingProd = { ...baseProductRaw, status: 'pending_moderation' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([pendingProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.queryByText('В архив')).toBeNull();
        expect(screen.queryByText('Снять с продажи')).toBeNull();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
      });
    });

    it('G: in_review shows NO archive, NO delete', async () => {
      const inReviewProd = { ...baseProductRaw, status: 'in_review' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([inReviewProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.queryByText('В архив')).toBeNull();
        expect(screen.queryByText('Снять с продажи')).toBeNull();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
      });
    });

    it('H: hidden and blocked show NO archive, NO delete', async () => {
      const hiddenProd = { ...baseProductRaw, id: 'prod-h', status: 'hidden' };
      const blockedProd = { ...baseProductRaw, id: 'prod-b', status: 'blocked' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([hiddenProd, blockedProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getAllByText('Платье миди').length).toBe(2);
      });

      // Hidden
      const menuBtnH = screen.getByTestId('product-actions-menu-btn-prod-h');
      fireEvent.click(menuBtnH);
      expect(screen.queryByText('В архив')).toBeNull();
      expect(screen.queryByText('Снять с продажи')).toBeNull();
      expect(screen.queryByText('Удалить черновик')).toBeNull();
      fireEvent.click(menuBtnH); // close

      // Blocked
      const menuBtnB = screen.getByTestId('product-actions-menu-btn-prod-b');
      fireEvent.click(menuBtnB);
      expect(screen.queryByText('В архив')).toBeNull();
      expect(screen.queryByText('Снять с продажи')).toBeNull();
      expect(screen.queryByText('Удалить черновик')).toBeNull();
    });

    it('I: archived shows NO lifecycle mutation actions', async () => {
      const archivedProd = { ...baseProductRaw, status: 'archived' };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([archivedProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      await waitFor(() => {
        expect(screen.queryByText('В архив')).toBeNull();
        expect(screen.queryByText('Снять с продажи')).toBeNull();
        expect(screen.queryByText('Удалить черновик')).toBeNull();
      });
    });
  });

  // ==========================================
  // DELETE (J - O)
  // ==========================================
  describe('Delete Draft Flow & Invariants', () => {
    it('J: delete requires confirmation modal', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open menu and click "Удалить черновик"
      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Удалить черновик'));

      // Modal appears
      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeTruthy();
        expect(screen.getByText('Удалить черновик?')).toBeTruthy();
        expect(screen.getByText('Карточка будет удалена без возможности восстановления.')).toBeTruthy();
        expect(screen.getByTestId('confirm-delete-btn')).toBeTruthy();
        expect(screen.getByTestId('cancel-lifecycle-btn')).toBeTruthy();
      });

      // API not called yet
      expect(deleteSellerProduct).not.toHaveBeenCalled();
    });

    it('K: cancel => no request and modal closes', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Удалить черновик'));

      await waitFor(() => {
        expect(screen.getByText('Удалить черновик?')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('cancel-lifecycle-btn'));

      await waitFor(() => {
        expect(screen.queryByText('Удалить черновик?')).toBeNull();
      });

      expect(deleteSellerProduct).not.toHaveBeenCalled();
    });

    it('L: successful delete removes row and closes active drawer', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);
      vi.mocked(deleteSellerProduct).mockResolvedValueOnce(undefined);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open drawer by clicking row
      fireEvent.click(screen.getByText('Платье миди'));
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      });

      // Delete from drawer action menu
      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      fireEvent.click(screen.getByText('Удалить черновик'));

      // Confirm in modal
      await waitFor(() => {
        expect(screen.getByTestId('confirm-delete-btn')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('confirm-delete-btn'));

      await waitFor(() => {
        expect(deleteSellerProduct).toHaveBeenCalledWith('prod-lifecycle-1');
        // Row is removed
        expect(screen.queryByText('Платье миди')).toBeNull();
        // Drawer is closed
        expect(screen.queryByTestId('seller-product-drawer')).toBeNull();
      });
    });

    it('M: double click/request prevented while deletion is running', async () => {
      let resolveDelete: () => void = () => {};
      const deletePromise = new Promise<void>((resolve) => {
        resolveDelete = resolve;
      });
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);
      vi.mocked(deleteSellerProduct).mockReturnValue(deletePromise);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Удалить черновик'));

      await waitFor(() => {
        expect(screen.getByTestId('confirm-delete-btn')).toBeTruthy();
      });

      // Click once
      fireEvent.click(screen.getByTestId('confirm-delete-btn'));
      // Click a second time immediately
      fireEvent.click(screen.getByTestId('confirm-delete-btn'));

      expect(deleteSellerProduct).toHaveBeenCalledTimes(1);

      // Confirm button is disabled
      expect((screen.getByTestId('confirm-delete-btn') as HTMLButtonElement).disabled).toBe(true);

      // Resolve
      resolveDelete();
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).toBeNull();
      });
    });

    it('N: product_not_disposable keeps row and drawer, shows seller message', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);
      vi.mocked(deleteSellerProduct).mockRejectedValueOnce({
        status: 409,
        code: 'product_not_disposable',
        message: 'Product cannot be deleted because it has history (inventory, orders, etc.). Archive it instead.',
      });

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open drawer
      fireEvent.click(screen.getByText('Платье миди'));
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      });

      // Attempt delete from drawer
      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      fireEvent.click(screen.getByText('Удалить черновик'));
      fireEvent.click(screen.getByTestId('confirm-delete-btn'));

      // Invariant checks:
      await waitFor(() => {
        // Row and drawer remain
        expect(screen.getAllByText('Платье миди').length).toBeGreaterThanOrEqual(2);
        // Drawer remains
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
        // Seller-facing message shown
        expect(
          screen.getByText(
            'Этот черновик нельзя удалить: с товаром уже связаны складские или другие операции. Его можно отправить в архив.'
          )
        ).toBeTruthy();
        // Archive action is provided inside error banner
        expect(screen.getByTestId('lifecycle-error-archive-cta')).toBeTruthy();
      });
    });

    it('N1: preserved non-disposable draft retains canonical ID/status after 409 and edit link targets /products/:id/edit', async () => {
      const candidateDraft: any = {
        id: '24758527-bdf4-4c9d-8332-d6fdbdcc2a97',
        title: 'худифсвмыаываа',
        slug: 'hoodie-candidate',
        status: 'draft',
        price: 1222,
        priceCents: 122200,
        totalStock: 22,
        availableStock: 21,
        categoryId: 'c741aa40-4f5f-4b58-8581-5cfae5e77c16',
        categoryName: 'Худи',
        sizes: [
          { size: 'Белый · XL', stock: 5 },
          { size: 'Красный · L', stock: 6 },
          { size: 'Красный · XL', stock: 5 },
          { size: 'Белый · L', stock: 5 },
        ],
        variants: [
          { id: 'v1', size: 'XL', colorName: 'Белый', totalStock: 5, availableStock: 5 },
          { id: 'v2', size: 'L', colorName: 'Красный', totalStock: 6, availableStock: 6 },
          { id: 'v3', size: 'XL', colorName: 'Красный', totalStock: 5, availableStock: 5 },
          { id: 'v4', size: 'L', colorName: 'Белый', totalStock: 5, availableStock: 5 },
        ],
        images: [
          { id: 'img-1', url: 'http://cdn/1.jpg', position: 0 },
        ],
      };

      vi.mocked(getSellerProducts).mockResolvedValue([candidateDraft]);
      vi.mocked(deleteSellerProduct).mockRejectedValueOnce({
        status: 409,
        code: 'product_not_disposable',
        message: 'Product cannot be deleted because it has history (inventory, orders, etc.). Archive it instead.',
      });

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('худифсвмыаываа')).toBeTruthy();
      });

      // 1. Open drawer
      fireEvent.click(screen.getByText('худифсвмыаываа'));
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      });

      // 2. Attempt delete -> 409 product_not_disposable
      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      fireEvent.click(screen.getByText('Удалить черновик'));
      fireEvent.click(screen.getByTestId('confirm-delete-btn'));

      // 3. Invariants A & B: Product row and drawer remain with status "Черновик" and 21 available stock
      await waitFor(() => {
        expect(screen.getAllByText('худифсвмыаываа').length).toBeGreaterThanOrEqual(2);
        expect(screen.getAllByText('Черновик').length).toBeGreaterThanOrEqual(1);
        expect(screen.getByText('21 шт. на складе')).toBeTruthy();
        expect(screen.getByTestId('lifecycle-error-message')).toBeTruthy();
      });

      // 4. Invariant C: Navigation target retains exact canonical product ID
      const editCta = screen.getByRole('link', { name: /Продолжить заполнение/i });
      expect(editCta.getAttribute('href')).toBe('/products/24758527-bdf4-4c9d-8332-d6fdbdcc2a97/edit');
    });

    it('O: invalid_status refreshes list and displays status change notification', async () => {
      vi.mocked(getSellerProducts).mockResolvedValue([baseProductRaw]);
      vi.mocked(deleteSellerProduct).mockRejectedValueOnce({
        status: 409,
        code: 'invalid_status',
        message: 'Only draft products can be hard deleted',
      });

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Удалить черновик'));
      fireEvent.click(screen.getByTestId('confirm-delete-btn'));

      await waitFor(() => {
        expect(
          screen.getByText('Статус товара изменился. Обновите список и попробуйте снова.')
        ).toBeTruthy();
        // getSellerProducts called to refresh list
        expect(getSellerProducts).toHaveBeenCalledTimes(2);
      });
    });
  });

  // ==========================================
  // ARCHIVE (P - S)
  // ==========================================
  describe('Archive / Stop Selling Flow & Invariants', () => {
    it('P: published archive confirmation uses "Снять товар с продажи?" copy', async () => {
      const publishedProd = {
        ...baseProductRaw,
        status: 'published',
        actualVisibility: true,
        storefrontUrl: 'http://shop.zamk/products/dress-midi',
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([publishedProd]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Снять с продажи'));

      await waitFor(() => {
        expect(screen.getByText('Снять товар с продажи?')).toBeTruthy();
        expect(
          screen.getByText('Товар исчезнет из магазина. Остатки и история операций сохранятся.')
        ).toBeTruthy();
        expect(screen.getByTestId('confirm-archive-btn').textContent).toBe('Снять с продажи');
      });
    });

    it('Q: successful published archive: status -> archived, Продажа -> "Снят с продажи", storefront CTA removed, stock unchanged', async () => {
      const publishedProd = {
        ...baseProductRaw,
        status: 'published',
        actualVisibility: true,
        storefrontUrl: 'http://shop.zamk/products/dress-midi',
      };
      const archivedProd = {
        ...publishedProd,
        status: 'archived',
        actualVisibility: false,
        storefrontUrl: undefined,
      };
      vi.mocked(getSellerProducts)
        .mockResolvedValueOnce([publishedProd])
        .mockResolvedValue([archivedProd]);
      vi.mocked(archiveSellerProduct).mockResolvedValueOnce(undefined);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open drawer to observe instant transition
      fireEvent.click(screen.getByText('Платье миди'));
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
        expect(screen.getByText('В продаже')).toBeTruthy();
        expect(screen.getByText('Открыть в магазине')).toBeTruthy();
      });

      // Trigger "Снять с продажи" from drawer
      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      fireEvent.click(screen.getByText('Снять с продажи'));
      fireEvent.click(screen.getByTestId('confirm-archive-btn'));

      await waitFor(() => {
        expect(archiveSellerProduct).toHaveBeenCalledWith('prod-lifecycle-1');
        // Lifecycle badge is now "В архиве"
        expect(screen.getAllByText('В архиве').length).toBeGreaterThan(0);
        // Storefront indicator is now "Снят с продажи"
        expect(screen.getByText('Снят с продажи')).toBeTruthy();
        // Storefront CTA link is removed
        expect(screen.queryByText('Открыть в магазине')).toBeNull();
        // Stock remains unchanged
        expect(screen.getByText('10 шт. на складе')).toBeTruthy();
      });
    });

    it('R: draft archive transitions status to archived', async () => {
      const archivedDraft = {
        ...baseProductRaw,
        status: 'archived',
        actualVisibility: false,
        storefrontUrl: undefined,
      };
      vi.mocked(getSellerProducts)
        .mockResolvedValueOnce([baseProductRaw])
        .mockResolvedValue([archivedDraft]);
      vi.mocked(archiveSellerProduct).mockResolvedValueOnce(undefined);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('В архив'));

      await waitFor(() => {
        expect(screen.getByText('Переместить товар в архив?')).toBeTruthy();
        expect(
          screen.getByText('Товар останется в истории, но больше не будет использоваться для продажи.')
        ).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('confirm-archive-btn'));

      await waitFor(() => {
        expect(archiveSellerProduct).toHaveBeenCalledWith('prod-lifecycle-1');
        expect(screen.getAllByText('В архиве').length).toBeGreaterThan(0);
      });
    });

    it('R1: approved archive from drawer: shows "В архив", opens modal, calls canonical archiveSellerProduct, transitions to archived with stock unchanged', async () => {
      const approvedProd = {
        ...baseProductRaw,
        status: 'approved',
        totalStock: 15,
        availableStock: 15,
      };
      const archivedApproved = {
        ...approvedProd,
        status: 'archived',
      };
      vi.mocked(getSellerProducts)
        .mockResolvedValueOnce([approvedProd])
        .mockResolvedValue([archivedApproved]);
      vi.mocked(archiveSellerProduct).mockResolvedValueOnce(undefined);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open drawer
      fireEvent.click(screen.getByText('Платье миди'));
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      });

      // Drawer action menu shows "В архив"
      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      expect(screen.getByText('В архив')).toBeTruthy();
      expect(screen.queryByText('Удалить черновик')).toBeNull();
      expect(screen.queryByText('Снять с продажи')).toBeNull();

      fireEvent.click(screen.getByText('В архив'));

      // Check confirmation modal copy for approved
      await waitFor(() => {
        expect(screen.getByText('Переместить товар в архив?')).toBeTruthy();
        expect(
          screen.getByText('Товар останется в истории, но больше не будет использоваться для продажи.')
        ).toBeTruthy();
        expect(screen.getByTestId('confirm-archive-btn').textContent).toBe('В архив');
      });

      fireEvent.click(screen.getByTestId('confirm-archive-btn'));

      await waitFor(() => {
        expect(archiveSellerProduct).toHaveBeenCalledWith('prod-lifecycle-1');
        expect(screen.getAllByText('В архиве').length).toBeGreaterThan(0);
        // Stock remains intact
        expect(screen.getByText('10 шт. на складе')).toBeTruthy();
      });
    });

    it('R2: out_of_stock archive from drawer: shows "В архив", calls canonical archiveSellerProduct, transitions to archived', async () => {
      const outOfStockProd = {
        ...baseProductRaw,
        status: 'out_of_stock',
        totalStock: 0,
        availableStock: 0,
      };
      const archivedOutOfStock = {
        ...outOfStockProd,
        status: 'archived',
      };
      vi.mocked(getSellerProducts)
        .mockResolvedValueOnce([outOfStockProd])
        .mockResolvedValue([archivedOutOfStock]);
      vi.mocked(archiveSellerProduct).mockResolvedValueOnce(undefined);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Open drawer
      fireEvent.click(screen.getByText('Платье миди'));
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      });

      // Drawer action menu shows "В архив"
      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      expect(screen.getByText('В архив')).toBeTruthy();
      expect(screen.queryByText('Удалить черновик')).toBeNull();

      fireEvent.click(screen.getByText('В архив'));
      await waitFor(() => {
        expect(screen.getByText('Переместить товар в архив?')).toBeTruthy();
      });
      fireEvent.click(screen.getByTestId('confirm-archive-btn'));

      await waitFor(() => {
        expect(archiveSellerProduct).toHaveBeenCalledWith('prod-lifecycle-1');
        expect(screen.getAllByText('В архиве').length).toBeGreaterThan(0);
      });
    });

    it('S: archive 409 does not leave optimistic archived state', async () => {
      const publishedProd = {
        ...baseProductRaw,
        status: 'published',
        actualVisibility: true,
        storefrontUrl: 'http://shop.zamk/products/dress-midi',
      };
      vi.mocked(getSellerProducts).mockResolvedValue([publishedProd]);
      vi.mocked(archiveSellerProduct).mockRejectedValueOnce({
        status: 409,
        code: 'invalid_transition',
        message: 'Cannot archive this product from its current status',
      });

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Снять с продажи'));
      fireEvent.click(screen.getByTestId('confirm-archive-btn'));

      await waitFor(() => {
        // Status remained "Опубликован", not "В архиве"
        expect(screen.getAllByText('Опубликован').length).toBeGreaterThan(0);
        // Error message shown
        expect(
          screen.getByText('Статус товара изменился, поэтому действие больше недоступно.')
        ).toBeTruthy();
      });
    });
  });

  // ==========================================
  // ROW MENU (T - U)
  // ==========================================
  describe('Row Menu Mechanics', () => {
    it('T: click ⋯ does not open drawer', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // Click the ⋯ action button
      const menuBtn = screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1');
      fireEvent.click(menuBtn);

      // Menu is open
      await waitFor(() => {
        expect(screen.getByTestId('product-actions-menu-prod-lifecycle-1')).toBeTruthy();
      });

      // Drawer is NOT open!
      expect(screen.queryByTestId('seller-product-drawer')).toBeNull();
    });

    it('U: row and drawer invoke same canonical actions and modal', async () => {
      vi.mocked(getSellerProducts).mockResolvedValue([baseProductRaw]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      // 1. From row menu: opens delete modal
      fireEvent.click(screen.getByTestId('product-actions-menu-btn-prod-lifecycle-1'));
      fireEvent.click(screen.getByText('Удалить черновик'));
      expect(screen.getByText('Удалить черновик?')).toBeTruthy();
      fireEvent.click(screen.getByTestId('cancel-lifecycle-btn'));

      // 2. From drawer menu: opens identical delete modal
      fireEvent.click(screen.getByText('Платье миди')); // open drawer
      await waitFor(() => {
        expect(screen.getByTestId('seller-product-drawer')).toBeTruthy();
      });

      fireEvent.click(screen.getByTestId('drawer-actions-menu-btn'));
      fireEvent.click(screen.getByText('Удалить черновик'));
      expect(screen.getByText('Удалить черновик?')).toBeTruthy();
      fireEvent.click(screen.getByTestId('cancel-lifecycle-btn'));
    });
  });

  // ==========================================
  // REGRESSION (V - X)
  // ==========================================
  describe('Assortment Regressions', () => {
    it('V: existing submit-moderation still works from drawer', async () => {
      vi.mocked(getSellerProducts).mockResolvedValue([baseProductRaw]);
      vi.mocked(submitSellerProductModeration).mockResolvedValueOnce(undefined);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByText('Платье миди'));
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Отправить на модерацию/i })).toBeTruthy();
      });

      fireEvent.click(screen.getByRole('button', { name: /Отправить на модерацию/i }));

      await waitFor(() => {
        expect(submitSellerProductModeration).toHaveBeenCalledWith('prod-lifecycle-1');
      });
    });

    it('W: existing Color · Size labels remain unchanged in drawer', async () => {
      vi.mocked(getSellerProducts).mockResolvedValueOnce([baseProductRaw]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      fireEvent.click(screen.getByText('Платье миди'));

      await waitFor(() => {
        expect(screen.getByText('Черный · S')).toBeTruthy();
        expect(screen.getByText('Черный · M')).toBeTruthy();
      });
    });

    it('X: existing stock/status separation remains intact in table row', async () => {
      const approvedZeroStock = {
        ...baseProductRaw,
        status: 'approved',
        totalStock: 0,
        availableStock: 0,
        variants: [],
      };
      vi.mocked(getSellerProducts).mockResolvedValueOnce([approvedZeroStock]);

      render(
        <MemoryRouter initialEntries={['/products']}>
          <SellerProducts />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByText('Платье миди')).toBeTruthy();
      });

      const rows = screen.getAllByRole('row');
      const productRow = rows[1];
      const cells = productRow.querySelectorAll('td');

      // Column 4 is Status: strictly "Одобрен"
      expect(cells[4].textContent).toContain('Одобрен');
      expect(cells[4].textContent).not.toContain('Требуется поставка');

      // Column 5 is Warehouse: "Нет на складе" + "Требуется поставка"
      expect(cells[5].textContent).toContain('Нет на складе');
      expect(cells[5].textContent).toContain('Требуется поставка');
    });
  });
});
