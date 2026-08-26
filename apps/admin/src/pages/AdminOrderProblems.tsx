import { useEffect, useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { ShieldAlert, ArrowRight, CheckCircle2, ArrowLeft, RefreshCw } from 'lucide-react';
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
  actionLabel: string;
}

export function AdminOrderProblems() {
  const navigate = useNavigate();
  const [problems, setProblems] = useState<ProblemItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const fetchProblems = async () => {
    try {
      setIsLoading(true);
      const [ordersData, fulfillmentsData] = await Promise.all([
        getAdminOrders({ limit: 100 }),
        getAdminFulfillments({}),
      ]);

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
          actionLabel: 'Открыть заказ',
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
          recommendedAction: 'Создать единое отправление вручную в разделе доставки',
          actionUrl: `/orders/${f.orderId}`,
          actionLabel: 'Открыть заказ',
        });
      });

      // 3. Paid Orders waiting for warehouse picking
      ordersData.items.filter(o => o.status === 'paid' && o.fulfillmentStatus === 'pending').forEach(o => {
        const matchingFulf = fulfillmentsData.find(f => f.orderId === o.id);
        const pickingUrl = matchingFulf?.id ? `/fulfillment/picking/${matchingFulf.id}` : '/fulfillment/picking';

        list.push({
          id: `noful-${o.id}`,
          type: 'no_fulfillments',
          title: 'Оплаченный заказ ожидает сборки на складе',
          severity: 'medium',
          orderId: o.id,
          orderNumber: o.orderNumber,
          fulfillmentId: matchingFulf?.id,
          timestamp: o.createdAt || new Date().toISOString(),
          recommendedAction: 'Перейти к сборке',
          actionUrl: pickingUrl,
          actionLabel: 'Перейти к сборке',
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
          <h1 className="text-2xl font-bold text-gray-900">Очередь операционных проблем</h1>
          <p className="text-sm text-gray-500 mt-1">Автоматическое выявление расхождений, задержек и конфликтов статусов</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={fetchProblems}
            disabled={isLoading}
            className="inline-flex items-center px-3.5 py-2 rounded-xl text-xs font-medium bg-white text-gray-700 hover:bg-gray-50 border border-gray-200 transition-colors shadow-sm disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </div>

      <div className="space-y-4">
        {isLoading ? (
          <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-600 mx-auto" />
            <p className="mt-3 text-sm text-gray-500 font-medium">Поиск операционных проблем...</p>
          </div>
        ) : problems.length === 0 ? (
          <div className="text-center py-16 bg-white rounded-xl border border-gray-200 shadow-sm space-y-3">
            <CheckCircle2 className="mx-auto h-12 w-12 text-emerald-500" />
            <h3 className="text-lg font-bold text-gray-900">Операционные проблемы не обнаружены</h3>
            <p className="text-sm text-gray-500 max-w-md mx-auto">Все заказы, сборки и отгрузки обрабатываются в штатном режиме.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {problems.map((prob) => (
              <div
                key={prob.id}
                className="bg-white border border-gray-200 rounded-xl p-5 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-sm hover:border-gray-300 transition-all"
              >
                <div className="flex items-start gap-4">
                  <div className="p-2.5 bg-amber-50 border border-amber-200 rounded-xl text-amber-600 shrink-0">
                    <ShieldAlert className="h-5 w-5" />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h4 className="font-bold text-gray-900 text-base">{prob.title}</h4>
                      <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold ${
                        prob.severity === 'critical'
                          ? 'bg-rose-50 text-rose-700 border border-rose-200'
                          : prob.severity === 'high'
                          ? 'bg-amber-50 text-amber-700 border border-amber-200'
                          : 'bg-blue-50 text-blue-700 border border-blue-200'
                      }`}>
                        {prob.severity === 'critical' ? 'Критическая' : prob.severity === 'high' ? 'Высокая' : 'Обычная'}
                      </span>
                    </div>
                    <p className="text-xs text-gray-500 mt-1">
                      {formatOrderNumber({ id: prob.orderId, orderNumber: prob.orderNumber })}
                      {prob.sellerName && ` • ${prob.sellerName}`}
                      {` • ${formatDateTime(prob.timestamp)}`}
                    </p>
                    <p className="text-xs text-indigo-600 mt-1.5 font-medium">
                      Рекомендуемое действие: {prob.recommendedAction}
                    </p>
                  </div>
                </div>

                <button
                  onClick={() => navigate(prob.actionUrl)}
                  className="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-xs rounded-xl inline-flex items-center justify-center gap-1.5 shrink-0 transition-colors shadow-sm"
                >
                  <span>{prob.actionLabel}</span>
                  <ArrowRight className="w-3.5 h-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
