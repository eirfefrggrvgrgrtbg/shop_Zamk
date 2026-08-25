import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useVisibilityPolling } from '../hooks/useVisibilityPolling';
import {
  AlertCircle,
  RefreshCw,
  Search,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  CheckCircle2,
  AlertTriangle,
  Clock,
  ChevronDown,
  X,
} from 'lucide-react';
import { SellerContextBanner } from '../components/SellerContextBanner';
import { getModerationProducts, getAdminProductErrorMessage } from '../api/adminProducts';
import type { AdminProductView } from '../api/adminProducts';
import { getProductStatusConfig, PRODUCT_STATUS_MAP } from '../utils/productStatusMapper';
import { getAdminSellers, getAdminCategories, getAdminBrands } from '@zamk/api-client/src/admin';

interface SellerOption {
  id: string;
  name: string;
}

interface OptionItem {
  id: string;
  name: string;
}

export function AdminModeration() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // State
  const [products, setProducts] = useState<AdminProductView[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sellersList, setSellersList] = useState<SellerOption[]>([]);
  const [categoriesList, setCategoriesList] = useState<OptionItem[]>([]);
  const [brandsList, setBrandsList] = useState<OptionItem[]>([]);
  const [categorySearch, setCategorySearch] = useState('');
  const [brandSearch, setBrandSearch] = useState('');

  // Popover States
  const [activePopover, setActivePopover] = useState<string | null>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  // Extract params from URL
  const queryQ = searchParams.get('q') || '';
  const queryStatus = searchParams.get('status') || 'pending_moderation';
  const querySellerId = searchParams.get('sellerId') || '';
  const queryCategoryId = searchParams.get('categoryId') || '';
  const queryCategoryIds = (searchParams.get('categoryIds') || '').split(',').filter(Boolean);
  const queryBrandId = searchParams.get('brandId') || '';
  const queryBrandIds = (searchParams.get('brandIds') || '').split(',').filter(Boolean);
  const queryHasProblems = searchParams.get('hasProblems') === 'true';
  const querySubmittedPeriod = searchParams.get('submittedPeriod') || '';
  const querySortBy = searchParams.get('sortBy') || 'submitted_at';
  const querySortOrder = searchParams.get('sortOrder') || 'asc';

  const flagKeys = [
    'noMainImage',
    'noDescription',
    'noBrand',
    'noVariants',
    'noPrice',
    'duplicateSku',
    'noStock',
    'resubmitted',
  ];
  const activeFlagsCount = flagKeys.filter((k) => searchParams.get(k) === 'true').length;

  // Search input state
  const [searchInput, setSearchInput] = useState(queryQ);

  const abortControllerRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef<number>(0);

  // Close popovers on outside click
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
        setActivePopover(null);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Fetch sellers, categories, and brands for filter popovers
  useEffect(() => {
    getAdminSellers({ limit: 100 })
      .then((res: { items?: any[] }) => {
        const mapped = (res.items || []).map((s: any) => ({
          id: s.id,
          name: s.brandName || s.name || s.id,
        }));
        setSellersList(mapped);
      })
      .catch(() => {});

    getAdminCategories()
      .then((res: any) => {
        const items = Array.isArray(res) ? res : res.items || [];
        setCategoriesList(items.map((c: any) => ({ id: c.id, name: c.name })));
      })
      .catch(() => {});

    getAdminBrands()
      .then((res: any) => {
        const items = Array.isArray(res) ? res : res.items || [];
        setBrandsList(items.map((b: any) => ({ id: b.id, name: b.name })));
      })
      .catch(() => {});
  }, []);

  // Helper to update search params
  const updateParam = (key: string, value: string | null) => {
    const next = new URLSearchParams(searchParams);
    if (value === null || value === '') {
      next.delete(key);
    } else {
      next.set(key, value);
    }
    setSearchParams(next, { replace: true });
  };

  const toggleCategoryFilter = (catId: string) => {
    let next: string[];
    if (queryCategoryIds.includes(catId)) {
      next = queryCategoryIds.filter((id) => id !== catId);
    } else {
      next = [...queryCategoryIds, catId];
    }
    updateParam('categoryIds', next.length > 0 ? next.join(',') : null);
  };

  const toggleBrandFilter = (bId: string) => {
    let next: string[];
    if (queryBrandIds.includes(bId)) {
      next = queryBrandIds.filter((id) => id !== bId);
    } else {
      next = [...queryBrandIds, bId];
    }
    updateParam('brandIds', next.length > 0 ? next.join(',') : null);
  };

  // Restore scroll position when returning from product detail
  useEffect(() => {
    const savedY = sessionStorage.getItem('adminModerationScrollY');
    if (savedY) {
      window.scrollTo(0, parseInt(savedY, 10));
      sessionStorage.removeItem('adminModerationScrollY');
    }
  }, [products]);

  // Fetch moderation queue products
  const fetchProducts = async (quiet = false) => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    abortControllerRef.current = new AbortController();
    const signal = abortControllerRef.current.signal;
    
    requestIdRef.current += 1;
    const currentRequestId = requestIdRef.current;

    try {
      if (!quiet) {
        setIsLoading(true);
        setProducts([]); // Clear old table data when filters change
      } else {
        setIsRefreshing(true);
      }
      setError(null);

      const res = await getModerationProducts({
        q: queryQ,
        status: queryStatus,
        sellerId: querySellerId,
        categoryId: queryCategoryId,
        categoryIds: queryCategoryIds.length > 0 ? queryCategoryIds.join(',') : undefined,
        brandId: queryBrandId,
        brandIds: queryBrandIds.length > 0 ? queryBrandIds.join(',') : undefined,
        submittedPeriod: querySubmittedPeriod || undefined,
        noMainImage: searchParams.get('noMainImage') === 'true' || undefined,
        noDescription: searchParams.get('noDescription') === 'true' || undefined,
        noBrand: searchParams.get('noBrand') === 'true' || undefined,
        noVariants: searchParams.get('noVariants') === 'true' || undefined,
        noPrice: searchParams.get('noPrice') === 'true' || undefined,
        duplicateSku: searchParams.get('duplicateSku') === 'true' || undefined,
        noStock: searchParams.get('noStock') === 'true' || undefined,
        resubmitted: searchParams.get('resubmitted') === 'true' || undefined,
        hasProblems: queryHasProblems,
        sortBy: querySortBy,
        sortOrder: querySortOrder,
        limit: 50,
        signal,
      } as any);

      if (signal.aborted || currentRequestId !== requestIdRef.current) return;

      setProducts(res.items);
      setTotalCount(res.totalCount);
    } catch (err: unknown) {
      if (signal.aborted || currentRequestId !== requestIdRef.current) return;
      if (!quiet) {
        setError(getAdminProductErrorMessage(err, 'Не удалось загрузить очередь модерации.'));
      }
    } finally {
      if (!signal.aborted && currentRequestId === requestIdRef.current) {
        setIsLoading(false);
        setIsRefreshing(false);
      }
    }
  };

  useEffect(() => {
    fetchProducts();
  }, [
    queryQ,
    queryStatus,
    querySellerId,
    queryCategoryId,
    queryCategoryIds.join(','),
    queryBrandId,
    queryBrandIds.join(','),
    querySubmittedPeriod,
    queryHasProblems,
    querySortBy,
    querySortOrder,
    searchParams.toString(),
  ]);

  useVisibilityPolling(useCallback(() => fetchProducts(true), []), 4000);

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    updateParam('q', searchInput.trim());
  };

  // Toggle sorting
  const handleSort = (field: string) => {
    if (querySortBy === field) {
      const nextOrder = querySortOrder === 'asc' ? 'desc' : 'asc';
      const next = new URLSearchParams(searchParams);
      next.set('sortOrder', nextOrder);
      setSearchParams(next, { replace: true });
    } else {
      const next = new URLSearchParams(searchParams);
      next.set('sortBy', field);
      next.set('sortOrder', 'asc');
      setSearchParams(next, { replace: true });
    }
  };

  const renderSortIndicator = (field: string) => {
    if (querySortBy !== field) return <ArrowUpDown className="h-3.5 w-3.5 ml-1 text-slate-400 opacity-60 inline" />;
    return querySortOrder === 'asc' ? (
      <ArrowUp className="h-3.5 w-3.5 ml-1 text-indigo-600 dark:text-indigo-400 inline" />
    ) : (
      <ArrowDown className="h-3.5 w-3.5 ml-1 text-indigo-600 dark:text-indigo-400 inline" />
    );
  };

  // Calculate waiting time badge
  const getWaitingTime = (submittedAt?: string) => {
    if (!submittedAt) return { text: 'Свежий', badgeClass: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300' };
    const diffMs = Date.now() - new Date(submittedAt).getTime();
    const hours = Math.floor(diffMs / (1000 * 60 * 60));
    const days = Math.floor(hours / 24);

    if (hours < 24) {
      return {
        text: hours <= 0 ? '< 1 ч' : `${hours} ч`,
        badgeClass: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/50',
      };
    } else if (hours < 48) {
      return {
        text: `${hours} ч (${days} дн)`,
        badgeClass: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300 border border-amber-200 dark:border-amber-800/50 font-medium',
      };
    } else {
      return {
        text: `${days} дн (${hours} ч)`,
        badgeClass: 'bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300 border border-rose-300 dark:border-rose-800 font-bold animate-pulse',
      };
    }
  };

  // Count product issues (deterministic rules)
  const getProductProblemsCount = (p: AdminProductView) => {
    let count = 0;
    if (!p.image) count++;
    if (!p.description || p.description.trim().length === 0) count++;
    if (!p.category) count++;
    if (!p.brand) count++;
    if (p.variants.some((v) => !v.price || v.price === 0)) count++;

    // Duplicate SKU
    const skus = p.variants.map((v) => v.sku).filter(Boolean);
    if (new Set(skus).size < skus.length) count++;

    return count;
  };

  // Open product detail page
  const handleOpenProduct = (productId: string) => {
    sessionStorage.setItem('adminModerationScrollY', window.scrollY.toString());
    sessionStorage.setItem('adminModerationReturnURL', location.pathname + location.search);
    navigate(`/moderation/products/${productId}`);
  };

  // Format currency helper
  const formatPrice = (price: number, currency: string) => {
    return new Intl.NumberFormat('ru-RU', { style: 'currency', currency, maximumFractionDigits: 0 }).format(price);
  };

  return (
    <div className="space-y-6">
      {/* Seller Context Banner */}
      <SellerContextBanner />

      {/* Header Section */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 bg-white dark:bg-slate-900 p-6 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-slate-900 dark:text-white">Модерация товаров</h1>
            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-200 dark:border-amber-800/50">
              {totalCount} в очереди
            </span>
          </div>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Полноценное рабочее место модератора по проверке карточек товаров, характеристик и вариантов
          </p>
        </div>

        {/* Refresh Icon Button */}
        <button
          onClick={() => fetchProducts(true)}
          disabled={isRefreshing}
          aria-label="Обновить очередь товаров"
          title="Обновить очередь"
          className="inline-flex items-center justify-center p-2.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 shadow-sm transition-all"
        >
          <RefreshCw className={`h-5 w-5 ${isRefreshing ? 'animate-spin text-indigo-600' : ''}`} />
        </button>
      </div>

      {/* Search & Pill Filter Bar */}
      <div className="bg-white dark:bg-slate-900 p-4 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm space-y-4" ref={popoverRef}>
        <div className="flex flex-col md:flex-row md:items-center gap-3">
          {/* Search Input */}
          <form onSubmit={handleSearchSubmit} className="flex-1 relative">
            <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Поиск по название товара, SKU, артикулу, продавцу..."
              className="w-full pl-10 pr-10 py-2 text-sm bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:bg-white dark:focus:bg-slate-900 dark:text-white transition-all"
            />
            {searchInput && (
              <button
                type="button"
                onClick={() => {
                  setSearchInput('');
                  updateParam('q', null);
                }}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </form>

          {/* Pill Filters */}
          <div className="flex items-center flex-wrap gap-2 relative">
            {/* Status Filter Pill */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActivePopover(activePopover === 'status' ? null : 'status')}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                  queryStatus !== 'all'
                    ? 'bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-800'
                    : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
                }`}
              >
                <span>Статус: {PRODUCT_STATUS_MAP[queryStatus]?.label || queryStatus}</span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>

              {activePopover === 'status' && (
                <div className="absolute left-0 mt-2 w-56 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 py-1.5 z-50 animate-in fade-in zoom-in-95">
                  <div className="px-3 py-1 text-[11px] font-semibold text-slate-400 uppercase">Фильтр статуса</div>
                  {[
                    { id: 'pending_moderation', label: 'Ожидает модерации' },
                    { id: 'approved', label: 'Одобрен' },
                    { id: 'rejected', label: 'Требуются исправления' },
                    { id: 'published', label: 'Опубликован' },
                    { id: 'hidden', label: 'Скрыт' },
                    { id: 'blocked', label: 'Заблокирован' },
                    { id: 'all', label: 'Все статусы' },
                  ].map((item) => (
                    <button
                      key={item.id}
                      onClick={() => {
                        updateParam('status', item.id === 'pending_moderation' ? null : item.id);
                        setActivePopover(null);
                      }}
                      className={`w-full text-left px-3 py-2 text-xs hover:bg-slate-100 dark:hover:bg-slate-700 flex items-center justify-between ${
                        queryStatus === item.id ? 'font-semibold text-indigo-600 dark:text-indigo-400 bg-indigo-50/50 dark:bg-indigo-950/30' : 'text-slate-700 dark:text-slate-300'
                      }`}
                    >
                      <span>{item.label}</span>
                      {queryStatus === item.id && <CheckCircle2 className="h-3.5 w-3.5 text-indigo-600" />}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Seller Filter Pill */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActivePopover(activePopover === 'seller' ? null : 'seller')}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                  querySellerId
                    ? 'bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-800'
                    : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
                }`}
              >
                <span>
                  Продавец: {sellersList.find((s) => s.id === querySellerId)?.name || (querySellerId ? 'Выбран' : 'Все')}
                </span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>

              {activePopover === 'seller' && (
                <div className="absolute left-0 mt-2 w-64 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 py-1.5 z-50 max-h-60 overflow-y-auto animate-in fade-in zoom-in-95">
                  <button
                    type="button"
                    onClick={() => {
                      updateParam('sellerId', null);
                      setActivePopover(null);
                    }}
                    className="w-full text-left px-3 py-2 text-xs text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-700"
                  >
                    Все продавцы
                  </button>
                  {sellersList.map((s) => (
                    <button
                      key={s.id}
                      type="button"
                      onClick={() => {
                        updateParam('sellerId', s.id);
                        setActivePopover(null);
                      }}
                      className={`w-full text-left px-3 py-2 text-xs hover:bg-slate-100 dark:hover:bg-slate-700 truncate ${
                        querySellerId === s.id ? 'font-semibold text-indigo-600 dark:text-indigo-400 bg-indigo-50/50' : 'text-slate-700 dark:text-slate-300'
                      }`}
                    >
                      {s.name}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Category Filter Pill */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActivePopover(activePopover === 'category' ? null : 'category')}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                  queryCategoryIds.length > 0
                    ? 'bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-800'
                    : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
                }`}
              >
                <span>
                  Категория: {queryCategoryIds.length > 0 ? `Выбрано (${queryCategoryIds.length})` : 'Все'}
                </span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>

              {activePopover === 'category' && (
                <div className="absolute left-0 mt-2 w-64 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 p-2 z-50 animate-in fade-in zoom-in-95 space-y-2">
                  <div className="px-2 py-1 text-[11px] font-semibold text-slate-400 uppercase">Фильтр по категориям</div>
                  <input
                    type="text"
                    value={categorySearch}
                    onChange={(e) => setCategorySearch(e.target.value)}
                    placeholder="Поиск категории..."
                    className="w-full px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  />
                  <div className="max-h-48 overflow-y-auto space-y-1">
                    {categoriesList
                      .filter((c) => c.name.toLowerCase().includes(categorySearch.toLowerCase()))
                      .map((cat) => {
                        const isSelected = queryCategoryIds.includes(cat.id);
                        return (
                          <button
                            key={cat.id}
                            type="button"
                            onClick={() => toggleCategoryFilter(cat.id)}
                            className={`w-full text-left px-2.5 py-1.5 text-xs rounded-md flex items-center justify-between transition-colors ${
                              isSelected ? 'bg-indigo-50 dark:bg-indigo-950/50 font-semibold text-indigo-600 dark:text-indigo-400' : 'hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300'
                            }`}
                          >
                            <span className="truncate">{cat.name}</span>
                            {isSelected && <CheckCircle2 className="h-3.5 w-3.5 text-indigo-600 flex-shrink-0" />}
                          </button>
                        );
                      })}
                  </div>
                  {queryCategoryIds.length > 0 && (
                    <button
                      type="button"
                      onClick={() => updateParam('categoryIds', null)}
                      className="w-full text-center py-1 text-[11px] text-rose-600 hover:underline"
                    >
                      Сбросить категории
                    </button>
                  )}
                </div>
              )}
            </div>

            {/* Brand Filter Pill */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActivePopover(activePopover === 'brand' ? null : 'brand')}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                  queryBrandIds.length > 0
                    ? 'bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-800'
                    : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
                }`}
              >
                <span>
                  Бренд: {queryBrandIds.length > 0 ? `Выбрано (${queryBrandIds.length})` : 'Все'}
                </span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>

              {activePopover === 'brand' && (
                <div className="absolute left-0 mt-2 w-64 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 p-2 z-50 animate-in fade-in zoom-in-95 space-y-2">
                  <div className="px-2 py-1 text-[11px] font-semibold text-slate-400 uppercase">Фильтр по брендам</div>
                  <input
                    type="text"
                    value={brandSearch}
                    onChange={(e) => setBrandSearch(e.target.value)}
                    placeholder="Поиск бренда..."
                    className="w-full px-2.5 py-1.5 text-xs bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  />
                  <div className="max-h-48 overflow-y-auto space-y-1">
                    {brandsList
                      .filter((b) => b.name.toLowerCase().includes(brandSearch.toLowerCase()))
                      .map((brand) => {
                        const isSelected = queryBrandIds.includes(brand.id);
                        return (
                          <button
                            key={brand.id}
                            type="button"
                            onClick={() => toggleBrandFilter(brand.id)}
                            className={`w-full text-left px-2.5 py-1.5 text-xs rounded-md flex items-center justify-between transition-colors ${
                              isSelected ? 'bg-indigo-50 dark:bg-indigo-950/50 font-semibold text-indigo-600 dark:text-indigo-400' : 'hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300'
                            }`}
                          >
                            <span className="truncate">{brand.name}</span>
                            {isSelected && <CheckCircle2 className="h-3.5 w-3.5 text-indigo-600 flex-shrink-0" />}
                          </button>
                        );
                      })}
                  </div>
                  {queryBrandIds.length > 0 && (
                    <button
                      type="button"
                      onClick={() => updateParam('brandIds', null)}
                      className="w-full text-center py-1 text-[11px] text-rose-600 hover:underline"
                    >
                      Сбросить бренды
                    </button>
                  )}
                </div>
              )}
            </div>

            {/* Submitted Period Filter Pill */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActivePopover(activePopover === 'submitted' ? null : 'submitted')}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                  querySubmittedPeriod
                    ? 'bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-800'
                    : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
                }`}
              >
                <span>
                  Дата отправки:{' '}
                  {querySubmittedPeriod === 'today'
                    ? 'Сегодня'
                    : querySubmittedPeriod === '3days'
                    ? 'Последние 3 дня'
                    : querySubmittedPeriod === '7days'
                    ? 'Последние 7 дней'
                    : querySubmittedPeriod === '30days'
                    ? 'Последние 30 дней'
                    : querySubmittedPeriod === 'custom'
                    ? 'Диапазон'
                    : 'Все'}
                </span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>

              {activePopover === 'submitted' && (
                <div className="absolute left-0 mt-2 w-64 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 p-2 z-50 animate-in fade-in zoom-in-95 space-y-1">
                  <div className="px-2 py-1 text-[11px] font-semibold text-slate-400 uppercase">Период отправки</div>
                  {[
                    { id: '', label: 'Все даты' },
                    { id: 'today', label: 'Сегодня' },
                    { id: '3days', label: 'Последние 3 дня' },
                    { id: '7days', label: 'Последние 7 дней' },
                    { id: '30days', label: 'Последние 30 дней' },
                  ].map((p) => (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => {
                        updateParam('submittedPeriod', p.id || null);
                        setActivePopover(null);
                      }}
                      className={`w-full text-left px-2.5 py-1.5 text-xs rounded-md flex items-center justify-between ${
                        querySubmittedPeriod === p.id ? 'bg-indigo-50 dark:bg-indigo-950/50 font-semibold text-indigo-600' : 'hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300'
                      }`}
                    >
                      <span>{p.label}</span>
                      {querySubmittedPeriod === p.id && <CheckCircle2 className="h-3.5 w-3.5 text-indigo-600" />}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* + Ещё (Flag Filters) Pill */}
            <div className="relative">
              <button
                type="button"
                onClick={() => setActivePopover(activePopover === 'more' ? null : 'more')}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                  activeFlagsCount > 0
                    ? 'bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/50 dark:text-indigo-300 dark:border-indigo-800 font-semibold'
                    : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
                }`}
              >
                <span>+ Ещё {activeFlagsCount > 0 ? `(${activeFlagsCount})` : ''}</span>
                <ChevronDown className="h-3.5 w-3.5" />
              </button>

              {activePopover === 'more' && (
                <div className="absolute left-0 mt-2 w-64 bg-white dark:bg-slate-800 rounded-xl shadow-xl border border-slate-200 dark:border-slate-700 p-2 z-50 animate-in fade-in zoom-in-95 space-y-1">
                  <div className="px-2 py-1 text-[11px] font-semibold text-slate-400 uppercase">Дополнительные фильтры</div>
                  {[
                    { key: 'noMainImage', label: 'Без главного фото' },
                    { key: 'noDescription', label: 'Без описания' },
                    { key: 'noBrand', label: 'Без бренда' },
                    { key: 'noVariants', label: 'Нет вариантов' },
                    { key: 'noPrice', label: 'Вариант без цены' },
                    { key: 'duplicateSku', label: 'Дублирующийся SKU' },
                    { key: 'noStock', label: 'Нет остатков' },
                    { key: 'resubmitted', label: 'Повторная модерация' },
                  ].map((flag) => {
                    const isChecked = searchParams.get(flag.key) === 'true';
                    return (
                      <button
                        key={flag.key}
                        type="button"
                        onClick={() => updateParam(flag.key, isChecked ? null : 'true')}
                        className={`w-full text-left px-2.5 py-1.5 text-xs rounded-md flex items-center justify-between ${
                          isChecked ? 'bg-indigo-50 dark:bg-indigo-950/50 font-semibold text-indigo-600' : 'hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300'
                        }`}
                      >
                        <span>{flag.label}</span>
                        {isChecked && <CheckCircle2 className="h-3.5 w-3.5 text-indigo-600" />}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Problems Filter Pill */}
            <button
              type="button"
              onClick={() => updateParam('hasProblems', queryHasProblems ? null : 'true')}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
                queryHasProblems
                  ? 'bg-rose-50 text-rose-700 border-rose-300 dark:bg-rose-950/50 dark:text-rose-300 dark:border-rose-800'
                  : 'bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 border-slate-200 dark:border-slate-700 hover:bg-slate-50'
              }`}
            >
              <AlertTriangle className="h-3.5 w-3.5 text-amber-500" />
              <span>Только с проблемами</span>
            </button>

            {/* Reset Filters */}
            {(queryQ || queryStatus !== 'pending_moderation' || querySellerId || queryCategoryIds.length > 0 || queryBrandIds.length > 0 || querySubmittedPeriod || activeFlagsCount > 0 || queryHasProblems) && (
              <button
                type="button"
                onClick={() => {
                  setSearchInput('');
                  setSearchParams(new URLSearchParams(), { replace: true });
                }}
                className="text-xs text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 underline ml-2"
              >
                Сбросить фильтры
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="p-4 bg-rose-50 dark:bg-rose-950/50 text-rose-700 dark:text-rose-300 rounded-xl border border-rose-200 dark:border-rose-800 flex items-center">
          <AlertCircle className="h-5 w-5 mr-3 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Queue Table */}
      {isLoading ? (
        <div className="bg-white dark:bg-slate-900 rounded-xl p-12 text-center border border-slate-200 dark:border-slate-800">
          <div className="animate-spin rounded-full h-8 w-8 border-2 border-indigo-600 border-t-transparent mx-auto"></div>
          <p className="mt-3 text-sm text-slate-500 dark:text-slate-400">Загрузка товаров из очереди модерации...</p>
        </div>
      ) : products.length === 0 ? (
        <div className="bg-white dark:bg-slate-900 rounded-xl p-12 text-center text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-800 space-y-3">
          <CheckCircle2 className="h-12 w-12 text-emerald-500 mx-auto" />
          <h3 className="text-base font-semibold text-slate-900 dark:text-white">Очередь модерации пуста</h3>
          <p className="text-sm text-slate-500 max-w-md mx-auto">
            Все товары проверены или не соответствуют выбранным критериям фильтрации.
          </p>
        </div>
      ) : (
        <div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 shadow-sm overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-[1200px] w-full divide-y divide-slate-200 dark:divide-slate-800 text-left">
              <thead className="bg-slate-50 dark:bg-slate-800/60 text-[11px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                <tr>
                  <th scope="col" className="px-5 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('title')}>
                    Товар {renderSortIndicator('title')}
                  </th>
                  <th scope="col" className="px-4 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('seller_name')}>
                    Продавец {renderSortIndicator('seller_name')}
                  </th>
                  <th scope="col" className="px-4 py-3">Категория</th>
                  <th scope="col" className="px-4 py-3">Бренд</th>
                  <th scope="col" className="px-4 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('price')}>
                    Цена {renderSortIndicator('price')}
                  </th>
                  <th scope="col" className="px-4 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('variants_count')}>
                    Варианты {renderSortIndicator('variants_count')}
                  </th>
                  <th scope="col" className="px-4 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('submitted_at')}>
                    Отправлен {renderSortIndicator('submitted_at')}
                  </th>
                  <th scope="col" className="px-4 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('waiting_time')}>
                    Ожидание {renderSortIndicator('waiting_time')}
                  </th>
                  <th scope="col" className="px-4 py-3 cursor-pointer hover:text-slate-900" onClick={() => handleSort('status')}>
                    Статус {renderSortIndicator('status')}
                  </th>
                  <th scope="col" className="px-4 py-3">Проблемы</th>
                  <th scope="col" className="px-5 py-3 text-right sticky right-0 bg-slate-50 dark:bg-slate-800/90 z-10 shadow-[-4px_0_8px_-2px_rgba(0,0,0,0.05)]">Действие</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800 text-xs">
                {products.map((p) => {
                  const statusCfg = getProductStatusConfig(p.status);
                  const wait = getWaitingTime(p.submittedAt);
                  const problemsCount = getProductProblemsCount(p);

                  return (
                    <tr
                      key={p.id}
                      onClick={() => handleOpenProduct(p.id)}
                      className="hover:bg-slate-50/80 dark:hover:bg-slate-800/50 cursor-pointer transition-colors group"
                    >
                      {/* Product Thumbnail & Title */}
                      <td className="px-5 py-3.5">
                        <div className="flex items-center gap-3 max-w-xs">
                          {p.image ? (
                            <img src={p.image} alt={p.title} className="h-10 w-10 rounded-lg object-cover border border-slate-200 dark:border-slate-700 flex-shrink-0" />
                          ) : (
                            <div className="h-10 w-10 rounded-lg bg-amber-50 dark:bg-amber-950/40 border border-amber-200 dark:border-amber-800 flex items-center justify-center text-amber-600 font-semibold text-[10px] flex-shrink-0">
                              Без фото
                            </div>
                          )}
                          <div className="min-w-0">
                            <p className="font-semibold text-slate-900 dark:text-white truncate" title={p.title}>
                              {p.title}
                            </p>
                            <p className="text-[11px] text-slate-400 font-mono">ID: {p.id.slice(0, 8)}...</p>
                          </div>
                        </div>
                      </td>

                      {/* Seller */}
                      <td className="px-4 py-3.5">
                        <p className="font-medium text-slate-900 dark:text-white truncate max-w-[140px]">
                          {p.sellerName || 'Продавец'}
                        </p>
                        {p.sellerOwnerEmail && (
                          <p className="text-[11px] text-slate-400 truncate max-w-[140px]">{p.sellerOwnerEmail}</p>
                        )}
                      </td>

                      {/* Category */}
                      <td className="px-4 py-3.5 text-slate-600 dark:text-slate-300 truncate max-w-[120px]">
                        {p.category || '—'}
                      </td>

                      {/* Brand */}
                      <td className="px-4 py-3.5 text-slate-600 dark:text-slate-300 truncate max-w-[100px]">
                        {p.brand || '—'}
                      </td>

                      {/* Price */}
                      <td className="px-4 py-3.5 font-semibold text-slate-900 dark:text-white whitespace-nowrap">
                        {formatPrice(p.price, p.currency)}
                      </td>

                      {/* Variants count */}
                      <td className="px-4 py-3.5 text-slate-600 dark:text-slate-300">
                        {p.variants.length} вар.
                      </td>

                      {/* Submitted At */}
                      <td className="px-4 py-3.5 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                        {p.submittedAt ? new Date(p.submittedAt).toLocaleDateString('ru-RU') : '—'}
                      </td>

                      {/* Waiting time badge */}
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] ${wait.badgeClass}`}>
                          <Clock className="h-3 w-3" />
                          <span>{wait.text}</span>
                        </span>
                      </td>

                      {/* Status */}
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-medium border ${statusCfg.badgeClass}`}>
                          <span className={`h-1.5 w-1.5 rounded-full ${statusCfg.dotClass}`} />
                          <span>{statusCfg.label}</span>
                        </span>
                      </td>

                      {/* Problems badge */}
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        {problemsCount > 0 ? (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300 border border-rose-200 dark:border-rose-800">
                            <AlertTriangle className="h-3 w-3 text-rose-500" />
                            <span>{problemsCount} замейч.</span>
                          </span>
                        ) : (
                          <span className="text-emerald-600 dark:text-emerald-400 font-medium text-[11px]">Норма</span>
                        )}
                      </td>

                      {/* Action Button */}
                      <td className="px-5 py-3.5 text-right whitespace-nowrap sticky right-0 bg-white dark:bg-slate-900 group-hover:bg-slate-50 dark:group-hover:bg-slate-800/90 z-10 shadow-[-4px_0_8px_-2px_rgba(0,0,0,0.05)]">
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleOpenProduct(p.id);
                          }}
                          className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-semibold text-indigo-600 dark:text-indigo-400 hover:text-indigo-900 bg-indigo-50 dark:bg-indigo-950/50 hover:bg-indigo-100 dark:hover:bg-indigo-900/60 rounded-lg transition-all"
                        >
                          <span>Проверить →</span>
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
