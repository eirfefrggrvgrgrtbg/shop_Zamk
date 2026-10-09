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
            <thead className="text-[10px] uppercase tracking-widest text-gray-900 border-b border-gray-900">
              <tr>
                <th className="py-3 font-bold">Источник</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Сессии</th>
                <th className="px-4 py-3 font-bold text-right">Заказы</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Единиц</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Конверсия</th>
                <th className="px-4 py-3 font-bold text-right">Выручка</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {loading ? (
                <tr>
                  <td colSpan={6} className="py-20">
                    <div className="flex justify-center">
                      <div className="w-5 h-5 border-2 border-gray-900 border-t-transparent rounded-full animate-spin" />
                    </div>
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={6} className="py-20 text-center">
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
                  <td colSpan={6} className="py-20 text-center text-gray-400 text-sm">
                    Нет данных за выбранный период
                  </td>
                </tr>
              ) : (
                sources.map((s, index) => {
                  const routeParam = s.sourceKind === 'unattributed' ? '_unattributed' : (s.sourceKey ?? s.source ?? '_unattributed');
                  const linkPath = `/marketing/sources/${encodeURIComponent(routeParam)}`;
                  const name = formatSource(s.sourceKind ?? '', s.sourceKey ?? s.source ?? null);
                  return (
                    <tr key={`${s.sourceKey ?? 'unattributed'}-${index}`} className="group hover:bg-gray-50/50 transition-colors">
                      <td className="py-4 align-top">
                        <div className="flex gap-4 items-center">
                          <div className="w-8 h-8 rounded bg-gray-100 flex items-center justify-center flex-shrink-0 text-[10px] font-bold text-gray-400">
                            {name.substring(0, 1).toUpperCase()}
                          </div>
                          <div>
                            <Link to={linkPath} className="font-medium text-gray-900 hover:text-gray-600 transition-colors">
                              {name}
                            </Link>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-4 align-middle text-right text-gray-600">
                        {number(s.visits)}
                      </td>
                      <td className="px-4 py-4 align-middle text-right text-gray-900">
                        {number(s.paidOrders)}
                      </td>
                      <td className="px-4 py-4 align-middle text-right text-gray-600">
                        {number(s.soldUnits)}
                      </td>
                      <td className="px-4 py-4 align-middle text-right">
                        {renderConversion(s)}
                      </td>
                      <td className="px-4 py-4 align-middle text-right">
                        {s.revenueCents > 0 ? (
                          <span className="font-serif tracking-tight text-gray-900">{money(s.revenueCents)}</span>
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
