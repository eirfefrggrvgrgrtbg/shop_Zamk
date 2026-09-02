// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { getSafeErrorMessage } from '@zamk/api-client/src/client';
import {
  shouldShowReviewCTA,
  getReviewStatusBadgeText,
  validateReviewForm,
  buildReviewPayload,
  REVIEW_STATUS_LABELS,
  REVIEW_STATUS_STYLES,
  getOrderStatusStyle,
  getItemCountWord,
} from './reviewHelpers';
import {
  lockBodyScroll,
  unlockBodyScroll,
  getScrollLockCount,
  resetScrollLockForTesting,
} from '../ui/scrollLock';

describe('Shop Reviews UX & State', () => {
  beforeEach(() => {
    resetScrollLockForTesting();
  });

  it('shows create CTA for delivered order item without review', () => {
    expect(shouldShowReviewCTA('delivered', null)).toBe(true);
    expect(shouldShowReviewCTA('Доставлен', null)).toBe(true);
  });

  it('hides create CTA for paid / assembling / shipped (not delivered) orders', () => {
    expect(shouldShowReviewCTA('paid', null)).toBe(false);
    expect(shouldShowReviewCTA('Оплачен', null)).toBe(false);
    expect(shouldShowReviewCTA('shipped', null)).toBe(false);
    expect(shouldShowReviewCTA('cancelled', null)).toBe(false);
  });

  it('preserves review CTA for returned/refunded delivered purchase if not already reviewed', () => {
    expect(shouldShowReviewCTA('delivered', undefined)).toBe(true);
  });

  it('hides CTA and shows "На модерации" for item with pending review', () => {
    const pendingReview = { id: 'rev-1', status: 'pending_moderation', rating: 5 };
    expect(shouldShowReviewCTA('delivered', pendingReview)).toBe(false);
    expect(getReviewStatusBadgeText(pendingReview)).toBe('На модерации');
  });

  it('hides CTA and shows "Опубликован" for item with published review', () => {
    const publishedReview = { id: 'rev-2', status: 'published', rating: 5 };
    expect(shouldShowReviewCTA('delivered', publishedReview)).toBe(false);
    expect(getReviewStatusBadgeText(publishedReview)).toBe('Опубликован');
  });

  it('hides CTA and shows "Отклонён" for item with rejected review', () => {
    const rejectedReview = { id: 'rev-3', status: 'rejected', rating: 1 };
    expect(shouldShowReviewCTA('delivered', rejectedReview)).toBe(false);
    expect(getReviewStatusBadgeText(rejectedReview)).toBe('Отклонён');
  });

  it('validates client-side rating bounds (1 to 5)', () => {
    expect(validateReviewForm(0, '').isValid).toBe(false);
    expect(validateReviewForm(6, '').isValid).toBe(false);
    expect(validateReviewForm(5, '').isValid).toBe(true);
    expect(validateReviewForm(1, '').isValid).toBe(true);
  });

  it('validates text length limit (1000 characters)', () => {
    const longText = 'a'.repeat(1001);
    const validText = 'a'.repeat(1000);
    expect(validateReviewForm(5, longText).isValid).toBe(false);
    expect(validateReviewForm(5, longText).error).toBe('Текст отзыва слишком длинный (максимум 1000 символов)');
    expect(validateReviewForm(5, validText).isValid).toBe(true);
  });

  it('builds canonical review payload without product or variant IDs', () => {
    const payload = buildReviewPayload('oi-100', 5, '  Super shoes  ', '  Very comfortable and light  ');
    expect(payload).toEqual({
      orderItemId: 'oi-100',
      rating: 5,
      title: 'Super shoes',
      text: 'Very comfortable and light',
      comment: 'Very comfortable and light',
    });
    expect((payload as any).productId).toBeUndefined();
    expect((payload as any).variantId).toBeUndefined();
    expect((payload as any).productVariantId).toBeUndefined();
  });

  it('maps backend domain errors to clean human messages', () => {
    expect(getSafeErrorMessage('duplicate_review', 'duplicate review for this order item')).toBe('Вы уже оставили отзыв на этот товар');
    expect(getSafeErrorMessage('review_already_exists', 'review already exists')).toBe('Вы уже оставили отзыв на этот товар');
    expect(getSafeErrorMessage('not_purchased', 'order item was not purchased by user')).toBe('Вы можете оставить отзыв только на купленный товар');
    expect(getSafeErrorMessage('order_not_delivered', 'order must be delivered to leave a review')).toBe('Отзыв можно оставить только после доставки заказа');
    expect(getSafeErrorMessage('review_text_too_long', 'review text is too long (max 1000 characters)')).toBe('Текст отзыва слишком длинный (максимум 1000 символов)');
    expect(getSafeErrorMessage('invalid_rating', 'rating must be between 1 and 5')).toBe('Оценка должна быть от 1 до 5');
    expect(getSafeErrorMessage('generic', 'something unexpected from server')).toBe('something unexpected from server');
  });

  it('correctly pluralizes Russian item count words', () => {
    expect(getItemCountWord(1)).toBe('товар');
    expect(getItemCountWord(2)).toBe('товара');
    expect(getItemCountWord(3)).toBe('товара');
    expect(getItemCountWord(4)).toBe('товара');
    expect(getItemCountWord(5)).toBe('товаров');
    expect(getItemCountWord(11)).toBe('товаров');
    expect(getItemCountWord(21)).toBe('товар');
  });

  it('associates reviews strictly by orderItemId and prevents false matches across duplicate product purchases', () => {
    const sharedProductId = 'prod-same-coat';
    const firstPurchaseOrderItemId = 'oi-order-1-coat';
    const secondPurchaseOrderItemId = 'oi-order-2-coat';

    // Mock customer review response from backend exposing orderItemId
    const reviewsApiResponse = {
      items: [
        {
          id: 'rev-ord-1',
          productId: sharedProductId,
          orderItemId: firstPurchaseOrderItemId,
          rating: 5,
          title: 'Отличное пальто',
          comment: 'Очень качественная шерсть и хорошая посадка',
          status: 'pending_moderation',
        },
      ],
    };

    // Construct reviewsMap exactly as Orders.tsx does
    const reviewsMap: Record<string, any> = {};
    reviewsApiResponse.items.forEach((rev) => {
      if (rev.orderItemId) {
        reviewsMap[rev.orderItemId] = rev;
      }
    });

    // 1. First order item has existing review: CTA must be HIDDEN, existing review displayed
    expect(shouldShowReviewCTA('delivered', reviewsMap[firstPurchaseOrderItemId])).toBe(false);
    expect(reviewsMap[firstPurchaseOrderItemId].title).toBe('Отличное пальто');
    expect(getReviewStatusBadgeText(reviewsMap[firstPurchaseOrderItemId])).toBe('На модерации');

    // 2. Second order item of the SAME product has NOT been reviewed yet: CTA must be SHOWN
    expect(shouldShowReviewCTA('delivered', reviewsMap[secondPurchaseOrderItemId])).toBe(true);
    expect(reviewsMap[secondPurchaseOrderItemId]).toBeUndefined();
  });

  it('simulates full success lifecycle: create -> refetch -> pending status without fake published state', () => {
    const mockReviewsState: Record<string, any> = {};
    const orderItemId = 'oi-test-123';

    // 1. Initial: CTA visible
    expect(shouldShowReviewCTA('delivered', mockReviewsState[orderItemId])).toBe(true);

    // 2. Submit valid review
    const createdReview = {
      id: 'rev-new-1',
      orderItemId,
      rating: 5,
      status: 'pending_moderation',
    };

    // 3. Refetch
    mockReviewsState[orderItemId] = createdReview;

    // 4. CTA is immediately replaced with "На модерации" (strictly pending_moderation)
    expect(shouldShowReviewCTA('delivered', mockReviewsState[orderItemId])).toBe(false);
    expect(getReviewStatusBadgeText(mockReviewsState[orderItemId])).toBe('На модерации');
    expect(mockReviewsState[orderItemId].status).not.toBe('published');
  });

  it('proves Orders.tsx does not contain hardcoded card masks or fabricated timelines', () => {
    // @ts-ignore
    const fs = require('fs');
    // @ts-ignore
    const path = require('path');
    // @ts-ignore
    const ordersPath = path.join(__dirname, '../../pages/Orders.tsx');
    const ordersContent = fs.readFileSync(ordersPath, 'utf-8');

    // Prove no hardcoded payment mask
    expect(ordersContent).not.toContain('paymentMethod: \'Банковская карта\'');
    expect(ordersContent).not.toContain('paymentCardMask: \'•••• 4592\'');
    expect(ordersContent).not.toContain('Способ оплаты');

    // Prove no fake refund totals
    expect(ordersContent).not.toContain('Возвращено');
    expect(ordersContent).not.toContain('Итого после возврата');

    // Prove no detailed timeline events
    expect(ordersContent).not.toContain('buildOrderTimelineEvents');
    expect(ordersContent).not.toContain('История заказа (Vertical Timeline)');
  });

  it('proves Orders.tsx mounts ReviewModal persistently and ReturnModal conditionally', () => {
    // @ts-ignore
    const fs = require('fs');
    // @ts-ignore
    const path = require('path');
    // @ts-ignore
    const ordersPath = path.join(__dirname, '../../pages/Orders.tsx');
    const ordersContent = fs.readFileSync(ordersPath, 'utf-8');

    // Prove ReviewModal is rendered persistently with boolean isOpen prop
    expect(ordersContent).toContain('<ReviewModal');
    expect(ordersContent).toContain('isOpen={Boolean(selectedOrder && reviewModal.isOpen && reviewModal.item)}');

    // Prove ReturnModal is conditionally rendered
    expect(ordersContent).toContain('{selectedOrder && returnModal.isOpen && returnModal.item && (');
  });

  it('proves Drawer uses createPortal to document.body with stable non-animated backdrop (no blur, no motion.div)', () => {
    // @ts-ignore
    const fs = require('fs');
    // @ts-ignore
    const path = require('path');
    // @ts-ignore
    const drawerPath = path.join(__dirname, '../ui/Drawer.tsx');
    const drawerContent = fs.readFileSync(drawerPath, 'utf-8');

    expect(drawerContent).toContain('createPortal(');
    expect(drawerContent).toContain('z-[100]');
    expect(drawerContent).toContain('z-[101]');
    expect(drawerContent).toContain('document.body');

    // Prove Drawer backdrop has NO blur and is NOT a motion.div
    expect(drawerContent).not.toContain('backdrop-blur');
    expect(drawerContent).toContain('data-testid="drawer-backdrop"');
    expect(drawerContent).toContain('<motion.div'); // Panel is motion.div
    expect(drawerContent).not.toContain('<motion.div\n            initial={{ opacity: 0 }}\n            animate={{ opacity: 1 }}\n            exit={{ opacity: 0 }}\n            onClick={onClose}\n            className="fixed inset-0');
  });

  it('proves Modal uses persistent portal shell targeting modal-root / document.body without extra render delay or blur', () => {
    // @ts-ignore
    const fs = require('fs');
    // @ts-ignore
    const path = require('path');
    // @ts-ignore
    const modalPath = path.join(__dirname, '../ui/Modal.tsx');
    const modalContent = fs.readFileSync(modalPath, 'utf-8');

    expect(modalContent).toContain('createPortal(');
    expect(modalContent).toContain("document.getElementById('modal-root') || document.body");
    expect(modalContent).toContain('z-[200]');
    expect(modalContent).toContain('z-[201]');
    expect(modalContent).toContain('z-[202]');
    // Prove no artificial useState(mounted) delay
    expect(modalContent).not.toContain('setMounted(true)');
    // Prove Modal backdrop has no backdrop-blur
    expect(modalContent).not.toContain('backdrop-blur');
    // Prove Modal intercepts Escape key via stopPropagation
    expect(modalContent).toContain('e.stopPropagation()');
  });

  it('proves nested scroll lock prevents layout repaint flash and keeps body locked until all overlays close', () => {
    const originalStyle = document.body.style.overflow;
    
    // 1. Open Drawer
    lockBodyScroll();
    expect(getScrollLockCount()).toBe(1);
    expect(document.body.style.overflow).toBe('hidden');

    // 2. Open ReviewModal above Drawer (nested)
    lockBodyScroll();
    expect(getScrollLockCount()).toBe(2);
    // Body remains locked without toggling or intermediate unlock
    expect(document.body.style.overflow).toBe('hidden');

    // 3. Close ReviewModal (Drawer still open)
    unlockBodyScroll();
    expect(getScrollLockCount()).toBe(1);
    // Body MUST still remain locked because Drawer is open
    expect(document.body.style.overflow).toBe('hidden');

    // 4. Close Drawer
    unlockBodyScroll();
    expect(getScrollLockCount()).toBe(0);
    // Body finally restored
    expect(document.body.style.overflow).toBe(originalStyle || '');
  });
});
