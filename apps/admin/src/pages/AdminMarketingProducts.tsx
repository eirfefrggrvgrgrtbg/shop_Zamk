import { useState, useEffect } from 'react';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { CustomSelect } from '../components/marketing/CustomSelect';
import { getMarketingProducts, ProductAnalyticsRow, ProductAnalyticsCoverage, MetricCoverage } from '../api/marketing';
import { getAdminCategories, getAdminSellers } from '../api/adminOperations';
import { number, money, percent } from '../components/marketing/marketingPresentation';

const SORT_OPTIONS = [
  { value: 'revenue', label: 'По выручке' },
  { value: 'sales', label: 'По продажам' },
  { value: 'views', label: 'По просмотрам' },
  { value: 'favorites', label: 'По избранному' },
  { value: 'conversion', label: 'По конверсии' },
  { value: 'high_views_low_sales', label: 'Много просмотров / мало продаж' },
];

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export const formatDesignerName = (name: string | null | undefined): string => {
  if (!name || name.trim() === '') return 'Без дизайнера';
  if (UUID_REGEX.test(name.trim())) return 'Без названия';
  return name;
};

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

export function AdminMarketingProducts() {
  const { period, range, select } = useMarketingRange();

  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState(''); // for debouncing
  const [categoryId, setCategoryId] = useState('');
  const [designerId, setDesignerId] = useState('');
  const [sort, setSort] = useState('revenue');
  const [reloadKey, setReloadKey] = useState(0);

  const [products, setProducts] = useState<ProductAnalyticsRow[]>([]);
  const [coverage, setCoverage] = useState<ProductAnalyticsCoverage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [categories, setCategories] = useState<{id: string, name: string}[]>([]);
  const [designers, setDesigners] = useState<{id: string, name: string}[]>([]);

  useEffect(() => {
    getAdminCategories().then(res => {
      const items = Array.isArray(res) ? res : (res as any).items || [];
      setCategories(items.map((c: any) => ({
        id: c.id,
        name: c.name || 'Без названия',
      })));
    }).catch(() => {});

    getAdminSellers().then((res: any) => {
      const items = res.items || [];
      setDesigners(items.map((i: any) => {
        const rawName = i.brandName || i.name || i.storeName;
        return {
          id: i.id,
          name: formatDesignerName(rawName),
        };
      }));
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
    getMarketingProducts(
      range.from,
      range.to,
      sort,
      undefined,
      search,
      categoryId,
      designerId
    )
      .then(res => {
        setProducts(res.products || []);
        setCoverage(res.coverage);
        setLoading(false);
      })
      .catch(err => {
        console.error(err);
        setError('Не удалось загрузить аналитику товаров');
        setLoading(false);
      });
  }, [range.from, range.to, search, categoryId, designerId, sort, reloadKey]);

  const isEmpty = products.length === 0;

  const renderBehavioralMetric = (value: number, cov: MetricCoverage | undefined, _label?: string) => {
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
    // Available: complete tracking, so 0 is truthful 0!
    return <span>{number(value)}</span>;
  };

  const renderConversion = (p: ProductAnalyticsRow) => {
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

  const hasPartial = coverage && Object.values(coverage).some(c => c.status === 'partial');

  return (
    <div className="max-w-[1200px] mx-auto px-8 py-8 min-h-screen">
      <div className="mb-8">
        <AdminMarketingTabs />
      </div>

      <div className="flex flex-col md:flex-row md:items-start justify-between gap-4 mb-10">
        <div>
          <h1 className="text-xl font-medium tracking-tight text-gray-900">Товары</h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-1">Интерес, продажи и эффективность товаров</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-6">
          <MarketingPeriodControl period={period} range={range} onSelect={select} />
        </div>
      </div>

      <div className="mb-12">
        {/* Filters */}
        <div className="flex flex-col lg:flex-row lg:items-end justify-between gap-8 mb-8">
          <div className="flex-1 max-w-sm">
            <div className="relative">
              <input
                type="text"
                placeholder="ПОИСК ПО НАЗВАНИЮ..."
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

            <div className="w-48">
              <CustomSelect
                value={designerId}
                onChange={setDesignerId}
                placeholder="Все дизайнеры"
                options={[
                  {value: '', label: 'Все дизайнеры'},
                  ...designers.map(d => ({value: d.id, label: d.name}))
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

        {/* Coverage Legend */}
        {hasPartial && (
          <div className="flex items-center gap-2 mb-4 text-[11px] text-gray-500">
            <span className="w-1.5 h-1.5 rounded-full bg-gray-400 inline-block"></span>
            <span>Поведенческие данные собраны не за весь период</span>
          </div>
        )}

        {/* Table */}
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="text-[10px] uppercase tracking-widest text-gray-500 border-b border-gray-200">
              <tr>
                <th className="py-3 pr-4 font-medium">Товар</th>
                <th className="px-4 py-3 font-medium text-right text-gray-900 border-l border-transparent">Выручка</th>
                <th className="px-4 py-3 font-medium text-right">Покупки</th>
                <th className="px-4 py-3 font-medium text-right border-l border-transparent">Просмотры</th>
                <th className="px-4 py-3 font-medium text-right">Избранное</th>
                <th className="px-4 py-3 font-medium text-right">В корзину</th>
                <th className="px-4 py-3 font-medium text-right">Конверсия</th>
                <th className="pl-4 py-3 font-medium text-right border-l border-transparent">Возвраты</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {loading ? (
                <tr>
                  <td colSpan={8} className="py-20">
                    <div className="flex flex-col items-center">
                      <div className="w-5 h-5 border-2 border-gray-900 border-t-transparent rounded-full animate-spin mb-3"></div>
                      <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-500">Загрузка товаров</span>
                    </div>
                  </td>
                </tr>
              ) : error ? (
                <tr>
                  <td colSpan={8} className="py-20 text-center">
                    <div className="text-red-600 mb-4 font-medium">{error}</div>
                    <button
                      onClick={() => setReloadKey(k => k + 1)}
                      className="px-4 py-2 border border-gray-900 text-[11px] font-bold uppercase tracking-widest hover:bg-gray-50 transition-colors"
                    >
                      Повторить
                    </button>
                  </td>
                </tr>
              ) : isEmpty ? (
                <tr>
                  <td colSpan={8} className="py-24 text-center">
                    <div className="text-[11px] uppercase tracking-widest font-medium text-gray-900 mb-2">Нет товаров</div>
                    <div className="text-[11px] text-gray-500 mb-6 max-w-sm mx-auto leading-relaxed">
                      За выбранный период нет товаров с аналитическими событиями или продажами, соответствующих фильтрам.
                    </div>
                    <button
                      onClick={() => { setSearch(''); setSearchInput(''); setCategoryId(''); setDesignerId(''); }}
                      className="px-4 py-2 border border-gray-900 text-[11px] font-bold uppercase tracking-widest hover:bg-gray-50 transition-colors"
                    >
                      Сбросить фильтры
                    </button>
                  </td>
                </tr>
              ) : (
                products.map((p, index) => (
                  <tr key={p.productId} className="group hover:bg-gray-50/50 transition-colors">
                    <td className="py-3 pr-4">
                      <div className="flex items-center gap-4">
                        <div className="text-[10px] font-medium text-gray-300 w-4 text-right tabular-nums select-none">
                          {String(index + 1).padStart(2, '0')}
                        </div>
                        <div className="w-10 h-14 flex-shrink-0 bg-gray-50 overflow-hidden rounded-sm">
                          {p.primaryImage ? (
                            <img src={p.primaryImage} alt="" className="h-full w-full object-cover mix-blend-multiply" />
                          ) : (
                            <div className="h-full w-full flex items-center justify-center text-gray-200">
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="square" strokeLinejoin="miter" strokeWidth="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"></path></svg>
                            </div>
                          )}
                        </div>
                        <div className="min-w-0 py-1 flex flex-col justify-center">
                          <div className="text-sm font-medium text-gray-900 truncate" title={p.productName}>
                            {p.productName}
                          </div>
                          <div className="text-[11px] text-gray-500 truncate mt-0.5">
                            <span>{formatDesignerName(p.designerName)}</span>
                            {p.categoryName ? (
                              <>
                                <span className="text-gray-300 mx-1.5">•</span>
                                <span>{p.categoryName}</span>
                              </>
                            ) : null}
                          </div>
                        </div>
                      </div>
                    </td>

                    <td className="px-4 py-3 text-right tabular-nums text-gray-900 font-medium border-l border-transparent">
                      {money(p.revenueCents)}
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums text-gray-900">
                      <div>{number(p.purchases)}</div>
                      {p.soldUnits > p.purchases && <div className="text-[10px] text-gray-400 mt-0.5">{number(p.soldUnits)} шт.</div>}
                    </td>

                    <td className="px-4 py-3 text-right tabular-nums text-gray-500 border-l border-transparent">
                      {renderBehavioralMetric(p.views, coverage?.views, 'просмотров')}
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums text-gray-500">
                      {renderBehavioralMetric(p.favorites, coverage?.favorites, 'избранного')}
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums text-gray-500">
                      {renderBehavioralMetric(p.addToCart, coverage?.addToCart, 'корзины')}
                    </td>

                    <td className="px-4 py-3 text-right tabular-nums text-gray-500">
                      {renderConversion(p)}
                    </td>
                    <td className="pl-4 py-3 text-right tabular-nums text-gray-900 border-l border-transparent">
                      {p.returns > 0 ? (
                        <div>
                          <span>{number(p.returns)}</span>
                          {p.returnedUnits > p.returns && <div className="text-[10px] text-gray-400 mt-0.5">{number(p.returnedUnits)} шт.</div>}
                        </div>
                      ) : p.returns === 0 ? (
                        <span className="text-gray-400 font-medium">0</span>
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
