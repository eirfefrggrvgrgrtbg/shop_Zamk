import { useState, useMemo } from 'react';
import { executeAdminMarketingQuery } from '../api/marketing';
import type { QueryRequest, QueryResponse, QueryColumn, QueryFilter, QuerySort } from '@zamk/api-client/src/types';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { number, money } from '../components/marketing/marketingPresentation';
import { formatSource } from '../utils/sourceFormatter';

import { Save, AlertCircle, Download, Database, LayoutGrid, Filter, ArrowUpDown, ChevronDown, X } from 'lucide-react';
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

export interface FilterItem { id: string; dimension: string; operator: string; value: string; }
export interface SortItem { id: string; field: string; direction: 'asc' | 'desc'; }

export function formatPercentage(value: unknown): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'number' && Number.isFinite(value)) {
    if (value === 0) return '0%';
    if (Math.abs(value) <= 1) return `${(value * 100).toFixed(2)}%`;
    return `${value.toFixed(2)}%`;
  }
  return '—';
}

export function mapErrorMessage(err: any): string {
  const code = err?.code || err?.error || '';
  switch (code) {
    case 'unsupported_combination': return 'Этот показатель нельзя использовать с выбранными разрезами.';
    case 'invalid_period': return 'Некорректный период запроса.';
    case 'period_too_long': return 'Период запроса не может превышать 366 дней.';
    case 'missing_metrics': return 'Выберите хотя бы один показатель.';
    case 'too_many_dimensions': return 'Максимум 2 разреза.';
    case 'invalid_dimension': return 'Некорректный разрез.';
    case 'invalid_metric': return 'Некорректный показатель.';
    case 'too_many_metrics': return 'Максимум 8 показателей.';
    case 'invalid_filter_dimension': return 'Некорректный разрез для фильтра.';
    case 'unsupported_filter_dimension': return 'Фильтр возможен только по выбранным разрезам.';
    case 'invalid_filter_operator': return 'Некорректный оператор фильтра.';
    case 'empty_filter_values': return 'Значение фильтра не может быть пустым.';
    case 'too_many_filters': return 'Максимум 10 фильтров.';
    case 'too_many_sort_keys': return 'Максимум 2 поля сортировки.';
    case 'invalid_sort_field': return 'Сортировка возможна только по выбранным разрезам или показателям.';
    case 'invalid_sort_direction': return 'Направление сортировки должно быть asc или desc.';
    case 'invalid_limit': return 'Лимит должен быть от 1 до 1000.';
    default: return 'Произошла ошибка при выполнении запроса. Попробуйте позже.';
  }
}

export function mapExportErrorMessage(err: any): string {
  const status = err?.status || err?.statusCode;
  if (status === 400) return 'Не удалось экспортировать отчёт. Проверьте параметры запроса.';
  if (status === 403) return 'Недостаточно прав для экспорта отчёта.';
  if (status === 404) return 'Не удалось экспортировать отчёт.';
  return 'Не удалось экспортировать отчёт. Попробуйте ещё раз.';
}

export function AdminMarketingQueryBuilder() {
  const { period, range, select } = useMarketingRange();

  const [dimensions, setDimensions] = useState<string[]>(['source']);
  const [metrics, setMetrics] = useState<string[]>(['revenue', 'orders']);
  const [filters, setFilters] = useState<FilterItem[]>([]);
  const [sort, setSort] = useState<SortItem[]>([]);
  const [limit, setLimit] = useState<number>(100);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<QueryResponse | null>(null);

  const [activeSavedQuery, setActiveSavedQuery] = useState<SavedQuery | null>(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [saveModalMode, setSaveModalMode] = useState<'create' | 'rename' | null>(null);
  const [queryToRename, setQueryToRename] = useState<SavedQuery | undefined>(undefined);
  const [isSavingChanges, setIsSavingChanges] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  const [isExportMenuOpen, setIsExportMenuOpen] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const buildCurrentQueryRequest = (): QueryRequest => {
    const queryFilters: QueryFilter[] = filters
      .map(f => {
        const trimmed = f.value.trim();
        const values = f.operator === 'in' ? trimmed.split(',').map(v => v.trim()).filter(Boolean) : (trimmed ? [trimmed] : []);
        return { dimension: f.dimension, operator: f.operator, values };
      })
      .filter(f => f.values.length > 0);

    const querySort: QuerySort[] = sort
      .filter(s => Boolean(s.field))
      .map(s => ({ field: s.field, direction: s.direction }));

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
    setLimit(spec.limit ?? 100);

    if (spec.period) {
      select('custom', { from: spec.period.from, to: spec.period.to });
    }

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
      const updated = await updateAdminMarketingSavedQuery(activeSavedQuery.id, { querySpec: currentQuerySpec });
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
      if (activeSavedQuery?.id === query.id) setActiveSavedQuery(query);
    }
    setRefreshKey(k => k + 1);
    setSaveModalMode(null);
  };

  const handleQueryDeleted = (deleted: SavedQuery) => {
    if (activeSavedQuery?.id === deleted.id) setActiveSavedQuery(null);
    setRefreshKey(k => k + 1);
  };

  const handleExport = async (format: 'csv' | 'xlsx') => {
    if (isExporting || metrics.length === 0) return;
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
      setExportError(mapExportErrorMessage(err));
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
    if (dimensions.includes(key)) {
      setDimensions(dimensions.filter(d => d !== key));
      setFilters(prev => prev.filter(f => f.dimension !== key));
      setSort(prev => prev.filter(s => s.field !== key));
    } else if (dimensions.length < 2) {
      setDimensions([...dimensions, key]);
    }
  };

  const toggleMetric = (key: string) => {
    if (metrics.includes(key)) {
      setMetrics(metrics.filter(m => m !== key));
      setSort(prev => prev.filter(s => s.field !== key));
    } else if (metrics.length < 8) {
      setMetrics([...metrics, key]);
    }
  };

  const addFilter = () => {
    if (dimensions.length === 0 || filters.length >= 10) return;
    setFilters([...filters, { id: String(Date.now()), dimension: dimensions[0], operator: 'eq', value: '' }]);
  };

  const addSort = () => {
    const availableFields = [...dimensions, ...metrics];
    if (availableFields.length === 0 || sort.length >= 2) return;
    setSort([...sort, { id: String(Date.now()), field: availableFields[0], direction: 'desc' }]);
  };

  const visibleColumns = useMemo(() => {
    if (!result?.columns) return [];
    return result.columns.filter(col => col.key !== 'sourceKey' && col.key !== 'sourceKind' && !col.key.startsWith('_raw_'));
  }, [result?.columns]);

  const availableSortFields = useMemo(() => {
    const dimOptions = dimensions.map(dKey => ({ key: dKey, label: DIMENSIONS.find(d => d.key === dKey)?.label || dKey }));
    const metOptions = metrics.map(mKey => ({ key: mKey, label: METRICS.find(m => m.key === mKey)?.label || mKey }));
    return [...dimOptions, ...metOptions];
  }, [dimensions, metrics]);

  const formatCell = (val: any, col: QueryColumn, row: any) => {
    if (col.key === 'source') return formatSource(row?.sourceKind || 'unattributed', row?.sourceKey || null);
    if (col.kind === 'metric' && result?.coverage?.[col.key]?.status === 'unavailable') return '—';
    if (val === null || val === undefined) return '—';
    if (typeof val === 'string' && UUID_REGEX.test(val)) return '—';
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
    <div className="marketing-workspace">
      <AdminMarketingTabs />

      <div className="m-header items-end mb-6">
        <div>
          <h1>Аналитика</h1>
          <p>Конструктор отчетов</p>
        </div>

        <div className="flex items-center gap-3">
          <MarketingPeriodControl period={period} range={range} onSelect={select} />

          <div className="relative">
            <button
              onClick={() => setIsSidebarOpen(!isSidebarOpen)}
              className={`m-button ${isSidebarOpen ? 'bg-gray-100' : ''}`}
            >
              <Database className="w-4 h-4 text-gray-400" />
              Сохранённые
            </button>
            <AdminMarketingSavedQueriesSidebar
              isOpen={isSidebarOpen}
              onClose={() => setIsSidebarOpen(false)}
              onSelect={handleLoadSavedQuery}
              onRename={(q) => { setQueryToRename(q); setSaveModalMode('rename'); }}
              onDeleted={handleQueryDeleted}
              activeQueryId={activeSavedQuery?.id || null}
              refreshKey={refreshKey}
            />
          </div>

          <div className="relative border-l border-gray-200 pl-3">
            <button
              type="button"
              onClick={() => setIsExportMenuOpen(!isExportMenuOpen)}
              disabled={metrics.length === 0 || isExporting}
              className="m-button"
            >
              <Download className="w-4 h-4 text-gray-400" />
              {isExporting ? 'Экспорт...' : 'Экспорт'}
            </button>
            {isExportMenuOpen && !isExporting && (
              <div className="absolute right-0 mt-2 w-36 rounded-md shadow-lg bg-white ring-1 ring-black ring-opacity-5 z-20 py-1" role="menu">
                <button
                  type="button"
                  role="menuitem" onClick={() => handleExport('csv')}
                  className="w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
                >
                  CSV
                </button>
                <button
                  type="button"
                  role="menuitem" onClick={() => handleExport('xlsx')}
                  className="w-full text-left px-4 py-2 text-sm text-gray-700 hover:bg-gray-50"
                >
                  Excel (.xlsx)
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {exportError && (
        <div className="m-error mb-6">
          <span className="text-red-700">{exportError}</span>
          <button type="button" onClick={() => setExportError(null)} className="text-red-800">Закрыть</button>
        </div>
      )}

      {activeSavedQuery && (
        <div className="mb-6 flex items-center justify-between p-3.5 bg-indigo-50/50 border border-indigo-100 rounded-lg">
          <div className="flex items-center gap-3">
            <span className="text-sm font-semibold text-indigo-900">
              <span className="sr-only">Сохранённый запрос: </span>
              {activeSavedQuery.name}
            </span>
            {isDirty && (
              <span className="text-[11px] font-bold uppercase tracking-widest text-amber-600 flex items-center bg-amber-50 px-2 py-0.5 rounded border border-amber-200/50">
                Изменено
              </span>
            )}
          </div>
          <div className="flex items-center gap-3">
            {isDirty ? (
              <button
                onClick={handleSaveChanges}
                disabled={metrics.length === 0 || isSavingChanges}
                className="text-[11px] font-bold uppercase tracking-widest text-indigo-700 hover:text-indigo-900 disabled:opacity-50"
              >
                {isSavingChanges ? 'Сохранение...' : 'Сохранить изменения'}
              </button>
            ) : (
              <span className="text-[11px] font-bold uppercase tracking-widest text-indigo-400">Сохранено</span>
            )}
            <button
              onClick={() => setActiveSavedQuery(null)}
              className="text-[11px] font-bold uppercase tracking-widest text-gray-500 hover:text-gray-900 border-l border-indigo-200 pl-3"
            >
              Сбросить
            </button>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-[340px_1fr] gap-6 items-start">
        {/* BUILD ZONE */}
        <div className="m-panel flex flex-col mb-0 sticky top-4">
          <div className="m-panel-head bg-gray-50/50 border-b border-gray-100">
            <h2 className="text-sm">Конфигурация</h2>
            {!activeSavedQuery && (
              <button
                onClick={() => setSaveModalMode('create')}
                disabled={metrics.length === 0}
                className="text-[11px] font-bold uppercase tracking-widest text-indigo-600 hover:text-indigo-800 disabled:opacity-50 flex items-center gap-1"
              >
                <Save className="w-3.5 h-3.5" />
                Сохранить
              </button>
            )}
          </div>

          <div className="p-5 space-y-7">
            {/* Dimensions */}
            <section>
              <div className="flex items-center gap-2 mb-3 text-[11px] font-bold uppercase tracking-widest text-gray-400">
                <LayoutGrid className="w-3.5 h-3.5" />
                Разрезы {dimensions.length}/2
              </div>
              <div className="flex flex-wrap gap-1.5">
                {DIMENSIONS.map(d => {
                  const active = dimensions.includes(d.key);
                  const disabled = !active && dimensions.length >= 2;
                  return (
                    <button
                      key={d.key}
                      onClick={() => toggleDimension(d.key)}
                      disabled={disabled}
                      className={`px-2.5 py-1 text-xs font-medium rounded border transition-colors ${
                        active ? 'bg-indigo-50 border-indigo-200 text-indigo-700' :
                        disabled ? 'bg-gray-50 border-gray-100 text-gray-300' : 'bg-white border-gray-200 text-gray-600 hover:border-gray-300'
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
              <div className="flex items-center gap-2 mb-3 text-[11px] font-bold uppercase tracking-widest text-gray-400">
                <Database className="w-3.5 h-3.5" />
                Показатели {metrics.length}/8
              </div>
              <div className="flex flex-wrap gap-1.5">
                {METRICS.map(m => {
                  const active = metrics.includes(m.key);
                  const disabled = !active && metrics.length >= 8;
                  return (
                    <button
                      key={m.key}
                      onClick={() => toggleMetric(m.key)}
                      disabled={disabled}
                      className={`px-2.5 py-1 text-xs font-medium rounded border transition-colors ${
                        active ? 'bg-indigo-50 border-indigo-200 text-indigo-700' :
                        disabled ? 'bg-gray-50 border-gray-100 text-gray-300' : 'bg-white border-gray-200 text-gray-600 hover:border-gray-300'
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
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2 text-[11px] font-bold uppercase tracking-widest text-gray-400">
                  <Filter className="w-3.5 h-3.5" />
                  Фильтры {filters.length}/10
                </div>
                {dimensions.length > 0 && filters.length < 10 && (
                  <button onClick={addFilter} className="text-[11px] font-bold uppercase tracking-widest text-indigo-600 hover:text-indigo-800">
                    Добавить
                  </button>
                )}
              </div>
              {dimensions.length === 0 ? (
                <p className="text-[11px] text-gray-400">Выберите разрез для фильтрации</p>
              ) : filters.length === 0 ? (
                <p className="text-[11px] text-gray-400">Нет фильтров</p>
              ) : (
                <div className="space-y-2">
                  {filters.map(f => (
                    <div key={f.id} className="flex flex-col gap-1.5 p-2 bg-gray-50 border border-gray-100 rounded">
                      <div className="flex items-center gap-1.5">
                        <select
                          aria-label="Разрез фильтра"
                          value={f.dimension}
                          onChange={e => setFilters(filters.map(x => x.id === f.id ? { ...x, dimension: e.target.value } : x))}
                          className="flex-1 bg-transparent border-0 text-xs font-semibold p-0 text-gray-700 focus:ring-0 cursor-pointer appearance-none"
                        >
                          {dimensions.map(dKey => (
                            <option key={dKey} value={dKey}>{DIMENSIONS.find(d => d.key === dKey)?.label || dKey}</option>
                          ))}
                        </select>
                        <select
                          aria-label="Оператор фильтра"
                          value={f.operator}
                          onChange={e => setFilters(filters.map(x => x.id === f.id ? { ...x, operator: e.target.value } : x))}
                          className="flex-1 bg-transparent border-0 text-[11px] font-medium p-0 text-gray-500 focus:ring-0 cursor-pointer appearance-none"
                        >
                          {FILTER_OPERATORS.map(op => <option key={op.key} value={op.key}>{op.label}</option>)}
                        </select>
                        <button title="Удалить" aria-label="Удалить фильтр" onClick={() => setFilters(filters.filter(x => x.id !== f.id))} className="text-gray-400 hover:text-red-500 ml-auto">
                          <X className="w-3 h-3" />
                        </button>
                      </div>
                      <input
                        type="text"
                        aria-label="Значение фильтра"
                        value={f.value}
                        onChange={e => setFilters(filters.map(x => x.id === f.id ? { ...x, value: e.target.value } : x))}
                        placeholder={f.operator === 'in' ? 'зн1, зн2' : 'Значение'}
                        className="w-full bg-white border border-gray-200 rounded px-2 py-1 text-xs text-gray-800 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 outline-none"
                      />
                    </div>
                  ))}
                </div>
              )}
            </section>

            {/* Sort */}
            <section>
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2 text-[11px] font-bold uppercase tracking-widest text-gray-400">
                  <ArrowUpDown className="w-3.5 h-3.5" />
                  Сортировка {sort.length}/2
                </div>
                {availableSortFields.length > 0 && sort.length < 2 && (
                  <button onClick={addSort} className="text-[11px] font-bold uppercase tracking-widest text-indigo-600 hover:text-indigo-800">
                    Добавить
                  </button>
                )}
              </div>
              {sort.length === 0 ? (
                <p className="text-[11px] text-gray-400">По умолчанию</p>
              ) : (
                <div className="space-y-2">
                  {sort.map(s => (
                    <div key={s.id} className="flex items-center gap-2 p-2 bg-gray-50 border border-gray-100 rounded">
                      <select
                        aria-label="Поле сортировки"
                        value={s.field}
                        onChange={e => setSort(sort.map(x => x.id === s.id ? { ...x, field: e.target.value } : x))}
                        className="flex-1 bg-transparent border-0 text-xs font-medium p-0 text-gray-700 focus:ring-0 truncate cursor-pointer appearance-none"
                      >
                        {availableSortFields.map(f => <option key={f.key} value={f.key}>{f.label}</option>)}
                      </select>
                      <select
                        aria-label="Направление сортировки"
                        value={s.direction}
                        onChange={e => setSort(sort.map(x => x.id === s.id ? { ...x, direction: e.target.value as 'asc'|'desc' } : x))}
                        className="w-20 bg-transparent border-0 text-[11px] font-medium p-0 text-gray-500 focus:ring-0 cursor-pointer appearance-none text-right pr-2"
                      >
                        <option value="desc">По убыв.</option>
                        <option value="asc">По возр.</option>
                      </select>
                      <button title="Удалить" aria-label="Удалить сортировку" onClick={() => setSort(sort.filter(x => x.id !== s.id))} className="text-gray-400 hover:text-red-500">
                        <X className="w-3 h-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </section>

            {/* Limit */}
            <section className="flex items-center justify-between border-t border-gray-100 pt-5">
              <span className="text-[11px] font-bold uppercase tracking-widest text-gray-400">Строк в отчете</span>
              <div className="relative">
                <select
                  aria-label="Лимит строк"
                  value={limit}
                  onChange={e => setLimit(Number(e.target.value))}
                  className="pl-3 pr-8 py-1.5 bg-white border border-gray-200 rounded text-xs font-semibold text-gray-700 appearance-none outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 cursor-pointer"
                >
                  {LIMIT_OPTIONS.map(l => <option key={l} value={l}>{l}</option>)}
                </select>
                <ChevronDown className="w-3 h-3 text-gray-400 absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
              </div>
            </section>

          </div>

          <div className="p-4 bg-gray-50/50 border-t border-gray-100 mt-auto">
            <button
              onClick={handleRun}
              disabled={loading || metrics.length === 0}
              className="w-full py-2.5 bg-[#27232e] text-white text-sm font-semibold rounded-lg hover:bg-black transition-colors disabled:opacity-50 shadow-sm"
            >
              {loading ? 'Загрузка...' : 'Построить отчет'}
            </button>
          </div>
        </div>

        {/* RESULT ZONE */}
        <div className="flex flex-col min-w-0">
          {error && (
            <div className="m-error mb-4">
              <div className="flex items-center gap-2 text-red-700">
                <AlertCircle className="w-4 h-4" />
                <span className="font-medium text-sm">{error}</span>
              </div>
              <button type="button" onClick={handleRun} className="text-red-800">Повторить</button>
            </div>
          )}

          <div className="m-panel flex-1 mb-0 flex flex-col min-h-[400px]">
            {loading ? (
              <div className="flex-1 flex flex-col items-center justify-center p-12">
                <div className="inline-block h-6 w-6 animate-spin rounded-full border-2 border-solid border-indigo-600 border-r-transparent mb-3" />
                <p className="text-sm text-gray-500 font-medium">Выполнение запроса...</p>
              </div>
            ) : result ? (
              <div className="m-table-scroll flex-1">
                <table className="m-table">
                  <thead>
                    <tr>
                      {visibleColumns.map((c, i) => {
                        const dimDef = DIMENSIONS.find(d => d.key === c.key);
                        const metDef = METRICS.find(m => m.key === c.key);
                        const label = COLUMN_LABELS[c.key] || dimDef?.label || metDef?.label || c.key;
                        const isNum = c.kind === 'metric' || c.type === 'number' || c.type === 'money' || c.type === 'percentage';
                        return (
                          <th key={i} className={isNum ? 'numeric' : ''}>
                            {label}
                          </th>
                        );
                      })}
                    </tr>
                  </thead>
                  <tbody>
                    {result.rows.length === 0 ? (
                      <tr>
                        <td colSpan={visibleColumns.length || 1} className="m-empty">
                          <strong>Нет данных</strong>
                          За выбранный период по заданным критериям результаты отсутствуют.
                        </td>
                      </tr>
                    ) : (
                      result.rows.map((row, i) => (
                        <tr key={i}>
                          {visibleColumns.map((c, j) => {
                            const isNum = c.kind === 'metric' || c.type === 'number' || c.type === 'money' || c.type === 'percentage';
                            return (
                              <td key={j} className={isNum ? 'numeric' : ''}>
                                {formatCell(row[c.key], c, row)}
                              </td>
                            );
                          })}
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="flex-1 flex items-center justify-center p-12 m-empty">
                <div>
                  <strong>Отчет не построен</strong>
                  Выберите нужные разрезы и показатели слева и нажмите «Построить отчет».
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

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
