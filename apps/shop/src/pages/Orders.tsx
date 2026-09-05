import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Package, ChevronRight, MapPin, Star, CheckCircle2, Circle, X } from 'lucide-react';
import { Drawer } from '../components/ui/Drawer';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';
import { ReturnModal } from '../components/orders/ReturnModal';
import { ReviewModal } from '../components/orders/ReviewModal';
import {
  shouldShowReviewCTA,
  getReviewStatusBadgeText,
  REVIEW_STATUS_STYLES,
  getOrderStatusStyle,
  getItemCountWord,
} from '../components/orders/reviewHelpers';
import { AccountLayout } from '../components/account/AccountLayout';
import { useToast } from '../contexts/ToastContext';

export function Orders() {
  return (
    <AccountLayout title="Мои заказы">
      <OrdersContent />
    </AccountLayout>
  );
}

function OrdersContent() {
  const { showToast } = useToast();
  const [orders, setOrders] = useState<any[]>([]);
  const [returnsMap, setReturnsMap] = useState<Record<string, any[]>>({});
  const [reviewsMap, setReviewsMap] = useState<Record<string, any>>({});
  const [selectedOrder, setSelectedOrder] = useState<any | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isPaying, setIsPaying] = useState(false);

  // Modal states
  const [returnModal, setReturnModal] = useState<{ isOpen: boolean; item: any | null }>({ isOpen: false, item: null });
  const [reviewModal, setReviewModal] = useState<{ isOpen: boolean; item: any | null }>({ isOpen: false, item: null });

  const loadData = async () => {
    try {
      setIsLoading(true);
      const { getOrders, getCustomerReturns, getCustomerReviews } = await import('@zamk/api-client/src/customer');
      const [data, returnsData, reviewsData] = await Promise.all([
        getOrders(),
        getCustomerReturns().catch(() => ({ items: [] })),
        getCustomerReviews().catch(() => ({ items: [] })),
      ]);

      const mapOrderStatus = (s: string) => {
        switch (s) {
          case 'awaiting_payment': return 'Ожидает оплаты';
          case 'paid': return 'Оплачен';
          case 'assembling': return 'Собирается';
          case 'packed': return 'Упакован';
          case 'shipped': return 'Отправлен';
          case 'delivered': return 'Доставлен';
          case 'cancelled': return 'Отменён';
          case 'returned': return 'Возврат';
          case 'refunded': return 'Возмещён';
          default: return s;
        }
      };

      const mapReturnStatus = (s: string) => {
        switch (s) {
          case 'requested': return 'Запрошен';
          case 'approved': return 'Одобрен';
          case 'rejected': return 'Отклонён';
          case 'item_received': return 'Получен на складе';
          case 'refunded': return 'Возмещён';
          case 'completed': return 'Завершён';
          case 'cancelled': return 'Отменён';
          default: return s;
        }
      };

      const rMap: Record<string, any[]> = {};
      if (returnsData && returnsData.items) {
        returnsData.items.forEach((r: any) => {
          const rData = r.return || r;
          if (!rMap[rData.orderId]) rMap[rData.orderId] = [];
          rMap[rData.orderId].push({
            ...rData,
            statusText: mapReturnStatus(rData.status),
            items: r.items || [],
          });
        });
      }
      setReturnsMap(rMap);

      const revMap: Record<string, any> = {};
      if (reviewsData && reviewsData.items) {
        reviewsData.items.forEach((rev: any) => {
          if (rev.orderItemId) {
            revMap[rev.orderItemId] = rev;
          }
        });
      }
      setReviewsMap(revMap);

      const mappedOrders = (data || []).map((o: any) => ({
        id: o.orderNumber || (o.id.split('-')[0].toUpperCase() + '-' + o.id.split('-')[1].substring(0, 4)),
        rawId: o.id,
        rawStatus: o.status,
        date: new Date(o.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }),
        createdAt: o.createdAt,
        status: mapOrderStatus(o.status),
        total: (o.totalPriceCents || 0) / 100,
        delivery: {
          address: o.deliveryAddress || 'Адрес не указан',
          methodName: o.deliveryMethodName || 'Курьерская доставка ZAMK',
          cost: o.deliveryPriceCents !== undefined ? (o.deliveryPriceCents || 0) / 100 : 0,
        },
        items: (o.items || []).map((i: any) => ({
          orderItemId: i.id || i.orderItemId,
          productId: i.productId,
          productVariantId: i.productVariantId,
          name: i.productTitle || i.title || 'Товар ZAMK',
          price: (i.priceCents || i.unitPriceCents || 0) / 100,
          size: i.size,
          color: i.color,
          quantity: i.quantity || 1,
          sellerName: i.sellerName || 'ZAMK Store',
          image: i.imageUrl || PRODUCT_PLACEHOLDER_IMAGE,
        })),
      }));

      setOrders(mappedOrders);

      // Keep selected order synced if open
      if (selectedOrder) {
        const updatedSelected = mappedOrders.find((mo: any) => mo.rawId === selectedOrder.rawId);
        if (updatedSelected) {
          setSelectedOrder(updatedSelected);
        }
      }
    } catch (err) {
      console.error('Failed to load orders', err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  return (
    <>
      <div>
        <div className="flex items-center justify-between mb-6">
          <span className="text-[14px] font-medium text-graphite/70 dark:text-white/70">
            {orders.length} {getItemCountWord(orders.length)}
          </span>
        </div>

        {/* Список заказов */}
        {isLoading ? (
          <div className="py-24 flex justify-center">
            <div className="animate-spin w-7 h-7 border-2 border-graphite border-t-transparent rounded-full dark:border-white dark:border-t-transparent" />
          </div>
        ) : orders.length > 0 ? (
          <div className="space-y-4">
            {orders.map((order) => (
              <div
                key={order.id}
                onClick={() => setSelectedOrder(order)}
                className="group bg-white/80 hover:bg-white dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-border-lighter dark:border-white/10 shadow-[0_4px_24px_rgba(0,0,0,0.03)] hover:shadow-[0_8px_32px_rgba(0,0,0,0.06)] rounded-3xl p-6 sm:p-7 transition-all duration-300 cursor-pointer"
              >
                {/* Верхняя строка: Номер, статус, дата, сумма */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-5 border-b border-border-lighter dark:border-white/10">
                  <div className="flex items-center gap-3">
                    <span className="text-[16px] sm:text-[17px] font-semibold text-graphite dark:text-white tracking-tight">
                      {order.id}
                    </span>
                    <span className={`text-[12px] px-3 py-1 rounded-full font-medium ${getOrderStatusStyle(order.rawStatus)}`}>
                      {order.status}
                    </span>
                  </div>
                  <div className="flex items-center gap-4 sm:gap-6">
                    <span className="text-graphite/70 dark:text-white/70 text-[13.5px] font-medium">{order.date}</span>
                    <span className="text-graphite/20 dark:text-white/20 hidden sm:inline">•</span>
                    <span className="font-semibold text-graphite dark:text-white text-[15px]">
                      {order.items.length} {getItemCountWord(order.items.length)} · {order.total.toLocaleString('ru-RU')} ₽
                    </span>
                  </div>
                </div>

                {/* Компактный список товаров заказа */}
                <div className="pt-5 space-y-4">
                  {order.items.map((item: any) => {
                    const rev = reviewsMap[item.orderItemId];
                    return (
                      <div key={item.orderItemId || item.productId} className="flex items-center justify-between gap-4">
                        <div className="flex items-center gap-4 min-w-0">
                          <div className="w-14 h-16 sm:w-16 sm:h-20 rounded-xl bg-graphite/5 dark:bg-white/10 overflow-hidden flex-shrink-0 border border-border-lighter dark:border-white/10">
                            <img
                              src={item.image || PRODUCT_PLACEHOLDER_IMAGE}
                              alt={item.name}
                              className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-500"
                            />
                          </div>
                          <div className="min-w-0">
                            <h4 className="text-[15px] font-medium text-graphite dark:text-white truncate group-hover:text-primary transition-colors">
                              {item.name}
                            </h4>
                            <p className="text-[13px] text-graphite/70 dark:text-white/70 mt-0.5 font-normal">
                              {[item.size, item.color].filter(Boolean).join(' · ') || 'Единый размер'}
                              {(item.quantity ?? 1) > 1 ? ` · ${item.quantity} шт.` : ''}
                            </p>

                            {/* Статус отзыва прямо под товаром при наличии */}
                            {rev && (
                              <div className="flex items-center gap-2 mt-1.5">
                                <span className={`text-[12px] px-2.5 py-0.5 rounded-full font-medium ${REVIEW_STATUS_STYLES[rev.status] || 'bg-graphite/5 dark:bg-white/10 text-graphite'}`}>
                                  {getReviewStatusBadgeText(rev)}
                                </span>
                                <div className="flex items-center text-amber-500 text-xs">
                                  {Array.from({ length: rev.rating || 5 }).map((_, i) => (
                                    <Star key={i} className="w-3.5 h-3.5 fill-current" />
                                  ))}
                                </div>
                              </div>
                            )}
                          </div>
                        </div>

                        <div className="text-right flex-shrink-0">
                          <p className="text-[15px] font-semibold text-graphite dark:text-white">
                            {item.price.toLocaleString('ru-RU')} ₽
                          </p>
                        </div>
                      </div>
                    );
                  })}
                </div>

                {/* Нижняя строчка с призывом открыть детали */}
                <div className="mt-5 pt-4 border-t border-border-lighter/70 dark:border-white/5 flex items-center justify-between text-[13.5px] text-graphite/70 dark:text-white/70">
                  <span className="group-hover:text-graphite dark:group-hover:text-white transition-colors font-medium">
                    Детали и история заказа
                  </span>
                  <span className="inline-flex items-center gap-1 font-semibold text-graphite dark:text-white group-hover:translate-x-0.5 transition-transform">
                    Открыть
                    <ChevronRight className="w-4 h-4" />
                  </span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="bg-white/50 hover:bg-white/80 dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-white/60 dark:border-white/10 shadow-[0_8px_30px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] rounded-[1.5rem] p-8 md:p-10 text-center">
            <Package className="w-12 h-12 text-ash-light dark:text-white/50 mx-auto mb-4" />
            <h2 className="text-xl font-medium text-graphite dark:text-white mb-2">У вас пока нет заказов.</h2>
            <p className="text-sm text-graphite/70 dark:text-white/70 mb-6">После оформления заказа он появится здесь и сохранится в вашем профиле.</p>
            <Link to="/catalog" className="inline-flex items-center justify-center rounded-full border border-graphite/20 dark:border-white/20 px-5 py-3 text-[13.5px] font-medium text-graphite dark:text-white hover:border-graphite dark:hover:border-white transition-colors">
              В каталог
            </Link>
          </div>
        )}
      </div>

      {/* Правый Drawer "Детали заказа" */}
      <Drawer
        isOpen={!!selectedOrder}
        onClose={() => setSelectedOrder(null)}
        position="right"
        widthClassName="w-full sm:w-[460px]"
        hideHeader={true}
      >
        {selectedOrder && (
          <div className="flex flex-col h-full bg-white dark:bg-[#111214]">
            {/* Хедер Drawer'а */}
            <div className="border-b border-border-lighter dark:border-white/10 pb-5">
              <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <h3 className="text-[18px] sm:text-[19px] font-semibold text-graphite dark:text-white tracking-tight">
                    Заказ {selectedOrder.id}
                  </h3>
                  <span className={`text-[12px] px-3 py-1 rounded-full font-medium ${getOrderStatusStyle(selectedOrder.rawStatus)}`}>
                    {selectedOrder.status}
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => setSelectedOrder(null)}
                  className="p-2 text-graphite/60 hover:text-graphite dark:text-white/60 dark:hover:text-white bg-ice/70 dark:bg-white/5 hover:bg-ice dark:hover:bg-white/10 rounded-full transition-all flex-shrink-0"
                  aria-label="Закрыть детали заказа"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
              <p className="text-[13.5px] font-medium text-graphite/70 dark:text-white/70 mt-1">{selectedOrder.date}</p>

              {/* Компактный прогресс-бар доставки (Оформлен → В пути → Доставлен) */}
              <div className="mt-5 pt-4 border-t border-border-lighter/60 dark:border-white/5">
                <div className="flex items-center justify-between text-xs">
                  {[
                    { label: 'Оформлен', isDone: true },
                    {
                      label: 'В пути',
                      isDone: selectedOrder.rawStatus === 'shipped' || selectedOrder.rawStatus === 'delivered' || selectedOrder.rawStatus === 'returned' || selectedOrder.rawStatus === 'refunded',
                    },
                    {
                      label: 'Доставлен',
                      isDone: selectedOrder.rawStatus === 'delivered' || selectedOrder.rawStatus === 'returned' || selectedOrder.rawStatus === 'refunded',
                    },
                  ].map((step, idx, arr) => (
                    <div key={step.label} className="flex items-center flex-1 last:flex-none">
                      <div className="flex items-center gap-1.5">
                        {step.isDone ? (
                          <CheckCircle2 className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
                        ) : (
                          <Circle className="w-4 h-4 text-graphite/30 dark:text-white/20" />
                        )}
                        <span className={`text-[13px] font-medium ${step.isDone ? 'text-graphite dark:text-white font-semibold' : 'text-graphite/50 dark:text-white/40'}`}>
                          {step.label}
                        </span>
                      </div>
                      {idx < arr.length - 1 && (
                        <div className={`h-px flex-1 mx-2 ${step.isDone && arr[idx + 1].isDone ? 'bg-emerald-600 dark:bg-emerald-400' : 'bg-border-lighter dark:bg-white/10'}`} />
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </div>

            {/* Контент Drawer'а */}
            <div className="py-6 space-y-7">
              {/* Главный блок товара (Product-First Content) */}
              <div className="space-y-6">
                {selectedOrder.items.map((item: any) => {
                  const rev = reviewsMap[item.orderItemId];
                  const hasReview = !!rev;
                  const isDelivered = selectedOrder.status === 'Доставлен' || selectedOrder.rawStatus === 'delivered';
                  const returnRecords = returnsMap[selectedOrder.rawId] || [];
                  const activeReturn = returnRecords.find((r: any) => r.status !== 'cancelled');

                  return (
                    <div key={item.orderItemId || item.productId} className="space-y-4">
                      {/* Карточка товара */}
                      <div className="flex gap-4 p-4 rounded-2xl bg-graphite/[0.02] dark:bg-white/5 border border-border-lighter dark:border-white/10">
                        <div className="w-20 h-24 sm:w-24 sm:h-28 rounded-xl bg-graphite/5 dark:bg-white/10 overflow-hidden flex-shrink-0 border border-border-lighter dark:border-white/10">
                          <img
                            src={item.image || PRODUCT_PLACEHOLDER_IMAGE}
                            alt={item.name}
                            className="w-full h-full object-cover object-center"
                          />
                        </div>
                        <div className="flex-1 flex flex-col justify-between py-0.5">
                          <div>
                            <Link
                              to={`/product/${item.productId}`}
                              className="text-[15.5px] font-semibold text-graphite dark:text-white hover:underline decoration-1 underline-offset-2 leading-snug"
                            >
                              {item.name}
                            </Link>
                            <p className="text-[13.5px] text-graphite/75 dark:text-white/75 mt-1 font-normal">
                              {[item.size, item.color].filter(Boolean).join(' · ') || 'Единый размер'}
                              {(item.quantity ?? 1) > 1 ? ` · ${item.quantity} шт.` : ''}
                            </p>
                            <p className="text-[13px] text-graphite/60 dark:text-white/50 mt-0.5">
                              Продавец: {item.sellerName || 'ZAMK Store'}
                            </p>
                          </div>
                          <p className="text-[16px] font-semibold text-graphite dark:text-white">
                            {item.price.toLocaleString('ru-RU')} ₽
                          </p>
                        </div>
                      </div>

                      {/* Действия по товару (Оставить отзыв / Оформить возврат) */}
                      {isDelivered && (
                        <div className="flex items-center gap-2 pt-1">
                          {!hasReview && (
                            <button
                              type="button"
                              onClick={(e) => {
                                e.preventDefault();
                                setReviewModal({
                                  isOpen: true,
                                  item: {
                                    orderId: selectedOrder.rawId,
                                    orderItemId: item.orderItemId,
                                    productName: item.name,
                                  },
                                });
                              }}
                              className="flex-1 py-2.5 px-4 rounded-xl bg-graphite dark:bg-white text-white dark:text-black text-[13.5px] font-medium hover:opacity-90 transition-opacity text-center shadow-sm"
                            >
                              Оставить отзыв
                            </button>
                          )}

                          {!activeReturn && (
                            <button
                              type="button"
                              onClick={(e) => {
                                e.preventDefault();
                                setReturnModal({
                                  isOpen: true,
                                  item: {
                                    orderItemId: item.orderItemId,
                                    maxQuantity: item.quantity,
                                    productName: item.name,
                                  },
                                });
                              }}
                              className={`py-2.5 px-4 rounded-xl border border-border-lighter dark:border-white/20 hover:border-graphite/40 dark:hover:border-white/40 text-graphite dark:text-white text-[13.5px] font-medium transition-colors text-center ${
                                hasReview ? 'flex-1' : ''
                              }`}
                            >
                              Оформить возврат
                            </button>
                          )}
                        </div>
                      )}

                      {/* Интеграция отзыва прямо под товаром: Секция "Ваш отзыв" */}
                      {hasReview && (
                        <div className="p-4 rounded-2xl bg-amber-50/50 dark:bg-amber-950/20 border border-amber-200/70 dark:border-amber-800/40 space-y-2.5">
                          <div className="flex items-center justify-between">
                            <span className="text-[14px] font-semibold text-graphite dark:text-white">
                              Ваш отзыв
                            </span>
                            <span className={`text-[12px] px-2.5 py-0.5 rounded-full font-medium ${REVIEW_STATUS_STYLES[rev.status] || 'bg-amber-100 text-amber-800'}`}>
                              {getReviewStatusBadgeText(rev)}
                            </span>
                          </div>

                          <div className="flex items-center gap-1 text-amber-500">
                            {Array.from({ length: 5 }).map((_, i) => (
                              <Star
                                key={i}
                                className={`w-4 h-4 ${i < (rev.rating || 5) ? 'fill-current' : 'text-gray-300 dark:text-white/20'}`}
                              />
                            ))}
                          </div>

                          {rev.title && (
                            <p className="text-[14.5px] font-semibold text-graphite dark:text-white leading-snug">
                              {rev.title}
                            </p>
                          )}

                          {(rev.comment || rev.text) && (
                            <p className="text-[13.5px] text-graphite/90 dark:text-white/90 leading-relaxed font-normal">
                              {rev.comment || rev.text}
                            </p>
                          )}

                          {rev.moderationComment && rev.status === 'rejected' && (
                            <div className="mt-2 p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-[13px] text-rose-700 dark:text-rose-400">
                              <span className="font-semibold">Причина отклонения: </span>
                              {rev.moderationComment}
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>

              {/* Доставка */}
              <div className="space-y-3 pt-2">
                <h4 className="text-[13.5px] font-semibold text-graphite dark:text-white uppercase tracking-wider">
                  Доставка
                </h4>
                <div className="grid grid-cols-1 gap-2.5">
                  <div className="flex items-start gap-3 p-3.5 rounded-2xl bg-graphite/[0.02] dark:bg-white/5 border border-border-lighter dark:border-white/10">
                    <MapPin className="w-4 h-4 text-graphite/60 dark:text-white/60 mt-0.5 flex-shrink-0" />
                    <div>
                      <p className="text-[12px] text-graphite/60 dark:text-white/50 uppercase tracking-wider font-semibold">Адрес доставки</p>
                      <p className="text-[14px] text-graphite dark:text-white font-medium mt-0.5">{selectedOrder.delivery.address}</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Итоговая сводка стоимости */}
              <div className="p-4 rounded-2xl bg-graphite/[0.03] dark:bg-white/5 border border-border-lighter dark:border-white/10 space-y-2.5">
                <div className="flex justify-between items-center text-[14px]">
                  <span className="text-graphite/80 dark:text-white/80 font-normal">Товары ({selectedOrder.items.length})</span>
                  <span className="font-semibold text-graphite dark:text-white">{selectedOrder.total.toLocaleString('ru-RU')} ₽</span>
                </div>
                <div className="flex justify-between items-center text-[14px]">
                  <span className="text-graphite/80 dark:text-white/80 font-normal">Доставка</span>
                  <span className="font-semibold text-graphite dark:text-white">
                    {selectedOrder.delivery.cost ? `${selectedOrder.delivery.cost.toLocaleString('ru-RU')} ₽` : 'Бесплатно'}
                  </span>
                </div>

                <div className="h-px bg-border-lighter dark:border-white/10 w-full my-1" />
                <div className="flex justify-between items-center pt-1">
                  <span className="text-[15.5px] font-semibold text-graphite dark:text-white">
                    Итого
                  </span>
                  <span className="text-[20px] font-bold text-graphite dark:text-white">
                    {selectedOrder.total.toLocaleString('ru-RU')} ₽
                  </span>
                </div>

                {selectedOrder.rawStatus === 'awaiting_payment' && (
                  <button
                    type="button"
                    disabled={isPaying}
                    onClick={async () => {
                      try {
                        setIsPaying(true);
                        const { createPayment } = await import('@zamk/api-client/src/customer');
                        const payment = await createPayment(selectedOrder.rawId, 'card');
                        if (payment.paymentUrl) {
                          window.location.href = payment.paymentUrl;
                        }
                      } catch (e: any) {
                        setIsPaying(false);
                        if (e.message && e.message.includes('insufficient_stock')) {
                          showToast('Товара больше нет в наличии для завершения оплаты', 'error');
                        } else {
                          showToast(e.message || 'Не удалось начать оплату заказа', 'error');
                        }
                      }
                    }}
                    className="w-full mt-4 py-3 rounded-full bg-graphite dark:bg-white text-white dark:text-graphite hover:opacity-90 font-semibold text-sm text-center transition-all disabled:opacity-50"
                  >
                    {isPaying ? 'Подготовка к оплате...' : 'Оплатить заказ'}
                  </button>
                )}
              </div>
            </div>
          </div>
        )}
      </Drawer>

      {/* Модалки */}
      {selectedOrder && returnModal.isOpen && returnModal.item && (
        <ReturnModal
          isOpen={returnModal.isOpen}
          onClose={() => setReturnModal({ isOpen: false, item: null })}
          orderId={selectedOrder.rawId}
          item={returnModal.item}
          onSuccess={loadData}
        />
      )}
      <ReviewModal
        isOpen={Boolean(selectedOrder && reviewModal.isOpen && reviewModal.item)}
        onClose={() => setReviewModal({ isOpen: false, item: null })}
        orderId={selectedOrder?.rawId}
        orderItemId={reviewModal.item?.orderItemId || ''}
        productName={reviewModal.item?.productName || ''}
        onSuccess={loadData}
      />
    </>
  );
}
