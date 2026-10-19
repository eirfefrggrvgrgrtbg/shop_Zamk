import { useEffect, useState, useMemo } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import { ArrowLeftIcon } from 'lucide-react';
import { formatInTimeZone } from 'date-fns-tz';
import { getAnalyticsProductDetail, ProductDetailResponse, VariantRow } from '../api/selleranalytics';
import { MetricCard } from '../components/analytics/MetricCard';
import { TimeseriesChart } from '../components/analytics/TimeseriesChart';
import { AnalyticsPeriodPicker } from '../components/analytics/AnalyticsPeriodPicker';
import { formatCents } from '../lib/utils';

type PeriodOption = 'today' | '7d' | '30d' | '90d' | 'custom';
const TIMEZONE = 'Europe/Moscow';

export function formatVelocity(vel: number): string {
  if (!vel || isNaN(vel) || !isFinite(vel) || vel <= 0) return '—';
  if (vel < 0.01) return '< 0,01 шт/дн';
  if (vel < 1) {
    return `${vel.toFixed(2).replace('.', ',')} шт/дн`;
  }
  return `${vel.toFixed(1).replace('.', ',')} шт/дн`;
}

export function formatDaysOfStock(variant: VariantRow): string {
  if (variant.availableStock === 0 || variant.stockCoverageState === 'out_of_stock') {
    return 'Нет в наличии';
  }
  if (variant.stockCoverageState === 'no_sales' || !variant.salesVelocity || variant.salesVelocity <= 0) {
    return 'Нет продаж за период';
  }
  if (variant.daysOfStock !== null && variant.daysOfStock !== undefined && !isNaN(variant.daysOfStock) && isFinite(variant.daysOfStock)) {
    return `≈ ${Math.round(variant.daysOfStock)} дн.`;
  }
  return '—';
}

export function SellerAnalyticsProduct() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const period = (searchParams.get('period') as PeriodOption) || '30d';

  const [data, setData] = useState<ProductDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const { from, to } = useMemo(() => {
    const now = new Date();
    
    const getBounds = (daysOffset: number) => {
      const d = new Date(now.getTime() + daysOffset * 24 * 60 * 60 * 1000);
      const tomorrow = new Date(now.getTime() + 24 * 60 * 60 * 1000);
      const ymdStart = formatInTimeZone(d, TIMEZONE, 'yyyy-MM-dd');
      const ymdEnd = formatInTimeZone(tomorrow, TIMEZONE, 'yyyy-MM-dd');
      // Moscow is fixed UTC+3
      return {
        start: `${ymdStart}T00:00:00+03:00`,
        end: `${ymdEnd}T00:00:00+03:00`
      };
    };

    let fromStr: string;
    let toStr: string = getBounds(0).end;

    if (period === 'today') {
      fromStr = getBounds(0).start;
    } else if (period === '7d') {
      fromStr = getBounds(-7).start;
    } else if (period === '30d') {
      fromStr = getBounds(-30).start;
    } else if (period === '90d') {
      fromStr = getBounds(-90).start;
    } else {
      const pFrom = searchParams.get('from');
      const pTo = searchParams.get('to');
      if (pFrom && pTo) {
        fromStr = pFrom;
        toStr = pTo;
      } else {
        fromStr = getBounds(-30).start;
      }
    }

    return { from: fromStr, to: toStr };
  }, [searchParams, period]);

  useEffect(() => {
    if (!id) return;
    let ignore = false;
    async function fetchDetail() {
      setLoading(true);
      setError(null);
      try {
        const res = await getAnalyticsProductDetail(id!, from, to);
        if (!ignore) {
          setData(res);
        }
      } catch (err: any) {
        if (!ignore) {
          setError(err);
        }
      } finally {
        if (!ignore) {
          setLoading(false);
        }
      }
    }
    fetchDetail();
    return () => { ignore = true; };
  }, [id, from, to]);



  if (loading) {
    return (
      <div className="p-4 md:p-6 max-w-7xl mx-auto space-y-4 animate-pulse">
        <div className="h-8 bg-gray-200 rounded w-1/4 mb-8"></div>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
          {[1,2,3,4,5].map(i => <div key={i} className="h-24 bg-gray-200 rounded-xl"></div>)}
        </div>
        <div className="h-80 bg-gray-200 rounded-xl"></div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-4 md:p-6 max-w-7xl mx-auto">
        <div className="p-6 text-center text-red-600 bg-red-50 rounded-xl">
          <p className="font-semibold">Не удалось загрузить аналитику товара</p>
          <p className="text-sm mt-1">{error?.message}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 md:p-6 max-w-7xl mx-auto space-y-4">
      <Link to={`/analytics?tab=products&period=${period}`} className="inline-flex items-center text-sm font-medium text-gray-500 hover:text-gray-900 mb-2">
        <ArrowLeftIcon className="w-4 h-4 mr-1" /> К списку товаров
      </Link>
      
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{data.title}</h1>
        </div>
        
        <AnalyticsPeriodPicker from={from} to={to} />
      </div>

      {/* KPI Strip */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        <MetricCard
          title="Выручка"
          value={formatCents(data.grossSales.currentCents)}
          comparisonState={data.grossSales.comparisonState as any}
          changePercent={data.grossSales.changePercent}
          explanation="Сумма доставленных продаж по данному товару за выбранный период."
        />
        <MetricCard
          title="Продано"
          value={`${data.unitsSold.current} шт.`}
          comparisonState={data.unitsSold.comparisonState as any}
          changePercent={data.unitsSold.changePercent}
          explanation="Количество единиц товара, доставленных покупателям за выбранный период."
        />
        <MetricCard
          title="Заказы с доставкой"
          value={`${data.orders.current} шт.`}
          comparisonState={data.orders.comparisonState as any}
          changePercent={data.orders.changePercent}
          explanation="Количество заказов, в которых товары были доставлены покупателям за выбранный период."
        />
        <MetricCard
          title="Возвращено за период"
          value={`${data.returnedUnits.current} шт.`}
          supportingText={`Доля возвратов: ${data.returnRate.currentPercent !== null && data.returnRate.currentPercent !== undefined ? `${data.returnRate.currentPercent.toFixed(1)}%` : '—'}`}
          explanation="Возвращено за период: единицы, принятые складом в выбранные календарные даты. Доля возвратов: процент возвращенных товаров от доставленных за выбранный период (может обновляться по мере возврата недавних покупок)."
        />
        <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
          <h3 className="text-sm font-medium text-gray-500 mb-1">Доступно</h3>
          <div className="flex items-baseline gap-3">
            <span className="text-2xl font-bold text-gray-900">{data.currentAvailableStock} шт.</span>
          </div>
        </div>
      </div>

      {/* Timeseries */}
      {data.timeseries && data.timeseries.length > 0 && (
        <TimeseriesChart data={data.timeseries} mode="product" />
      )}

      {/* Variants Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        <div className="p-5 border-b border-gray-200">
          <h2 className="text-lg font-bold text-gray-900">Варианты</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm whitespace-nowrap">
            <thead className="bg-gray-50 text-gray-600 font-medium border-b border-gray-200">
              <tr>
                <th className="px-4 py-3">Вариант</th>
                <th className="px-4 py-3">SKU</th>
                <th className="px-4 py-3 text-right">Продажи</th>
                <th className="px-4 py-3 text-right">Продано</th>
                <th className="px-4 py-3 text-right" title="Доля возвратов когорты и физически возвращено за период">Доля возвратов</th>
                <th className="px-4 py-3 text-right">Доступно</th>
                <th className="px-4 py-3 text-right">Скорость продаж</th>
                <th className="px-4 py-3 text-right">Запаса на</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {data.variants.map((v: VariantRow) => (
                <tr key={v.variantId} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium text-gray-900">
                    {v.displayName || v.sku || 'Без названия'}
                  </td>
                  <td className="px-4 py-3 text-gray-500">{v.sku || '—'}</td>
                  <td className="px-4 py-3 text-right font-semibold text-gray-900">
                    {formatCents(v.grossSalesCents)}
                  </td>
                  <td className="px-4 py-3 text-right text-gray-600">
                    {v.unitsSold} шт.
                  </td>
                  <td className="px-4 py-3 text-right text-gray-600">
                    <div>
                      {v.returnRatePercent !== null && v.returnRatePercent !== undefined ? (
                        `${v.returnRatePercent.toFixed(1)}%`
                      ) : (
                        <span className="text-gray-400">—</span>
                      )}
                    </div>
                    {v.returnedUnits > 0 && (
                      <div className="text-xs text-gray-400" title="Возвращено за период">
                        {v.returnedUnits} шт. за период
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right text-gray-600">
                    {v.availableStock} шт.
                  </td>
                  <td className="px-4 py-3 text-right text-gray-600">
                    {formatVelocity(v.salesVelocity)}
                  </td>
                  <td className="px-4 py-3 text-right font-medium text-gray-900">
                    {formatDaysOfStock(v)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
