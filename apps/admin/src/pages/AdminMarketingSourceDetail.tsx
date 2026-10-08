import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { MarketingPeriodControl, useMarketingRange } from '../components/marketing/MarketingPeriodControl';
import { getMarketingSourceDetail } from '../api/marketing';
import { SourceDetailResponse } from '../api/marketing';
import { TrendDataPoint } from '@zamk/api-client/src/types';
import { number, money, percent, conversion } from '../components/marketing/marketingPresentation';
import { formatSource } from '../utils/sourceFormatter';

const MONTH_SHORT = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек'];
const MONTH_FULL = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'];

function formatHumanDate(dateStr: string): string {
  const parts = dateStr.split('-');
  if (parts.length !== 3) return dateStr;
  const day = parseInt(parts[2], 10);
  const monthIdx = parseInt(parts[1], 10) - 1;
  return `${day} ${MONTH_SHORT[monthIdx] || ''}`;
}

function formatFullDate(dateStr: string): string {
  const parts = dateStr.split('-');
  if (parts.length !== 3) return dateStr;
  const day = parseInt(parts[2], 10);
  const monthIdx = parseInt(parts[1], 10) - 1;
  const year = parts[0];
  return `${day} ${MONTH_FULL[monthIdx] || ''} ${year}`;
}

function getRevenueTicks(maxCents: number): { max: number; ticks: number[] } {
  if (maxCents <= 0) return { max: 100000, ticks: [100000, 50000, 0] };
  const maxRub = maxCents / 100;
  const power = Math.pow(10, Math.floor(Math.log10(maxRub)));
  const normalized = maxRub / power;
  let step: number;
  if (normalized <= 2) step = 0.5 * power;
  else if (normalized <= 5) step = 1 * power;
  else step = 2 * power;
  const tickCount = 3;
  const niceMaxRub = Math.max(step * tickCount, Math.ceil(maxRub / (step * tickCount)) * step * tickCount);
  const tickStep = niceMaxRub / tickCount;
  return { max: niceMaxRub * 100, ticks: [niceMaxRub * 100, (niceMaxRub - tickStep) * 100, (niceMaxRub - tickStep * 2) * 100, 0] };
}

function getOrdersTicks(maxOrders: number): { max: number; ticks: number[] } {
  if (maxOrders <= 0) return { max: 4, ticks: [4, 2, 0] };
  if (maxOrders <= 4) {
    const ticks: number[] = [];
    for (let i = maxOrders; i >= 0; i--) ticks.push(i);
    return { max: maxOrders, ticks };
  }
  const step = Math.ceil(maxOrders / 3);
  const niceMax = step * 3;
  return { max: niceMax, ticks: [niceMax, step * 2, step, 0] };
}

function formatRevenueTick(cents: number): string {
  if (cents === 0) return '0 ₽';
  const rub = cents / 100;
  if (rub >= 1_000_000) {
    const m = rub / 1_000_000;
    return `${m % 1 === 0 ? m : m.toFixed(1)} млн ₽`;
  }
  if (rub >= 10_000) return `${Math.round(rub / 1_000)} тыс. ₽`;
  if (rub >= 1_000) return `${(rub / 1_000).toFixed(rub % 1_000 === 0 ? 0 : 1)} тыс. ₽`;
  return `${rub} ₽`;
}

function getXAxisTicks(data: TrendDataPoint[]): { index: number; label: string; date: string }[] {
  if (data.length === 0) return [];
  if (data.length <= 7) return data.map((d, i) => ({ index: i, label: formatHumanDate(d.date), date: d.date }));
  const count = Math.min(5, data.length);
  const step = (data.length - 1) / (count - 1);
  const ticks: { index: number; label: string; date: string }[] = [];
  for (let i = 0; i < count; i++) {
    const idx = Math.round(i * step);
    ticks.push({ index: idx, label: formatHumanDate(data[idx].date), date: data[idx].date });
  }
  return ticks;
}

function TrendChart({ data }: { data?: TrendDataPoint[] }) {
  const [metric, setMetric] = useState<'revenue' | 'orders'>('revenue');
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null);

  if (!data || data.length === 0) {
    return <div className="h-56 flex items-center justify-center text-[10px] text-gray-400 uppercase tracking-widest font-semibold">Нет данных для графика</div>;
  }

  const isRevenue = metric === 'revenue';
  const maxRevenue = Math.max(...data.map(d => d.revenueCents), 0);
  const maxOrders = Math.max(...data.map(d => d.paidOrders), 0);
  const isAllZero = isRevenue ? maxRevenue === 0 : maxOrders === 0;

  const { max: scaleMax, ticks } = isRevenue ? getRevenueTicks(maxRevenue) : getOrdersTicks(maxOrders);
  const xTicks = getXAxisTicks(data);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div className="flex items-baseline gap-3">
          <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900 flex items-center gap-1.5">Динамика<span className="sr-only">выручки и заказов</span></h2>
          <span className="text-[11px] text-gray-400 font-medium hidden sm:inline">{isRevenue ? 'Выручка по дням' : 'Заказы по дням'}</span>
        </div>
        <div role="group" aria-label="Метрика графика" className="inline-flex rounded-lg bg-gray-100 p-0.5">
          <button type="button" onClick={() => setMetric('revenue')} aria-pressed={metric === 'revenue'} className={`px-3 py-1 text-[11px] font-semibold rounded-md transition-all cursor-pointer ${metric === 'revenue' ? 'bg-white text-gray-900 shadow-xs' : 'text-gray-500 hover:text-gray-900'}`}>Выручка</button>
          <button type="button" onClick={() => setMetric('orders')} aria-pressed={metric === 'orders'} className={`px-3 py-1 text-[11px] font-semibold rounded-md transition-all cursor-pointer ${metric === 'orders' ? 'bg-white text-gray-900 shadow-xs' : 'text-gray-500 hover:text-gray-900'}`}>Заказы</button>
        </div>
      </div>
      <div className="relative pt-2">
        <div className="flex">
          <div className="w-16 sm:w-20 pr-3 flex flex-col justify-between items-end text-[10px] text-gray-400 font-medium tabular-nums select-none h-44 pb-1">
            {ticks.map((t, i) => <span key={i} className="leading-none">{isRevenue ? formatRevenueTick(t) : t}</span>)}
          </div>
          <div className="flex-1 relative h-44 border-b border-gray-200">
            <div className="absolute inset-0 flex flex-col justify-between pointer-events-none pb-0.5">
              {ticks.map((_, i) => <div key={i} className={`w-full border-t ${i === ticks.length - 1 ? 'border-transparent' : 'border-gray-100'}`} />)}
            </div>
            {isAllZero && (
              <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <span className="text-[11px] font-semibold uppercase tracking-widest text-gray-400 bg-white/80 px-3 py-1 rounded">
                  {isRevenue ? 'Нет выручки за выбранный период' : 'Нет заказов за выбранный период'}
                </span>
              </div>
            )}
            <div className="absolute inset-0 flex items-end justify-between gap-1 sm:gap-1.5 px-1 pb-0.5">
              {data.map((d, i) => {
                const val = isRevenue ? d.revenueCents : d.paidOrders;
                const heightPct = scaleMax > 0 ? (val / scaleMax) * 100 : 0;
                const isHovered = hoveredIdx === i;
                return (
                  <div key={d.date} className="flex-1 max-w-[20px] min-w-[3px] h-full flex flex-col justify-end items-center relative group/bar cursor-pointer" onMouseEnter={() => setHoveredIdx(i)} onMouseLeave={() => setHoveredIdx(null)}>
                    <div style={{ height: `${val > 0 ? Math.max(heightPct, 2) : 0}%` }} className={`w-full transition-all rounded-xs ${isHovered ? 'bg-gray-900 shadow-xs' : 'bg-gray-700/80 hover:bg-gray-900'}`} />
                    <span className="sr-only">{d.date}: {money(d.revenueCents)}, {d.paidOrders} заказов</span>
                  </div>
                );
              })}
            </div>
            {hoveredIdx !== null && data[hoveredIdx] && (
              <div className="absolute z-20 pointer-events-none bg-gray-900 text-white rounded-lg px-3 py-2 text-xs shadow-xl border border-gray-800 transition-all duration-75" style={{ left: `${(hoveredIdx / (data.length - 1 || 1)) * 100}%`, top: '0px', transform: 'translate(-50%, -105%)' }}>
                <div className="text-[10px] text-gray-400 font-medium mb-1 pb-1 border-b border-gray-800 whitespace-nowrap">{formatFullDate(data[hoveredIdx].date)}</div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between gap-4"><span className="text-gray-400 text-[10px]">Выручка</span><span className="font-semibold text-white tabular-nums">{money(data[hoveredIdx].revenueCents)}</span></div>
                  <div className="flex items-center justify-between gap-4"><span className="text-gray-400 text-[10px]">Заказы</span><span className="font-medium text-gray-200 tabular-nums">{number(data[hoveredIdx].paidOrders)}</span></div>
                </div>
              </div>
            )}
          </div>
        </div>
        <div className="flex">
          <div className="w-16 sm:w-20 pr-3 flex-shrink-0" />
          <div className="flex-1 relative h-6 mt-2">
            {xTicks.map(tick => {
              const leftPct = (tick.index / (data.length - 1 || 1)) * 100;
              let alignClass = '-translate-x-1/2 text-center';
              if (tick.index === 0) alignClass = 'translate-x-0 text-left';
              else if (tick.index === data.length - 1) alignClass = '-translate-x-full text-right';
              return (
                <div key={tick.date} className={`absolute top-0 text-[10px] text-gray-400 uppercase tracking-wider font-medium whitespace-nowrap select-none ${alignClass}`} style={{ left: `${leftPct}%` }}>
                  <span>{tick.label}</span><span className="sr-only">{tick.date}</span>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

function UnavailableMetric({ tooltip = 'Недостаточно данных' }: { tooltip?: string }) {
  return <span className="text-gray-300 font-normal cursor-help select-none" title={tooltip}>—</span>;
}

export function AdminMarketingSourceDetail() {
  const { source } = useParams<{ source: string }>();
  const { period, range, select } = useMarketingRange();

  const [snapshot, setSnapshot] = useState<SourceDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    if (!source) return;

    getMarketingSourceDetail(source, range.from, range.to)
      .then(res => {
        setSnapshot(res);
        setLoading(false);
      })
      .catch(err => {
        setError(err.message || 'Ошибка загрузки источника');
        setLoading(false);
      });
  }, [source, range]);

  if (loading) {
    return (
      <div className="p-8 max-w-6xl mx-auto min-h-screen flex items-center justify-center">
        <div className="flex flex-col items-center">
          <div className="w-5 h-5 border-2 border-gray-900 border-t-transparent rounded-full animate-spin mb-3"></div>
          <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-500">Загрузка данных</span>
        </div>
      </div>
    );
  }

  if (error || !snapshot) {
    return (
      <div className="p-8 max-w-6xl mx-auto min-h-screen flex items-center justify-center flex-col gap-4">
        <p className="text-red-700 font-medium">{error || 'Источник не найден'}</p>
        <Link to="/marketing/sources" className="text-[11px] uppercase tracking-widest font-bold underline">К списку источников</Link>
      </div>
    );
  }

  const { source: stats, coverage, trend, topProducts, topDesigners, campaigns } = snapshot;
  const name = formatSource(stats.sourceKind, stats.sourceKey ?? stats.source ?? null);

  const hasPartial = Object.values(coverage).some((c: any) => c.status === 'partial');

  function renderConversion(s: { visits: number, paidOrders: number, conversionRate?: number }) {
    if (coverage?.views?.status !== 'available') return <UnavailableMetric />;
    if (s.visits === 0 || s.paidOrders > s.visits) return <UnavailableMetric />;
    const bps = (s as any).conversionRateBps ?? (s.conversionRate && s.conversionRate > 1 ? s.conversionRate : Math.round((s.conversionRate ?? 0) * 10000));
    return <span className="text-gray-900 font-medium">{percent(bps || conversion(s.paidOrders, s.visits))}</span>;
  }

  return (
    <div className="p-8 max-w-6xl mx-auto min-h-screen">
      <div className="mb-6 flex items-center text-[10px] font-bold uppercase tracking-widest text-gray-500">
        <Link to="/marketing/overview" className="hover:text-gray-900 transition-colors">Маркетинг</Link>
        <span className="mx-2">/</span>
        <Link to="/marketing/sources" className="hover:text-gray-900 transition-colors">Источники</Link>
        <span className="mx-2">/</span>
        <span className="text-gray-900">{name}</span>
      </div>

      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-serif text-gray-900 tracking-tight flex items-center gap-3">
            <div className="w-10 h-10 rounded bg-gray-100 flex items-center justify-center flex-shrink-0 text-xs font-bold text-gray-400">
              {name.substring(0, 1).toUpperCase()}
            </div>
            {name}
          </h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-2">Аналитика по источнику</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-6">
          <MarketingPeriodControl period={period} range={range} onSelect={select} />
        </div>
      </div>

      {hasPartial && (
        <div className="flex items-center gap-2 mb-6 text-[11px] text-gray-500 bg-gray-50 py-2 px-3 inline-flex rounded">
          <span className="w-1.5 h-1.5 rounded-full bg-gray-400 inline-block"></span>
          <span>Поведенческие данные собраны не за весь период</span>
        </div>
      )}

      {/* Main KPI cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-px bg-gray-200 border border-gray-200 mb-8">
        <div className="bg-white p-6">
          <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Сессии</div>
          <div className="text-2xl font-light text-gray-900">{number(stats.visits)}</div>
        </div>
        <div className="bg-white p-6">
          <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Заказы</div>
          <div className="text-2xl font-light text-gray-900">{number(stats.paidOrders)}</div>
        </div>
        <div className="bg-white p-6">
          <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Конверсия</div>
          <div className="text-2xl font-light text-gray-900">{renderConversion(stats)}</div>
        </div>
        <div className="bg-white p-6">
          <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Выручка</div>
          <div className="text-2xl font-light text-gray-900">{money(stats.revenueCents)}</div>
        </div>
      </div>

      {/* Chart Section */}
      <div className="p-8 border border-gray-200 bg-white relative mb-12">
        <TrendChart data={trend} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-12">
        {/* Top Campaigns in this source */}
        <div>
          <div className="flex justify-between items-end border-b border-gray-900 pb-3 mb-5">
            <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900">Кампании ({name})</h2>
            <span className="text-[10px] font-bold uppercase tracking-widest text-gray-900 text-right">Выручка</span>
          </div>
          {campaigns.length > 0 ? (
            <div className="space-y-4">
              {campaigns.map((c: any, i: number) => (
                <div key={c.campaignId ?? `no-camp-${i}`} className="flex justify-between items-baseline text-sm border-b border-gray-100 pb-4 last:border-0">
                  <div className="flex-1 truncate pr-4">
                     <div className="font-medium">
                       {c.campaignId ? <Link to={`/marketing/campaigns/${c.campaignId}`} className="text-gray-900 hover:text-gray-600 transition-colors">{c.name}</Link> : <span className="text-gray-900">{c.name}</span>}
                     </div>
                     <div className="text-[11px] text-gray-500 mt-1">
                       <span>{number(c.visits)}</span> сессий · <span>{number(c.paidOrders)}</span> заказов
                     </div>
                  </div>
                  <div className="flex gap-6 items-baseline text-right">
                     <div className="text-gray-900 font-medium tabular-nums">{money(c.revenueCents)}</div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-8 text-[11px] uppercase tracking-widest font-medium text-gray-400 text-center">
              Нет кампаний в этом источнике
            </div>
          )}
        </div>

        {/* Top Designers in this source */}
        <div>
          <div className="flex justify-between items-end border-b border-gray-900 pb-3 mb-5">
            <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900">Популярные дизайнеры</h2>
            <span className="text-[10px] font-bold uppercase tracking-widest text-gray-900 text-right">Покупки</span>
          </div>
          {topDesigners.length > 0 ? (
            <div className="space-y-4">
              {topDesigners.map((d: any, i: number) => (
                <div key={d.designerId ?? i} className="flex justify-between items-center text-sm border-b border-gray-100 pb-4 last:border-0 group">
                  <div className="flex items-center gap-3">
                    {d.primaryImage ? (
                      <div className="w-8 h-10 bg-gray-100 flex-shrink-0 overflow-hidden">
                        <img src={d.primaryImage} alt="" className="w-full h-full object-cover grayscale mix-blend-multiply group-hover:grayscale-0 group-hover:mix-blend-normal transition-all duration-500" />
                      </div>
                    ) : (
                      <div className="w-8 h-10 bg-gray-50 border border-gray-100 flex-shrink-0"></div>
                    )}
                    <div className="font-medium text-gray-900">{d.designerName}</div>
                  </div>
                  <div className="text-gray-900 tabular-nums font-medium">
                    {number(d.purchases)}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="py-8 text-[11px] uppercase tracking-widest font-medium text-gray-400 text-center">
              Нет данных о дизайнерах
            </div>
          )}
        </div>
      </div>

      {/* Top Products */}
      <div className="mt-12 pt-12 border-t border-gray-200">
        <div className="flex justify-between items-end border-b border-gray-900 pb-3 mb-5">
          <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900">Популярные товары</h2>
          <span className="text-[10px] font-bold uppercase tracking-widest text-gray-900 text-right">Покупки</span>
        </div>
        {topProducts.length > 0 ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {topProducts.map((p: any, i: number) => (
              <div key={p.productId ?? i} className="flex gap-4 group">
                <div className="w-16 h-20 bg-gray-100 flex-shrink-0 overflow-hidden relative">
                  {p.primaryImage ? (
                    <img src={p.primaryImage} alt="" className="w-full h-full object-cover mix-blend-multiply grayscale group-hover:grayscale-0 group-hover:mix-blend-normal transition-all duration-500" />
                  ) : null}
                </div>
                <div className="flex flex-col justify-center">
                  <div className="font-medium text-gray-900 text-sm leading-snug line-clamp-2 mb-1">{p.name}</div>
                  <div className="text-[10px] uppercase tracking-widest text-gray-400 font-bold">{p.designerName}</div>
                  <div className="text-gray-900 text-sm mt-2 tabular-nums font-medium">
                    {number(p.purchases)} шт
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="py-8 text-[11px] uppercase tracking-widest font-medium text-gray-400 text-center">
            Нет данных о товарах
          </div>
        )}
      </div>
    </div>
  );
}
