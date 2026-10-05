import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { X, ExternalLink, Package, Loader2, AlertCircle, ArrowLeft } from 'lucide-react';
import { getAdminOrder, type AdminOrderView } from '../../api/adminOrders';

interface OrderQuickViewProps {
  orderId: string;
  onClose: () => void;
  onBack?: () => void;
}

export function OrderQuickView({ orderId, onClose, onBack }: OrderQuickViewProps) {
  const [order, setOrder] = useState<AdminOrderView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    setError(null);
    getAdminOrder(orderId)
      .then((data) => {
        if (mounted) {
          setOrder(data);
          setLoading(false);
        }
      })
      .catch((e: unknown) => {
        if (mounted) {
          const message = e instanceof Error ? e.message : 'Ошибка загрузки заказа';
          setError(message);
          setLoading(false);
        }
      });
    return () => {
      mounted = false;
    };
  }, [orderId]);

  return (
    <div
      data-testid="order-quick-view"
      className="absolute top-0 right-0 bottom-0 w-full sm:w-96 md:w-[410px] bg-white shadow-2xl z-30 flex flex-col border-l border-gray-200 transform transition-transform duration-300 translate-x-0"
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100 bg-gray-50/80 shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          {onBack && (
            <button
              type="button"
              onClick={onBack}
              title="Назад"
              aria-label="Назад"
              className="p-1.5 -ml-1 text-gray-400 hover:text-gray-600 hover:bg-gray-200/50 rounded-md transition-colors mr-1 cursor-pointer"
            >
              <ArrowLeft className="w-4 h-4" />
            </button>
          )}
          <Package className="w-4 h-4 text-gray-500 shrink-0" />
          <h3 className="font-semibold text-gray-900 text-sm truncate">
            Заказ {order?.orderNumber || orderId.slice(0, 8)}
          </h3>
        </div>
        <button
          type="button"
          onClick={onClose}
          title="Закрыть Quick View"
          aria-label="Закрыть"
          className="p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-200/50 rounded-md transition-colors cursor-pointer shrink-0"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 select-none">
        {loading ? (
          <div className="flex flex-col items-center justify-center h-48 text-gray-400 gap-3">
            <Loader2 className="w-6 h-6 animate-spin text-gray-300" />
            <span className="text-xs font-medium">Загрузка данных заказа...</span>
          </div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center h-48 text-red-500 gap-3 text-center">
            <AlertCircle className="w-6 h-6" />
            <span className="text-xs font-medium px-4">{error}</span>
          </div>
        ) : order ? (
          <div className="space-y-6">
            {/* Status & Totals */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-500">Статус</span>
                <span className="text-xs font-semibold px-2 py-0.5 bg-gray-100 text-gray-800 rounded">
                  {order.statusLabel}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-500">Оплата</span>
                <span className="text-xs font-semibold px-2 py-0.5 bg-blue-50 text-blue-700 rounded">
                  {order.paymentStatusLabel}
                </span>
              </div>
              <div className="flex items-center justify-between pt-2 border-t border-gray-100">
                <span className="text-xs font-medium text-gray-700">Итого</span>
                <span className="text-sm font-bold text-gray-900">
                  {order.totalAmount} ₽
                </span>
              </div>
            </div>

            {/* Customer Details */}
            <div>
              <h4 className="text-xs font-bold text-gray-900 uppercase tracking-wider mb-2.5">
                Покупатель
              </h4>
              <div className="bg-gray-50 p-3 rounded-lg border border-gray-100 space-y-2">
                {order.customerName && (
                  <p className="text-xs font-medium text-gray-900">{order.customerName}</p>
                )}
                {order.customerEmail && (
                  <p className="text-xs text-gray-600 truncate">{order.customerEmail}</p>
                )}
                {order.customerPhone && (
                  <p className="text-xs text-gray-600">{order.customerPhone}</p>
                )}
              </div>
            </div>

            {/* Delivery Details */}
            {order.deliveryMethodName && (
              <div>
                <h4 className="text-xs font-bold text-gray-900 uppercase tracking-wider mb-2.5">
                  Доставка
                </h4>
                <div className="bg-gray-50 p-3 rounded-lg border border-gray-100 space-y-1.5">
                  <p className="text-xs font-medium text-gray-800">{order.deliveryMethodName}</p>
                  {order.deliveryAddress && (
                    <p className="text-xs text-gray-600 leading-relaxed">{order.deliveryAddress}</p>
                  )}
                </div>
              </div>
            )}

            {/* Items */}
            <div>
              <h4 className="text-xs font-bold text-gray-900 uppercase tracking-wider mb-2.5">
                Товары ({order.unitsCount})
              </h4>
              <div className="space-y-2">
                {order.items.map((item, idx) => (
                  <div key={idx} className="flex gap-3 items-start p-2.5 border border-gray-100 rounded-lg bg-white">
                    {item.imageUrl ? (
                      <img src={item.imageUrl} alt={item.title} className="w-10 h-10 object-cover rounded bg-gray-50 flex-shrink-0" />
                    ) : (
                      <div className="w-10 h-10 rounded bg-gray-100 flex items-center justify-center flex-shrink-0">
                        <Package className="w-5 h-5 text-gray-400" />
                      </div>
                    )}
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-medium text-gray-900 line-clamp-2 leading-snug">{item.title}</p>
                      <div className="flex items-center gap-2 mt-1">
                        <span className="text-[11px] text-gray-500">{item.quantity} шт.</span>
                        <span className="text-[11px] font-semibold text-gray-700">{(item.priceCents / 100).toFixed(0)} ₽</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : null}
      </div>

      {/* Footer / Actions */}
      <div className="p-4 border-t border-gray-200 bg-white shrink-0">
        <Link
          to={`/orders/${orderId}`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center justify-center gap-2 w-full py-2 px-4 bg-gray-900 hover:bg-gray-800 text-white rounded-lg text-xs font-semibold transition-colors"
        >
          <span>Открыть полную карточку заказа</span>
          <ExternalLink className="w-3.5 h-3.5" />
        </Link>
      </div>
    </div>
  );
}
