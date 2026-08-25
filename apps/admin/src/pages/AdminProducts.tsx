import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useVisibilityPolling } from '../hooks/useVisibilityPolling';
import { SellerContextBanner } from '../components/SellerContextBanner';
import { FilterPopover } from '../components/FilterPopover';
import {
  Package,
  Search,
  AlertCircle,
  X,
  ChevronRight,
  EyeOff,
  Lock,
  Eye,
  Settings,
  RotateCcw,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  Boxes,
} from 'lucide-react';
import {
  getAdminProducts,
  hideProduct,
  publishProduct,
  blockProduct,
  getAdminProductErrorMessage,
} from '../api/adminProducts';
import {
  getAdminSellers,
  getAdminCategories,
  getAdminBrands,
} from '../api/adminOperations';
import type { AdminProductView } from '../api/adminProducts';
import { formatMoneyRubles } from '../utils/money';
import { computeStockInfo } from '../utils/stock';

// Status styling & labels
const STATUS_CONFIG: Record<string, { label: string; badge: string }> = {
  published: { label: 'Опубликован', badge: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300 border-emerald-200' },
  approved: { label: 'Одобрен', badge: 'bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300 border-blue-200' },
  pending_moderation: { label: 'Ожидает модерации', badge: 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border-amber-200' },
  in_review: { label: 'На проверке', badge: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-950/60 dark:text-indigo-300 border-indigo-200' },
  rejected: { label: 'Требуются исправления', badge: 'bg-rose-100 text-rose-800 dark:bg-rose-950/60 dark:text-rose-300 border-rose-200' },
  blocked: { label: 'Заблокирован', badge: 'bg-red-200 text-red-900 dark:bg-red-950/80 dark:text-red-300 border-red-300' },
  hidden: { label: 'Скрыт', badge: 'bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-300 border-slate-200' },
  draft: { label: 'Черновик', badge: 'bg-gray-200 text-gray-700 dark:bg-gray-800 dark:text-gray-400 border-gray-300' },
  out_of_stock: { label: 'Нет в наличии', badge: 'bg-orange-100 text-orange-800 dark:bg-orange-950/60 dark:text-orange-300 border-orange-200' },
  archived: { label: 'В архиве', badge: 'bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-400 border-zinc-300' },
};

const formatDate = (dateStr?: string | null) => {
  if (!dateStr) return '—';
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return '—';
  return date.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
};

// Test fixture removal filter helper
const isTestFixture = (name?: string) => {
  if (!name) return false;
  const n = name.trim();
  return (
    n === 'Category A' ||
    n === 'Brand A' ||
    n === 'Brand B' ||
    n === 'Smoke Cat' ||
    n === 'Smoke Brand' ||
    n === 'Dev Category'
  );
};

// Available Table Columns
interface ColumnConfig {
  id: string;
  label: string;
  mandatory?: boolean;
}

const ALL_COLUMNS: ColumnConfig[] = [
  { id: 'product', label: 'Товар', mandatory: true },
  { id: 'seller', label: 'Продавец' },
  { id: 'category_brand', label: 'Категория / Бренд' },
  { id: 'status', label: 'Статус', mandatory: true },
  { id: 'price', label: 'Цена' },
  { id: 'variants', label: 'Варианты' },
  { id: 'stock', label: 'Остаток' },
  { id: 'sales', label: 'Продажи' },
  { id: 'rating', label: 'Рейтинг' },
  { id: 'updated_at', label: 'Обновлён' },
  { id: 'actions', label: 'Действие', mandatory: true },
];

export function AdminProducts() {
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();

  const queryQ = searchParams.get('q') || '';
  const queryStatus = searchParams.get('status') || '';
  const queryStatusesStr = searchParams.get('statuses') || '';
  const querySellerId = searchParams.get('sellerId') || '';
  const queryCategoryId = searchParams.get('categoryId') || '';
  const queryCategoryIdsStr = searchParams.get('categoryIds') || '';
  const queryBrandId = searchParams.get('brandId') || '';
  const queryBrandIdsStr = searchParams.get('brandIds') || '';
  const queryStockStatus = searchParams.get('stockStatus') || '';
  const queryPublicationStatus = searchParams.get('publicationStatus') || '';
  const querySubmittedPeriod = searchParams.get('submittedPeriod') || '';
  const queryNoMainImage = searchParams.get('noMainImage') === 'true';
  const queryNoDescription = searchParams.get('noDescription') === 'true';
  const queryNoBrand = searchParams.get('noBrand') === 'true';
  const queryNoVariants = searchParams.get('noVariants') === 'true';
  const queryNoPrice = searchParams.get('noPrice') === 'true';
  const queryDuplicateSku = searchParams.get('duplicateSku') === 'true';
  const queryNoStock = searchParams.get('noStock') === 'true';
  const queryHasProblems = searchParams.get('hasProblems') === 'true';
  const querySortBy = searchParams.get('sortBy') || 'updated_at';
  const querySortOrder = searchParams.get('sortOrder') || 'desc';
  const queryPage = parseInt(searchParams.get('page') || '1', 10);
  const queryLimit = parseInt(searchParams.get('limit') || '25', 10);

  const queryStatuses = useMemo(() => (queryStatusesStr ? queryStatusesStr.split(',').filter(Boolean) : []), [queryStatusesStr]);
  const queryCategoryIds = useMemo(() => (queryCategoryIdsStr ? queryCategoryIdsStr.split(',').filter(Boolean) : []), [queryCategoryIdsStr]);
  const queryBrandIds = useMemo(() => (queryBrandIdsStr ? queryBrandIdsStr.split(',').filter(Boolean) : []), [queryBrandIdsStr]);

  const [searchInput, setSearchInput] = useState(queryQ);
  const [products, setProducts] = useState<AdminProductView[]>([]);
  const [totalCount, setTotalCount] = useState<number>(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [sellersList, setSellersList] = useState<Array<{ id: string; name: string; ownerEmail?: string }>>([]);
  const [categoriesList, setCategoriesList] = useState<Array<{ id: string; name: string }>>([]);
  const [brandsList, setBrandsList] = useState<Array<{ id: string; name: string }>>([]);
  const [sellerSearch, setSellerSearch] = useState('');
  const [categorySearch, setCategorySearch] = useState('');
  const [brandSearch, setBrandSearch] = useState('');

  const [activePopover, setActivePopover] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [bulkActionModal, setBulkActionModal] = useState<{ action: string; label: string } | null>(null);
  const [bulkReason, setBulkReason] = useState('');
  const [isBulkSubmitting, setIsBulkSubmitting] = useState(false);

  const [visibleColumns, setVisibleColumns] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('adminProducts_visibleColumns');
      if (saved) return JSON.parse(saved);
    } catch (_) {}
    return ALL_COLUMNS.map((c) => c.id);
  });

  const saveVisibleColumns = (cols: string[]) => {
    setVisibleColumns(cols);
    try {
      localStorage.setItem('adminProducts_visibleColumns', JSON.stringify(cols));
    } catch (_) {}
  };

  const abortControllerRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef<number>(0);

  useEffect(() => {
    getAdminSellers()
      .then((res: any) => {
        const items = res.items || [];
        setSellersList(
          items
            .filter((s: any) => s.id && !/^[0-9a-f]{8}-[0-9a-f]{4}-/i.test(s.brandName || s.legalName || s.name || ''))
            .map((s: any) => {
              const name = s.brandName || s.legalName || s.name || 'Магазин не назван';
              const email = s.ownerEmail || s.sellerOwnerEmail || s.email || '';
              return {
                id: s.id,
                name: email ? `${name} (${email})` : name,
                ownerEmail: email,
              };
            })
        );
      })
      .catch(() => {});

    getAdminCategories()
      .then((res: any) => {
        const items = Array.isArray(res) ? res : res.items || [];
        setCategoriesList(
          items
            .filter((c: any) => !isTestFixture(c.name))
            .map((c: any) => ({ id: c.id, name: c.name }))
        );
      })
      .catch(() => {});

    getAdminBrands()
      .then((res: any) => {
        const items = Array.isArray(res) ? res : res.items || [];
        setBrandsList(
          items
            .filter((b: any) => !isTestFixture(b.name))
            .map((b: any) => ({ id: b.id, name: b.name }))
        );
      })
      .catch(() => {});
  }, []);

  const updateParams = (paramsMap: Record<string, string | null>, resetPage = true) => {
    const next = new URLSearchParams(searchParams);
    Object.entries(paramsMap).forEach(([key, value]) => {
      if (value === null || value === '') {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });
    if (resetPage && !paramsMap.page) {
      next.set('page', '1');
    }
    setSearchParams(next, { replace: true });
  };

  const updateParam = (key: string, value: string | null, resetPage = true) => {
    updateParams({ [key]: value }, resetPage);
  };

  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchInput !== queryQ) {
        updateParam('q', searchInput || null);
      }
    }, 350);
    return () => clearTimeout(timer);
  }, [searchInput, queryQ]);

  const fetchProducts = useCallback(async (silent = false) => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    const controller = new AbortController();
    abortControllerRef.current = controller;
    const signal = controller.signal;
    const requestId = ++requestIdRef.current;

    if (!silent) {
      setIsLoading(true);
      setError(null);
    }

    try {
      const offset = (queryPage - 1) * queryLimit;
      const res = await getAdminProducts({
        q: queryQ || undefined,
        status: queryStatusesStr || queryStatus || undefined,
        sellerId: querySellerId || undefined,
        categoryId: queryCategoryId || undefined,
        categoryIds: queryCategoryIdsStr || undefined,
        brandId: queryBrandId || undefined,
        brandIds: queryBrandIdsStr || undefined,
        submittedPeriod: querySubmittedPeriod || undefined,
        noMainImage: queryNoMainImage || undefined,
        noDescription: queryNoDescription || undefined,
        noBrand: queryNoBrand || undefined,
        noVariants: queryNoVariants || undefined,
        noPrice: queryNoPrice || undefined,
        duplicateSku: queryDuplicateSku || undefined,
        noStock: queryNoStock || undefined,
        hasProblems: queryHasProblems || undefined,
        sortBy: querySortBy,
        sortOrder: querySortOrder,
        limit: queryLimit,
        offset,
        signal,
      });

      if (signal.aborted || requestId !== requestIdRef.current) return;

      setProducts(res.items ?? []);
      setTotalCount(res.totalCount ?? 0);
      setError(null);
    } catch (err: any) {
      if (err?.name === 'AbortError' || signal.aborted || requestId !== requestIdRef.current) return;
      if (!silent) {
        setProducts([]);
        setTotalCount(0);
        setError(getAdminProductErrorMessage(err, 'Не удалось загрузить товары'));
      }
    } finally {
      if (!silent && requestId === requestIdRef.current && !signal.aborted) {
        setIsLoading(false);
      }
    }
  }, [
    queryQ, queryStatus, queryStatusesStr, querySellerId, queryCategoryId, queryCategoryIdsStr,
    queryBrandId, queryBrandIdsStr, querySubmittedPeriod, queryNoMainImage, queryNoDescription,
    queryNoBrand, queryNoVariants, queryNoPrice, queryDuplicateSku, queryNoStock, queryHasProblems,
    querySortBy, querySortOrder, queryPage, queryLimit,
  ]);

  useEffect(() => {
    fetchProducts(false);
  }, [fetchProducts]);

  useVisibilityPolling(
    useCallback(() => {
      if (!bulkActionModal) {
        fetchProducts(true);
      }
    }, [bulkActionModal, fetchProducts]),
    4000
  );

  const handleSort = (columnKey: string) => {
    if (querySortBy === columnKey) {
      updateParam('sortOrder', querySortOrder === 'asc' ? 'desc' : 'asc', false);
    } else {
      updateParam('sortBy', columnKey, false);
      updateParam('sortOrder', 'asc', false);
    }
  };

  const getSortIcon = (columnKey: string) => {
    if (querySortBy !== columnKey) return <ArrowUpDown className="w-3.5 h-3.5 opacity-40 group-hover:opacity-70" />;
    return querySortOrder === 'asc' ? <ArrowUp className="w-3.5 h-3.5 text-indigo-600 dark:text-indigo-400" /> : <ArrowDown className="w-3.5 h-3.5 text-indigo-600 dark:text-indigo-400" />;
  };

  const toggleSelectAll = () => {
    setSelectedIds(selectedIds.length === products.length ? [] : products.map((p) => p.id));
  };

  const toggleSelectOne = (id: string) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id]));
  };

  const handleBulkSubmit = async () => {
    if (!bulkActionModal || selectedIds.length === 0) return;
    try {
      setIsBulkSubmitting(true);
      setError(null);
      let successCount = 0;
      let skippedCount = 0;

      for (const id of selectedIds) {
        const targetProd = products.find((p) => p.id === id);
        if (bulkActionModal.action === 'restore_storefront') {
          if (targetProd && targetProd.status === 'approved') {
            await publishProduct(id, bulkReason || 'Возврат на витрину администратором');
            successCount++;
          } else {
            skippedCount++;
          }
        } else if (bulkActionModal.action === 'hide') {
          await hideProduct(id, bulkReason || 'Массовое скрытие администратором');
          successCount++;
        } else if (bulkActionModal.action === 'block') {
          await blockProduct(id, bulkReason || 'Массовая блокировка администратором');
          successCount++;
        }
      }

      setSelectedIds([]);
      setBulkActionModal(null);
      setBulkReason('');
      if (skippedCount > 0) setError(`Успешно обработано: ${successCount}. Пропущено товаров не со статусом "Одобрен": ${skippedCount}.`);
      fetchProducts();
    } catch (err: any) {
      setError(getAdminProductErrorMessage(err, 'Ошибка выполнения массового действия.'));
    } finally {
      setIsBulkSubmitting(false);
    }
  };

  const handleResetAllFilters = () => {
    setSearchInput('');
    setSearchParams(new URLSearchParams({ page: '1', limit: String(queryLimit) }), { replace: true });
  };

  const hasActiveFilters =
    Boolean(queryQ) || Boolean(queryStatus) || queryStatuses.length > 0 ||
    Boolean(querySellerId) || Boolean(queryCategoryId) || queryCategoryIds.length > 0 ||
    Boolean(queryBrandId) || queryBrandIds.length > 0 || Boolean(queryStockStatus) ||
    Boolean(queryPublicationStatus) || Boolean(querySubmittedPeriod) || queryHasProblems;

  const totalPages = Math.ceil(totalCount / queryLimit) || 1;

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900 dark:text-white flex items-center gap-2">
            <span>Управление товарами</span>
            <span className="text-sm font-normal text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 rounded-full border border-slate-200 dark:border-slate-700">
              {totalCount}
            </span>
          </h1>
          <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
            Полный операционный центр управления товарами маркетплейса.
          </p>
        </div>

        {/* Search Bar */}
        <div className="relative w-full sm:w-80">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          <input
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Поиск по названию, SKU, ID, продавцу..."
            className="w-full pl-9 pr-8 py-2 text-xs bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500 shadow-sm"
          />
          {searchInput && (
            <button
              onClick={() => setSearchInput('')}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Seller Context Banner */}
      <SellerContextBanner />

      {/* Filter Row & Actions Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-50 dark:bg-slate-900/60 p-3 rounded-2xl border border-slate-200 dark:border-slate-800">
        <div className="flex flex-wrap items-center gap-2">
          {/* Status Filter Popover */}
          <div className="relative">
            <button
              data-testid="filter-status-btn"
              onClick={() => setActivePopover(activePopover === 'status' ? null : 'status')}
              className={`px-3 py-1.5 text-xs font-medium rounded-xl border transition-all flex items-center gap-1.5 ${
                queryStatuses.length > 0 || queryStatus
                  ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
                  : 'bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-800 hover:bg-slate-100'
              }`}
            >
              <span>
                Статус
                {queryStatuses.length > 0 ? `: ${queryStatuses.length} выбрано` : queryStatus ? `: ${STATUS_CONFIG[queryStatus]?.label || queryStatus}` : ''}
              </span>
            </button>
            <FilterPopover
              isOpen={activePopover === 'status'}
              onClose={() => setActivePopover(null)}
              onReset={() => {
                updateParams({
                  statuses: null,
                  status: null,
                });
                setActivePopover(null);
              }}
              onApply={() => setActivePopover(null)}
              widthClass="w-64"
            >
              <div className="space-y-2">
                <span className="font-semibold text-slate-700 dark:text-slate-300 text-xs block border-b pb-1">Статусы товара</span>
                {Object.entries(STATUS_CONFIG).map(([stKey, stCfg]) => {
                  const isChecked = queryStatuses.includes(stKey) || queryStatus === stKey;
                  return (
                    <label key={stKey} data-testid={`filter-status-label-${stKey}`} className="flex items-center gap-2 text-xs text-slate-700 dark:text-slate-300 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800 p-1 rounded-lg">
                      <input
                        data-testid={`filter-status-${stKey}`}
                        type="checkbox"
                        checked={isChecked}
                        onChange={() => {
                          let next = [...queryStatuses];
                          if (next.includes(stKey)) {
                            next = next.filter((k) => k !== stKey);
                          } else {
                            next.push(stKey);
                          }
                          updateParams({
                            statuses: next.length > 0 ? next.join(',') : null,
                            status: null,
                          });
                        }}
                        className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                      />
                      <span>{stCfg.label}</span>
                    </label>
                  );
                })}
              </div>
            </FilterPopover>
          </div>

          {/* Seller Filter Popover */}
          <div className="relative">
            <button
              onClick={() => setActivePopover(activePopover === 'seller' ? null : 'seller')}
              className={`px-3 py-1.5 text-xs font-medium rounded-xl border transition-all flex items-center gap-1.5 ${
                querySellerId
                  ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
                  : 'bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-800 hover:bg-slate-100'
              }`}
            >
              <span>
                Продавец
                {querySellerId ? `: ${sellersList.find((s) => s.id === querySellerId)?.name || 'Выбран'}` : ''}
              </span>
            </button>
            <FilterPopover
              isOpen={activePopover === 'seller'}
              onClose={() => setActivePopover(null)}
              onReset={() => {
                updateParam('sellerId', null);
                setActivePopover(null);
              }}
              onApply={() => setActivePopover(null)}
              widthClass="w-64"
            >
              <div className="space-y-2">
                <input
                  type="text"
                  value={sellerSearch}
                  onChange={(e) => setSellerSearch(e.target.value)}
                  placeholder="Поиск продавца..."
                  className="w-full px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none"
                />
                <div className="max-h-48 overflow-y-auto space-y-1">
                  {sellersList
                    .filter((s) => s.name.toLowerCase().includes(sellerSearch.toLowerCase()))
                    .map((s) => (
                      <button
                        key={s.id}
                        onClick={() => {
                          updateParam('sellerId', querySellerId === s.id ? null : s.id);
                          setActivePopover(null);
                        }}
                        className={`w-full text-left px-2 py-1.5 rounded-lg text-xs transition-colors ${
                          querySellerId === s.id
                            ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 font-medium'
                            : 'hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300'
                        }`}
                      >
                        {s.name}
                      </button>
                    ))}
                </div>
              </div>
            </FilterPopover>
          </div>

          {/* Category Filter Popover */}
          <div className="relative">
            <button
              onClick={() => setActivePopover(activePopover === 'category' ? null : 'category')}
              className={`px-3 py-1.5 text-xs font-medium rounded-xl border transition-all flex items-center gap-1.5 ${
                queryCategoryIds.length > 0 || queryCategoryId
                  ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
                  : 'bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-800 hover:bg-slate-100'
              }`}
            >
              <span>
                Категория
                {queryCategoryIds.length > 0 ? `: ${queryCategoryIds.length} выбрано` : queryCategoryId ? ': Выбрана' : ''}
              </span>
            </button>
            <FilterPopover
              isOpen={activePopover === 'category'}
              onClose={() => setActivePopover(null)}
              onReset={() => {
                updateParam('categoryIds', null);
                updateParam('categoryId', null);
                setActivePopover(null);
              }}
              onApply={() => setActivePopover(null)}
              widthClass="w-64"
            >
              <div className="space-y-2">
                <input
                  type="text"
                  value={categorySearch}
                  onChange={(e) => setCategorySearch(e.target.value)}
                  placeholder="Поиск категории..."
                  className="w-full px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none"
                />
                <div className="max-h-48 overflow-y-auto space-y-1">
                  {categoriesList
                    .filter((c) => c.name.toLowerCase().includes(categorySearch.toLowerCase()))
                    .map((c) => {
                      const isChecked = queryCategoryIds.includes(c.id) || queryCategoryId === c.id;
                      return (
                        <label key={c.id} className="flex items-center gap-2 text-xs text-slate-700 dark:text-slate-300 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800 p-1.5 rounded-lg">
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => {
                              let next = [...queryCategoryIds];
                              if (next.includes(c.id)) {
                                next = next.filter((id) => id !== c.id);
                              } else {
                                next.push(c.id);
                              }
                              updateParam('categoryIds', next.length > 0 ? next.join(',') : null);
                              updateParam('categoryId', null);
                            }}
                            className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                          />
                          <span>{c.name}</span>
                        </label>
                      );
                    })}
                </div>
              </div>
            </FilterPopover>
          </div>

          {/* Brand Filter Popover */}
          <div className="relative">
            <button
              onClick={() => setActivePopover(activePopover === 'brand' ? null : 'brand')}
              className={`px-3 py-1.5 text-xs font-medium rounded-xl border transition-all flex items-center gap-1.5 ${
                queryBrandIds.length > 0 || queryBrandId || queryNoBrand
                  ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 dark:text-indigo-300 border-indigo-300 dark:border-indigo-700'
                  : 'bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-800 hover:bg-slate-100'
              }`}
            >
              <span>
                Бренд
                {queryNoBrand
                  ? ': Без бренда'
                  : queryBrandIds.length > 0
                  ? `: ${queryBrandIds.length} выбрано`
                  : queryBrandId
                  ? ': Выбран'
                  : ''}
              </span>
            </button>
            <FilterPopover
              isOpen={activePopover === 'brand'}
              onClose={() => setActivePopover(null)}
              onReset={() => {
                updateParam('brandIds', null);
                updateParam('brandId', null);
                updateParam('noBrand', null);
                setActivePopover(null);
              }}
              onApply={() => setActivePopover(null)}
              widthClass="w-64"
            >
              <div className="space-y-2">
                <input
                  type="text"
                  value={brandSearch}
                  onChange={(e) => setBrandSearch(e.target.value)}
                  placeholder="Поиск бренда..."
                  className="w-full px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none"
                />
                <label className="flex items-center gap-2 text-xs text-amber-700 dark:text-amber-400 font-medium cursor-pointer p-1.5 rounded-lg bg-amber-50 dark:bg-amber-950/40">
                  <input
                    type="checkbox"
                    checked={queryNoBrand}
                    onChange={(e) => updateParam('noBrand', e.target.checked ? 'true' : null)}
                    className="rounded border-amber-300 text-amber-600 focus:ring-amber-500"
                  />
                  <span>Без бренда</span>
                </label>
                <div className="max-h-48 overflow-y-auto space-y-1">
                  {brandsList
                    .filter((b) => b.name.toLowerCase().includes(brandSearch.toLowerCase()))
                    .map((b) => {
                      const isChecked = queryBrandIds.includes(b.id) || queryBrandId === b.id;
                      return (
                        <label key={b.id} className="flex items-center gap-2 text-xs text-slate-700 dark:text-slate-300 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800 p-1.5 rounded-lg">
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => {
                              let next = [...queryBrandIds];
                              if (next.includes(b.id)) {
                                next = next.filter((id) => id !== b.id);
                              } else {
                                next.push(b.id);
                              }
                              updateParam('brandIds', next.length > 0 ? next.join(',') : null);
                              updateParam('brandId', null);
                            }}
                            className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                          />
                          <span>{b.name}</span>
                        </label>
                      );
                    })}
                </div>
              </div>
            </FilterPopover>
          </div>

          {/* Stock Filter */}
          <div className="relative">
            <button
              onClick={() => setActivePopover(activePopover === 'stock' ? null : 'stock')}
              className={`px-3 py-1.5 text-xs font-medium rounded-xl border transition-all flex items-center gap-1.5 ${
                queryStockStatus
                  ? 'bg-indigo-50 dark:bg-indigo-950/60 text-indigo-700 dark:text-indigo-300 border-indigo-300'
                  : 'bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 border-slate-200'
              }`}
            >
              <Boxes className="w-3.5 h-3.5 text-indigo-500" />
              <span>Остаток</span>
            </button>
            <FilterPopover
              isOpen={activePopover === 'stock'}
              onClose={() => setActivePopover(null)}
              onReset={() => updateParam('stockStatus', null)}
              onApply={() => setActivePopover(null)}
              widthClass="w-56"
            >
              <div className="space-y-1.5 text-xs">
                <span className="font-semibold text-slate-700 dark:text-slate-300 block border-b pb-1">Наличие на складе</span>
                {[
                  { id: 'in_stock', label: 'В наличии' },
                  { id: 'out_of_stock', label: 'Нет в наличии' },
                  { id: 'no_inventory', label: 'Нет складской записи' },
                ].map((st) => (
                  <button
                    key={st.id}
                    onClick={() => {
                      updateParam('stockStatus', queryStockStatus === st.id ? null : st.id);
                      setActivePopover(null);
                    }}
                    className={`w-full text-left px-2 py-1.5 rounded-lg transition-colors ${
                      queryStockStatus === st.id ? 'bg-indigo-50 text-indigo-700 font-semibold' : 'hover:bg-slate-100 text-slate-700'
                    }`}
                  >
                    {st.label}
                  </button>
                ))}
              </div>
            </FilterPopover>
          </div>

          {/* Problems Quick Filter */}
          <button
            onClick={() => updateParam('hasProblems', searchParams.get('hasProblems') === 'true' ? null : 'true')}
            className={`px-3 py-1.5 text-xs font-medium rounded-xl border transition-all flex items-center gap-1.5 ${
              searchParams.get('hasProblems') === 'true'
                ? 'bg-rose-50 dark:bg-rose-950/60 text-rose-700 border-rose-300'
                : 'bg-white text-slate-700 border-slate-200'
            }`}
          >
            <AlertCircle className="w-3.5 h-3.5 text-rose-500" />
            <span>Только с проблемами</span>
          </button>
        </div>

        {/* Right Toolbar Options */}
        <div className="flex items-center gap-2 ml-auto">
          {hasActiveFilters && (
            <button
              onClick={handleResetAllFilters}
              className="px-3 py-1.5 text-xs text-rose-600 dark:text-rose-400 font-medium hover:underline flex items-center gap-1"
            >
              <RotateCcw className="w-3 h-3" />
              Сбросить
            </button>
          )}

          {/* Column Settings Button */}
          <div className="relative">
            <button
              onClick={() => setActivePopover(activePopover === 'columns' ? null : 'columns')}
              className="p-1.5 text-slate-600 dark:text-slate-400 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl hover:bg-slate-100 transition-colors shadow-sm"
              title="Настройка колонок"
            >
              <Settings className="w-4 h-4" />
            </button>
            <FilterPopover
              isOpen={activePopover === 'columns'}
              onClose={() => setActivePopover(null)}
              onReset={() => saveVisibleColumns(ALL_COLUMNS.map((c) => c.id))}
              onApply={() => setActivePopover(null)}
              widthClass="w-56"
            >
              <div className="space-y-2 text-xs">
                <span className="font-semibold text-slate-700 dark:text-slate-300 block border-b pb-1">Колонки таблицы</span>
                {ALL_COLUMNS.map((col) => (
                  <label key={col.id} className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      disabled={col.mandatory}
                      checked={visibleColumns.includes(col.id)}
                      onChange={(e) => {
                        if (col.mandatory) return;
                        if (e.target.checked) {
                          saveVisibleColumns([...visibleColumns, col.id]);
                        } else {
                          saveVisibleColumns(visibleColumns.filter((id) => id !== col.id));
                        }
                      }}
                      className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500 disabled:opacity-50"
                    />
                    <span className={col.mandatory ? 'font-semibold' : ''}>{col.label}</span>
                  </label>
                ))}
              </div>
            </FilterPopover>
          </div>
        </div>
      </div>

      {/* Floating Bulk Actions Bar (Safe Restoration) */}
      {selectedIds.length > 0 && (
        <div className="bg-indigo-900 text-white p-3 rounded-2xl shadow-xl flex items-center justify-between gap-4 animate-in fade-in slide-in-from-top-2">
          <div className="flex items-center gap-3">
            <span className="bg-indigo-800 px-2.5 py-1 rounded-lg text-xs font-semibold">
              Выбрано: {selectedIds.length}
            </span>
            <span className="text-xs text-indigo-200 hidden sm:inline">Выполнить действие над выбранными:</span>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={() => setBulkActionModal({ action: 'hide', label: 'Скрыть выбранные товары' })}
              className="px-3 py-1.5 text-xs bg-indigo-800 hover:bg-indigo-700 rounded-xl transition-colors font-medium flex items-center gap-1.5"
            >
              <EyeOff className="w-3.5 h-3.5" />
              Скрыть
            </button>
            <button
              onClick={() => setBulkActionModal({ action: 'restore_storefront', label: 'Вернуть на витрину (только для одобренных)' })}
              className="px-3 py-1.5 text-xs bg-emerald-600 hover:bg-emerald-500 rounded-xl transition-colors font-medium flex items-center gap-1.5"
            >
              <Eye className="w-3.5 h-3.5" />
              Вернуть на витрину
            </button>
            <button
              onClick={() => setBulkActionModal({ action: 'block', label: 'Заблокировать выбранные товары' })}
              className="px-3 py-1.5 text-xs bg-rose-700 hover:bg-rose-600 rounded-xl transition-colors font-medium flex items-center gap-1.5"
            >
              <Lock className="w-3.5 h-3.5" />
              Заблокировать
            </button>
            <button
              onClick={() => setSelectedIds([])}
              className="px-2.5 py-1.5 text-xs text-indigo-300 hover:text-white transition-colors ml-2"
            >
              Отмена
            </button>
          </div>
        </div>
      )}

      {/* Main Table Container */}
      <div className="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-sm overflow-hidden">
        {error ? (
          <div className="p-8 text-center">
            <AlertCircle className="w-8 h-8 text-rose-500 mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-800 dark:text-slate-200">{error}</p>
            <button
              onClick={() => fetchProducts()}
              className="mt-4 px-4 py-2 text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-xl transition-colors"
            >
              Повторить попытку
            </button>
          </div>
        ) : isLoading ? (
          <div className="p-16 text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-2 border-indigo-600 border-t-transparent mx-auto"></div>
            <p className="mt-3 text-xs text-slate-500">Загрузка товаров...</p>
          </div>
        ) : products.length === 0 ? (
          <div className="p-16 text-center">
            <Package className="w-12 h-12 text-slate-300 dark:text-slate-700 mx-auto mb-3" />
            <h3 className="text-sm font-semibold text-slate-900 dark:text-white">Товары не найдены</h3>
            <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
              По выбранным условиям товары не найдены.
            </p>
            {hasActiveFilters && (
              <button
                onClick={handleResetAllFilters}
                className="mt-4 px-4 py-2 text-xs font-medium text-indigo-600 bg-indigo-50 rounded-xl hover:bg-indigo-100 transition-colors"
              >
                Сбросить фильтры
              </button>
            )}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-slate-50/70 dark:bg-slate-800/50 border-b border-slate-200 dark:border-slate-800 text-[11px] font-semibold uppercase tracking-wider text-slate-500">
                  <th className="py-3 px-4 w-10">
                    <input
                      type="checkbox"
                      checked={selectedIds.length === products.length && products.length > 0}
                      onChange={toggleSelectAll}
                      className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500 cursor-pointer"
                    />
                  </th>
                  {visibleColumns.includes('product') && (
                    <th className="py-3 px-4 cursor-pointer group" onClick={() => handleSort('title')}>
                      <div className="flex items-center gap-1.5">
                        <span>Товар</span>
                        {getSortIcon('title')}
                      </div>
                    </th>
                  )}
                  {visibleColumns.includes('seller') && (
                    <th className="py-3 px-4 cursor-pointer group" onClick={() => handleSort('seller_name')}>
                      <div className="flex items-center gap-1.5">
                        <span>Продавец</span>
                        {getSortIcon('seller_name')}
                      </div>
                    </th>
                  )}
                  {visibleColumns.includes('category_brand') && <th className="py-3 px-4">Категория / Бренд</th>}
                  {visibleColumns.includes('status') && (
                    <th className="py-3 px-4 cursor-pointer group" onClick={() => handleSort('status')}>
                      <div className="flex items-center gap-1.5">
                        <span>Статус</span>
                        {getSortIcon('status')}
                      </div>
                    </th>
                  )}
                  {visibleColumns.includes('price') && (
                    <th className="py-3 px-4 cursor-pointer group text-right" onClick={() => handleSort('price')}>
                      <div className="flex items-center justify-end gap-1.5">
                        <span>Цена</span>
                        {getSortIcon('price')}
                      </div>
                    </th>
                  )}
                  {visibleColumns.includes('variants') && (
                    <th className="py-3 px-4 cursor-pointer group text-center" onClick={() => handleSort('variants_count')}>
                      <div className="flex items-center justify-center gap-1.5">
                        <span>Варианты</span>
                        {getSortIcon('variants_count')}
                      </div>
                    </th>
                  )}
                  {visibleColumns.includes('stock') && <th className="py-3 px-4 text-center">Остаток</th>}
                  {visibleColumns.includes('sales') && <th className="py-3 px-4 text-right">Продажи</th>}
                  {visibleColumns.includes('rating') && <th className="py-3 px-4 text-center">Рейтинг</th>}
                  {visibleColumns.includes('updated_at') && (
                    <th className="py-3 px-4 cursor-pointer group" onClick={() => handleSort('created_at')}>
                      <div className="flex items-center gap-1.5">
                        <span>Обновлён</span>
                        {getSortIcon('created_at')}
                      </div>
                    </th>
                  )}
                  {visibleColumns.includes('actions') && <th className="py-3 px-4 text-right sticky right-0 bg-slate-50/90 backdrop-blur-sm">Действие</th>}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-xs">
                {products.map((p) => {
                  const st = STATUS_CONFIG[p.status] || { label: p.status, badge: 'bg-slate-100 text-slate-800' };
                  const isSelected = selectedIds.includes(p.id);
                  const stInfo = computeStockInfo(p.stock, p.reservedStock, p.variants?.length ?? 1, true);

                  return (
                    <tr
                      key={p.id}
                      className={`hover:bg-slate-50/80 transition-colors ${
                        isSelected ? 'bg-indigo-50/30' : ''
                      }`}
                    >
                      <td className="py-3 px-4">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => toggleSelectOne(p.id)}
                          className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500 cursor-pointer"
                        />
                      </td>

                      {/* Product Info */}
                      {visibleColumns.includes('product') && (
                        <td className="py-3 px-4 max-w-xs">
                          <div className="flex items-center gap-3">
                            {p.image ? (
                              <img src={p.image} alt={p.title} className="w-10 h-10 rounded-lg object-cover bg-slate-100 border border-slate-200 flex-shrink-0" />
                            ) : (
                              <div className="w-10 h-10 rounded-lg bg-slate-100 border border-slate-200 flex items-center justify-center flex-shrink-0">
                                <Package className="w-5 h-5 text-slate-400" />
                              </div>
                            )}
                            <div className="min-w-0">
                              <button
                                onClick={() => navigate(`/products/${p.id}`)}
                                className="font-semibold text-slate-900 dark:text-white hover:text-indigo-600 text-left line-clamp-1 block"
                              >
                                {p.title}
                              </button>
                              <div className="text-[11px] text-slate-500 font-mono mt-0.5 flex items-center gap-2">
                                <span>ID: {p.id.slice(0, 8)}...</span>
                              </div>
                            </div>
                          </div>
                        </td>
                      )}

                      {/* Seller Info */}
                      {visibleColumns.includes('seller') && (
                        <td className="py-3 px-4">
                          <button
                            onClick={() => p.sellerId && navigate(`/sellers/${p.sellerId}?tab=catalog`)}
                            className="font-medium text-indigo-600 hover:underline text-left block"
                          >
                            {p.sellerName || 'Магазин не назван'}
                          </button>
                          <span className="text-[11px] text-slate-400 block mt-0.5">{p.sellerOwnerEmail || '—'}</span>
                        </td>
                      )}

                      {/* Category & Brand */}
                      {visibleColumns.includes('category_brand') && (
                        <td className="py-3 px-4">
                          <span className="text-slate-800 font-medium block">{p.category || 'Не указана'}</span>
                          <span className="text-[11px] text-slate-400 block mt-0.5">{p.brand || 'Без бренда'}</span>
                        </td>
                      )}

                      {/* Status */}
                      {visibleColumns.includes('status') && (
                        <td className="py-3 px-4 whitespace-nowrap">
                          <span className={`px-2.5 py-1 rounded-full text-[11px] font-medium border ${st.badge}`}>
                            {st.label}
                          </span>
                        </td>
                      )}

                      {/* Price (Single format, rubles input) */}
                      {visibleColumns.includes('price') && (
                        <td className="py-3 px-4 text-right font-medium text-slate-900 dark:text-white whitespace-nowrap">
                          {formatMoneyRubles(p.price)}
                        </td>
                      )}

                      {/* Variants */}
                      {visibleColumns.includes('variants') && (
                        <td className="py-3 px-4 text-center">
                          {p.variants && p.variants.length > 0 ? (
                            <span className="bg-slate-100 px-2 py-0.5 rounded text-[11px] font-medium text-slate-700">
                              {p.variants.length} вар.
                            </span>
                          ) : (
                            <span className="text-rose-500 font-medium text-[11px] bg-rose-50 px-2 py-0.5 rounded">
                              0 вар.
                            </span>
                          )}
                        </td>
                      )}

                      {/* Stock (Unified badge) */}
                      {visibleColumns.includes('stock') && (
                        <td className="py-3 px-4 text-center whitespace-nowrap">
                          <span className={`px-2 py-0.5 rounded text-[11px] font-medium border ${stInfo.badgeClass}`}>
                            {stInfo.label}
                          </span>
                        </td>
                      )}

                      {/* Sales */}
                      {visibleColumns.includes('sales') && (
                        <td className="py-3 px-4 text-right whitespace-nowrap">
                          <span className="text-slate-700">Нет продаж</span>
                        </td>
                      )}

                      {/* Rating */}
                      {visibleColumns.includes('rating') && (
                        <td className="py-3 px-4 text-center whitespace-nowrap">
                          {p.rating && p.rating > 0 ? (
                            <span className="text-amber-600 font-semibold">★ {p.rating.toFixed(1)}</span>
                          ) : (
                            <span className="text-slate-400 text-[11px]">Нет отзывов</span>
                          )}
                        </td>
                      )}

                      {/* Updated Date */}
                      {visibleColumns.includes('updated_at') && (
                        <td className="py-3 px-4 text-slate-500 whitespace-nowrap">
                          {formatDate(p.createdAt)}
                        </td>
                      )}

                      {/* Actions Sticky Column */}
                      {visibleColumns.includes('actions') && (
                        <td className="py-3 px-4 text-right sticky right-0 bg-white shadow-sm whitespace-nowrap">
                          <button
                            onClick={() => navigate(`/products/${p.id}`)}
                            className="px-3 py-1 text-xs font-semibold text-indigo-600 bg-indigo-50 hover:bg-indigo-100 rounded-lg transition-colors inline-flex items-center gap-1"
                          >
                            <span>Открыть</span>
                            <ChevronRight className="w-3.5 h-3.5" />
                          </button>
                        </td>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination Footer */}
        <div className="p-4 border-t border-slate-200 dark:border-slate-800 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-500">
          <div>
            Показано {products.length > 0 ? (queryPage - 1) * queryLimit + 1 : 0} –{' '}
            {Math.min(queryPage * queryLimit, totalCount)} из {totalCount} товаров
          </div>

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <span>Строк на странице:</span>
              <select
                value={queryLimit}
                onChange={(e) => updateParam('limit', e.target.value)}
                className="bg-slate-50 border border-slate-200 rounded-lg px-2 py-1 text-xs focus:outline-none"
              >
                <option value="25">25</option>
                <option value="50">50</option>
                <option value="100">100</option>
              </select>
            </div>

            <div className="flex items-center gap-1">
              <button
                disabled={queryPage <= 1}
                onClick={() => updateParam('page', String(queryPage - 1), false)}
                className="px-3 py-1.5 rounded-lg border border-slate-200 bg-white disabled:opacity-40 hover:bg-slate-50 transition-colors"
              >
                Назад
              </button>
              <span className="px-2 font-medium text-slate-900">
                {queryPage} / {totalPages}
              </span>
              <button
                disabled={queryPage >= totalPages}
                onClick={() => updateParam('page', String(queryPage + 1), false)}
                className="px-3 py-1.5 rounded-lg border border-slate-200 bg-white disabled:opacity-40 hover:bg-slate-50 transition-colors"
              >
                Вперёд
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Bulk Action Confirmation Modal */}
      {bulkActionModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-slate-900 rounded-2xl shadow-2xl max-w-md w-full p-6 border border-slate-200 dark:border-slate-800 space-y-4">
            <h3 className="text-base font-bold text-slate-900 dark:text-white">{bulkActionModal.label}</h3>
            <p className="text-xs text-slate-500">Выбрано товаров: {selectedIds.length}</p>

            <div className="mb-4">
              <label className="text-xs font-medium text-slate-700 dark:text-slate-300 block mb-1">Укажите причину (обязательно)</label>
              <textarea
                value={bulkReason}
                onChange={(e) => setBulkReason(e.target.value)}
                placeholder="Причина массового изменения..."
                rows={3}
                className="w-full p-2.5 text-xs bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-xl focus:outline-none"
              />
            </div>

            <div className="flex items-center justify-end gap-3">
              <button
                onClick={() => setBulkActionModal(null)}
                className="px-4 py-2 text-xs font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 rounded-xl transition-colors"
              >
                Отмена
              </button>
              <button
                onClick={handleBulkSubmit}
                disabled={isBulkSubmitting || !bulkReason.trim()}
                className="px-4 py-2 text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 rounded-xl transition-colors shadow-sm"
              >
                {isBulkSubmitting ? 'Выполняем...' : 'Подтвердить'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
