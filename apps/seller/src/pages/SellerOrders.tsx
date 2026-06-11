import { useEffect, useState, useMemo } from 'react';
import { getSellerOrders } from '@zamk/api-client/src/seller';
import { ChevronDown, ChevronUp, Package, MapPin, User, Phone } from 'lucide-react';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  awaiting_payment: { label: 'Новый / Ожидает оплаты', color: 'bg-yellow-100 text-yellow-800' },
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

type FilterType = 'all' | 'new' | 'assembling' | 'shipped' | 'delivered' | 'cancelled';

export function SellerOrders() {
  const [orders, setOrders] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<FilterType>('all');
  const [expandedOrderId, setExpandedOrderId] = useState<string | null>(null);

  useEffect(() => {
    async function fetchOrders() {
      try {
        const data = await getSellerOrders();
        // The API returns { items: [...], totalCount: ... }
        setOrders(data.items || []);
      } catch (err: any) {
        setError(err.message || 'Не удалось загрузить заказы.');
      } finally {
        setIsLoading(false);
      }
    }
    fetchOrders();
  }, []);

  const filteredOrders = useMemo(() => {
    return orders.filter(order => {
      if (filter === 'all') return true;
      if (filter === 'new') return order.status === 'awaiting_payment' || order.status === 'paid';
      if (filter === 'assembling') return order.status === 'assembling' || order.status === 'packed';
      if (filter === 'shipped') return order.status === 'shipped';
      if (filter === 'delivered') return order.status === 'delivered';
      if (filter === 'cancelled') return order.status === 'cancelled' || order.status === 'returned' || order.status === 'refunded';
      return true;
    });
  }, [orders, filter]);

  if (isLoading) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center"><div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black"></div></div>;
  }

  if (error) {
    return <div className="min-h-screen pt-24 pb-24 flex justify-center text-red-500">{error}</div>;
  }

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <h1 className="text-2xl font-bold text-graphite dark:text-white mb-2">Заказы</h1>
      <p className="text-ash mb-8">Список заказов с вашими товарами (только для чтения). Все действия со статусами временно контролируются администратором.</p>
      
      {/* Filters */}
      <div className="flex flex-wrap gap-2 mb-6">
        {[
          { id: 'all', label: 'Все' },
          { id: 'new', label: 'Новые / Оплаченные' },
          { id: 'assembling', label: 'Собираются' },
          { id: 'shipped', label: 'Отправлены' },
          { id: 'delivered', label: 'Доставлены' },
          { id: 'cancelled', label: 'Отменённые' },
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

      {orders.length === 0 ? (
        <div className="py-12 text-center text-ash bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10">
          У вас пока нет заказов.
        </div>
      ) : filteredOrders.length === 0 ? (
        <div className="py-12 text-center text-ash bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10">
          В данной категории нет заказов.
        </div>
      ) : (
        <div className="space-y-4">
          {filteredOrders.map((order) => {
            const statusConfig = STATUS_LABELS[order.status] || { label: order.status, color: 'bg-gray-100 text-gray-800' };
            const isExpanded = expandedOrderId === order.id;
            
            // Calculate total for this seller's items
            const sellerTotal = (order.items || []).reduce((acc: number, item: any) => acc + (item.priceCents * item.quantity), 0);

            return (
              <div key={order.id} className="bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10 overflow-hidden">
                <div 
                  className="p-5 flex flex-col sm:flex-row sm:items-center justify-between cursor-pointer hover:bg-gray-50 dark:hover:bg-white/5 transition-colors"
                  onClick={() => setExpandedOrderId(isExpanded ? null : order.id)}
                >
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center gap-3">
                      <span className="font-bold text-graphite dark:text-white">Заказ #{order.id.split('-')[0]}</span>
                      <span className={`px-2 py-0.5 text-xs font-semibold rounded-full ${statusConfig.color}`}>
                        {statusConfig.label}
                      </span>
                    </div>
                    <span className="text-sm text-ash">{new Date(order.createdAt).toLocaleString('ru-RU')}</span>
                  </div>
                  <div className="flex items-center gap-6 mt-4 sm:mt-0">
                    <div className="text-right">
                      <p className="text-sm text-ash mb-1">Сумма ваших товаров</p>
                      <p className="font-semibold text-graphite dark:text-white">{currencyFormatter.format(sellerTotal / 100)}</p>
                    </div>
                    <button className="p-2 text-ash hover:text-graphite dark:hover:text-white transition-colors">
                      {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
                    </button>
                  </div>
                </div>

                {isExpanded && (
                  <div className="p-5 border-t border-border-soft dark:border-white/10 bg-gray-50/50 dark:bg-black/20">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                      {/* Buyer Info */}
                      <div>
                        <h3 className="text-sm font-semibold text-graphite dark:text-white uppercase tracking-wider mb-4 flex items-center gap-2">
                          <MapPin className="w-4 h-4" /> Доставка
                        </h3>
                        <div className="bg-white dark:bg-white/5 rounded-xl p-4 border border-border-soft dark:border-white/10 space-y-3 text-sm text-graphite dark:text-white/80">
                          <div className="flex items-start gap-2">
                            <User className="w-4 h-4 text-ash mt-0.5 shrink-0" />
                            <span>{order.customerName || '—'}</span>
                          </div>
                          <div className="flex items-start gap-2">
                            <Phone className="w-4 h-4 text-ash mt-0.5 shrink-0" />
                            <span>{order.customerPhone || '—'}</span>
                          </div>
                          <div className="flex items-start gap-2">
                            <MapPin className="w-4 h-4 text-ash mt-0.5 shrink-0" />
                            <span>{order.deliveryAddress || '—'}</span>
                          </div>
                          {order.shipmentStatus && (
                            <div className="flex items-start gap-2 pt-3 border-t border-border-soft dark:border-white/10 mt-3">
                              <Package className="w-4 h-4 text-blue-500 mt-0.5 shrink-0" />
                              <span className="text-blue-600 dark:text-blue-400 font-medium">Статус отгрузки: {SHIPMENT_STATUS_LABELS[order.shipmentStatus] || order.shipmentStatus}</span>
                            </div>
                          )}
                        </div>
                      </div>

                      {/* Items */}
                      <div>
                        <h3 className="text-sm font-semibold text-graphite dark:text-white uppercase tracking-wider mb-4 flex items-center gap-2">
                          <Package className="w-4 h-4" /> Ваши товары в заказе
                        </h3>
                        <div className="space-y-3">
                          {order.items?.map((item: any) => (
                            <div key={item.id} className="flex gap-4 bg-white dark:bg-white/5 p-3 rounded-xl border border-border-soft dark:border-white/10">
                              <div className="w-16 h-16 rounded-lg bg-gray-100 dark:bg-white/10 overflow-hidden shrink-0">
                                {item.imageUrl ? (
                                  <img src={item.imageUrl} alt={item.title} className="w-full h-full object-cover" />
                                ) : (
                                  <div className="w-full h-full flex items-center justify-center text-gray-400"><Package className="w-6 h-6" /></div>
                                )}
                              </div>
                              <div className="flex-1 min-w-0">
                                <h4 className="text-sm font-medium text-graphite dark:text-white truncate">{item.title}</h4>
                                {(item.variantSize || item.variantColor) && (
                                  <p className="text-xs text-ash mt-1">
                                    {[item.variantSize, item.variantColor].filter(Boolean).join(' / ')}
                                  </p>
                                )}
                                {item.sku && <p className="text-xs text-ash mt-1">Артикул: {item.sku}</p>}
                                <div className="mt-2 flex items-center justify-between">
                                  <span className="text-sm font-semibold text-graphite dark:text-white">
                                    {currencyFormatter.format(item.priceCents / 100)} × {item.quantity}
                                  </span>
                                  <span className="text-sm font-bold text-graphite dark:text-white">
                                    {currencyFormatter.format((item.priceCents * item.quantity) / 100)}
                                  </span>
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
