import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  ArrowLeft, ShoppingBag, Truck, CreditCard, Clock, User, MapPin, AlertTriangle, FileText
} from 'lucide-react';
import { getAdminOrder, getAdminOrderFulfillments } from '../api/adminOrders';
import type { AdminOrderView } from '../api/adminOrders';
import type { AdminFulfillment } from '@zamk/api-client/src/types';
import { formatMoney, formatOrderNumber, formatDateTime, orderStatusMap, fulfillmentStatusMap } from '../utils/orderFormatters';

export function AdminOrderDetail() {
  const { orderId } = useParams<{ orderId: string }>();
  const navigate = useNavigate();

  const [order, setOrder] = useState<AdminOrderView | null>(null);
  const [fulfillments, setFulfillments] = useState<AdminFulfillment[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'overview' | 'items_fulfillments' | 'payment' | 'delivery' | 'history'>('overview');
  const [internalComment, setInternalComment] = useState('');
  const [isSavedComment, setIsSavedComment] = useState(false);

  const fetchDetail = async () => {
    if (!orderId) return;
    try {
      setIsLoading(true);
      setError(null);
      const data = await getAdminOrder(orderId);
      setOrder(data);
      const fData = await getAdminOrderFulfillments(orderId);
      setFulfillments(fData);
    } catch (err: any) {
      setError(err.message || 'Не удалось загрузить досье заказа');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchDetail();
  }, [orderId]);

  if (isLoading) {
    return (
      <div className="text-center py-20 bg-white rounded-xl border border-gray-200 shadow-sm max-w-4xl mx-auto mt-6">
        <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-indigo-600 mx-auto" />
        <p className="mt-3 text-gray-500 text-sm font-medium">Загрузка досье заказа...</p>
      </div>
    );
  }

  if (error || !order) {
    return (
      <div className="max-w-4xl mx-auto p-6 space-y-4">
        <button
          onClick={() => navigate('/orders')}
          className="inline-flex items-center text-gray-500 hover:text-gray-900 text-sm font-medium transition-colors"
        >
          <ArrowLeft className="h-4 w-4 mr-1.5" /> Назад к заказам
        </button>
        <div className="p-6 bg-rose-50 border border-rose-200 text-rose-800 rounded-xl flex items-center gap-3 shadow-sm">
          <AlertTriangle className="h-6 w-6 shrink-0 text-rose-600" />
          <div>
            <h3 className="font-bold text-base text-gray-900">Ошибка загрузки заказа</h3>
            <p className="text-sm mt-1 text-gray-700">{error || 'Заказ не найден.'}</p>
          </div>
        </div>
      </div>
    );
  }

  const st = orderStatusMap[order.status] || { label: order.statusLabel || order.status, bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };

  const totalCents = order.totalPriceCents || (order.totalAmount * 100) || 0;
  const itemsSubtotalCents = order.items.reduce((sum, it) => sum + (it.subtotalPriceCents || (it.priceCents * it.quantity)), 0);
  const deliveryCents = totalCents > itemsSubtotalCents ? totalCents - itemsSubtotalCents : 0;

  return (
    <div data-testid="order-detail-page" className="max-w-7xl mx-auto space-y-6">
      {/* Top back & actions bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <button
          onClick={() => navigate('/orders')}
          className="inline-flex items-center text-gray-500 hover:text-gray-900 text-sm font-medium transition-colors"
        >
          <ArrowLeft className="h-4 w-4 mr-2" /> Все заказы
        </button>

        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => {
              setIsSavedComment(true);
              setTimeout(() => setIsSavedComment(false), 2000);
            }}
            className="px-3.5 py-1.5 bg-white hover:bg-gray-50 text-gray-700 text-xs font-semibold rounded-lg border border-gray-200 transition-colors shadow-sm"
          >
            {isSavedComment ? 'Комментарий сохранён ✓' : 'Добавить комментарий'}
          </button>
        </div>
      </div>

      {/* Header Banner */}
      <div className="bg-white border border-gray-200 rounded-xl p-6 shadow-sm space-y-4">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold text-gray-900">{formatOrderNumber(order)}</h1>
              <span className={`px-3 py-1 rounded-full text-xs font-semibold ${st.bg} ${st.text}`}>
                {st.label}
              </span>
            </div>
            <p className="text-xs text-gray-500 mt-1">
              Создан: {formatDateTime(order.createdAt)} • Оплата: {order.paymentStatusLabel || (order.paymentStatus === 'paid' ? 'Оплачен' : 'Ожидает оплаты')}
            </p>
          </div>

          <div className="text-left md:text-right">
            <div className="text-xs text-gray-500 font-medium">Итоговая сумма</div>
            <div className="text-2xl font-black text-indigo-600 mt-0.5">
              {formatMoney(totalCents)}
            </div>
          </div>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="border-b border-gray-200 bg-white rounded-xl px-4 shadow-sm">
        <nav className="-mb-px flex space-x-8 overflow-x-auto" aria-label="Order Tabs">
          {[
            { id: 'overview', testId: 'order-tab-overview', label: 'Обзор', icon: FileText },
            { id: 'items_fulfillments', testId: 'order-tab-items-fulfillments', label: 'Товары и сборки', icon: ShoppingBag, count: order.items.length },
            { id: 'payment', testId: 'order-tab-payment', label: 'Оплата', icon: CreditCard },
            { id: 'delivery', testId: 'order-tab-delivery', label: 'Доставка', icon: Truck },
            { id: 'history', testId: 'order-tab-history', label: 'История', icon: Clock },
          ].map((t) => {
            const Icon = t.icon;
            return (
              <button
                key={t.id}
                data-testid={t.testId}
                onClick={() => setActiveTab(t.id as any)}
                className={`py-4 px-1 border-b-2 font-medium text-sm inline-flex items-center gap-2 transition-colors whitespace-nowrap ${
                  activeTab === t.id
                    ? 'border-indigo-600 text-indigo-600 font-semibold'
                    : 'border-transparent text-gray-500 hover:text-gray-700'
                }`}
              >
                <Icon className="h-4 w-4" />
                <span>{t.label}</span>
                {t.count !== undefined && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-bold bg-gray-100 text-gray-700">
                    {t.count}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
      </div>

      {/* TAB CONTENT */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Customer & Address */}
          <div className="lg:col-span-2 space-y-6">
            <div className="bg-white border border-gray-200 rounded-xl p-5 space-y-4 shadow-sm">
              <h3 className="text-base font-bold text-gray-900 flex items-center gap-2 border-b border-gray-100 pb-3">
                <User className="h-4 w-4 text-indigo-600" />
                <span>Данные покупателя</span>
              </h3>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
                <div>
                  <dt className="text-xs text-gray-500 font-medium">ФИО / Имя</dt>
                  <dd className="text-gray-900 font-semibold mt-0.5">{order.customerName || 'Покупатель'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500 font-medium">Телефон</dt>
                  <dd className="text-gray-900 font-semibold mt-0.5">{order.customerPhone || '—'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500 font-medium">Email</dt>
                  <dd className="text-gray-900 font-semibold mt-0.5">{order.customerEmail || '—'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500 font-medium">Способ доставки</dt>
                  <dd className="text-gray-900 font-semibold mt-0.5">Доставка ZAMK</dd>
                </div>
              </dl>
            </div>

            <div className="bg-white border border-gray-200 rounded-xl p-5 space-y-4 shadow-sm">
              <h3 className="text-base font-bold text-gray-900 flex items-center gap-2 border-b border-gray-100 pb-3">
                <MapPin className="h-4 w-4 text-indigo-600" />
                <span>Адрес доставки</span>
              </h3>
              <p className="text-sm text-gray-700">{order.deliveryAddress || 'Адрес не указан'}</p>
            </div>

            {/* Internal comment */}
            <div className="bg-white border border-gray-200 rounded-xl p-5 space-y-3 shadow-sm">
              <h3 className="text-base font-bold text-gray-900 flex items-center gap-2 border-b border-gray-100 pb-3">
                <FileText className="h-4 w-4 text-indigo-600" />
                <span>Внутренний комментарий оператора</span>
              </h3>
              <textarea
                rows={3}
                value={internalComment}
                onChange={(e) => setInternalComment(e.target.value)}
                placeholder="Введите заметки для коллег по заказу..."
                className="w-full p-3 bg-gray-50 border border-gray-200 rounded-xl text-sm text-gray-900 placeholder-gray-400 focus:bg-white focus:outline-none focus:border-indigo-500"
              />
            </div>
          </div>

          {/* Financial summary sidebar */}
          <div className="space-y-6">
            <div className="bg-white border border-gray-200 rounded-xl p-5 space-y-4 shadow-sm">
              <h3 className="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">Сводка по оплате</h3>
              <div className="space-y-2.5 text-sm">
                <div className="flex justify-between text-gray-600">
                  <span>Товары ({order.items.length})</span>
                  <span className="text-gray-900 font-medium">{formatMoney(itemsSubtotalCents || totalCents)}</span>
                </div>
                {deliveryCents > 0 && (
                  <div className="flex justify-between text-gray-600">
                    <span>Доставка / Услуги</span>
                    <span className="text-gray-900 font-medium">{formatMoney(deliveryCents)}</span>
                  </div>
                )}
                <div className="border-t border-gray-100 pt-3 flex justify-between text-base font-bold text-gray-900">
                  <span>Итого к оплате</span>
                  <span className="text-indigo-600">{formatMoney(totalCents)}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'items_fulfillments' && (
        <div className="space-y-6">
          {/* Order Items Table */}
          <div className="bg-white border border-gray-200 rounded-xl p-5 space-y-4 shadow-sm">
            <h3 className="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">Товары заказа ({order.items.length})</h3>
            {order.items.length === 0 ? (
              <div className="p-8 text-center text-gray-500">В заказе нет товарных позиций.</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-gray-200 text-sm text-left">
                  <thead className="bg-gray-50 text-gray-600 text-xs font-semibold uppercase">
                    <tr>
                      <th className="px-4 py-3">Товар</th>
                      <th className="px-4 py-3">SKU</th>
                      <th className="px-4 py-3 text-center">Кол-во</th>
                      <th className="px-4 py-3 text-right">Цена</th>
                      <th className="px-4 py-3 text-right">Сумма</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100 text-gray-700">
                    {order.items.map((item) => (
                      <tr key={item.id}>
                        <td className="px-4 py-3">
                          <div className="font-semibold text-gray-900">{item.title}</div>
                          <div className="text-xs text-gray-500">
                            {item.variantSize && `Размер: ${item.variantSize}`} {item.variantColor && `• Цвет: ${item.variantColor}`}
                          </div>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-gray-500">{item.sku || '—'}</td>
                        <td className="px-4 py-3 text-center font-bold text-gray-900">×{item.quantity}</td>
                        <td className="px-4 py-3 text-right">{formatMoney(item.priceCents || 0)}</td>
                        <td className="px-4 py-3 text-right font-bold text-indigo-600">{formatMoney(item.subtotalPriceCents || item.priceCents * item.quantity || 0)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Fulfillments Section */}
          <div data-testid="order-fulfillments-section" className="space-y-4">
            <h3 className="text-base font-bold text-gray-900">Сборки заказа ({fulfillments.length})</h3>
            {fulfillments.length === 0 ? (
              <div data-testid="order-fulfillments-empty" className="p-8 text-center bg-white rounded-xl border border-gray-200 text-gray-500 shadow-sm">
                Сборка заказа формируется автоматически при подтверждении оплаты.
              </div>
            ) : (
              fulfillments.map((f) => {
                const fulSt = fulfillmentStatusMap[f.status] || { label: f.status, bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };

                return (
                  <div key={f.id} data-testid={`order-fulfillment-${f.id}`} className="bg-white border border-gray-200 rounded-xl p-5 space-y-4 shadow-sm">
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-gray-100 pb-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <h4 className="font-bold text-gray-900 text-base">{f.sellerName || 'Склад ZAMK'}</h4>
                          <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold ${fulSt.bg} ${fulSt.text}`}>
                            {fulSt.label}
                          </span>
                        </div>
                        <p className="text-xs text-gray-500 mt-0.5">
                          ID сборки: <span className="font-mono">{f.id}</span>
                        </p>
                      </div>
                    </div>

                    <div className="overflow-x-auto">
                      <table className="min-w-full divide-y divide-gray-100 text-xs text-left">
                        <thead className="bg-gray-50 text-gray-500 font-semibold">
                          <tr>
                            <th className="px-3 py-2">Товар</th>
                            <th className="px-3 py-2 text-center">Кол-во</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-100 text-gray-700">
                          {f.items?.map((it) => (
                            <tr key={it.orderItemId}>
                              <td className="px-3 py-2 font-medium">{it.productTitle}</td>
                              <td className="px-3 py-2 text-center font-bold">×{it.quantity}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}

      {activeTab === 'payment' && (
        <div className="bg-white border border-gray-200 rounded-xl p-6 space-y-4 shadow-sm">
          <h3 className="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">Детали транзакции</h3>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-xs text-gray-500 font-medium">Статус оплаты</dt>
              <dd className="text-gray-900 font-semibold mt-0.5">
                {order.paymentStatus === 'paid' ? 'Успешно оплачен' : order.paymentStatus === 'failed' ? 'Ошибка оплаты' : order.paymentStatus === 'cancelled' ? 'Отменена' : 'Ожидает оплаты'}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 font-medium">Сумма заказа</dt>
              <dd className="text-indigo-600 font-bold mt-0.5">{formatMoney(totalCents)}</dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 font-medium">Метод оплаты</dt>
              <dd className="text-gray-900 font-semibold mt-0.5">Банковская карта онлайн (T-Bank)</dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 font-medium">Дата создания</dt>
              <dd className="text-gray-900 font-semibold mt-0.5">{formatDateTime(order.createdAt)}</dd>
            </div>
          </dl>
        </div>
      )}

      {activeTab === 'delivery' && (
        <div className="bg-white border border-gray-200 rounded-xl p-6 space-y-4 shadow-sm">
          <h3 className="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">Информация о доставке</h3>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
            <div>
              <dt className="text-xs text-gray-500 font-medium">Способ доставки</dt>
              <dd className="text-gray-900 font-semibold mt-0.5">{order.deliveryMethodName || 'Курьерская доставка'}</dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 font-medium">Стоимость доставки</dt>
              <dd className="text-gray-900 font-semibold mt-0.5">{order.deliveryPriceCents !== undefined && order.deliveryPriceCents !== null ? formatMoney(order.deliveryPriceCents) : '—'}</dd>
            </div>
            <div>
              <dt className="text-xs text-gray-500 font-medium">Адрес получателя</dt>
              <dd className="text-gray-900 font-semibold mt-0.5">{order.deliveryAddress || 'Адрес не указан'}</dd>
            </div>
            {order.customerPhone && (
              <div>
                <dt className="text-xs text-gray-500 font-medium">Телефон получателя</dt>
                <dd className="text-gray-900 font-semibold mt-0.5">{order.customerPhone}</dd>
              </div>
            )}
          </dl>
        </div>
      )}

      {activeTab === 'history' && (
        <div className="bg-white border border-gray-200 rounded-xl p-6 space-y-4 shadow-sm">
          <h3 className="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">История заказа</h3>
          {order.timeline && order.timeline.length > 0 ? (
            <ul className="space-y-4 text-sm relative before:absolute before:inset-0 before:left-3 before:w-0.5 before:bg-gray-100">
              {order.timeline.map((event, idx) => {
                let dotBg = 'bg-indigo-600';
                if (event.type.includes('payment_succeeded') || event.title.includes('Оплата')) {
                  dotBg = 'bg-emerald-600';
                } else if (event.type.includes('failed') || event.type.includes('cancelled') || event.title.includes('отмен')) {
                  dotBg = 'bg-rose-600';
                } else if (event.title.includes('сборк') || event.title.includes('Упаковк')) {
                  dotBg = 'bg-blue-600';
                } else if (event.title.includes('Отгружен') || event.title.includes('Доставлен')) {
                  dotBg = 'bg-indigo-600';
                }

                return (
                  <li key={event.id || idx} className="flex items-start gap-4 relative">
                    <div className={`w-2.5 h-2.5 rounded-full ${dotBg} mt-1.5 ring-4 ring-white shrink-0 z-10`} />
                    <div className="flex-1 min-w-0">
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="font-semibold text-gray-900">{event.title}</span>
                        {event.context && (
                          <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-600 font-medium">{event.context}</span>
                        )}
                      </div>
                      <div className="text-xs text-gray-500 mt-0.5">{formatDateTime(event.timestamp)}</div>
                      {event.comment && (
                        <div className="text-xs text-gray-600 mt-1 italic">{event.comment}</div>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          ) : (
            <ul className="space-y-3 text-sm">
              <li className="flex items-start gap-3">
                <div className="w-2 h-2 rounded-full bg-indigo-600 mt-1.5" />
                <div>
                  <div className="font-semibold text-gray-900">Заказ создан</div>
                  <div className="text-xs text-gray-500">{formatDateTime(order.createdAt)}</div>
                </div>
              </li>
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
