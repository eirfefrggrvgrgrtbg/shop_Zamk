import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getSellerReturns } from '@zamk/api-client/src/seller';
import type { SellerReturn } from '@zamk/api-client/src/types';
import { adaptReturns } from '../api/sellerOperations';
import { Package, AlertCircle } from 'lucide-react';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  requested: { label: 'Запрошен', color: 'bg-yellow-100 text-yellow-800' },
  approved: { label: 'Одобрен', color: 'bg-blue-100 text-blue-800' },
  rejected: { label: 'Отклонён', color: 'bg-red-100 text-red-800' },
  item_received: { label: 'Получен', color: 'bg-indigo-100 text-indigo-800' },
  completed: { label: 'Завершён', color: 'bg-emerald-100 text-emerald-800' },
  refunded: { label: 'Возмещён', color: 'bg-gray-100 text-gray-800' },
  cancelled: { label: 'Отменён', color: 'bg-gray-100 text-gray-600' },
};

export function SellerReturns() {
  const [returns, setReturns] = useState<SellerReturn[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    async function fetchReturns() {
      try {
        const data = await getSellerReturns();
        setReturns(adaptReturns(data));
      } catch (err: any) {
        setError(err.message || 'Ошибка загрузки возвратов');
      } finally {
        setIsLoading(false);
      }
    }
    fetchReturns();
  }, []);

  if (isLoading) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center flex-col items-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black mb-4"></div>
        <div className="text-ash">Загружаем возвраты...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen pt-24 pb-24 flex justify-center">
        <div className="bg-red-50 text-red-600 p-6 rounded-2xl flex items-center gap-3">
          <AlertCircle className="w-6 h-6" />
          <span>{error}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-6xl mx-auto">
      <h1 className="text-2xl font-bold text-graphite dark:text-white mb-2">Возвраты</h1>
      <p className="text-ash mb-8">
        Возвраты покупателей по вашим товарам. Логистикой и проверкой занимается ZAMK (только для чтения).
      </p>

      {returns.length === 0 ? (
        <div className="py-12 text-center text-ash bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10">
          У вас пока нет возвратов.
        </div>
      ) : (
        <div className="bg-white dark:bg-white/5 rounded-2xl border border-border-soft dark:border-white/10 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-border-soft dark:border-white/10 text-sm text-ash">
                  <th className="p-4 font-medium">Товар</th>
                  <th className="p-4 font-medium">Заказ</th>
                  <th className="p-4 font-medium">Дата возврата</th>
                  <th className="p-4 font-medium">Статус</th>
                  <th className="p-4 font-medium text-right">Сумма возврата</th>
                </tr>
              </thead>
              <tbody>
                {returns.map((ret) => {
                  const statusConfig = STATUS_LABELS[ret.status] || { label: ret.status, color: 'bg-gray-100 text-gray-800' };
                  const shortOrderId = ret.orderNumber || ret.orderId.split('-')[0];
                  
                  return (
                    <tr 
                      key={ret.returnItemId} 
                      onClick={() => navigate(`/returns/${ret.returnId}`)}
                      className="border-b border-border-soft dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors cursor-pointer"
                    >
                      <td className="p-4">
                        <div className="flex items-center gap-3">
                          <div className="w-12 h-12 rounded-lg bg-gray-100 overflow-hidden shrink-0">
                            {ret.imageUrl ? (
                              <img src={ret.imageUrl} alt={ret.productTitle} className="w-full h-full object-cover" />
                            ) : (
                              <div className="w-full h-full flex items-center justify-center text-gray-400">
                                <Package className="w-6 h-6 opacity-20" />
                              </div>
                            )}
                          </div>
                          <div>
                            <div className="font-medium text-graphite dark:text-white line-clamp-1" title={ret.productTitle}>
                              {ret.productTitle}
                            </div>
                            <div className="text-sm text-ash flex items-center gap-2 mt-1">
                              {ret.sku && <span className="font-mono text-xs bg-gray-100 dark:bg-white/10 px-1 rounded">{ret.sku}</span>}
                              <span>{ret.quantity} шт.</span>
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="p-4 text-sm text-graphite dark:text-white font-medium">
                        #{shortOrderId}
                      </td>
                      <td className="p-4 text-sm text-ash">
                        {new Date(ret.createdAt).toLocaleDateString('ru-RU')}
                      </td>
                      <td className="p-4">
                        <div className="flex flex-col gap-1 items-start">
                          <span className={`px-2 py-0.5 text-xs font-semibold rounded-full ${statusConfig.color}`}>
                            {statusConfig.label}
                          </span>
                          {ret.reason && (
                            <span className="text-xs text-ash mt-1 italic max-w-[200px] truncate" title={ret.reason}>
                              Причина: {ret.reason}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="p-4 font-semibold text-graphite dark:text-white text-right">
                        {currencyFormatter.format(ret.subtotalPriceCents / 100)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
