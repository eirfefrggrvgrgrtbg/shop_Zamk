import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { validatePromo } from '@zamk/api-client/src/customer';
import { useAuth } from './AuthContext';
import { useCart } from './CartContext';
import {
  PROMO_STORAGE_KEY,
  cartSignature,
  isNetworkError,
  normalizePromoCode,
  promoErrorMessage,
} from '../lib/promo';

interface PromoQuote {
  code: string;
  discountCents: number;
  signature: string;
}

interface PromoContextType {
  /** Code the customer wants to use (sent to CreateOrder). Null when none. */
  appliedCode: string | null;
  /** Discount in roubles, only when the server confirmed it for the CURRENT cart; otherwise 0. */
  discount: number;
  /** A server check for the current cart is in flight (discount is hidden meanwhile). */
  isChecking: boolean;
  /** A code is selected but the server could not confirm it for the current cart yet. */
  isUnconfirmed: boolean;
  error: string | null;
  apply: (rawCode: string) => Promise<boolean>;
  remove: () => void;
  retry: () => void;
  /** Called by checkout when CreateOrder rejected the promo: drops it and shows the reason. */
  rejectAfterCheckout: (error: unknown) => void;
  clear: () => void;
}

const PromoContext = createContext<PromoContextType | undefined>(undefined);

function readStoredCode(): string | null {
  try {
    return sessionStorage.getItem(PROMO_STORAGE_KEY);
  } catch {
    return null;
  }
}

function writeStoredCode(code: string | null) {
  try {
    if (code) sessionStorage.setItem(PROMO_STORAGE_KEY, code);
    else sessionStorage.removeItem(PROMO_STORAGE_KEY);
  } catch {
    /* storage unavailable: the code simply won't survive a reload */
  }
}

export function PromoProvider({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitializing } = useAuth();
  const { items, isLoadingCart } = useCart();
  const [appliedCode, setAppliedCode] = useState<string | null>(() => readStoredCode());
  const [quote, setQuote] = useState<PromoQuote | null>(null);
  const [isChecking, setIsChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const seqRef = useRef(0);

  const signature = useMemo(() => cartSignature(items), [items]);
  const hasItems = items.length > 0;

  const setCode = useCallback((code: string | null) => {
    setAppliedCode(code);
    writeStoredCode(code);
  }, []);

  const clear = useCallback(() => {
    seqRef.current += 1;
    setQuote(null);
    setIsChecking(false);
    setError(null);
    setCode(null);
  }, [setCode]);

  // Re-confirm the selected code with the server whenever the cart contents change (and after reload).
  useEffect(() => {
    if (isInitializing || isLoadingCart) return;
    if (!isAuthenticated || !hasItems) {
      if (appliedCode && !isAuthenticated) {
        // Guest: nothing to validate against; keep nothing.
        seqRef.current += 1;
        setQuote(null);
        setIsChecking(false);
        setCode(null);
      } else if (!hasItems) {
        seqRef.current += 1;
        setQuote(null);
        setIsChecking(false);
      }
      return;
    }
    if (!appliedCode) return;
    if (quote && quote.code === appliedCode && quote.signature === signature) return;

    const seq = ++seqRef.current;
    setIsChecking(true);
    validatePromo(appliedCode)
      .then((res) => {
        if (seq !== seqRef.current) return;
        setQuote({ code: res.code || appliedCode, discountCents: res.discountCents, signature });
        setError(null);
      })
      .catch((e) => {
        if (seq !== seqRef.current) return;
        setQuote(null);
        if (isNetworkError(e)) {
          setError(promoErrorMessage(e));
        } else {
          setError(`${promoErrorMessage(e)} Промокод удалён.`);
          setCode(null);
        }
      })
      .finally(() => {
        if (seq === seqRef.current) setIsChecking(false);
      });
    // `quote` is intentionally excluded: it is the result of this effect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appliedCode, signature, hasItems, isAuthenticated, isInitializing, isLoadingCart, retryTick]);

  const apply = useCallback(
    async (rawCode: string): Promise<boolean> => {
      const code = normalizePromoCode(rawCode);
      if (!code) {
        setError('Введите промокод.');
        return false;
      }
      const seq = ++seqRef.current;
      setIsChecking(true);
      setError(null);
      try {
        const res = await validatePromo(code);
        if (seq !== seqRef.current) return false;
        setQuote({ code: res.code || code, discountCents: res.discountCents, signature });
        setCode(code);
        return true;
      } catch (e) {
        if (seq !== seqRef.current) return false;
        // A failed replacement keeps the previously confirmed code and its quote.
        setError(promoErrorMessage(e));
        // This attempt cancelled any in-flight re-check of the current code; schedule it again.
        setRetryTick((n) => n + 1);
        return false;
      } finally {
        if (seq === seqRef.current) setIsChecking(false);
      }
    },
    [signature, setCode],
  );

  const remove = useCallback(() => clear(), [clear]);
  const retry = useCallback(() => setRetryTick((n) => n + 1), []);

  const rejectAfterCheckout = useCallback(
    (e: unknown) => {
      seqRef.current += 1;
      setQuote(null);
      setIsChecking(false);
      setError(`${promoErrorMessage(e)} Промокод удалён, итог пересчитан без скидки.`);
      setCode(null);
    },
    [setCode],
  );

  const activeQuote = quote && quote.code === appliedCode && quote.signature === signature ? quote : null;

  const value = useMemo<PromoContextType>(
    () => ({
      appliedCode,
      discount: activeQuote ? activeQuote.discountCents / 100 : 0,
      isChecking,
      isUnconfirmed: !!appliedCode && !activeQuote && hasItems,
      error,
      apply,
      remove,
      retry,
      rejectAfterCheckout,
      clear,
    }),
    [appliedCode, activeQuote, isChecking, hasItems, error, apply, remove, retry, rejectAfterCheckout, clear],
  );

  return <PromoContext.Provider value={value}>{children}</PromoContext.Provider>;
}

export function usePromo() {
  const ctx = useContext(PromoContext);
  if (!ctx) throw new Error('usePromo must be used within PromoProvider');
  return ctx;
}
