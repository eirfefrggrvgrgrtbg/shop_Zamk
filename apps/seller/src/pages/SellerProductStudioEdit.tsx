import { useCallback, useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  getSellerProduct,
  getSellerCategorySchema,
  getSellerColors,
  getSellerSizeValues,
  getSellerDictionaryValues,
  type SellerCategorySchema,
  type SellerDictionaryValue,
  type SellerColor,
} from '@zamk/api-client/src/seller';
import {
  ProductStudioProvider,
  type ProductStudioDraft,
} from '../contexts/ProductStudioContext';
import { ProductStudio } from '../components/product-studio/ProductStudio';
import {
  hydrateProductStudioDraft,
  resolveSizeSystemForProduct,
} from '../components/product-studio/productStudioHydration';
import { Loader2, AlertCircle, ArrowLeft, RefreshCw } from 'lucide-react';

interface ErrorState {
  type: '404' | '403' | '500' | 'integrity';
  message: string;
}

export default function SellerProductStudioEdit() {
  const { id } = useParams<{ id: string }>();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ErrorState | null>(null);
  const [draft, setDraft] = useState<ProductStudioDraft | null>(null);
  const [categorySchema, setCategorySchema] = useState<SellerCategorySchema | null>(null);
  const [canonicalColors, setCanonicalColors] = useState<SellerColor[]>([]);
  const [dictionaryValuesMap, setDictionaryValuesMap] = useState<Record<string, SellerDictionaryValue[]>>({});
  const [resolvedSizeSystemId, setResolvedSizeSystemId] = useState<string | null>(null);

  const loadProductAndDependencies = useCallback(async () => {
    if (!id) {
      setError({
        type: '404',
        message: 'Товар не найден',
      });
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      // 1. Fetch the seller product
      const product = await getSellerProduct(id);
      if (!product || !product.id) {
        setError({
          type: '404',
          message: 'Товар не найден',
        });
        setLoading(false);
        return;
      }

      // 2. Fetch category schema & canonical colors in parallel
      const [schema, colors] = await Promise.all([
        product.categoryId
          ? getSellerCategorySchema(product.categoryId)
          : Promise.resolve(null),
        getSellerColors(),
      ]);

      if (!schema) {
        throw new Error('Не удалось загрузить схему категории');
      }

      // 3. Fetch canonical dictionary values strictly for all dictionary attributes in schema
      const dictMap: Record<string, SellerDictionaryValue[]> = {};
      const dictIdsToFetch = new Set<string>();
      (schema.attributes || []).forEach((a) => {
        if (a.dictionaryId) {
          dictIdsToFetch.add(a.dictionaryId);
        }
      });

      if (dictIdsToFetch.size > 0) {
        await Promise.all(
          Array.from(dictIdsToFetch).map(async (dictId) => {
            try {
              const vals = await getSellerDictionaryValues(dictId);
              dictMap[dictId] = vals || [];
            } catch (err: any) {
              throw new Error(
                `Data integrity error: failed to fetch canonical dictionary "${dictId}": ${err?.message || 'unknown error'}`
              );
            }
          })
        );
      }

      // 4. Resolve size system (strict: no guessing, no ambiguity)
      const sizeValueIds = Array.from(
        new Set(
          (product.variants || [])
            .map((v) => v.sizeValueId)
            .filter(Boolean) as string[]
        )
      );

      const sizeSystemResolution = await resolveSizeSystemForProduct(
        sizeValueIds,
        schema,
        getSellerSizeValues
      );

      if (sizeSystemResolution.error) {
        setError({
          type: 'integrity',
          message: sizeSystemResolution.error,
        });
        setLoading(false);
        return;
      }

      // 5. Hydrate ProductStudioDraft (strictly validates references)
      const hydratedDraft = hydrateProductStudioDraft({
        product,
        categorySchema: schema,
        canonicalColors: colors,
        dictionaryValuesMap: dictMap,
      });

      setDraft(hydratedDraft);
      setCategorySchema(schema);
      setCanonicalColors(colors);
      setDictionaryValuesMap(dictMap);
      setResolvedSizeSystemId(sizeSystemResolution.systemId);
    } catch (err: any) {
      console.error('Failed to hydrate product studio edit:', err);

      const status = err?.status || err?.statusCode || (err?.response && err.response.status);
      const code = err?.code || '';

      if (status === 404 || code === 'not_found' || code === 'NOT_FOUND') {
        setError({
          type: '404',
          message: 'Товар не найден',
        });
      } else if (status === 403 || code === 'forbidden' || code === 'FORBIDDEN') {
        setError({
          type: '403',
          message: 'Нет доступа к этому товару',
        });
      } else if (err?.message?.includes('Data integrity error')) {
        setError({
          type: 'integrity',
          message: err.message,
        });
      } else {
        setError({
          type: '500',
          message: 'Не удалось загрузить товар',
        });
      }
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadProductAndDependencies();
  }, [loadProductAndDependencies]);

  // Loading State
  if (loading) {
    return (
      <div
        data-testid="studio-edit-loading"
        className="flex min-h-[70vh] flex-col items-center justify-center gap-3"
      >
        <Loader2 className="h-8 w-8 animate-spin text-gray-400 dark:text-gray-500" />
        <p className="text-xs text-gray-500 dark:text-gray-400">Загрузка данных товара...</p>
      </div>
    );
  }

  // Error States
  if (error) {
    if (error.type === '404') {
      return (
        <div
          data-testid="studio-edit-error-404"
          className="flex min-h-[70vh] flex-col items-center justify-center p-6 text-center"
        >
          <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <AlertCircle className="h-7 w-7" />
          </div>
          <h2 className="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
            Товар не найден
          </h2>
          <p className="mb-6 max-w-sm text-sm text-gray-500 dark:text-gray-400">
            Запрашиваемый товар не существует или был удален.
          </p>
          <Link
            to="/products"
            data-testid="studio-edit-back-to-products"
            className="inline-flex items-center gap-2 rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-medium text-white hover:bg-black dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            Вернуться в ассортимент
          </Link>
        </div>
      );
    }

    if (error.type === '403') {
      return (
        <div
          data-testid="studio-edit-error-403"
          className="flex min-h-[70vh] flex-col items-center justify-center p-6 text-center"
        >
          <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-50 text-amber-600 dark:bg-amber-950/20 dark:text-amber-400">
            <AlertCircle className="h-7 w-7" />
          </div>
          <h2 className="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
            Нет доступа к этому товару
          </h2>
          <p className="mb-6 max-w-sm text-sm text-gray-500 dark:text-gray-400">
            У вас нет прав для редактирования этого товара.
          </p>
          <Link
            to="/products"
            data-testid="studio-edit-back-to-products"
            className="inline-flex items-center gap-2 rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-medium text-white hover:bg-black dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            Вернуться в ассортимент
          </Link>
        </div>
      );
    }

    // 500 / Network / Integrity
    return (
      <div
        data-testid="studio-edit-error-500"
        className="flex min-h-[70vh] flex-col items-center justify-center p-6 text-center"
      >
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 text-red-600 dark:bg-red-950/20 dark:text-red-400">
          <AlertCircle className="h-7 w-7" />
        </div>
        <h2 className="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          Не удалось загрузить товар
        </h2>
        <p className="mb-6 max-w-sm text-sm text-gray-500 dark:text-gray-400">
          {error.message || 'Произошла ошибка при загрузке данных с сервера. Попробуйте еще раз.'}
        </p>
        <div className="flex items-center gap-3">
          <button
            type="button"
            data-testid="studio-edit-retry"
            onClick={loadProductAndDependencies}
            className="inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-indigo-700 transition-colors cursor-pointer"
          >
            <RefreshCw className="h-4 w-4" />
            Повторить
          </button>
          <Link
            to="/products"
            data-testid="studio-edit-back-to-products"
            className="inline-flex items-center gap-2 rounded-xl border border-gray-200 dark:border-white/10 px-5 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
            Вернуться в ассортимент
          </Link>
        </div>
      </div>
    );
  }

  if (!draft) {
    return null;
  }

  return (
    <ProductStudioProvider
      entryMode="edit"
      initialDraft={draft}
      initialCategorySchema={categorySchema}
      initialSizeSystemId={resolvedSizeSystemId}
      canonicalColors={canonicalColors}
      dictionaryValuesMap={dictionaryValuesMap}
    >
      <ProductStudio />
    </ProductStudioProvider>
  );
}
