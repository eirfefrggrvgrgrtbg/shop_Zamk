import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  getAdminSellers,
  createAdminSeller,
} from '@zamk/api-client/src/admin';
import type { AdminSeller } from '@zamk/api-client/src/types';
import {
  Plus,
  Store as StoreIcon,
  Search,
  AlertTriangle,
  X,
  Settings2,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { PermissionGuard } from '../components/PermissionGuard';
import { FilterPopover } from '../components/FilterPopover';
import { formatStatus, getStatusBadgeClass } from '../utils/statusMapper';

type SortField =
  | 'created_at'
  | 'owner_name'
  | 'brand_name'
  | 'status'
  | 'rating'
  | 'gross_sales'
  | 'orders_count'
  | 'warnings_active'
  | 'performance_score';

type SortOrder = 'asc' | 'desc';

interface ColumnDef {
  id: string;
  label: string;
  sortKey?: SortField;
  isNumeric?: boolean;
}

const ALL_COLUMNS: ColumnDef[] = [
  { id: 'owner', label: 'Продавец', sortKey: 'owner_name', isNumeric: false },
  { id: 'brand', label: 'Магазин', sortKey: 'brand_name', isNumeric: false },
  { id: 'onboarding_stage', label: 'Этап подключения', sortKey: 'status', isNumeric: false },
  { id: 'performance', label: 'Эффективность', sortKey: 'performance_score', isNumeric: true },
  { id: 'rating', label: 'Рейтинг', sortKey: 'rating', isNumeric: true },
  { id: 'turnover', label: 'Оборот за период', sortKey: 'gross_sales', isNumeric: true },
  { id: 'orders', label: 'Заказы', sortKey: 'orders_count', isNumeric: true },
  { id: 'problems', label: 'Проблемы', sortKey: 'warnings_active', isNumeric: true },
  { id: 'last_active', label: 'Последняя активность', sortKey: 'created_at', isNumeric: false },
];

const DEFAULT_COLUMNS_KEYS = [
  'owner',
  'brand',
  'onboarding_stage',
  'performance',
  'rating',
  'turnover',
  'orders',
  'problems',
  'last_active',
];

// Read URL parameters into initial filters state
function parseURLFilters(params: URLSearchParams): Record<string, any> {
  const f: Record<string, any> = {};
  const statuses = params.getAll('status');
  if (statuses.length > 0) f.status = statuses;
  if (params.has('storeCreated')) f.storeCreated = params.get('storeCreated') === 'true';
  if (params.has('ratingMin')) f.ratingMin = params.get('ratingMin');
  if (params.has('ratingMax')) f.ratingMax = params.get('ratingMax');
  if (params.has('reviewsMin')) f.reviewsMin = params.get('reviewsMin');
  if (params.has('reviewsOption')) f.reviewsOption = params.get('reviewsOption');
  const perfCats = params.getAll('performanceCategory');
  if (perfCats.length > 0) f.performanceCategory = perfCats;
  if (params.has('turnoverMin')) f.turnoverMin = params.get('turnoverMin');
  if (params.has('turnoverMax')) f.turnoverMax = params.get('turnoverMax');
  if (params.has('ordersMin')) f.ordersMin = params.get('ordersMin');
  if (params.has('hasWarnings')) f.hasWarnings = params.get('hasWarnings') === 'true';
  if (params.has('hasViolations')) f.hasViolations = params.get('hasViolations') === 'true';
  if (params.has('noProblems')) f.noProblems = params.get('noProblems') === 'true';
  if (params.has('hasImprovementPlan')) f.hasImprovementPlan = params.get('hasImprovementPlan') === 'true';
  return f;
}

export function AdminSellers() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  // AbortController ref for race condition prevention
  const abortControllerRef = useRef<AbortController | null>(null);

  // CANONICAL FILTERS STATE
  const [filters, setFilters] = useState<Record<string, any>>(() => parseURLFilters(searchParams));

  // Search & Sort states
  const [searchQuery, setSearchQuery] = useState(() => searchParams.get('search') || '');
  const [sortField, setSortField] = useState<SortField | undefined>(
    () => (searchParams.get('sort') as SortField) || undefined
  );
  const [sortOrder, setSortOrder] = useState<SortOrder | undefined>(
    () => (searchParams.get('direction') as SortOrder) || undefined
  );
  const page = parseInt(searchParams.get('page') || '1', 10);
  const limit = 25;

  // Data & UI states
  const [sellers, setSellers] = useState<AdminSeller[]>([]);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Active Open Popover Key ('status' | 'store' | 'rating' | 'performance' | 'problems' | 'more' | 'turnover' | 'orders')
  const [openPopover, setOpenPopover] = useState<string | null>(null);

  // Active extra filter buttons added via [+ Ещё]
  const [activeExtraFilters, setActiveExtraFilters] = useState<string[]>(() => {
    const extra: string[] = [];
    if (searchParams.has('turnoverMin') || searchParams.has('turnoverMax')) extra.push('turnover');
    if (searchParams.has('ordersMin')) extra.push('orders');
    return extra;
  });

  // Local drafts for each popover
  const [statusDraft, setStatusDraft] = useState<string[]>(() =>
    Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : []
  );
  const [storeDraft, setStoreDraft] = useState<string>(() =>
    filters.storeCreated === true ? 'created' : filters.storeCreated === false ? 'not_created' : 'any'
  );
  const [ratingDraft, setRatingDraft] = useState({
    min: filters.ratingMin || '',
    max: filters.ratingMax || '',
    reviewsMin: filters.reviewsMin || '',
    option: filters.reviewsOption || 'any',
  });
  const [perfDraft, setPerfDraft] = useState({
    min: filters.performanceScoreMin || '',
    max: filters.performanceScoreMax || '',
    categories: Array.isArray(filters.performanceCategory)
      ? filters.performanceCategory
      : filters.performanceCategory
      ? [filters.performanceCategory]
      : [],
  });
  const [probDraft, setProbDraft] = useState({
    noProblems: filters.noProblems || false,
    hasWarnings: filters.hasWarnings || false,
    hasViolations: filters.hasViolations || false,
    hasImprovementPlan: filters.hasImprovementPlan || false,
  });
  const [turnoverDraft, setTurnoverDraft] = useState({
    min: filters.turnoverMin || '',
    max: filters.turnoverMax || '',
  });

  // Sync draft states whenever main filters update
  useEffect(() => {
    setStatusDraft(Array.isArray(filters.status) ? filters.status : filters.status ? [filters.status] : []);
    setStoreDraft(filters.storeCreated === true ? 'created' : filters.storeCreated === false ? 'not_created' : 'any');
    setRatingDraft({
      min: filters.ratingMin || '',
      max: filters.ratingMax || '',
      reviewsMin: filters.reviewsMin || '',
      option: filters.reviewsOption || 'any',
    });
    setPerfDraft({
      min: filters.performanceScoreMin || '',
      max: filters.performanceScoreMax || '',
      categories: Array.isArray(filters.performanceCategory)
        ? filters.performanceCategory
        : filters.performanceCategory
        ? [filters.performanceCategory]
        : [],
    });
    setProbDraft({
      noProblems: filters.noProblems || false,
      hasWarnings: filters.hasWarnings || false,
      hasViolations: filters.hasViolations || false,
      hasImprovementPlan: filters.hasImprovementPlan || false,
    });
    setTurnoverDraft({
      min: filters.turnoverMin || '',
      max: filters.turnoverMax || '',
    });
  }, [filters]);

  // Gear Popover & Column setup with localStorage
  const [showGearPopover, setShowGearPopover] = useState(false);
  const [columns, setColumns] = useState<string[]>(() => {
    try {
      const stored = localStorage.getItem('adminSellersColumns');
      return stored ? JSON.parse(stored) : DEFAULT_COLUMNS_KEYS;
    } catch {
      return DEFAULT_COLUMNS_KEYS;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem('adminSellersColumns', JSON.stringify(columns));
    } catch {}
  }, [columns]);

  const toggleColumn = (colId: string) => {
    setColumns(prev => (prev.includes(colId) ? prev.filter(c => c !== colId) : [...prev, colId]));
  };

  const moveColumn = (index: number, direction: 'up' | 'down') => {
    const next = [...columns];
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= next.length) return;
    const temp = next[index];
    next[index] = next[targetIndex];
    next[targetIndex] = temp;
    setColumns(next);
  };

  const resetColumnsToDefault = () => {
    setColumns(DEFAULT_COLUMNS_KEYS);
  };

  // Sync state -> URL & issue API Request without page scroll jump
  const updateURLAndFetch = useCallback(
    async (
      currentFilters: Record<string, any>,
      currentSearch: string,
      currentSortField?: SortField,
      currentSortOrder?: SortOrder,
      currentPage: number = 1
    ) => {
      // Abort previous pending request
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      abortControllerRef.current = new AbortController();

      const params = new URLSearchParams();
      if (currentSearch.trim()) params.set('search', currentSearch.trim());
      if (currentSortField) params.set('sort', currentSortField);
      if (currentSortOrder) params.set('direction', currentSortOrder);
      if (currentPage > 1) params.set('page', currentPage.toString());

      Object.entries(currentFilters).forEach(([k, v]) => {
        if (v === undefined || v === '' || v === false) return;
        if (Array.isArray(v)) {
          v.forEach(item => params.append(k, item));
        } else {
          params.set(k, String(v));
        }
      });

      // Update URL silently in replace mode (preserves window.scrollY and mounted page)
      setSearchParams(params, { replace: true });

      setIsLoading(true);
      setError(null);
      try {
        const res = await getAdminSellers({
          search: currentSearch.trim() || undefined,
          sort: currentSortField,
          direction: currentSortOrder,
          page: currentPage,
          limit,
          ...currentFilters,
        });

        if (abortControllerRef.current?.signal.aborted) return;

        setSellers(res.items);
        setTotal(res.totalCount);
        setTotalPages(res.totalPages);
      } catch (err: any) {
        if (err.name === 'AbortError' || err.message === 'canceled') return;
        setError(err.message || 'Ошибка загрузки списка продавцов');
      } finally {
        setIsLoading(false);
      }
    },
    [limit, setSearchParams]
  );

  // Initial fetch on mount
  const isMountedRef = useRef(false);
  useEffect(() => {
    if (!isMountedRef.current) {
      isMountedRef.current = true;
      updateURLAndFetch(filters, searchQuery, sortField, sortOrder, page);
    }
  }, []);

  // Search Debounce (400ms)
  useEffect(() => {
    if (!isMountedRef.current) return;
    const timer = setTimeout(() => {
      updateURLAndFetch(filters, searchQuery, sortField, sortOrder, 1);
    }, 400);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Apply draft from Popover to canonical filters
  const applyFilter = (key: string, updatedDraft: any) => {
    const nextFilters = { ...filters };

    if (key === 'status') {
      if (updatedDraft.length > 0) nextFilters.status = updatedDraft;
      else delete nextFilters.status;
    } else if (key === 'store') {
      if (updatedDraft === 'created') nextFilters.storeCreated = true;
      else if (updatedDraft === 'not_created') nextFilters.storeCreated = false;
      else delete nextFilters.storeCreated;
    } else if (key === 'rating') {
      if (updatedDraft.min) nextFilters.ratingMin = updatedDraft.min;
      else delete nextFilters.ratingMin;
      if (updatedDraft.max) nextFilters.ratingMax = updatedDraft.max;
      else delete nextFilters.ratingMax;
      if (updatedDraft.reviewsMin) nextFilters.reviewsMin = updatedDraft.reviewsMin;
      else delete nextFilters.reviewsMin;
      if (updatedDraft.option && updatedDraft.option !== 'any') nextFilters.reviewsOption = updatedDraft.option;
      else delete nextFilters.reviewsOption;
    } else if (key === 'performance') {
      if (updatedDraft.categories && updatedDraft.categories.length > 0) {
        nextFilters.performanceCategory = updatedDraft.categories;
      } else delete nextFilters.performanceCategory;
    } else if (key === 'problems') {
      if (updatedDraft.noProblems) nextFilters.noProblems = true;
      else delete nextFilters.noProblems;
      if (updatedDraft.hasWarnings) nextFilters.hasWarnings = true;
      else delete nextFilters.hasWarnings;
      if (updatedDraft.hasViolations) nextFilters.hasViolations = true;
      else delete nextFilters.hasViolations;
      if (updatedDraft.hasImprovementPlan) nextFilters.hasImprovementPlan = true;
      else delete nextFilters.hasImprovementPlan;
    } else if (key === 'turnover') {
      if (updatedDraft.min) nextFilters.turnoverMin = updatedDraft.min;
      else delete nextFilters.turnoverMin;
      if (updatedDraft.max) nextFilters.turnoverMax = updatedDraft.max;
      else delete nextFilters.turnoverMax;
    }

    setFilters(nextFilters);
    setOpenPopover(null);
    updateURLAndFetch(nextFilters, searchQuery, sortField, sortOrder, 1);
  };

  // Remove single filter completely by clicking (x) on button
  const handleRemoveFilter = (keys: string[]) => {
    const nextFilters = { ...filters };
    keys.forEach(k => delete nextFilters[k]);
    setFilters(nextFilters);
    updateURLAndFetch(nextFilters, searchQuery, sortField, sortOrder, 1);
  };

  // Atomic Reset All Handler
  const handleResetAll = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    const emptyFilters = {};
    setFilters(emptyFilters);
    setSearchQuery('');
    setSortField(undefined);
    setSortOrder(undefined);
    setActiveExtraFilters([]);
    setOpenPopover(null);
    updateURLAndFetch(emptyFilters, '', undefined, undefined, 1);
  };

  // 3-Step Column Sorting Cycle Handler (Unsorted -> Primary -> Reverse -> Unsorted)
  const handleColumnSortClick = (colDef: ColumnDef) => {
    if (!colDef.sortKey) return;
    const key = colDef.sortKey;

    let nextField: SortField | undefined = sortField;
    let nextOrder: SortOrder | undefined = sortOrder;

    const primaryDirection: SortOrder = colDef.isNumeric ? 'desc' : 'asc';
    const reverseDirection: SortOrder = primaryDirection === 'desc' ? 'asc' : 'desc';

    if (sortField !== key) {
      nextField = key;
      nextOrder = primaryDirection;
    } else if (sortOrder === primaryDirection) {
      nextOrder = reverseDirection;
    } else {
      nextField = undefined;
      nextOrder = undefined;
    }

    setSortField(nextField);
    setSortOrder(nextOrder);
    updateURLAndFetch(filters, searchQuery, nextField, nextOrder, page);
  };

  const renderSortIndicator = (colDef: ColumnDef) => {
    if (!colDef.sortKey) return null;
    if (sortField !== colDef.sortKey) {
      return <span className="ml-1 text-gray-400 opacity-40 hover:opacity-100 font-mono text-xs">↕</span>;
    }
    if (sortOrder === 'asc') {
      return <span className="ml-1 text-black dark:text-white font-bold text-xs">↑</span>;
    }
    return <span className="ml-1 text-black dark:text-white font-bold text-xs">↓</span>;
  };

  // Invite Modal state
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteName, setInviteName] = useState('');
  const [tempPassword, setTempPassword] = useState<string | null>(null);

  const handleInviteSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await createAdminSeller({
        ownerEmail: inviteEmail.trim(),
        ownerName: inviteName.trim(),
      });
      if (res.temporaryPassword) {
        setTempPassword(res.temporaryPassword);
      }
      updateURLAndFetch(filters, searchQuery, sortField, sortOrder, page);
    } catch (err: any) {
      setError(err.message || 'Ошибка отправки приглашения');
    }
  };

  const hasActiveFilters = Object.keys(filters).length > 0;

  // Helper label formatters for Filter Buttons
  const getStatusButtonLabel = () => {
    if (!filters.status || (Array.isArray(filters.status) && filters.status.length === 0)) return 'Статус';
    if (Array.isArray(filters.status)) {
      if (filters.status.length === 1) return `Статус: ${formatStatus(filters.status[0])}`;
      return `Статус: ${filters.status.length} выбрано`;
    }
    return `Статус: ${formatStatus(filters.status)}`;
  };

  const getStoreButtonLabel = () => {
    if (filters.storeCreated === true) return 'Магазин: Создан';
    if (filters.storeCreated === false) return 'Магазин: Не создан';
    return 'Магазин';
  };

  const getRatingButtonLabel = () => {
    if (filters.reviewsOption === 'none') return 'Нет отзывов';
    if (filters.ratingMin) return `Рейтинг: от ${filters.ratingMin}`;
    if (filters.reviewsMin) return `Отзывы: от ${filters.reviewsMin}`;
    return 'Рейтинг';
  };

  const getPerfButtonLabel = () => {
    if (Array.isArray(filters.performanceCategory) && filters.performanceCategory.length > 0) {
      if (filters.performanceCategory.length === 1) return `Эффективность: ${formatStatus(filters.performanceCategory[0])}`;
      return `Эффективность: ${filters.performanceCategory.length} категории`;
    }
    return 'Эффективность';
  };

  const getProbButtonLabel = () => {
    if (filters.noProblems) return 'Проблемы: нет';
    const activeCount = [filters.hasWarnings, filters.hasViolations, filters.hasImprovementPlan].filter(Boolean).length;
    if (activeCount > 0) return `Проблемы: ${activeCount} типа`;
    return 'Проблемы';
  };

  const openDossier = (sellerId: string, tab: string = 'overview', period: string = '30d', section?: string, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    sessionStorage.setItem('adminSellersReturnURL', location.pathname + location.search);
    sessionStorage.setItem('adminSellersScrollY', String(window.scrollY));
    let url = `/sellers/${sellerId}?tab=${tab}&period=${period}`;
    if (section) {
      url += `&section=${section}`;
    }
    navigate(url);
  };

  useEffect(() => {
    const savedScroll = sessionStorage.getItem('adminSellersScrollY');
    if (savedScroll && !isLoading) {
      setTimeout(() => {
        window.scrollTo(0, parseInt(savedScroll, 10));
        sessionStorage.removeItem('adminSellersScrollY');
      }, 50);
    }
  }, [isLoading]);

  return (
    <div className="space-y-6">
      {/* Top Header Area */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center space-x-3">
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Продавцы</h1>
            <span className="bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 px-3 py-1 rounded-full text-xs font-semibold">
              {total}
            </span>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Управление продавцами, магазинами и доступом
          </p>
        </div>

        <PermissionGuard permission="sellers.create_access">
          <button
            onClick={() => {
              setInviteEmail('');
              setInviteName('');
              setTempPassword(null);
              setShowInviteModal(true);
            }}
            className="px-4 py-2.5 bg-black dark:bg-white text-white dark:text-black font-medium text-sm rounded-xl hover:bg-gray-800 dark:hover:bg-gray-100 transition-colors inline-flex items-center shadow-sm"
          >
            <Plus className="h-4 w-4 mr-2" /> Пригласить продавца
          </button>
        </PermissionGuard>
      </div>

      {error && (
        <div className="bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 p-4 rounded-xl text-sm">
          {error}
        </div>
      )}

      {/* SEARCH BAR (Full Width) */}
      <div className="relative">
        <Search className="absolute left-4 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
        <input
          type="text"
          placeholder="Поиск по магазину, email, владельцу..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full pl-11 pr-4 py-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl focus:outline-none focus:ring-2 focus:ring-black dark:focus:ring-white transition-all shadow-sm text-sm text-gray-900 dark:text-white"
        />
      </div>

      {/* HORIZONTAL FILTER BUTTON BAR (Directly under search, matching screenshot reference) */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          
          {/* FILTER 1: СТАТУС */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setOpenPopover(openPopover === 'status' ? null : 'status')}
              className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-gray-800 border rounded-xl text-xs font-medium transition-colors ${
                filters.status
                  ? 'border-gray-400 dark:border-gray-500 font-semibold text-gray-900 dark:text-white shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:border-gray-300'
              }`}
            >
              <span>{getStatusButtonLabel()}</span>
              {filters.status ? (
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveFilter(['status']);
                  }}
                  className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full ml-1"
                >
                  <X className="w-3 h-3 text-gray-400 hover:text-black" />
                </span>
              ) : openPopover === 'status' ? (
                <ChevronUp className="w-3.5 h-3.5 text-gray-400" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
              )}
            </button>

            <FilterPopover
              isOpen={openPopover === 'status'}
              onClose={() => setOpenPopover(null)}
              onReset={() => setStatusDraft([])}
              onApply={() => applyFilter('status', statusDraft)}
              widthClass="w-56"
            >
              <div className="space-y-2">
                {[
                  { id: 'active', label: 'Активен' },
                  { id: 'pending_setup', label: 'Ожидает настройки' },
                  { id: 'pending_review', label: 'Ожидает проверки' },
                  { id: 'blocked', label: 'Заблокирован' },
                  { id: 'archived', label: 'В архиве' },
                ].map(st => {
                  const isChecked = statusDraft.includes(st.id);
                  return (
                    <label key={st.id} className="flex items-center space-x-2.5 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={isChecked}
                        onChange={() => {
                          if (isChecked) setStatusDraft(statusDraft.filter(i => i !== st.id));
                          else setStatusDraft([...statusDraft, st.id]);
                        }}
                        className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600"
                      />
                      <span className="text-gray-800 dark:text-gray-200">{st.label}</span>
                    </label>
                  );
                })}
              </div>
            </FilterPopover>
          </div>

          {/* FILTER 2: МАГАЗИН */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setOpenPopover(openPopover === 'store' ? null : 'store')}
              className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-gray-800 border rounded-xl text-xs font-medium transition-colors ${
                filters.storeCreated !== undefined
                  ? 'border-gray-400 dark:border-gray-500 font-semibold text-gray-900 dark:text-white shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:border-gray-300'
              }`}
            >
              <span>{getStoreButtonLabel()}</span>
              {filters.storeCreated !== undefined ? (
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveFilter(['storeCreated']);
                  }}
                  className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full ml-1"
                >
                  <X className="w-3 h-3 text-gray-400 hover:text-black" />
                </span>
              ) : openPopover === 'store' ? (
                <ChevronUp className="w-3.5 h-3.5 text-gray-400" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
              )}
            </button>

            <FilterPopover
              isOpen={openPopover === 'store'}
              onClose={() => setOpenPopover(null)}
              onReset={() => setStoreDraft('any')}
              onApply={() => applyFilter('store', storeDraft)}
              widthClass="w-60"
            >
              <div className="space-y-2">
                {[
                  { id: 'any', label: 'Любой' },
                  { id: 'created', label: 'Создан' },
                  { id: 'not_created', label: 'Не создан' },
                  { id: 'incomplete', label: 'Профиль заполнен не полностью' },
                ].map(opt => (
                  <label key={opt.id} className="flex items-center space-x-2.5 cursor-pointer">
                    <input
                      type="radio"
                      name="storeRadio"
                      checked={storeDraft === opt.id}
                      onChange={() => setStoreDraft(opt.id)}
                      className="text-black focus:ring-black dark:border-gray-600"
                    />
                    <span className="text-gray-800 dark:text-gray-200">{opt.label}</span>
                  </label>
                ))}
              </div>
            </FilterPopover>
          </div>

          {/* FILTER 3: РЕЙТИНГ */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setOpenPopover(openPopover === 'rating' ? null : 'rating')}
              className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-gray-800 border rounded-xl text-xs font-medium transition-colors ${
                filters.ratingMin || filters.reviewsMin || filters.reviewsOption
                  ? 'border-gray-400 dark:border-gray-500 font-semibold text-gray-900 dark:text-white shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:border-gray-300'
              }`}
            >
              <span>{getRatingButtonLabel()}</span>
              {filters.ratingMin || filters.reviewsMin || filters.reviewsOption ? (
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveFilter(['ratingMin', 'ratingMax', 'reviewsMin', 'reviewsOption']);
                  }}
                  className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full ml-1"
                >
                  <X className="w-3 h-3 text-gray-400 hover:text-black" />
                </span>
              ) : openPopover === 'rating' ? (
                <ChevronUp className="w-3.5 h-3.5 text-gray-400" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
              )}
            </button>

            <FilterPopover
              isOpen={openPopover === 'rating'}
              onClose={() => setOpenPopover(null)}
              onReset={() => setRatingDraft({ min: '', max: '', reviewsMin: '', option: 'any' })}
              onApply={() => applyFilter('rating', ratingDraft)}
              widthClass="w-64"
            >
              <div className="space-y-3">
                <div className="flex space-x-2">
                  <div className="flex-1">
                    <label className="block text-[10px] text-gray-400 mb-1">Рейтинг от</label>
                    <input
                      type="number"
                      step="0.1"
                      placeholder="4.0"
                      value={ratingDraft.min}
                      onChange={(e) => setRatingDraft({ ...ratingDraft, min: e.target.value })}
                      className="w-full p-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                    />
                  </div>
                  <div className="flex-1">
                    <label className="block text-[10px] text-gray-400 mb-1">до</label>
                    <input
                      type="number"
                      step="0.1"
                      placeholder="5.0"
                      value={ratingDraft.max}
                      onChange={(e) => setRatingDraft({ ...ratingDraft, max: e.target.value })}
                      className="w-full p-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-[10px] text-gray-400 mb-1">Минимум отзывов</label>
                  <input
                    type="number"
                    placeholder="10"
                    value={ratingDraft.reviewsMin}
                    onChange={(e) => setRatingDraft({ ...ratingDraft, reviewsMin: e.target.value })}
                    className="w-full p-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                  />
                </div>

                <div className="pt-2 border-t border-gray-100 dark:border-gray-700 space-y-1.5">
                  {[
                    { id: 'any', label: 'Любые отзывы' },
                    { id: 'has', label: 'Есть отзывы' },
                    { id: 'none', label: 'Нет отзывов' },
                  ].map(opt => (
                    <label key={opt.id} className="flex items-center space-x-2 cursor-pointer">
                      <input
                        type="radio"
                        name="reviewsOptionRadio"
                        checked={ratingDraft.option === opt.id}
                        onChange={() => setRatingDraft({ ...ratingDraft, option: opt.id })}
                        className="text-black focus:ring-black dark:border-gray-600"
                      />
                      <span className="text-gray-800 dark:text-gray-200">{opt.label}</span>
                    </label>
                  ))}
                </div>
              </div>
            </FilterPopover>
          </div>

          {/* FILTER 4: ЭФФЕКТИВНОСТЬ */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setOpenPopover(openPopover === 'performance' ? null : 'performance')}
              className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-gray-800 border rounded-xl text-xs font-medium transition-colors ${
                filters.performanceCategory
                  ? 'border-gray-400 dark:border-gray-500 font-semibold text-gray-900 dark:text-white shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:border-gray-300'
              }`}
            >
              <span>{getPerfButtonLabel()}</span>
              {filters.performanceCategory ? (
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveFilter(['performanceCategory']);
                  }}
                  className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full ml-1"
                >
                  <X className="w-3 h-3 text-gray-400 hover:text-black" />
                </span>
              ) : openPopover === 'performance' ? (
                <ChevronUp className="w-3.5 h-3.5 text-gray-400" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
              )}
            </button>

            <FilterPopover
              isOpen={openPopover === 'performance'}
              onClose={() => setOpenPopover(null)}
              onReset={() => setPerfDraft({ min: '', max: '', categories: [] })}
              onApply={() => applyFilter('performance', perfDraft)}
              widthClass="w-64"
            >
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <label className="block text-[10px] text-gray-400 mb-1">Категории эффективности:</label>
                  {[
                    { id: 'high', label: 'Высокая' },
                    { id: 'stable', label: 'Стабильная' },
                    { id: 'needs_attention', label: 'Требует внимания' },
                    { id: 'low', label: 'Низкая' },
                    { id: 'no_data', label: 'Недостаточно данных' },
                  ].map(cat => {
                    const isChecked = perfDraft.categories.includes(cat.id);
                    return (
                      <label key={cat.id} className="flex items-center space-x-2.5 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => {
                            if (isChecked) {
                              setPerfDraft({
                                ...perfDraft,
                                categories: perfDraft.categories.filter(c => c !== cat.id),
                              });
                            } else {
                              setPerfDraft({
                                ...perfDraft,
                                categories: [...perfDraft.categories, cat.id],
                              });
                            }
                          }}
                          className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600"
                        />
                        <span className="text-gray-800 dark:text-gray-200">{cat.label}</span>
                      </label>
                    );
                  })}
                </div>
              </div>
            </FilterPopover>
          </div>

          {/* FILTER 5: ПРОБЛЕМЫ */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setOpenPopover(openPopover === 'problems' ? null : 'problems')}
              className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-gray-800 border rounded-xl text-xs font-medium transition-colors ${
                filters.noProblems || filters.hasWarnings || filters.hasViolations || filters.hasImprovementPlan
                  ? 'border-gray-400 dark:border-gray-500 font-semibold text-gray-900 dark:text-white shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:border-gray-300'
              }`}
            >
              <span>{getProbButtonLabel()}</span>
              {filters.noProblems || filters.hasWarnings || filters.hasViolations || filters.hasImprovementPlan ? (
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    handleRemoveFilter(['noProblems', 'hasWarnings', 'hasViolations', 'hasImprovementPlan']);
                  }}
                  className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full ml-1"
                >
                  <X className="w-3 h-3 text-gray-400 hover:text-black" />
                </span>
              ) : openPopover === 'problems' ? (
                <ChevronUp className="w-3.5 h-3.5 text-gray-400" />
              ) : (
                <ChevronDown className="w-3.5 h-3.5 text-gray-400" />
              )}
            </button>

            <FilterPopover
              isOpen={openPopover === 'problems'}
              onClose={() => setOpenPopover(null)}
              onReset={() => setProbDraft({ noProblems: false, hasWarnings: false, hasViolations: false, hasImprovementPlan: false })}
              onApply={() => applyFilter('problems', probDraft)}
              widthClass="w-64"
            >
              <div className="space-y-2">
                <label className="flex items-center space-x-2.5 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={probDraft.noProblems}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setProbDraft({ noProblems: true, hasWarnings: false, hasViolations: false, hasImprovementPlan: false });
                      } else {
                        setProbDraft({ ...probDraft, noProblems: false });
                      }
                    }}
                    className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600"
                  />
                  <span className="font-semibold text-gray-900 dark:text-white">Без проблем</span>
                </label>

                <div className="pt-2 border-t border-gray-100 dark:border-gray-700 space-y-2">
                  <label className="flex items-center space-x-2.5 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={probDraft.hasWarnings}
                      disabled={probDraft.noProblems}
                      onChange={(e) => setProbDraft({ ...probDraft, hasWarnings: e.target.checked, noProblems: false })}
                      className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600 disabled:opacity-40"
                    />
                    <span className="text-gray-800 dark:text-gray-200">Есть предупреждения</span>
                  </label>

                  <label className="flex items-center space-x-2.5 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={probDraft.hasViolations}
                      disabled={probDraft.noProblems}
                      onChange={(e) => setProbDraft({ ...probDraft, hasViolations: e.target.checked, noProblems: false })}
                      className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600 disabled:opacity-40"
                    />
                    <span className="text-gray-800 dark:text-gray-200">Есть нарушения</span>
                  </label>

                  <label className="flex items-center space-x-2.5 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={probDraft.hasImprovementPlan}
                      disabled={probDraft.noProblems}
                      onChange={(e) => setProbDraft({ ...probDraft, hasImprovementPlan: e.target.checked, noProblems: false })}
                      className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600 disabled:opacity-40"
                    />
                    <span className="text-gray-800 dark:text-gray-200">Есть план улучшения</span>
                  </label>
                </div>
              </div>
            </FilterPopover>
          </div>

          {/* DYNAMIC EXTRA FILTERS (Added via + Ещё) */}
          {activeExtraFilters.includes('turnover') && (
            <div className="relative">
              <button
                type="button"
                onClick={() => setOpenPopover(openPopover === 'turnover' ? null : 'turnover')}
                className={`flex items-center space-x-1.5 px-3 py-1.5 bg-white dark:bg-gray-800 border rounded-xl text-xs font-medium transition-colors ${
                  filters.turnoverMin || filters.turnoverMax
                    ? 'border-gray-400 dark:border-gray-500 font-semibold text-gray-900 dark:text-white shadow-sm'
                    : 'border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:border-gray-300'
                }`}
              >
                <span>
                  {filters.turnoverMin
                    ? `Оборот: от ${filters.turnoverMin} ₽`
                    : 'Оборот'}
                </span>
                <span
                  onClick={(e) => {
                    e.stopPropagation();
                    setActiveExtraFilters(activeExtraFilters.filter(f => f !== 'turnover'));
                    handleRemoveFilter(['turnoverMin', 'turnoverMax']);
                  }}
                  className="p-0.5 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-full ml-1"
                >
                  <X className="w-3 h-3 text-gray-400 hover:text-black" />
                </span>
              </button>

              <FilterPopover
                isOpen={openPopover === 'turnover'}
                onClose={() => setOpenPopover(null)}
                onReset={() => setTurnoverDraft({ min: '', max: '' })}
                onApply={() => applyFilter('turnover', turnoverDraft)}
                widthClass="w-60"
              >
                <div className="space-y-2">
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Оборот от (₽)</label>
                    <input
                      type="number"
                      placeholder="100000"
                      value={turnoverDraft.min}
                      onChange={(e) => setTurnoverDraft({ ...turnoverDraft, min: e.target.value })}
                      className="w-full p-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">до (₽)</label>
                    <input
                      type="number"
                      placeholder="1000000"
                      value={turnoverDraft.max}
                      onChange={(e) => setTurnoverDraft({ ...turnoverDraft, max: e.target.value })}
                      className="w-full p-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                    />
                  </div>
                </div>
              </FilterPopover>
            </div>
          )}

          {/* BUTTON: [+ Ещё] */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setOpenPopover(openPopover === 'more' ? null : 'more')}
              className="flex items-center space-x-1 px-3 py-1.5 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:border-gray-300 rounded-xl text-xs font-medium text-gray-700 dark:text-gray-300 transition-colors shadow-sm"
            >
              <span>+ Ещё</span>
            </button>

            {openPopover === 'more' && (
              <>
                <div
                  className="fixed inset-0 z-30"
                  onClick={() => setOpenPopover(null)}
                />
                <div className="absolute left-0 top-full mt-2 w-60 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl shadow-xl z-50 p-2 text-xs animate-in fade-in zoom-in-95">
                  <div className="px-2 py-1 text-[10px] font-bold text-gray-400 uppercase tracking-wider">
                    Продажи
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      if (!activeExtraFilters.includes('turnover')) {
                        setActiveExtraFilters([...activeExtraFilters, 'turnover']);
                      }
                      setOpenPopover('turnover');
                    }}
                    className="w-full text-left px-2.5 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700 rounded-lg text-gray-800 dark:text-gray-200 font-medium"
                  >
                    Оборот за период
                  </button>

                  <div className="px-2 py-1 text-[10px] font-bold text-gray-400 uppercase tracking-wider mt-2">
                    Активность
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      setOpenPopover(null);
                    }}
                    className="w-full text-left px-2.5 py-1.5 hover:bg-gray-50 dark:hover:bg-gray-700 rounded-lg text-gray-800 dark:text-gray-200 font-medium"
                  >
                    Последняя активность
                  </button>
                </div>
              </>
            )}
          </div>

        </div>

        {/* BUTTON: [Сбросить все ×] (Far Right, visible only when active filters exist) */}
        {hasActiveFilters && (
          <button
            type="button"
            onClick={handleResetAll}
            className="flex items-center space-x-1 text-xs font-semibold text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white px-2 py-1.5 transition-colors"
          >
            <span>Сбросить все</span>
            <X className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {/* Sellers List Table Container */}
      <div className="bg-white dark:bg-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden flex flex-col">
        
        {/* Top Header Bar above Table with Counters & Gear Popover */}
        <div className="p-4 border-b border-gray-100 dark:border-gray-700 flex items-center justify-between">
          <div className="text-xs font-medium text-gray-600 dark:text-gray-400 space-x-3">
            <span>Найдено: <strong className="text-gray-900 dark:text-white">{total}</strong> продавцов</span>
            <span>•</span>
            <span>Показано: <strong className="text-gray-900 dark:text-white">{sellers.length} из {total}</strong></span>
          </div>

          {/* GEAR ICON BUTTON & POPOVER (Unchanged structure) */}
          <div className="relative">
            <button
              aria-label="Настроить таблицу продавцов"
              onClick={() => setShowGearPopover(!showGearPopover)}
              className="p-2 bg-gray-50 dark:bg-gray-700/50 hover:bg-gray-100 dark:hover:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-xl transition-colors text-gray-600 dark:text-gray-300"
            >
              <Settings2 className="w-4 h-4" />
            </button>

            {showGearPopover && (
              <>
                <div
                  className="fixed inset-0 z-30"
                  onClick={() => setShowGearPopover(false)}
                />
                <div className="absolute right-0 top-full mt-2 w-64 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl shadow-xl z-40 p-3 text-xs overflow-hidden animate-in fade-in slide-in-from-top-2">
                  <div className="flex items-center justify-between pb-2 border-b border-gray-100 dark:border-gray-700 mb-2">
                    <span className="font-bold text-gray-900 dark:text-white">Столбцы таблицы</span>
                    <button
                      onClick={resetColumnsToDefault}
                      className="text-[11px] font-semibold text-blue-600 hover:underline"
                    >
                      Вернуть стандартный вид
                    </button>
                  </div>

                  <div className="space-y-1 max-h-60 overflow-y-auto">
                    {ALL_COLUMNS.map(col => {
                      const isVisible = columns.includes(col.id);
                      const orderIndex = columns.indexOf(col.id);
                      return (
                        <div
                          key={col.id}
                          className="flex items-center justify-between p-1.5 hover:bg-gray-50 dark:hover:bg-gray-700/50 rounded-lg text-gray-700 dark:text-gray-300"
                        >
                          <label className="flex items-center space-x-2 cursor-pointer flex-1">
                            <input
                              type="checkbox"
                              checked={isVisible}
                              onChange={() => toggleColumn(col.id)}
                              className="rounded border-gray-300 text-black focus:ring-black dark:border-gray-600"
                            />
                            <span>{col.label}</span>
                          </label>

                          {isVisible && (
                            <div className="flex items-center space-x-1">
                              <button
                                disabled={orderIndex === 0}
                                onClick={() => moveColumn(orderIndex, 'up')}
                                className="p-1 text-gray-400 hover:text-gray-700 disabled:opacity-30"
                              >
                                <ChevronUp className="w-3 h-3" />
                              </button>
                              <button
                                disabled={orderIndex === columns.length - 1}
                                onClick={() => moveColumn(orderIndex, 'down')}
                                className="p-1 text-gray-400 hover:text-gray-700 disabled:opacity-30"
                              >
                                <ChevronDown className="w-3 h-3" />
                              </button>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              </>
            )}
          </div>
        </div>

        {isLoading ? (
          <div className="p-8 text-center text-gray-500">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-black dark:border-white mx-auto mb-2"></div>
            Загрузка списка...
          </div>
        ) : sellers.length === 0 ? (
          <div className="p-12 text-center text-gray-500">
            <StoreIcon className="h-10 w-10 mx-auto text-gray-400 mb-3" />
            <p className="text-base font-medium">Продавцы не найдены</p>
            <p className="text-sm mt-1">Попробуйте изменить параметры поиска или фильтры.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 dark:bg-gray-900/50 border-b border-gray-200 dark:border-gray-700 text-xs text-gray-500 dark:text-gray-400 uppercase font-semibold">
                <tr>
                  {columns.map(colId => {
                    const colDef = ALL_COLUMNS.find(c => c.id === colId);
                    if (!colDef) return null;
                    return (
                      <th
                        key={colDef.id}
                        onClick={() => handleColumnSortClick(colDef)}
                        className={`py-3 px-4 select-none ${colDef.sortKey ? 'cursor-pointer hover:text-black dark:hover:text-white transition-colors' : ''}`}
                      >
                        <div className="flex items-center space-x-1">
                          <span>{colDef.label}</span>
                          {renderSortIndicator(colDef)}
                        </div>
                      </th>
                    );
                  })}
                  <th className="py-3 px-4 text-right">Действие</th>
                </tr>
              </thead>

              <tbody className="divide-y divide-gray-100 dark:divide-gray-700/60 text-xs">
                {sellers.map((s) => (
                  <tr
                    key={s.id}
                    onClick={() => openDossier(s.id, 'overview', '30d')}
                    className="hover:bg-gray-50 dark:hover:bg-gray-700/40 cursor-pointer transition-colors"
                  >
                    {columns.map(colId => {
                      if (colId === 'owner') {
                        return (
                          <td key="owner" className="py-3.5 px-4" onClick={(e) => openDossier(s.id, 'access', '30d', undefined, e)}>
                            <p className="font-semibold text-gray-900 dark:text-white hover:text-blue-600 transition-colors">{s.ownerName}</p>
                            <p className="text-gray-400">{s.ownerEmail}</p>
                          </td>
                        );
                      }
                      if (colId === 'brand') {
                        return (
                          <td key="brand" className="py-3.5 px-4 font-medium text-gray-900 dark:text-white" onClick={(e) => openDossier(s.id, 'access', '30d', undefined, e)}>
                            {s.brandName || <span className="text-gray-400 italic font-normal">Без магазина</span>}
                          </td>
                        );
                      }
                      if (colId === 'onboarding_stage') {
                        return (
                          <td key="onboarding" className="py-3.5 px-4" onClick={(e) => openDossier(s.id, 'access', '30d', undefined, e)}>
                            <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-[11px] font-medium ${getStatusBadgeClass(s.status)}`}>
                              {formatStatus(s.status)}
                            </span>
                          </td>
                        );
                      }
                      if (colId === 'performance') {
                        return (
                          <td key="perf" className="py-3.5 px-4" onClick={(e) => openDossier(s.id, 'quality', '30d', undefined, e)}>
                            {s.performanceScore !== undefined && s.performanceScore !== null ? (
                              <span className="font-bold text-gray-900 dark:text-white hover:text-blue-600 transition-colors">{s.performanceScore}%</span>
                            ) : (
                              <span className="text-gray-400">Недостаточно данных</span>
                            )}
                          </td>
                        );
                      }
                      if (colId === 'rating') {
                        return (
                          <td key="rating" className="py-3.5 px-4 font-medium text-gray-900 dark:text-white" onClick={(e) => openDossier(s.id, 'quality', '30d', 'reviews', e)}>
                            {s.reviewsCount && s.reviewsCount > 0 ? (
                              <span className="hover:text-blue-600 transition-colors">{s.averageRating.toFixed(1)} ★</span>
                            ) : (
                              <span className="text-gray-400">Нет отзывов</span>
                            )}
                          </td>
                        );
                      }
                      if (colId === 'turnover') {
                        return (
                          <td key="turnover" className="py-3.5 px-4 font-semibold text-gray-900 dark:text-white" onClick={(e) => openDossier(s.id, 'sales', '30d', undefined, e)}>
                            <span className="hover:text-blue-600 transition-colors">{(s.grossSales30d ? (s.grossSales30d / 100).toLocaleString('ru-RU') : 0)} ₽</span>
                          </td>
                        );
                      }
                      if (colId === 'orders') {
                        return (
                          <td key="orders" className="py-3.5 px-4 text-gray-700 dark:text-gray-300" onClick={(e) => openDossier(s.id, 'sales', '30d', undefined, e)}>
                            <span className="hover:text-blue-600 font-medium transition-colors">{s.ordersCount30d || 0}</span>
                          </td>
                        );
                      }
                      if (colId === 'problems') {
                        const problemsCount = s.warningsActive || 0;
                        return (
                          <td key="problems" className="py-3.5 px-4" onClick={(e) => openDossier(s.id, 'control', '30d', undefined, e)}>
                            {problemsCount > 0 ? (
                              <span className="inline-flex items-center space-x-1 text-amber-600 bg-amber-50 dark:bg-amber-900/20 px-2 py-0.5 rounded-md font-semibold hover:bg-amber-100 transition-colors">
                                <AlertTriangle className="w-3.5 h-3.5" />
                                <span>{problemsCount} проблемы</span>
                              </span>
                            ) : (
                              <span className="text-gray-400 font-normal">Нет проблем</span>
                            )}
                          </td>
                        );
                      }
                      if (colId === 'last_active') {
                        return (
                          <td key="active" className="py-3.5 px-4 text-gray-500" onClick={(e) => openDossier(s.id, 'access', '30d', 'activity', e)}>
                            {new Date(s.createdAt).toLocaleDateString('ru-RU')}
                          </td>
                        );
                      }
                      return null;
                    })}
                    <td className="py-3.5 px-4 text-right" onClick={(e) => openDossier(s.id, 'overview', '30d', undefined, e)}>
                      <button className="text-blue-600 font-bold hover:underline">Досье →</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="p-4 border-t border-gray-100 dark:border-gray-700 flex items-center justify-between bg-white dark:bg-gray-800">
            <span className="text-xs text-gray-500">
              Страница {page} из {totalPages}
            </span>
            <div className="flex space-x-2">
              <button
                disabled={page <= 1}
                onClick={() => updateURLAndFetch(filters, searchQuery, sortField, sortOrder, page - 1)}
                className="p-2 border border-gray-200 dark:border-gray-700 rounded-xl hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-30"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                disabled={page >= totalPages}
                onClick={() => updateURLAndFetch(filters, searchQuery, sortField, sortOrder, page + 1)}
                className="p-2 border border-gray-200 dark:border-gray-700 rounded-xl hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-30"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* MODAL: INVITE SELLER */}
      {showInviteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="bg-white dark:bg-gray-800 rounded-3xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h3 className="font-bold text-lg text-gray-900 dark:text-white">Пригласить продавца</h3>
            {tempPassword ? (
              <div className="space-y-3 text-xs">
                <p className="text-gray-600 dark:text-gray-300">Продавец создан. Передайте ему временный пароль:</p>
                <div className="p-3 bg-gray-100 dark:bg-gray-900 rounded-xl font-mono text-center text-sm font-bold select-all">
                  {tempPassword}
                </div>
                <button
                  onClick={() => setShowInviteModal(false)}
                  className="w-full py-2.5 bg-black dark:bg-white text-white dark:text-black font-bold rounded-xl"
                >
                  Закрыть
                </button>
              </div>
            ) : (
              <form onSubmit={handleInviteSubmit} className="space-y-3 text-xs">
                <div>
                  <label className="block text-gray-700 dark:text-gray-300 font-semibold mb-1">ФИО владельца</label>
                  <input
                    type="text"
                    required
                    value={inviteName}
                    onChange={(e) => setInviteName(e.target.value)}
                    placeholder="Иван Иванов"
                    className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                  />
                </div>
                <div>
                  <label className="block text-gray-700 dark:text-gray-300 font-semibold mb-1">Email владельца</label>
                  <input
                    type="email"
                    required
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    placeholder="seller@example.com"
                    className="w-full p-2.5 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-xl"
                  />
                </div>

                <div className="flex justify-end space-x-2 pt-2">
                  <button
                    type="button"
                    onClick={() => setShowInviteModal(false)}
                    className="px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200 font-bold rounded-xl"
                  >
                    Отмена
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-2 bg-black dark:bg-white text-white dark:text-black font-bold rounded-xl"
                  >
                    Отправить
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
