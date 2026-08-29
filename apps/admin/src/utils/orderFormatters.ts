export function formatMoney(cents: number): string {
  const rubles = Math.round(cents / 100);
  return new Intl.NumberFormat('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    maximumFractionDigits: 0,
  }).format(rubles);
}

export function formatOrderNumber(order: { id: string; orderNumber?: string | null }): string {
  if (order.orderNumber) {
    return order.orderNumber;
  }
  return order.id ? `ORD-${order.id.slice(0, 8).toUpperCase()}` : '—';
}

export function formatDateTime(dateStr?: string | null): string {
  if (!dateStr) return '—';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '—';
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export const orderStatusMap: Record<string, { label: string; bg: string; text: string }> = {
  created: { label: 'Создан', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' },
  awaiting_payment: { label: 'Ожидает оплаты', bg: 'bg-amber-50 border border-amber-200', text: 'text-amber-700' },
  paid: { label: 'Оплачен', bg: 'bg-emerald-50 border border-emerald-200', text: 'text-emerald-700' },
  assembling: { label: 'Собирается', bg: 'bg-blue-50 border border-blue-200', text: 'text-blue-700' },
  packed: { label: 'Упакован', bg: 'bg-indigo-50 border border-indigo-200', text: 'text-indigo-700' },
  shipped: { label: 'В пути', bg: 'bg-purple-50 border border-purple-200', text: 'text-purple-700' },
  delivered: { label: 'Доставлен', bg: 'bg-emerald-50 border border-emerald-300', text: 'text-emerald-800' },
  cancelled: { label: 'Отменён', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' },
};

export const fulfillmentStatusMap: Record<string, { label: string; bg: string; text: string }> = {
  awaiting_payment: { label: 'Ожидает оплаты', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-600' },
  paid: { label: 'Ожидает сборки', bg: 'bg-amber-50 border border-amber-200', text: 'text-amber-700' },
  pending: { label: 'Не сформирована', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-500' },
  assembling: { label: 'В сборке', bg: 'bg-blue-50 border border-blue-200', text: 'text-blue-700' },
  packed: { label: 'Упакована', bg: 'bg-indigo-50 border border-indigo-200', text: 'text-indigo-700' },
  accepted: { label: 'Принята на хабе', bg: 'bg-emerald-50 border border-emerald-200', text: 'text-emerald-700' },
  discrepancy: { label: 'Расхождение', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' },
  shipped: { label: 'Отгружена', bg: 'bg-purple-50 border border-purple-200', text: 'text-purple-700' },
  delivered: { label: 'Доставлена', bg: 'bg-emerald-50 border border-emerald-300', text: 'text-emerald-800' },
  cancelled: { label: 'Отменена', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' },
};

export interface OrderFulfillmentBadgeInput {
  status: string;
  fulfillmentStatus?: string | null;
  fulfillmentsCount?: number;
}

export function getOrderFulfillmentBadge(order: OrderFulfillmentBadgeInput): { label: string; bg: string; text: string } {
  // If no fulfillment exists
  if (!order.fulfillmentsCount || order.fulfillmentsCount <= 0) {
    if (order.status === 'awaiting_payment') {
      return { label: 'Ожидает оплаты', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-600' };
    }
    return { label: 'Не сформирована', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-500' };
  }

  // Real fulfillment exists
  const fStatus = order.fulfillmentStatus;
  switch (fStatus) {
    case 'paid':
      return { label: 'Ожидает сборки', bg: 'bg-amber-50 border border-amber-200', text: 'text-amber-700' };
    case 'assembling':
      return { label: 'В сборке', bg: 'bg-blue-50 border border-blue-200', text: 'text-blue-700' };
    case 'packed':
      return { label: 'Упакована', bg: 'bg-indigo-50 border border-indigo-200', text: 'text-indigo-700' };
    case 'accepted':
      return { label: 'Принята на хабе', bg: 'bg-emerald-50 border border-emerald-200', text: 'text-emerald-700' };
    case 'discrepancy':
      return { label: 'Расхождение', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' };
    case 'shipped':
      return { label: 'Отгружена', bg: 'bg-purple-50 border border-purple-200', text: 'text-purple-700' };
    case 'delivered':
      return { label: 'Доставлена', bg: 'bg-emerald-50 border border-emerald-300', text: 'text-emerald-800' };
    case 'cancelled':
      return { label: 'Отменена', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' };
    case 'awaiting_payment':
      return { label: 'Ожидает оплаты', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-600' };
    default:
      if (!fStatus || fStatus === 'pending') {
        if (order.status === 'awaiting_payment') {
          return { label: 'Ожидает оплаты', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-600' };
        }
        return { label: 'Не сформирована', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-500' };
      }
      return { label: fStatus, bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };
  }
}

export const paymentStatusMap: Record<string, { label: string; bg: string; text: string }> = {
  pending: { label: 'Ожидает оплаты', bg: 'bg-amber-50 border border-amber-200', text: 'text-amber-700' },
  succeeded: { label: 'Оплачен', bg: 'bg-emerald-50 border border-emerald-200', text: 'text-emerald-700' },
  failed: { label: 'Ошибка оплаты', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' },
  refunded: { label: 'Возвращён', bg: 'bg-purple-50 border border-purple-200', text: 'text-purple-700' },
};

export function getOrderPaymentBadge(order: { paymentStatus?: string | null }): { label: string; bg: string; text: string } {
  const ps = order.paymentStatus || 'pending';

  switch (ps) {
    case 'paid':
    case 'succeeded':
      return { label: 'Оплачен', bg: 'bg-emerald-50 border border-emerald-200', text: 'text-emerald-700' };
    case 'failed':
      return { label: 'Ошибка', bg: 'bg-rose-50 border border-rose-200', text: 'text-rose-700' };
    case 'cancelled':
      return { label: 'Отменен', bg: 'bg-gray-100 border border-gray-200', text: 'text-gray-700' };
    case 'pending':
    case 'created':
    case 'awaiting_payment':
    default:
      return { label: 'Ожидает', bg: 'bg-amber-50 border border-amber-200', text: 'text-amber-700' };
  }
}
