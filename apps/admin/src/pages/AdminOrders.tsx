import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Search, AlertCircle, ShoppingCart, Filter, ArrowRight } from 'lucide-react';
import { AdminOrdersTabs } from '../components/orders/AdminOrdersTabs';
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

  // Operational KPI Cards
  const kpis = [
    { key: 'status', val: 'paid', label: 'Новые оплаченные', count: orders.filter(o => o.status === 'paid').length, color: 'text-emerald-400' },
    { key: 'status', val: 'assembling', label: 'В работе', count: orders.filter(o => o.status === 'assembling').length, color: 'text-blue-400' },
    { key: 'fulfillment', val: 'packed', label: 'Ожидают приёмки', count: orders.filter(o => o.fulfillmentStatus === 'packed').length, color: 'text-indigo-400' },
    { key: 'problem', val: 'yes', label: 'С проблемами', count: orders.filter(o => o.status === 'cancelled' || o.fulfillmentStatus === 'discrepancy').length, color: 'text-amber-400' },
    { key: 'status', val: 'delivered', label: 'Доставлены сегодня', count: orders.filter(o => o.status === 'delivered').length, color: 'text-emerald-300' },
  ];

  return (
    <div data-testid="orders-page" className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Операционный центр заказов</h1>
          <p className="text-sm text-slate-400 mt-1">Управление заказами, сборками продавцов и приёмкой на хабе ZAMK</p>
        </div>
      </div>

      <AdminOrdersTabs />

      <div className="px-4 sm:px-6 space-y-6">
        {/* Operational KPIs */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
          {kpis.map((kpi, idx) => (
            <button
              key={idx}
              onClick={() => updateParam(kpi.key, searchParams.get(kpi.key) === kpi.val ? '' : kpi.val)}
              className={`p-4 rounded-xl border text-left transition-all ${
                searchParams.get(kpi.key) === kpi.val
                  ? 'bg-indigo-950/80 border-indigo-500 shadow-lg shadow-indigo-950/50'
                  : 'bg-slate-900/60 border-slate-800 hover:border-slate-700'
              }`}
            >
              <div className="text-xs text-slate-400 font-medium">{kpi.label}</div>
              <div className={`text-2xl font-bold mt-1 ${kpi.color}`}>{kpi.count}</div>
            </button>
          ))}
        </div>

        {/* Search & Popover Filters */}
        <div className="bg-slate-900/70 p-4 rounded-xl border border-slate-800 flex flex-col md:flex-row gap-3 items-stretch md:items-center justify-between">
          <div className="relative flex-1">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-slate-400" />
            <input
              type="text"
              placeholder="Поиск по номеру заказа, покупателю, email, тел, сборке..."
              value={searchQuery}
              onChange={(e) => updateParam('q', e.target.value)}
              className="w-full pl-10 pr-4 py-2 text-sm bg-slate-950 border border-slate-700 rounded-xl text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div className="flex flex-wrap gap-2 items-center">
            <div className="flex items-center gap-1.5 text-xs text-slate-400 mr-1">
              <Filter className="h-3.5 w-3.5" />
              <span>Фильтры:</span>
            </div>

            <select
              value={statusFilter}
              onChange={(e) => updateParam('status', e.target.value)}
              className="px-3 py-1.5 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
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
              className="px-3 py-1.5 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
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
                className="px-2.5 py-1 text-xs text-rose-400 hover:text-rose-300 font-medium"
              >
                Сбросить
              </button>
            )}
          </div>
        </div>

        {error && (
          <div className="p-4 bg-rose-950/80 border border-rose-800 text-rose-300 rounded-xl flex items-center gap-3">
            <AlertCircle className="h-5 w-5 shrink-0" />
            <span className="text-sm font-medium">{error}</span>
          </div>
        )}

        {/* Orders Table */}
        {isLoading ? (
          <div className="text-center py-16 bg-slate-900/50 rounded-xl border border-slate-800">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto" />
            <p className="mt-3 text-sm text-slate-400">Загрузка заказов...</p>
          </div>
        ) : orders.length === 0 ? (
          <div className="text-center py-16 bg-slate-900/50 rounded-xl border border-slate-800">
            <ShoppingCart className="mx-auto h-12 w-12 text-slate-600" />
            <h3 className="mt-3 text-base font-semibold text-slate-200">Заказы не найдены</h3>
            <p className="mt-1 text-sm text-slate-400">Попробуйте изменить параметры поиска или сбросить фильтры.</p>
          </div>
        ) : (
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
            <div className="overflow-x-auto">
              <table data-testid="orders-table" className="min-w-full divide-y divide-slate-800 text-sm text-left">
                <thead className="bg-slate-950/70 text-slate-400 text-xs font-semibold uppercase tracking-wider">
                  <tr>
                    <th className="px-4 py-3.5">Заказ</th>
                    <th className="px-4 py-3.5">Покупатель</th>
                    <th className="px-4 py-3.5">Создан</th>
                    <th className="px-4 py-3.5">Оплата</th>
                    <th className="px-4 py-3.5">Состав</th>
                    <th className="px-4 py-3.5">Сборки</th>
                    <th className="px-4 py-3.5 text-right">Итого</th>
                    <th className="px-4 py-3.5">Статус</th>
                    <th className="px-4 py-3.5 text-right">Действие</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {orders.map((order) => {
                    const st = orderStatusMap[order.status] || { label: order.statusLabel || order.status, bg: 'bg-slate-800', text: 'text-slate-300' };
                    const fulSt = fulfillmentStatusMap[order.fulfillmentStatus] || { label: order.fulfillmentStatus, bg: 'bg-slate-800', text: 'text-slate-300' };

                    return (
                      <tr
                        key={order.id}
                        onClick={() => navigate(`/orders/${order.id}`)}
                        className="hover:bg-slate-800/40 cursor-pointer transition-colors"
                      >
                        <td className="px-4 py-3.5 font-bold text-indigo-400 whitespace-nowrap">
                          {formatOrderNumber(order)}
                        </td>
                        <td className="px-4 py-3.5">
                          <div className="font-medium text-white">{order.customerName || 'Покупатель'}</div>
                          <div className="text-xs text-slate-400">{order.customerPhone || order.customerEmail}</div>
                        </td>
                        <td className="px-4 py-3.5 text-slate-400 text-xs whitespace-nowrap">
                          {formatDateTime(order.createdAt)}
                        </td>
                        <td className="px-4 py-3.5">
                          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                            order.status === 'paid' || order.status === 'delivered' ? 'bg-emerald-950 text-emerald-300 border border-emerald-800/60' : 'bg-amber-950 text-amber-300 border border-amber-800/60'
                          }`}>
                            {order.status === 'paid' || order.status === 'delivered' ? 'Оплачен' : 'Ожидает'}
                          </span>
                        </td>
                        <td className="px-4 py-3.5 whitespace-nowrap">
                          <div className="text-sm text-slate-300">
                            {order.itemPositionsCount} позиции &middot; {order.unitsCount} единицы
                          </div>
                        </td>
                        <td className="px-4 py-3.5">
                          <div className="font-medium text-white mb-1">
                            {order.fulfillmentsCount} {order.fulfillmentsCount === 1 ? 'сборка' : order.fulfillmentsCount > 1 && order.fulfillmentsCount < 5 ? 'сборки' : 'сборок'}
                          </div>
                          <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold ${fulSt.bg} ${fulSt.text} border border-slate-700`}>
                            {fulSt.label}
                          </span>
                        </td>
                        <td className="px-4 py-3.5 text-right font-bold text-white whitespace-nowrap">
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
                            className="inline-flex items-center gap-1 text-xs font-bold text-indigo-400 hover:text-indigo-300 bg-indigo-950/60 px-3 py-1.5 rounded-lg border border-indigo-800/60 hover:bg-indigo-900/60 transition-all"
                          >
                            <span>Открыть заказ</span>
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
              <div className="px-4 py-3.5 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between text-xs text-slate-400">
                <div>
                  Показано {(page - 1) * limit + 1}–{Math.min(page * limit, totalCount)} из {totalCount} заказов
                </div>
                <div className="flex space-x-2">
                  <button
                    disabled={page === 1}
                    onClick={() => updateParam('page', String(page - 1))}
                    className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded-lg"
                  >
                    Назад
                  </button>
                  <button
                    disabled={page * limit >= totalCount}
                    onClick={() => updateParam('page', String(page + 1))}
                    className="px-3 py-1 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded-lg"
                  >
                    Вперёд
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
