import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getSellerReturn } from '@zamk/api-client/src/seller';
import type { SellerReturn } from '@zamk/api-client/src/types';
import {
  Package,
  ArrowLeft,
  AlertCircle,
  Clock,
  CheckCircle2,
  XCircle,
  Truck,
  ShieldCheck,
  QrCode,
  Check,
  AlertTriangle,
} from 'lucide-react';
import { cn } from '../lib/utils';

export type ReturnPresentationMode =
  | 'PRE_PHYSICAL'
  | 'LOGISTICS'
  | 'WAREHOUSE_PROCESSING'
  | 'FINAL_OUTCOME';

export function getReturnPresentationMode(item: {
  status: string;
  physicalOutcome: string;
  inspectionCompleted?: boolean;
}): ReturnPresentationMode {
  // If warehouse inspection is authoritatively completed, it is a FINAL_OUTCOME.
  if (item.inspectionCompleted) {
    return 'FINAL_OUTCOME';
  }

  const outcome = item.physicalOutcome;
  const status = item.status;

  // Final disposition outcomes (when explicitly recorded as completed)
  if (
    outcome === 'restocked' ||
    outcome === 'damaged' ||
    outcome === 'rejected' ||
    outcome === 'not_received' ||
    outcome === 'partial_restock' ||
    status === 'item_received' ||
    status === 'completed' ||
    status === 'refunded'
  ) {
    return 'FINAL_OUTCOME';
  }

  // Active warehouse receiving/inspection
  if (outcome === 'in_inspection' || status === 'receiving') {
    return 'WAREHOUSE_PROCESSING';
  }

  // Active return logistics before warehouse receiving
  if (
    outcome === 'awaiting_shipment' ||
    outcome === 'awaiting_handover' ||
    outcome === 'in_transit' ||
    outcome === 'arrived_at_zamk' ||
    outcome === 'awaiting_arrival' ||
    status === 'approved'
  ) {
    return 'LOGISTICS';
  }

  // Pre-physical customer/support policy flow
  if (
    outcome === 'requested' ||
    outcome === 'needs_info' ||
    outcome === 'rejected_by_support' ||
    outcome === 'cancelled' ||
    status === 'requested' ||
    status === 'needs_info' ||
    status === 'rejected' ||
    status === 'cancelled'
  ) {
    return 'PRE_PHYSICAL';
  }

  return 'PRE_PHYSICAL';
}

const currencyFormatter = new Intl.NumberFormat('ru-RU', {
  style: 'currency',
  currency: 'RUB',
  maximumFractionDigits: 0,
});

const STATUS_LABELS: Record<string, { label: string; color: string; dot: string }> = {
  requested: { label: 'Возврат оформлен', color: 'text-yellow-800 bg-yellow-100', dot: 'bg-yellow-500' },
  needs_info: { label: 'Требуется информация', color: 'text-yellow-800 bg-yellow-100', dot: 'bg-yellow-500' },
  approved: { label: 'В пути в ZAMK', color: 'text-blue-800 bg-blue-100', dot: 'bg-blue-500' },
  rejected: { label: 'Отклонён', color: 'text-red-800 bg-red-100', dot: 'bg-red-500' },
  item_received: { label: 'Получен ZAMK', color: 'text-indigo-800 bg-indigo-100', dot: 'bg-indigo-500' },
  completed: { label: 'Возврат завершён', color: 'text-emerald-800 bg-emerald-100', dot: 'bg-emerald-500' },
  refunded: { label: 'Возврат завершён', color: 'text-emerald-800 bg-emerald-100', dot: 'bg-emerald-500' },
  cancelled: { label: 'Отменён', color: 'text-gray-600 bg-gray-100', dot: 'bg-gray-500' },
};

export const RETURN_REASON_LABELS: Record<string, string> = {
  defective: 'Товар неисправен',
  damaged: 'Товар повреждён',
  wrong_item: 'Привезли не тот товар',
  not_as_described: 'Не соответствует описанию',
  incomplete: 'Неполная комплектация',
  size_fit: 'Не подошёл размер',
  changed_mind: 'Передумал',
  other: 'Другое',
};

const OUTCOME_CONFIG: Record<string, { label: string; color: string }> = {
  restocked: {
    label: 'Возвращён в продажу',
    color: 'text-emerald-800 bg-emerald-100 border-emerald-200',
  },
  damaged: {
    label: 'Повреждён (брак)',
    color: 'text-rose-800 bg-rose-100 border-rose-200',
  },
  rejected: {
    label: 'Отклонён складом',
    color: 'text-red-800 bg-red-100 border-red-200',
  },
  partial_restock: {
    label: 'Частично в продажу',
    color: 'text-blue-800 bg-blue-100 border-blue-200',
  },
  in_inspection: {
    label: 'Проверяется на складе ZAMK',
    color: 'text-indigo-800 bg-indigo-100 border-indigo-200',
  },
  arrived_at_zamk: {
    label: 'Поступил на склад ZAMK',
    color: 'text-blue-800 bg-blue-100 border-blue-200',
  },
  in_transit: {
    label: 'В пути на склад ZAMK',
    color: 'text-amber-800 bg-amber-100 border-amber-200',
  },
  awaiting_handover: {
    label: 'Ожидает передачи перевозчику',
    color: 'text-amber-800 bg-amber-100 border-amber-200',
  },
  awaiting_shipment: {
    label: 'Ожидает отправки покупателем',
    color: 'text-amber-800 bg-amber-100 border-amber-200',
  },
  awaiting_arrival: {
    label: 'В пути на склад ZAMK',
    color: 'text-amber-800 bg-amber-100 border-amber-200',
  },
  needs_info: {
    label: 'Требуется информация',
    color: 'text-yellow-800 bg-yellow-100 border-yellow-200',
  },
  requested: {
    label: 'На рассмотрении',
    color: 'text-blue-800 bg-blue-100 border-blue-200',
  },
  not_received: {
    label: 'Не поступил',
    color: 'text-gray-700 bg-gray-100 border-gray-200',
  },
  rejected_by_support: {
    label: 'Отклонён поддержкой',
    color: 'text-red-800 bg-red-100 border-red-200',
  },
  cancelled: {
    label: 'Отменён покупателем',
    color: 'text-gray-600 bg-gray-100 border-gray-200',
  },
};

export interface SellerOutcomeMeaning {
  primary: string;
  consequence: string;
}

export function getSellerOutcomeMeaning(item: {
  physicalOutcome: string;
  quantity?: number;
  restockedQuantity?: number;
  damagedQuantity?: number;
  rejectedQuantity?: number;
  notReceivedQuantity?: number;
}): SellerOutcomeMeaning {
  const count = item.quantity || 1;
  const restocked = item.restockedQuantity || 0;

  switch (item.physicalOutcome) {
    case 'restocked':
      return {
        primary: 'Товар снова доступен к продаже',
        consequence: count === 1
          ? 'ZAMK принял и проверил товар. Возвращённая единица добавлена обратно в доступный складской остаток.'
          : `ZAMK принял и проверил товар. Возвращённые единицы (${restocked} шт.) добавлены обратно в доступный складской остаток.`,
      };
    case 'damaged':
      return {
        primary: 'Товар не возвращён в продажу',
        consequence: 'При проверке ZAMK товар признан повреждённым. Доступный остаток продавца не увеличен.',
      };
    case 'rejected':
      return {
        primary: 'Товар отклонён складом и не возвращён в продажу',
        consequence: 'Товар не прошёл проверку склада ZAMK. Доступный складской остаток не увеличен.',
      };
    case 'partial_restock':
      return {
        primary: 'Часть товара возвращена в продажу',
        consequence: `В доступный остаток возвращено: ${restocked} из ${count} шт. Остальные единицы повреждены или не приняты складом. Сверьтесь с подробным распределением ниже.`,
      };
    case 'not_received':
      return {
        primary: 'Товар фактически не поступил на склад',
        consequence: 'Физическая единица не была получена складом ZAMK при обработке возврата. Складской остаток не увеличен.',
      };
    case 'in_inspection':
      return {
        primary: 'Идёт проверка товара на складе ZAMK',
        consequence: 'Специалисты склада ZAMK проверяют состояние товара. Окончательное решение по возврату в продажу ещё не принято, складской остаток пока не изменился.',
      };
    case 'arrived_at_zamk':
      return {
        primary: 'Товар поступил на склад ZAMK и ожидает проверки',
        consequence: 'Посылка доставлена на склад ZAMK. Физическая проверка ещё не начата, складской остаток пока не меняется.',
      };
    case 'in_transit':
    case 'awaiting_arrival':
      return {
        primary: 'Товар находится в пути на склад ZAMK',
        consequence: 'Товар перемещается службой доставки на склад ZAMK. Решение о возврате в продажу будет принято после физической приёмки. Складской остаток пока не меняется.',
      };
    case 'awaiting_handover':
      return {
        primary: 'Ожидается передача возврата в доставку',
        consequence: 'Возврат подтверждён, покупатель ещё не передал товар перевозчику. Физический результат и решение по остаткам будут определены после доставки и проверки на складе.',
      };
    case 'awaiting_shipment':
      return {
        primary: 'Ожидается отправка товара покупателем',
        consequence: 'Покупатель оформляет отправку возврата. Товар ещё не поступил в логистику, складской остаток не меняется.',
      };
    case 'needs_info':
      return {
        primary: 'По возврату пока нет физического результата',
        consequence: 'Поддержка запросила дополнительную информацию у покупателя. Складской остаток продавца пока не меняется.',
      };
    case 'requested':
      return {
        primary: 'Заявка на возврат находится на рассмотрении',
        consequence: 'Поддержка рассматривает обращение покупателя. Товар не отправлен на склад, складской остаток не меняется.',
      };
    case 'rejected_by_support':
      return {
        primary: 'Возврат отклонён поддержкой',
        consequence: 'Заявка отклонена до отправки на склад. Физический товар остаётся у покупателя, складской остаток не меняется.',
      };
    case 'cancelled':
      return {
        primary: 'Возврат отменён покупателем',
        consequence: 'Покупатель отозвал заявку на возврат. Товар остаётся у покупателя, складской остаток продавца не меняется.',
      };
    default:
      return {
        primary: 'Обработка возврата завершена',
        consequence: 'Проверка завершена. Актуальное распределение по остаткам отражено в количественных показателях ниже.',
      };
  }
}

const SHIPMENT_STATUS_LABELS: Record<string, string> = {
  draft: 'Черновик накладной',
  awaiting_handover: 'Ожидает передачи перевозчику',
  handed_over: 'Передан перевозчику',
  in_transit: 'В пути',
  arrived_at_zamk: 'Доставлен на склад ZAMK',
  cancelled: 'Отменена',
};

const SHIPMENT_METHOD_LABELS: Record<string, string> = {
  cdek_courier: 'СДЭК (Курьер)',
  cdek_office: 'СДЭК (ПВЗ)',
};

const DISPOSITION_LABELS: Record<string, { label: string; color: string }> = {
  restock: { label: 'В продажу', color: 'text-emerald-700 bg-emerald-50 border-emerald-200' },
  damaged: { label: 'Брак / повреждён', color: 'text-rose-700 bg-rose-50 border-rose-200' },
  reject: { label: 'Отклонён', color: 'text-red-700 bg-red-50 border-red-200' },
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
        setReturnItems(data.items || (Array.isArray(data) ? data : []));
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

  const ret = returnItems[0];
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
        {returnItems.map(item => {
          const presentationMode = getReturnPresentationMode(item);
          const outcome = OUTCOME_CONFIG[item.physicalOutcome] || {
            label: item.physicalOutcome || 'Обработка',
            color: 'text-gray-800 bg-gray-100 border-gray-200',
            description: '',
          };

          return (
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

              <div className="p-6 space-y-6">
                {/* Reason */}
                <div>
                  <h4 className="text-xs font-bold uppercase tracking-wider text-ash mb-3">Причина возврата покупателем</h4>
                  <div className="bg-gray-50/50 dark:bg-white/5 border border-border-soft dark:border-white/10 p-4 rounded-xl text-sm text-graphite dark:text-white/90">
                    <p className="font-medium mb-1">{item.reason ? (RETURN_REASON_LABELS[item.reason] || item.reason) : 'Не указана'}</p>
                    {item.condition && <p className="text-graphite-light mt-2 italic text-xs">Комментарий: {item.condition}</p>}
                  </div>
                </div>

                {/* Section 1: Logistics */}
                <div className="bg-blue-50/40 dark:bg-blue-950/20 border border-blue-100 dark:border-blue-900/30 rounded-xl p-5">
                  <div className="flex items-center gap-2 mb-3">
                    <Truck className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                    <h4 className="text-sm font-bold text-blue-950 dark:text-blue-200">Логистика возврата</h4>
                  </div>
                  <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4 text-sm">
                    <div>
                      <span className="text-xs text-ash block mb-1">Способ доставки</span>
                      <span className="font-medium text-graphite dark:text-white">
                        {item.shipmentMethod ? (SHIPMENT_METHOD_LABELS[item.shipmentMethod] || item.shipmentMethod) : '—'}
                      </span>
                    </div>
                    <div>
                      <span className="text-xs text-ash block mb-1">Трек-номер</span>
                      <span className="font-mono font-medium text-graphite dark:text-white">
                        {item.trackingNumber || 'Не присвоен'}
                      </span>
                    </div>
                    <div>
                      <span className="text-xs text-ash block mb-1">Статус перевозки</span>
                      <span className="font-medium text-graphite dark:text-white">
                        {item.logisticsStatus ? (SHIPMENT_STATUS_LABELS[item.logisticsStatus] || item.logisticsStatus) : '—'}
                      </span>
                    </div>
                  </div>
                  <div className="mt-3 pt-3 border-t border-blue-100/80 dark:border-blue-900/40 flex items-center gap-2 text-xs">
                    {item.arrivedAtZamk ? (
                      <span className="inline-flex items-center text-emerald-700 dark:text-emerald-400 font-medium gap-1.5">
                        <Check className="w-3.5 h-3.5" /> Посылка поступила на склад ZAMK
                      </span>
                    ) : item.status === 'needs_info' ? (
                      <span className="inline-flex items-center text-amber-700 dark:text-amber-400 font-medium gap-1.5">
                        <Clock className="w-3.5 h-3.5" /> Ожидается ответ покупателя по запросу информации
                      </span>
                    ) : item.status === 'requested' ? (
                      <span className="inline-flex items-center text-blue-700 dark:text-blue-400 font-medium gap-1.5">
                        <Clock className="w-3.5 h-3.5" /> Заявка на возврат ожидает рассмотрения поддержкой
                      </span>
                    ) : (
                      <span className="inline-flex items-center text-amber-700 dark:text-amber-400 font-medium gap-1.5">
                        <Clock className="w-3.5 h-3.5" /> Ожидается поступление на склад ZAMK
                      </span>
                    )}
                  </div>
                </div>

                {/* Section 2: Warehouse Physical Outcome */}
                <div className="bg-white dark:bg-white/5 border border-border-soft dark:border-white/10 rounded-xl p-5 shadow-sm">
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
                    <div className="flex items-center gap-2">
                      <ShieldCheck className="w-5 h-5 text-indigo-600 dark:text-indigo-400" />
                      <h4 className="text-sm font-bold text-graphite dark:text-white">Результат проверки ZAMK</h4>
                    </div>
                    <span className={cn("px-3 py-1 rounded-full text-xs font-semibold border inline-flex items-center gap-1.5", outcome.color)}>
                      {item.physicalOutcome === 'restocked' && <CheckCircle2 className="w-3.5 h-3.5" />}
                      {item.physicalOutcome === 'damaged' && <AlertTriangle className="w-3.5 h-3.5" />}
                      {item.physicalOutcome === 'rejected' && <XCircle className="w-3.5 h-3.5" />}
                      {outcome.label}
                    </span>
                  </div>

                  {/* High-priority Seller Business Outcome Summary */}
                  {(() => {
                    const sellerMeaning = getSellerOutcomeMeaning(item);
                    return (
                      <div data-testid="seller-outcome-meaning" className="mb-4 p-4 rounded-xl border bg-gray-50/70 dark:bg-white/5 border-border-soft dark:border-white/10">
                        <div className="text-xs font-bold uppercase tracking-wider text-ash mb-1">
                          Что это значит для вас
                        </div>
                        <div className="text-sm font-semibold text-graphite dark:text-white mb-1">
                          {sellerMeaning.primary}
                        </div>
                        <p className="text-xs text-graphite-light dark:text-white/70 leading-relaxed">
                          {sellerMeaning.consequence}
                        </p>
                      </div>
                    );
                  })()}

                  {/* Presentation Mode Body */}
                  {presentationMode === 'FINAL_OUTCOME' && (
                    <>
                      {/* Quantity Breakdown Counters */}
                      <div data-testid="final-outcome-breakdown" className="grid grid-cols-2 sm:grid-cols-5 gap-3 mb-4">
                        <div className="bg-gray-50 dark:bg-white/5 p-3 rounded-xl text-center border border-gray-100 dark:border-white/5">
                          <span className="text-xs text-ash block mb-1">Заявлено</span>
                          <span className="text-lg font-bold text-graphite dark:text-white">{item.quantity}</span>
                        </div>
                        <div className="bg-emerald-50 dark:bg-emerald-950/20 p-3 rounded-xl text-center border border-emerald-100 dark:border-emerald-900/30">
                          <span className="text-xs text-emerald-800 dark:text-emerald-300 block mb-1">В продажу</span>
                          <span className="text-lg font-bold text-emerald-700 dark:text-emerald-400">{item.restockedQuantity ?? 0}</span>
                        </div>
                        <div className="bg-rose-50 dark:bg-rose-950/20 p-3 rounded-xl text-center border border-rose-100 dark:border-rose-900/30">
                          <span className="text-xs text-rose-800 dark:text-rose-300 block mb-1">Брак</span>
                          <span className="text-lg font-bold text-rose-700 dark:text-rose-400">{item.damagedQuantity ?? 0}</span>
                        </div>
                        <div className="bg-red-50 dark:bg-red-950/20 p-3 rounded-xl text-center border border-red-100 dark:border-red-900/30">
                          <span className="text-xs text-red-800 dark:text-red-300 block mb-1">Отклонено</span>
                          <span className="text-lg font-bold text-red-700 dark:text-red-400">{item.rejectedQuantity ?? 0}</span>
                        </div>
                        <div className="bg-gray-100 dark:bg-white/10 p-3 rounded-xl text-center border border-gray-200 dark:border-white/10">
                          <span className="text-xs text-ash block mb-1">Не поступило</span>
                          <span className="text-lg font-bold text-graphite-light dark:text-white/60">{item.notReceivedQuantity ?? 0}</span>
                        </div>
                      </div>

                      {/* Serialized Units Table */}
                      {item.units && item.units.length > 0 && (
                        <div className="mt-4 pt-4 border-t border-border-soft dark:border-white/10">
                          <h5 className="text-xs font-semibold text-graphite-light dark:text-white/60 mb-2 flex items-center gap-1.5">
                            <QrCode className="w-3.5 h-3.5" /> Идентифицированные единицы (ZMU)
                          </h5>
                          <div className="overflow-x-auto">
                            <table className="w-full text-left text-xs">
                              <thead>
                                <tr className="border-b border-border-soft dark:border-white/10 text-ash">
                                  <th className="py-2 px-3 font-medium">Код единицы (ZMU)</th>
                                  <th className="py-2 px-3 font-medium">Решение склада</th>
                                  <th className="py-2 px-3 font-medium">Время сканирования</th>
                                </tr>
                              </thead>
                              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                                {item.units.map(u => {
                                  const disp = u.disposition ? (DISPOSITION_LABELS[u.disposition] || { label: u.disposition, color: 'text-gray-700 bg-gray-50 border-gray-200' }) : null;
                                  return (
                                    <tr key={u.unitCode}>
                                      <td className="py-2 px-3 font-mono font-medium text-graphite dark:text-white">
                                        {u.unitCode}
                                      </td>
                                      <td className="py-2 px-3">
                                        {disp ? (
                                          <span className={cn("px-2 py-0.5 rounded-full text-xs font-medium border", disp.color)}>
                                            {disp.label}
                                          </span>
                                        ) : (
                                          <span className="text-ash">Не проверен</span>
                                        )}
                                      </td>
                                      <td className="py-2 px-3 text-ash">
                                        {u.scannedAt ? new Date(u.scannedAt).toLocaleString('ru-RU') : '—'}
                                      </td>
                                    </tr>
                                  );
                                })}
                              </tbody>
                            </table>
                          </div>
                        </div>
                      )}
                    </>
                  )}

                  {presentationMode === 'PRE_PHYSICAL' && (
                    <div data-testid="pre-physical-status" className="p-3 bg-gray-50/50 dark:bg-white/5 rounded-xl border border-border-soft dark:border-white/10 text-xs text-ash flex items-center gap-2">
                      <Clock className="w-4 h-4 text-ash shrink-0" />
                      <span>Физическая обработка ещё не началась</span>
                    </div>
                  )}

                  {presentationMode === 'LOGISTICS' && (
                    <div data-testid="logistics-status" className="p-3 bg-blue-50/30 dark:bg-blue-950/10 rounded-xl border border-blue-100/70 dark:border-blue-900/20 text-xs text-blue-900 dark:text-blue-200 flex items-center gap-2">
                      <Truck className="w-4 h-4 text-blue-600 dark:text-blue-400 shrink-0" />
                      <span>Товар в процессе доставки на склад ZAMK. Физическая проверка ещё не начата.</span>
                    </div>
                  )}

                  {presentationMode === 'WAREHOUSE_PROCESSING' && (
                    <div data-testid="warehouse-processing-status" className="space-y-3">
                      <div className="p-3 bg-amber-50/30 dark:bg-amber-950/10 rounded-xl border border-amber-100 dark:border-amber-900/30 text-xs text-amber-900 dark:text-amber-200 flex items-center gap-2">
                        <Clock className="w-4 h-4 text-amber-600 dark:text-amber-400 shrink-0" />
                        <span>Идёт приёмка и осмотр товара специалистами склада ZAMK. Окончательное решение по возврату в продажу ещё не принято.</span>
                      </div>
                      {item.units && item.units.length > 0 && (
                        <div className="mt-4 pt-4 border-t border-border-soft dark:border-white/10">
                          <h5 className="text-xs font-semibold text-graphite-light dark:text-white/60 mb-2 flex items-center gap-1.5">
                            <QrCode className="w-3.5 h-3.5" /> Отсканированные единицы в процессе проверки
                          </h5>
                          <div className="overflow-x-auto">
                            <table className="w-full text-left text-xs">
                              <thead>
                                <tr className="border-b border-border-soft dark:border-white/10 text-ash">
                                  <th className="py-2 px-3 font-medium">Код единицы (ZMU)</th>
                                  <th className="py-2 px-3 font-medium">Текущий статус</th>
                                  <th className="py-2 px-3 font-medium">Время сканирования</th>
                                </tr>
                              </thead>
                              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                                {item.units.map(u => {
                                  const disp = u.disposition ? (DISPOSITION_LABELS[u.disposition] || { label: u.disposition, color: 'text-gray-700 bg-gray-50 border-gray-200' }) : null;
                                  return (
                                    <tr key={u.unitCode}>
                                      <td className="py-2 px-3 font-mono font-medium text-graphite dark:text-white">
                                        {u.unitCode}
                                      </td>
                                      <td className="py-2 px-3">
                                        {disp ? (
                                          <span className={cn("px-2 py-0.5 rounded-full text-xs font-medium border", disp.color)}>
                                            {disp.label}
                                          </span>
                                        ) : (
                                          <span className="text-ash">Ожидает решения</span>
                                        )}
                                      </td>
                                      <td className="py-2 px-3 text-ash">
                                        {u.scannedAt ? new Date(u.scannedAt).toLocaleString('ru-RU') : '—'}
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
                  )}

                  {/* Inspection timing metadata */}
                  {(item.receivingStartedAt || item.completedAt || item.inspectionCompleted) && (
                    <div className="mt-3 pt-3 border-t border-dashed border-gray-200 dark:border-white/10 flex flex-wrap gap-4 text-xs text-ash">
                      {item.receivingStartedAt && (
                        <span>Начало приёмки: {new Date(item.receivingStartedAt).toLocaleString('ru-RU')}</span>
                      )}
                      {item.completedAt && (
                        <span>Завершение приёмки: {new Date(item.completedAt).toLocaleString('ru-RU')}</span>
                      )}
                      <span>Статус проверки: {item.inspectionCompleted ? 'Завершена' : 'В процессе / Ожидает'}</span>
                    </div>
                  )}
                </div>

                {/* Section 3: Financial adjustment */}
                <div>
                  <h4 className="text-xs font-bold uppercase tracking-wider text-ash mb-3">Финансовый результат</h4>
                  {item.financialAdjustmentCents != null ? (
                    <div className="flex items-start gap-3 bg-red-50/50 p-4 rounded-xl border border-red-100">
                      <div className="text-sm">
                        <p className="font-bold text-red-600">
                          {currencyFormatter.format(item.financialAdjustmentCents / 100)}
                        </p>
                        <p className="text-red-800/80 mt-1 text-xs font-medium">
                          {item.financialImpactType === 'frozen' && 'Удержание из замороженных средств'}
                          {item.financialImpactType === 'available' && 'Удержание из доступного баланса'}
                          {item.financialImpactType === 'debt' && 'Сумма будет удержана из будущих выплат.'}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div className="text-sm text-graphite-light italic bg-gray-50/50 p-4 rounded-xl border border-gray-100">
                      Удержание пока не сформировано
                    </div>
                  )}
                </div>
              </div>

              {/* Footer */}
              <div className="bg-gray-100/50 dark:bg-white/5 border-t border-border-soft dark:border-white/10 px-6 py-4">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center text-xs text-graphite-light">
                  <span>Последнее обновление: {new Date(item.updatedAt).toLocaleString('ru-RU')}</span>
                  <span>Возврат № {item.returnId.split('-')[0]}</span>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
