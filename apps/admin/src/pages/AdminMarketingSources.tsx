import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { CustomSelect } from '../components/marketing/CustomSelect';
import { getMarketingSources } from '../api/marketing';
import { SourceMetrics } from '../api/marketing';
import { number, money, percent, conversion } from '../components/marketing/marketingPresentation';
import { formatSource } from '../utils/sourceFormatter';

const SORT_OPTIONS = [
  { value: 'visits', label: 'По сессиям' },
  { value: 'revenue', label: 'По выручке' },
  { value: 'orders', label: 'По заказам' },
  { value: 'conversion', label: 'По конверсии' },
];

function UnavailableMetric({ tooltip = 'Недостаточно данных' }: { tooltip?: string }) {
  return (
    <span
      className="text-gray-300 font-normal cursor-help select-none"
      title={tooltip}
    >
      —
    </span>
  );
}

function SourceIdentity({ s, name }: { s: SourceMetrics, name: string }) {
  if (s.sourceKind === 'direct') {
    return (
      <div className="w-10 h-10 rounded-full bg-gray-50 flex items-center justify-center flex-shrink-0">
        <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M14 5l7 7m0 0l-7 7m7-7H3"></path></svg>
      </div>
    );
  }
  if (s.sourceKind === 'unattributed') {
    return (
      <div className="w-10 h-10 rounded-full bg-gray-50 flex items-center justify-center flex-shrink-0 border border-gray-100">
        <svg className="w-4 h-4 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
      </div>
    );
  }
  return (
    <div className="w-10 h-10 rounded-full bg-gray-100 flex items-center justify-center flex-shrink-0 text-[11px] font-bold text-gray-500 uppercase">
      {name.substring(0, 1)}
    </div>
  );
}

export function AdminMarketingSources() {
  const { period, range, select } = useMarketingRange();

  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [sort, setSort] = useState('visits');
  const [reloadKey, setReloadKey] = useState(0);

  const [sources, setSources] = useState<SourceMetrics[]>([]);
  const [coverage, setCoverage] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const handler = setTimeout(() => {
      setSearch(searchInput);
    }, 400);
    return () => clearTimeout(handler);
  }, [searchInput]);

  useEffect(() => {
    setLoading(true);
    setError(null);
    getMarketingSources(range.from, range.to, search, sort)
      .then(res => {
        setSources(res.sources || []);
        setCoverage(res.coverage);
        setLoading(false);
      })
      .catch(err => {
        setError(err.message || 'Ошибка загрузки');
        setLoading(false);
      });
  }, [range, sort, search, reloadKey]);

  const isEmpty = sources.length === 0 && !loading && !error;
  const hasPartial = coverage && Object.values(coverage).some((c: any) => c.status === 'partial');
  const maxRevenue = sources.length > 0 ? Math.max(...sources.map(s => s.revenueCents)) : 0;

  const renderGrowth = (s: SourceMetrics) => {
    if (s.previousRevenueCents <= 0) {
      return <UnavailableMetric tooltip="Нет данных за предыдущий период" />;
    }
    if (s.revenueChangePct === undefined || s.revenueChangePct === null) {
      return <UnavailableMetric tooltip="Нет данных за предыдущий период" />;
    }
    const val = s.revenueChangePct;
    if (val === 0) return <span className="text-gray-400 font-medium">—</span>;
    const isPositive = val > 0;

    return (
      <span className={`inline-flex items-center gap-0.5 font-medium ${isPositive ? 'text-emerald-600' : 'text-rose-600'}`}>
        {isPositive ? '↑' : '↓'}
        <span>{Math.abs(Math.round(val))}%</span>
      </span>
    );
  };

  function renderConversion(s: SourceMetrics) {
    if (coverage?.views?.status !== 'available') {
      return <UnavailableMetric />;
    }
    if (s.visits === 0 || s.paidOrders > s.visits) {
      return <UnavailableMetric />;
    }
    const bps = (s as any).conversionRateBps ?? (s.conversionRate > 1 ? s.conversionRate : Math.round((s.conversionRate ?? 0) * 10000));
    return (
      <span className="text-gray-900 font-medium tabular-nums">
        {percent(bps || conversion(s.paidOrders, s.visits))}
      </span>
    );
  }

  return (
    <div className="max-w-[1200px] mx-auto px-8 py-8 min-h-screen">
      <div className="mb-8">
        <AdminMarketingTabs />
      </div>

      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4 mb-10">
        <div>
          <h1 className="text-xl font-medium tracking-tight text-gray-900">Источники трафика</h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-1">Эффективность каналов привлечения</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-6">
          <MarketingPeriodControl period={period} range={range} onSelect={select} />
        </div>
      </div>

      <div className="bg-white border-y border-gray-200 py-6 mb-8">
        <div className="flex flex-col md:flex-row gap-6 justify-between mb-8">
          <div className="w-full md:w-64">
            <div className="relative">
              <input
                type="text"
                placeholder="ПОИСК ПО ИСТОЧНИКУ..."
                value={searchInput}
                onChange={e => setSearchInput(e.target.value)}
                className="w-full bg-transparent border-b border-gray-900 pb-1.5 text-[11px] font-bold uppercase tracking-widest text-gray-900 placeholder:text-gray-400 focus:outline-none focus:border-gray-900"
              />
              <svg className="w-3 h-3 absolute right-0 top-1 text-gray-900" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="square" strokeLinejoin="miter" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path></svg>
            </div>
          </div>

          <div className="flex flex-wrap items-baseline gap-6 lg:gap-8">
            <div className="w-56">
              <CustomSelect
                value={sort}
                onChange={setSort}
                placeholder="Сортировка"
                options={SORT_OPTIONS.map(opt => {
                   const isConvDisabled = opt.value === 'conversion' && coverage?.views?.status !== 'available';
                   return {
                     value: opt.value,
                     label: opt.label,
                     disabled: isConvDisabled,
                     disabledReason: isConvDisabled ? 'Недостаточно данных для сортировки' : undefined
                   };
                })}
              />
            </div>
          </div>
        </div>

        {hasPartial && (
          <div className="flex items-center gap-2 mb-4 text-[11px] text-gray-500">
            <span className="w-1.5 h-1.5 rounded-full bg-gray-400 inline-block"></span>
            <span>Поведенческие данные собраны не за весь период</span>
          </div>
        )}

        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-[10px] uppercase tracking-widest text-gray-500 border-b border-gray-200">
              <tr>
                <th className="py-3 pr-4 font-medium">Источник</th>
                <th className="px-4 py-3 font-medium text-right text-gray-900 border-l border-transparent">Выручка</th>
                <th className="px-4 py-3 font-medium text-right">Δ Выручки</th>
                <th className="px-4 py-3 font-medium text-right">Заказы</th>
                <th className="px-4 py-3 font-medium text-right">Единиц</th>
                <th className="px-4 py-3 font-medium text-right border-l border-transparent">Сессии</th>
                <th className="px-4 py-3 font-medium text-right">Конверсия</th>
                <th className="px-4 py-3 font-medium text-right border-l border-transparent">Новые</th>
                <th className="px-4 py-3 font-medium text-right">Повторные</th>
                <th className="pl-4 py-3 font-medium text-right border-l border-transparent">Возвраты</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {loading ? (
                <tr>
                  <td colSpan={10} className="py-20">
                    <div className="flex justify-center">
                      <div className="w-5 h-5 border-2 border-gray-900 border-t-transparent rounded-full animate-spin" />
                    </div>
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={10} className="py-20 text-center">
                    <p className="text-red-700 font-medium mb-4">{error}</p>
                    <button
                      onClick={() => setReloadKey(k => k + 1)}
                      className="px-6 py-2 bg-gray-900 text-white text-xs font-medium uppercase tracking-widest hover:bg-black"
                    >
                      Повторить
                    </button>
                  </td>
                </tr>
              ) : isEmpty ? (
                <tr>
                  <td colSpan={10} className="py-24 text-center">
                    <div className="text-[11px] uppercase tracking-widest font-medium text-gray-900 mb-2">Нет данных</div>
                    <div className="text-[11px] text-gray-500 max-w-sm mx-auto leading-relaxed">
                      Нет данных об источниках трафика за выбранный период.
                    </div>
                  </td>
                </tr>
              ) : (
                sources.map((s, index) => {
                  const routeParam = s.sourceKind === 'unattributed' ? '_unattributed' : (s.sourceKey ?? s.source ?? '_unattributed');
                  const linkPath = `/marketing/sources/${encodeURIComponent(routeParam)}`;
                  const name = formatSource(s.sourceKind ?? '', s.sourceKey ?? s.source ?? null);
                  const sharePct = maxRevenue > 0 ? (s.revenueCents / maxRevenue) * 100 : 0;
                  return (
                    <tr key={`${s.sourceKey ?? 'unattributed'}-${index}`} className="group hover:bg-gray-50/50 transition-colors">
                      <td className="py-3 pr-4">
                        <Link to={linkPath} className="flex items-center gap-4 group/link">
                          <div className="text-[10px] font-medium text-gray-300 w-4 text-right tabular-nums select-none">
                            {String(index + 1).padStart(2, '0')}
                          </div>
                          <SourceIdentity s={s} name={name} />
                          <div className="min-w-0 py-1 flex flex-col justify-center">
                            <div className="text-sm font-medium text-gray-900 leading-snug truncate group-hover/link:text-gray-500 transition-colors flex items-center gap-2">
                              {name}
                              <svg className="w-3.5 h-3.5 text-gray-300 opacity-0 group-hover:opacity-100 transition-opacity" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7"></path></svg>
                            </div>
                          </div>
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-gray-900 font-medium border-l border-transparent relative z-0">
                        {s.revenueCents > 0 ? (
                          <>
                            <div className="absolute inset-y-1.5 right-4 bg-gray-100 rounded-sm -z-10" style={{ width: `calc(${sharePct}% - 2rem)` }} />
                            {money(s.revenueCents)}
                          </>
                        ) : (
                          <span className="text-gray-300">—</span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-gray-900">
                        {renderGrowth(s)}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-gray-900">
                        {s.paidOrders > 0 ? number(s.paidOrders) : <span className="text-gray-300">—</span>}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-gray-500">
                        {s.soldUnits > 0 ? number(s.soldUnits) : <span className="text-gray-300">—</span>}
                      </td>

                      <td className="px-4 py-3 text-right tabular-nums text-gray-500 border-l border-transparent">
                        {number(s.visits)}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-gray-500">
                        {renderConversion(s)}
                      </td>

                      <td className="px-4 py-3 text-right tabular-nums text-gray-500 border-l border-transparent">
                        {s.newCustomers > 0 ? number(s.newCustomers) : <span className="text-gray-300">—</span>}
                      </td>
                      <td className="px-4 py-3 text-right tabular-nums text-gray-500">
                        {s.repeatCustomers > 0 ? number(s.repeatCustomers) : <span className="text-gray-300">—</span>}
                      </td>

                      <td className="pl-4 py-3 text-right tabular-nums text-gray-900 border-l border-transparent">
                        {s.returnsCount > 0 ? (
                          <span>{number(s.returnsCount)}</span>
                        ) : s.returnsCount === 0 ? (
                          <span className="text-gray-400 font-medium">0</span>
                        ) : (
                          <span className="text-gray-300">—</span>
                        )}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
