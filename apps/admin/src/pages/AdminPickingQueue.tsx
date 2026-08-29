import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PackageCheck, ArrowRight, RefreshCw, AlertCircle, Clock, CheckCircle2, Box } from 'lucide-react';
import { getAdminPickingQueue, PickingQueueItem } from '../api/adminPicking';
import { formatOrderNumber } from '../utils/orderFormatters';

export function AdminPickingQueue() {
  const navigate = useNavigate();
  const [items, setItems] = useState<PickingQueueItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchQueue = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminPickingQueue();
      setItems(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить очередь сборок.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchQueue();
  }, []);

  const totalOrders = items.length;
  const assemblingOrders = items.filter((i) => i.status === 'assembling').length;
  const pendingOrders = items.filter((i) => i.status === 'paid').length;
  const totalUnits = items.reduce((sum, i) => sum + i.totalQuantity, 0);
  const pickedUnits = items.reduce((sum, i) => sum + i.pickedQuantity, 0);

  return (
    <div data-testid="picking-queue-page" className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Сборка заказов</h1>
          <p className="text-sm text-gray-500 mt-1">Оплаченные заказы, готовые к комплектации на складе ZAMK</p>
        </div>
        <div className="flex items-center space-x-3">
          <button
            onClick={fetchQueue}
            disabled={isLoading}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      {/* KPI Chips */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="p-3.5 rounded-xl border bg-white border-gray-200 shadow-sm">
          <div className="text-xs text-gray-500 font-medium">Всего к сборке</div>
          <div className="text-xl font-bold mt-1 text-gray-900">{totalOrders}</div>
        </div>
        <div className="p-3.5 rounded-xl border bg-white border-gray-200 shadow-sm">
          <div className="text-xs text-gray-500 font-medium">В процессе сборки</div>
          <div className="text-xl font-bold mt-1 text-blue-600">{assemblingOrders}</div>
        </div>
        <div className="p-3.5 rounded-xl border bg-white border-gray-200 shadow-sm">
          <div className="text-xs text-gray-500 font-medium">Ожидают сборки</div>
          <div className="text-xl font-bold mt-1 text-amber-600">{pendingOrders}</div>
        </div>
        <div className="p-3.5 rounded-xl border bg-white border-gray-200 shadow-sm">
          <div className="text-xs text-gray-500 font-medium">Собрано единиц</div>
          <div className="text-xl font-bold mt-1 text-emerald-600">
            {pickedUnits} / {totalUnits}
          </div>
        </div>
      </div>

      {/* Error State */}
      {error && (
        <div className="p-4 rounded-xl bg-rose-50 border border-rose-200 text-rose-800 flex items-center justify-between shadow-sm">
          <div className="flex items-center space-x-3">
            <AlertCircle className="w-5 h-5 flex-shrink-0 text-rose-600" />
            <span className="text-sm font-medium">{error}</span>
          </div>
          <button
            onClick={fetchQueue}
            className="px-3 py-1.5 bg-rose-100 hover:bg-rose-200 rounded-lg text-xs font-semibold text-rose-800 transition-colors"
          >
            Повторить
          </button>
        </div>
      )}

      {/* Loading State */}
      {isLoading && !error && (
        <div className="p-12 text-center text-gray-500 bg-white rounded-xl border border-gray-200 shadow-sm">
          <RefreshCw className="w-8 h-8 animate-spin mx-auto text-indigo-600 mb-3" />
          <p className="text-sm font-medium">Загрузка очереди сборок...</p>
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !error && items.length === 0 && (
        <div className="p-12 text-center bg-white rounded-xl border border-gray-200 shadow-sm">
          <PackageCheck className="w-12 h-12 mx-auto text-gray-400 mb-3" />
          <h3 className="text-base font-semibold text-gray-900">Нет заказов для сборки</h3>
          <p className="text-xs text-gray-500 mt-1 max-w-sm mx-auto">
            Все оплаченные заказы уже собраны или пока отсутствуют новые заказы к сборке.
          </p>
        </div>
      )}

      {/* Queue List */}
      {!isLoading && !error && items.length > 0 && (
        <div className="space-y-3">
          {items.map((item) => {
            const isAssembling = item.status === 'assembling' || item.pickedQuantity > 0;
            const formattedDate = item.createdAt
              ? new Date(item.createdAt).toLocaleString('ru-RU', {
                  day: '2-digit',
                  month: '2-digit',
                  hour: '2-digit',
                  minute: '2-digit',
                })
              : null;

            return (
              <div
                key={item.fulfillmentId}
                className="bg-white border border-gray-200 hover:border-gray-300 rounded-xl p-5 transition-all shadow-sm flex flex-col md:flex-row md:items-center md:justify-between gap-4"
              >
                <div className="space-y-2 flex-1">
                  <div className="flex flex-wrap items-center gap-2.5">
                    <span className="text-base font-bold text-gray-900 tracking-tight">
                      Заказ #{formatOrderNumber({ id: item.orderId, orderNumber: item.orderNumber })}
                    </span>
                    {isAssembling ? (
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-50 text-blue-700 border border-blue-200">
                        Сборка
                      </span>
                    ) : (
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200">
                        Ожидает сборки
                      </span>
                    )}
                    {item.sellerName && (
                      <span className="text-xs text-gray-500 font-medium">
                        Продавец: <span className="text-gray-700">{item.sellerName}</span>
                      </span>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500">
                    <div className="flex items-center gap-1.5">
                      <Box className="w-3.5 h-3.5 text-gray-400" />
                      <span>
                        {item.itemPositionsCount} {item.itemPositionsCount === 1 ? 'позиция' : item.itemPositionsCount > 1 && item.itemPositionsCount < 5 ? 'позиции' : 'позиций'} · {item.totalQuantity} {item.totalQuantity === 1 ? 'единица' : 'единиц'}
                      </span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <CheckCircle2 className="w-3.5 h-3.5 text-gray-400" />
                      <span className={item.pickedQuantity > 0 ? 'text-indigo-600 font-semibold' : ''}>
                        Собрано {item.pickedQuantity} / {item.totalQuantity}
                      </span>
                    </div>
                    {formattedDate && (
                      <div className="flex items-center gap-1.5 text-gray-400">
                        <Clock className="w-3.5 h-3.5" />
                        <span>{formattedDate}</span>
                      </div>
                    )}
                  </div>

                  {/* Progress Bar */}
                  <div className="w-full max-w-md bg-gray-100 rounded-full h-2 overflow-hidden border border-gray-200">
                    <div
                      className={`h-2 rounded-full transition-all duration-300 ${
                        item.isComplete
                          ? 'bg-emerald-500'
                          : isAssembling
                          ? 'bg-indigo-600'
                          : 'bg-gray-400'
                      }`}
                      style={{ width: `${item.progressPercent}%` }}
                    />
                  </div>
                </div>

                <div className="flex items-center justify-end">
                  <button
                    onClick={() =>
                      navigate(
                        item.isComplete
                          ? `/fulfillment/packing/${item.fulfillmentId}`
                          : `/fulfillment/picking/${item.fulfillmentId}`
                      )
                    }
                    className={`inline-flex items-center justify-center px-4 py-2.5 rounded-xl text-sm font-semibold transition-all shadow-sm ${
                      item.isComplete
                        ? 'bg-emerald-600 hover:bg-emerald-700 text-white'
                        : isAssembling
                        ? 'bg-indigo-600 hover:bg-indigo-700 text-white'
                        : 'bg-white hover:bg-gray-50 text-gray-700 border border-gray-300'
                    }`}
                  >
                    <span>
                      {item.isComplete
                        ? 'Перейти к упаковке'
                        : isAssembling
                        ? 'Продолжить сборку'
                        : 'Начать сборку'}
                    </span>
                    <ArrowRight className="w-4 h-4 ml-2" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
