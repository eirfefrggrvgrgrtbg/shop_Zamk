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
  return `[MISSING ORDER_NUMBER]`;
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
  created: { label: 'Создан', bg: 'bg-slate-700', text: 'text-slate-200' },
  awaiting_payment: { label: 'Ожидает оплаты', bg: 'bg-amber-900/60', text: 'text-amber-300' },
  paid: { label: 'Оплачен', bg: 'bg-emerald-950', text: 'text-emerald-300' },
  assembling: { label: 'Собирается', bg: 'bg-blue-950', text: 'text-blue-300' },
  packed: { label: 'Упакован', bg: 'bg-indigo-950', text: 'text-indigo-300' },
  shipped: { label: 'В пути', bg: 'bg-purple-950', text: 'text-purple-300' },
  delivered: { label: 'Доставлен', bg: 'bg-emerald-900', text: 'text-emerald-200' },
  cancelled: { label: 'Отменён', bg: 'bg-rose-950', text: 'text-rose-300' },
};

export const fulfillmentStatusMap: Record<string, { label: string; bg: string; text: string }> = {
  awaiting_payment: { label: 'Ожидает оплаты', bg: 'bg-slate-800', text: 'text-slate-300' },
  paid: { label: 'Оплачена', bg: 'bg-emerald-950', text: 'text-emerald-300' },
  pending: { label: 'Ожидает сборки', bg: 'bg-amber-950', text: 'text-amber-300' },
  assembling: { label: 'Собирается', bg: 'bg-blue-950', text: 'text-blue-300' },
  packed: { label: 'Ожидает приёмки', bg: 'bg-indigo-950', text: 'text-indigo-300' },
  accepted: { label: 'Принята на хабе', bg: 'bg-emerald-900', text: 'text-emerald-200' },
  discrepancy: { label: 'Обнаружено расхождение', bg: 'bg-rose-950', text: 'text-rose-300' },
  shipped: { label: 'Передана в доставку', bg: 'bg-purple-950', text: 'text-purple-300' },
  delivered: { label: 'Доставлена', bg: 'bg-emerald-950', text: 'text-emerald-200' },
  cancelled: { label: 'Отменена', bg: 'bg-rose-950', text: 'text-rose-400' },
};

export const paymentStatusMap: Record<string, { label: string; bg: string; text: string }> = {
  pending: { label: 'Ожидает оплаты', bg: 'bg-amber-950', text: 'text-amber-300' },
  succeeded: { label: 'Оплачен', bg: 'bg-emerald-950', text: 'text-emerald-300' },
  failed: { label: 'Ошибка оплаты', bg: 'bg-rose-950', text: 'text-rose-300' },
  refunded: { label: 'Возвращён', bg: 'bg-purple-950', text: 'text-purple-300' },
};
