import { Check, CircleDot, AlertCircle } from 'lucide-react';
import { formatCustomerReturnStatus, formatReturnShipmentStatus } from '@zamk/api-client/src/types';

export interface ReturnProgressStep {
  key: string;
  label: string;
  description?: string;
}

export const RETURN_STAGES: ReturnProgressStep[] = [
  { key: 'requested', label: 'Заявка', description: 'Заявка отправлена' },
  { key: 'approved', label: 'Одобрено', description: 'Возврат согласован' },
  { key: 'shipping', label: 'Отправка', description: 'Передача в доставку' },
  { key: 'in_transit', label: 'В пути', description: 'Доставка в ZAMK' },
  { key: 'receiving', label: 'Приёмка', description: 'Проверка товара' },
  { key: 'refunded', label: 'Возврат денег', description: 'Выплата покупателю' },
];

export function getReturnProgressIndex(returnStatus: string, shipmentStatus?: string): number {
  if (returnStatus === 'requested') {
    return 0;
  }
  if (returnStatus === 'approved') {
    if (!shipmentStatus || shipmentStatus === 'draft') {
      return 1;
    }
    if (shipmentStatus === 'awaiting_handover') {
      return 2;
    }
    if (shipmentStatus === 'handed_over' || shipmentStatus === 'in_transit') {
      return 3;
    }
    if (shipmentStatus === 'arrived_at_zamk') {
      return 4;
    }
    return 1;
  }
  if (returnStatus === 'receiving' || returnStatus === 'item_received') {
    return 4;
  }
  if (returnStatus === 'refunded' || returnStatus === 'completed') {
    return 5;
  }
  return 0;
}

export function ReturnLifecycleProgress({
  returnStatus,
  shipmentStatus,
  adminComment,
}: {
  returnStatus: string;
  shipmentStatus?: string;
  adminComment?: string | null;
}) {
  const isTerminalNegative = returnStatus === 'rejected' || returnStatus === 'cancelled';

  if (isTerminalNegative) {
    const isRejected = returnStatus === 'rejected';
    return (
      <div className="p-5 rounded-2xl bg-red-50/80 dark:bg-red-950/30 border border-red-200/80 dark:border-red-900/40">
        <div className="flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-red-600 dark:text-red-400 flex-shrink-0 mt-0.5" />
          <div>
            <h4 className="text-sm font-semibold text-red-900 dark:text-red-200">
              {isRejected ? 'Возврат отклонён' : 'Возврат отменён'}
            </h4>
            {adminComment && (
              <p className="text-sm text-red-700 dark:text-red-300 mt-1">
                Причина отклонения: {adminComment}
              </p>
            )}
          </div>
        </div>
      </div>
    );
  }

  const currentIndex = getReturnProgressIndex(returnStatus, shipmentStatus);

  return (
    <div className="p-5 md:p-6 rounded-[1.25rem] bg-white/80 dark:bg-white/10 backdrop-blur-xl border border-graphite/10 dark:border-white/15 shadow-sm font-sans">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
        <span className="text-xs uppercase tracking-wider font-semibold text-graphite/70 dark:text-white/70">
          Этап возврата
        </span>
        <span className="text-xs md:text-sm font-medium text-graphite dark:text-white">
          Текущий статус: <span className="font-semibold">{formatCustomerReturnStatus(returnStatus)}</span>
          {shipmentStatus && <span className="text-graphite/70 dark:text-white/70"> ({formatReturnShipmentStatus(shipmentStatus)})</span>}
        </span>
      </div>

      <div className="relative">
        {/* Progress Bar Track */}
        <div className="hidden md:block absolute top-4 left-6 right-6 h-0.5 bg-graphite/10 dark:bg-white/15 -z-0" />
        <div
          className="hidden md:block absolute top-4 left-6 h-0.5 bg-black dark:bg-white transition-all duration-500 -z-0"
          style={{ width: `calc(${(currentIndex / (RETURN_STAGES.length - 1)) * 100}% - 24px)` }}
        />

        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3 sm:gap-4 relative z-10">
          {RETURN_STAGES.map((stage, idx) => {
            const isCompleted = idx < currentIndex;
            const isCurrent = idx === currentIndex;

            return (
              <div key={stage.key} className="flex flex-col items-center text-center">
                <div
                  className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-semibold mb-1.5 transition-all duration-300 ${
                    isCompleted
                      ? 'bg-black text-white dark:bg-white dark:text-black shadow-sm'
                      : isCurrent
                      ? 'bg-black text-white dark:bg-white dark:text-black ring-4 ring-black/10 dark:ring-white/20 font-bold'
                      : 'bg-graphite/5 dark:bg-white/5 text-graphite/60 dark:text-white/60 border border-graphite/20 dark:border-white/20'
                  }`}
                >
                  {isCompleted ? (
                    <Check className="w-4 h-4" />
                  ) : isCurrent ? (
                    <CircleDot className="w-4 h-4 animate-pulse" />
                  ) : (
                    <span>{idx + 1}</span>
                  )}
                </div>
                <span
                  className={`text-xs ${
                    isCurrent
                      ? 'text-graphite dark:text-white font-bold'
                      : isCompleted
                      ? 'text-graphite/85 dark:text-white/85 font-medium'
                      : 'text-graphite/55 dark:text-white/55 font-medium'
                  }`}
                >
                  {stage.label}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
