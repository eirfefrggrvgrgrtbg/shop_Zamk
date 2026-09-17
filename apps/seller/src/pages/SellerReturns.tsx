import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getSellerReturns } from '@zamk/api-client/src/seller';
import type { SellerReturn } from '@zamk/api-client/src/types';
import { adaptReturns } from '../api/sellerOperations';
import { Package, AlertCircle } from 'lucide-react';
import { RETURN_REASON_LABELS } from './SellerReturnDetail';
import { SellerPageFrame, SellerPageHeader } from '../components/SellerPageFrame';
import { SellerSurface, SellerTableShell } from '../components/SellerSurface';
import { cn } from '../lib/utils';

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  requested: { label: 'Запрошен', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
  needs_info: { label: 'Требуется информация', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
  approved: { label: 'Одобрен', color: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40' },
  rejected: { label: 'Отклонён', color: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900/40' },
  item_received: { label: 'Получен', color: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-950/30 dark:text-indigo-400 dark:border-indigo-900/40' },
  completed: { label: 'Завершён', color: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-400 dark:border-emerald-900/40' },
  refunded: { label: 'Возмещён', color: 'bg-gray-100 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/80 dark:border-white/10' },
  cancelled: { label: 'Отменён', color: 'bg-gray-100 text-gray-600 border-gray-200 dark:bg-white/10 dark:text-white/60 dark:border-white/10' },
};

const OUTCOME_LABELS: Record<string, { label: string; color: string }> = {
  restocked: { label: 'Возвращён в продажу', color: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/30 dark:text-emerald-400 dark:border-emerald-900/40' },
  damaged: { label: 'Повреждён (брак)', color: 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/30 dark:text-rose-400 dark:border-rose-900/40' },
  rejected: { label: 'Отклонён складом', color: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900/40' },
  partial_restock: { label: 'Частично в продажу', color: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40' },
  in_inspection: { label: 'Проверяется', color: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-950/30 dark:text-indigo-400 dark:border-indigo-900/40' },
  arrived_at_zamk: { label: 'На складе ZAMK', color: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40' },
  in_transit: { label: 'В пути на склад', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
  awaiting_handover: { label: 'Ожидает передачи', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
  awaiting_shipment: { label: 'Ожидает отправки', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
  awaiting_arrival: { label: 'В пути на склад', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/30 dark:text-amber-400 dark:border-amber-900/40' },
  needs_info: { label: 'Требуется информация', color: 'bg-yellow-50 text-yellow-800 border-yellow-200 dark:bg-yellow-950/30 dark:text-yellow-400 dark:border-yellow-900/40' },
  requested: { label: 'На рассмотрении', color: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/30 dark:text-blue-400 dark:border-blue-900/40' },
  not_received: { label: 'Не поступил', color: 'bg-gray-50 text-gray-700 border-gray-200 dark:bg-white/10 dark:text-white/70 dark:border-white/10' },
  rejected_by_support: { label: 'Отклонён поддержкой', color: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/30 dark:text-red-400 dark:border-red-900/40' },
  cancelled: { label: 'Отменён', color: 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-white/10 dark:text-white/60 dark:border-white/10' },
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
      <SellerPageFrame variant="summary">
        <div className="flex justify-center flex-col items-center py-20">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-black mb-4"></div>
          <div className="text-gray-500">Загружаем возвраты...</div>
        </div>
      </SellerPageFrame>
    );
  }

  if (error) {
    return (
      <SellerPageFrame variant="summary">
        <div className="flex justify-center py-20">
          <div className="bg-red-50 text-red-600 p-6 rounded-xl flex items-center gap-3 border border-red-200 dark:bg-red-950/20 dark:border-red-900/30 dark:text-red-300">
            <AlertCircle className="w-6 h-6" />
            <span>{error}</span>
          </div>
        </div>
      </SellerPageFrame>
    );
  }

  return (
    <SellerPageFrame variant="summary">
      <SellerPageHeader
        eyebrow="Продажи"
        title="Возвраты"
        description="Возвраты покупателей по вашим товарам. Логистикой и проверкой занимается ZAMK (только для чтения)."
      />

      {returns.length === 0 ? (
        <SellerSurface className="py-12 text-center text-gray-500 dark:text-gray-400">
          У вас пока нет возвратов.
        </SellerSurface>
      ) : (
        <SellerTableShell>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="bg-gray-50/60 dark:bg-white/[0.02] border-b border-gray-200 dark:border-white/10 text-xs font-semibold uppercase tracking-wider text-gray-500">
                  <th className="px-4 py-3.5">Товар</th>
                  <th className="px-4 py-3.5">Заказ</th>
                  <th className="px-4 py-3.5">Дата возврата</th>
                  <th className="px-4 py-3.5">Статус</th>
                  <th className="px-4 py-3.5 text-right">Финансы</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                {returns.map((ret) => {
                  const statusConfig = STATUS_LABELS[ret.status] || { label: ret.status, color: 'bg-gray-100 text-gray-800 border-gray-200' };
                  const shortOrderId = ret.orderNumber || ret.orderId.split('-')[0];
                  
                  return (
                    <tr 
                      key={ret.returnItemId} 
                      onClick={() => navigate(`/returns/${ret.returnId}`)}
                      className="hover:bg-gray-50/60 dark:hover:bg-white/[0.02] transition-colors cursor-pointer"
                    >
                      <td className="px-4 py-3.5">
                        <div className="flex items-center gap-3">
                          <div className="w-11 h-11 rounded-lg bg-gray-100 dark:bg-white/5 border border-gray-200/60 dark:border-white/10 overflow-hidden shrink-0">
                            {ret.imageUrl ? (
                              <img src={ret.imageUrl} alt={ret.productTitle} className="w-full h-full object-cover" />
                            ) : (
                              <div className="w-full h-full flex items-center justify-center text-gray-400">
                                <Package className="w-5 h-5 opacity-20" />
                              </div>
                            )}
                          </div>
                          <div>
                            <div className="font-medium text-sm text-gray-900 dark:text-white line-clamp-1" title={ret.productTitle}>
                              {ret.productTitle}
                            </div>
                            <div className="text-xs text-gray-500 dark:text-gray-400 flex items-center gap-2 mt-0.5">
                              {ret.sku && <span className="font-mono bg-gray-100 dark:bg-white/10 px-1.5 py-0.5 rounded">{ret.sku}</span>}
                              <span>{ret.quantity} шт.</span>
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3.5 text-sm text-gray-900 dark:text-white font-medium">
                        #{shortOrderId}
                      </td>
                      <td className="px-4 py-3.5 text-sm text-gray-500 dark:text-gray-400">
                        {new Date(ret.createdAt).toLocaleDateString('ru-RU')}
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="flex flex-col gap-1 items-start">
                          <span className={cn('inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium', statusConfig.color)}>
                            {statusConfig.label}
                          </span>
                          {ret.physicalOutcome && OUTCOME_LABELS[ret.physicalOutcome] && (
                            <span className={cn('inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-medium', OUTCOME_LABELS[ret.physicalOutcome].color)}>
                              {OUTCOME_LABELS[ret.physicalOutcome].label}
                            </span>
                          )}
                          {ret.reason && (
                            <span className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 italic max-w-[200px] truncate" title={RETURN_REASON_LABELS[ret.reason] || ret.reason}>
                              Причина: {RETURN_REASON_LABELS[ret.reason] || ret.reason}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3.5 font-semibold text-gray-900 dark:text-white text-right text-sm">
                        {ret.financialAdjustment ? (
                          <div className="flex flex-col items-end gap-0.5">
                            <span className="text-red-600 dark:text-red-400 font-medium">
                              −{currencyFormatter.format(ret.financialAdjustment.deductionCents / 100)}
                            </span>
                            <span className="text-[10px] text-gray-500 font-normal">Отмена дохода</span>
                            {ret.compensation?.status === 'credited' && (
                              <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300 border border-emerald-200/60 dark:border-emerald-800/40 mt-0.5">
                                Компенсация ZAMK
                              </span>
                            )}
                            {ret.compensation?.status === 'pending' && (
                              <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300 border border-amber-200/60 dark:border-amber-800/40 mt-0.5">
                                Ответственность определяется
                              </span>
                            )}
                          </div>
                        ) : (
                          <span className="text-gray-400 font-normal">—</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </SellerTableShell>
      )}
    </SellerPageFrame>
  );
}
