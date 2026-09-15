import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Search,
  RefreshCw,
  PackageCheck,
  AlertCircle,
  Clock,
  Package,
} from 'lucide-react';
import {
  getAdminReturnReceivingQueue,
  getAdminReturnErrorMessage,
  type AdminReturnReceivingQueueItem,
} from '../api/adminReturns';

function formatOperationalTime(dateStr?: string): string {
  if (!dateStr) return '';
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '';
    const now = new Date();
    const isToday =
      d.getDate() === now.getDate() &&
      d.getMonth() === now.getMonth() &&
      d.getFullYear() === now.getFullYear();
    const isYesterday =
      d.getDate() === now.getDate() - 1 &&
      d.getMonth() === now.getMonth() &&
      d.getFullYear() === now.getFullYear();

    const timePart = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
    if (isToday) return `сегодня, ${timePart}`;
    if (isYesterday) return `вчера, ${timePart}`;

    return d.toLocaleDateString('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return '';
  }
}

export function AdminReturnReceivingQueue() {
  const navigate = useNavigate();
  const [queue, setQueue] = useState<AdminReturnReceivingQueueItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');

  const fetchQueue = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const items = await getAdminReturnReceivingQueue();
      setQueue(items || []);
    } catch (err: unknown) {
      setError(getAdminReturnErrorMessage(err, 'Не удалось загрузить очередь приёмки возвратов'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchQueue();
  }, []);

  const filteredQueue = useMemo(() => {
    if (!searchQuery.trim()) return queue;
    const q = searchQuery.trim().toLowerCase();
    return queue.filter((item) => {
      const retId = item.returnId.toLowerCase();
      const ordNum = item.orderNumber.toLowerCase();
      const tracking = (item.trackingNumber || '').toLowerCase();
      const seller = (item.sellerName || '').toLowerCase();
      const prod = (item.productSummary || '').toLowerCase();
      return (
        retId.includes(q) ||
        ordNum.includes(q) ||
        tracking.includes(q) ||
        seller.includes(q) ||
        prod.includes(q)
      );
    });
  }, [queue, searchQuery]);

  const countArrived = queue.filter((q) => q.returnStatus === 'approved').length;
  const countReceiving = queue.filter((q) => q.returnStatus === 'receiving').length;
  const totalRemainingUnits = queue.reduce((sum, q) => sum + (q.remainingUnitsCount || 0), 0);

  return (
    <div className="space-y-6 pb-20 max-w-5xl">
      {/* Page Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Приёмка возвратов</h1>
        <p className="text-sm mt-1 text-gray-500">
          Возвраты, которые прибыли на склад и требуют физической приёмки
        </p>
      </div>

      {/* Search Toolbar */}
      <div className="bg-white border border-gray-200 rounded-xl p-4 sm:p-5 shadow-2xs">
        <div className="relative">
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Номер возврата, заказа или трек-номер..."
            className="w-full h-10 pl-10 pr-4 bg-gray-50 border border-gray-200 rounded-lg text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:bg-white transition-colors"
          />
          <Search className="w-4 h-4 text-gray-400 absolute left-3.5 top-3" />
        </div>
        <p className="text-xs text-gray-400 mt-2">
          Введите ID возврата, номер заказа (ORD-...) или трек-номер отправления
        </p>
      </div>

      {/* Queue Header & Inline Summary Chips */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pt-2">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">
            Ожидают приёмки · {queue.length}
          </h2>
          <p className="text-xs text-gray-500 mt-0.5">
            Физически прибывшие отправления, требующие проверки и оприходования
          </p>
          <div className="flex flex-wrap items-center gap-2 mt-2">
            <span className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
              Ожидают начала {countArrived}
            </span>
            <span className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
              В процессе {countReceiving}
            </span>
            <span className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium bg-gray-100 text-gray-600 border border-gray-200">
              Осталось принять {totalRemainingUnits} шт.
            </span>
          </div>
        </div>
        <button
          type="button"
          onClick={fetchQueue}
          disabled={isLoading}
          className="inline-flex items-center self-start sm:self-auto px-3 py-1.5 border border-gray-200 rounded-lg text-xs font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 transition shadow-2xs"
        >
          <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
          Обновить
        </button>
      </div>

      {/* Error State */}
      {error && (
        <div className="bg-rose-50 border border-rose-200 rounded-xl p-4 flex items-center justify-between">
          <div className="flex items-center space-x-2 text-rose-800 text-xs font-medium">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button
            type="button"
            onClick={fetchQueue}
            className="text-xs font-semibold text-rose-700 hover:text-rose-900 underline ml-3"
          >
            Повторить
          </button>
        </div>
      )}

      {/* Loading State */}
      {isLoading && !error && (
        <div className="bg-white border border-gray-200 rounded-xl p-12 text-center shadow-2xs">
          <RefreshCw className="w-6 h-6 animate-spin mx-auto text-indigo-600 mb-2" />
          <p className="text-xs text-gray-500 font-medium">Загрузка очереди приёмки возвратов...</p>
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !error && queue.length === 0 && (
        <div className="bg-white border border-gray-200 rounded-xl p-8 text-center shadow-2xs">
          <PackageCheck className="w-8 h-8 text-gray-400 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-gray-900">
            Нет возвратов, ожидающих приёмки
          </h3>
          <p className="text-xs text-gray-500 max-w-md mx-auto mt-1">
            Все прибывшие возвраты обработаны, либо новые возвраты ещё не поступили на склад.
          </p>
        </div>
      )}

      {/* Search No Matches State */}
      {!isLoading && !error && queue.length > 0 && filteredQueue.length === 0 && (
        <div className="bg-white border border-gray-200 rounded-xl p-8 text-center shadow-2xs">
          <Search className="w-8 h-8 text-gray-400 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-gray-900">
            Ничего не найдено
          </h3>
          <p className="text-xs text-gray-500 max-w-md mx-auto mt-1">
            По запросу «{searchQuery}» возвратов в очереди не найдено.
          </p>
        </div>
      )}

      {/* Queue Cards List */}
      {!isLoading && !error && filteredQueue.length > 0 && (
        <div className="space-y-3" data-testid="returns-receiving-queue-list">
          {filteredQueue.map((item) => {
            const isReceiving = item.returnStatus === 'receiving';
            const arrivedTimeStr = formatOperationalTime(item.arrivedAt || item.createdAt);
            const startedTimeStr = formatOperationalTime(item.receivingStartedAt);
            const returnDisplayId = `RET-${item.returnId.slice(0, 8).toUpperCase()}`;

            return (
              <div
                key={item.returnId}
                onClick={() => navigate(`/returns/${item.returnId}/receiving`)}
                className="bg-white border border-gray-200 rounded-xl p-4 sm:p-5 transition hover:border-indigo-300 hover:shadow-2xs cursor-pointer flex flex-col sm:flex-row sm:items-center justify-between gap-4"
              >
                <div className="space-y-1.5 min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-bold text-gray-900 tracking-tight font-mono">
                      {returnDisplayId}
                    </span>
                    {isReceiving ? (
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-indigo-50 text-indigo-700 border border-indigo-200">
                        Приёмка начата
                      </span>
                    ) : (
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
                        Ожидает приёмки
                      </span>
                    )}
                    <span className="text-xs text-gray-500 font-medium">
                      Заказ <strong className="text-gray-700 font-semibold">{item.orderNumber}</strong>
                    </span>
                    {item.trackingNumber && (
                      <span className="text-xs font-mono text-gray-400 bg-gray-50 px-1.5 py-0.5 rounded border border-gray-100">
                        {item.trackingNumber}
                      </span>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500">
                    {item.sellerName && (
                      <span className="font-medium text-gray-700 truncate max-w-xs">
                        {item.sellerName}
                      </span>
                    )}
                    {item.productSummary && (
                      <>
                        <span className="text-gray-300">·</span>
                        <span className="text-gray-600 truncate max-w-sm">
                          {item.productSummary}
                        </span>
                      </>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 pt-0.5">
                    {isReceiving ? (
                      <span className="font-medium text-indigo-700">
                        Принято {item.receivedUnitsCount} из {item.expectedUnitsCount} · осталось {item.remainingUnitsCount}
                      </span>
                    ) : (
                      <span className="font-medium text-gray-700 flex items-center gap-1">
                        <Package className="w-3.5 h-3.5 text-gray-400" />
                        {item.expectedUnitsCount} {item.expectedUnitsCount === 1 ? 'единица' : 'единицы'}
                      </span>
                    )}

                    <span className="text-gray-300">·</span>
                    <span className="flex items-center gap-1 text-gray-500">
                      <Clock className="w-3.5 h-3.5 text-gray-400" />
                      {isReceiving && startedTimeStr
                        ? `Начата ${startedTimeStr}`
                        : arrivedTimeStr
                        ? `Прибыл ${arrivedTimeStr}`
                        : 'Прибыл на склад'}
                    </span>
                  </div>
                </div>

                <div className="shrink-0 flex sm:flex-col justify-end">
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      navigate(`/returns/${item.returnId}/receiving`);
                    }}
                    className={`px-4 py-2 text-xs font-semibold rounded-lg text-white transition shadow-2xs ${
                      isReceiving
                        ? 'bg-indigo-600 hover:bg-indigo-700'
                        : 'bg-gray-900 hover:bg-black'
                    }`}
                  >
                    {isReceiving ? 'Продолжить приёмку' : 'Начать приёмку'}
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
