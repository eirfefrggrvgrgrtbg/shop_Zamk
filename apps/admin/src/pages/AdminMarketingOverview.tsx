import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { getMarketingOverview, getMarketingSources, getMarketingCampaigns, getMarketingTrend } from '../api/marketing';
import type { AnalyticsOverviewResponse, SourceMetrics, CampaignMetrics } from '../api/marketing';
import type { TrendDataPoint } from '@zamk/api-client/src/types';
import { AdminMarketingTabs } from '../components/marketing/AdminMarketingTabs';
import { MarketingPeriodControl, useMarketingRange, type MarketingRange } from '../components/marketing/MarketingPeriodControl';
import { finite, number, money, percent, conversion } from '../components/marketing/marketingPresentation';
import { formatSource } from '../utils/sourceFormatter';

type Snapshot = { overview: AnalyticsOverviewResponse; sources: SourceMetrics[]; campaigns: CampaignMetrics[]; range: MarketingRange };

function pluralize(count: number, forms: [string, string, string]) {
  const m10 = count % 10;
  const m100 = count % 100;
  if (m10 === 1 && m100 !== 11) return forms[0];
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return forms[1];
  return forms[2];
}

function renderComparison(current: number | undefined | null, previous: number | undefined | null, inverseGood = false) {
  if (!finite(current) || !finite(previous) || previous === 0) return null;
  const diff = current - previous;
  if (diff === 0) return <div className="text-[11px] text-gray-400 mt-2 tracking-wide font-medium">БЕЗ ИЗМЕНЕНИЙ</div>;
  const diffPercent = (diff / previous) * 100;
  const isGood = diff > 0 ? !inverseGood : inverseGood;
  const color = isGood ? 'text-emerald-600' : 'text-rose-600';
  const prefix = diff > 0 ? '+' : '';

  return (
    <div className={`text-[11px] ${color} mt-2 font-medium tracking-wide flex items-baseline gap-1.5`}>
      <span>{prefix}{diffPercent.toFixed(1)}%</span>
      <span className="text-gray-400 font-normal">vs пред. период</span>
    </div>
  );
}

const MONTH_SHORT = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек'];
const MONTH_FULL = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'
];

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

  const ticks = [
    niceMaxRub * 100,
    (niceMaxRub - tickStep) * 100,
    (niceMaxRub - tickStep * 2) * 100,
    0
  ];
  return { max: niceMaxRub * 100, ticks };
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
  return {
    max: niceMax,
    ticks: [niceMax, step * 2, step, 0]
  };
}

function formatRevenueTick(cents: number): string {
  if (cents === 0) return '0 ₽';
  const rub = cents / 100;
  if (rub >= 1_000_000) {
    const m = rub / 1_000_000;
    return `${m % 1 === 0 ? m : m.toFixed(1)} млн ₽`;
  }
  if (rub >= 10_000) {
    return `${Math.round(rub / 1_000)} тыс. ₽`;
  }
  if (rub >= 1_000) {
    return `${(rub / 1_000).toFixed(rub % 1_000 === 0 ? 0 : 1)} тыс. ₽`;
  }
  return `${rub} ₽`;
}

function getXAxisTicks(data: TrendDataPoint[]): { index: number; label: string; date: string }[] {
  if (data.length === 0) return [];
  if (data.length <= 7) {
    return data.map((d, i) => ({ index: i, label: formatHumanDate(d.date), date: d.date }));
  }
  const count = Math.min(5, data.length);
  const step = (data.length - 1) / (count - 1);
  const ticks: { index: number; label: string; date: string }[] = [];
  for (let i = 0; i < count; i++) {
    const idx = Math.round(i * step);
    ticks.push({
      index: idx,
      label: formatHumanDate(data[idx].date),
      date: data[idx].date,
    });
  }
  return ticks;
}

function TrendChart({ data }: { data?: TrendDataPoint[] }) {
  const [metric, setMetric] = useState<'revenue' | 'orders'>('revenue');
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null);

  if (!data || data.length === 0) {
    return (
      <div className="h-56 flex items-center justify-center text-[10px] text-gray-400 uppercase tracking-widest font-semibold">
        Нет данных для графика
      </div>
    );
  }

  const isRevenue = metric === 'revenue';
  const maxRevenue = Math.max(...data.map(d => d.revenueCents), 0);
  const maxOrders = Math.max(...data.map(d => d.paidOrders), 0);

  const isAllZero = isRevenue ? maxRevenue === 0 : maxOrders === 0;

  const { max: scaleMax, ticks } = isRevenue
    ? getRevenueTicks(maxRevenue)
    : getOrdersTicks(maxOrders);

  const xTicks = getXAxisTicks(data);

  return (
    <div className="flex flex-col gap-4">
      {/* Header with Title and Segmented Switcher */}
      <div className="flex items-center justify-between">
        <div className="flex items-baseline gap-3">
          <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900 flex items-center gap-1.5">
            Динамика
            <span className="sr-only">выручки и заказов</span>
          </h2>
          <span className="text-[11px] text-gray-400 font-medium hidden sm:inline">
            {isRevenue ? 'Выручка по дням' : 'Заказы по дням'}
          </span>
        </div>

        {/* Segmented Metric Selector */}
        <div
          role="group"
          aria-label="Метрика графика"
          className="inline-flex rounded-lg bg-gray-100 p-0.5"
        >
          <button
            type="button"
            onClick={() => setMetric('revenue')}
            aria-pressed={metric === 'revenue'}
            className={`px-3 py-1 text-[11px] font-semibold rounded-md transition-all cursor-pointer ${
              metric === 'revenue'
                ? 'bg-white text-gray-900 shadow-xs'
                : 'text-gray-500 hover:text-gray-900'
            }`}
          >
            Выручка
          </button>
          <button
            type="button"
            onClick={() => setMetric('orders')}
            aria-pressed={metric === 'orders'}
            className={`px-3 py-1 text-[11px] font-semibold rounded-md transition-all cursor-pointer ${
              metric === 'orders'
                ? 'bg-white text-gray-900 shadow-xs'
                : 'text-gray-500 hover:text-gray-900'
            }`}
          >
            Заказы
          </button>
        </div>
      </div>

      {/* Main Chart Area with Y-axis gutter and Plot Surface */}
      <div className="relative pt-2">
        <div className="flex">
          {/* Y Axis Labels */}
          <div className="w-16 sm:w-20 pr-3 flex flex-col justify-between items-end text-[10px] text-gray-400 font-medium tabular-nums select-none h-44 pb-1">
            {ticks.map((t, i) => (
              <span key={i} className="leading-none">
                {isRevenue ? formatRevenueTick(t) : t}
              </span>
            ))}
          </div>

          {/* Plotting Frame */}
          <div className="flex-1 relative h-44 border-b border-gray-200">
            {/* Horizontal Grid lines matching ticks */}
            <div className="absolute inset-0 flex flex-col justify-between pointer-events-none pb-0.5">
              {ticks.map((_, i) => (
                <div
                  key={i}
                  className={`w-full border-t ${
                    i === ticks.length - 1 ? 'border-transparent' : 'border-gray-100'
                  }`}
                />
              ))}
            </div>

            {/* Zero State Overlay */}
            {isAllZero ? (
              <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <span className="text-[11px] font-semibold uppercase tracking-widest text-gray-400 bg-white/80 px-3 py-1 rounded">
                  {isRevenue ? 'Нет выручки за выбранный период' : 'Нет заказов за выбранный период'}
                </span>
              </div>
            ) : null}

            {/* Bars Container */}
            <div className="absolute inset-0 flex items-end justify-between gap-1 sm:gap-1.5 px-1 pb-0.5">
              {data.map((d, i) => {
                const val = isRevenue ? d.revenueCents : d.paidOrders;
                const heightPct = scaleMax > 0 ? (val / scaleMax) * 100 : 0;
                const isHovered = hoveredIdx === i;

                return (
                  <div
                    key={d.date}
                    className="flex-1 max-w-[20px] min-w-[3px] h-full flex flex-col justify-end items-center relative group/bar cursor-pointer"
                    onMouseEnter={() => setHoveredIdx(i)}
                    onMouseLeave={() => setHoveredIdx(null)}
                  >
                    {/* The Bar */}
                    <div
                      style={{ height: `${val > 0 ? Math.max(heightPct, 2) : 0}%` }}
                      className={`w-full transition-all rounded-xs ${
                        isHovered ? 'bg-gray-900 shadow-xs' : 'bg-gray-700/80 hover:bg-gray-900'
                      }`}
                    />

                    {/* Accessible Title */}
                    <span className="sr-only">
                      {d.date}: {money(d.revenueCents)}, {d.paidOrders} заказов
                    </span>
                  </div>
                );
              })}
            </div>

            {/* Rich Hover Tooltip */}
            {hoveredIdx !== null && data[hoveredIdx] && (
              <div
                className="absolute z-20 pointer-events-none bg-gray-900 text-white rounded-lg px-3 py-2 text-xs shadow-xl border border-gray-800 transition-all duration-75"
                style={{
                  left: `${(hoveredIdx / (data.length - 1 || 1)) * 100}%`,
                  top: '0px',
                  transform: 'translate(-50%, -105%)',
                }}
              >
                <div className="text-[10px] text-gray-400 font-medium mb-1 pb-1 border-b border-gray-800 whitespace-nowrap">
                  {formatFullDate(data[hoveredIdx].date)}
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between gap-4">
                    <span className="text-gray-400 text-[10px]">Выручка</span>
                    <span className="font-semibold text-white tabular-nums">
                      {money(data[hoveredIdx].revenueCents)}
                    </span>
                  </div>
                  <div className="flex items-center justify-between gap-4">
                    <span className="text-gray-400 text-[10px]">Заказы</span>
                    <span className="font-medium text-gray-200 tabular-nums">
                      {number(data[hoveredIdx].paidOrders)}
                    </span>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* X Axis Labels */}
        <div className="flex">
          <div className="w-16 sm:w-20 pr-3 flex-shrink-0" />
          <div className="flex-1 relative h-6 mt-2">
            {xTicks.map(tick => {
              const leftPct = (tick.index / (data.length - 1 || 1)) * 100;
              let alignClass = '-translate-x-1/2 text-center';
              if (tick.index === 0) alignClass = 'translate-x-0 text-left';
              else if (tick.index === data.length - 1) alignClass = '-translate-x-full text-right';

              return (
                <div
                  key={tick.date}
                  className={`absolute top-0 text-[10px] text-gray-400 uppercase tracking-wider font-medium whitespace-nowrap select-none ${alignClass}`}
                  style={{ left: `${leftPct}%` }}
                >
                  <span>{tick.label}</span>
                  <span className="sr-only">{tick.date}</span>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

export function AdminMarketingOverview() {
  const { period, range, select } = useMarketingRange();
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);

  const [trendData, setTrendData] = useState<TrendDataPoint[]>([]);
  const [trendError, setTrendError] = useState(false);
  const [trendRetry, setTrendRetry] = useState(0);

  useEffect(() => {
    let active = true;
    const fetchCore = async () => {
      setLoading(true);
      setError(false);
      try {
        const [overview, sourcesRes, campaignsRes] = await Promise.all([
          getMarketingOverview(range.from, range.to),
          getMarketingSources(range.from, range.to),
          getMarketingCampaigns(range.from, range.to)
        ]);
        if (!overview) throw new Error('overview is null');
        if (active) {
          setSnapshot({
            overview,
            sources: Array.isArray(sourcesRes) ? sourcesRes : sourcesRes?.sources || [],
            campaigns: Array.isArray(campaignsRes) ? campaignsRes : campaignsRes?.campaigns || [],
            range
          });
          setLoading(false);
        }
      } catch (err) {
        if (active) {
          setError(true);
          setLoading(false);
        }
      }
    };
    fetchCore();
    return () => { active = false; };
  }, [range, retry]);

  useEffect(() => {
    let active = true;
    const fetchTrend = async () => {
      setTrendError(false);
      try {
        const data = await getMarketingTrend(range.from, range.to);
        if (active) setTrendData(Array.isArray(data) ? data : data?.trend || []);
      } catch (err) {
        if (active) setTrendError(true);
      }
    };
    fetchTrend();
    return () => { active = false; };
  }, [range, trendRetry]);

  if (!snapshot && loading) {
    return (
      <div className="p-8 max-w-6xl mx-auto flex items-center justify-center min-h-[60vh]" data-testid="marketing-hero">
        <div data-testid="overview-loading" className="flex flex-col items-center">
           <div className="w-5 h-5 border-2 border-gray-900 border-t-transparent rounded-full animate-spin mb-3"></div>
           <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-500">Загрузка данных</span>
        </div>
      </div>
    );
  }

  if (!snapshot && error) {
    return (
      <div className="p-8 max-w-6xl mx-auto flex flex-col items-center justify-center min-h-[60vh] gap-4" data-testid="marketing-hero">
        <div role="alert" className="text-gray-900 font-medium">Не удалось обновить данные</div>
        <button onClick={() => setRetry(v => v + 1)} className="px-4 py-2 border border-gray-200 rounded text-[11px] font-bold uppercase tracking-widest hover:bg-gray-50 transition-colors">
          Повторить
        </button>
        <div role="region" aria-label="Показатели источников" className="hidden"></div>
      </div>
    );
  }

  const { overview: { current, previous }, sources, campaigns } = snapshot!;
  const totalVisits = current.visits;
  const isAnomalousConversion = (current.paidOrders > 0 && current.visits === 0) || (current.visits > 0 && current.paidOrders > current.visits);

  const attributedCampaigns = campaigns.filter(c => c.campaignId && c.name !== 'Без кампании');
  const unattributedStats = campaigns.find(c => !c.campaignId || c.name === 'Без кампании');

  return (
    <div className="p-8 max-w-6xl mx-auto min-h-screen" data-testid="marketing-hero">
      {error && (
        <div role="alert" className="mb-6 p-4 bg-red-50 text-red-700 rounded text-sm flex justify-between items-center">
          <span>Не удалось обновить данные</span>
          <button onClick={() => setRetry(v => v + 1)} className="underline font-medium">Повторить</button>
        </div>
      )}

      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-serif text-gray-900 tracking-tight">Маркетинг</h1>
          <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 mt-2">Трафик, продажи и эффективность</p>
        </div>
        <div className="flex items-center justify-between md:justify-end flex-wrap gap-6">
          {loading && <div data-testid="overview-loading" className="text-[10px] uppercase tracking-widest font-semibold text-gray-400 animate-pulse">Обновляем...</div>}
          <MarketingPeriodControl period={period} range={range} onSelect={select} />
        </div>
      </div>

      <div className="mb-10">
        <AdminMarketingTabs />
      </div>

      <div className="bg-white border-y border-gray-200 mb-12">
        <div className="grid grid-cols-1 md:grid-cols-5 divide-y md:divide-y-0 md:divide-x divide-gray-200">

          {/* Revenue */}
          <div className="p-8 md:col-span-2 flex flex-col justify-center">
            <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-3">Выручка</div>
            <div className="text-4xl font-light tracking-tight text-gray-900" data-testid="marketing-revenue">{money(current.revenueCents)}</div>
            {renderComparison(current.revenueCents, previous.revenueCents)}
          </div>

          {/* Orders & AOV */}
          <div className="p-8 flex flex-col justify-between">
            <div>
              <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Заказы</div>
              <div className="text-2xl font-light text-gray-900">{number(current.paidOrders)}</div>
              {renderComparison(current.paidOrders, previous.paidOrders)}
            </div>
            <div className="mt-8 pt-6 border-t border-gray-100">
              <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Средний чек</div>
              <div className="text-lg font-medium text-gray-900">{money(current.aovCents)}</div>
              {renderComparison(current.aovCents, previous.aovCents)}
            </div>
          </div>

          {/* Sessions & Conversion */}
          <div className="p-8 flex flex-col justify-between">
            <div>
              <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Сессии</div>
              <div className="text-2xl font-light text-gray-900">{number(current.visits)}</div>
              {renderComparison(current.visits, previous.visits)}
            </div>
            <div className="mt-8 pt-6 border-t border-gray-100">
              <div className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 mb-2">Конверсия</div>
              {isAnomalousConversion ? (
                <div data-testid="anomalous-conversion-notice" role="status" aria-label="Предупреждение о недостаточности данных конверсии">
                  <div className="text-2xl font-light text-gray-300">—</div>
                  <div className="text-[10px] font-semibold tracking-wider text-amber-800 bg-amber-50 px-2 py-1 inline-block mt-2">
                    Недостаточно данных
                  </div>
                  <div className="text-[10px] text-gray-500 mt-2 leading-tight">
                    <span className="sr-only">Покрытие аналитики низкое</span>
                    {number(current.visits)} {pluralize(current.visits, ['сессия', 'сессии', 'сессий'])} на {number(current.paidOrders)} {pluralize(current.paidOrders, ['оплаченный заказ', 'оплаченных заказа', 'оплаченных заказов'])}
                    <div className="text-gray-400 mt-0.5">Недостаточно данных для достоверной конверсии</div>
                  </div>
                </div>
              ) : (
                <>
                  <div className="text-lg font-medium text-gray-900">{percent(current.conversionRateBps)}</div>
                  {renderComparison(current.conversionRateBps, previous.conversionRateBps)}
                </>
              )}
            </div>
          </div>

          {/* Audience & Quality */}
          <div className="p-8 flex flex-col justify-center bg-gray-50/50 relative">
            <h2 className="sr-only">Качество продаж</h2>
            <div className="space-y-4 w-full">
              <div className="flex justify-between items-baseline border-b border-gray-200 pb-2">
                <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-900">Возвраты</span>
                <span className="text-sm font-medium text-gray-900">{money(current.returnedAmountCents)}</span>
              </div>
              <div className="flex justify-between items-baseline border-b border-gray-200 pb-2">
                <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-900">Единиц</span>
                <span className="text-sm font-medium text-gray-900">{number(current.soldUnits)}</span>
              </div>
              <div className="flex justify-between items-baseline border-b border-gray-200 pb-2">
                <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-900">Новые</span>
                <span className="text-sm font-medium text-gray-900">{number(current.newCustomers)}</span>
              </div>
              <div className="flex justify-between items-baseline">
                <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-900">Повторные</span>
                <span className="text-sm font-medium text-gray-900">{number(current.repeatCustomers)}</span>
              </div>
            </div>
          </div>

        </div>

        {/* Chart Section */}
        <div className="p-8 border-t border-gray-200 bg-white relative">
          {trendError ? (
            <div className="w-full h-56 flex flex-col items-center justify-center gap-2" data-testid="trend-error-area">
              <span className="text-[10px] uppercase tracking-widest font-semibold text-gray-500">Не удалось загрузить динамику</span>
              <button onClick={() => setTrendRetry(v => v + 1)} className="text-[10px] uppercase tracking-widest font-semibold text-gray-900 underline">Повторить</button>
            </div>
          ) : (
            <TrendChart data={trendData} />
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-12 lg:gap-20">

        {/* Traffic Sources */}
        <div role="region" aria-label="Показатели источников">
          <div className="flex justify-between items-end border-b border-gray-900 pb-3 mb-5">
            <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900">Источники трафика</h2>
            <span className="text-[10px] font-bold uppercase tracking-widest text-gray-900 text-right">Сессии</span>
          </div>
          <div className="space-y-4">
            {[...sources].sort((a, b) => b.visits - a.visits).slice(0, 8).map((source, index) => {
              const rawShare = totalVisits > 0 && finite(source.visits) ? source.visits / totalVisits * 100 : 0;
              const share = finite(rawShare) ? Math.max(0, Math.min(100, rawShare)) : 0;
              const name = formatSource(source.sourceKind ?? '', source.sourceKey ?? source.source ?? null);
              const routeParam = source.sourceKind === 'unattributed' ? '_unattributed' : (source.sourceKey ?? source.source ?? '_unattributed');
              const linkPath = `/marketing/sources/${encodeURIComponent(routeParam)}`;

              return (
                <div key={`${source.sourceKey ?? 'unattributed'}-${index}`} role="row" className="group">
                  <div className="flex justify-between items-baseline text-sm mb-2">
                    <Link to={linkPath} className="text-gray-900 font-medium hover:text-gray-600 transition-colors">{name}</Link>
                    <span className="text-gray-900 tabular-nums">
                       {number(source.visits)} <span className="text-gray-400 text-[10px] font-medium ml-2 w-8 inline-block text-right">{Math.round(share)}%</span>
                    </span>
                  </div>
                  <div className="w-full bg-gray-100 h-[2px] overflow-hidden">
                    <div className="bg-gray-900 h-full transition-all" style={{ width: `${share}%` }}></div>
                  </div>
                </div>
              );
            })}
            {!sources.length && <p className="text-[11px] uppercase tracking-widest font-medium text-gray-400 text-center py-8">Нет данных за этот период</p>}
            {sources.length > 0 && (
              <div className="pt-2">
                <Link to="/marketing/sources" className="text-[10px] font-bold uppercase tracking-widest text-gray-900 hover:text-gray-500 transition-colors">
                  Все источники →
                </Link>
              </div>
            )}
          </div>
        </div>

        {/* Top Campaigns */}
        <div role="region" aria-label="Показатели кампаний">
          <div className="flex justify-between items-end border-b border-gray-900 pb-3 mb-5">
            <h2 className="text-[10px] font-bold uppercase tracking-widest text-gray-900">Лучшие кампании</h2>
            <div className="flex gap-6 items-baseline">
              <span className="sr-only">Сессии</span>
              <span className="sr-only">Заказы</span>
              <span className="text-[10px] font-bold uppercase tracking-widest text-gray-900 text-right">Выручка</span>
              <span className="text-[10px] font-bold uppercase tracking-widest text-gray-900 text-right w-12 hidden sm:inline-block">
                Конв.<span className="sr-only">Конверсия</span>
              </span>
            </div>
          </div>
          {attributedCampaigns.length > 0 ? (
            <div className="space-y-4">
              {attributedCampaigns.sort((a, b) => b.revenueCents - a.revenueCents).slice(0, 6).map((campaign, i) => (
                <div key={campaign.campaignId ?? `none-${i}`} className="flex justify-between items-baseline text-sm border-b border-gray-100 pb-4 last:border-0">
                  <div className="flex-1 truncate pr-4">
                     <div className="font-medium">
                       {campaign.campaignId ? <Link to={`/marketing/campaigns/${campaign.campaignId}`} className="text-gray-900 hover:text-gray-600 transition-colors">{campaign.name}</Link> : <span className="text-gray-900">{campaign.name}</span>}
                     </div>
                     <div className="text-[11px] text-gray-500 mt-1">
                       <span>{number(campaign.visits)}</span> сессий · <span>{number(campaign.paidOrders)}</span> заказов
                     </div>
                  </div>
                  <div className="flex gap-6 items-baseline text-right">
                     <div className="text-gray-900 font-medium tabular-nums">{money(campaign.revenueCents)}</div>
                     <div className="text-[11px] font-medium text-gray-400 tabular-nums w-12 hidden sm:inline-block">
                       <span>{campaign.visits > 0 && campaign.paidOrders > campaign.visits ? '—' : percent(campaign.conversionRateBps ?? conversion(campaign.paidOrders, campaign.visits))}</span>
                     </div>
                  </div>
                </div>
              ))}
              <div className="pt-2">
                <Link to="/marketing/campaigns" className="text-[10px] font-bold uppercase tracking-widest text-gray-900 hover:text-gray-500 transition-colors">
                  Все кампании →
                </Link>
              </div>
            </div>
          ) : (
            <div className="py-8">
              <div className="text-[11px] uppercase tracking-widest font-medium text-gray-900 mb-2">Нет данных за этот период</div>
              {unattributedStats && (
                <div className="text-[11px] text-gray-500 leading-relaxed">
                  {number(unattributedStats.visits)} сессий и {number(unattributedStats.paidOrders)} заказов атрибутированы как органика.
                </div>
              )}
            </div>
          )}
        </div>

      </div>
    </div>
  );
}
