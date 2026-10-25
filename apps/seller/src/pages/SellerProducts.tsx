import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useVisibilityPolling } from '../hooks/useVisibilityPolling';
import {
  AlertTriangle,
  BarChart3,
  ExternalLink,
  Eye,
  Edit2,
  PackagePlus,
  Search,
  ShoppingBag,
  Sparkles,
  Send,
} from 'lucide-react';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import {
  SellerSurface,
  SellerKpiCard,
  SellerTableShell,
  SellerDrawer,
} from '../components/SellerSurface';
import {
  availabilityOwnerLabels,
  getProductAvailability,
  getStorefrontStatusInfo,
  getSkuState,
  statusLabels,
  type SellerProduct,
  type SellerProductStatus,
} from '../lib/seller-products';
import {
  getSellerProductsPaginated,
  getSellerMe,
  getSellerCategories,
  submitSellerProductModeration,
  deleteSellerProduct,
  archiveSellerProduct,
} from '@zamk/api-client/src/seller';
import type { SellerProductsSummary } from '@zamk/api-client/src/types';
import { adaptProductList } from '../api/adapter';
import { cn } from '../lib/utils';
import { prepareAddProductNavigation } from '../components/product-studio/productStudioCreateSession';
import { ProductActionMenu } from '../components/ProductActionMenu';
import { ProductLifecycleModal } from '../components/ProductLifecycleModal';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const numberFormatter = new Intl.NumberFormat('ru-RU');
const formatCurrency = (value: number) => currencyFormatter.format(value);
const formatNumber = (value: number) => numberFormatter.format(value);

type StatusFilter = SellerProductStatus | 'all' | 'moderation';
type AvailabilityFilter = 'all' | 'available' | 'unavailable';

const PAGE_SIZE = 20;

const availabilityFilterOptions: Array<{ value: AvailabilityFilter; label: string }> = [
  { value: 'all', label: 'Все товары' },
  { value: 'available', label: 'Можно купить' },
  { value: 'unavailable', label: 'Нельзя купить' },
];

const statusFilterOptions: Array<{ value: StatusFilter; label: string }> = [
  { value: 'all', label: 'Все статусы' },
  { value: 'moderation', label: 'Ожидают проверки ZAMK' },
  { value: 'published', label: statusLabels.published },
  { value: 'draft', label: statusLabels.draft },
  { value: 'pending_moderation', label: statusLabels.pending_moderation },
  { value: 'in_review', label: statusLabels.in_review },
  { value: 'approved', label: statusLabels.approved },
  { value: 'rejected', label: statusLabels.rejected },
  { value: 'hidden', label: statusLabels.hidden },
  { value: 'blocked', label: statusLabels.blocked },
  { value: 'out_of_stock', label: statusLabels.out_of_stock },
  { value: 'archived', label: statusLabels.archived },
];

function ProductBadge({ children, tone = 'neutral' }: { children: React.ReactNode; tone?: 'neutral' | 'good' | 'warning' | 'danger' | 'info' }) {
  const styles = {
    neutral: 'bg-gray-100 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/80 dark:border-white/10',
    good: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-400 dark:border-emerald-900/40',
    warning: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40',
    danger: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900/40',
    info: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40',
  };

  return <span className={cn('inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium', styles[tone])}>{children}</span>;
}

function getStatusTone(status: SellerProductStatus) {
  const tones: Record<SellerProductStatus, 'neutral' | 'good' | 'warning' | 'danger' | 'info'> = {
    draft: 'neutral',
    pending_moderation: 'info',
    in_review: 'info',
    approved: 'good',
    published: 'good',
    rejected: 'danger',
    hidden: 'warning',
    blocked: 'danger',
    out_of_stock: 'warning',
    archived: 'neutral',
  };

  return tones[status] || 'neutral';
}

function ProductAvatar({ product }: { product: SellerProduct }) {
  if (product.mainPhoto && product.mainPhoto.startsWith('http')) {
    return (
      <img src={product.mainPhoto} alt={product.title} className="h-14 w-14 shrink-0 rounded-2xl object-cover shadow-sm" />
    );
  }
  
  const initials = product.title
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || 'ПР';

  return (
    <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-graphite to-accent text-sm font-bold text-white shadow-sm dark:from-white dark:to-accent dark:text-black">
      {initials}
    </div>
  );
}

function ProductDetailPanel({
  product,
  sellerStatus,
  onProductUpdated,
  onRequestDelete,
  onRequestArchive,
  onOpenEdit,
  lifecycleError,
  isNotDisposableError,
}: {
  product: SellerProduct;
  sellerStatus: string;
  onProductUpdated?: () => Promise<void> | void;
  onRequestDelete?: (product: SellerProduct) => void;
  onRequestArchive?: (product: SellerProduct) => void;
  onOpenEdit?: (product: SellerProduct) => void;
  lifecycleError?: string | null;
  isNotDisposableError?: boolean;
}) {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isDrawerMenuOpen, setIsDrawerMenuOpen] = useState(false);

  const totalStock = product.sizes.reduce((sum, item) => sum + (item.active === false ? 0 : item.stock || 0), 0);
  const availability = getProductAvailability(product);
  const isApprovedAndNoStock = product.status === 'approved' && totalStock === 0;

  const handleSubmitModeration = async () => {
    setIsSubmitting(true);
    setSubmitError(null);
    try {
      await submitSellerProductModeration(product.id);
      await onProductUpdated?.();
    } catch (err: any) {
      setSubmitError(err?.message || 'Не удалось отправить товар на модерацию');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="flex flex-col justify-between h-full space-y-6">
      <div>
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-4 min-w-0 flex-1">
            <ProductAvatar product={product} />
            <div className="min-w-0 flex-1">
              <p className="text-xs font-mono uppercase tracking-wider text-gray-500 dark:text-gray-400">{product.sku}</p>
              <h2 className="mt-1 text-xl font-semibold tracking-tight text-gray-900 dark:text-white line-clamp-2">{product.title}</h2>
              <div className="mt-2.5 flex flex-wrap gap-2">
                <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
                {isApprovedAndNoStock && (
                  <ProductBadge tone="warning">Требуется поставка</ProductBadge>
                )}
              </div>
              {product.status === 'rejected' && product.rejectionReason && (
                <div className="mt-4 rounded-lg border border-red-200 bg-red-50 p-3.5 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200">
                  <span className="mb-1 block font-medium">Причина отклонения:</span>
                  {product.rejectionReason}
                </div>
              )}
            </div>
          </div>

          <ProductActionMenu
            product={product}
            sellerStatus={sellerStatus}
            variant="drawer"
            isOpen={isDrawerMenuOpen}
            onToggle={() => setIsDrawerMenuOpen(!isDrawerMenuOpen)}
            onClose={() => setIsDrawerMenuOpen(false)}
            onOpenEdit={onOpenEdit}
            onRequestDelete={onRequestDelete}
            onRequestArchive={onRequestArchive}
          />
        </div>

        {lifecycleError && (
          <div
            data-testid="lifecycle-error-message"
            className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3.5 text-sm text-amber-800 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-200 flex flex-col gap-2"
          >
            <span>{lifecycleError}</span>
            {isNotDisposableError && product.status === 'draft' && onRequestArchive && (
              <button
                type="button"
                data-testid="lifecycle-error-archive-cta"
                onClick={() => onRequestArchive(product)}
                className="self-start text-xs font-semibold text-amber-900 dark:text-amber-100 underline hover:no-underline cursor-pointer"
              >
                Отправить в архив
              </button>
            )}
          </div>
        )}

        <div className="mt-6 grid gap-3 sm:grid-cols-2">
          <div className="rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Цена</p>
            <p className="mt-1.5 text-lg font-bold text-gray-900 dark:text-white">{formatCurrency(product.price)}</p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Склад ZAMK</p>
            <p className="mt-1.5 text-sm font-medium text-gray-900 dark:text-white">
              {totalStock > 0 ? `${totalStock} шт. на складе` : 'Ожидается поставка'}
            </p>
          </div>
          <div className="rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Продажа</p>
            {(() => {
              const sf = getStorefrontStatusInfo(product);
              return (
                <p className={cn('mt-1.5 text-sm', sf.toneClass)}>
                  {sf.label}
                </p>
              );
            })()}
          </div>
          <div className="rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Покупка</p>
            <p
              className={cn('mt-1.5 text-sm font-medium', availability.state === 'available' ? 'text-emerald-600 dark:text-emerald-400' : availability.state === 'unavailable' ? 'text-amber-600 dark:text-amber-400' : 'text-gray-500')}
              data-testid="drawer-availability"
            >
              {availability.label}
            </p>
          </div>
        </div>

        {availability.state === 'unavailable' && availability.reasons.length > 0 && (
          <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3.5 text-sm text-amber-900 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-200" data-testid="drawer-unavailable-reasons">
            <p className="mb-1.5 font-medium">Почему нельзя купить</p>
            <ul className="space-y-1">
              {availability.reasons.map((reason) => (
                <li key={reason.code} className="flex flex-wrap items-baseline gap-x-2">
                  <span>{reason.text}</span>
                  <span className="text-[11px] uppercase tracking-wide opacity-70">{availabilityOwnerLabels[reason.owner]}</span>
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
          <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Варианты (SKU)</p>
          <div className="mt-2.5 flex flex-col gap-1.5">
            {product.sizes.map((item, idx) => (
              <div key={`${item.size}-${idx}`} className="flex justify-between items-center rounded-md border border-gray-200/70 bg-white dark:bg-white/[0.03] px-3 py-2 text-sm text-gray-800 dark:border-white/10 dark:text-white/80">
                <span>{item.size}</span>
                <span className="flex items-center gap-2 text-gray-500 font-medium">
                  {item.active !== undefined && (
                    <ProductBadge tone={getSkuState(item).tone}>{getSkuState(item).label}</ProductBadge>
                  )}
                  ZAMK: {item.stock} шт.
                </span>
              </div>
            ))}
          </div>
        </div>

        {isApprovedAndNoStock && (
          <div className="mt-4 rounded-lg border border-blue-200 bg-blue-50 p-3.5 text-sm text-blue-800 dark:border-blue-900/30 dark:bg-blue-950/20 dark:text-blue-200">
            Товар одобрен. Для старта продаж необходимо оформить поставку на склад ZAMK.
          </div>
        )}

        {product.description && product.description.trim() ? (
          <div className="mt-4 rounded-lg border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
            <p className="text-xs font-medium text-gray-500 dark:text-gray-400">Описание</p>
            <p className="mt-1.5 text-sm leading-relaxed text-gray-600 dark:text-gray-400 line-clamp-3 whitespace-pre-line">
              {product.description}
            </p>
          </div>
        ) : null}
      </div>
      
      <div className="pt-4 border-t border-gray-100 dark:border-white/10 space-y-3">
        {submitError && (
          <div
            data-testid="submit-error-message"
            className="rounded-lg border border-red-200 bg-red-50 p-3.5 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200"
          >
            {submitError}
          </div>
        )}

        {sellerStatus === 'blocked' || sellerStatus === 'archived' ? (
          <div className="rounded-lg border border-red-200 bg-red-50 p-3.5 text-sm text-red-800 dark:border-red-900/30 dark:bg-red-900/20 dark:text-red-200">
            Действия недоступны из-за статуса магазина.
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {product.status === 'draft' ? (
              <>
                <Link
                  to={`/products/${product.id}/edit`}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
                >
                  <Edit2 className="h-4 w-4" />
                  Продолжить заполнение
                </Link>
                <button
                  type="button"
                  onClick={handleSubmitModeration}
                  disabled={isSubmitting}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-900 shadow-sm transition-colors hover:bg-gray-50 hover:border-gray-300 disabled:opacity-50 dark:border-white/10 dark:bg-white/5 dark:text-white dark:hover:bg-white/10"
                >
                  <Send className="h-4 w-4 text-gray-500" />
                  {isSubmitting ? 'Отправка...' : 'Отправить на модерацию'}
                </button>
              </>
            ) : product.status === 'rejected' ? (
              <Link
                to={`/products/${product.id}/edit`}
                className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-red-600 px-4 text-sm font-medium text-white shadow-sm transition-colors hover:bg-red-700"
              >
                <AlertTriangle className="h-4 w-4" />
                Исправить карточку
              </Link>
            ) : product.status === 'published' ? (
              product.storefrontUrl && product.storefrontUrl.trim() ? (
                <>
                  <a
                    href={product.storefrontUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
                  >
                    <ExternalLink className="h-4 w-4" />
                    Открыть в магазине
                  </a>
                  <Link
                    to={`/products/${product.id}/edit`}
                    className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-900 shadow-sm transition-colors hover:bg-gray-50 hover:border-gray-300 dark:border-white/10 dark:bg-white/5 dark:text-white dark:hover:bg-white/10"
                  >
                    <Edit2 className="h-4 w-4 text-gray-500" />
                    Редактировать карточку
                  </Link>
                </>
              ) : (
                <Link
                  to={`/products/${product.id}/edit`}
                  className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
                >
                  <Edit2 className="h-4 w-4" />
                  Редактировать карточку
                </Link>
              )
            ) : product.status === 'archived' ? (
              <div className="text-center py-2 text-sm text-gray-500 dark:text-gray-400">
                Товар находится в архиве.
              </div>
            ) : (
              <Link
                to={`/products/${product.id}/edit`}
                className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-4 text-sm font-medium text-gray-900 shadow-sm transition-colors hover:bg-gray-50 hover:border-gray-300 dark:border-white/10 dark:bg-white/5 dark:text-white dark:hover:bg-white/10"
              >
                <Eye className="h-4 w-4 text-gray-500" />
                Открыть карточку
              </Link>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export function SellerProducts() {
  const navigate = useNavigate();
  const [products, setProducts] = useState<SellerProduct[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [summary, setSummary] = useState<SellerProductsSummary | null>(null);
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [availability, setAvailability] = useState<AvailabilityFilter>('all');
  const [offset, setOffset] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [hasLoaded, setHasLoaded] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [sellerStatus, setSellerStatus] = useState<string>('active');
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [modalType, setModalType] = useState<'delete' | 'archive' | null>(null);
  const [targetProduct, setTargetProduct] = useState<SellerProduct | null>(null);
  const [isSubmittingLifecycle, setIsSubmittingLifecycle] = useState(false);
  const [lifecycleError, setLifecycleError] = useState<string | null>(null);
  const [isNotDisposableError, setIsNotDisposableError] = useState(false);

  // Filters change faster than the server answers: only the newest request may write to the page.
  const latestRequest = useRef(0);

  const loadData = useCallback(async (silent = false) => {
    const requestId = ++latestRequest.current;
    const isStale = () => requestId !== latestRequest.current;
    try {
      if (!silent) {
        setIsLoading(true);
        setError('');
      }
      const [me, rawProducts, rawCategories] = await Promise.all([
        getSellerMe(),
        getSellerProductsPaginated({
          q: debouncedQuery || undefined,
          status: status === 'all' ? undefined : status,
          availability: availability === 'all' ? undefined : availability,
          limit: PAGE_SIZE,
          offset,
        }),
        typeof getSellerCategories === 'function' ? getSellerCategories().catch(() => []) : Promise.resolve([])
      ]);
      if (isStale()) return;
      setSellerStatus(me.seller.status);
      const categoryMap: Record<string, string> = {};
      if (Array.isArray(rawCategories)) {
        for (const cat of rawCategories) {
          if (cat?.id && cat?.name) {
            categoryMap[cat.id] = cat.name;
          }
        }
      }
      const adapted = adaptProductList(rawProducts.items, categoryMap);
      setProducts(adapted);
      setTotalCount(rawProducts.totalCount ?? adapted.length);
      setSummary(rawProducts.summary ?? null);
      setHasLoaded(true);
      setSelectedId((prev) => {
        if (prev && adapted.some((p) => p.id === prev)) return prev;
        return null;
      });
    } catch (err: any) {
      if (!silent && !isStale()) {
        setError(err.message || 'Ошибка загрузки товаров');
      }
    } finally {
      if (!isStale()) {
        setIsLoading(false);
      }
    }
  }, [debouncedQuery, status, availability, offset]);

  useEffect(() => {
    loadData(false);
  }, [loadData]);

  // Search is server-side: wait for a pause in typing, and go back to the first page for a new query.
  useEffect(() => {
    if (query.trim() === debouncedQuery) return;
    const timer = setTimeout(() => {
      setDebouncedQuery(query.trim());
      setOffset(0);
    }, 300);
    return () => clearTimeout(timer);
  }, [query, debouncedQuery]);

  const changeStatus = (next: StatusFilter) => {
    setSelectedId(null);
    setOffset(0);
    setStatus(next);
  };
  const changeAvailability = (next: AvailabilityFilter) => {
    setSelectedId(null);
    setOffset(0);
    setAvailability(next);
  };

  useVisibilityPolling(useCallback(() => loadData(true), [loadData]), 4000);

  const filteredProducts = products; // filtered and paginated by the server

  const selectedProduct = selectedId ? (products.find((product) => product.id === selectedId) || null) : null;

  const handleOpenEdit = useCallback((p: SellerProduct) => {
    navigate(`/products/${p.id}/edit`);
  }, [navigate]);

  const handleRequestDelete = useCallback((product: SellerProduct) => {
    setLifecycleError(null);
    setIsNotDisposableError(false);
    setTargetProduct(product);
    setModalType('delete');
  }, []);

  const handleRequestArchive = useCallback((product: SellerProduct) => {
    setLifecycleError(null);
    setIsNotDisposableError(false);
    setTargetProduct(product);
    setModalType('archive');
  }, []);

  const handleConfirmDelete = async () => {
    if (!targetProduct || isSubmittingLifecycle) return;
    setIsSubmittingLifecycle(true);
    const deletedId = targetProduct.id;
    try {
      await deleteSellerProduct(deletedId);
      setProducts((prev) => prev.filter((p) => p.id !== deletedId));
      setSelectedId((prev) => (prev === deletedId ? null : prev));
      setModalType(null);
      setTargetProduct(null);
      setLifecycleError(null);
      setIsNotDisposableError(false);
      loadData(true);
    } catch (err: any) {
      setTargetProduct(null);
      const code = err?.code || '';
      const msg = err?.message || '';
      if (
        code === 'product_not_disposable' ||
        code === 'PRODUCT_NOT_DISPOSABLE' ||
        msg.includes('disposable') ||
        (err?.status === 409 && msg.includes('history'))
      ) {
        setIsNotDisposableError(true);
        setLifecycleError(
          'Этот черновик нельзя удалить: с товаром уже связаны складские или другие операции. Его можно отправить в архив.'
        );
        setModalType(null);
      } else if (
        code === 'invalid_status' ||
        code === 'INVALID_STATUS' ||
        (err?.status === 409 && msg.includes('draft'))
      ) {
        setLifecycleError('Статус товара изменился. Обновите список и попробуйте снова.');
        setModalType(null);
        loadData(true);
      } else if (err?.status === 404 || code === 'not_found') {
        setLifecycleError('Товар не найден.');
        setModalType(null);
        setSelectedId((prev) => (prev === deletedId ? null : prev));
        loadData(true);
      } else {
        setLifecycleError(msg || 'Не удалось удалить товар');
        setModalType(null);
      }
    } finally {
      setIsSubmittingLifecycle(false);
    }
  };

  const handleConfirmArchive = async () => {
    if (!targetProduct || isSubmittingLifecycle) return;
    setIsSubmittingLifecycle(true);
    const archivedId = targetProduct.id;
    try {
      await archiveSellerProduct(archivedId);
      setProducts((prev) =>
        prev.map((p) =>
          p.id === archivedId
            ? {
                ...p,
                status: 'archived',
                actualVisibility: false,
                storefrontUrl: undefined,
              }
            : p
        )
      );
      setModalType(null);
      setTargetProduct(null);
      setLifecycleError(null);
      setIsNotDisposableError(false);
      loadData(true);
    } catch (err: any) {
      const code = err?.code || '';
      const msg = err?.message || '';
      if (
        code === 'invalid_transition' ||
        code === 'INVALID_TRANSITION' ||
        err?.status === 409
      ) {
        setLifecycleError('Статус товара изменился, поэтому действие больше недоступно.');
        setModalType(null);
        loadData(true);
      } else if (code === 'seller_blocked' || code === 'SELLER_BLOCKED' || err?.status === 403) {
        setLifecycleError(msg || 'Магазин заблокирован или архивирован. Действие недоступно.');
        setModalType(null);
      } else if (err?.status === 404 || code === 'not_found') {
        setLifecycleError('Товар не найден.');
        setModalType(null);
        setSelectedId((prev) => (prev === archivedId ? null : prev));
        loadData(true);
      } else {
        setLifecycleError(msg || 'Не удалось переместить товар в архив');
        setModalType(null);
      }
    } finally {
      setIsSubmittingLifecycle(false);
    }
  };

  if (isLoading && !hasLoaded) {
    return (
      <SellerPageFrame variant="wide">
        <div className="flex justify-center py-20">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div>
        </div>
      </SellerPageFrame>
    );
  }

  if (error && !hasLoaded) {
    return (
      <SellerPageFrame variant="wide">
        <div role="alert" className="flex flex-col items-center gap-3 py-20 text-red-500">
          <span>{error}</span>
          <button type="button" onClick={() => loadData(false)} className="rounded-lg border border-red-300 px-3 py-1.5 text-sm text-red-700 hover:bg-red-50">
            Повторить
          </button>
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="wide">
      <SellerPageHeader
        eyebrow="Ассортимент"
        title="Мои товары"
        description="Управляйте карточками товаров. После модерации необходимо оформить поставку на склад ZAMK для старта продаж."
        action={
          <Link
            to="/products/new"
            onClick={() => prepareAddProductNavigation()}
            className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-black px-4 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            <PackagePlus className="h-4 w-4" />
            Добавить товар
          </Link>
        }
      />

      {lifecycleError && !selectedProduct && (
        <div
          data-testid="page-lifecycle-error-banner"
          className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3.5 text-sm text-amber-800 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-200 flex items-center justify-between"
        >
          <span>{lifecycleError}</span>
          <button
            type="button"
            onClick={() => setLifecycleError(null)}
            className="text-xs text-amber-700 hover:text-amber-900 font-medium cursor-pointer"
          >
            Закрыть
          </button>
        </div>
      )}

        <section className="mt-6 grid grid-cols-2 lg:grid-cols-4 gap-4">
          <SellerKpiCard
            label="Всего карточек"
            value={formatNumber(summary?.total ?? totalCount)}
            supportText="создано в системе"
            icon={ShoppingBag}
          />
          <SellerKpiCard
            label="Опубликовано"
            value={formatNumber(summary?.published ?? 0)}
            supportText="статус «Опубликован»"
            icon={Sparkles}
          />
          <SellerKpiCard
            label="Можно купить"
            value={formatNumber(summary?.purchasable ?? 0)}
            supportText="доступны покупателям сейчас"
            accent={(summary?.purchasable ?? 0) > 0 ? "positive" : undefined}
            icon={BarChart3}
          />
          <SellerKpiCard
            label="На проверке"
            value={formatNumber(summary?.awaitingReview ?? 0)}
            supportText="ожидают решения ZAMK"
            accent={(summary?.awaitingReview ?? 0) > 0 ? "warning" : undefined}
            icon={AlertTriangle}
          />
        </section>

        <SellerSurface className="mt-6 p-4">
          <div className="grid gap-3 sm:grid-cols-[1fr_240px]">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Поиск по названию, SKU или штрихкоду..."
                className="h-10 w-full rounded-lg border border-gray-200 bg-white pl-10 pr-3 text-sm text-gray-900 outline-none placeholder:text-gray-400 focus:border-gray-900 focus:ring-1 focus:ring-gray-900 dark:border-white/10 dark:bg-white/5 dark:text-white"
              />
            </label>
            <select
              value={status}
              aria-label="Статус товара"
              onChange={(event) => changeStatus(event.target.value as StatusFilter)}
              className="h-10 rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 outline-none focus:border-gray-900 focus:ring-1 focus:ring-gray-900 dark:border-white/10 dark:bg-white/5 dark:text-white"
            >
              {statusFilterOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
          <div className="mt-3 flex flex-wrap gap-2" role="group" aria-label="Доступность для покупателей">
            {availabilityFilterOptions.map((option) => (
              <button
                key={option.value}
                type="button"
                aria-pressed={availability === option.value}
                data-testid={`products-availability-${option.value}`}
                onClick={() => changeAvailability(option.value)}
                className={cn(
                  'px-3.5 py-1.5 text-sm rounded-full border transition-colors',
                  availability === option.value
                    ? 'bg-gray-900 text-white border-gray-900 dark:bg-white dark:text-gray-900 dark:border-white'
                    : 'bg-white text-gray-700 border-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-300 dark:border-white/10',
                )}
              >
                {option.label}
              </button>
            ))}
          </div>
        </SellerSurface>

        {error && hasLoaded && (
          <div role="alert" className="mt-4 flex flex-wrap items-center gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            <span className="min-w-0 flex-1">{error}</span>
            <button type="button" onClick={() => loadData(false)} className="rounded-lg border border-red-300 bg-white px-3 py-1.5 text-red-700 hover:bg-red-50">
              Повторить
            </button>
          </div>
        )}

          {products.length === 0 && !error && (status !== 'all' || availability !== 'all' || debouncedQuery !== '') ? (
            <SellerSurface className="mt-6 p-10 text-center text-sm text-gray-500" data-testid="products-empty-filtered">
              По выбранным условиям товаров нет.
            </SellerSurface>
          ) : products.length === 0 && !error ? (
            <SellerSurface className="mt-6 flex flex-col items-center justify-center p-12 text-center">
              <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-gray-50 border border-gray-200 text-gray-400 dark:bg-white/5 dark:border-white/10 dark:text-white/60">
                <PackagePlus className="h-6 w-6" />
              </div>
              <h2 className="mb-2 text-xl font-semibold text-gray-900 dark:text-white">У вас пока нет товаров</h2>
              <p className="mb-6 max-w-md text-sm text-gray-500 dark:text-gray-400">
                Добавьте первый товар, чтобы отправить его на модерацию.
              </p>
              <Link
                to="/products/new"
                onClick={() => prepareAddProductNavigation()}
                className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-800"
              >
                <PackagePlus className="h-4 w-4" />
                Добавить товар
              </Link>
            </SellerSurface>
          ) : products.length === 0 ? null : (
            <>
              <SellerTableShell className={cn('mt-6 transition-opacity', isLoading && 'opacity-60')}>
                <div className="relative overflow-x-auto">
                  <table className="w-full text-left text-sm whitespace-nowrap">
                    <thead>
                      <tr className="bg-gray-50/60 dark:bg-white/[0.02] border-b border-gray-200 dark:border-white/10 text-xs font-semibold uppercase tracking-wider text-gray-500">
                        <th className="px-4 py-3.5 w-16">Фото</th>
                        <th className="px-4 py-3.5 min-w-[200px]">Товар</th>
                        <th className="px-4 py-3.5">Варианты</th>
                        <th className="px-4 py-3.5">Цена</th>
                        <th className="px-4 py-3.5">Статус</th>
                        <th className="px-4 py-3.5">Склад ZAMK</th>
                        <th className="px-4 py-3.5">Покупка</th>
                        <th className="px-4 py-3.5 text-right w-12"><span className="sr-only">Действия</span></th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                      {filteredProducts.map((product) => {
                        const isSelected = product.id === selectedId;

                        return (
                          <tr
                            key={product.id}
                            className={cn('hover:bg-gray-50/60 dark:hover:bg-white/[0.02] cursor-pointer transition-colors', isSelected && 'bg-gray-50 dark:bg-white/5')}
                            onClick={() => setSelectedId(product.id)}
                            onDoubleClick={() => navigate(`/products/${product.id}/edit`)}
                          >
                            <td className="px-4 py-3.5">
                              <ProductAvatar product={product} />
                            </td>
                            <td className="px-4 py-3.5">
                              <span className="block font-medium text-gray-900 dark:text-white max-w-[280px] truncate">{product.title}</span>
                              <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">{product.category}</span>
                            </td>
                            <td className="px-4 py-3.5 text-gray-700 dark:text-gray-300">
                              {product.sizes.length} SKU
                              {(() => {
                                const av = getProductAvailability(product);
                                return av.partial ? (
                                  <span className="block text-xs text-amber-600 dark:text-amber-400" data-testid="partial-sku">
                                    Доступны {av.skuSellable} из {av.skuTotal}
                                  </span>
                                ) : null;
                              })()}
                            </td>
                            <td className="px-4 py-3.5 font-medium text-gray-900 dark:text-white">{formatCurrency(product.price)}</td>
                            <td className="px-4 py-3.5">
                              <ProductBadge tone={getStatusTone(product.status)}>{statusLabels[product.status]}</ProductBadge>
                            </td>
                            <td className="px-4 py-3.5 text-gray-500 dark:text-gray-400">
                              {(() => {
                                const total = product.sizes?.reduce((sum, s) => sum + (s.active === false ? 0 : s.stock || 0), 0) || 0;
                                const needsSupply = (product.status === 'approved' || product.status === 'published') && total === 0;

                                return total > 0 ? (
                                  <span className="font-semibold text-emerald-600 dark:text-emerald-400">{total} шт.</span>
                                ) : (
                                  <div className="flex flex-col gap-0.5 items-start">
                                    <span>Нет на складе</span>
                                    {needsSupply && (
                                      <span className="text-xs font-medium text-amber-600 dark:text-amber-400">Требуется поставка</span>
                                    )}
                                  </div>
                                );
                              })()}
                            </td>
                            <td className="px-4 py-3.5" data-testid="product-availability">
                              {(() => {
                                const av = getProductAvailability(product);
                                if (av.state === 'available') {
                                  return <ProductBadge tone="good">{av.label}</ProductBadge>;
                                }
                                if (av.state === 'unknown') {
                                  return <span className="text-gray-400 dark:text-gray-500">{av.label}</span>;
                                }
                                const main = av.reasons[0];
                                return (
                                  <div className="flex max-w-[240px] flex-col items-start gap-1 whitespace-normal">
                                    <ProductBadge tone="warning">{av.label}</ProductBadge>
                                    {main && (
                                      <span className="text-xs text-gray-500 dark:text-gray-400" data-testid="availability-reason">
                                        {main.text}
                                        {av.reasons.length > 1 ? ` · ещё ${av.reasons.length - 1}` : ''}
                                      </span>
                                    )}
                                    {main && (
                                      <span className="text-[11px] uppercase tracking-wide text-gray-400 dark:text-gray-500">{availabilityOwnerLabels[main.owner]}</span>
                                    )}
                                  </div>
                                );
                              })()}
                            </td>
                            <td className="px-4 py-3.5 text-right" onClick={(e) => e.stopPropagation()}>
                              <ProductActionMenu
                                product={product}
                                sellerStatus={sellerStatus}
                                variant="row"
                                isOpen={openMenuId === product.id}
                                onToggle={() => setOpenMenuId(openMenuId === product.id ? null : product.id)}
                                onClose={() => setOpenMenuId(null)}
                                onOpenEdit={handleOpenEdit}
                                onOpenDrawer={(p) => setSelectedId(p.id)}
                                onRequestDelete={handleRequestDelete}
                                onRequestArchive={handleRequestArchive}
                              />
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </SellerTableShell>

              {totalCount > PAGE_SIZE && (
                <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500" data-testid="products-pagination">
                  <span>
                    {offset + 1}–{Math.min(offset + PAGE_SIZE, totalCount)} из {totalCount}
                  </span>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      disabled={offset === 0 || isLoading}
                      onClick={() => { setSelectedId(null); setOffset(Math.max(0, offset - PAGE_SIZE)); }}
                      className="rounded-lg border border-gray-200 px-3 py-1.5 disabled:opacity-40 dark:border-white/10"
                    >
                      Назад
                    </button>
                    <button
                      type="button"
                      disabled={offset + PAGE_SIZE >= totalCount || isLoading}
                      onClick={() => { setSelectedId(null); setOffset(offset + PAGE_SIZE); }}
                      className="rounded-lg border border-gray-200 px-3 py-1.5 disabled:opacity-40 dark:border-white/10"
                    >
                      Вперёд
                    </button>
                  </div>
                </div>
              )}

              <SellerDrawer
                isOpen={Boolean(selectedProduct)}
                onClose={() => setSelectedId(null)}
                title="Карточка товара"
                data-testid="seller-product-drawer"
              >
                {selectedProduct && (
                  <ProductDetailPanel
                    product={selectedProduct}
                    sellerStatus={sellerStatus}
                    onProductUpdated={() => loadData(true)}
                    onRequestDelete={handleRequestDelete}
                    onRequestArchive={handleRequestArchive}
                    onOpenEdit={handleOpenEdit}
                    lifecycleError={lifecycleError}
                    isNotDisposableError={isNotDisposableError}
                  />
                )}
              </SellerDrawer>
            </>
          )}

      <ProductLifecycleModal
        isOpen={Boolean(modalType && targetProduct)}
        onClose={() => {
          if (!isSubmittingLifecycle) {
            setModalType(null);
            setTargetProduct(null);
          }
        }}
        onConfirm={modalType === 'delete' ? handleConfirmDelete : handleConfirmArchive}
        title={
          modalType === 'delete'
            ? 'Удалить черновик?'
            : targetProduct?.status === 'published'
            ? 'Снять товар с продажи?'
            : 'Переместить товар в архив?'
        }
        body={
          modalType === 'delete'
            ? 'Карточка будет удалена без возможности восстановления.'
            : targetProduct?.status === 'published'
            ? 'Товар исчезнет из магазина. Остатки и история операций сохранятся.'
            : 'Товар останется в истории, но больше не будет использоваться для продажи.'
        }
        confirmLabel={
          modalType === 'delete'
            ? 'Удалить'
            : targetProduct?.status === 'published'
            ? 'Снять с продажи'
            : 'В архив'
        }
        isDestructive={modalType === 'delete'}
        isLoading={isSubmittingLifecycle}
        confirmTestId={modalType === 'delete' ? 'confirm-delete-btn' : 'confirm-archive-btn'}
        cancelTestId="cancel-lifecycle-btn"
      />
    </SellerPageFrame>
  );
}
