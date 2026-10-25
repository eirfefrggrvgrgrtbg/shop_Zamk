import React, { useEffect, useState, useCallback } from 'react';
import { getSellerOrders, getSellerOrderSummary } from '@zamk/api-client/src/seller';
import type { SellerOrder, SellerOrderFilter, SellerOrderItem } from '@zamk/api-client/src/types';
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
};

const SHIPMENT_STATUS_LABELS: Record<string, string> = {
  pending: 'В обработке ZAMK',
  paid: 'Ожидает сборки на складе ZAMK',
  assembling: 'В обработке ZAMK',
  packed: 'В обработке ZAMK',
  shipped: 'Передан в доставку',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
};

const NEXT_STEP_TEXT: Record<string, string> = {
  buyer: 'Ожидаем оплату покупателя',
  zamk: 'Следующий шаг — за складом ZAMK',
  none: 'Действий не требуется',
};

const FILTERS: { id: SellerOrderFilter; label: string }[] = [
  { id: 'all', label: 'Все' },
  { id: 'new', label: 'Новые' },
  { id: 'processing', label: 'В обработке ZAMK' },
  { id: 'shipped', label: 'Отгружены' },
  { id: 'delivered', label: 'Доставлены' },
  { id: 'returns', label: 'С возвратами' },
];

const PAGE_SIZE = 20;

function itemReturnBadges(item: SellerOrderItem) {
  const out: { key: string; text: string; color: string }[] = [];
  if (item.returnRequestedUnits) {
    out.push({ key: 'req', text: `Запрошен возврат: ${item.returnRequestedUnits} шт.`, color: 'bg-orange-100 text-orange-800' });
  }
  if (item.returnInProgressUnits) {
    out.push({ key: 'prog', text: `Возврат в обработке: ${item.returnInProgressUnits} шт.`, color: 'bg-amber-100 text-amber-800' });
  }
  if (item.returnRefundedUnits) {
    out.push({ key: 'ref', text: `Возмещено: ${item.returnRefundedUnits} шт.`, color: 'bg-rose-100 text-rose-800' });
  }
  return out;
}

export function SellerOrders() {
  const [orders, setOrders] = useState<SellerOrder[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [summary, setSummary] = useState<any>(null);
  const [filter, setFilter] = useState<SellerOrderFilter>('all');
  const [offset, setOffset] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [expandedOrderId, setExpandedOrderId] = useState<string | null>(null);

  const fetchOrders = useCallback(async () => {
    setIsLoading(true);
    setError('');
    try {
      const ordersData = await getSellerOrders({ filter, limit: PAGE_SIZE, offset });
      setOrders(ordersData.items || []);
      setTotalCount(ordersData.totalCount ?? (ordersData.items || []).length);
    } catch (err: any) {
      setOrders([]);
      setError(err.status === 403 ? 'Недостаточно прав для просмотра продаж.' : 'Не удалось загрузить данные продаж.');
    } finally {
      setIsLoading(false);
    }
  }, [filter, offset]);

  useEffect(() => {
    fetchOrders();
  }, [fetchOrders]);

  useEffect(() => {
    getSellerOrderSummary().then(setSummary).catch(() => setSummary(null));
  }, []);

  const selectFilter = (next: SellerOrderFilter) => {
    if (next === filter) return;
    setExpandedOrderId(null);
    setOffset(0);
    setFilter(next);
  };

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

      <div className="flex flex-wrap gap-2" role="group" aria-label="Фильтр заказов">
        {FILTERS.map((f) => (
          <button
            key={f.id}
            type="button"
            onClick={() => selectFilter(f.id)}
            aria-pressed={filter === f.id}
            data-testid={`orders-filter-${f.id}`}
            className={`px-3.5 py-1.5 text-sm rounded-full border transition-colors ${
              filter === f.id
                ? 'bg-gray-900 text-white border-gray-900 dark:bg-white dark:text-gray-900 dark:border-white'
                : 'bg-white text-gray-700 border-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-300 dark:border-white/10'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {error ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-6 flex flex-wrap items-center gap-3 text-red-600" role="alert">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span className="flex-1 min-w-0">{error}</span>
          <button
            type="button"
            onClick={fetchOrders}
            className="px-3 py-1.5 text-sm rounded-lg border border-red-300 bg-white text-red-700 hover:bg-red-50"
          >
            Повторить
          </button>
        </div>
      ) : isLoading ? (
        <SellerSurface className="py-20 flex flex-col justify-center items-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-gray-900 mb-4"></div>
          <div className="text-sm text-gray-500">Загружаем продажи...</div>
        </SellerSurface>
      ) : orders.length === 0 ? (
        <SellerSurface className="py-12 text-center text-gray-500">
          {filter === 'all' ? 'У вас пока нет продаж.' : 'В этом разделе заказов нет.'}
        </SellerSurface>
      ) : (
        <SellerTableShell>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-gray-50/60 dark:bg-white/[0.02] border-b border-gray-200 dark:border-white/10 text-xs font-semibold uppercase tracking-wider text-gray-500">
                  <th className="px-2 sm:px-4 py-3.5">Заказ</th>
                  <th className="px-2 sm:px-4 py-3.5">Статус</th>
                  <th className="px-2 sm:px-4 py-3.5 text-right hidden sm:table-cell">Товаров</th>
                  <th className="px-2 sm:px-4 py-3.5 text-right">Сумма (Ваша)</th>
                  <th className="px-1 sm:px-4 py-3.5 w-8 sm:w-10"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                {orders.map((order) => {
                  const stage = order.fulfillmentStage || order.deliveryStatus;
                  const statusConfig = STATUS_LABELS[order.deliveryStatus] || STATUS_LABELS[stage] || { label: order.deliveryStatus, color: 'bg-gray-100 text-gray-800' };
                  const deliveryLabel = SHIPMENT_STATUS_LABELS[order.deliveryStatus] || order.deliveryStatus;
                  const owner = order.nextStepOwner || 'none';
                  const isExpanded = expandedOrderId === order.id;
                  const shortOrderId = order.orderNumber || order.id.split('-')[0];

                  return (
                    <React.Fragment key={order.id}>
                      <tr
                        onClick={() => setExpandedOrderId(isExpanded ? null : order.id)}
                        className="cursor-pointer hover:bg-gray-50/60 dark:hover:bg-white/[0.02] transition-colors"
                        data-testid={`order-row-${shortOrderId}`}
                      >
                        <td className="px-2 sm:px-4 py-3.5 align-top">
                          <div className="font-medium text-gray-900 dark:text-white break-all min-w-[5.5rem]">#{shortOrderId}</div>
                          <div className="text-xs text-gray-500 mt-0.5">{new Date(order.createdAt).toLocaleDateString('ru-RU')}</div>
                        </td>
                        <td className="px-2 sm:px-4 py-3.5 align-top">
                          <div className="flex flex-col gap-1 items-start">
                            <span className={`px-2.5 py-0.5 text-xs font-medium rounded-full ${statusConfig.color}`}>
                              {statusConfig.label}
                            </span>
                            {stage !== 'cancelled' && stage !== 'awaiting_payment' && deliveryLabel !== statusConfig.label && (
                              <span className="text-[11px] text-gray-500 bg-gray-100 dark:bg-white/10 px-2 py-0.5 rounded-full">
                                🚚 {deliveryLabel}
                              </span>
                            )}
                            {order.hasReturns && (
                              <span className="text-[11px] font-medium text-orange-800 bg-orange-100 px-2 py-0.5 rounded-full" data-testid="order-has-returns">
                                Есть возврат
                              </span>
                            )}
                            <span className="text-[11px] text-gray-500">{NEXT_STEP_TEXT[owner]}</span>
                          </div>
                        </td>
                        <td className="px-2 sm:px-4 py-3.5 text-sm font-medium text-gray-900 dark:text-white text-right align-top whitespace-nowrap hidden sm:table-cell">
                          {order.sellerUnits} шт.
                        </td>
                        <td className="px-2 sm:px-4 py-3.5 font-semibold text-gray-900 dark:text-white text-right align-top whitespace-nowrap">
                          {currencyFormatter.format(order.sellerGrossAmount / 100)}
                          <div className="sm:hidden text-xs font-normal text-gray-500">{order.sellerUnits} шт.</div>
                        </td>
                        <td className="px-2 sm:px-4 py-3.5 text-right align-top">
                          <button
                            type="button"
                            aria-expanded={isExpanded}
                            aria-label={isExpanded ? 'Свернуть заказ' : 'Развернуть заказ'}
                            className="text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
                          >
                            {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                          </button>
                        </td>
                      </tr>

                      {isExpanded && (
                        <tr>
                          <td colSpan={5} className="p-0 border-b border-gray-100 dark:border-white/5">
                            <div className="p-4 sm:p-6 bg-gray-50/60 dark:bg-black/20" data-testid="order-detail">
                              {order.deliveryStatus === 'shipped' ? (
                                <div className="mb-4 p-3.5 rounded-xl bg-blue-50/70 dark:bg-blue-950/30 border border-blue-200/70 dark:border-blue-900/40 text-blue-900 dark:text-blue-200 text-xs flex items-center gap-2.5" data-testid="shipped-observer-notice">
                                  <Package className="w-4 h-4 text-blue-600 dark:text-blue-400 shrink-0" />
                                  <span>
                                    Заказ отгружен со склада ZAMK и передан в доставку. От продавца действий не требуется.
                                  </span>
                                </div>
                              ) : (
                                <div className="mb-4 p-3.5 rounded-xl bg-gray-100/80 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-gray-700 dark:text-gray-300 text-xs flex items-center gap-2.5" data-testid="fbo-notice">
                                  <Package className="w-4 h-4 shrink-0" />
                                  <span>
                                    Сборку, упаковку и отправку выполняет склад ZAMK. {NEXT_STEP_TEXT[owner]}.
                                  </span>
                                </div>
                              )}
                              <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2 mb-4">
                                <Package className="w-4 h-4 text-gray-400" />
                                Ваши товары в заказе #{shortOrderId}
                              </h3>
                              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {order.items?.map((item) => {
                                  const returnBadges = itemReturnBadges(item);
                                  const itemStatus = STATUS_LABELS[item.fulfillmentStage === 'processing' ? order.deliveryStatus : item.fulfillmentStage || order.deliveryStatus];
                                  return (
                                    <div key={item.id} className="flex gap-4 p-4 bg-white dark:bg-black/40 rounded-lg border border-gray-200 dark:border-white/10" data-testid="order-item">
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
                                          {item.sku && <span>SKU: {item.sku}</span>}
                                        </div>
                                        <div className="flex flex-wrap items-center gap-1.5 mt-2">
                                          {itemStatus && (
                                            <span className={`px-2 py-0.5 text-[11px] font-medium rounded-full ${itemStatus.color}`} data-testid="item-status">
                                              {itemStatus.label}
                                            </span>
                                          )}
                                          {returnBadges.map((b) => (
                                            <span key={b.key} className={`px-2 py-0.5 text-[11px] font-medium rounded-full ${b.color}`} data-testid={`item-return-${b.key}`}>
                                              {b.text}
                                            </span>
                                          ))}
                                        </div>
                                        <div className="flex items-center justify-between mt-2 gap-2">
                                          <span className="text-sm text-gray-500">{item.quantity} шт. × {currencyFormatter.format(item.priceCents / 100)}</span>
                                          <span className="font-medium text-gray-900 dark:text-white whitespace-nowrap">{currencyFormatter.format(item.subtotalPriceCents / 100)}</span>
                                        </div>
                                      </div>
                                    </div>
                                  );
                                })}
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

      {!error && totalCount > PAGE_SIZE && (
        <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500" data-testid="orders-pagination">
          <span>
            {offset + 1}–{Math.min(offset + PAGE_SIZE, totalCount)} из {totalCount}
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={offset === 0 || isLoading}
              onClick={() => { setExpandedOrderId(null); setOffset(Math.max(0, offset - PAGE_SIZE)); }}
              className="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-white/10 disabled:opacity-40"
            >
              Назад
            </button>
            <button
              type="button"
              disabled={offset + PAGE_SIZE >= totalCount || isLoading}
              onClick={() => { setExpandedOrderId(null); setOffset(offset + PAGE_SIZE); }}
              className="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-white/10 disabled:opacity-40"
            >
              Вперёд
            </button>
          </div>
        </div>
      )}
    </SellerPageFrame>
  );
}
