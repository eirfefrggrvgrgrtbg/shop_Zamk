import { useState, useEffect } from 'react';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { CustomSelect } from '../components/marketing/CustomSelect';
import { getAdminDesignerAnalytics } from '@zamk/api-client';
import { DesignerAnalyticsRow } from '@zamk/api-client/src/types';
import { getAdminCategories } from '../api/adminOperations';
import { number, money, percent } from '../components/marketing/marketingPresentation';

const SORT_OPTIONS = [
  { value: 'revenue', label: 'По выручке' },
  { value: 'sales', label: 'По продажам' },
  { value: 'views', label: 'По просмотрам' },
  { value: 'favorites', label: 'По избранному' },
  { value: 'conversion', label: 'По конверсии' },
  { value: 'high_views_low_sales', label: 'Много просмотров / мало продаж' },
  { value: 'revenue_growth', label: 'Рост выручки' },
  { value: 'revenue_drop', label: 'Падение выручки' },
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

export function AdminMarketingDesigners() {
  const { period, range, select } = useMarketingRange();

  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [sort, setSort] = useState('revenue');
  const [reloadKey, setReloadKey] = useState(0);

  const [designers, setDesigners] = useState<DesignerAnalyticsRow[]>([]);
  const [coverage, setCoverage] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [categories, setCategories] = useState<{id: string, name: string}[]>([]);

  useEffect(() => {
    getAdminCategories().then(res => {
      const items = Array.isArray(res) ? res : (res as any).items || [];
      setCategories(items.map((c: any) => ({
        id: c.id,
        name: c.name || 'Без названия',
      })));
    }).catch(() => {});
  }, []);

  useEffect(() => {
    const handler = setTimeout(() => {
      setSearch(searchInput);
    }, 400);
    return () => clearTimeout(handler);
  }, [searchInput]);

  useEffect(() => {
    setLoading(true);
    setError(null);
    getAdminDesignerAnalytics(
      range.from,
      range.to,
      sort,
      search,
      categoryId
    )
      .then(res => {
        setDesigners(res.designers || []);
        setCoverage(res.coverage);
        setLoading(false);
      })
      .catch(err => {
        console.error(err);
        setError('Не удалось загрузить аналитику дизайнеров');
        setLoading(false);
      });
  }, [range.from, range.to, search, categoryId, sort, reloadKey]);

  const isEmpty = designers.length === 0;

  const renderBehavioralMetric = (value: number, cov: any, _label?: string) => {
    if (!cov || cov.status === 'unavailable') {
      return <UnavailableMetric tooltip="Недостаточно данных" />;
    }
    if (cov.status === 'partial') {
      if (value > 0) {
        return (
          <span
            className="inline-flex items-center gap-1.5 cursor-help group/partial"
            title={`Зафиксировано минимум ${number(value)}. Трекинг покрывает не весь выбранный период.`}
          >
            <span>{number(value)}</span>
            <span className="w-1.5 h-1.5 rounded-full bg-gray-300 group-hover/partial:bg-gray-900 transition-colors"></span>
          </span>
        );
      }
      return <UnavailableMetric tooltip="Недостаточно данных" />;
    }
    return <span>{number(value)}</span>;
  };

  const renderConversion = (p: DesignerAnalyticsRow) => {
    if (!coverage?.views || coverage.views.status !== 'available') {
      return <UnavailableMetric tooltip="Недостаточно данных для достоверной конверсии" />;
    }
    if (p.views <= 0) {
      return <span className="text-gray-300" title="Нет просмотров">—</span>;
    }
    return (
      <span className={p.conversionRate < 0.005 && p.views > 100 ? "text-amber-700 font-medium" : ""}>
        {percent(Math.round(p.conversionRate * 10000))}
      </span>
    );
  };

  const renderGrowth = (p: DesignerAnalyticsRow) => {
    if (p.previousRevenueCents <= 0) {
      return <UnavailableMetric tooltip="Нет данных за предыдущий период" />;
    }
    if (p.revenueChangePct === undefined || p.revenueChangePct === null) {
      return <UnavailableMetric tooltip="Нет данных за предыдущий период" />;
    }
    const val = p.revenueChangePct;
    const isPositive = val > 0;
    const isNegative = val < 0;
    let colorClass = "text-gray-500";
    if (isPositive) colorClass = "text-green-700";
    if (isNegative) colorClass = "text-amber-700";

    return (
      <span className={`font-medium ${colorClass}`}>
        {isPositive ? '+' : ''}{number(Math.round(val))}%
      </span>
    );
  };

  const hasPartial = coverage && Object.values(coverage).some((c: any) => c.status === 'partial');

  return (
    <div className="max-w-[1200px] mx-auto px-8 py-8 min-h-screen">
      <div className="mb-8">
        <AdminMarketingTabs />
      </div>

      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4 mb-10">
        <div>
          <h1 className="text-xl font-medium tracking-tight text-gray-900">Дизайнеры</h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-1">Эффективность брендов и авторов</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-6">
          <MarketingPeriodControl period={period} range={range} onSelect={select} />
        </div>
      </div>

      <div className="mb-12">
        <div className="flex flex-col lg:flex-row lg:items-end justify-between gap-8 mb-8">
          <div className="flex-1 max-w-sm">
            <div className="relative">
              <input
                type="text"
                placeholder="ПОИСК ПО ИМЕНИ..."
                value={searchInput}
                onChange={e => setSearchInput(e.target.value)}
                className="w-full bg-transparent border-b border-gray-900 pb-1.5 text-[11px] font-bold uppercase tracking-widest text-gray-900 placeholder:text-gray-400 focus:outline-none focus:border-gray-900"
              />
              <svg className="w-3 h-3 absolute right-0 top-1 text-gray-900" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="square" strokeLinejoin="miter" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path></svg>
            </div>
          </div>

          <div className="flex flex-wrap items-baseline gap-6 lg:gap-8">
            <div className="w-48">
              <CustomSelect
                value={categoryId}
                onChange={setCategoryId}
                placeholder="Все категории"
                options={[
                  {value: '', label: 'Все категории'},
                  ...categories.map(c => ({value: c.id, label: c.name}))
                ]}
              />
            </div>

            <div className="w-56">
              <CustomSelect
                value={sort}
                onChange={setSort}
                placeholder="Сортировка"
                options={SORT_OPTIONS.map(opt => {
                   const isFavDisabled = opt.value === 'favorites' && coverage?.favorites?.status !== 'available';
                   const isConvDisabled = opt.value === 'conversion' && coverage?.views?.status !== 'available';
                   return {
                     value: opt.value,
                     label: opt.label,
                     disabled: isFavDisabled || isConvDisabled,
                     disabledReason: isFavDisabled || isConvDisabled ? 'Недостаточно данных для сортировки' : undefined
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
                <th className="py-3 font-bold">Дизайнер</th>
                <th className="px-4 py-3 font-bold text-right">Товары</th>
                <th className="px-4 py-3 font-bold text-right">Выручка</th>
                <th className="px-4 py-3 font-bold text-right">Δ Выручки</th>
                <th className="px-4 py-3 font-bold text-right">Покупки</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Просмотры</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Избранное</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Конверсия</th>
                <th className="px-4 py-3 font-bold text-right text-gray-400">Возвраты</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {loading ? (
                <tr>
                  <td colSpan={9} className="py-20">
                    <div className="flex justify-center">
                      <div className="w-5 h-5 border-2 border-gray-900 border-t-transparent rounded-full animate-spin" />
                    </div>
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={9} className="py-20 text-center">
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
                  <td colSpan={9} className="py-20 text-center text-gray-400 text-sm">
                    Нет данных за выбранный период
                  </td>
                </tr>
              ) : (
                designers.map((d) => (
                  <tr key={d.designerId} className="group hover:bg-gray-50/50 transition-colors">
                    <td className="py-4 align-top">
                      <div className="flex gap-4">
                        {d.primaryImage ? (
                          <div className="w-12 h-16 bg-gray-100 flex-shrink-0 overflow-hidden">
                            <img src={d.primaryImage} alt="" className="w-full h-full object-cover grayscale mix-blend-multiply group-hover:grayscale-0 group-hover:mix-blend-normal transition-all duration-500" />
                          </div>
                        ) : (
                          <div className="w-12 h-16 bg-gray-50 border border-gray-100 flex-shrink-0"></div>
                        )}
                        <div>
                          <div className="font-medium text-gray-900 leading-snug">{d.designerName}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-4 align-top text-right text-gray-600">
                      {number(d.productsCount)}
                    </td>
                    <td className="px-4 py-4 align-top text-right">
                      {d.revenueCents > 0 ? (
                        <span className="font-serif tracking-tight">{money(d.revenueCents)}</span>
                      ) : (
                        <span className="text-gray-300">—</span>
                      )}
                    </td>
                    <td className="px-4 py-4 align-top text-right">
                      {renderGrowth(d)}
                    </td>
                    <td className="px-4 py-4 align-top text-right">
                      {d.purchases > 0 ? (
                        <span className="text-gray-900">{number(d.purchases)}</span>
                      ) : (
                        <span className="text-gray-300">—</span>
                      )}
                      {d.soldUnits > d.purchases && (
                        <div className="text-[10px] text-gray-400 mt-0.5" title="Продано единиц">{number(d.soldUnits)} шт</div>
                      )}
                    </td>
                    <td className="px-4 py-4 align-top text-right text-gray-600">
                      {renderBehavioralMetric(d.views, coverage?.views, 'Просмотры')}
                    </td>
                    <td className="px-4 py-4 align-top text-right text-gray-600">
                      {renderBehavioralMetric(d.favorites, coverage?.favorites, 'Избранное')}
                    </td>
                    <td className="px-4 py-4 align-top text-right">
                      {renderConversion(d)}
                    </td>
                    <td className="px-4 py-4 align-top text-right text-gray-600">
                      {d.returns > 0 ? (
                        <span>{number(d.returns)}</span>
                      ) : (
                        <span className="text-gray-300">—</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
