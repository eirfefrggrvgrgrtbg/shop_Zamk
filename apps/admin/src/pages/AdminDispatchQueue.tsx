import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Truck,
  RefreshCw,
  AlertCircle,
  Clock,
  ArrowRight,
  Boxes,
  MapPin,
} from 'lucide-react';
import { getAdminDispatchQueue, DispatchQueueItem } from '../api/adminPicking';
import { formatOrderNumber } from '../utils/orderFormatters';

export function AdminDispatchQueue() {
  const navigate = useNavigate();
  const [items, setItems] = useState<DispatchQueueItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchQueue = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminDispatchQueue();
      setItems(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить очередь отгрузки.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchQueue();
  }, []);

  const totalOrders = items.length;
  const totalUnits = items.reduce((sum, i) => sum + i.totalQuantity, 0);

  const formatTimestamp = (dateStr?: string | null) => {
    if (!dateStr) return null;
    try {
      const d = new Date(dateStr);
      return d.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return null;
    }
  };

  return (
    <div data-testid="dispatch-queue-page" className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Отгрузка</h1>
          <p className="text-sm text-gray-500 mt-1">
            Заказы, упакованные и готовые к передаче в доставку
          </p>
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

      {/* KPI Chip */}
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <div className="p-3.5 rounded-xl border bg-white border-gray-200 shadow-sm">
          <div className="text-xs text-gray-500 font-medium">Ожидают отгрузки</div>
          <div className="text-xl font-bold mt-1 text-gray-900" data-testid="dispatch-queue-count">
            {totalOrders}
          </div>
        </div>
        <div className="p-3.5 rounded-xl border bg-white border-gray-200 shadow-sm">
          <div className="text-xs text-gray-500 font-medium">Всего единиц к отгрузке</div>
          <div className="text-xl font-bold mt-1 text-indigo-600">
            {totalUnits} шт.
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
          <p className="text-sm font-medium">Загрузка очереди отгрузки...</p>
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !error && items.length === 0 && (
        <div className="p-12 text-center bg-white rounded-xl border border-gray-200 shadow-sm space-y-3">
          <div className="w-12 h-12 rounded-2xl bg-gray-50 flex items-center justify-center mx-auto text-gray-400">
            <Truck className="w-6 h-6" />
          </div>
          <h3 className="text-base font-bold text-gray-900">Нет заказов, ожидающих отгрузки</h3>
          <p className="text-xs text-gray-500 max-w-md mx-auto">
            Все упакованные заказы уже переданы в доставку, либо текущие заказы находятся на этапе сборки и упаковки.
          </p>
        </div>
      )}

      {/* Queue List */}
      {!isLoading && !error && items.length > 0 && (
        <div className="space-y-3" data-testid="dispatch-queue-list">
          {items.map((item) => {
            const packedTimestamp = formatTimestamp(item.packedAt || item.createdAt);

            return (
              <div
                key={item.fulfillmentId}
                onClick={() => navigate(`/fulfillment/dispatch/${item.fulfillmentId}`)}
                className="bg-white rounded-xl border border-gray-200 p-4 sm:p-5 hover:border-indigo-300 hover:shadow-md transition-all cursor-pointer flex flex-col sm:flex-row sm:items-center justify-between gap-4 group"
              >
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2 sm:gap-3">
                    <span className="font-bold text-base text-gray-900 tracking-tight group-hover:text-indigo-600 transition-colors">
                      Заказ #{formatOrderNumber({ id: item.orderId, orderNumber: item.orderNumber })}
                    </span>
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-blue-50 text-blue-700 border border-blue-200">
                      Упакован (готов к отгрузке)
                    </span>
                    <span className="text-xs font-mono text-gray-400">
                      ID: {item.fulfillmentId.slice(0, 8)}
                    </span>
                  </div>

                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500">
                    <span className="flex items-center gap-1 font-medium text-gray-700">
                      <Boxes className="w-3.5 h-3.5 text-gray-400" />
                      {item.itemsCount} поз. · {item.totalQuantity} шт.
                    </span>
                    {item.deliveryMethodName && (
                      <span className="flex items-center gap-1 text-gray-600">
                        <MapPin className="w-3.5 h-3.5 text-gray-400" />
                        Доставка: {item.deliveryMethodName}
                      </span>
                    )}
                    {item.carrier && (
                      <span className="flex items-center gap-1 text-gray-600">
                        <Truck className="w-3.5 h-3.5 text-gray-400" />
                        Служба: {item.carrier}
                      </span>
                    )}
                    {packedTimestamp && (
                      <span className="flex items-center gap-1 text-gray-400">
                        <Clock className="w-3.5 h-3.5" />
                        Упакован {packedTimestamp}
                      </span>
                    )}
                  </div>
                </div>

                <div className="flex items-center sm:self-center shrink-0">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      navigate(`/fulfillment/dispatch/${item.fulfillmentId}`);
                    }}
                    className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-4 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-bold rounded-xl transition-colors shadow-sm"
                  >
                    <span>Отгрузить</span>
                    <ArrowRight className="w-4 h-4" />
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
