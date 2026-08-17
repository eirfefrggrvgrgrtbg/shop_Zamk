import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { AlertCircle, TrendingDown, TrendingUp, Package, AlertTriangle } from 'lucide-react';
import { 
  getAnalyticsOverview, 
  getAnalyticsProducts, 
  OverviewResponse, 
  ProductRow,
  InsightDTO
} from '../../api/selleranalytics';
import { MetricCard } from './MetricCard';
import { TimeseriesChart } from './TimeseriesChart';
import { formatCents } from '../../lib/utils';
import { cn } from '../../lib/utils';

interface OverviewTabProps {
  from: string;
  to: string;
}

export function OverviewTab({ from, to }: OverviewTabProps) {
  const [data, setData] = useState<OverviewResponse | null>(null);
  const [topProducts, setTopProducts] = useState<ProductRow[]>([]);
  const [insights, setInsights] = useState<{ insight: InsightDTO, title: string }[]>([]);
  
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [retryTrigger, setRetryTrigger] = useState(0);

  useEffect(() => {
    let ignore = false;
    async function fetchData() {
      setLoading(true);
      setError(null);
      try {
        const overviewRes = getAnalyticsOverview(from, to);
        const productsRes = getAnalyticsProducts(from, to, 'gross_sales', 'desc', 5, 0);
        
        const [overview, products] = await Promise.all([overviewRes, productsRes]);
        if (ignore) return;
        
        setData(overview);

        // API already sorted and limited to 5
        setTopProducts(products.items);

        // Process Insights
        const collectedInsights: { insight: InsightDTO, title: string }[] = [];
        if (overview.insights) {
          overview.insights.forEach(i => {
            const product = products.items.find(p => p.productId === i.productId);
            collectedInsights.push({ insight: i, title: product?.title || 'Неизвестный товар' });
          });
        }
        
        // Take top 5 most severe insights
        const severityScore: Record<string, number> = { 'high': 3, 'medium': 2, 'low': 1 };
        collectedInsights.sort((a, b) => severityScore[b.insight.severity] - severityScore[a.insight.severity]);
        setInsights(collectedInsights.slice(0, 5));

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
    fetchData();
    return () => {
      ignore = true;
    };
  }, [from, to, retryTrigger]);

  if (loading) {
    return (
      <div className="space-y-6 animate-pulse">
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
          {[1, 2, 3, 4, 5, 6].map(i => (
            <div key={i} className="h-24 bg-gray-200 rounded-xl"></div>
          ))}
        </div>
        <div className="h-80 bg-gray-200 rounded-xl"></div>
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-8 text-center text-red-600 bg-red-50 rounded-xl">
        <p className="font-semibold">Не удалось загрузить данные</p>
        <p className="text-sm mt-1">{error?.message || 'Неизвестная ошибка'}</p>
        <button onClick={() => setRetryTrigger(r => r + 1)} className="mt-4 px-4 py-2 text-sm font-medium bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors">
          Повторить попытку
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* KPI Strip */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        <MetricCard
          title="Продажи"
          value={formatCents(data.grossSales.currentCents)}
          comparisonState={data.grossSales.comparisonState as any}
          changePercent={data.grossSales.changePercent}
        />
        <MetricCard
          title="Заказы"
          value={`${data.orders.current} шт.`}
          comparisonState={data.orders.comparisonState as any}
          changePercent={data.orders.changePercent}
        />
        <MetricCard
          title="Продано"
          value={`${data.unitsSold.current} шт.`}
          comparisonState={data.unitsSold.comparisonState as any}
          changePercent={data.unitsSold.changePercent}
        />
        <MetricCard
          title="Средний чек"
          value={formatCents(data.averageOrderValue.currentCents)}
          comparisonState={data.averageOrderValue.comparisonState as any}
          changePercent={data.averageOrderValue.changePercent}
        />
        <MetricCard
          title="Доход после возвратов"
          value={formatCents(data.netCommercialEarning.currentCents)}
          comparisonState={data.netCommercialEarning.comparisonState as any}
          changePercent={data.netCommercialEarning.changePercent}
        />
        <div className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm">
          <h3 className="text-sm font-medium text-gray-500 mb-1">Возвраты</h3>
          <div className="flex items-baseline gap-3">
            <span className="text-2xl font-bold text-gray-900">{data.returnRate.currentPercent.toFixed(1)}%</span>
            <span className="text-sm text-gray-500">{data.returnedUnits.current} шт.</span>
          </div>
        </div>
      </div>

      {/* Main Chart */}
      <TimeseriesChart data={data.timeseries} />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* Insights */}
        <div>
          <h2 className="text-lg font-bold text-gray-900 mb-4">Что изменилось</h2>
          {insights.length === 0 ? (
            <div className="bg-white rounded-xl border border-gray-200 p-8 text-center text-gray-500">
              Пока нет значимых изменений
            </div>
          ) : (
            <div className="space-y-4">
              {insights.map((item, idx) => (
                <div key={idx} className="bg-white rounded-xl border border-gray-200 p-5 shadow-sm flex items-start gap-4">
                  <div className={cn(
                    "p-2 rounded-full",
                    item.insight.severity === 'high' ? "bg-red-100 text-red-600" :
                    item.insight.severity === 'medium' ? "bg-amber-100 text-amber-600" :
                    "bg-blue-100 text-blue-600"
                  )}>
                    {item.insight.type === 'out_of_stock' || item.insight.type === 'low_stock' ? <Package className="w-5 h-5" /> :
                     item.insight.type === 'falling' ? <TrendingDown className="w-5 h-5" /> :
                     item.insight.type === 'growing' ? <TrendingUp className="w-5 h-5" /> :
                     item.insight.type === 'high_return_rate' ? <AlertTriangle className="w-5 h-5" /> :
                     <AlertCircle className="w-5 h-5" />}
                  </div>
                  <div className="flex-1">
                    <h4 className="font-medium text-gray-900">{item.title}</h4>
                    <p className="text-sm font-semibold mt-1">
                      {item.insight.type === 'out_of_stock' && 'Нет в наличии'}
                      {item.insight.type === 'low_stock' && 'Заканчивается товар'}
                      {item.insight.type === 'falling' && 'Продажи снижаются'}
                      {item.insight.type === 'growing' && 'Продажи растут'}
                      {item.insight.type === 'high_return_rate' && 'Высокая доля возвратов'}
                      {item.insight.type === 'no_sales' && 'Нет продаж'}
                    </p>
                    <p className="text-sm text-gray-600 mt-1">
                      {item.insight.type === 'low_stock' && `Остатка хватит примерно на ${item.insight.evidence.daysOfStock?.toFixed(1)} дней. Сейчас доступно ${item.insight.evidence.available} шт., скорость продаж — ${item.insight.evidence.salesVelocity?.toFixed(1)} шт./день.`}
                      {item.insight.type === 'out_of_stock' && `Доступно ${item.insight.evidence.available} шт., продано ${item.insight.evidence.unitsSold} шт.`}
                      {item.insight.type === 'falling' && `Падение на ${Math.abs(item.insight.evidence.changePercent || 0).toFixed(1)}%.`}
                      {item.insight.type === 'growing' && `Рост на ${(item.insight.evidence.changePercent || 0).toFixed(1)}%.`}
                      {item.insight.type === 'high_return_rate' && `Доля возвратов: ${(item.insight.evidence.returnRatePercent || 0).toFixed(1)}% (продано ${item.insight.evidence.unitsSold}, возвращено ${item.insight.evidence.returnedUnits}).`}
                    </p>
                    <Link to={`/analytics/products/${item.insight.productId}`} className="text-blue-600 text-sm font-medium mt-3 inline-block hover:underline">
                      Посмотреть товар
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Top Products */}
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-bold text-gray-900">Лидеры продаж</h2>
            <Link to="?tab=products" className="text-sm font-medium text-blue-600 hover:underline">
              Все товары &rarr;
            </Link>
          </div>
          {topProducts.length === 0 ? (
            <div className="bg-white rounded-xl border border-gray-200 p-8 text-center text-gray-500">
              Нет проданных товаров
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
              <table className="w-full text-left text-sm">
                <thead className="bg-gray-50 text-gray-500 font-medium border-b border-gray-200">
                  <tr>
                    <th className="px-4 py-3">Товар</th>
                    <th className="px-4 py-3 text-right">Продажи</th>
                    <th className="px-4 py-3 text-right">Продано</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {topProducts.map(p => (
                    <tr key={p.productId} className="hover:bg-gray-50 transition-colors">
                      <td className="px-4 py-3 font-medium text-gray-900">
                        <Link to={`/analytics/products/${p.productId}`} className="hover:underline">
                          {p.title}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-right font-semibold">
                        {formatCents(p.grossSalesCents)}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-600">
                        {p.unitsSold} шт.
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
