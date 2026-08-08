import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { PackageCheck, Filter, AlertCircle, ArrowRight } from 'lucide-react';
import { AdminOrdersTabs } from '../components/orders/AdminOrdersTabs';
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
    { status: 'assembling', label: 'Собираются', count: fulfillments.filter(f => f.status === 'assembling').length, color: 'text-blue-400' },
    { status: 'packed', label: 'Упакованы / Приёмка', count: fulfillments.filter(f => f.status === 'packed').length, color: 'text-indigo-400' },
    { status: 'accepted', label: 'Приняты на хабе', count: fulfillments.filter(f => f.status === 'accepted').length, color: 'text-emerald-400' },
    { status: 'discrepancy', label: 'С расхождениями', count: fulfillments.filter(f => f.status === 'discrepancy').length, color: 'text-rose-400' },
  ];

  return (
    <div data-testid="fulfillments-page" className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Очередь сборок продавцов</h1>
          <p className="text-sm text-slate-400 mt-1">Отслеживание статусов упаковки и готовности к приёмке на склад ZAMK</p>
        </div>
      </div>

      <AdminOrdersTabs />

      <div className="px-4 sm:px-6 space-y-6">
        {/* KPI Cards */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {kpis.map((kpi, idx) => (
            <button
              key={idx}
              onClick={() => updateParam('status', statusFilter === kpi.status ? '' : kpi.status)}
              className={`p-4 rounded-xl border text-left transition-all ${
                statusFilter === kpi.status
                  ? 'bg-indigo-950/80 border-indigo-500 shadow-lg'
                  : 'bg-slate-900/60 border-slate-800 hover:border-slate-700'
              }`}
            >
              <div className="text-xs text-slate-400 font-medium">{kpi.label}</div>
              <div className={`text-2xl font-bold mt-1 ${kpi.color}`}>{kpi.count}</div>
            </button>
          ))}
        </div>

        {/* Filters */}
        <div className="bg-slate-900/70 p-4 rounded-xl border border-slate-800 flex flex-wrap gap-3 items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-slate-400">
            <Filter className="h-4 w-4" />
            <span>Фильтр по статусу сборки:</span>
          </div>

          <div className="flex flex-wrap gap-2">
            <select
              value={statusFilter}
              onChange={(e) => updateParam('status', e.target.value)}
              className="px-3 py-1.5 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
            >
              <option value="">Все статусы</option>
              <option value="assembling">Собирается</option>
              <option value="packed">Ожидает приёмки (Упакована)</option>
              <option value="accepted">Принята на хабе</option>
              <option value="discrepancy">Расхождение</option>
              <option value="cancelled">Отменена</option>
            </select>

            {statusFilter && (
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

        {/* Fulfillments Table */}
        {isLoading ? (
          <div className="text-center py-16 bg-slate-900/50 rounded-xl border border-slate-800">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto" />
            <p className="mt-3 text-sm text-slate-400">Загрузка сборок...</p>
          </div>
        ) : fulfillments.length === 0 ? (
          <div className="text-center py-16 bg-slate-900/50 rounded-xl border border-slate-800">
            <PackageCheck className="mx-auto h-12 w-12 text-slate-600" />
            <h3 className="mt-3 text-base font-semibold text-slate-200">Сборки не найдены</h3>
            <p className="mt-1 text-sm text-slate-400">Нет сборок продавцов с выбранным статусом.</p>
          </div>
        ) : (
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl overflow-hidden shadow-xl">
            <div className="overflow-x-auto">
              <table data-testid="fulfillments-table" className="min-w-full divide-y divide-slate-800 text-sm text-left">
                <thead className="bg-slate-950/70 text-slate-400 text-xs font-semibold uppercase tracking-wider">
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
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {fulfillments.map((f) => {
                    const st = fulfillmentStatusMap[f.status] || { label: f.status, bg: 'bg-slate-800', text: 'text-slate-300' };

                    return (
                      <tr
                        key={f.id}
                        onClick={() => navigate(`/orders/${f.orderId}`)}
                        className="hover:bg-slate-800/40 cursor-pointer transition-colors"
                      >
                        <td className="px-4 py-3.5 font-mono text-xs font-bold text-indigo-400">
                          {f.receivingCode || `FUL-${f.id.substring(0, 8)}`}
                        </td>
                        <td className="px-4 py-3.5 font-semibold text-white">
                          {formatOrderNumber({ id: f.orderId, orderNumber: f.orderNumber })}
                        </td>
                        <td className="px-4 py-3.5 font-medium text-white">
                          {f.sellerName || 'Продавец ZAMK'}
                        </td>
                        <td className="px-4 py-3.5 text-center font-bold text-slate-200">
                          {f.items?.length || 1} поз.
                        </td>
                        <td className="px-4 py-3.5 text-right font-bold text-white whitespace-nowrap">
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
                            className="inline-flex items-center gap-1 text-xs font-bold text-indigo-400 hover:text-indigo-300 bg-indigo-950/60 px-3 py-1.5 rounded-lg border border-indigo-800/60 transition-all"
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
    </div>
  );
}
