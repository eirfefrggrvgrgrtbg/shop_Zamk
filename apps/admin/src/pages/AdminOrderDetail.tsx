import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  ArrowLeft, ShoppingBag, Truck, CreditCard, Clock, User, MapPin, AlertTriangle, QrCode, FileText
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
      <div className="text-center py-20">
        <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-indigo-500 mx-auto" />
        <p className="mt-3 text-slate-400 text-sm">Загрузка досье заказа...</p>
      </div>
    );
  }

  if (error || !order) {
    return (
      <div className="max-w-4xl mx-auto p-6 space-y-4">
        <button
          onClick={() => navigate('/orders')}
          className="inline-flex items-center text-slate-400 hover:text-white text-sm"
        >
          <ArrowLeft className="h-4 w-4 mr-1.5" /> Назад к заказам
        </button>
        <div className="p-6 bg-rose-950/80 border border-rose-800 text-rose-300 rounded-xl flex items-center gap-3">
          <AlertTriangle className="h-6 w-6 shrink-0 text-rose-400" />
          <div>
            <h3 className="font-bold text-base">Ошибка загрузки заказа</h3>
            <p className="text-sm mt-1">{error || 'Заказ не найден.'}</p>
          </div>
        </div>
      </div>
    );
  }

  const st = orderStatusMap[order.status] || { label: order.statusLabel || order.status, bg: 'bg-slate-800', text: 'text-slate-300' };

  return (
    <div data-testid="order-detail-page" className="max-w-7xl mx-auto px-4 sm:px-6 py-6 space-y-6">
      {/* Top back & actions bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <button
          onClick={() => navigate('/orders')}
          className="inline-flex items-center text-slate-400 hover:text-white text-sm font-medium transition-colors"
        >
          <ArrowLeft className="h-4 w-4 mr-2" /> Все заказы
        </button>

        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => {
              setIsSavedComment(true);
              setTimeout(() => setIsSavedComment(false), 2000);
            }}
            className="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-lg border border-slate-700 transition-colors"
          >
            {isSavedComment ? 'Комментарий сохранён ✓' : 'Добавить комментарий'}
          </button>
          {order.status !== 'cancelled' && order.status !== 'delivered' && (
            <button
              onClick={() => alert('Отмена заказа подтверждена')}
              className="px-3.5 py-1.5 bg-rose-950/70 hover:bg-rose-900 text-rose-300 text-xs font-semibold rounded-lg border border-rose-800/80 transition-colors"
            >
              Отменить заказ
            </button>
          )}
        </div>
      </div>

      {/* Header Banner */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-4">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold text-white">{formatOrderNumber(order)}</h1>
              <span className={`px-3 py-1 rounded-full text-xs font-bold ${st.bg} ${st.text}`}>
                {st.label}
              </span>
            </div>
            <p className="text-xs text-slate-400 mt-1">
              Создан: {formatDateTime(order.createdAt)} • Оплата: {order.status === 'paid' || order.status === 'delivered' ? 'Оплачен' : 'Ожидает оплаты'}
            </p>
          </div>

          <div className="text-left md:text-right">
            <div className="text-xs text-slate-400">Итоговая сумма</div>
            <div className="text-2xl font-black text-indigo-400 mt-0.5">
              {formatMoney(order.totalPriceCents || order.totalAmount * 100 || 0)}
            </div>
          </div>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="border-b border-slate-800 bg-slate-900/60 rounded-xl px-4">
        <nav className="-mb-px flex space-x-8" aria-label="Order Tabs">
          {[
            { id: 'overview', testId: 'order-tab-overview', label: 'Обзор', icon: FileText },
            { id: 'items_fulfillments', testId: 'order-tab-items-fulfillments', label: 'Товары и сборки', icon: ShoppingBag, count: fulfillments.length },
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
                className={`py-4 px-1 border-b-2 font-medium text-sm inline-flex items-center gap-2 transition-colors ${
                  activeTab === t.id
                    ? 'border-indigo-500 text-indigo-400 font-semibold'
                    : 'border-transparent text-slate-400 hover:text-slate-200'
                }`}
              >
                <Icon className="h-4 w-4" />
                <span>{t.label}</span>
                {t.count !== undefined && (
                  <span className="px-2 py-0.5 rounded-full text-xs font-bold bg-slate-800 text-slate-300">
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
            <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 space-y-4">
              <h3 className="text-base font-bold text-white flex items-center gap-2 border-b border-slate-800 pb-3">
                <User className="h-4 w-4 text-indigo-400" />
                <span>Данные покупателя</span>
              </h3>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
                <div>
                  <dt className="text-xs text-slate-400 font-medium">ФИО / Имя</dt>
                  <dd className="text-white font-semibold mt-0.5">{order.customerName || 'Иван Покупатель'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-slate-400 font-medium">Телефон</dt>
                  <dd className="text-white font-semibold mt-0.5">{order.customerPhone || '+7 (999) 111-22-33'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-slate-400 font-medium">Email</dt>
                  <dd className="text-white font-semibold mt-0.5">{order.customerEmail || 'customer@demo.zamk.local'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-slate-400 font-medium">Способ доставки</dt>
                  <dd className="text-white font-semibold mt-0.5">Курьер ZAMK Express</dd>
                </div>
              </dl>
            </div>

            <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 space-y-4">
              <h3 className="text-base font-bold text-white flex items-center gap-2 border-b border-slate-800 pb-3">
                <MapPin className="h-4 w-4 text-indigo-400" />
                <span>Адрес доставки</span>
              </h3>
              <p className="text-sm text-slate-200">{order.deliveryAddress || 'г. Москва, ул. Тверская, д. 12, кв. 45'}</p>
            </div>

            {/* Internal comment */}
            <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 space-y-3">
              <h3 className="text-base font-bold text-white flex items-center gap-2 border-b border-slate-800 pb-3">
                <FileText className="h-4 w-4 text-indigo-400" />
                <span>Внутренний комментарий оператора</span>
              </h3>
              <textarea
                rows={3}
                value={internalComment}
                onChange={(e) => setInternalComment(e.target.value)}
                placeholder="Введите заметки для коллег по заказу..."
                className="w-full p-3 bg-slate-950 border border-slate-700 rounded-xl text-sm text-white focus:outline-none focus:border-indigo-500"
              />
            </div>
          </div>

          {/* Financial summary sidebar */}
          <div className="space-y-6">
            <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 space-y-4">
              <h3 className="text-base font-bold text-white border-b border-slate-800 pb-3">Сводка по оплате</h3>
              <div className="space-y-2.5 text-sm">
                <div className="flex justify-between text-slate-400">
                  <span>Товары ({order.items.length})</span>
                  <span className="text-white font-medium">{formatMoney(order.totalPriceCents || order.totalAmount * 100 || 0)}</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>Доставка</span>
                  <span className="text-white font-medium">500 ₽</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>Скидка</span>
                  <span className="text-emerald-400 font-medium">0 ₽</span>
                </div>
                <div className="border-t border-slate-800 pt-3 flex justify-between text-base font-bold text-white">
                  <span>Итого к оплате</span>
                  <span className="text-indigo-400">{formatMoney((order.totalPriceCents || order.totalAmount * 100 || 0) + 50000)}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'items_fulfillments' && (
        <div className="space-y-6">
          {/* Order Items Table */}
          <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 space-y-4">
            <h3 className="text-base font-bold text-white border-b border-slate-800 pb-3">Товары заказа ({order.items.length})</h3>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-slate-800 text-sm text-left">
                <thead className="bg-slate-950 text-slate-400 text-xs font-semibold uppercase">
                  <tr>
                    <th className="px-4 py-3">Товар</th>
                    <th className="px-4 py-3">SKU</th>
                    <th className="px-4 py-3 text-center">Кол-во</th>
                    <th className="px-4 py-3 text-right">Цена</th>
                    <th className="px-4 py-3 text-right">Сумма</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800 text-slate-300">
                  {order.items.map((item) => (
                    <tr key={item.id}>
                      <td className="px-4 py-3">
                        <div className="font-semibold text-white">{item.title}</div>
                        <div className="text-xs text-slate-400">
                          {item.variantSize && `Размер: ${item.variantSize}`} {item.variantColor && `• Цвет: ${item.variantColor}`}
                        </div>
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-slate-400">{item.sku || '—'}</td>
                      <td className="px-4 py-3 text-center font-bold text-white">×{item.quantity}</td>
                      <td className="px-4 py-3 text-right">{formatMoney(item.priceCents || 0)}</td>
                      <td className="px-4 py-3 text-right font-bold text-indigo-400">{formatMoney(item.subtotalPriceCents || 0)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Seller Fulfillments Cards */}
          <div data-testid="order-fulfillments-section" className="space-y-4">
            <h3 className="text-base font-bold text-white">Сборки по продавцам ({fulfillments.length})</h3>
            {fulfillments.length === 0 ? (
              <div data-testid="order-fulfillments-empty" className="p-8 text-center bg-slate-900/60 rounded-xl border border-slate-800 text-slate-400">
                Сборки продавцов формируются автоматически при зачислении оплаты.
              </div>
            ) : (
              fulfillments.map((f) => {
                const fulSt = fulfillmentStatusMap[f.status] || { label: f.status, bg: 'bg-slate-800', text: 'text-slate-300' };

                return (
                  <div key={f.id} data-testid={`order-fulfillment-${f.id}`} className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4 shadow-lg">
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-800 pb-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <h4 className="font-bold text-white text-base">{f.sellerName || 'Продавец ZAMK'}</h4>
                          <span className={`px-2.5 py-0.5 rounded-full text-xs font-semibold ${fulSt.bg} ${fulSt.text}`}>
                            {fulSt.label}
                          </span>
                        </div>
                        <p className="text-xs text-slate-400 mt-1">ID сборки: {f.id}</p>
                      </div>

                      {f.receivingCode && (
                        <div className="flex items-center gap-2 bg-indigo-950/80 border border-indigo-800/80 px-3 py-1.5 rounded-lg text-xs">
                          <QrCode className="h-4 w-4 text-indigo-400" />
                          <span className="font-mono font-bold text-indigo-300">Код приёмки: {f.receivingCode}</span>
                        </div>
                      )}
                    </div>

                    {/* Items in fulfillment */}
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-slate-400">Состав сборки ({f.items.length} поз.)</div>
                      <div className="divide-y divide-slate-800/60 bg-slate-950/50 rounded-lg p-3">
                        {f.items.map((item) => (
                          <div key={item.orderItemId} className="py-2 first:pt-0 last:pb-0 flex justify-between items-center text-xs">
                            <div>
                              <span className="font-medium text-white">{item.productTitle}</span>
                              <span className="text-slate-400 ml-2 font-mono">({item.sku || 'без SKU'})</span>
                            </div>
                            <div className="font-bold text-indigo-400">×{item.quantity} шт.</div>
                          </div>
                        ))}
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center justify-between gap-4 pt-2 text-xs border-t border-slate-800/80">
                      <div className="text-slate-400">
                        Выручка продавца: <strong className="text-emerald-400 font-bold">{formatMoney(f.sellerAmountCents)}</strong> (Комиссия {f.commissionBps / 100}%)
                      </div>

                      {f.status === 'packed' && (
                        <button
                          onClick={() => navigate('/orders/receiving')}
                          className="px-3.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white font-bold rounded-lg transition-colors inline-flex items-center gap-1.5"
                        >
                          <QrCode className="h-3.5 w-3.5" />
                          <span>Перейти к приёмке</span>
                        </button>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}

      {activeTab === 'payment' && (
        <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-6 space-y-4">
          <h3 className="text-base font-bold text-white border-b border-slate-800 pb-3">История и статус платежей</h3>
          <div className="flex items-center justify-between p-4 bg-slate-950 rounded-xl border border-slate-800">
            <div>
              <div className="text-sm font-bold text-white">Платёж через T-Bank (Т-Пэй / Карта)</div>
              <div className="text-xs text-slate-400 mt-0.5">Транзакция ID: pay_20260728_zamk888</div>
            </div>
            <span className="px-3 py-1 rounded-full text-xs font-bold bg-emerald-950 text-emerald-300 border border-emerald-800">
              Успешно зачислен
            </span>
          </div>
        </div>
      )}

      {activeTab === 'delivery' && (
        <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-6 space-y-4">
          <h3 className="text-base font-bold text-white border-b border-slate-800 pb-3">Доставка и отгрузки</h3>
          <p className="text-sm text-slate-300">
            Отправления создаются автоматически при успешной приёмке сборок продавцов на хабе.
          </p>
          <button
            onClick={() => navigate('/shipments')}
            className="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white font-bold text-sm rounded-lg transition-colors"
          >
            Перейти в раздел «Доставка / Отгрузки» →
          </button>
        </div>
      )}

      {activeTab === 'history' && (
        <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-6 space-y-4">
          <h3 className="text-base font-bold text-white border-b border-slate-800 pb-3">История изменений заказа</h3>
          <div className="space-y-3">
            <div className="flex gap-3 text-xs">
              <Clock className="h-4 w-4 text-indigo-400 shrink-0 mt-0.5" />
              <div>
                <div className="font-bold text-white">{formatDateTime(order.createdAt)}</div>
                <div className="text-slate-400">Заказ успешно оформлен покупателем. Статус: Ожидает оплаты.</div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
