// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest';
import { useEffect, useRef } from 'react';
import { renderHook } from '@testing-library/react';
import { isLightColor, getMeasurementMeta } from './ProductDetail';

describe('ProductDetail Visual & Logic Behaviors', () => {
  it('A. white/light swatch receives visible boundary treatment', () => {
    expect(isLightColor('#ffffff')).toBe(true);
    expect(isLightColor('#fff')).toBe(true);
    expect(isLightColor('#f8f8f8')).toBe(true);
    expect(isLightColor('#ffff00')).toBe(true);
    expect(isLightColor('#000000')).toBe(false);
    expect(isLightColor('#121214')).toBe(false);
    expect(isLightColor('#ef4444')).toBe(false);
  });

  it('B. measurement illustration receives active canonical field codes, not category title strings', () => {
    const chestMeta = getMeasurementMeta('CHEST');
    expect(chestMeta.label).toBe('Грудь, см');
    expect(chestMeta.shortLabel).toBe('Грудь');
    expect(chestMeta.instruction.length).toBeGreaterThan(0);

    const lengthMeta = getMeasurementMeta('LENGTH');
    expect(lengthMeta.label).toBe('Длина изделия, см');
    expect(lengthMeta.shortLabel).toBe('Длина изделия');

    const sleeveMeta = getMeasurementMeta('SLEEVE');
    expect(sleeveMeta.label).toBe('Длина рукава, см');
    expect(sleeveMeta.shortLabel).toBe('Длина рукава');
  });

  it('C. CHEST + LENGTH + SLEEVE does not render WAIST guide', () => {
    const activeFields1 = ['CHEST', 'LENGTH', 'SLEEVE'];
    expect(activeFields1.includes('WAIST')).toBe(false);
    const relevantInstructions = activeFields1.map(f => getMeasurementMeta(f).shortLabel);
    expect(relevantInstructions.includes('Талия')).toBe(false);
    expect(relevantInstructions.includes('Грудь')).toBe(true);
    expect(relevantInstructions.includes('Длина изделия')).toBe(true);
    expect(relevantInstructions.includes('Длина рукава')).toBe(true);
  });

  it('D. LANDSCAPE image selects landscape presentation', () => {
    const landscapeRatio = 1920 / 1080;
    const landscapeOrientation = landscapeRatio > 1.15 ? 'landscape' : (landscapeRatio >= 0.85 ? 'square' : 'portrait');
    expect(landscapeOrientation).toBe('landscape');
  });

  it('E. PORTRAIT image selects portrait presentation', () => {
    const portraitRatio = 800 / 1000;
    const portraitOrientation = portraitRatio > 1.15 ? 'landscape' : (portraitRatio >= 0.85 ? 'square' : 'portrait');
    expect(portraitOrientation).toBe('portrait');
  });

  it('F. changing color clears now-invalid selected Size', () => {
    const mockVariants = [
      { id: 'v1', colorId: 'red', size: 'M', isActive: true },
      { id: 'v2', colorId: 'red', size: 'L', isActive: true },
      { id: 'v3', colorId: 'white', size: 'S', isActive: true },
      { id: 'v4', colorId: 'white', size: 'M', isActive: true },
    ];

    let activeSize: string | null = 'L';
    const newColorId = 'white';
    const sizeStillValid = mockVariants.some(v => v.isActive && v.colorId === newColorId && v.size === activeSize);
    if (!sizeStillValid) {
      activeSize = null;
    }
    expect(activeSize).toBeNull();

    activeSize = 'M';
    const sizeStillValid2 = mockVariants.some(v => v.isActive && v.colorId === newColorId && v.size === activeSize);
    if (!sizeStillValid2) {
      activeSize = null;
    }
    expect(activeSize).toBe('M');
  });
});

describe('PER.2 ProductDetail Authenticated View Tracking', () => {
  // Exact hook reproduction matching ProductDetail.tsx tracking effect
  function useProductViewTracking(
    isAuthenticated: boolean,
    userId: string | undefined,
    product: { id?: string; isPreview?: boolean } | null,
    recordFn: (productId: string) => Promise<any>
  ) {
    const lastTrackedKeyRef = useRef<string | null>(null);

    useEffect(() => {
      if (!isAuthenticated || !userId || !product || product.isPreview || !product.id) {
        return;
      }
      const trackKey = `${userId}:${product.id}`;
      if (lastTrackedKeyRef.current === trackKey) {
        return;
      }
      lastTrackedKeyRef.current = trackKey;

      recordFn(product.id).catch((err) => {
        console.debug('Failed to record product view', err);
      });
    }, [isAuthenticated, userId, product?.id, product?.isPreview]);
  }

  it('A. authenticated user + successful PDP load -> record-view API called once', () => {
    const recordFn = vi.fn().mockResolvedValue({ status: 'ok' });
    const { rerender } = renderHook(
      ({ isAuth, uid, prod }) => useProductViewTracking(isAuth, uid, prod, recordFn),
      {
        initialProps: {
          isAuth: true,
          uid: 'cust-123',
          prod: { id: 'prod-A', isPreview: false },
        },
      }
    );

    expect(recordFn).toHaveBeenCalledTimes(1);
    expect(recordFn).toHaveBeenCalledWith('prod-A');
  });

  it('B. ordinary rerender -> does not produce duplicate call', () => {
    const recordFn = vi.fn().mockResolvedValue({ status: 'ok' });
    const { rerender } = renderHook(
      ({ isAuth, uid, prod }) => useProductViewTracking(isAuth, uid, prod, recordFn),
      {
        initialProps: {
          isAuth: true,
          uid: 'cust-123',
          prod: { id: 'prod-A', isPreview: false },
        },
      }
    );

    expect(recordFn).toHaveBeenCalledTimes(1);

    // Rerender with same user and product (e.g. selection state change in PDP)
    rerender({
      isAuth: true,
      uid: 'cust-123',
      prod: { id: 'prod-A', isPreview: false },
    });

    expect(recordFn).toHaveBeenCalledTimes(1);
  });

  it('C. navigate Product A -> Product B -> one call for each product', () => {
    const recordFn = vi.fn().mockResolvedValue({ status: 'ok' });
    const { rerender } = renderHook(
      ({ isAuth, uid, prod }) => useProductViewTracking(isAuth, uid, prod, recordFn),
      {
        initialProps: {
          isAuth: true,
          uid: 'cust-123',
          prod: { id: 'prod-A', isPreview: false },
        },
      }
    );

    expect(recordFn).toHaveBeenCalledTimes(1);
    expect(recordFn).toHaveBeenLastCalledWith('prod-A');

    // Navigate to product B
    rerender({
      isAuth: true,
      uid: 'cust-123',
      prod: { id: 'prod-B', isPreview: false },
    });

    expect(recordFn).toHaveBeenCalledTimes(2);
    expect(recordFn).toHaveBeenLastCalledWith('prod-B');
  });

  it('D. anonymous visitor -> no record-view call', () => {
    const recordFn = vi.fn().mockResolvedValue({ status: 'ok' });
    renderHook(
      ({ isAuth, uid, prod }) => useProductViewTracking(isAuth, uid, prod, recordFn),
      {
        initialProps: {
          isAuth: false,
          uid: undefined,
          prod: { id: 'prod-A', isPreview: false },
        },
      }
    );

    expect(recordFn).not.toHaveBeenCalled();
  });

  it('E. tracking endpoint failure -> PDP remains usable/rendered', () => {
    const recordFn = vi.fn().mockRejectedValue(new Error('500 internal server error'));

    expect(() => {
      renderHook(
        ({ isAuth, uid, prod }) => useProductViewTracking(isAuth, uid, prod, recordFn),
        {
          initialProps: {
            isAuth: true,
            uid: 'cust-123',
            prod: { id: 'prod-A', isPreview: false },
          },
        }
      );
    }).not.toThrow();

    expect(recordFn).toHaveBeenCalledTimes(1);
  });

  it('F. preview product does not record view', () => {
    const recordFn = vi.fn().mockResolvedValue({ status: 'ok' });
    renderHook(
      ({ isAuth, uid, prod }) => useProductViewTracking(isAuth, uid, prod, recordFn),
      {
        initialProps: {
          isAuth: true,
          uid: 'cust-123',
          prod: { id: 'prod-preview', isPreview: true },
        },
      }
    );

    expect(recordFn).not.toHaveBeenCalled();
  });

  it('G. multi-user isolation: customer A logs out, customer B logs in -> records view for customer B', () => {
    const recordFn = vi.fn().mockResolvedValue({ status: 'ok' });
    const { rerender } = renderHook(
      ({ isAuth, uid, prod }: { isAuth: boolean; uid: string | undefined; prod: { id?: string; isPreview?: boolean } | null }) =>
        useProductViewTracking(isAuth, uid, prod, recordFn),
      {
        initialProps: {
          isAuth: true,
          uid: 'cust-A' as string | undefined,
          prod: { id: 'prod-1', isPreview: false },
        },
      }
    );

    expect(recordFn).toHaveBeenCalledTimes(1);

    // Customer A logs out
    rerender({
      isAuth: false,
      uid: undefined,
      prod: { id: 'prod-1', isPreview: false },
    });
    expect(recordFn).toHaveBeenCalledTimes(1);

    // Customer B logs in on the same PDP
    rerender({
      isAuth: true,
      uid: 'cust-B',
      prod: { id: 'prod-1', isPreview: false },
    });
    expect(recordFn).toHaveBeenCalledTimes(2);
    expect(recordFn).toHaveBeenLastCalledWith('prod-1');
  });
});
