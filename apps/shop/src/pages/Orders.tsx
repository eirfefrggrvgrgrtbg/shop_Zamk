import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Package, ChevronRight, MapPin, CreditCard, Truck } from 'lucide-react';
import { Drawer } from '../components/ui/Drawer';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';
import { ReturnModal } from '../components/orders/ReturnModal';
import { ReviewModal } from '../components/orders/ReviewModal';
import { AccountNav } from '../components/account/AccountNav';
import { CustomerProtectedRoute } from '../components/account/CustomerProtectedRoute';

export function Orders() {
  return (
    <CustomerProtectedRoute
      title="Мои заказы"
      description="Войдите в аккаунт, чтобы просматривать историю заказов."
    >
      <OrdersContent />
    </CustomerProtectedRoute>
  );
}

function OrdersContent() {
  const [orders, setOrders] = useState<any[]>([]);
  const [returnsMap, setReturnsMap] = useState<Record<string, any[]>>({});
  const [selectedOrder, setSelectedOrder] = useState<any | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Modal states
  const [returnModal, setReturnModal] = useState<{isOpen: boolean, item: any | null}>({isOpen: false, item: null});
  const [reviewModal, setReviewModal] = useState<{isOpen: boolean, item: any | null}>({isOpen: false, item: null});

  const [fulfillments, setFulfillments] = useState<any[]>([]);
  const [isFulfillmentsLoading, setIsFulfillmentsLoading] = useState(false);
  const [fulfillmentsError, setFulfillmentsError] = useState<string | null>(null);

  const mapFulfillmentStatus = (s: string) => {
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

  const mapShipmentStatus = (s: string) => {
    switch (s) {
      case 'pending': return 'Ожидает подготовки';
      case 'assembling': return 'Собирается';
      case 'packed': return 'Упакован';
      case 'shipped': return 'Отправлен';
      case 'delivered': return 'Доставлен';
      case 'cancelled': return 'Отменён';
      default: return s;
    }
  };

  useEffect(() => {
    if (selectedOrder) {
      loadFulfillments(selectedOrder.rawId);
    } else {
      setFulfillments([]);
      setFulfillmentsError(null);
    }
  }, [selectedOrder]);

  const loadFulfillments = async (orderId: string) => {
    setIsFulfillmentsLoading(true);
    setFulfillmentsError(null);
    try {
      const { getCustomerOrderFulfillments } = await import('@zamk/api-client/src/customer');
      const fData = await getCustomerOrderFulfillments(orderId);
      setFulfillments(fData || []);
    } catch (err) {
      setFulfillmentsError("Не удалось загрузить состав заказа по продавцам.");
    } finally {
      setIsFulfillmentsLoading(false);
    }
  };

  const loadData = async () => {
    try {
      setIsLoading(true);
      const { getOrders, getCustomerReturns } = await import('@zamk/api-client/src/customer');
      const [data, returnsData] = await Promise.all([
        getOrders(),
        getCustomerReturns().catch(() => ({ items: [] }))
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
              case 'item_received': return 'Получен';
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
                statusText: mapReturnStatus(rData.status)
              });
            });
          }
          setReturnsMap(rMap);

          // Map to match the expected format roughly
          setOrders(data.map((o: any) => ({
            id: o.id.split('-')[0].toUpperCase() + '-' + o.id.split('-')[1].substring(0, 4),
            rawId: o.id,
            date: new Date(o.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }),
            status: mapOrderStatus(o.status),
            total: o.totalPriceCents / 100,
            delivery: {
              address: o.deliveryAddress,
            },
            items: o.items?.map((i: any) => ({
              orderItemId: i.id,
              productId: i.productId,
              id: i.productVariantId,
              name: i.productTitle || 'Товар',
              price: i.priceCents / 100,
              size: i.size,
              color: i.color,
              quantity: i.quantity,
              image: i.imageUrl || PRODUCT_PLACEHOLDER_IMAGE,
            })) || []
          })));
        
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
    <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[800px]'>
        
        {/* Шапка страницы */}
        <div className="mb-10 md:mb-12">
          <AccountNav />
          <div className="flex items-end justify-between mt-4">
            <h1 className="text-4xl md:text-5xl font-serif text-graphite dark:text-white tracking-tight leading-none">
              Мои заказы
            </h1>
            <span className="text-sm text-ash dark:text-white/60 mb-1 hidden sm:block">
              {orders.length} заказа(ов)
            </span>
          </div>
        </div>

        {/* Список заказов */}
        {isLoading ? (
          <div className="py-20 flex justify-center"><div className="animate-spin w-8 h-8 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent" /></div>
        ) : orders.length > 0 ? (
          <div className="space-y-5">
          {orders.map((order) => (
            <div 
              key={order.id} 
              className="group bg-white/50 hover:bg-white/80 dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-white/60 dark:border-white/10 shadow-[0_8px_30px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] rounded-[1.5rem] p-6 md:p-8 transition-all duration-500"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-5 sm:gap-0 border-b border-graphite/5 dark:border-white/10 pb-6 mb-6">
                <div>
                  <div className="flex items-center gap-4 mb-2">
                    <span className="font-medium text-graphite dark:text-white text-lg tracking-wide">{order.id}</span>
                    <span className={`text-[11px] uppercase tracking-wider font-medium px-3 py-1 rounded-full ${order.status === 'В пути' ? 'text-blue-600 dark:text-blue-400 bg-blue-50/80 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900/50' : 'text-graphite dark:text-white/70 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20'}`}>
                      {order.status}
                    </span>
                  </div>
                  <p className="text-[13.5px] text-ash dark:text-white/60 font-medium tracking-wide">{order.date}</p>
                </div>
                <div className="sm:text-right">
                  <p className="text-[12px] text-ash dark:text-white/60 uppercase tracking-wider mb-1">Сумма заказа</p>
                  <p className="font-serif text-xl text-graphite dark:text-white">{order.total.toLocaleString('ru-RU')} ₽</p>
                </div>
              </div>
              
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-6">
                <div className="flex-1">
                  <p className="text-[14px] text-graphite/70 dark:text-white/70 leading-relaxed font-medium">
                    {order.items.map((item: any) => {
                      const details = [item.name];
                      if (item.size) details.push(`Размер: ${item.size}`);
                      if (item.color) details.push(`Цвет: ${item.color}`);
                      if ((item.quantity ?? 1) > 1) details.push(`× ${item.quantity}`);
                      return details.join(' · ');
                    }).join(' / ')}
                  </p>
                </div>
                <button 
                  type="button"
                  onClick={() => setSelectedOrder(order)}
                  className="inline-flex items-center justify-center gap-2 px-6 py-3 bg-transparent border border-graphite/20 dark:border-white/20 hover:border-graphite dark:hover:border-white text-[13px] font-medium text-graphite dark:text-white rounded-full transition-all group-hover:bg-white dark:group-hover:bg-white/5"
                >
                  Детали заказа
                  <ChevronRight className="w-[14px] h-[14px]" />
                </button>
              </div>
            </div>
          ))}
          </div>
        ) : (
          <div className="bg-white/50 hover:bg-white/80 dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-white/60 dark:border-white/10 shadow-[0_8px_30px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] rounded-[1.5rem] p-8 md:p-10 text-center">
            <Package className="w-12 h-12 text-ash-light dark:text-white/50 mx-auto mb-4" />
            <h2 className="text-xl font-medium text-graphite dark:text-white mb-2">У вас пока нет заказов.</h2>
            <p className="text-sm text-ash dark:text-white/60 mb-6">После оформления заказа он появится здесь и сохранится после перезагрузки.</p>
            <Link to="/catalog" className="inline-flex items-center justify-center rounded-full border border-graphite/20 dark:border-white/20 px-5 py-3 text-[13px] font-medium text-graphite dark:text-white hover:border-graphite dark:hover:border-white transition-colors">
              В каталог
            </Link>
          </div>
        )}

      </div>

      <Drawer 
        isOpen={!!selectedOrder} 
        onClose={() => setSelectedOrder(null)} 
        position="right"
      >
        {selectedOrder && (
          <div className="flex flex-col h-full overflow-y-auto w-full max-w-[480px] bg-white dark:bg-black sm:rounded-l-[2rem]">
            {/* Header Drawer'а */}
            <div className="sticky top-0 z-10 bg-white/80 dark:bg-white/5 backdrop-blur-xl border-b border-border-lighter dark:border-white/10 px-6 py-5 flex items-center justify-between">
              <div>
                <h3 className="text-xl font-serif text-graphite dark:text-white">Заказ {selectedOrder.id}</h3>
                <p className="text-sm text-ash dark:text-white/60 mt-0.5">{selectedOrder.date}</p>
              </div>
              <button 
                type="button"
                onClick={() => setSelectedOrder(null)}
                className="w-10 h-10 rounded-full bg-graphite/5 dark:bg-white/10 flex items-center justify-center text-graphite dark:text-white hover:bg-graphite/10 dark:hover:bg-white/20 transition-colors"
                aria-label="Закрыть детали заказа"
              >
                ✕
              </button>
            </div>

            {/* Контент Drawer'а */}
            <div className="flex-1 p-6 space-y-8">
              
              {/* Статус */}
              <div className="flex items-center gap-3">
                <span className={`text-[12px] uppercase tracking-wider font-medium px-4 py-1.5 rounded-full ${selectedOrder.status === 'Отправлен' || selectedOrder.status === 'В пути' ? 'text-blue-600 dark:text-blue-400 bg-blue-50/80 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900/50' : 'text-graphite dark:text-white/70 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20'}`}>
                  {selectedOrder.status}
                </span>
              </div>

              {/* Состав заказа по продавцам */}
              <div>
                <h4 className="text-[13px] uppercase tracking-[0.05em] text-ash dark:text-white/60 font-medium mb-4">Состав заказа по продавцам</h4>
                
                {isFulfillmentsLoading ? (
                  <p className="text-[13px] text-ash dark:text-white/60">Загружаем состав заказа...</p>
                ) : fulfillmentsError ? (
                  <p className="text-[13px] text-red-500">{fulfillmentsError}</p>
                ) : fulfillments.length === 0 ? (
                  <p className="text-[13px] text-ash dark:text-white/60">Состав заказа не найден.</p>
                ) : (
                  <div className="space-y-6">
                    {fulfillments.map((f: any) => (
                      <div key={f.id} className="bg-graphite/[0.02] dark:bg-white/5 rounded-xl p-4 border border-graphite/5 dark:border-white/10">
                        {/* Фулфилмент Хедер */}
                        <div className="flex justify-between items-start mb-4 pb-3 border-b border-graphite/5 dark:border-white/10">
                          <div>
                            <p className="text-[14px] font-medium text-graphite dark:text-white">{f.sellerName || 'Неизвестный магазин'}</p>
                            <p className="text-[12px] text-ash dark:text-white/60 mt-0.5">Сборка #{f.id.substring(0, 8)}</p>
                          </div>
                          <div className="text-right">
                            <span className="text-[11px] uppercase tracking-wider font-medium px-2 py-0.5 rounded bg-graphite/10 dark:bg-white/20 text-graphite dark:text-white/80">
                              {mapFulfillmentStatus(f.status)}
                            </span>
                            {f.shipmentStatus && (
                              <p className="text-[11px] text-blue-600 dark:text-blue-400 mt-1">
                                {mapShipmentStatus(f.shipmentStatus)}
                              </p>
                            )}
                          </div>
                        </div>

                        {/* Товары в сборке */}
                        <div className="space-y-4">
                          {f.items.map((item: any, idx: number) => (
                            <div key={idx} className="flex flex-col gap-2 transition-colors">
                              <Link to={`/product/${item.productId}`} className="flex gap-4 group cursor-pointer">
                                <div className="w-16 h-20 bg-graphite/5 dark:bg-white/10 rounded-lg overflow-hidden flex-shrink-0">
                                  <img 
                                    src={item.imageUrl || PRODUCT_PLACEHOLDER_IMAGE} 
                                    alt={item.productTitle} 
                                    className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-700" 
                                  />
                                </div>
                                <div className="flex-1 py-0.5 flex flex-col justify-between">
                                  <div>
                                    <p className="text-[13.5px] font-medium text-graphite dark:text-white leading-snug group-hover:underline decoration-1 underline-offset-2">
                                      {item.productTitle}{(item.quantity ?? 1) > 1 ? ` × ${item.quantity}` : ''}
                                    </p>
                                    {item.sku && <p className="text-[12px] text-ash dark:text-white/60 mt-1">SKU: {item.sku}</p>}
                                  </div>
                                  <p className="font-serif text-[14px] text-graphite dark:text-white">{(item.unitPriceCents / 100).toLocaleString('ru-RU')} ₽</p>
                                </div>
                              </Link>
                              
                              {/* Кнопки возврата / отзыва */}
                              {selectedOrder.status === 'Доставлен' && (
                                <div className="flex gap-2 mt-1 ml-20">
                                  <button 
                                    onClick={(e) => {
                                      e.preventDefault();
                                      setReturnModal({ isOpen: true, item: { orderItemId: item.orderItemId, maxQuantity: item.quantity, productName: item.productTitle } });
                                    }}
                                    className="text-[11px] font-medium text-error hover:underline transition-colors"
                                  >
                                    Оформить возврат
                                  </button>
                                  <span className="text-ash/30">•</span>
                                  <button 
                                    onClick={(e) => {
                                      e.preventDefault();
                                      setReviewModal({ isOpen: true, item: { orderItemId: item.orderItemId, productName: item.productTitle } });
                                    }}
                                    className="text-[11px] font-medium text-primary hover:underline transition-colors"
                                  >
                                    Оставить отзыв
                                  </button>
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <div className="h-px bg-graphite/5 dark:bg-white/10 w-full"></div>

              {/* Возвраты и возмещения */}
              <div>
                <h4 className="text-[13px] uppercase tracking-[0.05em] text-ash dark:text-white/60 font-medium mb-4">Возвраты и возмещения</h4>
                {returnsMap[selectedOrder.rawId] && returnsMap[selectedOrder.rawId].length > 0 ? (
                  <div className="space-y-3">
                    {returnsMap[selectedOrder.rawId].map((ret: any, idx: number) => (
                      <div key={idx} className="p-3 bg-graphite/5 dark:bg-white/10 rounded-xl">
                        <div className="flex justify-between items-start mb-1">
                          <span className="text-[13px] font-medium text-graphite dark:text-white">Возврат #{ret.id.split('-')[0].toUpperCase()}</span>
                          <span className="text-[11px] uppercase tracking-wider font-medium px-2 py-0.5 rounded-full bg-graphite/10 dark:bg-white/20 text-graphite dark:text-white/80">
                            {ret.statusText}
                          </span>
                        </div>
                        {ret.adminComment && (
                          <p className="text-[12px] text-ash dark:text-white/60 mt-2 p-2 bg-white/50 dark:bg-black/20 rounded">
                            Комментарий: {ret.adminComment}
                          </p>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-[13px] text-ash dark:text-white/60">По этому заказу пока нет возвратов.</p>
                )}
              </div>

              <div className="h-px bg-graphite/5 dark:bg-white/10 w-full"></div>

              {/* Детали доставки */}
              <div>
                <h4 className="text-[13px] uppercase tracking-[0.05em] text-ash dark:text-white/60 font-medium mb-4">Доставка и оплата</h4>
                <div className="space-y-5">
                  <div className="flex gap-3">
                    <MapPin className="w-5 h-5 text-ash dark:text-white/60 mt-0.5" />
                    <div>
                      <p className="text-[14px] text-graphite dark:text-white font-medium">{selectedOrder.delivery.address}</p>
                    </div>
                  </div>
                  <div className="flex gap-3">
                    <CreditCard className="w-5 h-5 text-ash dark:text-white/60 mt-0.5" />
                    <div>
                      <p className="text-[14px] text-graphite dark:text-white font-medium">{selectedOrder.paymentMethod ?? 'Оплачено картой'}</p>
                      <p className="text-[13px] text-ash dark:text-white/60 mt-0.5">**** 4592</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Итого */}
              <div className="bg-graphite/5 dark:bg-white/10 rounded-2xl p-5 mt-8">
                <div className="flex justify-between items-center mb-2">
                  <span className="text-sm text-graphite/70 dark:text-white/70">Товары ({selectedOrder.items.length})</span>
                  <span className="text-sm font-medium text-graphite dark:text-white">{selectedOrder.total.toLocaleString('ru-RU')} ₽</span>
                </div>
                <div className="flex justify-between items-center mb-4">
                  <span className="text-sm text-graphite/70 dark:text-white/70">Доставка</span>
                  <span className="text-sm font-medium text-graphite dark:text-white">
                    {selectedOrder.deliveryCost !== undefined
                      ? (selectedOrder.deliveryCost ? `${selectedOrder.deliveryCost.toLocaleString('ru-RU')} ₽` : 'Бесплатно')
                      : 'Бесплатно'}
                  </span>
                </div>
                <div className="h-px bg-graphite/10 dark:bg-white/20 w-full mb-4"></div>
                <div className="flex justify-between items-center">
                  <span className="text-base font-medium text-graphite dark:text-white">Итого</span>
                  <span className="text-xl font-serif text-graphite dark:text-white">{selectedOrder.total.toLocaleString('ru-RU')} ₽</span>
                </div>
              </div>
            </div>
          </div>
        )}
      </Drawer>

      {/* Modals */}
      {selectedOrder && returnModal.isOpen && returnModal.item && (
        <ReturnModal 
          isOpen={returnModal.isOpen} 
          onClose={() => setReturnModal({ isOpen: false, item: null })} 
          orderId={selectedOrder.rawId} 
          item={returnModal.item}
          onSuccess={loadData}
        />
      )}
      {selectedOrder && reviewModal.isOpen && reviewModal.item && (
        <ReviewModal 
          isOpen={reviewModal.isOpen} 
          onClose={() => setReviewModal({ isOpen: false, item: null })} 
          orderId={selectedOrder.rawId} 
          orderItemId={reviewModal.item.orderItemId}
          productName={reviewModal.item.productName}
          onSuccess={loadData}
        />
      )}
    </div>
  );
}