import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { RotateCcw } from 'lucide-react';
import { AccountLayout } from '../components/account/AccountLayout';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';

export function Returns() {
  return (
    <AccountLayout title="Мои возвраты">
      <ReturnsContent />
    </AccountLayout>
  );
}

function ReturnsContent() {
  const [returns, setReturns] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);

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

  useEffect(() => {
    const loadReturns = async () => {
      try {
        setIsLoading(true);
        const { getCustomerReturns } = await import('@zamk/api-client/src/customer');
        const data = await getCustomerReturns();
        const items = data?.items || [];
        
        setReturns(items.map((r: any) => {
          const rData = r.return || r;
          return {
            id: rData.id.split('-')[0].toUpperCase(),
            rawId: rData.id,
            date: new Date(rData.createdAt).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }),
            status: mapReturnStatus(rData.status),
            reason: rData.reason || 'Не указана',
            items: rData.items || []
          };
        }));
      } catch (err) {
        console.error('Failed to load returns', err);
      } finally {
        setIsLoading(false);
      }
    };
    loadReturns();
  }, []);

  if (isLoading) {
    return (
      <div className="py-20 flex justify-center">
        <div className="animate-spin w-8 h-8 border-2 border-black border-t-transparent rounded-full dark:border-white dark:border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      {returns.length > 0 ? (
        returns.map((ret) => (
          <div 
            key={ret.rawId} 
            className="group bg-white/50 hover:bg-white/80 dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-white/60 dark:border-white/10 shadow-[0_8px_30px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] rounded-[1.5rem] p-6 md:p-8 transition-all duration-500"
          >
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-5 sm:gap-0 border-b border-graphite/5 dark:border-white/10 pb-6 mb-6">
              <div>
                <div className="flex items-center gap-4 mb-2">
                  <span className={`text-[11px] uppercase tracking-wider font-medium px-3 py-1 rounded-full text-graphite dark:text-white/70 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20`}>
                    {ret.status}
                  </span>
                  <span className="text-[13.5px] text-ash dark:text-white/60 font-medium tracking-wide">{ret.date}</span>
                </div>
                <p className="text-[14px] font-medium text-graphite dark:text-white mt-1">Причина: {ret.reason}</p>
              </div>
              <div className="sm:text-right">
                <span className="text-sm text-ash dark:text-white/60">№ {ret.id}</span>
              </div>
            </div>
            
            <div className="flex flex-col gap-4">
              {ret.items.length > 0 ? (
                ret.items.map((item: any, idx: number) => (
                  <div key={idx} className="flex gap-4">
                    <div className="w-16 h-20 bg-graphite/5 dark:bg-white/10 rounded-lg overflow-hidden flex-shrink-0">
                      <img 
                        src={item.imageUrl || PRODUCT_PLACEHOLDER_IMAGE} 
                        alt="Товар" 
                        className="w-full h-full object-cover object-center" 
                      />
                    </div>
                    <div className="flex-1 py-1">
                      <p className="text-[14px] font-medium text-graphite dark:text-white leading-snug">
                        {item.productTitle || 'Неизвестный товар'} {(item.quantity ?? 1) > 1 ? `× ${item.quantity}` : ''}
                      </p>
                    </div>
                  </div>
                ))
              ) : (
                <p className="text-[14px] text-graphite/70 dark:text-white/70 leading-relaxed font-medium">
                  Информация о товарах недоступна
                </p>
              )}
            </div>
          </div>
        ))
      ) : (
        <div className="bg-white/50 hover:bg-white/80 dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-white/60 dark:border-white/10 shadow-[0_8px_30px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] rounded-[1.5rem] p-8 md:p-10 text-center">
          <RotateCcw className="w-12 h-12 text-ash-light dark:text-white/50 mx-auto mb-4" />
          <h2 className="text-xl font-medium text-graphite dark:text-white mb-2">У вас пока нет возвратов</h2>
          <p className="text-sm text-ash dark:text-white/60 mb-6">Оформить возврат можно на странице заказа после его получения.</p>
          <Link to="/orders" className="inline-flex items-center justify-center rounded-full border border-graphite/20 dark:border-white/20 px-5 py-3 text-[13px] font-medium text-graphite dark:text-white hover:border-graphite dark:hover:border-white transition-colors">
            К заказам
          </Link>
        </div>
      )}
    </div>
  );
}
