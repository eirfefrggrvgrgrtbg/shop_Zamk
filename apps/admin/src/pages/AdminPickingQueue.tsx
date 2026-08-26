import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { PackageCheck, ArrowRight, RefreshCw, AlertCircle, Clock, CheckCircle2, Box } from 'lucide-react';
import { AdminOrdersTabs } from '../components/orders/AdminOrdersTabs';
import { getAdminPickingQueue, PickingQueueItem } from '../api/adminPicking';

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
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Заказы к сборке</h1>
          <p className="text-sm text-slate-400 mt-1">Очередь оплаченных заказов для сборки на складе</p>
        </div>
        <div className="mt-4 sm:mt-0 flex items-center space-x-3">
          <button
            onClick={fetchQueue}
            disabled={isLoading}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-slate-800 text-slate-200 hover:bg-slate-700 hover:text-white border border-slate-700 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      <AdminOrdersTabs />

      <div className="px-4 sm:px-6 space-y-6">
        {/* KPI Cards */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          <div className="p-4 rounded-xl border bg-slate-900/60 border-slate-800">
            <div className="text-xs text-slate-400 font-medium">Всего к сборке</div>
            <div className="text-2xl font-bold mt-1 text-white">{totalOrders}</div>
          </div>
          <div className="p-4 rounded-xl border bg-slate-900/60 border-slate-800">
            <div className="text-xs text-slate-400 font-medium">В процессе сборки</div>
            <div className="text-2xl font-bold mt-1 text-blue-400">{assemblingOrders}</div>
          </div>
          <div className="p-4 rounded-xl border bg-slate-900/60 border-slate-800">
            <div className="text-xs text-slate-400 font-medium">Ожидают сборки</div>
            <div className="text-2xl font-bold mt-1 text-amber-400">{pendingOrders}</div>
          </div>
          <div className="p-4 rounded-xl border bg-slate-900/60 border-slate-800">
            <div className="text-xs text-slate-400 font-medium">Собрано единиц</div>
            <div className="text-2xl font-bold mt-1 text-emerald-400">
              {pickedUnits} / {totalUnits}
            </div>
          </div>
        </div>

        {/* Error State */}
        {error && (
          <div className="p-4 rounded-xl bg-rose-950/50 border border-rose-800 text-rose-300 flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <AlertCircle className="w-5 h-5 flex-shrink-0 text-rose-400" />
              <span>{error}</span>
            </div>
            <button
              onClick={fetchQueue}
              className="px-3 py-1 bg-rose-900/60 hover:bg-rose-900 rounded-lg text-xs font-semibold text-rose-200 transition-colors"
            >
              Повторить
            </button>
          </div>
        )}

        {/* Loading State */}
        {isLoading && !error && (
          <div className="p-12 text-center text-slate-400 bg-slate-900/40 rounded-2xl border border-slate-800">
            <RefreshCw className="w-8 h-8 animate-spin mx-auto text-indigo-400 mb-3" />
            <p className="text-sm font-medium">Загрузка очереди сборок...</p>
          </div>
        )}

        {/* Empty State */}
        {!isLoading && !error && items.length === 0 && (
          <div className="p-12 text-center bg-slate-900/40 rounded-2xl border border-slate-800">
            <PackageCheck className="w-12 h-12 mx-auto text-slate-600 mb-3" />
            <h3 className="text-base font-semibold text-slate-200">Нет заказов для сборки</h3>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
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
                  className="bg-slate-900/70 border border-slate-800 hover:border-slate-700 rounded-2xl p-5 transition-all shadow-sm flex flex-col md:flex-row md:items-center md:justify-between gap-4"
                >
                  <div className="space-y-2 flex-1">
                    <div className="flex flex-wrap items-center gap-2.5">
                      <span className="text-base font-bold text-white tracking-tight">
                        Заказ #{item.orderNumber || item.orderId.slice(0, 8)}
                      </span>
                      {isAssembling ? (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-950/80 text-blue-400 border border-blue-800/60">
                          Сборка
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-950/80 text-amber-400 border border-amber-800/60">
                          Ожидает сборки
                        </span>
                      )}
                      {item.sellerName && (
                        <span className="text-xs text-slate-400 font-medium">
                          Продавец: <span className="text-slate-300">{item.sellerName}</span>
                        </span>
                      )}
                    </div>

                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-400">
                      <div className="flex items-center gap-1.5">
                        <Box className="w-3.5 h-3.5 text-slate-500" />
                        <span>
                          {item.itemPositionsCount} {item.itemPositionsCount === 1 ? 'позиция' : 'позиций'} · {item.totalQuantity} {item.totalQuantity === 1 ? 'единица' : 'единиц'}
                        </span>
                      </div>
                      <div className="flex items-center gap-1.5">
                        <CheckCircle2 className="w-3.5 h-3.5 text-slate-500" />
                        <span className={item.pickedQuantity > 0 ? 'text-indigo-300 font-medium' : ''}>
                          Собрано {item.pickedQuantity} / {item.totalQuantity}
                        </span>
                      </div>
                      {formattedDate && (
                        <div className="flex items-center gap-1.5 text-slate-500">
                          <Clock className="w-3.5 h-3.5" />
                          <span>{formattedDate}</span>
                        </div>
                      )}
                    </div>

                    {/* Progress Bar */}
                    <div className="w-full max-w-md bg-slate-800 rounded-full h-2 overflow-hidden">
                      <div
                        className={`h-2 rounded-full transition-all duration-300 ${
                          item.isComplete
                            ? 'bg-emerald-500'
                            : isAssembling
                            ? 'bg-indigo-500'
                            : 'bg-slate-600'
                        }`}
                        style={{ width: `${item.progressPercent}%` }}
                      />
                    </div>
                  </div>

                  <div className="flex items-center justify-end">
                    <button
                      onClick={() => navigate(`/fulfillment/picking/${item.fulfillmentId}`)}
                      className={`inline-flex items-center justify-center px-4 py-2.5 rounded-xl text-sm font-semibold transition-all shadow-md ${
                        isAssembling
                          ? 'bg-indigo-600 hover:bg-indigo-500 text-white shadow-indigo-900/30'
                          : 'bg-slate-800 hover:bg-slate-700 text-white border border-slate-700'
                      }`}
                    >
                      <span>{isAssembling ? 'Продолжить сборку' : 'Начать сборку'}</span>
                      <ArrowRight className="w-4 h-4 ml-2" />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
