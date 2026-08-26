import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams, Link } from 'react-router-dom';
import { PackageCheck, Filter, AlertCircle, ArrowRight, ArrowLeft, RefreshCw } from 'lucide-react';
import { formatMoney, formatOrderNumber, fulfillmentStatusMap } from '../utils/orderFormatters';
import { getAdminFulfillments } from '../api/adminOrders';
import type { AdminFulfillment } from '@zamk/api-client/src/types';

export function AdminFulfillmentsList() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const [fulfillments, setFulfillments] = useState<AdminFulfillment[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const statusFilter = searchParams.get('status') || '';

  const updateParam = (key: string, value: string) => {
    const newParams = new URLSearchParams(searchParams);
    if (value) {
      newParams.set(key, value);
    } else {
      newParams.delete(key);
    }
    setSearchParams(newParams);
  };

  const fetchFulfillments = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminFulfillments({
        status: statusFilter || undefined,
      });
      setFulfillments(data);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить очередь сборок.');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchFulfillments();
  }, [searchParams]);

  const kpis = [
    { status: 'assembling', label: 'Собираются', count: fulfillments.filter(f => f.status === 'assembling').length, color: 'text-blue-600' },
    { status: 'packed', label: 'Упакованы', count: fulfillments.filter(f => f.status === 'packed').length, color: 'text-indigo-600' },
    { status: 'accepted', label: 'Приняты на хабе', count: fulfillments.filter(f => f.status === 'accepted').length, color: 'text-emerald-600' },
    { status: 'discrepancy', label: 'С расхождениями', count: fulfillments.filter(f => f.status === 'discrepancy').length, color: 'text-rose-600' },
  ];

  return (
    <div data-testid="fulfillments-page" className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Link
              to="/orders"
              className="inline-flex items-center text-xs font-semibold text-gray-500 hover:text-gray-900 transition-colors"
            >
              <ArrowLeft className="w-3.5 h-3.5 mr-1" />
              К заказам
            </Link>
          </div>
          <h1 className="text-2xl font-bold text-gray-900">Сборки заказов</h1>
          <p className="text-sm text-gray-500 mt-1">Мониторинг статусов сборок по заказам</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={fetchFulfillments}
            disabled={isLoading}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {kpis.map((kpi, idx) => (
          <button
            key={idx}
            onClick={() => updateParam('status', statusFilter === kpi.status ? '' : kpi.status)}
            className={`p-3.5 rounded-xl border text-left transition-all ${
              statusFilter === kpi.status
                ? 'bg-indigo-50/70 border-indigo-300 shadow-sm ring-1 ring-indigo-200'
                : 'bg-white border-gray-200 hover:border-gray-300 shadow-sm'
            }`}
          >
            <div className="text-xs text-gray-500 font-medium">{kpi.label}</div>
            <div className={`text-xl font-bold mt-1 ${kpi.color}`}>{kpi.count}</div>
          </button>
        ))}
      </div>

      {/* Filters */}
      <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm flex flex-wrap gap-3 items-center justify-between">
        <div className="flex items-center gap-2 text-xs text-gray-500">
          <Filter className="h-4 w-4" />
          <span>Фильтр по статусу сборки:</span>
        </div>

        <div className="flex flex-wrap gap-2">
          <select
            value={statusFilter}
            onChange={(e) => updateParam('status', e.target.value)}
            className="px-3 py-2 text-xs bg-gray-50 border border-gray-200 rounded-lg text-gray-700 focus:bg-white focus:outline-none focus:border-indigo-500"
          >
            <option value="">Все статусы</option>
            <option value="assembling">Собирается</option>
            <option value="packed">Упакована</option>
            <option value="accepted">Принята на хабе</option>
            <option value="discrepancy">Расхождение</option>
            <option value="cancelled">Отменена</option>
          </select>

          {statusFilter && (
            <button
              onClick={() => setSearchParams(new URLSearchParams())}
              className="px-2.5 py-1 text-xs text-rose-600 hover:text-rose-700 font-medium"
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

      {/* Fulfillments Table */}
      {isLoading ? (
        <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
          <p className="mt-3 text-sm text-gray-500 font-medium">Загрузка сборок...</p>
        </div>
      ) : fulfillments.length === 0 ? (
        <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
          <PackageCheck className="mx-auto h-12 w-12 text-gray-400" />
          <h3 className="mt-3 text-base font-semibold text-gray-900">Сборки не найдены</h3>
          <p className="mt-1 text-sm text-gray-500">Нет сборок с выбранным статусом.</p>
        </div>
      ) : (
        <div className="bg-white border border-gray-200 rounded-xl overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table data-testid="fulfillments-table" className="min-w-full divide-y divide-gray-200 text-sm text-left">
              <thead className="bg-gray-50 text-gray-600 text-xs font-semibold uppercase tracking-wider">
                <tr>
                  <th className="px-4 py-3.5">Код / Сборка</th>
                  <th className="px-4 py-3.5">Заказ</th>
                  <th className="px-4 py-3.5">Продавец</th>
                  <th className="px-4 py-3.5 text-center">Позиций</th>
                  <th className="px-4 py-3.5 text-right">Сумма</th>
                  <th className="px-4 py-3.5">Статус</th>
                  <th className="px-4 py-3.5 text-right">Действие</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 text-gray-700">
                {fulfillments.map((f) => {
                  const st = fulfillmentStatusMap[f.status] || { label: f.status, bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };

                  return (
                    <tr
                      key={f.id}
                      onClick={() => navigate(`/orders/${f.orderId}`)}
                      className="hover:bg-gray-50/80 cursor-pointer transition-colors"
                    >
                      <td className="px-4 py-3.5 font-mono text-xs font-bold text-indigo-600">
                        {f.receivingCode || `FUL-${f.id.substring(0, 8)}`}
                      </td>
                      <td className="px-4 py-3.5 font-semibold text-gray-900">
                        {formatOrderNumber({ id: f.orderId, orderNumber: f.orderNumber })}
                      </td>
                      <td className="px-4 py-3.5 font-medium text-gray-900">
                        {f.sellerName || 'Склад ZAMK'}
                      </td>
                      <td className="px-4 py-3.5 text-center font-bold text-gray-700">
                        {f.items?.length || 1} поз.
                      </td>
                      <td className="px-4 py-3.5 text-right font-bold text-gray-900 whitespace-nowrap">
                        {formatMoney(f.subtotalCents || 0)}
                      </td>
                      <td className="px-4 py-3.5 whitespace-nowrap">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${st.bg} ${st.text}`}>
                          {st.label}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 text-right">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            navigate(`/orders/${f.orderId}`);
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
        </div>
      )}
    </div>
  );
}
