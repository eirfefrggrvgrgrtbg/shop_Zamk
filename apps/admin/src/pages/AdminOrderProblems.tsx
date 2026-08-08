import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert, ArrowRight, CheckCircle2 } from 'lucide-react';
import { AdminOrdersTabs } from '../components/orders/AdminOrdersTabs';
import { formatOrderNumber, formatDateTime } from '../utils/orderFormatters';
import { getAdminOrders, getAdminFulfillments } from '../api/adminOrders';

interface ProblemItem {
  id: string;
  type: 'discrepancy' | 'overdue' | 'no_fulfillments' | 'no_shipment' | 'cancelled_active';
  title: string;
  severity: 'high' | 'critical' | 'medium';
  orderId: string;
  orderNumber?: string | null;
  fulfillmentId?: string;
  sellerName?: string | null;
  timestamp: string;
  recommendedAction: string;
  actionUrl: string;
}

export function AdminOrderProblems() {
  const navigate = useNavigate();
  const [problems, setProblems] = useState<ProblemItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const fetchProblems = async () => {
    try {
      setIsLoading(true);
      const ordersData = await getAdminOrders({ limit: 100 });
      const fulfillmentsData = await getAdminFulfillments({});

      const list: ProblemItem[] = [];

      // 1. Receiving Discrepancies
      fulfillmentsData.filter(f => f.status === 'discrepancy').forEach(f => {
        list.push({
          id: `disc-${f.id}`,
          type: 'discrepancy',
          title: 'Расхождение при приёмке на хабе',
          severity: 'high',
          orderId: f.orderId,
          orderNumber: f.orderNumber,
          fulfillmentId: f.id,
          sellerName: f.sellerName,
          timestamp: f.updatedAt || f.createdAt,
          recommendedAction: 'Проверить акт расхождения и связаться с продавцом',
          actionUrl: `/orders/${f.orderId}`,
        });
      });

      // 2. Accepted Fulfillments without Shipment
      fulfillmentsData.filter(f => f.status === 'accepted' && !f.shipmentId).forEach(f => {
        list.push({
          id: `noship-${f.id}`,
          type: 'no_shipment',
          title: 'Сборка принята, но отгрузка не создана',
          severity: 'critical',
          orderId: f.orderId,
          orderNumber: f.orderNumber,
          fulfillmentId: f.id,
          sellerName: f.sellerName,
          timestamp: f.updatedAt || f.createdAt,
          recommendedAction: 'Создать единое отправление вручную в разделе приёмки',
          actionUrl: `/orders/${f.orderId}`,
        });
      });

      // 3. Paid Orders without Fulfillments
      ordersData.items.filter(o => o.status === 'paid' && o.fulfillmentStatus === 'pending').forEach(o => {
        list.push({
          id: `noful-${o.id}`,
          type: 'no_fulfillments',
          title: 'Оплаченный заказ не передан в сборку',
          severity: 'medium',
          orderId: o.id,
          orderNumber: o.orderNumber,
          timestamp: o.createdAt || new Date().toISOString(),
          recommendedAction: 'Уведомить продавцов о необходимости начала сборки',
          actionUrl: `/orders/${o.id}`,
        });
      });

      setProblems(list);
    } catch (_) {
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchProblems();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Очередь операционных проблем</h1>
          <p className="text-sm text-slate-400 mt-1">Автоматическое выявление расхождений, задержек и конфликтов статусов</p>
        </div>
      </div>

      <AdminOrdersTabs />

      <div className="px-4 sm:px-6 space-y-6">
        {isLoading ? (
          <div className="text-center py-16 bg-slate-900/50 rounded-xl border border-slate-800">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto" />
            <p className="mt-3 text-sm text-slate-400">Поиск операционных проблем...</p>
          </div>
        ) : problems.length === 0 ? (
          <div className="text-center py-16 bg-slate-900/50 rounded-xl border border-slate-800 space-y-3">
            <CheckCircle2 className="mx-auto h-12 w-12 text-emerald-400" />
            <h3 className="text-lg font-bold text-white">Операционные проблемы не обнаружены</h3>
            <p className="text-sm text-slate-400">Все заказы, сборки и отгрузки обрабатываются штатно.</p>
          </div>
        ) : (
          <div className="space-y-4">
            {problems.map((prob) => (
              <div
                key={prob.id}
                className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-lg hover:border-slate-700 transition-all"
              >
                <div className="flex items-start gap-4">
                  <div className="p-3 bg-amber-950/80 border border-amber-800/80 rounded-xl text-amber-400 shrink-0">
                    <ShieldAlert className="h-6 w-6" />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h4 className="font-bold text-white text-base">{prob.title}</h4>
                      <span className={`px-2.5 py-0.5 rounded-full text-xs font-bold ${
                        prob.severity === 'critical' ? 'bg-rose-950 text-rose-300 border border-rose-800' : 'bg-amber-950 text-amber-300 border border-amber-800'
                      }`}>
                        {prob.severity === 'critical' ? 'Критическая' : 'Высокая'}
                      </span>
                    </div>
                    <p className="text-xs text-slate-400 mt-1">
                      {formatOrderNumber({ id: prob.orderId, orderNumber: prob.orderNumber })} • {prob.sellerName || 'Продавец ZAMK'} • {formatDateTime(prob.timestamp)}
                    </p>
                    <p className="text-xs text-indigo-300 mt-2 font-medium">
                      Рекомендуемое действие: {prob.recommendedAction}
                    </p>
                  </div>
                </div>

                <button
                  onClick={() => navigate(prob.actionUrl)}
                  className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white font-bold text-xs rounded-xl inline-flex items-center justify-center gap-1.5 shrink-0 transition-colors"
                >
                  <span>Решить проблему</span>
                  <ArrowRight className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
