import { useState, useMemo } from 'react';
import { executeAdminMarketingQuery } from '../api/marketing';
import type { QueryRequest, QueryResponse, QueryColumn, QueryFilter, QuerySort } from '@zamk/api-client/src/types';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { number, money } from '../components/marketing/marketingPresentation';
import { formatSource } from '../utils/sourceFormatter';


import { Save, FolderOpen, AlertCircle, Download } from 'lucide-react';
import { AdminMarketingSavedQueriesSidebar } from '../components/marketing/AdminMarketingSavedQueriesSidebar';
import { AdminMarketingSaveQueryModal } from '../components/marketing/AdminMarketingSaveQueryModal';
import { updateAdminMarketingSavedQuery, exportAdminMarketingQuery } from '@zamk/api-client/src/admin';
import type { SavedQuery } from '@zamk/api-client/src/types';
import { deepEqual } from '../utils/deepEqual';
import { normalizeQueryRequest } from '../utils/normalizeQueryRequest';
import { mapSavedQueryError } from '../utils/savedQueryError';

export const DIMENSIONS = [
  { key: 'day', label: 'День' },
  { key: 'source', label: 'Источник' },
  { key: 'campaign', label: 'Кампания' },
  { key: 'product', label: 'Товар' },
  { key: 'designer', label: 'Дизайнер' },
  { key: 'category', label: 'Категория' },
];

export const COLUMN_LABELS: Record<string, string> = {
  day: 'День',
  source: 'Источник',
  campaign: 'Кампания',
  product: 'Товар',
  productName: 'Товар',
  productCode: 'Артикул',
  designer: 'Дизайнер',
  designerName: 'Дизайнер',
  category: 'Категория',
  categoryName: 'Категория',
};

export const METRICS = [
  { key: 'sessions', label: 'Сессии', type: 'number' },
  { key: 'orders', label: 'Заказы', type: 'number' },
  { key: 'sold_units', label: 'Продано шт.', type: 'number' },
  { key: 'revenue', label: 'Выручка', type: 'money' },
  { key: 'product_views', label: 'Просмотры', type: 'number' },
  { key: 'favorites', label: 'В избранное', type: 'number' },
  { key: 'add_to_cart', label: 'В корзину', type: 'number' },
  { key: 'conversion', label: 'Конверсия', type: 'percentage' },
  { key: 'returns', label: 'Возвраты', type: 'number' },
  { key: 'returned_units', label: 'Возвращено шт.', type: 'number' },
];

const FILTER_OPERATORS = [
  { key: 'eq', label: '= Равно' },
  { key: 'neq', label: '≠ Не равно' },
  { key: 'in', label: 'В списке' },
  { key: 'contains', label: 'Содержит' },
];

const LIMIT_OPTIONS = [50, 100, 250, 500, 1000];

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export interface FilterItem {
  id: string;
  dimension: string;
  operator: string;
  value: string;
}

export interface SortItem {
  id: string;
  field: string;
  direction: 'asc' | 'desc';
}

export function formatPercentage(value: unknown): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'number' && Number.isFinite(value)) {
    if (value === 0) return '0%';
    if (Math.abs(value) <= 1) {
      return `${(value * 100).toFixed(2)}%`;
    }
    return `${value.toFixed(2)}%`;
  }
  return '—';
}

export function mapErrorMessage(err: any): string {
  const code = err?.code || err?.error || '';
  switch (code) {
    case 'unsupported_combination':
      return 'Этот показатель нельзя использовать с выбранными разрезами.';
    case 'invalid_period':
      return 'Некорректный период запроса.';
    case 'period_too_long':
      return 'Период запроса не может превышать 366 дней.';
    case 'missing_metrics':
      return 'Выберите хотя бы один показатель.';
    case 'too_many_dimensions':
      return 'Максимум 2 разреза.';
    case 'invalid_dimension':
      return 'Некорректный разрез.';
    case 'invalid_metric':
      return 'Некорректный показатель.';
    case 'too_many_metrics':
      return 'Максимум 8 показателей.';
    case 'invalid_filter_dimension':
      return 'Некорректный разрез для фильтра.';
    case 'unsupported_filter_dimension':
      return 'Фильтр возможен только по выбранным разрезам.';
    case 'invalid_filter_operator':
      return 'Некорректный оператор фильтра.';
    case 'empty_filter_values':
      return 'Значение фильтра не может быть пустым.';
    case 'too_many_filters':
      return 'Максимум 10 фильтров.';
    case 'too_many_sort_keys':
      return 'Максимум 2 поля сортировки.';
    case 'invalid_sort_field':
      return 'Сортировка возможна только по выбранным разрезам или показателям.';
    case 'invalid_sort_direction':
      return 'Направление сортировки должно быть asc или desc.';
    case 'invalid_limit':
      return 'Лимит должен быть от 1 до 1000.';
    default:
      return 'Произошла ошибка при выполнении запроса. Попробуйте позже.';
  }
}

export function mapExportErrorMessage(err: any): string {
  const status = err?.status || err?.statusCode;
  if (status === 400) {
    return 'Не удалось экспортировать отчёт. Проверьте параметры запроса.';
  }
  if (status === 403) {
    return 'Недостаточно прав для экспорта отчёта.';
  }
  if (status === 404) {
    return 'Не удалось экспортировать отчёт.';
  }
  return 'Не удалось экспортировать отчёт. Попробуйте ещё раз.';
}

export function AdminMarketingQueryBuilder() {
  const { period, range, select } = useMarketingRange();

  const [dimensions, setDimensions] = useState<string[]>(['source']);
  const [metrics, setMetrics] = useState<string[]>(['revenue', 'orders']);
  const [filters, setFilters] = useState<FilterItem[]>([]);
  const [sort, setSort] = useState<SortItem[]>([]);
  const [limit, setLimit] = useState<number>(100);

  const [dimFeedback, setDimFeedback] = useState<string | null>(null);
  const [metricFeedback, setMetricFeedback] = useState<string | null>(null);
  const [sortFeedback, setSortFeedback] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<QueryResponse | null>(null);

  // Saved Queries State
  const [activeSavedQuery, setActiveSavedQuery] = useState<SavedQuery | null>(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [saveModalMode, setSaveModalMode] = useState<'create' | 'rename' | null>(null);
  const [queryToRename, setQueryToRename] = useState<SavedQuery | undefined>(undefined);
  const [isSavingChanges, setIsSavingChanges] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  // Export State
  const [isExportMenuOpen, setIsExportMenuOpen] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const buildCurrentQueryRequest = (): QueryRequest => {
    const queryFilters: QueryFilter[] = filters
      .map(f => {
        const trimmed = f.value.trim();
        const values = f.operator === 'in'
          ? trimmed.split(',').map(v => v.trim()).filter(Boolean)
          : (trimmed ? [trimmed] : []);
        return {
          dimension: f.dimension,
          operator: f.operator,
          values,
        };
      })
      .filter(f => f.values.length > 0);

    const querySort: QuerySort[] = sort
      .filter(s => Boolean(s.field))
      .map(s => ({
        field: s.field,
        direction: s.direction,
      }));

    return {
      version: 1,
      period: {
        from: typeof range.from === 'string' ? range.from : new Date(range.from).toISOString(),
        to: typeof range.to === 'string' ? range.to : new Date(range.to).toISOString(),
      },
      dimensions,
      metrics,
      ...(queryFilters.length > 0 ? { filters: queryFilters } : {}),
      ...(querySort.length > 0 ? { sort: querySort } : {}),
      limit,
    };
  };

  const currentQuerySpec = buildCurrentQueryRequest();
  const isDirty = activeSavedQuery
    ? !deepEqual(normalizeQueryRequest(activeSavedQuery.querySpec), normalizeQueryRequest(currentQuerySpec))
    : false;

  const handleLoadSavedQuery = (query: SavedQuery) => {
    setActiveSavedQuery(query);
    const spec = query.querySpec;
    setDimensions(spec.dimensions ? [...spec.dimensions] : []);
    setMetrics(spec.metrics ? [...spec.metrics] : []);
    // Preserves M5 default limit of 100
    setLimit(spec.limit ?? 100);

    // Period: absolute [from, to)
    if (spec.period) {
      select('custom', { from: spec.period.from, to: spec.period.to });
    }

    // Filters
    if (spec.filters && spec.filters.length > 0) {
      setFilters(
        spec.filters.map(f => ({
          id: crypto.randomUUID(),
          dimension: f.dimension,
          operator: f.operator,
          value: f.operator === 'in' ? f.values.join(', ') : f.values[0] || '',
        }))
      );
    } else {
      setFilters([]);
    }

    // Sort
    if (spec.sort && spec.sort.length > 0) {
      setSort(
        spec.sort.map(s => ({
          id: crypto.randomUUID(),
          field: s.field,
          direction: s.direction,
        }))
      );
    } else {
      setSort([]);
    }

    setIsSidebarOpen(false);
  };

  const handleSaveChanges = async () => {
    if (!activeSavedQuery) return;
    setIsSavingChanges(true);
    setError(null);
    try {
      const updated = await updateAdminMarketingSavedQuery(activeSavedQuery.id, {
        querySpec: currentQuerySpec,
      });
      setActiveSavedQuery(updated);
      setRefreshKey(k => k + 1);
    } catch (err: unknown) {
      setError(mapSavedQueryError(err));
    } finally {
      setIsSavingChanges(false);
    }
  };

  const handleSaveModalSuccess = (query: SavedQuery) => {
    if (saveModalMode === 'create') {
      setActiveSavedQuery(query);
    } else if (saveModalMode === 'rename') {
      if (activeSavedQuery?.id === query.id) {
        setActiveSavedQuery(query);
      }
    }
    setRefreshKey(k => k + 1);
    setSaveModalMode(null);
  };

  const handleQueryDeleted = (deleted: SavedQuery) => {
    if (activeSavedQuery?.id === deleted.id) {
      setActiveSavedQuery(null);
    }
    setRefreshKey(k => k + 1);
  };

  const handleExport = async (format: 'csv' | 'xlsx') => {
    if (isExporting) return;
    if (metrics.length === 0) return;
    setIsExporting(true);
    setIsExportMenuOpen(false);
    setError(null);
    setExportError(null);

    try {
      const res = await exportAdminMarketingQuery(format, buildCurrentQueryRequest());
      if (res && res.blob) {
        const url = window.URL.createObjectURL(res.blob);
        const a = document.createElement('a');
        a.style.display = 'none';
        a.href = url;
        a.download = res.filename || `zamk-marketing-report.${format}`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        window.URL.revokeObjectURL(url);
      }
    } catch (err: any) {
      const msg = mapExportErrorMessage(err);
      setExportError(msg);
    } finally {
      setIsExporting(false);
    }
  };

  const handleRun = async () => {
    if (!range.from || !range.to) return;
    if (metrics.length === 0) {
      setError('Выберите хотя бы один показатель.');
      return;
    }
    setLoading(true);
    setError(null);

    try {
      const res = await executeAdminMarketingQuery(currentQuerySpec);
      setResult(res);
    } catch (err: any) {
      setError(mapErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const toggleDimension = (key: string) => {
    setDimFeedback(null);
    if (dimensions.includes(key)) {
      const nextDims = dimensions.filter(d => d !== key);
      setDimensions(nextDims);
      // Remove any filter or sort that was tied to this dimension
      setFilters(prev => prev.filter(f => f.dimension !== key));
      setSort(prev => prev.filter(s => s.field !== key));
    } else {
      if (dimensions.length >= 2) {
        setDimFeedback('Максимум 2 разреза');
        return;
      }
      setDimensions([...dimensions, key]);
    }
  };

  const toggleMetric = (key: string) => {
    setMetricFeedback(null);
    if (metrics.includes(key)) {
      setMetrics(metrics.filter(m => m !== key));
      setSort(prev => prev.filter(s => s.field !== key));
    } else {
      if (metrics.length >= 8) {
        setMetricFeedback('Максимум 8 показателей');
        return;
      }
      setMetrics([...metrics, key]);
    }
  };

  const addFilter = () => {
    if (dimensions.length === 0) return;
    if (filters.length >= 10) return;
    setFilters([
      ...filters,
      {
        id: String(Date.now() + Math.random()),
        dimension: dimensions[0],
        operator: 'eq',
        value: '',
      },
    ]);
  };

  const removeFilter = (id: string) => {
    setFilters(filters.filter(f => f.id !== id));
  };

  const updateFilter = (id: string, patch: Partial<FilterItem>) => {
    setFilters(filters.map(f => (f.id === id ? { ...f, ...patch } : f)));
  };

  const addSort = () => {
    setSortFeedback(null);
    if (sort.length >= 2) {
      setSortFeedback('Максимум 2 сортировки');
      return;
    }
    const availableFields = [...dimensions, ...metrics];
    if (availableFields.length === 0) return;
    setSort([
      ...sort,
      {
        id: String(Date.now() + Math.random()),
        field: availableFields[0],
        direction: 'desc',
      },
    ]);
  };

  const removeSort = (id: string) => {
    setSortFeedback(null);
    setSort(sort.filter(s => s.id !== id));
  };

  const updateSort = (id: string, patch: Partial<SortItem>) => {
    setSort(sort.map(s => (s.id === id ? { ...s, ...patch } : s)));
  };

  // Projected visible columns (hiding raw and technical split columns)
  const visibleColumns = useMemo(() => {
    if (!result?.columns) return [];
    return result.columns.filter(col => {
      if (col.key === 'sourceKey' || col.key === 'sourceKind' || col.key.startsWith('_raw_')) {
        return false;
      }
      return true;
    });
  }, [result?.columns]);

  const availableSortFields = useMemo(() => {
    const dimOptions = dimensions.map(dKey => ({
      key: dKey,
      label: DIMENSIONS.find(d => d.key === dKey)?.label || dKey,
    }));
    const metOptions = metrics.map(mKey => ({
      key: mKey,
      label: METRICS.find(m => m.key === mKey)?.label || mKey,
    }));
    return [...dimOptions, ...metOptions];
  }, [dimensions, metrics]);

  const formatCell = (val: any, col: QueryColumn, row: any) => {
    if (col.key === 'source') {
      return formatSource(row?.sourceKind || 'unattributed', row?.sourceKey || null);
    }

    // Coverage check for metrics
    if (col.kind === 'metric' && result?.coverage?.[col.key]?.status === 'unavailable') {
      return '—';
    }

    if (val === null || val === undefined) return '—';

    if (typeof val === 'string' && UUID_REGEX.test(val)) {
      return '—';
    }

    if (col.kind === 'dimension') {
      if (col.type === 'date') {
        const d = new Date(val);
        return Number.isFinite(d.getTime()) ? d.toLocaleDateString('ru-RU') : String(val);
      }
      return String(val);
    }

    if (col.type === 'money') return money(val);
    if (col.type === 'percentage') return formatPercentage(val);
    return number(val);
  };

  return (
    <div className="max-w-[1200px] mx-auto px-8 py-8 min-h-screen">
      <div className="mb-8">
        <AdminMarketingTabs />
      </div>

      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4 mb-10">
        <div>
          <h1 className="text-xl font-medium tracking-tight text-gray-900">Аналитика</h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-1">Конструктор отчетов</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-4">
          <MarketingPeriodControl period={period} range={range} onSelect={select} />

          <button
            onClick={() => setIsSidebarOpen(true)}
            className="inline-flex items-center px-4 py-2 border border-gray-300 shadow-sm text-[11px] font-bold uppercase tracking-widest rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            <FolderOpen className="h-4 w-4 mr-2 text-gray-400" />
            Сохранённые
          </button>

          {activeSavedQuery && !isDirty ? (
            <button
              disabled
              className="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-[11px] font-bold uppercase tracking-widest rounded-md text-indigo-400 bg-indigo-50 cursor-not-allowed"
            >
              <Save className="h-4 w-4 mr-2" />
              Сохранено
            </button>
          ) : activeSavedQuery && isDirty ? (
            <button
              onClick={handleSaveChanges}
              disabled={metrics.length === 0 || isSavingChanges}
              className="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-[11px] font-bold uppercase tracking-widest rounded-md text-white bg-gray-900 hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
            >
              <Save className="h-4 w-4 mr-2" />
              {isSavingChanges ? 'Сохранение...' : 'Сохранить изменения'}
            </button>
          ) : (
            <button
              onClick={() => setSaveModalMode('create')}
              disabled={metrics.length === 0}
              className="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-[11px] font-bold uppercase tracking-widest rounded-md text-white bg-gray-900 hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
            >
              <Save className="h-4 w-4 mr-2" />
              Сохранить
            </button>
          )}

          {/* Export action */}
          <div className="relative">
            <button
              type="button"
              onClick={() => setIsExportMenuOpen(prev => !prev)}
              disabled={metrics.length === 0 || isExporting}
              className="inline-flex items-center px-4 py-2 border border-gray-300 shadow-sm text-[11px] font-bold uppercase tracking-widest rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50"
            >
              <Download className="h-4 w-4 mr-2 text-gray-400" />
              {isExporting ? 'Экспорт...' : 'Экспорт'}
            </button>

            {isExportMenuOpen && !isExporting && (
              <div
                className="absolute right-0 mt-2 w-44 rounded-md shadow-lg bg-white ring-1 ring-black ring-opacity-5 border border-gray-100 z-20 py-1"
                role="menu"
              >
                <button
                  type="button"
                  onClick={() => handleExport('csv')}
                  disabled={isExporting}
                  className="w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 flex items-center justify-between"
                  role="menuitem"
                >
                  CSV
                </button>
                <button
                  type="button"
                  onClick={() => handleExport('xlsx')}
                  disabled={isExporting}
                  className="w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 flex items-center justify-between"
                  role="menuitem"
                >
                  Excel (.xlsx)
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {exportError && (
        <div className="mb-4 p-3 bg-red-50 text-red-700 text-xs font-medium rounded-lg border border-red-100 flex items-center justify-between" role="alert">
          <p>{exportError}</p>
          <button
            type="button"
            onClick={() => setExportError(null)}
            className="text-xs text-red-600 hover:text-red-800 font-semibold ml-4"
          >
            Закрыть
          </button>
        </div>
      )}

      {activeSavedQuery && (
        <div className="mb-6 bg-gray-50 border border-gray-200 rounded-md p-4 flex items-center justify-between">
          <div className="flex flex-col">
             <span className="text-sm font-medium text-gray-900">
               Сохранённый запрос: {activeSavedQuery.name}
             </span>
             {isDirty && (
               <span className="text-xs text-amber-600 mt-1 flex items-center">
                 <AlertCircle className="w-3 h-3 mr-1"/>
                 Есть несохранённые изменения
               </span>
             )}
          </div>
          <button
             onClick={() => setActiveSavedQuery(null)}
             className="text-gray-500 hover:text-gray-900 text-xs font-bold tracking-widest uppercase"
          >
             Сбросить
          </button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
        <div className="space-y-6 lg:col-span-1">
          {/* Dimensions */}
          <section>
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-xs font-bold uppercase tracking-widest text-gray-400">Разрезы (max 2)</h3>
              {dimFeedback && (
                <span className="text-[11px] text-amber-600 font-medium" role="alert">
                  {dimFeedback}
                </span>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              {DIMENSIONS.map(d => {
                const active = dimensions.includes(d.key);
                const disabled = !active && dimensions.length >= 2;
                return (
                  <button
                    key={d.key}
                    type="button"
                    onClick={() => toggleDimension(d.key)}
                    className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                      active
                        ? 'bg-gray-900 text-white'
                        : disabled
                        ? 'bg-gray-100 text-gray-400 cursor-not-allowed'
                        : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                    }`}
                  >
                    {d.label}
                  </button>
                );
              })}
            </div>
          </section>

          {/* Metrics */}
          <section>
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-xs font-bold uppercase tracking-widest text-gray-400">Показатели (max 8)</h3>
              {metricFeedback && (
                <span className="text-[11px] text-amber-600 font-medium" role="alert">
                  {metricFeedback}
                </span>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              {METRICS.map(m => {
                const active = metrics.includes(m.key);
                const disabled = !active && metrics.length >= 8;
                return (
                  <button
                    key={m.key}
                    type="button"
                    onClick={() => toggleMetric(m.key)}
                    className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                      active
                        ? 'bg-indigo-600 text-white'
                        : disabled
                        ? 'bg-gray-100 text-gray-400 cursor-not-allowed'
                        : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                    }`}
                  >
                    {m.label}
                  </button>
                );
              })}
            </div>
          </section>

          {/* Filters */}
          <section>
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-xs font-bold uppercase tracking-widest text-gray-400">Фильтры</h3>
              {dimensions.length > 0 && filters.length < 10 && (
                <button
                  type="button"
                  onClick={addFilter}
                  className="text-xs text-indigo-600 hover:text-indigo-800 font-medium"
                >
                  + Добавить фильтр
                </button>
              )}
            </div>

            {dimensions.length === 0 && (
              <p className="text-xs text-gray-400 italic">Для добавления фильтров выберите хотя бы один разрез</p>
            )}

            {filters.length > 0 && (
              <div className="space-y-3 mt-2">
                {filters.map(f => (
                  <div key={f.id} className="p-2.5 bg-gray-50 border border-gray-200 rounded-lg space-y-2 text-xs">
                    <div className="flex items-center gap-2">
                      <select
                        aria-label="Разрез фильтра"
                        value={f.dimension}
                        onChange={e => updateFilter(f.id, { dimension: e.target.value })}
                        className="flex-1 bg-white border border-gray-300 rounded px-2 py-1 text-xs text-gray-800"
                      >
                        {dimensions.map(dKey => (
                          <option key={dKey} value={dKey}>
                            {DIMENSIONS.find(d => d.key === dKey)?.label || dKey}
                          </option>
                        ))}
                      </select>

                      <select
                        aria-label="Оператор фильтра"
                        value={f.operator}
                        onChange={e => updateFilter(f.id, { operator: e.target.value })}
                        className="bg-white border border-gray-300 rounded px-2 py-1 text-xs text-gray-800"
                      >
                        {FILTER_OPERATORS.map(op => (
                          <option key={op.key} value={op.key}>
                            {op.label}
                          </option>
                        ))}
                      </select>

                      <button
                        type="button"
                        onClick={() => removeFilter(f.id)}
                        className="text-red-500 hover:text-red-700 px-1 font-bold"
                        title="Удалить фильтр"
                      >
                        ✕
                      </button>
                    </div>

                    <input
                      type="text"
                      aria-label="Значение фильтра"
                      placeholder={f.operator === 'in' ? 'значение1, значение2' : 'Значение'}
                      value={f.value}
                      onChange={e => updateFilter(f.id, { value: e.target.value })}
                      className="w-full bg-white border border-gray-300 rounded px-2 py-1 text-xs text-gray-800"
                    />
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Sort */}
          <section>
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-xs font-bold uppercase tracking-widest text-gray-400">Сортировка (max 2)</h3>
              {availableSortFields.length > 0 && sort.length < 2 && (
                <button
                  type="button"
                  onClick={addSort}
                  className="text-xs text-indigo-600 hover:text-indigo-800 font-medium"
                >
                  + Добавить сортировку
                </button>
              )}
            </div>

            {sortFeedback && (
              <span className="text-[11px] text-amber-600 font-medium block mb-2" role="alert">
                {sortFeedback}
              </span>
            )}

            {sort.length > 0 && (
              <div className="space-y-2 mt-2">
                {sort.map(s => (
                  <div key={s.id} className="flex items-center gap-2 p-2 bg-gray-50 border border-gray-200 rounded-lg text-xs">
                    <select
                      aria-label="Поле сортировки"
                      value={s.field}
                      onChange={e => updateSort(s.id, { field: e.target.value })}
                      className="flex-1 bg-white border border-gray-300 rounded px-2 py-1 text-xs text-gray-800"
                    >
                      {availableSortFields.map(f => (
                        <option key={f.key} value={f.key}>
                          {f.label}
                        </option>
                      ))}
                    </select>

                    <select
                      aria-label="Направление сортировки"
                      value={s.direction}
                      onChange={e => updateSort(s.id, { direction: e.target.value as 'asc' | 'desc' })}
                      className="bg-white border border-gray-300 rounded px-2 py-1 text-xs text-gray-800"
                    >
                      <option value="desc">По убыванию</option>
                      <option value="asc">По возрастанию</option>
                    </select>

                    <button
                      type="button"
                      onClick={() => removeSort(s.id)}
                      className="text-red-500 hover:text-red-700 px-1 font-bold"
                      title="Удалить сортировку"
                    >
                      ✕
                    </button>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Limit */}
          <section>
            <h3 className="text-xs font-bold uppercase tracking-widest text-gray-400 mb-2">Строк в отчете</h3>
            <select
              aria-label="Лимит строк"
              value={limit}
              onChange={e => setLimit(Number(e.target.value))}
              className="w-full bg-white border border-gray-300 rounded-lg px-3 py-1.5 text-xs text-gray-800 font-medium"
            >
              {LIMIT_OPTIONS.map(l => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </section>

          {/* Build button */}
          <button
            type="button"
            onClick={handleRun}
            disabled={loading || metrics.length === 0}
            className="w-full py-2.5 bg-gray-900 text-white text-sm font-semibold rounded-lg hover:bg-gray-800 disabled:opacity-50 transition-colors shadow-sm"
          >
            {loading ? 'Загрузка...' : 'Построить отчет'}
          </button>

          {/* Error Banner */}
          {error && (
            <div className="p-3 bg-red-50 text-red-700 text-xs font-medium rounded-lg border border-red-100 space-y-2">
              <p>{error}</p>
              <button
                type="button"
                onClick={handleRun}
                disabled={loading}
                className="text-xs text-red-800 underline font-semibold hover:text-red-900"
              >
                Повторить попытку
              </button>
            </div>
          )}
        </div>

        {/* Results Area */}
        <div className="lg:col-span-3">
          {loading && (
            <div className="h-64 flex flex-col items-center justify-center border border-gray-200 rounded-xl bg-white shadow-sm">
              <div className="inline-block h-6 w-6 animate-spin rounded-full border-2 border-solid border-indigo-600 border-r-transparent mb-3" />
              <p className="text-sm text-gray-500 font-medium">Построение отчета...</p>
            </div>
          )}

          {!loading && result && (
            <div className="bg-white border border-gray-200 rounded-xl overflow-hidden shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="border-b border-gray-200 bg-gray-50/50">
                      {visibleColumns.map((c, i) => {
                        const dimDef = DIMENSIONS.find(d => d.key === c.key);
                        const metDef = METRICS.find(m => m.key === c.key);
                        const label = COLUMN_LABELS[c.key] || dimDef?.label || metDef?.label || c.key;
                        const coverage = result.coverage?.[c.key];
                        return (
                          <th
                            key={i}
                            className="px-4 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wider whitespace-nowrap"
                          >
                            <span className="inline-flex items-center gap-1.5">
                              {label}
                              {coverage?.status === 'partial' && (
                                <span
                                  className="text-[10px] text-amber-700 bg-amber-50 px-1 py-0.5 rounded font-normal normal-case border border-amber-200/60"
                                  title="Неполные данные за выбранный период"
                                >
                                  ~ неполные данные
                                </span>
                              )}
                            </span>
                          </th>
                        );
                      })}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100 text-sm">
                    {result.rows.length === 0 ? (
                      <tr>
                        <td colSpan={visibleColumns.length || 1} className="px-4 py-12 text-center text-gray-400">
                          Нет данных за выбранный период
                        </td>
                      </tr>
                    ) : (
                      result.rows.map((row, i) => (
                        <tr key={i} className="hover:bg-gray-50/50 transition-colors">
                          {visibleColumns.map((c, j) => (
                            <td
                              key={j}
                              className={`px-4 py-3 ${
                                c.kind === 'metric' ? 'tabular-nums text-gray-900 font-medium' : 'text-gray-700'
                              }`}
                            >
                              {formatCell(row[c.key], c, row)}
                            </td>
                          ))}
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {!loading && !result && !error && (
            <div className="h-64 flex items-center justify-center border border-dashed border-gray-200 rounded-xl bg-gray-50/30">
              <p className="text-sm text-gray-400 font-medium">Выберите параметры и нажмите «Построить отчет»</p>
            </div>
          )}
        </div>
      </div>

      <AdminMarketingSavedQueriesSidebar
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
        onSelect={handleLoadSavedQuery}
        onRename={(query) => {
          setQueryToRename(query);
          setSaveModalMode('rename');
        }}
        onDeleted={handleQueryDeleted}
        activeQueryId={activeSavedQuery?.id || null}
        refreshKey={refreshKey}
      />

      <AdminMarketingSaveQueryModal
        isOpen={saveModalMode !== null}
        onClose={() => setSaveModalMode(null)}
        mode={saveModalMode || 'create'}
        initialQuery={saveModalMode === 'rename' ? queryToRename : undefined}
        querySpec={saveModalMode === 'create' ? currentQuerySpec : null}
        onSuccess={handleSaveModalSuccess}
      />
</div>
  );
}
