import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  getSellerColors,
  getSellerProduct,
  createSellerProduct,
  type SellerColor,
} from '@zamk/api-client/src/seller';
import { ProductStudioProvider, type ProductStudioDraft } from '../contexts/ProductStudioContext';
import { ProductStudio } from '../components/product-studio/ProductStudio';
import {
  loadProductStudioCreateSession,
  saveProductStudioCreateSession,
  clearProductStudioCreateSession,
  isProductStudioCreateSessionMalformed,
} from '../components/product-studio/productStudioCreateSession';
import { Loader2, AlertCircle, RefreshCw } from 'lucide-react';

export const INITIAL_EMPTY_STUDIO_DRAFT: ProductStudioDraft = {
  title: '',
  description: '',
  categoryId: '',
  images: [],
  variants: [],
};

export function SellerProductStudioNew() {
  const navigate = useNavigate();
  // Check synchronously if a recovery session exists
  const initialSession = typeof window !== 'undefined' ? loadProductStudioCreateSession() : null;
  const initialMalformed = typeof window !== 'undefined' ? isProductStudioCreateSessionMalformed() : false;
  const [isRecovering, setIsRecovering] = useState<boolean>(Boolean(initialSession && initialSession.phase !== 'completed'));
  const [recoveryError, setRecoveryError] = useState<string | null>(
    initialMalformed ? 'Обнаружены некорректные данные сессии создания товара в локальном хранилище браузера.' : null
  );
  const [isMalformedState, setIsMalformedState] = useState<boolean>(initialMalformed);
  const [canonicalColors, setCanonicalColors] = useState<SellerColor[]>([]);

  const checkRecoveryAndLoad = useCallback(async () => {
    // 1. Fetch canonical colors
    getSellerColors().then((colors) => {
      setCanonicalColors(colors || []);
    }).catch(() => {});

    // 2. Check for malformed session storage
    if (isProductStudioCreateSessionMalformed()) {
      setIsMalformedState(true);
      setRecoveryError('Обнаружены некорректные данные сессии создания товара в локальном хранилище браузера.');
      setIsRecovering(false);
      return;
    }
    setIsMalformedState(false);

    // 3. Check for active durable create session
    const session = loadProductStudioCreateSession();
    if (!session) {
      setIsRecovering(false);
      return;
    }

    // Phase: completed
    // Stale completed bookkeeping: NOT recovery. Attempt cleanup; if successful, render fresh Create.
    if (session.phase === 'completed') {
      const cleared = clearProductStudioCreateSession();
      if (!cleared && isProductStudioCreateSessionMalformed()) {
        setRecoveryError('Не удалось очистить завершенную сессию создания товара в браузере.');
      }
      setIsRecovering(false);
      return;
    }

    setIsRecovering(true);
    setRecoveryError(null);

    // Hard Refresh Case A: productId already authoritative (identity_established)
    if (session.productId && session.phase === 'identity_established') {
      try {
        const product = await getSellerProduct(session.productId);
        if (product && product.id) {
          try {
            saveProductStudioCreateSession({
              ...session,
              phase: 'completed',
            });
          } catch (persistErr) {
            console.warn('Failed to persist completed create session marker:', persistErr);
          }
          clearProductStudioCreateSession();
          navigate(`/products/${session.productId}/edit`, { replace: true });
          return;
        }
      } catch (_err: any) {
        setRecoveryError('Не удалось восстановить созданный товар. Попробуйте еще раз.');
        setIsRecovering(false);
        return;
      }
    }

    // Hard Refresh Case B: snapshot exists, response loss recovery (identity_pending)
    if (session.clientCreateId && session.createRequestSnapshot && session.phase === 'identity_pending') {
      try {
        const created = await createSellerProduct(session.createRequestSnapshot, {
          idempotencyKey: session.clientCreateId,
        });
        if (created && created.id) {
          saveProductStudioCreateSession({
            ...session,
            productId: created.id,
            phase: 'identity_established',
          });
          const product = await getSellerProduct(created.id);
          if (product && product.id) {
            try {
              saveProductStudioCreateSession({
                ...session,
                productId: created.id,
                phase: 'completed',
              });
            } catch (persistErr) {
              console.warn('Failed to persist completed create session marker:', persistErr);
            }
            clearProductStudioCreateSession();
            navigate(`/products/${created.id}/edit`, { replace: true });
            return;
          }
        }
      } catch (err: any) {
        setRecoveryError(
          err?.status === 409
            ? 'Конфликт операции создания товара. Обратитесь в поддержку.'
            : 'Не удалось восстановить черновик товара. Проверьте соединение и обновите страницу.'
        );
        setIsRecovering(false);
        return;
      }
    }

    setIsRecovering(false);
  }, [navigate]);

  useEffect(() => {
    checkRecoveryAndLoad();
  }, [checkRecoveryAndLoad]);

  if (isRecovering) {
    return (
      <div
        data-testid="studio-new-loading"
        className="flex min-h-[70vh] flex-col items-center justify-center gap-3"
      >
        <Loader2 className="h-8 w-8 animate-spin text-gray-400 dark:text-gray-500" />
        <p className="text-xs text-gray-500 dark:text-gray-400">Восстановление данных товара...</p>
      </div>
    );
  }

  if (recoveryError) {
    return (
      <div
        data-testid="studio-new-recovery-error"
        className="flex min-h-[70vh] flex-col items-center justify-center p-6 text-center"
      >
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 text-red-600 dark:bg-red-950/20 dark:text-red-400">
          <AlertCircle className="h-7 w-7" />
        </div>
        <h2 className="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {isMalformedState ? 'Ошибка данных сессии' : 'Ошибка восстановления черновика'}
        </h2>
        <p className="mb-6 max-w-sm text-sm text-gray-500 dark:text-gray-400">
          {recoveryError}
        </p>
        <div className="flex items-center gap-3">
          {isMalformedState ? (
            <button
              type="button"
              data-testid="studio-new-clear-malformed-session"
              onClick={() => {
                clearProductStudioCreateSession();
                setRecoveryError(null);
                setIsMalformedState(false);
              }}
              className="inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-indigo-700 transition-colors cursor-pointer"
            >
              <RefreshCw className="h-4 w-4" />
              Очистить и создать новый
            </button>
          ) : (
            <button
              type="button"
              data-testid="studio-new-retry-recovery"
              onClick={checkRecoveryAndLoad}
              className="inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-indigo-700 transition-colors cursor-pointer"
            >
              <RefreshCw className="h-4 w-4" />
              Повторить попытку
            </button>
          )}
          <button
            type="button"
            data-testid="studio-new-back-to-products"
            onClick={() => navigate('/products')}
            className="inline-flex items-center gap-2 rounded-xl border border-gray-200 dark:border-white/10 px-5 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors cursor-pointer"
          >
            К списку товаров
          </button>
        </div>
      </div>
    );
  }

  return (
    <ProductStudioProvider
      entryMode="create"
      initialDraft={INITIAL_EMPTY_STUDIO_DRAFT}
      canonicalColors={canonicalColors}
      onNavigate={navigate}
    >
      <ProductStudio />
    </ProductStudioProvider>
  );
}

export default SellerProductStudioNew;
