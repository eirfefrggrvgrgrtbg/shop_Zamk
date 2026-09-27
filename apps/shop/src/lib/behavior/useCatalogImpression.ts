import { useEffect, useRef } from 'react';
import { trackCatalogImpression } from './behaviorClient';

export interface UseCatalogImpressionOptions {
  productId: string;
  placement?: string;
  disabled?: boolean;
}

export function useCatalogImpression<T extends HTMLElement = HTMLDivElement>({
  productId,
  placement,
  disabled = false,
}: UseCatalogImpressionOptions) {
  const elementRef = useRef<T | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (disabled || !productId || !placement) {
      return;
    }

    const node = elementRef.current;
    if (!node || typeof IntersectionObserver === 'undefined') {
      return;
    }

    const clearTimer = () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
            if (timerRef.current === null) {
              timerRef.current = setTimeout(() => {
                trackCatalogImpression(productId, placement);
                timerRef.current = null;
              }, 1000);
            }
          } else {
            clearTimer();
          }
        }
      },
      {
        threshold: [0.5],
      }
    );

    observer.observe(node);

    return () => {
      clearTimer();
      observer.disconnect();
    };
  }, [productId, placement, disabled]);

  return elementRef;
}
