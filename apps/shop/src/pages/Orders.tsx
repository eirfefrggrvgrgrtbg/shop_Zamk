import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Package, ChevronRight, ArrowLeft, MapPin, CreditCard, Truck } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { Drawer } from '../components/ui/Drawer';
import { ORDER_STORAGE_EVENT, type OrderRecord, getOrdersWithDefaults } from '../lib/orders';

const MOCK_ORDERS: OrderRecord[] = [
  {
    id: 'ZMK-10293',
    date: '20 марта 2026',
    status: 'В пути',
    statusColor: 'text-blue-600 bg-blue-50/80 border border-blue-100',
    total: 32500,
    delivery: {
      address: 'Москва, ул. Тверская 12, кв. 44',
      service: 'Курьер СДЭК'
    },
    items: [
      { id: 'p8', name: 'Пальто-кокон северной серии', size: 'M', price: 34900, image: 'https://wsrv.nl/?url=images.unsplash.com/photo-1539533113208-f6df8cc8b543&w=800&output=webp' },
      { id: 'p4', name: 'Свитер объемной вязки', size: 'L', price: 12500, image: 'https://wsrv.nl/?url=images.unsplash.com/photo-1576566588028-4147f3842f27&w=800&output=webp' }
    ]
  },
  {
    id: 'ZMK-08112',
    date: '15 февраля 2026',
    status: 'Доставлен',
    statusColor: 'text-graphite bg-graphite/5 border border-graphite/10',
    total: 18900,
    delivery: {
      address: 'Санкт-Петербург, Невский пр. 45, кв. 12',
      service: 'Пункт выдачи Boxberry'
    },
    items: [
      { id: 'p12', name: 'Тренч с деконструкцией', size: 'M', price: 28900, image: 'https://wsrv.nl/?url=images.unsplash.com/photo-1585487000143-6519d58aef14&w=800&output=webp' }
    ]
  },
  {
    id: 'ZMK-05441',
    date: '10 января 2026',
    status: 'Доставлен',
    statusColor: 'text-graphite bg-graphite/5 border border-graphite/10',
    total: 45000,
    delivery: {
      address: 'Москва, Пятницкая 10',
      service: 'Курьер ZAMK VIP'
    },
    items: [
      { id: 'p1', name: 'Анорак ледяной линии', size: 'L', price: 14900, image: 'https://wsrv.nl/?url=images.unsplash.com/photo-1591047139829-d91aecb6caea&w=800&output=webp' },
      { id: 'p5', name: 'Кроссовки аэр-сетки', size: '42', price: 21500, image: 'https://wsrv.nl/?url=images.unsplash.com/photo-1542291026-7eec264c27ff&w=800&output=webp' }
    ]
  }
];

export function Orders() {
  const { isAuthenticated } = useAuth();
  const [orders, setOrders] = useState<OrderRecord[]>(() => getOrdersWithDefaults(MOCK_ORDERS));
  const [selectedOrder, setSelectedOrder] = useState<OrderRecord | null>(null);

  useEffect(() => {
    const syncOrders = () => setOrders(getOrdersWithDefaults(MOCK_ORDERS));
    window.addEventListener(ORDER_STORAGE_EVENT, syncOrders);
    return () => window.removeEventListener(ORDER_STORAGE_EVENT, syncOrders);
  }, []);
  
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
        {orders.length > 0 ? (
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
                    {order.items.map((item) => {
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
                  {selectedOrder.items.map((item, idx) => (
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

            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}