import { useState, useMemo } from 'react';
import { executeAdminMarketingQuery } from '../api/marketing';
import type { QueryRequest, QueryResponse, QueryColumn, QueryFilter, QuerySort } from '@zamk/api-client/src/types';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { number, money } from '../components/marketing/marketingPresentation';
import { formatSource } from '../utils/sourceFormatter';

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

  const handleRun = async () => {
    if (!range.from || !range.to) return;
    if (metrics.length === 0) {
      setError('Выберите хотя бы один показатель.');
      return;
    }
    setLoading(true);
    setError(null);

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

    const req: QueryRequest = {
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

    try {
      const res = await executeAdminMarketingQuery(req);
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
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex items-end justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-gray-900 mb-1">Аналитика</h1>
          <p className="text-sm text-gray-500 font-medium">Конструктор отчетов</p>
        </div>
      </div>

      <AdminMarketingTabs />

      <div className="mt-6 mb-6">
        <MarketingPeriodControl period={period} range={range} onSelect={select} />
      </div>

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
    </div>
  );
}
