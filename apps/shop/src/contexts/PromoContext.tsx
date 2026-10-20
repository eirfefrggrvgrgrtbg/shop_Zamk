import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { validatePromo } from '@zamk/api-client/src/customer';
import { useAuth } from './AuthContext';
import { useCart } from './CartContext';
import { PromoContext, type PromoContextType } from './promoContextValue';
import {
  PROMO_STORAGE_KEY,
  cartSignature,
  isNetworkError,
  normalizePromoCode,
  promoErrorMessage,
} from '../lib/promo';

/** Result of the last server check, tied to the exact (code, cart, retry) it was computed for. */
type CheckResult = { key: string; status: 'ok'; discountCents: number } | { key: string; status: 'failed' };

/** The stored code belongs to one buyer; another buyer (or a logged-out session) never sees it. */
function readStoredCode(userId: string | null): string | null {
  if (!userId) return null;
  try {
    const raw = sessionStorage.getItem(PROMO_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as { userId?: string; code?: string };
    return parsed.userId === userId && parsed.code ? parsed.code : null;
  } catch {
    return null;
  }
}

function writeStoredCode(userId: string | null, code: string | null) {
  try {
    if (userId && code) sessionStorage.setItem(PROMO_STORAGE_KEY, JSON.stringify({ userId, code }));
    else sessionStorage.removeItem(PROMO_STORAGE_KEY);
  } catch {
    /* storage unavailable: the code simply won't survive a reload */
  }
}

function removeStoredCode() {
  writeStoredCode(null, null);
}

export function PromoProvider({ children }: { children: React.ReactNode }) {
  const { user, isAuthenticated, isInitializing } = useAuth();
  const { items, isLoadingCart } = useCart();
  const userId = isAuthenticated ? (user?.id ?? null) : null;

  // In-memory choice of the current buyer; falls back to what sessionStorage holds for that buyer.
  const [override, setOverride] = useState<{ owner: string; code: string | null } | null>(null);
  const appliedCode = userId
    ? override && override.owner === userId
      ? override.code
      : readStoredCode(userId)
    : null;

  const [check, setCheck] = useState<CheckResult | null>(null);
  const [isApplying, setIsApplying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [retryTick, setRetryTick] = useState(0);
  const applySeqRef = useRef(0);

  const signature = useMemo(() => cartSignature(items), [items]);
  const hasItems = items.length > 0;
  const keyFor = useCallback(
    (code: string) => `${userId}|${code}|${signature}|${retryTick}`,
    [userId, signature, retryTick],
  );

  const setCode = useCallback(
    (code: string | null) => {
      if (!userId) return;
      setOverride({ owner: userId, code });
      writeStoredCode(userId, code);
    },
    [userId],
  );

  // Logout / another buyer: drop the stored code. Side effect only; no state is set here.
  useEffect(() => {
    if (isInitializing) return;
    if (!userId) {
      removeStoredCode();
      return;
    }
    try {
      const raw = sessionStorage.getItem(PROMO_STORAGE_KEY);
      if (raw && (JSON.parse(raw) as { userId?: string }).userId !== userId) removeStoredCode();
    } catch {
      removeStoredCode();
    }
  }, [userId, isInitializing]);

  const currentKey = appliedCode ? keyFor(appliedCode) : null;
  const confirmed = check && check.status === 'ok' && check.key === currentKey ? check : null;
  const eligible = !!appliedCode && hasItems && !isInitializing && !isLoadingCart;
  const alreadyChecked = !!check && check.key === currentKey;

  // Confirm the selected code with the server for the current cart (after reload and cart changes).
  useEffect(() => {
    if (!eligible || alreadyChecked || !appliedCode || !currentKey) return;
    let cancelled = false;
    validatePromo(appliedCode)
      .then((res) => {
        if (cancelled) return;
        setCheck({ key: currentKey, status: 'ok', discountCents: res.discountCents });
        setError(null);
      })
      .catch((e) => {
        if (cancelled) return;
        setCheck({ key: currentKey, status: 'failed' });
        if (isNetworkError(e)) {
          setError(promoErrorMessage(e));
        } else {
          setError(`${promoErrorMessage(e)} Промокод удалён.`);
          setCode(null);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [eligible, alreadyChecked, appliedCode, currentKey, setCode]);

  const apply = useCallback(
    async (rawCode: string): Promise<boolean> => {
      const code = normalizePromoCode(rawCode);
      if (!code) {
        setError('Введите промокод.');
        return false;
      }
      const seq = ++applySeqRef.current;
      setIsApplying(true);
      setError(null);
      try {
        const res = await validatePromo(code);
        if (seq !== applySeqRef.current) return false;
        setCheck({ key: keyFor(code), status: 'ok', discountCents: res.discountCents });
        setCode(code);
        return true;
      } catch (e) {
        // A failed replacement keeps the previously confirmed code and its quote.
        if (seq === applySeqRef.current) setError(promoErrorMessage(e));
        return false;
      } finally {
        if (seq === applySeqRef.current) setIsApplying(false);
      }
    },
    [keyFor, setCode],
  );

  const clear = useCallback(() => {
    applySeqRef.current += 1;
    setIsApplying(false);
    setError(null);
    setCheck(null);
    setCode(null);
  }, [setCode]);

  const retry = useCallback(() => setRetryTick((n) => n + 1), []);

  const rejectAfterCheckout = useCallback(
    (e: unknown) => {
      applySeqRef.current += 1;
      setIsApplying(false);
      setCheck(null);
      setError(`${promoErrorMessage(e)} Промокод удалён, итог пересчитан без скидки.`);
      setCode(null);
    },
    [setCode],
  );

  const isChecking = isApplying || (eligible && !alreadyChecked);

  const value = useMemo<PromoContextType>(
    () => ({
      appliedCode,
      discount: confirmed ? confirmed.discountCents / 100 : 0,
      isChecking,
      isUnconfirmed: !!appliedCode && hasItems && !confirmed,
      error,
      apply,
      remove: clear,
      retry,
      rejectAfterCheckout,
      clear,
    }),
    [appliedCode, confirmed, isChecking, hasItems, error, apply, clear, retry, rejectAfterCheckout],
  );

  return <PromoContext.Provider value={value}>{children}</PromoContext.Provider>;
}
