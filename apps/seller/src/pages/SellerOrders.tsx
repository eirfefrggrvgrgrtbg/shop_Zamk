import React, { useEffect, useState, useCallback } from 'react';
import { getSellerOrders, getSellerOrderSummary } from '@zamk/api-client/src/seller';
import type { SellerOrder } from '@zamk/api-client/src/types';
import { ChevronDown, ChevronUp, Package, AlertCircle, TrendingUp, RefreshCcw } from 'lucide-react';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  awaiting_payment: { label: 'Ожидает оплаты', color: 'bg-yellow-100 text-yellow-800' },
  paid: { label: 'Оплачен', color: 'bg-emerald-100 text-emerald-800' },
  cancelled: { label: 'Отменён', color: 'bg-red-100 text-red-800' },
  returned: { label: 'Возврат', color: 'bg-rose-100 text-rose-800' },
  refunded: { label: 'Возмещён', color: 'bg-gray-100 text-gray-800' },
};

const SHIPMENT_STATUS_LABELS: Record<string, string> = {
  pending: 'В обработке ZAMK',
  assembling: 'В обработке ZAMK',
  packed: 'В обработке ZAMK',
  shipped: 'Передан в доставку',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
};

export function SellerOrders() {
  const [orders, setOrders] = useState<SellerOrder[]>([]);
  const [summary, setSummary] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [expandedOrderId, setExpandedOrderId] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const [ordersData, summaryData] = await Promise.all([
        getSellerOrders(),
        getSellerOrderSummary()
      ]);
      setOrders(ordersData.items || []);
      setSummary(summaryData);
    } catch (err: any) {
      if (err.status === 403) {
        setError('Недостаточно прав для просмотра продаж.');
      } else {
        setError('Не удалось загрузить данные продаж.');
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  if (isLoading) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center flex-col items-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black mb-4"></div>
        <div className="text-ash">Загружаем продажи...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center">
        <div className="bg-red-50 text-red-600 p-6 rounded-2xl flex items-center gap-3">
          <AlertCircle className="w-6 h-6" />
          <span>{error}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <h1 className="text-2xl font-bold text-graphite dark:text-white mb-2">Продажи</h1>
      <p className="text-ash mb-8">
        Заказы покупателей с вашими товарами. Сборкой и доставкой занимается ZAMK.
      </p>

      {/* Summary Cards */}
      {summary && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          <div className="bg-white dark:bg-white/5 rounded-2xl p-5 border border-border-soft dark:border-white/10">
            <div className="flex items-center gap-3 mb-2 text-ash">
              <Package className="w-5 h-5" />
              <span className="font-medium">За сегодня</span>
            </div>
            <div className="text-2xl font-bold text-graphite dark:text-white mb-1">
              {summary.todayUnits} шт.
            </div>
            <div className="text-sm text-ash">{summary.todayOrders} заказов</div>
          </div>
          
          <div className="bg-white dark:bg-white/5 rounded-2xl p-5 border border-border-soft dark:border-white/10">
            <div className="flex items-center gap-3 mb-2 text-ash">
              <TrendingUp className="w-5 h-5" />
              <span className="font-medium">За 7 дней</span>
            </div>
            <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
              {currencyFormatter.format(summary.last7dGross / 100)}
            </div>
          </div>

          <div className="bg-white dark:bg-white/5 rounded-2xl p-5 border border-border-soft dark:border-white/10">
            <div className="flex items-center gap-3 mb-2 text-ash">
              <TrendingUp className="w-5 h-5 text-indigo-500" />
              <span className="font-medium">За 30 дней</span>
            </div>
            <div className="text-2xl font-bold text-graphite dark:text-white">
              {currencyFormatter.format(summary.last30dGross / 100)}
            </div>
          </div>

          <div className="bg-white dark:bg-white/5 rounded-2xl p-5 border border-border-soft dark:border-white/10">
            <div className="flex items-center gap-3 mb-2 text-ash">
              <RefreshCcw className="w-5 h-5 text-rose-500" />
              <span className="font-medium">Возвраты (всего)</span>
            </div>
            <div className="text-2xl font-bold text-rose-600 dark:text-rose-400 mb-1">
              {currencyFormatter.format(summary.returnsAmount / 100)}
            </div>
            <div className="text-sm text-ash">{summary.returnsCount} шт.</div>
          </div>
        </div>
      )}

      {orders.length === 0 ? (
        <div className="py-12 text-center text-ash bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10">
          У вас пока нет продаж.
        </div>
      ) : (
        <div className="bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-border-soft dark:border-white/10 text-sm text-ash">
                  <th className="p-4 font-medium">Заказ</th>
                  <th className="p-4 font-medium">Дата</th>
                  <th className="p-4 font-medium">Статус заказа</th>
                  <th className="p-4 font-medium text-right">Товаров</th>
                  <th className="p-4 font-medium text-right">Сумма (Ваша)</th>
                  <th className="p-4 font-medium w-10"></th>
                </tr>
              </thead>
              <tbody>
                {orders.map((order) => {
                  const statusConfig = STATUS_LABELS[order.commercialStatus] || { label: order.commercialStatus, color: 'bg-gray-100 text-gray-800' };
                  const deliveryLabel = SHIPMENT_STATUS_LABELS[order.deliveryStatus] || order.deliveryStatus;
                  const isExpanded = expandedOrderId === order.id;
                  const shortOrderId = order.orderNumber || order.id.split('-')[0];
                  
                  return (
                    <React.Fragment key={order.id}>
                      <tr 
                        onClick={() => setExpandedOrderId(isExpanded ? null : order.id)}
                        className="border-b border-border-soft dark:border-white/10 cursor-pointer hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
                      >
                        <td className="p-4 font-medium text-graphite dark:text-white">
                          #{shortOrderId}
                        </td>
                        <td className="p-4 text-sm text-ash">
                          {new Date(order.createdAt).toLocaleDateString('ru-RU')}
                        </td>
                        <td className="p-4">
                          <div className="flex flex-col gap-1 items-start">
                            <span className={`px-2 py-0.5 text-xs font-semibold rounded-full ${statusConfig.color}`}>
                              {statusConfig.label}
                            </span>
                            {!['cancelled', 'returned', 'refunded'].includes(order.commercialStatus) && (
                              <span className="text-[11px] text-ash bg-gray-100 dark:bg-white/10 px-2 py-0.5 rounded-full">
                                🚚 {deliveryLabel}
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="p-4 text-sm font-medium text-graphite dark:text-white text-right">
                          {order.sellerUnits} шт.
                        </td>
                        <td className="p-4 font-semibold text-graphite dark:text-white text-right">
                          {currencyFormatter.format(order.sellerGrossAmount / 100)}
                        </td>
                        <td className="p-4 text-right">
                          <button className="text-ash hover:text-graphite dark:hover:text-white transition-colors">
                            {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                          </button>
                        </td>
                      </tr>
                      
                      {isExpanded && (
                        <tr>
                          <td colSpan={6} className="p-0 border-b border-border-soft dark:border-white/10">
                            <div className="p-6 bg-gray-50/50 dark:bg-black/20">
                              <h3 className="font-semibold text-graphite dark:text-white flex items-center gap-2 mb-4">
                                <Package className="w-4 h-4" />
                                Ваши товары в заказе #{shortOrderId}
                              </h3>
                              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {order.items?.map((item: any) => (
                                  <div key={item.id} className="flex gap-4 p-4 bg-white dark:bg-black/40 rounded-xl border border-border-soft dark:border-white/10">
                                    <div className="w-16 h-16 rounded-lg bg-gray-100 overflow-hidden shrink-0">
                                      {item.imageUrl ? (
                                        <img src={item.imageUrl} alt={item.title} className="w-full h-full object-cover" />
                                      ) : (
                                        <div className="w-full h-full flex items-center justify-center text-gray-400">
                                          <Package className="w-8 h-8 opacity-20" />
                                        </div>
                                      )}
                                    </div>
                                    <div className="flex-1 min-w-0">
                                      <h4 className="font-medium text-graphite dark:text-white truncate" title={item.title}>
                                        {item.title}
                                      </h4>
                                      <div className="text-xs text-ash mt-0.5 flex flex-wrap gap-2">
                                        {item.variantSize && <span>Размер: {item.variantSize}</span>}
                                        {item.variantColor && <span>Цвет: {item.variantColor}</span>}
                                        {(item.sku) && (
                                          <span>SKU: {item.sku}</span>
                                        )}
                                      </div>
                                      <div className="flex items-center justify-between mt-2">
                                        <span className="text-sm text-ash">{item.quantity} шт. × {currencyFormatter.format(item.priceCents / 100)}</span>
                                        <span className="font-medium text-graphite dark:text-white">{currencyFormatter.format(item.subtotalPriceCents / 100)}</span>
                                      </div>
                                    </div>
                                  </div>
                                ))}
                              </div>
                            </div>
                          </td>
                        </tr>
                      )}
                    </React.Fragment>
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
