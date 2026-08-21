import { useEffect, useState } from 'react';
import { getAnalyticsOverview, OverviewResponse } from '../../api/selleranalytics';
import { formatCents } from '../../lib/utils';
import { HelpCircle } from 'lucide-react';

interface FinanceTabProps {
  from: string;
  to: string;
}

export function FinanceTab({ from, to }: FinanceTabProps) {
  const [data, setData] = useState<OverviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const [retryTrigger, setRetryTrigger] = useState(0);

  useEffect(() => {
    let ignore = false;
    async function fetchOverview() {
      setLoading(true);
      setError(null);
      try {
        const res = await getAnalyticsOverview(from, to);
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
    fetchOverview();
    return () => {
      ignore = true;
    };
  }, [from, to, retryTrigger]);

  if (loading) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 max-w-3xl animate-pulse">
        <div className="h-8 bg-gray-200 rounded mb-6 w-1/4"></div>
        {[1, 2, 3, 4, 5].map(i => <div key={i} className="h-14 bg-gray-100 rounded mb-3"></div>)}
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="p-8 text-center text-red-600 bg-red-50 rounded-xl max-w-3xl">
        <p className="font-semibold">Не удалось загрузить финансовую аналитику</p>
        <p className="text-sm mt-1">{error?.message || 'Неизвестная ошибка'}</p>
        <button onClick={() => setRetryTrigger(r => r + 1)} className="mt-4 px-4 py-2 text-sm font-medium bg-red-100 text-red-700 rounded-md hover:bg-red-200 transition-colors">
          Повторить попытку
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        <div className="p-6 border-b border-gray-200">
          <h2 className="text-lg font-bold text-gray-900">Структура доходов</h2>
          <p className="text-sm text-gray-500 mt-1">
            Аналитика по всем доставленным заказам и фактическим возвратам за выбранный период.
          </p>
        </div>
        <div className="p-6">
          <div className="space-y-4">
            
            {/* Gross Sales */}
            <div className="flex justify-between items-center py-2">
              <div className="flex items-center gap-2">
                <span className="font-medium text-gray-900">Продажи</span>
              </div>
              <span className="font-semibold text-gray-900">
                {formatCents(data.grossSales.currentCents)}
              </span>
            </div>

            {/* Commission */}
            <div className="flex justify-between items-center py-2 text-red-600">
              <div className="flex items-center gap-2">
                <span>Комиссия ZAMK</span>
              </div>
              <span>− {formatCents(Math.abs(data.commission.currentCents))}</span>
            </div>

            {/* Subtotal */}
            <div className="flex justify-between items-center py-3 border-t border-gray-100 bg-gray-50 -mx-6 px-6">
              <span className="font-medium text-gray-900">Доход до возвратов</span>
              <span className="font-semibold text-gray-900">
                {formatCents(data.sellerEarningBeforeReturns.currentCents)}
              </span>
            </div>

            {/* Returns */}
            <div className="flex justify-between items-center py-2 text-gray-900">
              <div className="flex items-center gap-2">
                <span>Удержания по возвратам</span>
                <span className="text-xs font-medium bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full">
                  {data.returnedUnits.current} шт.
                </span>
              </div>
              <span>− {formatCents(Math.abs(data.returnDeductions.currentCents))}</span>
            </div>

            {/* Other Adjustments */}
            {data.otherAdjustments.currentCents !== 0 && (
              <div className="flex justify-between items-center py-2 text-gray-600">
                <span>Прочие корректировки</span>
                <span>
                  {data.otherAdjustments.currentCents > 0 ? '+' : '−'} {formatCents(Math.abs(data.otherAdjustments.currentCents))}
                </span>
              </div>
            )}

            {/* Total */}
            <div className="flex justify-between items-center py-4 border-t border-gray-200 mt-2">
              <span className="text-lg font-bold text-gray-900">Коммерческий результат</span>
              <span className="text-xl font-bold text-gray-900">
                {formatCents(data.netCommercialEarning.currentCents)}
              </span>
            </div>
            
          </div>
        </div>
      </div>

      {/* Return Rate Insight */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-6 flex items-start gap-4">
        <div className="bg-blue-50 p-3 rounded-full text-blue-600">
          <HelpCircle className="w-6 h-6" />
        </div>
        <div>
          <h3 className="font-semibold text-gray-900 mb-1">Доля возвратов: {data.returnRate.currentPercent.toFixed(1)}%</h3>
          <p className="text-sm text-gray-600">
            В расчет доли возвратов включены только фактически завершенные возвраты (товар прибыл на склад и деньги удержаны). Возвраты в пути не учитываются.
          </p>
        </div>
      </div>

    </div>
  );
}
