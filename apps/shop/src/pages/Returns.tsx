import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { RotateCcw, ArrowRight } from 'lucide-react';
import { AccountLayout } from '../components/account/AccountLayout';
import { PRODUCT_PLACEHOLDER_IMAGE } from '../api/publicCatalog';
import { getCustomerReturns } from '@zamk/api-client/src/customer';
import {
  type CustomerReturnRecord,
  formatCustomerReturnStatus,
  formatReturnReason,
  formatReturnShipmentStatus,
} from '@zamk/api-client/src/types';

export function Returns() {
  return (
    <AccountLayout title="Мои возвраты">
      <ReturnsContent />
    </AccountLayout>
  );
}

function ReturnsContent() {
  const [returns, setReturns] = useState<CustomerReturnRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const loadReturns = async () => {
      try {
        setIsLoading(true);
        const data = await getCustomerReturns();
        setReturns(data?.items || []);
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
        returns.map((ret) => {
          const dateStr = new Date(ret.createdAt).toLocaleDateString('ru-RU', {
            day: 'numeric',
            month: 'long',
            year: 'numeric',
          });
          const orderLabel = ret.orderNumber ? `Заказ ${ret.orderNumber}` : 'Заказ';

          return (
            <div
              key={ret.id}
              className="group bg-white/50 hover:bg-white/80 dark:bg-white/5 dark:hover:bg-white/10 backdrop-blur-xl border border-white/60 dark:border-white/10 shadow-[0_8px_30px_rgba(100,130,170,0.06)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.3)] rounded-[1.5rem] p-6 md:p-8 transition-all duration-500"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-graphite/5 dark:border-white/10 pb-5 mb-5">
                <div>
                  <div className="flex flex-wrap items-center gap-3 mb-1.5">
                    <span className="text-[11px] font-medium px-3 py-1 rounded-full text-graphite dark:text-white/70 bg-graphite/5 dark:bg-white/10 border border-graphite/10 dark:border-white/20">
                      {formatCustomerReturnStatus(ret.status)}
                    </span>
                    {ret.shipment?.status && (
                      <span className="text-[11px] font-medium px-2.5 py-0.5 rounded-full text-ash dark:text-white/60 bg-graphite/5 dark:bg-white/5">
                        {formatReturnShipmentStatus(ret.shipment.status)}
                      </span>
                    )}
                    <span className="text-[13px] text-ash dark:text-white/60 font-medium tracking-wide">
                      {dateStr}
                    </span>
                  </div>
                  <p className="text-[13.5px] font-medium text-graphite dark:text-white">
                    Причина: {formatReturnReason(ret.reason)}
                  </p>
                </div>
                <div className="sm:text-right">
                  <span className="text-sm font-medium text-graphite dark:text-white">
                    {orderLabel}
                  </span>
                </div>
              </div>

              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div className="flex flex-col gap-3 flex-1">
                  {ret.items && ret.items.length > 0 ? (
                    ret.items.map((item, idx) => {
                      const variantDetails = [item.variantSize, item.variantColor].filter(Boolean).join(' · ');
                      return (
                        <div key={item.id || idx} className="flex gap-4">
                          <div className="w-14 h-16 bg-graphite/5 dark:bg-white/10 rounded-lg overflow-hidden flex-shrink-0">
                            <img
                              src={item.productImageUrl || PRODUCT_PLACEHOLDER_IMAGE}
                              alt={item.productTitle || 'Товар'}
                              className="w-full h-full object-cover object-center"
                            />
                          </div>
                          <div className="flex-1 py-0.5">
                            <p className="text-[13.5px] font-medium text-graphite dark:text-white leading-snug">
                              {item.productTitle || 'Неизвестный товар'}
                            </p>
                            {variantDetails && (
                              <p className="text-[12px] text-ash dark:text-white/60 mt-0.5">
                                {variantDetails}
                              </p>
                            )}
                            <p className="text-[12px] text-ash dark:text-white/60 mt-0.5">
                              {item.quantity ?? 1} шт.
                            </p>
                          </div>
                        </div>
                      );
                    })
                  ) : (
                    <p className="text-[13px] text-graphite/70 dark:text-white/70 font-medium">
                      Информация о товарах недоступна
                    </p>
                  )}
                </div>

                <div className="pt-2 sm:pt-0 flex justify-end">
                  <Link
                    to={`/returns/${ret.id}`}
                    className="inline-flex items-center gap-2 px-4 py-2 rounded-full border border-graphite/20 dark:border-white/20 text-xs font-medium text-graphite dark:text-white hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
                  >
                    <span>Открыть возврат</span>
                    <ArrowRight className="w-3.5 h-3.5" />
                  </Link>
                </div>
              </div>
            </div>
          );
        })
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
