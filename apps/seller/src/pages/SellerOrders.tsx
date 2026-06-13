import { useEffect, useState, useMemo, useCallback } from 'react';
import { getSellerFulfillments, markSellerFulfillmentAssembling, markSellerFulfillmentPacked } from '@zamk/api-client/src/seller';
import type { SellerFulfillment } from '@zamk/api-client/src/types';
import { ChevronDown, ChevronUp, Package, MapPin, User, Phone, AlertCircle, CheckCircle2 } from 'lucide-react';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  awaiting_payment: { label: 'Ожидает оплаты', color: 'bg-yellow-100 text-yellow-800' },
  paid: { label: 'Оплачен', color: 'bg-emerald-100 text-emerald-800' },
  assembling: { label: 'Собирается', color: 'bg-blue-100 text-blue-800' },
  packed: { label: 'Упакован', color: 'bg-indigo-100 text-indigo-800' },
  shipped: { label: 'Отправлен', color: 'bg-purple-100 text-purple-800' },
  delivered: { label: 'Доставлен', color: 'bg-green-100 text-green-800' },
  cancelled: { label: 'Отменён', color: 'bg-red-100 text-red-800' },
  returned: { label: 'Возврат', color: 'bg-rose-100 text-rose-800' },
  refunded: { label: 'Возмещён', color: 'bg-gray-100 text-gray-800' },
};

const SHIPMENT_STATUS_LABELS: Record<string, string> = {
  pending: 'Ожидает подготовки',
  assembling: 'Собирается',
  packed: 'Упакован',
  shipped: 'Отправлен',
  delivered: 'Доставлен',
  cancelled: 'Отменён',
};

type FilterType = 'all' | 'paid' | 'assembling' | 'packed' | 'shipped' | 'delivered' | 'cancelled';

export function SellerOrders() {
  const [fulfillments, setFulfillments] = useState<SellerFulfillment[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<FilterType>('all');
  const [expandedFulfillmentId, setExpandedFulfillmentId] = useState<string | null>(null);
  const [actionError, setActionError] = useState('');
  const [actionSuccess, setActionSuccess] = useState('');
  const [isActionLoading, setIsActionLoading] = useState(false);

  const fetchFulfillments = useCallback(async () => {
    try {
      const data = await getSellerFulfillments();
      setFulfillments(data.items || []);
    } catch (err: any) {
      if (err.status === 403) {
        setError('Недостаточно прав для просмотра заказов.');
      } else {
        setError('Не удалось загрузить заказы продавца.');
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchFulfillments();
  }, [fetchFulfillments]);

  const handleMarkAssembling = async (id: string) => {
    setActionError('');
    setActionSuccess('');
    setIsActionLoading(true);
    try {
      await markSellerFulfillmentAssembling(id);
      setActionSuccess('Сборка успешно начата.');
      await fetchFulfillments();
    } catch (err: any) {
      setActionError(err.body?.error?.message || 'Не удалось обновить статус.');
    } finally {
      setIsActionLoading(false);
    }
  };

  const handleMarkPacked = async (id: string) => {
    setActionError('');
    setActionSuccess('');
    setIsActionLoading(true);
    try {
      await markSellerFulfillmentPacked(id);
      setActionSuccess('Сборка отмечена как упакованная.');
      await fetchFulfillments();
    } catch (err: any) {
      setActionError(err.body?.error?.message || 'Не удалось обновить статус.');
    } finally {
      setIsActionLoading(false);
    }
  };

  const filteredFulfillments = useMemo(() => {
    return fulfillments.filter(f => {
      if (filter === 'all') return true;
      if (filter === 'paid') return f.status === 'paid' || f.status === 'awaiting_payment';
      if (filter === 'assembling') return f.status === 'assembling';
      if (filter === 'packed') return f.status === 'packed';
      if (filter === 'shipped') return f.status === 'shipped';
      if (filter === 'delivered') return f.status === 'delivered';
      if (filter === 'cancelled') return ['cancelled', 'returned', 'refunded'].includes(f.status);
      return true;
    });
  }, [fulfillments, filter]);

  if (isLoading) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center flex-col items-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black mb-4"></div>
        <div className="text-ash">Загружаем заказы...</div>
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
      <h1 className="text-2xl font-bold text-graphite dark:text-white mb-2">Заказы (Сборки)</h1>
      <p className="text-ash mb-8">
        Список ваших товаров, сгруппированных по заказам покупателей.
      </p>
      
      {/* Filters */}
      <div className="flex flex-wrap gap-2 mb-6">
        {[
          { id: 'all', label: 'Все' },
          { id: 'paid', label: 'Оплаченные' },
          { id: 'assembling', label: 'Собираются' },
          { id: 'packed', label: 'Упакованы' },
          { id: 'shipped', label: 'Отправлены' },
          { id: 'delivered', label: 'Доставлены' },
          { id: 'cancelled', label: 'Отменённые / Возвраты' },
        ].map((f) => (
          <button
            key={f.id}
            onClick={() => setFilter(f.id as FilterType)}
            className={`px-4 py-2 rounded-[18px] text-sm font-medium transition-colors ${
              filter === f.id
                ? 'bg-graphite text-white dark:bg-white dark:text-black'
                : 'bg-white text-graphite border border-border-soft hover:bg-gray-50 dark:bg-white/5 dark:text-white dark:border-white/10 dark:hover:bg-white/10'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {fulfillments.length === 0 ? (
        <div className="py-12 text-center text-ash bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10">
          У вас пока нет заказов.
        </div>
      ) : filteredFulfillments.length === 0 ? (
        <div className="py-12 text-center text-ash bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10">
          В данной категории нет заказов.
        </div>
      ) : (
        <div className="space-y-4">
          {filteredFulfillments.map((fulfillment) => {
            const statusConfig = STATUS_LABELS[fulfillment.status] || { label: fulfillment.status, color: 'bg-gray-100 text-gray-800' };
            const isExpanded = expandedFulfillmentId === fulfillment.id;
            const shortOrderId = fulfillment.orderId.split('-')[0];
            const shortFulfillmentId = fulfillment.id.split('-')[0];
            const itemCount = fulfillment.items?.length || 0;
            
            return (
              <div key={fulfillment.id} className="bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10 overflow-hidden">
                <div 
                  className="p-5 flex flex-col sm:flex-row sm:items-center justify-between cursor-pointer hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
                  onClick={() => setExpandedFulfillmentId(isExpanded ? null : fulfillment.id)}
                >
                  <div className="flex flex-col gap-2">
                    <div className="flex flex-wrap items-center gap-3">
                      <span className="font-bold text-graphite dark:text-white">Сборка #{shortFulfillmentId}</span>
                      <span className="text-sm text-ash bg-gray-100 dark:bg-white/10 px-2 py-0.5 rounded-full">Заказ #{shortOrderId}</span>
                      <span className={`px-2 py-0.5 text-xs font-semibold rounded-full ${statusConfig.color}`}>
                        {statusConfig.label}
                      </span>
                      {fulfillment.shipmentStatus && (
                        <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-blue-50 text-blue-700 flex items-center gap-1">
                          <Package className="w-3 h-3" />
                          {SHIPMENT_STATUS_LABELS[fulfillment.shipmentStatus] || fulfillment.shipmentStatus}
                        </span>
                      )}
                    </div>
                    <span className="text-sm text-ash">{new Date(fulfillment.createdAt).toLocaleString('ru-RU')}</span>
                    <div className="text-sm text-ash mt-1">
                      {fulfillment.customerName || 'Покупатель'}, {itemCount} {itemCount === 1 ? 'товар' : itemCount < 5 ? 'товара' : 'товаров'}
                    </div>
                  </div>
                  <div className="flex items-center gap-6 mt-4 sm:mt-0">
                    <div className="text-right">
                      <p className="text-sm text-ash mb-1">Сумма сборки</p>
                      <p className="font-semibold text-graphite dark:text-white">{currencyFormatter.format(fulfillment.subtotalCents / 100)}</p>
                    </div>
                    <button className="p-2 text-ash hover:text-graphite dark:hover:text-white transition-colors">
                      {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                    </button>
                  </div>
                </div>

                {isExpanded && (
                  <div className="p-5 border-t border-border-soft dark:border-white/10 bg-gray-50/50 dark:bg-black/20">
                    {actionError && expandedFulfillmentId === fulfillment.id && (
                      <div className="mb-6 bg-red-50 text-red-600 p-4 rounded-xl flex gap-3 text-sm">
                        <AlertCircle className="w-5 h-5 shrink-0" />
                        <p>{actionError}</p>
                      </div>
                    )}
                    {actionSuccess && expandedFulfillmentId === fulfillment.id && (
                      <div className="mb-6 bg-green-50 text-green-700 p-4 rounded-xl flex gap-3 text-sm">
                        <CheckCircle2 className="w-5 h-5 shrink-0" />
                        <p>{actionSuccess}</p>
                      </div>
                    )}

                    <div className="mb-6">
                      {fulfillment.status === 'paid' && (
                        <button
                          onClick={() => handleMarkAssembling(fulfillment.id)}
                          disabled={isActionLoading}
                          className="bg-black text-white dark:bg-white dark:text-black px-4 py-2 rounded-xl font-medium disabled:opacity-50"
                        >
                          {isActionLoading ? 'Обработка...' : 'Начать сборку'}
                        </button>
                      )}
                      {fulfillment.status === 'assembling' && (
                        <button
                          onClick={() => handleMarkPacked(fulfillment.id)}
                          disabled={isActionLoading}
                          className="bg-indigo-600 text-white px-4 py-2 rounded-xl font-medium hover:bg-indigo-700 transition disabled:opacity-50"
                        >
                          {isActionLoading ? 'Обработка...' : 'Отметить как упаковано'}
                        </button>
                      )}
                      {fulfillment.status === 'packed' && (
                        <div className="bg-gray-50 text-gray-700 p-4 rounded-xl border border-gray-200 text-sm">
                          Сборка упакована. Отгрузку создаст администратор.
                        </div>
                      )}
                      {fulfillment.status === 'shipped' && (
                        <div className="bg-purple-50 text-purple-800 p-4 rounded-xl border border-purple-100 text-sm">
                          Сборка отправлена покупателю.
                        </div>
                      )}
                      {fulfillment.status === 'delivered' && (
                        <div className="bg-green-50 text-green-800 p-4 rounded-xl border border-green-100 text-sm">
                          Сборка успешно доставлена.
                        </div>
                      )}
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                      {/* Customer Info */}
                      <div className="space-y-4">
                        <h3 className="font-semibold text-graphite dark:text-white flex items-center gap-2">
                          <User className="w-4 h-4" />
                          Данные покупателя
                        </h3>
                        <div className="space-y-3 bg-white dark:bg-black/40 p-4 rounded-xl border border-border-soft dark:border-white/10">
                          <div className="flex items-start gap-3">
                            <User className="w-4 h-4 text-ash mt-0.5 shrink-0" />
                            <div>
                              <p className="text-xs text-ash">Имя</p>
                              <p className="text-sm font-medium text-graphite dark:text-white">
                                {fulfillment.customerName || 'Не указано'}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-start gap-3">
                            <Phone className="w-4 h-4 text-ash mt-0.5 shrink-0" />
                            <div>
                              <p className="text-xs text-ash">Телефон</p>
                              <p className="text-sm font-medium text-graphite dark:text-white">
                                {fulfillment.customerPhone || 'Не указан'}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-start gap-3">
                            <MapPin className="w-4 h-4 text-ash mt-0.5 shrink-0" />
                            <div>
                              <p className="text-xs text-ash">Адрес доставки</p>
                              <p className="text-sm font-medium text-graphite dark:text-white leading-snug">
                                {fulfillment.deliveryAddress || 'Не указан'}
                              </p>
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* Items */}
                      <div className="space-y-4">
                        <h3 className="font-semibold text-graphite dark:text-white flex items-center gap-2">
                          <Package className="w-4 h-4" />
                          Товары ({fulfillment.items?.length || 0})
                        </h3>
                        <div className="space-y-3">
                          {fulfillment.items?.map((item) => (
                            <div key={item.orderItemId} className="flex gap-4 p-4 bg-white dark:bg-black/40 rounded-xl border border-border-soft dark:border-white/10">
                              <div className="w-16 h-16 rounded-lg bg-gray-100 overflow-hidden shrink-0">
                                {item.imageUrl ? (
                                  <img src={item.imageUrl} alt={item.productTitle} className="w-full h-full object-cover" />
                                ) : (
                                  <div className="w-full h-full flex items-center justify-center text-gray-400">
                                    <Package className="w-8 h-8 opacity-20" />
                                  </div>
                                )}
                              </div>
                              <div className="flex-1 min-w-0">
                                <h4 className="font-medium text-graphite dark:text-white truncate" title={item.productTitle}>
                                  {item.productTitle}
                                </h4>
                                {(item.sku || item.variantId) && (
                                  <p className="text-xs text-ash mt-0.5">
                                    SKU: {item.sku || item.variantId}
                                  </p>
                                )}
                                <div className="flex items-center justify-between mt-2">
                                  <span className="text-sm text-ash">{item.quantity} шт. × {currencyFormatter.format(item.unitPriceCents / 100)}</span>
                                  <span className="font-medium text-graphite dark:text-white">{currencyFormatter.format(item.lineTotalCents / 100)}</span>
                                </div>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
