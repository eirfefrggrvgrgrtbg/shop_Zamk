import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Package, ChevronRight, ArrowLeft, MapPin, CreditCard, Truck } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { Drawer } from '../components/ui/Drawer';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';

export function Orders() {
  const { isAuthenticated } = useAuth();
  const [orders, setOrders] = useState<any[]>([]);
  const [selectedOrder, setSelectedOrder] = useState<any | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    if (!isAuthenticated) return;
    
    let isMounted = true;
    const fetchOrders = async () => {
      try {
        setIsLoading(true);
        const { getOrders } = await import('@zamk/api-client/src/customer');
        const data = await getOrders();
        if (isMounted) {
          // Map to match the expected format roughly
          setOrders(data.map(o => ({
            id: o.id.split('-')[0].toUpperCase() + '-' + o.id.split('-')[1].substring(0, 4),
            rawId: o.id,
            date: new Date(o.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }),
            status: o.status === 'created' ? 'Создан' : o.status === 'paid' ? 'Оплачен' : o.status === 'shipped' ? 'В пути' : o.status === 'delivered' ? 'Доставлен' : o.status,
            total: o.totalPriceCents / 100,
            delivery: {
              address: o.deliveryAddress,
              service: 'Курьер'
            },
            items: o.items.map((item: any) => ({
              id: item.productId,
              orderItemId: item.id,  // P0 fix: store raw order item ID for return/review
              name: item.title || 'Товар',
              price: item.priceCents / 100,
              quantity: item.quantity,
              image: item.imageUrl || PRODUCT_PLACEHOLDER_IMAGE
            }))
          })));
        }
      } catch (e) {
        console.error('Failed to load orders', e);
      } finally {
        if (isMounted) setIsLoading(false);
      }
    };
    
    fetchOrders();
    return () => { isMounted = false; };
  }, [isAuthenticated]);
  
  if (!isAuthenticated) {
    return (
      <div className="pt-32 pb-20 min-h-[70vh] flex flex-col items-center justify-center text-center px-4">
        <Package className="w-12 h-12 text-ash-light dark:text-white/50 mb-4" />
        <h2 className="text-xl font-medium text-graphite dark:text-white mb-2">Доступ закрыт</h2>
        <p className="text-sm text-ash dark:text-white/60 mb-6">Войдите в аккаунт, чтобы просматривать историю заказов</p>
        <Link to="/" className="text-[13px] font-medium border-b border-graphite dark:border-white pb-0.5 uppercase tracking-wide dark:text-white">
          Вернуться на главную
        </Link>
      </div>
    );
  }

  return (
    <div className='relative z-10 min-h-screen pt-24 md:pt-32 pb-20'>
      <div className='container mx-auto px-4 sm:px-6 max-w-[800px]'>
        
        {/* Шапка страницы */}
        <div className="mb-10 md:mb-12">
          <Link to="/profile" className="inline-flex items-center gap-2 text-[13px] text-ash dark:text-white/60 hover:text-graphite dark:hover:text-white transition-colors mb-6 font-medium uppercase tracking-[0.04em]">
            <ArrowLeft className="w-[14px] h-[14px]" />
            Мой профиль
          </Link>
          <div className="flex items-end justify-between">
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
            <h2 className="text-xl font-medium text-graphite dark:text-white mb-2">Заказов пока нет</h2>
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
                <span className={`text-[12px] uppercase tracking-wider font-medium px-4 py-1.5 rounded-full ${selectedOrder.status === 'В пути' ? 'text-blue-600 dark:text-blue-400 bg-blue-50/80 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900/50' : 'text-graphite dark:text-white/70 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20'}`}>
                  {selectedOrder.status}
                </span>
                <span className="text-sm text-ash dark:text-white/60">Ожидаемая дата: 25 марта</span>
              </div>

              {/* Состав заказа */}
              <div>
                <h4 className="text-[13px] uppercase tracking-[0.05em] text-ash dark:text-white/60 font-medium mb-4">Состав заказа</h4>
                <div className="space-y-4">
                  {selectedOrder.items.map((item: any, idx: number) => (
                    <Link to={`/product/${item.id}`} key={idx} className="flex gap-4 group cursor-pointer hover:bg-graphite/[0.02] dark:hover:bg-white/5 p-2 -m-2 rounded-xl transition-colors">
                      <div className="w-20 h-24 bg-graphite/5 dark:bg-white/10 rounded-xl overflow-hidden flex-shrink-0">
                        <img 
                          src={item.image} 
                          alt={item.name} 
                          className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-700" 
                        />
                      </div>
                      <div className="flex-1 py-1 flex flex-col justify-between">
                        <div>
                          <p className="text-[14.5px] font-medium text-graphite dark:text-white leading-snug group-hover:underline decoration-1 underline-offset-2">
                            {item.name}{(item.quantity ?? 1) > 1 ? ` × ${item.quantity}` : ''}
                          </p>
                          {item.size && <p className="text-[13px] text-ash dark:text-white/60 mt-1">Размер: {item.size}</p>}
                          {item.color && <p className="text-[13px] text-ash dark:text-white/60 mt-1">Цвет: {item.color}</p>}
                        </div>
                        <p className="font-serif text-[15px] text-graphite dark:text-white">{item.price.toLocaleString('ru-RU')} ₽</p>
                      </div>
                    </Link>
                  ))}
                </div>
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
                      <p className="text-[13px] text-ash dark:text-white/60 mt-0.5">{selectedOrder.delivery.service}</p>
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

              {selectedOrder.status === 'Доставлен' && (
                <div className="mt-8 flex gap-3">
                  <button 
                    onClick={async () => {
                      const reason = window.prompt('Укажите причину возврата:');
                      if (reason) {
                        try {
                          const { createReturn } = await import('@zamk/api-client/src/customer');
                          // P0 fix: orderId in path, items[] required by backend
                          await createReturn(selectedOrder.rawId, {
                            reason,
                            items: selectedOrder.items.map((item: any) => ({
                              orderItemId: item.orderItemId,
                              quantity: item.quantity ?? 1,
                            })),
                          });
                          alert('Заявка на возврат успешно создана');
                        } catch (e: any) {
                          alert(e.message || 'Ошибка при создании возврата');
                        }
                      }
                    }}
                    className="flex-1 py-3 border border-error text-error text-[13px] font-medium rounded-full hover:bg-error hover:text-white transition-colors"
                  >
                    Оформить возврат
                  </button>
                  <button 
                    onClick={async () => {
                      const firstItem = selectedOrder.items[0];
                      if (!firstItem?.orderItemId) return;
                      const comment = window.prompt('Ваш отзыв:');
                      if (comment) {
                        try {
                          const { createReview } = await import('@zamk/api-client/src/customer');
                          // P0 fix: orderId + orderItemId in path, field is comment not content
                          await createReview(selectedOrder.rawId, firstItem.orderItemId, { rating: 5, title: 'Отзыв', comment });
                          alert('Отзыв успешно добавлен');
                        } catch (e: any) {
                          alert(e.message || 'Ошибка при добавлении отзыва');
                        }
                      }
                    }}
                    className="flex-1 py-3 border border-graphite dark:border-white text-graphite dark:text-white text-[13px] font-medium rounded-full hover:bg-graphite hover:text-white dark:hover:bg-white dark:hover:text-black transition-colors"
                  >
                    Оставить отзыв
                  </button>
                </div>
              )}

            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}