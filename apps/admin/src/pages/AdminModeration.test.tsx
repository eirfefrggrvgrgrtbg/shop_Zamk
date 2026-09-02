// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AdminModerationReviews } from './AdminModerationReviews';
import { AdminModerationQueue } from './AdminModerationQueue';
import { AdminModerationSellers } from './AdminModerationSellers';
import { ReviewDetailOverlay } from './ReviewDetailOverlay';
import { AdminLayout } from '../components/AdminLayout';
import * as adminReviewsApi from '../api/adminReviews';
import * as adminProductsApi from '../api/adminProducts';
import * as adminSellersApi from '@zamk/api-client/src/admin';

// Mock auth context
vi.mock('../contexts/AdminAuthContext', () => ({
  useAdminAuth: () => ({
    user: { id: 'admin-1', email: 'admin@zamk.ru', role: 'admin' },
    isAuthenticated: true,
    isLoading: false,
    staff: { id: 'staff-1', permissions: ['*'] },
    hasPermission: () => true,
    hasAnyPermission: () => true,
    logout: vi.fn(),
  }),
}));

// Mock API calls
vi.mock('../api/adminReviews', () => ({
  getAdminReviews: vi.fn(),
  getAdminReview: vi.fn(),
  moderateAdminReview: vi.fn(),
  getAdminReviewErrorMessage: (_err: any, fallback: string) => fallback,
}));

vi.mock('../api/adminProducts', () => ({
  getModerationProducts: vi.fn(),
  getAdminProduct: vi.fn().mockResolvedValue({
    id: 'prod-coat-1',
    title: 'Dev Wool Coat',
    sellerName: 'ZAMK Dev Store',
    priceCents: 1500000,
    mainImageUrl: 'https://example.com/coat.jpg',
  }),
  getAdminProductErrorMessage: (_err: any, fallback: string) => fallback,
}));

vi.mock('@zamk/api-client/src/admin', () => ({
  getAdminSellers: vi.fn(),
  updateAdminSellerStatus: vi.fn(),
  getAdminCategories: vi.fn().mockResolvedValue([]),
  getAdminBrands: vi.fn().mockResolvedValue([]),
}));

const mockReview: adminReviewsApi.AdminReviewView = {
  id: '647bb3fd-d1f2-4e11-80a2-df9f162d15ba',
  productId: 'prod-coat-1',
  productTitle: 'Dev Wool Coat',
  sellerId: 'seller-zamk',
  sellerName: 'ZAMK Dev Store',
  rating: 5,
  title: 'Отличное пальто',
  comment: 'Очень качественная шерсть и хорошая посадка',
  status: 'pending_moderation',
  statusLabel: 'На проверке',
  createdAt: '2026-09-01T13:26:00Z',
};

const mockProduct = {
  id: 'prod-dress-1',
  title: 'Summer Silk Dress',
  sellerName: 'Silk Studio',
  variantsCount: 2,
  status: 'pending_moderation',
  submittedAt: '2026-09-01T14:00:00Z',
};

const mockSellers = [
  {
    id: 'seller-review-1',
    brandName: 'Nordic Knitwear',
    ownerName: 'Anna Lind',
    ownerEmail: 'anna@nordic.se',
    status: 'pending_review',
    createdAt: '2026-09-01T12:00:00Z',
  },
  {
    id: 'seller-setup-1',
    brandName: 'Baltic Boots',
    ownerName: 'Jonas Kazlauskas',
    ownerEmail: 'jonas@baltic.lt',
    status: 'pending_setup',
    createdAt: '2026-09-01T11:00:00Z',
  },
  {
    id: 'seller-pending-1',
    brandName: 'Alpine Gear',
    ownerName: 'Lucas Meyer',
    ownerEmail: 'lucas@alpine.ch',
    status: 'pending',
    createdAt: '2026-09-01T10:00:00Z',
  },
  {
    id: 'seller-active-1',
    brandName: 'Established Fashion',
    ownerName: 'Elena Rostova',
    ownerEmail: 'elena@fashion.ru',
    status: 'active',
    createdAt: '2026-08-01T10:00:00Z',
  },
];

describe('Admin Moderation Information Architecture', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (adminReviewsApi.getAdminReviews as any).mockResolvedValue([mockReview]);
    (adminReviewsApi.getAdminReview as any).mockResolvedValue(mockReview);
    (adminProductsApi.getModerationProducts as any).mockResolvedValue({ items: [mockProduct], totalCount: 1 });
    (adminSellersApi.getAdminSellers as any).mockResolvedValue({ items: mockSellers, totalCount: mockSellers.length });
  });

  afterEach(() => {
    cleanup();
  });

  it('renders sidebar with Модерация and actionable seller count (including pending_review)', async () => {
    render(
      <MemoryRouter initialEntries={['/moderation/reviews']}>
        <AdminLayout>
          <div>Moderation Content</div>
        </AdminLayout>
      </MemoryRouter>
    );

    // Sidebar should contain "Модерация"
    expect(screen.getAllByText('Модерация').length).toBeGreaterThan(0);

    // Sidebar sub-items should be present
    expect(screen.getByText('Очередь')).toBeDefined();
    expect(screen.getAllByText('Продавцы').length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('Товары').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Отзывы')).toBeDefined();

    // Standalone "Отзывы" should NOT be present in the main sidebar root level
    const links = screen.getAllByRole('link');
    const standaloneReviewsLink = links.find((l) => l.getAttribute('href') === '/reviews');
    expect(standaloneReviewsLink).toBeUndefined();
  });

  it('renders Review Moderation in Master-Detail layout with Russian labels and details', async () => {
    render(
      <MemoryRouter initialEntries={['/moderation/reviews']}>
        <AdminModerationReviews />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText('Dev Wool Coat').length).toBeGreaterThan(0);
    });

    // Master-Detail shows review title & snippet
    expect(screen.getAllByText('Отличное пальто').length).toBe(2);
    expect(screen.getAllByText('Очень качественная шерсть и хорошая посадка').length).toBeGreaterThan(0);

    // Status is in Russian
    expect(screen.getAllByText('На проверке').length).toBeGreaterThan(0);

    // Detail pane displays action buttons for pending_moderation
    expect(screen.getByRole('button', { name: 'Опубликовать' })).toBeDefined();
    expect(screen.getByRole('button', { name: 'Отклонить' })).toBeDefined();
  });

  it('opens rejection modal with reason presets and requires non-empty comment', async () => {
    render(
      <MemoryRouter initialEntries={['/moderation/reviews']}>
        <AdminModerationReviews />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Отклонить' })).toBeDefined();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Отклонить' }));

    // Modal opens
    expect(screen.getByRole('heading', { name: 'Отклонить отзыв' })).toBeDefined();
    expect(screen.getByText('Шаблоны причин:')).toBeDefined();
    expect(screen.getByText('Не относится к товару')).toBeDefined();
    expect(screen.getByText('Спам / реклама')).toBeDefined();

    // Click preset chip
    fireEvent.click(screen.getByText('Не относится к товару'));

    const textarea = screen.getByPlaceholderText('Опишите причину отклонения...') as HTMLTextAreaElement;
    expect(textarea.value).toBe('Не относится к товару');

    // Submit rejection
    const submitBtn = screen.getByRole('button', { name: 'Отклонить отзыв' });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(adminReviewsApi.moderateAdminReview).toHaveBeenCalledWith(
        '647bb3fd-d1f2-4e11-80a2-df9f162d15ba',
        'reject',
        'Не относится к товару'
      );
    });
  });

  it('renders unified moderation inbox (Очередь) with pending_review sellers and dossier links', async () => {
    render(
      <MemoryRouter initialEntries={['/moderation/queue']}>
        <AdminModerationQueue />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getAllByText(/Отзыв на «Dev Wool Coat»/i).length).toBeGreaterThan(0);
      expect(screen.getAllByText('Summer Silk Dress').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Nordic Knitwear').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Baltic Boots').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Alpine Gear').length).toBeGreaterThan(0);
    });

    // Select pending_review seller item in queue
    const sellerCard = screen.getByText('Nordic Knitwear');
    fireEvent.click(sellerCard);

    // Verify link to canonical seller dossier and no direct activation button
    const sellerDossierLink = screen.getByRole('link', { name: /Перейти к проверке продавца/i });
    expect(sellerDossierLink.getAttribute('href')).toBe('/sellers/seller-review-1');
    expect(screen.queryByRole('button', { name: /Активировать/i })).toBeNull();
  });

  it('renders sellers moderation page (/moderation/sellers) with actionable states only', async () => {
    render(
      <MemoryRouter initialEntries={['/moderation/sellers']}>
        <AdminModerationSellers />
      </MemoryRouter>
    );

    await waitFor(() => {
      // Actionable sellers appear
      expect(screen.getAllByText('Nordic Knitwear').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Baltic Boots').length).toBeGreaterThan(0);
      expect(screen.getAllByText('Alpine Gear').length).toBeGreaterThan(0);
    });

    // Active seller does NOT appear in decision queue
    expect(screen.queryByText('Established Fashion')).toBeNull();

    // Verify link to dossier and absence of direct activation
    const dossierLinks = screen.getAllByRole('link', { name: /Перейти к проверке продавца/i });
    expect(dossierLinks.length).toBe(3);
    expect(screen.queryByRole('button', { name: /Активировать/i })).toBeNull();
  });

  describe('ReviewDetailOverlay Action Matrix & Deep Linking', () => {
    const dummyGetStatusBadge = (status: string, label: string) => <span>{label || status}</span>;

    it('shows exact actions for pending_moderation: Опубликовать, Отклонить, Заблокировать отзыв (no Скрыть)', async () => {
      const onAction = vi.fn();
      render(
        <ReviewDetailOverlay
          isOpen={true}
          onClose={vi.fn()}
          review={{ ...mockReview, status: 'pending_moderation', statusLabel: 'На проверке' }}
          onAction={onAction}
          isSubmitting={false}
          getStatusBadge={dummyGetStatusBadge}
        />
      );

      expect(screen.getByRole('button', { name: 'Опубликовать' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Отклонить' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Заблокировать отзыв' })).toBeDefined();
      expect(screen.queryByRole('button', { name: 'Скрыть' })).toBeNull();
    });

    it('shows exact actions for published: Скрыть, Заблокировать отзыв (no Опубликовать, no Отклонить)', async () => {
      const onAction = vi.fn();
      render(
        <ReviewDetailOverlay
          isOpen={true}
          onClose={vi.fn()}
          review={{ ...mockReview, status: 'published', statusLabel: 'Опубликован' }}
          onAction={onAction}
          isSubmitting={false}
          getStatusBadge={dummyGetStatusBadge}
        />
      );

      expect(screen.getByRole('button', { name: 'Скрыть' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Заблокировать отзыв' })).toBeDefined();
      expect(screen.queryByRole('button', { name: 'Опубликовать' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Отклонить' })).toBeNull();
    });

    it('shows exact actions for rejected: Опубликовать, Заблокировать отзыв (no Отклонить, no Скрыть)', async () => {
      const onAction = vi.fn();
      render(
        <ReviewDetailOverlay
          isOpen={true}
          onClose={vi.fn()}
          review={{ ...mockReview, status: 'rejected', statusLabel: 'Отклонён', moderationComment: 'Спам' }}
          onAction={onAction}
          isSubmitting={false}
          getStatusBadge={dummyGetStatusBadge}
        />
      );

      expect(screen.getByRole('button', { name: 'Опубликовать' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Заблокировать отзыв' })).toBeDefined();
      expect(screen.queryByRole('button', { name: 'Отклонить' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Скрыть' })).toBeNull();
      expect(screen.getByText('Причина отклонения')).toBeDefined();
    });

    it('shows exact actions for hidden: Опубликовать, Заблокировать отзыв (no Отклонить, no Скрыть)', async () => {
      const onAction = vi.fn();
      render(
        <ReviewDetailOverlay
          isOpen={true}
          onClose={vi.fn()}
          review={{ ...mockReview, status: 'hidden', statusLabel: 'Скрыт' }}
          onAction={onAction}
          isSubmitting={false}
          getStatusBadge={dummyGetStatusBadge}
        />
      );

      expect(screen.getByRole('button', { name: 'Опубликовать' })).toBeDefined();
      expect(screen.getByRole('button', { name: 'Заблокировать отзыв' })).toBeDefined();
      expect(screen.queryByRole('button', { name: 'Отклонить' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Скрыть' })).toBeNull();
    });

    it('shows blocked notice and no action buttons for blocked', async () => {
      const onAction = vi.fn();
      render(
        <ReviewDetailOverlay
          isOpen={true}
          onClose={vi.fn()}
          review={{ ...mockReview, status: 'blocked', statusLabel: 'Заблокирован' }}
          onAction={onAction}
          isSubmitting={false}
          getStatusBadge={dummyGetStatusBadge}
        />
      );

      expect(screen.getByText('Отзыв заблокирован в системе.')).toBeDefined();
      expect(screen.queryByRole('button', { name: 'Опубликовать' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Отклонить' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Скрыть' })).toBeNull();
      expect(screen.queryByRole('button', { name: 'Заблокировать отзыв' })).toBeNull();
    });

    it('executes block action with required comment and calls onAction with block', async () => {
      const onAction = vi.fn().mockResolvedValue(undefined);
      render(
        <ReviewDetailOverlay
          isOpen={true}
          onClose={vi.fn()}
          review={{ ...mockReview, status: 'pending_moderation', statusLabel: 'На проверке' }}
          onAction={onAction}
          isSubmitting={false}
          getStatusBadge={dummyGetStatusBadge}
        />
      );

      fireEvent.click(screen.getByRole('button', { name: 'Заблокировать отзыв' }));

      expect(screen.getByText(/Причина блокировки отзыва/i)).toBeDefined();

      // Enter comment and submit
      const textarea = screen.getByPlaceholderText('Опишите причину...') as HTMLTextAreaElement;
      fireEvent.change(textarea, { target: { value: 'Нарушение правил' } });

      fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

      await waitFor(() => {
        expect(onAction).toHaveBeenCalledWith('block', '647bb3fd-d1f2-4e11-80a2-df9f162d15ba', 'Нарушение правил');
      });
    });

    it('opens overlay directly when ?view=detail is in deep-link URL', async () => {
      render(
        <MemoryRouter initialEntries={['/moderation/reviews?selected=647bb3fd-d1f2-4e11-80a2-df9f162d15ba&view=detail']}>
          <AdminModerationReviews />
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(screen.getByTestId('review-detail-overlay')).toBeDefined();
        expect(screen.getByText('Оригинальный товар')).toBeDefined();
      });
    });
  });
});
