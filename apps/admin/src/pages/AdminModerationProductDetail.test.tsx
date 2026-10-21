// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AdminModerationProductDetail } from './AdminModerationProductDetail';
import * as adminProductsApi from '../api/adminProducts';

vi.mock('../api/adminProducts', async (importOriginal) => {
  const actual = await importOriginal<typeof adminProductsApi>();
  return {
    ...actual,
    getAdminProduct: vi.fn(),
    getAdminProductModerationHistory: vi.fn().mockResolvedValue({ items: [] }),
    getModerationProducts: vi.fn().mockResolvedValue({ items: [] }),
    startProductReview: vi.fn().mockResolvedValue(undefined),
    approveProduct: vi.fn().mockResolvedValue(undefined),
    rejectProduct: vi.fn().mockResolvedValue(undefined),
    blockProduct: vi.fn().mockResolvedValue(undefined),
    createProductPreviewLink: vi.fn().mockResolvedValue({ pageUrl: 'https://example.com/p', expiresAt: '2026-10-15T00:00:00Z' }),
    getAdminProductErrorMessage: (_err: any, fallback: string) => fallback,
  };
});

describe('AdminModerationProductDetail — AI Panel & Moderation Controls Boundary', () => {
  const baseProduct: adminProductsApi.AdminProductView = {
    id: 'prod-12345678-0000-0000-0000-000000000001',
    title: 'Шерстяное оверсайз пальто',
    description: 'Теплое шерстяное пальто премиального кроя.',
    sellerId: 'seller-1',
    sellerName: 'ZAMK Fashion House',
    sellerIsActive: true,
    categoryId: 'cat-coats',
    category: 'Пальто',
    brandId: 'brand-1',
    brand: 'ZAMK Atelier',
    price: 35000,
    priceCents: 3500000,
    currency: 'RUB',
    status: 'in_review',
    statusLabel: 'На проверке',
    image: 'https://example.com/coat1.jpg',
    gallery: [
      { id: 'img-1', url: 'https://example.com/coat1.jpg', altText: 'Фото 1' },
      { id: 'img-2', url: 'https://example.com/coat2.jpg', altText: 'Фото 2' },
    ],
    variants: [
      { id: 'var-1', label: 'Черный / M', sku: 'ZMK-COAT-BLK-M', price: 35000, isActive: true },
    ],
    vision: {
      status: 'succeeded',
      categoryConsistency: 'contradictory',
      observedCategoryText: 'На изображениях распознана футболка, пальто не обнаружено',
      moderation: {
        recommendation: 'review_recommended',
        recommendationReason: 'Обнаружено расхождение между категорией Пальто и фото Футболки',
        contradictions: [
          {
            field: 'category',
            declared: 'Пальто',
            observed: 'Футболка',
            reason: 'На всех представленных фото видна легкая футболка с круглым вырезом',
          },
        ],
        qualityIssues: ['vision.quality.blur'],
      },
      silhouette: 'vision.silhouette.regular',
      dominantColors: [
        { id: 'col-black', code: 'black', name: 'Черный', hex: '#000000' },
      ],
      imageSetConsistency: false,
      evidence: [
        {
          field: 'category',
          valueId: 't_shirt',
          imageIds: ['img-1'],
        },
      ],
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(adminProductsApi.getAdminProduct).mockResolvedValue(baseProduct);
  });

  const renderComponent = (productId = 'prod-12345678-0000-0000-0000-000000000001') => {
    return render(
      <MemoryRouter initialEntries={[`/moderation/products/${productId}`]}>
        <Routes>
          <Route path="/moderation/products/:productId" element={<AdminModerationProductDetail />} />
        </Routes>
      </MemoryRouter>
    );
  };

  // Integration: renders AI-анализ panel inside moderation review
  it('renders AI-анализ panel in review flow alongside product data', async () => {
    renderComponent();

    // Check product loaded
    const title = await screen.findByText('Шерстяное оверсайз пальто');
    expect(title).toBeDefined();

    // Check AI panel is rendered
    expect(screen.getByTestId('ai-vision-panel')).toBeDefined();
    expect(screen.getByText('AI-анализ изображений')).toBeDefined();
    expect(screen.getByText('Рекомендуется дополнительная проверка')).toBeDefined();
    expect(screen.getByText('Возможное несоответствие категории')).toBeDefined();
    expect(screen.getByText(/На изображениях распознана футболка/)).toBeDefined();
  });

  // R. AUTOMATIC MODERATION ACTIONS = NONE
  it('R: verifies NO automatic moderation actions are introduced in the AI panel', async () => {
    renderComponent();

    await screen.findByText('Шерстяное оверсайз пальто');

    const aiPanel = screen.getByTestId('ai-vision-panel');

    // Panel must not contain any action buttons that trigger approve, reject, or revision
    const panelButtons = aiPanel.querySelectorAll('button');
    panelButtons.forEach((btn) => {
      const text = btn.textContent || '';
      expect(text).not.toMatch(/Одобрить/i);
      expect(text).not.toMatch(/Отклонить/i);
      expect(text).not.toMatch(/Вернуть на исправление/i);
      expect(text).not.toMatch(/Заблокировать/i);
      expect(text).not.toMatch(/Применить решение/i);
    });

    // Zero API mutation calls executed on render
    expect(adminProductsApi.approveProduct).not.toHaveBeenCalled();
    expect(adminProductsApi.rejectProduct).not.toHaveBeenCalled();
    expect(adminProductsApi.blockProduct).not.toHaveBeenCalled();
    expect(adminProductsApi.startProductReview).not.toHaveBeenCalled();
  });

  // S. EXISTING MODERATION ACTIONS REMAIN UNCHANGED
  it('S: verifies existing moderation actions remain functional and require manual admin input', async () => {
    renderComponent();

    await screen.findByText('Шерстяное оверсайз пальто');

    // Existing primary moderation buttons
    const approveBtn = screen.getByTestId('btn-approve');
    const reviseBtn = screen.getByTestId('btn-revise');
    const rejectBtn = screen.getByTestId('btn-reject');

    expect(approveBtn).toBeDefined();
    expect(reviseBtn).toBeDefined();
    expect(rejectBtn).toBeDefined();

    // Test Revise button opens standard modal
    fireEvent.click(reviseBtn);
    expect(screen.getByText('Вернуть товар на исправление')).toBeDefined();

    // Fill minimum required comment
    const commentInput = screen.getByPlaceholderText(/Опишите, что именно нужно поправить/i);
    fireEvent.change(commentInput, { target: { value: 'Категория товара не соответствует изображениям.' } });

    // Submit revision
    const submitBtn = screen.getByTestId('modal-submit-btn');
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(adminProductsApi.rejectProduct).toHaveBeenCalledWith(
        'prod-12345678-0000-0000-0000-000000000001',
        expect.stringContaining('Категория товара не соответствует изображениям.')
      );
    });
  });

  // C5.1: a published product's pending revision is shown and decided without blocking the live product
  it('shows the pending revision matrix and offers approve / return, not block, for a published product', async () => {
    vi.mocked(adminProductsApi.getAdminProduct).mockResolvedValue({
      ...baseProduct,
      status: 'published',
      statusLabel: 'Опубликован',
      variants: [
        { id: 'var-1', label: 'Черный / S', sku: 'BS', price: 100, isActive: true },
        { id: 'var-2', label: 'Белый / M', sku: 'WM', price: 110, isActive: true },
      ],
      pendingRevision: {
        id: 'rev-1',
        variantsChanged: true,
        imagesChanged: false,
        imagesCount: 0,
        images: [],
        imagesMissing: 0,
        variants: [
          { id: 'var-1', label: 'Черный / S', sku: 'BS', price: 100, isActive: true },
          { id: 'var-new', label: 'Черный / M', sku: 'BM', price: 120, isActive: true },
        ],
      },
    });
    renderComponent();

    const block = await screen.findByTestId('pending-revision');
    expect(block.textContent).toContain('Черный / M');
    expect(block.textContent).toContain('Новый');
    expect(block.textContent).toContain('Будет отключён');
    expect(screen.getAllByTestId('pending-revision-variant')).toHaveLength(3);
    expect(screen.getByTestId('btn-approve')).toBeDefined();
    expect(screen.getByTestId('btn-revise')).toBeDefined();
    expect(screen.queryByTestId('btn-reject')).toBeNull();
    expect(screen.queryByTestId('btn-start-review')).toBeNull();
  });

  it('shows the proposed gallery with new photos and flags a new photo without 4:5', async () => {
    vi.mocked(adminProductsApi.getAdminProduct).mockResolvedValue({
      ...baseProduct,
      status: 'published',
      statusLabel: 'Опубликован',
      pendingRevision: {
        id: 'rev-2',
        variantsChanged: false,
        imagesChanged: true,
        imagesCount: 3,
        imagesMissing: 0,
        variants: [],
        images: [
          { id: 'n1', url: 'https://example.com/n1_r.jpg', isMain: true, isNew: true, hasRendition: true },
          { id: 'o1', url: 'https://example.com/o1.jpg', isMain: false, isNew: false, hasRendition: true },
          { id: 'n2', url: 'https://example.com/n2.jpg', isMain: false, isNew: true, hasRendition: false },
        ],
      },
    });
    renderComponent();

    const gallery = await screen.findByTestId('pending-revision-gallery');
    const items = screen.getAllByTestId('pending-revision-image');
    expect(items).toHaveLength(3);
    expect(items.map((i) => i.getAttribute('data-new'))).toEqual(['true', 'false', 'true']);
    expect(gallery.textContent).toContain('Нет 4:5');
    expect(screen.queryByTestId('pending-revision-images-missing')).toBeNull();
  });
});
