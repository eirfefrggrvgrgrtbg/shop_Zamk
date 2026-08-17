import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getSellerReturn } from '@zamk/api-client/src/seller';
import type { SellerReturn } from '@zamk/api-client/src/types';
import { Package, ArrowLeft, AlertCircle, Clock, CheckCircle2, XCircle } from 'lucide-react';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string; dot: string }> = {
  requested: { label: 'Возврат оформлен', color: 'text-yellow-800 bg-yellow-100', dot: 'bg-yellow-500' },
  approved: { label: 'В пути в ZAMK', color: 'text-blue-800 bg-blue-100', dot: 'bg-blue-500' },
  rejected: { label: 'Отклонён', color: 'text-red-800 bg-red-100', dot: 'bg-red-500' },
  item_received: { label: 'Получен ZAMK', color: 'text-indigo-800 bg-indigo-100', dot: 'bg-indigo-500' },
  completed: { label: 'Возврат завершён', color: 'text-emerald-800 bg-emerald-100', dot: 'bg-emerald-500' },
  refunded: { label: 'Возврат завершён', color: 'text-emerald-800 bg-emerald-100', dot: 'bg-emerald-500' },
  cancelled: { label: 'Отменён', color: 'text-gray-600 bg-gray-100', dot: 'bg-gray-500' },
};

export function SellerReturnDetail() {
  const { id } = useParams<{ id: string }>();
  const [returnItems, setReturnItems] = useState<SellerReturn[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function fetchReturn() {
      if (!id) return;
      try {
        const data = await getSellerReturn(id);
        setReturnItems((data as any).items || (Array.isArray(data) ? data : []));
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки возврата');
      } finally {
        setIsLoading(false);
      }
    }
    fetchReturn();
  }, [id]);

  if (isLoading) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center flex-col items-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black mb-4"></div>
        <div className="text-ash">Загрузка данных возврата...</div>
      </div>
    );
  }

  if (error || returnItems.length === 0) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center">
        <div className="bg-red-50 text-red-600 p-6 rounded-2xl flex items-center gap-3">
          <AlertCircle className="w-6 h-6" />
          <span>{error || 'Возврат не найден'}</span>
        </div>
      </div>
    );
  }

  const ret = returnItems[0]; // For Seller, we display per return group or just the first item info since order is shared
  const statusConfig = STATUS_LABELS[ret.status] || { label: ret.status, color: 'text-gray-800 bg-gray-100', dot: 'bg-gray-500' };

  return (
    <div className="p-8 max-w-4xl mx-auto">
      <Link to="/returns" className="inline-flex items-center text-sm font-medium text-graphite-light hover:text-graphite transition-colors mb-6">
        <ArrowLeft className="w-4 h-4 mr-1" /> К списку возвратов
      </Link>

      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
        <div>
          <h1 className="text-2xl font-bold text-graphite dark:text-white flex items-center gap-3">
            Возврат по заказу #{ret.orderNumber || ret.orderId.split('-')[0]}
            <span className={cn("px-3 py-1 rounded-full text-sm font-semibold border", statusConfig.color)}>
              {statusConfig.label}
            </span>
          </h1>
          <div className="text-sm text-ash mt-2">
            Создан {new Date(ret.createdAt).toLocaleString('ru-RU')}
          </div>
        </div>
      </div>

      <div className="grid gap-6">
        {returnItems.map(item => (
          <div key={item.returnItemId} className="bg-white dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-2xl overflow-hidden shadow-sm">
            
            {/* Header / Item Info */}
            <div className="p-6 border-b border-border-soft dark:border-white/10 flex flex-col sm:flex-row gap-6">
              <div className="w-24 h-32 bg-gray-100 rounded-xl overflow-hidden shrink-0 border border-gray-200">
                {item.imageUrl ? (
                  <img src={item.imageUrl} alt={item.productTitle} className="w-full h-full object-cover" />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-gray-400">
                    <Package className="w-8 h-8 opacity-20" />
                  </div>
                )}
              </div>
              <div className="flex-1 flex flex-col justify-between">
                <div>
                  <h3 className="text-lg font-bold text-graphite dark:text-white">{item.productTitle}</h3>
                  <div className="text-sm text-graphite-light dark:text-white/60 mt-2 space-y-1">
                    {item.variantSize && <p>Размер: <span className="font-medium text-graphite dark:text-white">{item.variantSize}</span></p>}
                    {item.variantColor && <p>Цвет: <span className="font-medium text-graphite dark:text-white">{item.variantColor}</span></p>}
                    {item.sku && <p>SKU: <span className="font-mono text-xs bg-gray-100 dark:bg-white/10 px-1 rounded">{item.sku}</span></p>}
                    <p>Количество: <span className="font-medium text-graphite dark:text-white">{item.quantity} шт.</span></p>
                  </div>
                </div>
                <div className="mt-4 pt-4 border-t border-dashed border-gray-200 dark:border-white/10">
                  <div className="text-sm">
                    <span className="text-graphite-light">Историческая стоимость:</span>{' '}
                    <span className="font-semibold text-graphite dark:text-white">{currencyFormatter.format(item.subtotalPriceCents / 100)}</span>
                  </div>
                </div>
              </div>
            </div>

            <div className="p-6 bg-gray-50/50 dark:bg-transparent grid grid-cols-1 md:grid-cols-2 gap-8">
              {/* Reason */}
              <div>
                <h4 className="text-xs font-bold uppercase tracking-wider text-ash mb-3">Причина возврата</h4>
                <div className="bg-white dark:bg-white/5 border border-border-soft dark:border-white/10 p-4 rounded-xl text-sm text-graphite dark:text-white/90 shadow-sm">
                  <p className="font-medium mb-1">{item.reason || 'Не указана'}</p>
                  {item.condition && <p className="text-graphite-light mt-2 italic text-xs">Комментарий: {item.condition}</p>}
                </div>
              </div>

              {/* Disposition & Finance */}
              <div className="space-y-6">
                <div>
                  <h4 className="text-xs font-bold uppercase tracking-wider text-ash mb-3">Решение ZAMK (Склад)</h4>
                  <div className="flex items-start gap-3">
                    {item.status === 'requested' || item.status === 'approved' ? (
                      <>
                        <Clock className="w-5 h-5 text-yellow-500 shrink-0 mt-0.5" />
                        <div className="text-sm">
                          <p className="font-medium text-graphite dark:text-white">Ожидается поступление на склад</p>
                          <p className="text-graphite-light mt-1">Товар еще не проверен специалистами ZAMK.</p>
                        </div>
                      </>
                    ) : item.status === 'item_received' && !item.restock ? (
                      <>
                        <Clock className="w-5 h-5 text-indigo-500 shrink-0 mt-0.5" />
                        <div className="text-sm">
                          <p className="font-medium text-graphite dark:text-white">Проверяется</p>
                          <p className="text-graphite-light mt-1">Товар получен, ожидается решение.</p>
                        </div>
                      </>
                    ) : item.restock ? (
                      <>
                        <CheckCircle2 className="w-5 h-5 text-emerald-500 shrink-0 mt-0.5" />
                        <div className="text-sm">
                          <p className="font-medium text-graphite dark:text-white">Пригоден к продаже</p>
                          <p className="text-graphite-light mt-1">{item.quantity} ед. возвращена в доступный остаток ZAMK</p>
                        </div>
                      </>
                    ) : (
                      <>
                        <XCircle className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                        <div className="text-sm">
                          <p className="font-medium text-graphite dark:text-white">Не пригоден к продаже</p>
                          <p className="text-graphite-light mt-1">{item.quantity} ед. признана непригодной для продажи</p>
                          {item.adminComment && <p className="mt-2 text-xs italic bg-white border border-red-100 p-2 rounded text-red-800">"{item.adminComment}"</p>}
                        </div>
                      </>
                    )}
                  </div>
                </div>

                <div>
                  <h4 className="text-xs font-bold uppercase tracking-wider text-ash mb-3">Финансовый результат</h4>
                  {item.financialAdjustmentCents != null ? (
                    <div className="flex items-start gap-3 bg-red-50/50 p-3 rounded-xl border border-red-100">
                      <div className="text-sm">
                        <p className="font-bold text-red-600">
                          {currencyFormatter.format(item.financialAdjustmentCents / 100)}
                        </p>
                        <p className="text-red-800/80 mt-1 text-xs font-medium">
                          {item.financialImpactType === 'frozen' && 'Из замороженных средств'}
                          {item.financialImpactType === 'available' && 'Из доступного баланса'}
                          {item.financialImpactType === 'debt' && 'Сумма будет удержана из будущих выплат.'}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div className="text-sm text-graphite-light italic">
                      Удержание пока не сформировано
                    </div>
                  )}
                </div>
              </div>
            </div>
            
            {/* Timeline */}
            <div className="bg-gray-100/50 dark:bg-white/5 border-t border-border-soft dark:border-white/10 px-6 py-4">
              <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center text-xs text-graphite-light">
                <span>Последнее обновление: {new Date(item.updatedAt).toLocaleString('ru-RU')}</span>
                <span>Возврат № {item.returnId.split('-')[0]}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
