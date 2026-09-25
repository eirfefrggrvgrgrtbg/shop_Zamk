import React, { useEffect, useState, useCallback } from 'react';
import { getSellerOrders, getSellerOrderSummary } from '@zamk/api-client/src/seller';
import type { SellerOrder } from '@zamk/api-client/src/types';
import { ChevronDown, ChevronUp, Package, AlertCircle, TrendingUp, RefreshCcw } from 'lucide-react';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface, SellerKpiCard, SellerTableShell } from '../components/SellerSurface';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  awaiting_payment: { label: 'Ожидает оплаты', color: 'bg-yellow-100 text-yellow-800' },
  paid: { label: 'Оплачен', color: 'bg-emerald-100 text-emerald-800' },
  assembling: { label: 'Собирается', color: 'bg-blue-100 text-blue-800' },
  packed: { label: 'Собран', color: 'bg-indigo-100 text-indigo-800' },
  shipped: { label: 'Отгружен', color: 'bg-blue-100 text-blue-800' },
  delivered: { label: 'Доставлен', color: 'bg-emerald-100 text-emerald-800' },
  cancelled: { label: 'Отменён', color: 'bg-red-100 text-red-800' },
  has_return: { label: 'Есть возврат', color: 'bg-orange-100 text-orange-800' },
  fully_returned: { label: 'Возвращён', color: 'bg-rose-100 text-rose-800' },
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
      <SellerPageFrame variant="summary">
        <SellerSurface className="py-20 flex flex-col justify-center items-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-gray-900 mb-4"></div>
          <div className="text-sm text-gray-500">Загружаем продажи...</div>
        </SellerSurface>
      </SellerPageFrame>
    );
  }

  if (error) {
    return (
      <SellerPageFrame variant="summary">
        <div className="rounded-lg border border-red-200 bg-red-50 p-6 flex items-center gap-3 text-red-600">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="summary">
      <SellerPageHeader
        eyebrow="Продажи"
        title="Заказы"
        description="Заказы покупателей с вашими товарами. Сборкой и доставкой занимается ZAMK."
      />

      {/* Summary Cards */}
      {summary && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <SellerKpiCard
            label="За сегодня"
            value={`${summary.todayUnits} шт.`}
            supportText={`${summary.todayOrders} заказов`}
            icon={Package}
          />
          <SellerKpiCard
            label="За 7 дней"
            value={currencyFormatter.format(summary.last7dGross / 100)}
            accent="positive"
            icon={TrendingUp}
          />
          <SellerKpiCard
            label="За 30 дней"
            value={currencyFormatter.format(summary.last30dGross / 100)}
            icon={TrendingUp}
          />
          <SellerKpiCard
            label="Возвраты (всего)"
            value={currencyFormatter.format(summary.returnsAmount / 100)}
            supportText={`${summary.returnsCount} шт.`}
            accent="danger"
            icon={RefreshCcw}
          />
        </div>
      )}

      {orders.length === 0 ? (
        <SellerSurface className="py-12 text-center text-gray-500">
          У вас пока нет продаж.
        </SellerSurface>
      ) : (
        <SellerTableShell>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-gray-50/60 dark:bg-white/[0.02] border-b border-gray-200 dark:border-white/10 text-xs font-semibold uppercase tracking-wider text-gray-500">
                  <th className="px-4 py-3.5">Заказ</th>
                  <th className="px-4 py-3.5">Дата</th>
                  <th className="px-4 py-3.5">Статус заказа</th>
                  <th className="px-4 py-3.5 text-right">Товаров</th>
                  <th className="px-4 py-3.5 text-right">Сумма (Ваша)</th>
                  <th className="px-4 py-3.5 w-10"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                {orders.map((order) => {
                  const statusConfig = STATUS_LABELS[order.commercialStatus] || { label: order.commercialStatus, color: 'bg-gray-100 text-gray-800' };
                  const deliveryLabel = SHIPMENT_STATUS_LABELS[order.deliveryStatus] || order.deliveryStatus;
                  const isExpanded = expandedOrderId === order.id;
                  const shortOrderId = order.orderNumber || order.id.split('-')[0];
                  
                  return (
                    <React.Fragment key={order.id}>
                      <tr 
                        onClick={() => setExpandedOrderId(isExpanded ? null : order.id)}
                        className="cursor-pointer hover:bg-gray-50/60 dark:hover:bg-white/[0.02] transition-colors"
                      >
                        <td className="px-4 py-3.5 font-medium text-gray-900 dark:text-white">
                          #{shortOrderId}
                        </td>
                        <td className="px-4 py-3.5 text-sm text-gray-500">
                          {new Date(order.createdAt).toLocaleDateString('ru-RU')}
                        </td>
                        <td className="px-4 py-3.5">
                          <div className="flex flex-col gap-1 items-start">
                            <span className={`px-2.5 py-0.5 text-xs font-medium rounded-full ${statusConfig.color}`}>
                              {statusConfig.label}
                            </span>
                            {!['cancelled', 'returned', 'refunded'].includes(order.commercialStatus) && (
                              <span className="text-[11px] text-gray-500 bg-gray-100 dark:bg-white/10 px-2 py-0.5 rounded-full">
                                🚚 {deliveryLabel}
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3.5 text-sm font-medium text-gray-900 dark:text-white text-right">
                          {order.sellerUnits} шт.
                        </td>
                        <td className="px-4 py-3.5 font-semibold text-gray-900 dark:text-white text-right">
                          {currencyFormatter.format(order.sellerGrossAmount / 100)}
                        </td>
                        <td className="px-4 py-3.5 text-right">
                          <button className="text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors">
                            {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                          </button>
                        </td>
                      </tr>
                      
                      {isExpanded && (
                        <tr>
                          <td colSpan={6} className="p-0 border-b border-gray-100 dark:border-white/5">
                            <div className="p-5 sm:p-6 bg-gray-50/60 dark:bg-black/20">
                              {order.commercialStatus === 'shipped' && (
                                <div className="mb-4 p-3.5 rounded-xl bg-blue-50/70 dark:bg-blue-950/30 border border-blue-200/70 dark:border-blue-900/40 text-blue-900 dark:text-blue-200 text-xs flex items-center gap-2.5" data-testid="shipped-observer-notice">
                                  <Package className="w-4 h-4 text-blue-600 dark:text-blue-400 shrink-0" />
                                  <span>
                                    Заказ отгружен со склада ZAMK и передан в доставку. От продавца действий не требуется.
                                  </span>
                                </div>
                              )}
                              <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2 mb-4">
                                <Package className="w-4 h-4 text-gray-400" />
                                Ваши товары в заказе #{shortOrderId}
                              </h3>
                              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {order.items?.map((item: any) => (
                                  <div key={item.id} className="flex gap-4 p-4 bg-white dark:bg-black/40 rounded-lg border border-gray-200 dark:border-white/10">
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
                                      <h4 className="font-medium text-gray-900 dark:text-white truncate" title={item.title}>
                                        {item.title}
                                      </h4>
                                      <div className="text-xs text-gray-500 mt-0.5 flex flex-wrap gap-2">
                                        {item.variantSize && <span>Размер: {item.variantSize}</span>}
                                        {item.variantColor && <span>Цвет: {item.variantColor}</span>}
                                        {(item.sku) && (
                                          <span>SKU: {item.sku}</span>
                                        )}
                                      </div>
                                      <div className="flex items-center justify-between mt-2">
                                        <span className="text-sm text-gray-500">{item.quantity} шт. × {currencyFormatter.format(item.priceCents / 100)}</span>
                                        <span className="font-medium text-gray-900 dark:text-white">{currencyFormatter.format(item.subtotalPriceCents / 100)}</span>
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
        </SellerTableShell>
      )}
    </SellerPageFrame>
  );
}
