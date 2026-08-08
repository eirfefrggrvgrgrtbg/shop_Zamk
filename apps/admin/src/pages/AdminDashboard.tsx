import { useEffect, useState } from 'react';
import {
  RefreshCw,
  AlertTriangle,
  AlertCircle,
  ChevronRight,
  ArrowUpRight,
  ArrowDownRight,
  Minus
} from 'lucide-react';
import { getDashboardSummary } from '@zamk/api-client';
import type { AdminDashboardSummary } from '@zamk/api-client';
import { Link } from 'react-router-dom';

const formatCents = (cents: number | null | undefined) => {
  if (cents == null || !Number.isFinite(cents) || isNaN(cents)) return '—';
  return (cents / 100).toLocaleString('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  });
};

// -------------------------------------------------------------
// Route Normalizer Helper
// -------------------------------------------------------------
function normalizeRoute(rawLink?: string, title?: string): { path: string; disabled: boolean } {
  if (title?.toLowerCase().includes('модераци') && title?.toLowerCase().includes('товар')) {
    return { path: '/moderation', disabled: false };
  }
  if (!rawLink) {
    if (title?.toLowerCase().includes('платеж')) return { path: '/payments', disabled: false };
    if (title?.toLowerCase().includes('заказ')) return { path: '/orders', disabled: false };
    if (title?.toLowerCase().includes('продавец')) return { path: '/sellers', disabled: false };
    if (title?.toLowerCase().includes('аукцион') || title?.toLowerCase().includes('лот')) return { path: '/auctions', disabled: false };
    if (title?.toLowerCase().includes('остаток')) return { path: '/inventory', disabled: false };
    return { path: '#', disabled: true };
  }

  let route = rawLink.replace(/^\/admin/, '');
  if (!route.startsWith('/')) route = '/' + route;

  if (route === '/inventory' && title?.toLowerCase().includes('модераци')) {
    route = '/moderation';
  }

  return { path: route, disabled: false };
}

// -------------------------------------------------------------
// Trend Helpers
// -------------------------------------------------------------
function calculateTrend(current: number | null | undefined, comparison: number | null | undefined, inverseDirection = false) {
  if (current == null || comparison == null || !Number.isFinite(current) || !Number.isFinite(comparison)) {
    return {
      percentage: null,
      direction: 'neutral' as const,
      severity: 'neutral' as const,
      formattedPercent: 'Недостаточно данных',
      hasComparison: false,
    };
  }

  if (comparison === 0 && current === 0) {
    return {
      percentage: 0,
      direction: 'neutral' as const,
      severity: 'neutral' as const,
      formattedPercent: 'Без изменений',
      hasComparison: true,
    };
  }
  
  if (comparison === 0 && current > 0) {
    return {
      percentage: null,
      direction: 'up' as const,
      severity: inverseDirection ? 'danger' as const : 'success' as const,
      formattedPercent: 'Рост относительно нулевого периода',
      hasComparison: true,
    };
  }

  if (current === 0 && comparison > 0) {
    return {
      percentage: -100,
      direction: 'down' as const,
      severity: inverseDirection ? 'success' as const : 'danger' as const,
      formattedPercent: '-100%',
      hasComparison: true,
    };
  }

  const percentage = ((current - comparison) / comparison) * 100;
  
  let direction: 'up'|'down'|'neutral' = 'neutral';
  if (percentage > 0.5) direction = 'up';
  else if (percentage < -0.5) direction = 'down';

  let severity: 'success'|'danger'|'neutral' = 'neutral';
  if (direction === 'up') {
    severity = inverseDirection ? 'danger' : 'success';
  } else if (direction === 'down') {
    severity = inverseDirection ? 'success' : 'danger';
  }

  let pctString = 'Без изменений';
  if (direction !== 'neutral') {
     const sign = percentage > 0 ? '+' : '';
     pctString = `${sign}${percentage.toFixed(1)}%`;
  }

  return {
    percentage,
    direction,
    severity,
    formattedPercent: pctString,
    hasComparison: true,
  };
}

interface TrendCardProps {
  title: string;
  value: string | number;
  trend: ReturnType<typeof calculateTrend>;
  periodText: string;
}

function TrendCard({ title, value, trend, periodText }: TrendCardProps) {
  let severityColor = 'text-gray-500';
  let bgColor = 'bg-gray-100 dark:bg-zinc-800/80';
  
  if (trend.severity === 'success') {
    severityColor = 'text-emerald-700 dark:text-emerald-400';
    bgColor = 'bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200/50 dark:border-emerald-900/30';
  } else if (trend.severity === 'danger') {
    severityColor = 'text-rose-700 dark:text-rose-400';
    bgColor = 'bg-rose-50 dark:bg-rose-950/40 border border-rose-200/50 dark:border-rose-900/30';
  }

  const Icon = trend.direction === 'up' ? ArrowUpRight : trend.direction === 'down' ? ArrowDownRight : Minus;

  return (
    <div className="bg-white dark:bg-zinc-900 rounded-xl p-6 border border-gray-200 dark:border-zinc-800 shadow-sm flex flex-col justify-between hover:border-gray-300 dark:hover:border-zinc-700 transition-colors">
      <span className="text-sm font-medium text-gray-500 dark:text-zinc-400 mb-2">{title}</span>
      <p className="text-3xl font-bold text-gray-900 dark:text-white mb-4">{value}</p>
      
      <div className="flex items-center gap-2 flex-wrap">
        <span className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold ${bgColor} ${severityColor}`}>
          {trend.hasComparison && <Icon className="w-4 h-4" />}
          {trend.formattedPercent}
        </span>
        <span className="text-xs text-gray-500 dark:text-zinc-500 leading-tight">{periodText}</span>
      </div>
    </div>
  );
}

// -------------------------------------------------------------
// Section Box with Rows
// -------------------------------------------------------------
interface SectionRow {
  label: string;
  value: string | number;
  highlight?: 'warning' | 'danger' | 'success';
}

interface SectionBoxProps {
  title: string;
  linkPath: string;
  linkText: string;
  rows: SectionRow[];
}

function SectionBox({ title, linkPath, linkText, rows }: SectionBoxProps) {
  return (
    <div className="bg-white dark:bg-zinc-900 rounded-xl p-5 border border-gray-200 dark:border-zinc-800 shadow-sm flex flex-col justify-between">
      <div>
        <h3 className="text-sm font-bold text-gray-900 dark:text-white uppercase tracking-wider mb-3 font-sans">
          {title}
        </h3>
        <div className="divide-y divide-gray-100 dark:divide-zinc-800/60 my-2">
          {rows.map((row, idx) => {
            const isZero = row.value === 0 || row.value === '0 ₽' || row.value === '0';
            
            let valueClass = 'text-gray-900 dark:text-gray-100 font-medium';
            let labelClass = 'text-gray-700 dark:text-zinc-300 font-normal';

            if (isZero) {
              valueClass = 'text-gray-400 dark:text-zinc-600 font-normal';
              labelClass = 'text-gray-400 dark:text-zinc-500 font-normal';
            } else if (row.highlight === 'warning') {
              valueClass = 'text-amber-600 dark:text-amber-400 font-medium';
            } else if (row.highlight === 'danger') {
              valueClass = 'text-rose-600 dark:text-rose-400 font-medium';
            } else if (row.highlight === 'success') {
              valueClass = 'text-emerald-600 dark:text-emerald-400 font-medium';
            }

            return (
              <div key={idx} className="flex items-center justify-between py-2.5 text-[15px]">
                <span className={labelClass}>{row.label}</span>
                <span className={valueClass}>{row.value}</span>
              </div>
            );
          })}
        </div>
      </div>
      <div className="mt-3 pt-2">
        <Link
          to={linkPath}
          className="text-sm font-medium text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 inline-flex items-center gap-1 transition-colors"
        >
          {linkText} <ChevronRight className="w-4 h-4" />
        </Link>
      </div>
    </div>
  );
}

// -------------------------------------------------------------
// Skeleton Loader
// -------------------------------------------------------------
function DashboardSkeleton() {
  return (
    <div className="animate-pulse space-y-6">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <div key={i} className="h-32 bg-gray-200 dark:bg-zinc-800 rounded-xl"></div>
        ))}
      </div>
      <div className="h-24 bg-gray-200 dark:bg-zinc-800 rounded-xl"></div>
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {[1, 2, 3, 4, 5, 6].map((i) => (
          <div key={i} className="h-48 bg-gray-200 dark:bg-zinc-800 rounded-xl"></div>
        ))}
      </div>
    </div>
  );
}

// -------------------------------------------------------------
// Main Component
// -------------------------------------------------------------
export function AdminDashboard() {
  const [summary, setSummary] = useState<AdminDashboardSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const loadStats = async () => {
    setIsLoading(true);
    setError('');

    try {
      const data = await getDashboardSummary();
      setSummary(data);
      setLastUpdated(new Date());
    } catch (err: any) {
      console.error(err);
      setError('Не удалось загрузить сводку. Проверьте соединение с сервером.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadStats();
  }, []);

  const renderAttentionBlock = () => {
    const attentionItems = summary?.attention || [];
    let activeItems = attentionItems.filter((item) => item.count > 0);

    // Sort danger first, then by count descending
    activeItems.sort((a, b) => {
      if (a.severity === 'danger' && b.severity !== 'danger') return -1;
      if (a.severity !== 'danger' && b.severity === 'danger') return 1;
      return b.count - a.count;
    });

    // Limit to 7 items
    activeItems = activeItems.slice(0, 7);

    if (activeItems.length === 0) return null;

    return (
      <div className="bg-white dark:bg-zinc-900 rounded-xl p-5 border border-gray-200 dark:border-zinc-800 shadow-sm space-y-4">
        <h2 className="text-sm font-bold uppercase tracking-wider text-gray-700 dark:text-zinc-300 flex items-center gap-2 font-sans">
          <AlertTriangle className="h-4 w-4 text-amber-500" />
          Требует внимания
        </h2>
        {/* Adjusted grid for better wrapping on wide screens */}
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
          {activeItems.map((item, idx) => {
            const isDanger = item.severity === 'danger';
            const { path, disabled } = normalizeRoute(item.link, item.title);

            let cardStyle =
              'bg-amber-50/60 dark:bg-amber-950/20 border-amber-200/50 dark:border-amber-900/40 text-amber-900 dark:text-amber-100 hover:bg-amber-100/50 dark:hover:bg-amber-900/30';
            let badgeStyle = 'bg-amber-200/70 text-amber-900 dark:bg-amber-900/60 dark:text-amber-200';

            if (isDanger) {
              cardStyle =
                'bg-rose-50/60 dark:bg-rose-950/20 border-rose-200/50 dark:border-rose-900/40 text-rose-900 dark:text-rose-100 hover:bg-rose-100/50 dark:hover:bg-rose-900/30';
              badgeStyle = 'bg-rose-200/70 text-rose-900 dark:bg-rose-900/60 dark:text-rose-200';
            }

            if (disabled) {
              return (
                <div
                  key={idx}
                  title="Раздел пока недоступен"
                  className={`border rounded-lg p-3 flex items-start justify-between opacity-60 cursor-not-allowed ${cardStyle}`}
                >
                  <div className="flex items-start gap-2.5 min-w-0 pr-2">
                    {isDanger ? (
                      <AlertCircle className="h-4 w-4 text-rose-500 shrink-0 mt-0.5" />
                    ) : (
                      <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                    )}
                    <span className="text-[15px] font-semibold leading-tight break-words">{item.title}</span>
                  </div>
                  <span className={`px-2 py-0.5 rounded text-sm font-bold ${badgeStyle} shrink-0`}>
                    {item.count}
                  </span>
                </div>
              );
            }

            return (
              <Link
                key={idx}
                to={path}
                className={`border rounded-lg p-3 flex items-start justify-between transition-colors group cursor-pointer ${cardStyle}`}
              >
                <div className="flex items-start gap-2.5 min-w-0 pr-2">
                  {isDanger ? (
                    <AlertCircle className="h-4 w-4 text-rose-500 shrink-0 mt-0.5" />
                  ) : (
                    <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                  )}
                  <span className="text-[15px] font-semibold leading-tight break-words group-hover:underline">{item.title}</span>
                </div>
                <div className="flex items-center gap-1.5 shrink-0 mt-0.5">
                  <span className={`px-2 py-0.5 rounded text-sm font-bold ${badgeStyle}`}>
                    {item.count}
                  </span>
                  <ChevronRight className="w-4 h-4 opacity-50 group-hover:opacity-100" />
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-8 pb-12 font-sans">
      {/* 1. Заголовок и обновление */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-gray-200 dark:border-zinc-800">
        <div>
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white tracking-tight font-sans">Главная</h1>
          <p className="text-base text-gray-500 dark:text-zinc-400 mt-1">
            Состояние магазина и задачи, требующие внимания
          </p>
        </div>
        <div className="flex items-center gap-4">
          {lastUpdated && (
            <span className="text-sm text-gray-500 dark:text-zinc-400">
              Обновлено: {lastUpdated.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
            </span>
          )}
          <button
            onClick={loadStats}
            disabled={isLoading}
            title="Обновить данные"
            className="p-2.5 rounded-lg border border-gray-200 dark:border-zinc-700 bg-white dark:bg-zinc-800 text-gray-600 dark:text-zinc-300 hover:bg-gray-50 dark:hover:bg-zinc-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50 transition-colors flex items-center justify-center"
          >
            <RefreshCw className={`h-5 w-5 ${isLoading ? 'animate-spin text-indigo-600' : ''}`} />
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-xl bg-rose-50/80 dark:bg-rose-950/30 border border-rose-200 dark:border-rose-900/50 p-4">
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-3">
              <AlertCircle className="h-5 w-5 text-rose-500 shrink-0" aria-hidden="true" />
              <span className="text-base font-medium text-rose-900 dark:text-rose-200">{error}</span>
            </div>
            <button
              onClick={loadStats}
              className="text-sm font-semibold text-rose-700 dark:text-rose-300 hover:underline"
            >
              Повторить
            </button>
          </div>
        </div>
      )}

      {isLoading && !summary && <DashboardSkeleton />}

      {summary && (
        <div className="space-y-8">
          {/* 2. Динамика */}
          <section>
            <h2 className="text-sm font-bold uppercase tracking-wider text-gray-500 dark:text-zinc-400 mb-4 font-sans">
              Динамика
            </h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-5">
              <TrendCard
                title="Оборот (7 дней)"
                value={formatCents(summary.overview.revenue7dCents)}
                trend={calculateTrend(summary.overview.revenue7dCents, summary.overview.previousRevenue7dCents)}
                periodText="к предыдущим 7 дням"
              />
              <TrendCard
                title="Заказы сегодня"
                value={summary.overview.ordersToday}
                trend={calculateTrend(summary.overview.ordersToday, summary.overview.averageDailyOrders20d)}
                periodText="к среднему за предыдущие 20 дней"
              />
              <TrendCard
                title="Средний чек (7 дней)"
                value={formatCents(summary.overview.averageOrderValue7dCents)}
                trend={calculateTrend(summary.overview.averageOrderValue7dCents, summary.overview.previousAverageOrderValue7dCents)}
                periodText="к предыдущим 7 дням"
              />
              <TrendCard
                title="Возвраты и отмены (7 дней)"
                value={summary.overview.returns7d}
                trend={calculateTrend(summary.overview.returns7d, summary.overview.previousReturns7d, true)}
                periodText="к предыдущим 7 дням"
              />
            </div>
          </section>

          {/* 3. Требует внимания */}
          {renderAttentionBlock()}

          {/* 4. Краткие рабочие блоки */}
          {/* 4. Рабочие процессы */}
          <section>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <SectionBox
                title="Заказы и доставка"
                linkPath="/orders"
                linkText="Открыть заказы"
                rows={[
                  {
                    label: 'Новые / В обработке',
                    value: summary.orders.newOrPending,
                    highlight: summary.orders.newOrPending > 0 ? 'warning' : undefined,
                  },
                  { label: 'В сборке', value: summary.orders.inFulfillment },
                  { label: 'Отправлены / Доставлены', value: summary.orders.shippedOrDelivered },
                  { label: 'Проблемные / Возвращенные', value: summary.orders.cancelledOrRefunded, highlight: summary.orders.cancelledOrRefunded > 0 ? 'danger' : undefined },
                ]}
              />
              <SectionBox
                title="Каталог и модерация"
                linkPath="/moderation"
                linkText="Открыть модерацию"
                rows={[
                  {
                    label: 'На модерации',
                    value: summary.products.pendingModeration,
                    highlight: summary.products.pendingModeration > 0 ? 'warning' : undefined,
                  },
                  { label: 'Опубликованы', value: summary.products.published },
                  { label: 'Отклоненные / Заблокированные', value: summary.products.rejectedOrBlocked, highlight: summary.products.rejectedOrBlocked > 0 ? 'danger' : undefined },
                  { label: 'Нет в наличии', value: summary.products.outOfStock },
                ]}
              />
              <SectionBox
                title="Склад и поставки"
                linkPath="/inventory"
                linkText="Открыть склад"
                rows={[
                  {
                    label: 'Низкий остаток',
                    value: summary.inventory.lowStockVariants,
                    highlight: summary.inventory.lowStockVariants > 0 ? 'warning' : undefined,
                  },
                  {
                    label: 'Отсутствуют',
                    value: summary.inventory.outOfStockCount,
                    highlight: summary.inventory.outOfStockCount > 0 ? 'danger' : undefined,
                  },
                  { label: 'Резерв товара', value: summary.inventory.reservedStock },
                ]}
              />
              <SectionBox
                title="Продавцы"
                linkPath="/sellers"
                linkText="Открыть продавцов"
                rows={[
                  { label: 'Активные', value: summary.sellers.active },
                  {
                    label: 'Ожидают проверки',
                    value: summary.sellers.waitingModeration,
                    highlight: summary.sellers.waitingModeration > 0 ? 'warning' : undefined,
                  },
                  {
                    label: 'Заблокированные',
                    value: summary.sellers.blocked,
                    highlight: summary.sellers.blocked > 0 ? 'danger' : undefined,
                  },
                ]}
              />
            </div>
          </section>

          {/* 5. Финансовое состояние */}
          <section className="pt-6 border-t border-gray-200 dark:border-zinc-800">
            <h2 className="text-sm font-bold uppercase tracking-wider text-gray-500 dark:text-zinc-400 mb-4 font-sans">
              Финансовое состояние
            </h2>
            <div className="bg-white dark:bg-zinc-900 rounded-xl p-6 border border-gray-200 dark:border-zinc-800 shadow-sm flex flex-col justify-between">
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6 divide-y sm:divide-y-0 sm:divide-x divide-gray-100 dark:divide-zinc-800/60">
                <div className="pt-4 sm:pt-0 sm:pl-0 sm:pr-6 flex flex-col justify-center">
                  <span className="text-[13px] text-gray-500 dark:text-zinc-400 mb-1">Оплачено покупателями</span>
                  <span className="text-xl font-bold text-gray-900 dark:text-white">{formatCents(summary.payments.paidOrdersSumCents)}</span>
                </div>
                <div className="pt-4 sm:pt-0 sm:px-6 flex flex-col justify-center">
                  <span className="text-[13px] text-gray-500 dark:text-zinc-400 mb-1">Ожидает выплаты продавцам</span>
                  <span className={`text-xl font-bold ${summary.payments.pendingPayoutsCents > 0 ? 'text-amber-600 dark:text-amber-500' : 'text-gray-900 dark:text-white'}`}>{formatCents(summary.payments.pendingPayoutsCents)}</span>
                </div>
                <div className="pt-4 sm:pt-0 sm:px-6 flex flex-col justify-center">
                  <span className="text-[13px] text-gray-500 dark:text-zinc-400 mb-1">Ошибки платежей</span>
                  <span className={`text-xl font-bold ${summary.payments.failedPaymentsCount > 0 ? 'text-rose-600 dark:text-rose-500' : 'text-gray-900 dark:text-white'}`}>{summary.payments.failedPaymentsCount}</span>
                </div>
                <div className="pt-4 sm:pt-0 sm:px-6 flex flex-col justify-center">
                  <span className="text-[13px] text-gray-500 dark:text-zinc-400 mb-1">Выплачено продавцам</span>
                  <span className="text-xl font-bold text-gray-900 dark:text-white">{formatCents(summary.payments.paidPayoutsCents || 0)}</span>
                </div>
                <div className="pt-4 sm:pt-0 sm:pl-6 flex flex-col justify-center">
                  <span className="text-[13px] text-gray-400 dark:text-zinc-500 mb-1">Комиссия платформы</span>
                  <span className="text-sm font-medium text-gray-400 dark:text-zinc-500">Комиссия ZAMOK пока не настроена</span>
                </div>
              </div>
              <div className="mt-6 pt-4 border-t border-gray-100 dark:border-zinc-800/60">
                <Link
                  to="/payments"
                  className="text-sm font-medium text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 inline-flex items-center gap-1 transition-colors"
                >
                  Открыть финансы <ChevronRight className="w-4 h-4" />
                </Link>
              </div>
            </div>
          </section>

          {/* 6. Аукционы (Условный блок) */}
          {(summary.auctions.active > 0 || summary.auctions.awaitingPayment > 0 || summary.auctions.unpaidManualReview > 0) && (
            <section className="pt-6 border-t border-gray-200 dark:border-zinc-800">
              <h2 className="text-sm font-bold uppercase tracking-wider text-gray-500 dark:text-zinc-400 mb-4 font-sans">
                Аукционы
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
                 <SectionBox
                  title="Статус аукционов"
                  linkPath="/auctions"
                  linkText="Открыть аукционы"
                  rows={[
                    { label: 'Активные', value: summary.auctions.active },
                    { label: 'Ожидают оплаты', value: summary.auctions.awaitingPayment },
                    {
                      label: 'Без оплаты (ручная пров.)',
                      value: summary.auctions.unpaidManualReview,
                      highlight: summary.auctions.unpaidManualReview > 0 ? 'danger' : undefined,
                    },
                  ]}
                />
              </div>
            </section>
          )}

        </div>
      )}
    </div>
  );
}
