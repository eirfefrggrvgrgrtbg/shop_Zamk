import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams, Link } from 'react-router-dom';
import { Search, AlertCircle, ShoppingCart, Filter, ArrowRight, AlertTriangle, RefreshCw } from 'lucide-react';
import { formatMoney, formatOrderNumber, formatDateTime, orderStatusMap, fulfillmentStatusMap } from '../utils/orderFormatters';
import { getAdminOrders } from '../api/adminOrders';
import type { AdminOrderView } from '../api/adminOrders';

export function AdminOrders() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const [orders, setOrders] = useState<AdminOrderView[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const page = parseInt(searchParams.get('page') || '1', 10);
  const limit = 20;

  const searchQuery = searchParams.get('q') || '';
  const statusFilter = searchParams.get('status') || '';
  const fulfillmentFilter = searchParams.get('fulfillment') || '';

  const updateParam = (key: string, value: string) => {
    const newParams = new URLSearchParams(searchParams);
    if (value) {
      newParams.set(key, value);
    } else {
      newParams.delete(key);
    }
    newParams.set('page', '1');
    setSearchParams(newParams);
  };

  const fetchOrders = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminOrders({
        limit,
        offset: (page - 1) * limit,
        q: searchQuery,
        status: statusFilter,
        fulfillmentStatus: fulfillmentFilter,
      });
      setOrders(data.items);
      setTotalCount(data.totalCount);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить заказы.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchOrders();
  }, [searchParams]);

  // Operational KPI Chips
  const kpis = [
    { key: 'status', val: 'paid', label: 'Новые оплаченные', count: orders.filter(o => o.status === 'paid').length, color: 'text-emerald-700' },
    { key: 'status', val: 'assembling', label: 'В работе', count: orders.filter(o => o.status === 'assembling').length, color: 'text-blue-700' },
    { key: 'fulfillment', val: 'packed', label: 'Ожидают приёмки', count: orders.filter(o => o.fulfillmentStatus === 'packed').length, color: 'text-indigo-700' },
    { key: 'status', val: 'delivered', label: 'Доставлены', count: orders.filter(o => o.status === 'delivered').length, color: 'text-emerald-800' },
  ];

  const problemCount = orders.filter(o => o.status === 'cancelled' || o.fulfillmentStatus === 'discrepancy').length;

  return (
    <div data-testid="orders-page" className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Заказы</h1>
          <p className="text-sm text-gray-500 mt-1">Управление заказами покупателей и операционный мониторинг исполнения</p>
        </div>
        <div className="flex items-center gap-3">
          <Link
            to="/orders/problems"
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-semibold bg-amber-50 text-amber-800 border border-amber-200 hover:bg-amber-100 transition-colors shadow-sm"
          >
            <AlertTriangle className="w-4 h-4 text-amber-600" />
            <span>Очередь проблем {problemCount > 0 ? `(${problemCount})` : ''}</span>
          </Link>
          <button
            onClick={fetchOrders}
            disabled={isLoading}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      {/* Operational KPI Chips */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {kpis.map((kpi, idx) => {
          const isSelected = searchParams.get(kpi.key) === kpi.val;
          return (
            <button
              key={idx}
              onClick={() => updateParam(kpi.key, isSelected ? '' : kpi.val)}
              className={`p-3.5 rounded-xl border text-left transition-all ${
                isSelected
                  ? 'bg-indigo-50/70 border-indigo-300 shadow-sm ring-1 ring-indigo-200'
                  : 'bg-white border-gray-200 hover:border-gray-300 shadow-sm'
              }`}
            >
              <div className="text-xs font-medium text-gray-500">{kpi.label}</div>
              <div className={`text-xl font-bold mt-1 ${isSelected ? 'text-indigo-700' : 'text-gray-900'}`}>{kpi.count}</div>
            </button>
          );
        })}
      </div>

      {/* Search & Filters */}
      <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex flex-col md:flex-row gap-3 items-stretch md:items-center justify-between">
        <div className="relative flex-1">
          <Search className="absolute left-3.5 top-3 h-4 w-4 text-gray-400" />
          <input
            type="text"
            placeholder="Поиск по номеру заказа, покупателю, email, телефону..."
            value={searchQuery}
            onChange={(e) => updateParam('q', e.target.value)}
            className="w-full pl-10 pr-4 py-2 text-sm bg-gray-50 border border-gray-200 rounded-xl text-gray-900 placeholder-gray-400 focus:bg-white focus:outline-none focus:border-indigo-500 transition-colors"
          />
        </div>

        <div className="flex flex-wrap gap-2 items-center">
          <div className="flex items-center gap-1.5 text-xs text-gray-500 mr-1">
            <Filter className="h-3.5 w-3.5" />
            <span>Фильтры:</span>
          </div>

          <select
            value={statusFilter}
            onChange={(e) => updateParam('status', e.target.value)}
            className="px-3 py-2 text-xs bg-gray-50 border border-gray-200 rounded-lg text-gray-700 focus:bg-white focus:outline-none focus:border-indigo-500"
          >
            <option value="">Все статусы заказа</option>
            <option value="awaiting_payment">Ожидает оплаты</option>
            <option value="paid">Оплачен</option>
            <option value="assembling">Собирается</option>
            <option value="packed">Упакован</option>
            <option value="shipped">В пути</option>
            <option value="delivered">Доставлен</option>
            <option value="cancelled">Отменён</option>
          </select>

          <select
            value={fulfillmentFilter}
            onChange={(e) => updateParam('fulfillment', e.target.value)}
            className="px-3 py-2 text-xs bg-gray-50 border border-gray-200 rounded-lg text-gray-700 focus:bg-white focus:outline-none focus:border-indigo-500"
          >
            <option value="">Все сборки</option>
            <option value="pending">Ожидает сборки</option>
            <option value="assembling">Собирается</option>
            <option value="packed">Ожидает приёмки</option>
            <option value="accepted">Принята на хабе</option>
            <option value="discrepancy">Расхождение</option>
          </select>

          {(statusFilter || fulfillmentFilter || searchQuery) && (
            <button
              onClick={() => setSearchParams(new URLSearchParams())}
              className="px-2.5 py-1 text-xs text-rose-600 hover:text-rose-700 font-medium transition-colors"
            >
              Сбросить
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="p-4 bg-rose-50 border border-rose-200 text-rose-800 rounded-xl flex items-center gap-3">
          <AlertCircle className="h-5 w-5 shrink-0 text-rose-600" />
          <span className="text-sm font-medium">{error}</span>
        </div>
      )}

      {/* Orders Table */}
      {isLoading ? (
        <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-3 text-sm text-gray-500 font-medium">Загрузка заказов...</p>
        </div>
      ) : orders.length === 0 ? (
        <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
          <ShoppingCart className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-3 text-base font-semibold text-gray-900">Заказы не найдены</h3>
          <p className="mt-1 text-sm text-gray-500">Попробуйте изменить параметры поиска или сбросить фильтры.</p>
        </div>
      ) : (
        <div className="bg-white border border-gray-200 rounded-xl overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table data-testid="orders-table" className="min-w-full divide-y divide-gray-200 text-sm text-left">
              <thead className="bg-gray-50 text-gray-600 text-xs font-semibold uppercase tracking-wider">
                <tr>
                  <th className="px-4 py-3.5">Заказ</th>
                  <th className="px-4 py-3.5">Покупатель</th>
                  <th className="px-4 py-3.5">Создан</th>
                  <th className="px-4 py-3.5">Оплата</th>
                  <th className="px-4 py-3.5">Состав</th>
                  <th className="px-4 py-3.5">Сборка</th>
                  <th className="px-4 py-3.5 text-right">Итого</th>
                  <th className="px-4 py-3.5">Статус</th>
                  <th className="px-4 py-3.5 text-right">Действие</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 text-gray-700">
                {orders.map((order) => {
                  const st = orderStatusMap[order.status] || { label: order.statusLabel || order.status, bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };
                  const fulSt = fulfillmentStatusMap[order.fulfillmentStatus] || { label: order.fulfillmentStatus, bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };

                  return (
                    <tr
                      key={order.id}
                      onClick={() => navigate(`/orders/${order.id}`)}
                      className="hover:bg-gray-50/80 cursor-pointer transition-colors"
                    >
                      <td className="px-4 py-3.5 font-bold text-indigo-600 whitespace-nowrap">
                        {formatOrderNumber(order)}
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="font-medium text-gray-900">{order.customerName || 'Покупатель'}</div>
                        <div className="text-xs text-gray-500">{order.customerPhone || order.customerEmail}</div>
                      </td>
                      <td className="px-4 py-3.5 text-gray-500 text-xs whitespace-nowrap">
                        {formatDateTime(order.createdAt)}
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                          order.status === 'paid' || order.status === 'delivered'
                            ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                            : 'bg-amber-50 text-amber-700 border border-amber-200'
                        }`}>
                          {order.status === 'paid' || order.status === 'delivered' ? 'Оплачен' : 'Ожидает'}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <div className="text-xs text-gray-700">
                          {order.itemPositionsCount} {order.itemPositionsCount === 1 ? 'позиция' : order.itemPositionsCount > 1 && order.itemPositionsCount < 5 ? 'позиции' : 'позиций'} &middot; {order.unitsCount} {order.unitsCount === 1 ? 'ед.' : 'ед.'}
                        </div>
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <span className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold ${fulSt.bg} ${fulSt.text}`}>
                          {fulSt.label}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 text-right font-bold text-gray-900 whitespace-nowrap">
                        {formatMoney(order.totalPriceCents || order.totalAmount * 100 || 0)}
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${st.bg} ${st.text}`}>
                          {st.label}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 text-right">
                        <button
                          data-testid={`order-open-${order.id}`}
                          data-fulfillments-count={order.fulfillmentsCount}
                          onClick={(e) => {
                            e.stopPropagation();
                            navigate(`/orders/${order.id}`);
                          }}
                          className="inline-flex items-center gap-1 text-xs font-semibold text-indigo-600 bg-indigo-50 hover:bg-indigo-100 px-3 py-1.5 rounded-lg border border-indigo-200 transition-colors"
                        >
                          <span>Открыть</span>
                          <ArrowRight className="h-3.5 w-3.5" />
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalCount > limit && (
            <div className="px-4 py-3 border-t border-gray-200 bg-gray-50 flex items-center justify-between text-xs text-gray-600">
              <div>
                Показано {(page - 1) * limit + 1}–{Math.min(page * limit, totalCount)} из {totalCount} заказов
              </div>
              <div className="flex space-x-2">
                <button
                  disabled={page === 1}
                  onClick={() => updateParam('page', String(page - 1))}
                  className="px-3 py-1 bg-white border border-gray-200 hover:bg-gray-50 disabled:opacity-40 rounded-lg text-gray-700 transition-colors"
                >
                  Назад
                </button>
                <button
                  disabled={page * limit >= totalCount}
                  onClick={() => updateParam('page', String(page + 1))}
                  className="px-3 py-1 bg-white border border-gray-200 hover:bg-gray-50 disabled:opacity-40 rounded-lg text-gray-700 transition-colors"
                >
                  Вперёд
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
